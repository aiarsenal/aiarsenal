package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentScanController 安全扫描：config / job / job-item / report。
type AgentScanController struct{ svc *service.AgentScanService }

func NewAgentScanController(s *service.AgentScanService) *AgentScanController {
	return &AgentScanController{svc: s}
}

// ===== ScanConfig =====

func (h *AgentScanController) ListConfigs(c *gin.Context) {
	var req dto.AgentScanConfigSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListConfigs(c, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentScanConfigSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentScanController) GetConfig(c *gin.Context) {
	id := c.Query("scanConfigId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetConfig(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentScanController) AddConfig(c *gin.Context) {
	var req dto.AgentScanConfigAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentScanConfigReq
	if _, err := h.svc.AddConfig(c, &entity.AgentSecurityScanConfig{
		DefaultMode: r.DefaultMode, QuickAnalyzersJson: r.QuickAnalyzersJson, DeepAnalyzersJson: r.DeepAnalyzersJson,
		QuickTimeoutSeconds: r.QuickTimeoutSeconds, DeepTimeoutSeconds: r.DeepTimeoutSeconds,
		AllowFallback: r.AllowFallback, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentScanController) EditConfig(c *gin.Context) {
	var req dto.AgentScanConfigEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentScanConfigReq
	if err := h.svc.EditConfig(c, &entity.AgentSecurityScanConfig{
		ScanConfigId: req.ScanConfigId, DefaultMode: r.DefaultMode, QuickAnalyzersJson: r.QuickAnalyzersJson,
		DeepAnalyzersJson: r.DeepAnalyzersJson, QuickTimeoutSeconds: r.QuickTimeoutSeconds,
		DeepTimeoutSeconds: r.DeepTimeoutSeconds, AllowFallback: r.AllowFallback, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentScanController) DeleteConfigs(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteConfigs(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== ScanJob =====

func (h *AgentScanController) ListJobs(c *gin.Context) {
	var req dto.AgentScanJobSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListJobs(c, req.AssetType, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentScanJobSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentScanController) GetJob(c *gin.Context) {
	id := c.Query("scanJobId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetJob(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentScanController) AddJob(c *gin.Context) {
	var req dto.AgentScanJobAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentScanJobReq
	if _, err := h.svc.AddJob(c, &entity.AgentSecurityScanJob{
		AssetType: r.AssetType, ScanMode: r.ScanMode, Status: r.Status, ScopeJson: r.ScopeJson,
		TotalItems: r.TotalItems, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentScanController) EditJob(c *gin.Context) {
	var req dto.AgentScanJobEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentScanJobReq
	if err := h.svc.EditJob(c, &entity.AgentSecurityScanJob{
		ScanJobId: req.ScanJobId, AssetType: r.AssetType, ScanMode: r.ScanMode, Status: r.Status,
		ScopeJson: r.ScopeJson, TotalItems: r.TotalItems,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentScanController) DeleteJobs(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteJobs(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== JobItem =====

func (h *AgentScanController) ListJobItems(c *gin.Context) {
	var req dto.AgentScanJobItemSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListJobItems(c, req.JobId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentScanJobItemSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentScanController) GetJobItem(c *gin.Context) {
	id := c.Query("jobItemId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetJobItem(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

// ===== Report（只读） =====

func (h *AgentScanController) ListReports(c *gin.Context) {
	var req dto.AgentScanReportSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListReports(c, req.JobId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentScanReportSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentScanController) GetReport(c *gin.Context) {
	id := c.Query("scanReportId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetReport(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}
