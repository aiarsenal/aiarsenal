package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// LLM 监控控制器：用量 / 日聚合 / 错误 / 审计 / 健康检查 / 提示词（前端只读）。

type LLMMonitorController struct{ svc *service.LLMMonitorService }

func NewLLMMonitorController(s *service.LLMMonitorService) *LLMMonitorController {
	return &LLMMonitorController{svc: s}
}

// ===== SpendLog =====
func (h *LLMMonitorController) ListSpendLogs(c *gin.Context) {
	var req dto.LLMSpendLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListSpendLogs(c, req.UserId, req.TeamId, req.Model, req.ApiKey, req.DateRange, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMSpendLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMMonitorController) GetSpendLog(c *gin.Context) {
	rid := c.Query("requestId")
	if rid == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetSpendLog(c, rid)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

// ===== SpendAggregate（花费归集报表） =====
func (h *LLMMonitorController) AggregateSpend(c *gin.Context) {
	var req dto.LLMSpendAggregateReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.Dimension == "" {
		req.Dimension = "date"
	}
	list, totals, err := h.svc.AggregateSpend(c, req.Dimension, req.DateRange)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.LLMSpendAggregateRes{List: list, Totals: totals})
}

// ===== DailySpend =====
func (h *LLMMonitorController) ListDailySpend(c *gin.Context) {
	var req dto.LLMDailySpendSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListDailySpend(c, req.ScopeType, req.ScopeId, req.DateRange, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMDailySpendSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ===== ErrorLog =====
func (h *LLMMonitorController) ListErrorLogs(c *gin.Context) {
	var req dto.LLMErrorLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListErrorLogs(c, req.Model, req.ApiKey, req.DateRange, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMErrorLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ===== AuditLog =====
func (h *LLMMonitorController) ListAuditLogs(c *gin.Context) {
	var req dto.LLMAuditLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAuditLogs(c, req.TableName, req.Action, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMAuditLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ===== HealthCheck =====
func (h *LLMMonitorController) ListHealthChecks(c *gin.Context) {
	var req dto.LLMHealthCheckSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListHealthChecks(c, req.ModelName, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMHealthCheckSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ===== Prompt =====
func (h *LLMMonitorController) ListPrompts(c *gin.Context) {
	var req dto.LLMPromptSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListPrompts(c, req.Name, req.Environment, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMPromptSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMMonitorController) GetPromptVersions(c *gin.Context) {
	pid := c.Query("promptId")
	if pid == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.GetPromptVersions(c, pid)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *LLMMonitorController) AddPrompt(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Version     int    `json:"version"`
		Environment string `json:"environment"`
		ExtraParams string `json:"extraParams"`
		PromptInfo  string `json:"promptInfo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.Version == 0 {
		req.Version = 1
	}
	if req.Environment == "" {
		req.Environment = "development"
	}
	if _, err := h.svc.AddPrompt(c, &entity.LLMPrompt{
		Name: req.Name, Version: req.Version, Environment: req.Environment,
		ExtraParams: req.ExtraParams, PromptInfo: req.PromptInfo, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
