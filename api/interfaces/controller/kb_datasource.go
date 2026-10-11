package controller

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// KbDatasourceController KB 数据源连接器（web 抓取/数据库/API 拉取/对象存储/外部知识库）。
// 目录可写者可管：创建/改配走目录写权限校验；触发与运行记录同口径。
// 契约与前端 kb/datasource.ts 对齐：datasourceId/type/cron/enabled(1|0)/config(jsonb 对象)。
type KbDatasourceController struct {
	svc *service.KbDatasourceService
}

func NewKbDatasourceController(s *service.KbDatasourceService) *KbDatasourceController {
	return &KbDatasourceController{svc: s}
}

// kbDsSaveReq 创建/编辑共用请求体。config 为整包 JSON 对象（各类型差异化字段），
// 后端校验后 jsonb 原样存储；enabled 用 1/0（前端契约）。
type kbDsSaveReq struct {
	DatasourceId string          `json:"datasourceId"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	DirId        string          `json:"dirId"`
	Description  string          `json:"description"`
	Enabled      *int            `json:"enabled"`
	Cron         string          `json:"cron"`
	Config       json.RawMessage `json:"config"`
}

// List 数据源列表（分页）。
// GET /api/v1/kb/datasource/list?keyword=&type=&pageNum=&pageSize=
func (h *KbDatasourceController) List(c *gin.Context) {
	pageNum, pageSize := pageOf(c)
	list, total, err := h.svc.List(c, c.Query("keyword"), c.Query("type"), pageNum, pageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, d := range list {
		items = append(items, dsView(d))
	}
	middleware.Success(c, gin.H{"list": items, "total": total, "currentPage": pageNum})
}

// Get 数据源详情。
// GET /api/v1/kb/datasource/get?datasourceId=
func (h *KbDatasourceController) Get(c *gin.Context) {
	d, err := h.svc.Get(c, c.Query("datasourceId"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dsView(d))
}

// Add 新建数据源（校验目录写权限/连接器类型/config JSON/cron）。
// POST /api/v1/kb/datasource/add
func (h *KbDatasourceController) Add(c *gin.Context) {
	var req kbDsSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	d, err := h.svc.Create(c, req.DirId, req.Type, req.Name, req.Description, string(req.Config), req.Cron,
		req.Enabled == nil || *req.Enabled == 1, middleware.GetUserId(c), middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"datasourceId": d.DsId})
}

// Edit 修改数据源（config 整包覆盖；改动即时对齐 cron 任务）。
// PUT /api/v1/kb/datasource/edit
func (h *KbDatasourceController) Edit(c *gin.Context) {
	var req kbDsSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	var enabled *bool
	if req.Enabled != nil {
		v := *req.Enabled == 1
		enabled = &v
	}
	if _, err := h.svc.Update(c, req.DatasourceId, req.DirId, req.Type, req.Name, req.Description,
		string(req.Config), req.Cron, enabled, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Delete 删除数据源（批量 ids；已入库文档保留）。
// DELETE /api/v1/kb/datasource/delete  body: {ids: []}
func (h *KbDatasourceController) Delete(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	for _, id := range req.Ids {
		if err := h.svc.Delete(c, id, middleware.GetIdentityPtr(c)); err != nil {
			middleware.Fail(c, err.Error())
			return
		}
	}
	middleware.Success(c, struct{}{})
}

// Trigger 手动触发一次同步（异步执行；进度查 /:id/runs）。
// POST /api/v1/kb/datasource/:datasourceId/trigger
func (h *KbDatasourceController) Trigger(c *gin.Context) {
	run, err := h.svc.TriggerRun(c, c.Param("datasourceId"), middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"runId": run.RunId})
}

// Runs 同步运行记录（倒序分页）。
// GET /api/v1/kb/datasource/:datasourceId/runs?pageNum=&pageSize=
func (h *KbDatasourceController) Runs(c *gin.Context) {
	pageNum, pageSize := pageOf(c)
	list, total, err := h.svc.ListRuns(c, c.Param("datasourceId"), pageNum, pageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, r := range list {
		items = append(items, runView(r))
	}
	middleware.Success(c, gin.H{"list": items, "total": total, "currentPage": pageNum})
}

// CancelRun 取消运行中的 run（当前引擎不支持中断，单次同步 30 分钟硬超时兜底）。
// POST /api/v1/kb/datasource/runs/cancel  body: {runId}
func (h *KbDatasourceController) CancelRun(c *gin.Context) {
	middleware.Fail(c, "暂不支持取消运行：单次同步上限 30 分钟，超时自动结束")
}

// ---- 视图组装（实体 → 前端契约字段）----

// dsView 数据源行视图：enabled 转 1/0、config 还原 JSON 对象、cron 推算 nextRunAt。
func dsView(d *entity.KbDatasource) gin.H {
	enabled := 0
	if d.IsEnabled {
		enabled = 1
	}
	cfg := json.RawMessage(d.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	v := gin.H{
		"datasourceId":  d.DsId,
		"name":          d.Name,
		"type":          d.DsType,
		"dirId":         d.DirId,
		"description":   d.Description,
		"enabled":       enabled,
		"cron":          d.Schedule,
		"config":        cfg,
		"lastRunAt":     d.LastRunAt,
		"lastRunStatus": d.LastStatus,
		"createdBy":     d.CreatedBy,
		"createdAt":     d.CreatedAt,
		"updatedAt":     d.UpdatedAt,
	}
	// 下次执行时间：启用且带合法 cron 才推算（富化字段，推算失败静默省略）
	if d.IsEnabled && d.Schedule != "" {
		if sch, err := cron.ParseStandard(d.Schedule); err == nil {
			v["nextRunAt"] = sch.Next(time.Now()).Format("2006-01-02 15:04:05")
		}
	}
	return v
}

// runView 运行记录视图：trigger 取触发来源、durationMs 由起止时间算、stats 摊平成契约键
// （fetched=pages+rows+objects 等拉取计数合并；docsCreated/docsUpdated=入库 created/updated）。
func runView(r *entity.KbDatasourceRun) gin.H {
	durationMs := 0
	if r.StartedAt != nil && r.FinishedAt != nil {
		durationMs = int(time.Time(*r.FinishedAt).Sub(time.Time(*r.StartedAt)).Milliseconds())
		if durationMs < 0 {
			durationMs = 0
		}
	}
	stats := gin.H{}
	var parsed struct {
		Counters map[string]float64 `json:"counters"`
	}
	if json.Unmarshal([]byte(r.Stats), &parsed) == nil && parsed.Counters != nil {
		fetched := 0.0
		for _, k := range []string{"pages", "rows", "objects", "items", "messages"} {
			fetched += parsed.Counters[k]
		}
		stats = gin.H{
			"fetched":     int(fetched),
			"docsCreated": int(parsed.Counters["created"]),
			"docsUpdated": int(parsed.Counters["updated"]),
			"skipped":     int(parsed.Counters["skipped"]),
		}
	}
	return gin.H{
		"runId":        r.RunId,
		"datasourceId": r.DatasourceId,
		"status":       r.Status,
		"trigger":      r.TriggerType,
		"startedAt":    r.StartedAt,
		"finishedAt":   r.FinishedAt,
		"durationMs":   durationMs,
		"stats":        stats,
		"error":        r.Error,
		"createdAt":    r.StartedAt,
	}
}

// pageOf 解析分页参数（缺省 1/10，上限 100）。
func pageOf(c *gin.Context) (pageNum, pageSize int) {
	pageNum, _ = strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return
}
