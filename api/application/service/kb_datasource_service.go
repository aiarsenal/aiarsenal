package service

// kb_datasource_service 知识库数据源：连接器任务的 CRUD、手动触发、定时调度与
// 同步执行引擎。连接器（取数）在 infrastructure/kbprovider，本服务负责装配：
// 每次 run 建 aa_kb_datasource_run 记录 → 连接器 Sync 逐条回调 ingest（复用
// KbService 的清洗/分块/嵌入/去重管线）→ 收尾回写 stats/last_run_at/last_status。
// 检索透传：实现 KbService.ExternalKBSearcher，外部知识库目录搜索远端代理。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/kbprovider"
	"aiarsenal/infrastructure/scheduler"
)

// KbDatasourceService 数据源连接器服务。
// sch 经 SetScheduler 回指（main.go 构造顺序无关）；nil = 不做定时调度（单测/CLI）。
type KbDatasourceService struct {
	dsRepo    repository.IKbDatasourceRepository
	runRepo   repository.IKbDatasourceRunRepository
	dirRepo   repository.IKbDirRepository
	grantRepo repository.IKbGrantRepository
	kb        *KbService
	sch       *scheduler.Scheduler

	mu      sync.Mutex
	running map[string]bool // 进程内同数据源互斥（跨进程由 runRepo.CountRunning 兜底）
}

// NewKbDatasourceService 构造（kb = 入库管线宿主，必传）。
func NewKbDatasourceService(dsRepo repository.IKbDatasourceRepository, runRepo repository.IKbDatasourceRunRepository,
	dirRepo repository.IKbDirRepository, grantRepo repository.IKbGrantRepository, kb *KbService) *KbDatasourceService {
	return &KbDatasourceService{
		dsRepo: dsRepo, runRepo: runRepo, dirRepo: dirRepo, grantRepo: grantRepo,
		kb: kb, running: map[string]bool{},
	}
}

// SetScheduler 注入调度器（main.go 装配点；随后应调 RegisterSchedules 起注册）。
func (s *KbDatasourceService) SetScheduler(sch *scheduler.Scheduler) { s.sch = sch }

// schedulerJobName 调度任务名（同名 Upsert 语义：改 cron/停用重复注册即可）。
func schedulerJobName(dsId string) string { return "kb-ds-" + dsId }

// ---- CRUD ----

// Create 新建数据源。DirId 必须存在且（非-nil 身份时）可写；dsType 校验已注册连接器；
// config 校验为合法 JSON 对象；schedule 非空须为合法 5 字段 cron 并即时注册。
func (s *KbDatasourceService) Create(ctx context.Context, dirId, dsType, name, description, configJSON, schedule string, enabled bool, userId string, t *model.Identity) (*entity.KbDatasource, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("名称不能为空")
	}
	if _, err := kbprovider.For(dsType); err != nil {
		return nil, fmt.Errorf("dsType：%w（可选：%s）", err, strings.Join(kbprovider.Types(), "/"))
	}
	if _, err := kbParseConfig(configJSON); err != nil {
		return nil, err
	}
	if err := s.assertDirWrite(ctx, dirId, t); err != nil {
		return nil, err
	}
	schedule, err := kbNormalizeSchedule(schedule)
	if err != nil {
		return nil, err
	}
	d := &entity.KbDatasource{
		DirId: dirId, DsType: dsType, Name: name, Description: description,
		Config: kbCompactJSON(configJSON), Schedule: schedule, IsEnabled: enabled,
		CreatedBy: userId,
	}
	if err := s.dsRepo.Add(ctx, d); err != nil {
		return nil, err
	}
	s.syncSchedule(d) // 启用且带 cron：注册调度
	return d, nil
}

// Update 修改数据源（name/dir/type/config/schedule/enabled 任选）。改动即时反映到调度。
func (s *KbDatasourceService) Update(ctx context.Context, dsId, dirId, dsType, name, description, configJSON, schedule string, enabled *bool, t *model.Identity) (*entity.KbDatasource, error) {
	d, err := s.dsRepo.Get(ctx, dsId)
	if err != nil {
		return nil, errors.New("数据源不存在")
	}
	if err := s.assertDirWrite(ctx, d.DirId, t); err != nil {
		return nil, err
	}
	if dirId != "" && dirId != d.DirId {
		if err := s.assertDirWrite(ctx, dirId, t); err != nil {
			return nil, err
		}
		d.DirId = dirId
	}
	if dsType != "" && dsType != d.DsType {
		if _, err := kbprovider.For(dsType); err != nil {
			return nil, fmt.Errorf("dsType：%w", err)
		}
		d.DsType = dsType
	}
	if name != "" {
		if name = strings.TrimSpace(name); name == "" {
			return nil, errors.New("名称不能为空")
		}
		d.Name = name
	}
	d.Description = description // 说明随编辑整包覆盖（空串=清空）
	if configJSON != "" {
		if _, err := kbParseConfig(configJSON); err != nil {
			return nil, err
		}
		d.Config = kbCompactJSON(configJSON)
	}
	if schedule != "" || d.Schedule != "" { // 传新值或本有 cron 都走归一（空串=取消调度）
		norm, err := kbNormalizeSchedule(schedule)
		if err != nil {
			return nil, err
		}
		d.Schedule = norm
	}
	if enabled != nil {
		d.IsEnabled = *enabled
	}
	if err := s.dsRepo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.syncSchedule(d)
	return d, nil
}

// Delete 删除数据源（连同执行记录、摘除调度）。删除者须对归属目录可写。
func (s *KbDatasourceService) Delete(ctx context.Context, dsId string, t *model.Identity) error {
	d, err := s.dsRepo.Get(ctx, dsId)
	if err != nil {
		return errors.New("数据源不存在")
	}
	if err := s.assertDirWrite(ctx, d.DirId, t); err != nil {
		return err
	}
	if err := s.dsRepo.Delete(ctx, dsId); err != nil {
		return err
	}
	_ = s.runRepo.DeleteByDatasource(ctx, dsId)
	s.schUnregister(dsId)
	return nil
}

// Get 详情（附执行记录数由 ListRuns 承担，这里只回实体）。
func (s *KbDatasourceService) Get(ctx context.Context, dsId string) (*entity.KbDatasource, error) {
	return s.dsRepo.Get(ctx, dsId)
}

// List 分页（keyword 模糊名称；dsType ”=全部）。
func (s *KbDatasourceService) List(ctx context.Context, keyword, dsType string, pageNum, pageSize int) ([]*entity.KbDatasource, int64, error) {
	return s.dsRepo.List(ctx, keyword, dsType, pageNum, pageSize)
}

// ListRuns 某数据源的执行记录（倒序分页）。
func (s *KbDatasourceService) ListRuns(ctx context.Context, dsId string, pageNum, pageSize int) ([]*entity.KbDatasourceRun, int64, error) {
	return s.runRepo.ListByDatasource(ctx, dsId, pageNum, pageSize)
}

// ---- 触发与调度 ----

// TriggerRun 手动触发一次同步。进程内互斥 + 跨进程 CountRunning 双保险；
// 返回 run 记录 id，执行异步进行（HTTP 请求不等长任务跑完）。
func (s *KbDatasourceService) TriggerRun(ctx context.Context, dsId string, t *model.Identity) (*entity.KbDatasourceRun, error) {
	d, err := s.dsRepo.Get(ctx, dsId)
	if err != nil {
		return nil, errors.New("数据源不存在")
	}
	if !d.IsEnabled {
		return nil, errors.New("数据源已停用")
	}
	if err := s.assertDirWrite(ctx, d.DirId, t); err != nil {
		return nil, err
	}
	if err := s.beginRun(d.DsId); err != nil {
		return nil, err
	}
	run, err := s.startRun(context.Background(), d, "manual")
	if err != nil {
		s.endRun(d.DsId) // 建记录失败要放回互斥位
		return nil, err
	}
	return run, nil
}

// RegisterSchedules 启动注册：全部启用且带 cron 的数据源（main.go 进程启动时调一次）。
func (s *KbDatasourceService) RegisterSchedules(ctx context.Context) {
	if s.sch == nil {
		return
	}
	list, err := s.dsRepo.ListScheduled(ctx)
	if err != nil {
		slog.Error("kb-datasource: 启动注册调度失败", "err", err)
		return
	}
	for _, d := range list {
		s.syncSchedule(d)
	}
	slog.Info("kb-datasource: 调度注册完成", "count", len(list))
}

// syncSchedule 按当前定义对齐调度：启用+有 cron → 注册（Upsert）；否则摘除。
func (s *KbDatasourceService) syncSchedule(d *entity.KbDatasource) {
	if s.sch == nil {
		return
	}
	name := schedulerJobName(d.DsId)
	if !d.IsEnabled || d.Schedule == "" {
		s.sch.Unregister(name)
		return
	}
	ds := *d // 拷贝给闭包（注册的是当下定义）
	if err := s.sch.Register(name, ds.Schedule, func(ctx context.Context) {
		s.runScheduled(ctx, &ds)
	}); err != nil {
		slog.Error("kb-datasource: 注册调度失败", "ds", ds.DsId, "err", err)
	}
}

// schUnregister 摘除某数据源的调度。
func (s *KbDatasourceService) schUnregister(dsId string) {
	if s.sch != nil {
		s.sch.Unregister(schedulerJobName(dsId))
	}
}

// runScheduled 定时触达点：进程内互斥（跳过并记 debug），竞争失败静默——
// 下一轮 cron 再跑，不重试堆积。
func (s *KbDatasourceService) runScheduled(ctx context.Context, d *entity.KbDatasource) {
	if err := s.beginRun(d.DsId); err != nil {
		slog.Debug("kb-datasource: 上一次同步仍在跑，跳过本轮", "ds", d.DsId)
		return
	}
	if _, err := s.startRun(ctx, d, "cron"); err != nil {
		s.endRun(d.DsId)
		slog.Error("kb-datasource: 定时同步起跑失败", "ds", d.DsId, "err", err)
	}
}

// beginRun 进程内互斥占位。
func (s *KbDatasourceService) beginRun(dsId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[dsId] {
		return errors.New("该数据源正在同步中")
	}
	s.running[dsId] = true
	return nil
}

// endRun 释放互斥位。
func (s *KbDatasourceService) endRun(dsId string) {
	s.mu.Lock()
	delete(s.running, dsId)
	s.mu.Unlock()
}

// ---- 同步执行 ----

// startRun 建 run 记录并异步执行：连接器 Sync + ingest 回调（KbService.IngestSourceText
// upsert）→ Finish 落 stats/error → SetRunState 回写最近一次执行。全程不 panic 外溢。
// trigger 记触发来源（manual=手动接口 / cron=定时调度），落 trigger_type 列供前端展示。
func (s *KbDatasourceService) startRun(ctx context.Context, d *entity.KbDatasource, trigger string) (*entity.KbDatasourceRun, error) {
	// 跨进程兜底：同一数据源已有 running 记录则拒绝（多实例部署防重）
	if n, err := s.runRepo.CountRunning(ctx, d.DsId); err == nil && n > 0 {
		return nil, errors.New("该数据源正在同步中（存在未结束的执行记录）")
	}
	cfg, err := kbParseConfig(d.Config)
	if err != nil {
		return nil, err
	}
	p, err := kbprovider.For(d.DsType)
	if err != nil {
		return nil, err
	}
	run := &entity.KbDatasourceRun{DatasourceId: d.DsId, Status: "running", TriggerType: trigger, Stats: "{}"}
	if err := s.runRepo.Add(ctx, run); err != nil {
		return nil, err
	}
	go s.execute(run, d, p, cfg) // 长任务异步；ctx 用 go 自管，不随 HTTP 请求中断
	return run, nil
}

// execute 单次同步主体（独立 goroutine）。
func (s *KbDatasourceService) execute(run *entity.KbDatasourceRun, d *entity.KbDatasource, p kbprovider.Provider, cfg map[string]any) {
	defer s.endRun(d.DsId)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	reporter := kbprovider.NewRunReporter()
	result := &SourceIngestResult{}
	ingest := func(title, content string, meta map[string]any) error {
		key, _ := meta["key"].(string)
		summary, _ := meta["summary"].(string)
		action, err := s.kb.IngestSourceText(ctx, d.DirId, key, title, summary, content)
		if err != nil {
			return fmt.Errorf("入库失败（%s）：%w", key, err)
		}
		switch action {
		case SourceCreated:
			result.Created++
		case SourceUpdated:
			result.Updated++
		default:
			result.Unchanged++
		}
		return nil
	}

	err := p.Sync(ctx, cfg, ingest, reporter)
	// provider 计数（pages/rows/objects/skipped…）+ ingest 回调计数合并为一份快照
	stats := reporter.Snapshot()
	stats["created"] = result.Created
	stats["updated"] = result.Updated
	stats["unchanged"] = result.Unchanged
	statsJSON, _ := json.Marshal(map[string]any{"counters": stats, "notes": reporter.Notes()})

	now := types.Time(time.Now())
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	finCtx, finCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finCancel()
	_ = s.runRepo.Finish(finCtx, run.RunId, status, string(statsJSON), errMsg, &now)
	_ = s.dsRepo.SetRunState(finCtx, d.DsId, &now, status)
	slog.Info("kb-datasource: 同步结束", "ds", d.DsId, "run", run.RunId, "status", status,
		"created", result.Created, "updated", result.Updated)
}

// ---- 检索透传（KbService.ExternalKBSearcher 实现） ----

// SearchExternal 外部知识库目录搜索：internal 之外，把 external_kb 类型的启用数据源
// 并发代理到远端，结果行带 source=external 与数据源名。权限：t 非 nil 时仅透传
// 其可读目录挂的数据源；nil 身份（系统路径）全透传。单数据源失败降级跳过不整体报错。
func (s *KbDatasourceService) SearchExternal(ctx context.Context, query string, k int, t *model.Identity) []map[string]interface{} {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	list, err := s.dsRepo.ListEnabledByType(ctx, "external_kb")
	if err != nil {
		return nil
	}
	// 过滤可见：可读目录集合（nil 身份全放行）
	allowed := map[string]bool{}
	if t != nil {
		v, err := buildKbPermView(ctx, s.dirRepo, s.grantRepo, *t)
		if err != nil {
			return nil
		}
		for _, d := range list {
			dirId := d.DirId
			if dirId == "" { // 未挂目录的外部源：非管理员不可见
				if t.IsAdmin {
					allowed[d.DsId] = true
				}
				continue
			}
			if v.readableDirs[dirId] {
				allowed[d.DsId] = true
			}
		}
	} else {
		for _, d := range list {
			allowed[d.DsId] = true
		}
	}

	type hit struct{ rows []map[string]interface{} }
	var wg sync.WaitGroup
	ch := make(chan hit, len(list))
	for _, d := range list {
		if !allowed[d.DsId] {
			continue
		}
		wg.Add(1)
		go func(d *entity.KbDatasource) {
			defer wg.Done()
			var cfg map[string]any
			if err := json.Unmarshal([]byte(d.Config), &cfg); err != nil {
				return
			}
			p, err := kbprovider.For(d.DsType)
			if err != nil {
				return
			}
			ek, ok := p.(interface {
				ProxySearch(ctx context.Context, cfg map[string]any, query string, k int) ([]kbprovider.ExternalHit, error)
			})
			if !ok {
				return
			}
			sctx, cancel := context.WithTimeout(ctx, 8*time.Second) // 透传限时：不拖累内部检索响应
			defer cancel()
			hits, err := ek.ProxySearch(sctx, cfg, query, k)
			if err != nil {
				slog.Debug("kb-datasource: 外部检索失败（降级跳过）", "ds", d.DsId, "err", err)
				return
			}
			rows := make([]map[string]interface{}, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, map[string]interface{}{
					"doc_id":     "", // 外部命中无本库文档 id
					"title":      h.Title,
					"file_name":  "",
					"file_type":  "external",
					"seq":        0,
					"content":    h.Content,
					"score":      h.Score,
					"source":     "external",
					"datasource": d.Name,
					"ds_id":      d.DsId,
					"key":        h.Key,
				})
			}
			ch <- hit{rows: rows}
		}(d)
	}
	wg.Wait()
	close(ch)

	var out []map[string]interface{}
	for h := range ch {
		out = append(out, h.rows...)
	}
	return out
}

// ---- 内部工具 ----

// assertDirWrite 数据源管理口权限：目标目录存在且（非-nil 身份时）可写；” = 根仅管理员。
func (s *KbDatasourceService) assertDirWrite(ctx context.Context, dirId string, t *model.Identity) error {
	if t == nil {
		return nil
	}
	if dirId == "" {
		if !t.IsAdmin {
			return errors.New("根目录仅管理员可管理数据源")
		}
		return nil
	}
	d, err := s.dirRepo.Get(ctx, dirId)
	if err != nil {
		return errors.New("目录不存在")
	}
	v, err := buildKbPermView(ctx, s.dirRepo, s.grantRepo, *t)
	if err != nil {
		return err
	}
	if !v.canWriteDir(*t, d) {
		return errors.New("没有操作权限")
	}
	return nil
}

// kbParseConfig config jsonb → map（空/空白按 {}；必须是对象）。
func kbParseConfig(configJSON string) (map[string]any, error) {
	s := strings.TrimSpace(configJSON)
	if s == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("config 不是合法 JSON 对象：%w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// kbCompactJSON 配置规整存储：重序列化去空白（非法由调用方先 kbParseConfig 拦）。
func kbCompactJSON(configJSON string) string {
	m, err := kbParseConfig(configJSON)
	if err != nil {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// kbNormalizeSchedule cron 归一：trim；” = 仅手动（合法）；非空须为 5 字段标准 cron。
func kbNormalizeSchedule(schedule string) (string, error) {
	s := strings.TrimSpace(schedule)
	if s == "" {
		return "", nil
	}
	if _, err := cron.ParseStandard(s); err != nil {
		return "", fmt.Errorf("schedule 不是合法 5 字段 cron 表达式：%w", err)
	}
	return s, nil
}
