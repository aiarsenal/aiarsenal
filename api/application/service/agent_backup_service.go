package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/agentruntime"
	"aiarsenal/infrastructure/idgen"
	"aiarsenal/infrastructure/scheduler"
)

// agent_backup_service.go — 实例工作区备份/恢复 + 定时备份计划（aa_agent_backup /
// aa_agent_backup_schedule 接线）。
//
// 备份实现（localdocker 直连桥接）：CopyFromContainer(容器 /aa) tar 流 → gzip →
// <backupDir>/<backupId>.tar.gz（backupDir 取 config agent.backupDir，默认
// /var/lib/aiarsenal/backup）。恢复为反向解包：gunzip → CopyToContainer(/)。
// 取舍说明：不做临时容器/逐文件快照——单机场景直接打包主卷 /aa 已覆盖 agent 安装、
// 配置与业务数据；挂载的附加持久卷（aiarsenal-vol-*）不在备份范围（卷本身即持久层）。
// CVM 运行时容器在远端宿主，不支持。
//
// 定时：infrastructure/scheduler 进程内 cron，构造时按库内 active 计划逐条注册
// + 保留期清理任务，最后 sch.Start()（幂等，重复调用无副作用）。

// 备份状态（aa_agent_backup.status）：0=pending 1=running 3=done 4=failed。
// backup_type：0=manual 1=schedule。
const (
	backupStatusPending = 0
	backupStatusRunning = 1
	backupStatusDone    = 3
	backupStatusFailed  = 4
)

// AgentBackupService 备份/恢复与定时计划服务。
type AgentBackupService struct {
	backupRepo   repository.IAgentBackupRepository
	scheduleRepo repository.IAgentBackupScheduleRepository
	instRepo     repository.IAgentInstanceRepository
	adapter      rt.Adapter
	bridge       *agentruntime.Client
	sch          *scheduler.Scheduler
	backupDir    string
	instSvc      *AgentInstanceService // main.go 装配后回指：恢复时复用 Start/Stop（容器与状态同步）
}

// NewAgentBackupService 构造：注册全部 active 备份计划 + 保留期清理任务并启动调度。
func NewAgentBackupService(
	backupRepo repository.IAgentBackupRepository,
	scheduleRepo repository.IAgentBackupScheduleRepository,
	instRepo repository.IAgentInstanceRepository,
	adapter rt.Adapter,
	bridge *agentruntime.Client,
	sch *scheduler.Scheduler,
) *AgentBackupService {
	s := &AgentBackupService{
		backupRepo: backupRepo, scheduleRepo: scheduleRepo, instRepo: instRepo,
		adapter: adapter, bridge: bridge, sch: sch,
		backupDir: config.Get().AgentBackupDir(),
	}
	// 备份目录兜底创建（失败不阻断启动，执行时会再试并落 failed）。
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		log.Printf("[backup] 备份目录创建失败: %v（备份执行时会再次尝试）", err)
	}
	// 启动即按库注册全部 active 计划（进程重启不丢定时备份）。
	if plans, err := scheduleRepo.ListAll(context.Background()); err != nil {
		log.Printf("[backup] 读取备份计划失败（跳过注册）: %v", err)
	} else {
		for _, p := range plans {
			if p.IsActive != 1 {
				continue
			}
			if err := s.registerJob(p); err != nil {
				log.Printf("[backup:%s] 计划注册失败: %v", p.BackupScheduleId, err)
			}
		}
	}
	// 保留期清理：每天 03:17 清过期备份（物理文件 + 库行）。错开整点降低与其它任务撞车概率。
	_ = sch.Register("agent-backup-retention", "17 3 * * *", func(ctx context.Context) { s.CleanupExpired(ctx, 200) })
	sch.Start()
	return s
}

// SetInstanceService main.go 装配完 AgentInstanceService 后回指（恢复要停/启实例）。
func (s *AgentBackupService) SetInstanceService(v *AgentInstanceService) { s.instSvc = v }

// isLocal 仅本机 Docker 运行时支持备份/恢复。
func (s *AgentBackupService) isLocal() bool {
	return s.adapter != nil && s.adapter.Provider() == "localdocker"
}

// resolve 校验实例存在 + 运行时支持，返回容器引用。
func (s *AgentBackupService) resolve(ctx context.Context, instanceId string) (string, error) {
	if !s.isLocal() {
		return "", errors.New("仅本机 Docker 运行时支持备份/恢复")
	}
	inst, err := s.instRepo.Get(ctx, instanceId)
	if err != nil {
		return "", err
	}
	return agentruntime.ContainerRef(instanceId, inst.ContainerId), nil
}

// Create 手动备份：落 pending 行即返回，执行丢后台协程（打包大工作区可能数分钟）。
// expiresAt 可空（空=永久保留，不参与保留期清理）。
func (s *AgentBackupService) Create(ctx context.Context, e *entity.AgentBackup, expiresAt *time.Time) (string, error) {
	if _, err := s.resolve(ctx, e.InstanceId); err != nil {
		return "", err
	}
	if e.BackupName == "" {
		e.BackupName = "手动备份 " + time.Now().Format("2006-01-02 15:04:05")
	}
	e.BackupId = idgen.New() // 预生成：落盘路径含 id（repo.Add 对已置 id 不覆盖）
	e.BackupPath = filepath.Join(s.backupDir, e.BackupId+".tar.gz")
	e.Status = backupStatusPending
	e.BackupType = 0
	if expiresAt != nil {
		t := types.Time(*expiresAt)
		e.ExpiresAt = &t
	}
	id, err := s.backupRepo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	go s.run(e)
	return id, nil
}

// run 执行备份：pending→running→done/failed。用 background ctx（请求早已返回）。
func (s *AgentBackupService) run(e *entity.AgentBackup) {
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[backup:%s] panic: %v", e.BackupId, r)
			e.Status = backupStatusFailed
			_ = s.backupRepo.Update(ctx, e)
		}
	}()
	e.Status = backupStatusRunning
	if err := s.backupRepo.Update(ctx, e); err != nil {
		log.Printf("[backup:%s] 状态更新失败: %v", e.BackupId, err)
		return
	}
	ref, err := s.resolve(ctx, e.InstanceId)
	if err != nil {
		log.Printf("[backup:%s] 前置校验失败: %v", e.BackupId, err)
		e.Status = backupStatusFailed
		_ = s.backupRepo.Update(ctx, e)
		return
	}
	size, err := s.bridge.BackupWorkspace(ctx, ref, e.BackupPath)
	if err != nil {
		log.Printf("[backup:%s] 打包失败: %v", e.BackupId, err)
		_ = os.Remove(e.BackupPath) // 半截包清掉，避免恢复误用
		e.Status = backupStatusFailed
		_ = s.backupRepo.Update(ctx, e)
		return
	}
	e.BackupSizeGb = int(math.Ceil(float64(size) / 1e9)) // 字节→GB 向上取整（<1GB 记 0，字段为整型 GB）
	now := time.Now()
	nowTypes := types.Time(now)
	e.CompletedAt = &nowTypes
	e.Status = backupStatusDone
	if err := s.backupRepo.Update(ctx, e); err != nil {
		log.Printf("[backup:%s] 完成状态落库失败: %v", e.BackupId, err)
	}
	log.Printf("[backup:%s] 完成: instance=%s size=%dB path=%s", e.BackupId, e.InstanceId, size, e.BackupPath)
}

// List 备份分页。instanceId 空=全部实例（走 ListAll）。
func (s *AgentBackupService) List(ctx context.Context, instanceId string, pageNum, pageSize int) ([]*entity.AgentBackup, int64, error) {
	if instanceId == "" {
		rows, err := s.backupRepo.ListAll(ctx)
		if err != nil {
			return nil, 0, err
		}
		return rows, int64(len(rows)), nil
	}
	return s.backupRepo.List(ctx, instanceId, pageNum, pageSize)
}

// Get 备份详情。
func (s *AgentBackupService) Get(ctx context.Context, id string) (*entity.AgentBackup, error) {
	return s.backupRepo.Get(ctx, id)
}

// Delete 删除备份：物理文件与库行一起删（文件缺失按已删处理）。
func (s *AgentBackupService) Delete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		row, err := s.backupRepo.Get(ctx, id)
		if err != nil {
			return err
		}
		if row.BackupPath != "" {
			_ = os.Remove(row.BackupPath)
		}
		if err := s.backupRepo.Delete(ctx, []string{id}); err != nil {
			return err
		}
	}
	return nil
}

// Restore 从备份恢复工作区：停实例 → 解包覆盖 /aa → 按原状态启停。
// 仅覆盖 /aa 主卷（备份范围同前述取舍）；恢复期间工作区短暂不可用。
func (s *AgentBackupService) Restore(ctx context.Context, backupId, userId string) error {
	row, err := s.backupRepo.Get(ctx, backupId)
	if err != nil {
		return err
	}
	if row.Status != backupStatusDone {
		return errors.New("备份未完成，不能恢复")
	}
	if _, err := os.Stat(row.BackupPath); err != nil {
		return fmt.Errorf("备份文件缺失: %w", err)
	}
	inst, err := s.instRepo.Get(ctx, row.InstanceId)
	if err != nil {
		return err
	}
	if !s.isLocal() {
		return errors.New("仅本机 Docker 运行时支持备份/恢复")
	}
	ref := agentruntime.ContainerRef(row.InstanceId, inst.ContainerId)
	wasRunning := inst.Status == 1
	// 停实例再解包：运行中覆盖 /aa 可能读到半写文件；停机窗口 = 解包时长。
	if wasRunning && s.instSvc != nil {
		if err := s.instSvc.Stop(ctx, row.InstanceId); err != nil {
			return fmt.Errorf("恢复前停止实例失败: %w", err)
		}
	}
	if err := s.bridge.RestoreWorkspace(ctx, ref, row.BackupPath); err != nil {
		// 恢复失败也要把实例拉起来（原来在跑的话），不留死实例。
		if wasRunning && s.instSvc != nil {
			_ = s.instSvc.Start(ctx, row.InstanceId)
		}
		return fmt.Errorf("恢复失败: %w", err)
	}
	if wasRunning && s.instSvc != nil {
		if err := s.instSvc.Start(ctx, row.InstanceId); err != nil {
			return fmt.Errorf("恢复后启动实例失败: %w", err)
		}
	}
	log.Printf("[backup:%s] 恢复完成: instance=%s operator=%s", row.BackupId, row.InstanceId, userId)
	return nil
}

// ==================== 备份计划 ====================

// backupJobName 计划对应的调度任务名（Register 同名替换 / Unregister 停用）。
func backupJobName(scheduleId string) string { return "agent-backup-schedule-" + scheduleId }

// registerJob 把一条计划注册进调度器（cron 表达式合法性由 Register 校验）。
func (s *AgentBackupService) registerJob(p *entity.AgentBackupSchedule) error {
	sid, instId := p.BackupScheduleId, p.InstanceId
	return s.sch.Register(backupJobName(sid), p.CronExpression, func(ctx context.Context) {
		s.runScheduled(ctx, sid, instId)
	})
}

// runScheduled 定时触发：按计划建一条 type=schedule 的备份（expires = now + retentionDays）。
func (s *AgentBackupService) runScheduled(ctx context.Context, scheduleId, instanceId string) {
	p, err := s.scheduleRepo.Get(ctx, scheduleId)
	if err != nil {
		log.Printf("[backup:%s] 读取计划失败: %v", scheduleId, err)
		return
	}
	if p.IsActive != 1 {
		return // 已停用（Unregister 失联时兜底）
	}
	var expires *time.Time
	if p.RetentionDays > 0 {
		t := time.Now().AddDate(0, 0, p.RetentionDays)
		expires = &t
	}
	if _, err := s.Create(ctx, &entity.AgentBackup{
		InstanceId: instanceId,
		BackupName: p.ScheduleName + " " + time.Now().Format("2006-01-02 15:04"),
		BackupType: 1,
		CreatedBy:  "schedule:" + p.ScheduleName,
	}, expires); err != nil {
		log.Printf("[backup:%s] 定时备份创建失败: %v", scheduleId, err)
	}
}

// ListSchedules 计划分页（实例维度）。
func (s *AgentBackupService) ListSchedules(ctx context.Context, instanceId string, pageNum, pageSize int) ([]*entity.AgentBackupSchedule, int64, error) {
	return s.scheduleRepo.List(ctx, instanceId, pageNum, pageSize)
}

// GetSchedule 计划详情（编辑前归属校验用）。
func (s *AgentBackupService) GetSchedule(ctx context.Context, id string) (*entity.AgentBackupSchedule, error) {
	return s.scheduleRepo.Get(ctx, id)
}

// AddSchedule 新建计划：实例须存在，cron 非法直接报错；isActive=1 时注册进调度器。
func (s *AgentBackupService) AddSchedule(ctx context.Context, e *entity.AgentBackupSchedule) (string, error) {
	if _, err := s.instRepo.Get(ctx, e.InstanceId); err != nil {
		return "", fmt.Errorf("实例不存在: %w", err)
	}
	if e.RetentionDays < 0 {
		return "", errors.New("保留天数不能为负")
	}
	if e.IsActive != 1 && e.IsActive != 0 {
		return "", errors.New("isActive 取值 0/1")
	}
	// 先注册校验 cron；不启用则立即摘除（Register 同时承担非法表达式校验职责）。
	if err := s.registerJob(e); err != nil {
		return "", err
	}
	if e.IsActive != 1 {
		s.sch.Unregister(backupJobName(e.BackupScheduleId))
	}
	return s.scheduleRepo.Add(ctx, e)
}

// EditSchedule 改计划：重新注册（Register 同名替换，cron/启停变化即时生效）。
func (s *AgentBackupService) EditSchedule(ctx context.Context, e *entity.AgentBackupSchedule) error {
	old, err := s.scheduleRepo.Get(ctx, e.BackupScheduleId)
	if err != nil {
		return err
	}
	if e.CronExpression == "" {
		e.CronExpression = old.CronExpression
	}
	if e.InstanceId == "" {
		e.InstanceId = old.InstanceId
	}
	// gorm Updates 结构体零值不落列（isActive=0 / retentionDays=0 要能生效），显式 map 更新。
	if err := s.scheduleRepo.EditScheduleFields(ctx, e.BackupScheduleId, map[string]interface{}{
		"schedule_name":   e.ScheduleName,
		"cron_expression": e.CronExpression,
		"retention_days":  e.RetentionDays,
		"is_active":       e.IsActive,
		"instance_id":     e.InstanceId,
		"updated_at":      time.Now(),
	}); err != nil {
		return err
	}
	merged, err := s.scheduleRepo.Get(ctx, e.BackupScheduleId)
	if err != nil {
		return err
	}
	if merged.IsActive == 1 {
		return s.registerJob(merged)
	}
	s.sch.Unregister(backupJobName(merged.BackupScheduleId))
	return nil
}

// DeleteSchedule 删计划：先摘调度任务再删行。
func (s *AgentBackupService) DeleteSchedule(ctx context.Context, ids []string) error {
	for _, id := range ids {
		s.sch.Unregister(backupJobName(id))
	}
	return s.scheduleRepo.Delete(ctx, ids)
}

// CleanupExpired 保留期清理：物理文件删掉后删库行（limit 控制单次批量，防一次拖太久）。
func (s *AgentBackupService) CleanupExpired(ctx context.Context, limit int) {
	rows, err := s.backupRepo.ListExpired(ctx, time.Now(), limit)
	if err != nil {
		log.Printf("[backup] 读取过期备份失败: %v", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.BackupPath != "" {
			_ = os.Remove(r.BackupPath)
		}
		ids = append(ids, r.BackupId)
	}
	if err := s.backupRepo.Delete(ctx, ids); err != nil {
		log.Printf("[backup] 过期备份清理落库失败: %v", err)
		return
	}
	log.Printf("[backup] 已清理 %d 条过期备份", len(ids))
}
