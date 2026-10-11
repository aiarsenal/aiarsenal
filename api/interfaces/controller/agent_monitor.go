package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentMonitorController 治理监控（只读）：invocation / audit-event /
// cost-record / risk-hit / audit-log。audit-log 支持清理。
type AgentMonitorController struct{ svc *service.AgentMonitorService }

func NewAgentMonitorController(s *service.AgentMonitorService) *AgentMonitorController {
	return &AgentMonitorController{svc: s}
}

func (h *AgentMonitorController) ListInvocations(c *gin.Context) {
	var req dto.AgentInvocationSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListInvocations(c, req.UserId, req.ModelId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentInvocationSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentMonitorController) GetInvocation(c *gin.Context) {
	id := c.Query("invocationId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetInvocation(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentMonitorController) ListAuditEvents(c *gin.Context) {
	var req dto.AgentAuditEventSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAuditEvents(c, req.UserId, req.EventType, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentAuditEventSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentMonitorController) GetAuditEvent(c *gin.Context) {
	id := c.Query("auditEventId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetAuditEvent(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentMonitorController) ListCostRecords(c *gin.Context) {
	var req dto.AgentCostRecordSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListCostRecords(c, req.UserId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentCostRecordSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentMonitorController) GetCostRecord(c *gin.Context) {
	id := c.Query("costRecordId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetCostRecord(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentMonitorController) ListRiskHits(c *gin.Context) {
	var req dto.AgentRiskHitSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListRiskHits(c, req.RuleId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentRiskHitSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentMonitorController) GetRiskHit(c *gin.Context) {
	id := c.Query("riskHitId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetRiskHit(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentMonitorController) ListAuditLogs(c *gin.Context) {
	var req dto.AgentAuditLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAuditLogs(c, req.UserId, req.Action, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentAuditLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentMonitorController) GetAuditLog(c *gin.Context) {
	id := c.Query("auditLogId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetAuditLog(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentMonitorController) DeleteAuditLogs(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteAuditLogs(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
