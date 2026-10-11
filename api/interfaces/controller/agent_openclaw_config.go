package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// agent_openclaw_config.go — OpenClaw 配置页 API（v2 占位菜单落地）。
// 契约与前端 agent/openclaw.ts 对齐：GET /configs 列表、GET /configs/:instanceId
// 详情（含 content 全文）、PUT /configs/:instanceId 保存下发（body {content}）、
// POST /configs/validate 校验（不入库）。
// 读=最近一次 config_push 主档反向读回；写=走既有 config_push 下发通道。

// AgentOpenclawConfigController OpenClaw 配置页控制器。
type AgentOpenclawConfigController struct {
	svc     *service.AgentOpenclawConfigService
	instSvc *service.AgentInstanceService // owner 校验
}

// NewAgentOpenclawConfigController 构造。
func NewAgentOpenclawConfigController(svc *service.AgentOpenclawConfigService, instSvc *service.AgentInstanceService) *AgentOpenclawConfigController {
	return &AgentOpenclawConfigController{svc: svc, instSvc: instSvc}
}

// checkOwner 实例存在 + 归属校验。
func (h *AgentOpenclawConfigController) checkOwner(c *gin.Context, id string) bool {
	uid := middleware.GetUserId(c)
	ins, err := h.instSvc.Get(c, id)
	if err != nil || ins == nil {
		middleware.Fail(c, "未找到实例")
		return false
	}
	if ins.UserId != uid {
		middleware.Fail(c, "无权访问该实例")
		return false
	}
	return true
}

// List 配置列表（GET /agent/openclaw/configs）。全部 openclaw 实例 + 主档元信息
// （content 不回，省流量——详情接口才带全文）。
func (h *AgentOpenclawConfigController) List(c *gin.Context) {
	list, err := h.svc.List(c)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, it := range list {
		items = append(items, openclawConfigView(it, false))
	}
	middleware.Success(c, gin.H{"list": items})
}

// Get 配置详情（GET /agent/openclaw/configs/:instanceId，含 content 全文）。
func (h *AgentOpenclawConfigController) Get(c *gin.Context) {
	id := c.Param("instanceId")
	if !h.checkOwner(c, id) {
		return
	}
	it, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, openclawConfigView(it, true))
}

// Save 保存并下发（PUT /agent/openclaw/configs/:instanceId，body {content}）。
// source 记 dispatch:config-page，config_change 列表据此区分配置页来源；
// 默认热重载（配置页语义=改完即生效）。
func (h *AgentOpenclawConfigController) Save(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id := c.Param("instanceId")
	if !h.checkOwner(c, id) {
		return
	}
	hash, err := h.svc.Save(c, id, req.Content, middleware.GetUserId(c), true)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"instanceId": id, "configHash": hash})
}

// Validate 校验配置文本（POST /agent/openclaw/configs/validate，body {content}；
// JSON 语法 + 平台锚点段检查，不入库不下发）。
func (h *AgentOpenclawConfigController) Validate(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	errs, err := h.svc.Validate(req.Content)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"valid": len(errs) == 0, "errors": errs})
}

// openclawConfigView 条目 → 前端行视图；withContent=false 时省略全文（列表行）。
func openclawConfigView(it *service.OpenclawConfigItem, withContent bool) gin.H {
	v := gin.H{
		"instanceId": it.InstanceId,
		"name":       it.Name,
		"agent":      it.Agent,
		"version":    it.Version,
		"status":     it.Status,
		"configHash": it.ConfigHash,
		"configSize": it.ConfigSize,
		"syncedAt":   it.SyncedAt,
		"updatedAt":  it.UpdatedAt,
	}
	if withContent {
		v["content"] = it.Content
	}
	return v
}
