package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// agent_rollout.go — 运行时滚动发布（管理员态，挂 agent 管理路由）。
// 请求/响应结构体内联在本文件（dto 目录不归本任务改）。

// AgentRolloutController 滚动发布控制器。
type AgentRolloutController struct{ svc *service.AgentRolloutService }

// NewAgentRolloutController 构造。
func NewAgentRolloutController(svc *service.AgentRolloutService) *AgentRolloutController {
	return &AgentRolloutController{svc: svc}
}

// RolloutCreateReq 创建发布单请求体。
type RolloutCreateReq struct {
	RuntimeType    string   `json:"runtimeType"`    // 缺省取 adapter provider（localdocker）
	TargetImageRef string   `json:"targetImageRef" binding:"required"`
	BatchSize      int      `json:"batchSize"`      // <1 默认 1
	MaxUnavailable int      `json:"maxUnavailable"` // 记录字段（restart-only 批次推进按 batchSize）
	InstanceIds    []string `json:"instanceIds" binding:"required"`
}

// Create 创建发布单（POST /agent/runtime/rollout/create）。
func (h *AgentRolloutController) Create(c *gin.Context) {
	var req RolloutCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id, err := h.svc.Create(c, &entity.AgentRuntimeRollout{
		RuntimeType:    req.RuntimeType,
		TargetImageRef: req.TargetImageRef,
		BatchSize:      req.BatchSize,
		MaxUnavailable: req.MaxUnavailable,
		StartedBy:      middleware.GetUserId(c),
	}, req.InstanceIds)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"runtimeRolloutId": id})
}

// List 发布单分页（GET /agent/runtime/rollout/list?status=&pageNum=&pageSize=；status 空=全部）。
func (h *AgentRolloutController) List(c *gin.Context) {
	status := -1
	if s := c.Query("status"); s != "" {
		if v := queryInt(c, "status", -1); v >= 0 {
			status = v
		}
	}
	list, total, err := h.svc.List(c, status, queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": total})
}

// Progress 进度查询（GET /agent/runtime/rollout/:id/progress；入口惰性推进一次）。
// 返回发布单 + pod 批次（pod_name 即 aiarsenal-<instanceId>，前端可直接对到实例）。
func (h *AgentRolloutController) Progress(c *gin.Context) {
	ro, pods, err := h.svc.Progress(c, c.Param("id"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"rollout": ro, "pods": pods})
}

// Pause 暂停（POST /agent/runtime/rollout/:id/pause）。
func (h *AgentRolloutController) Pause(c *gin.Context) {
	if err := h.svc.Pause(c, c.Param("id")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Resume 恢复（POST /agent/runtime/rollout/:id/resume）。
func (h *AgentRolloutController) Resume(c *gin.Context) {
	if err := h.svc.Resume(c, c.Param("id")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Cancel 取消（POST /agent/runtime/rollout/:id/cancel）：未完成 pod 标 failed(reason=cancelled)。
func (h *AgentRolloutController) Cancel(c *gin.Context) {
	if err := h.svc.Cancel(c, c.Param("id")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
