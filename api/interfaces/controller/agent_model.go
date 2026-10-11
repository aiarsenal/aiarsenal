package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentModelController AI Gateway 治理配置：model（api_key 加密）/ risk-rule。
type AgentModelController struct{ svc *service.AgentModelService }

func NewAgentModelController(s *service.AgentModelService) *AgentModelController {
	return &AgentModelController{svc: s}
}

// ===== Model =====

func (h *AgentModelController) ListModels(c *gin.Context) {
	var req dto.AgentModelSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListModels(c, req.Name, req.ProviderType, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentModelSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentModelController) GetModel(c *gin.Context) {
	id := c.Query("agentModelId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetModel(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentModelController) AddModel(c *gin.Context) {
	var req dto.AgentModelAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentModelReq
	if _, err := h.svc.AddModel(c, &entity.AgentModel{
		DisplayName: r.DisplayName, Description: r.Description, ProviderType: r.ProviderType,
		ProtocolType: r.ProtocolType, BaseUrl: r.BaseUrl, ProviderModelName: r.ProviderModelName,
		ApiKeySecretRef: r.ApiKeySecretRef, IsSecure: r.IsSecure, IsActive: r.IsActive,
		InputPrice: r.InputPrice, OutputPrice: r.OutputPrice, Currency: r.Currency,
		CreatedBy: middleware.GetUserId(c),
	}, r.ApiKey); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentModelController) EditModel(c *gin.Context) {
	var req dto.AgentModelEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentModelReq
	if err := h.svc.EditModel(c, &entity.AgentModel{
		AgentModelId: req.AgentModelId, DisplayName: r.DisplayName, Description: r.Description,
		ProviderType: r.ProviderType, ProtocolType: r.ProtocolType, BaseUrl: r.BaseUrl,
		ProviderModelName: r.ProviderModelName, ApiKeySecretRef: r.ApiKeySecretRef, IsSecure: r.IsSecure,
		IsActive: r.IsActive, InputPrice: r.InputPrice, OutputPrice: r.OutputPrice, Currency: r.Currency,
		UpdatedBy: middleware.GetUserId(c),
	}, r.ApiKey); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentModelController) DeleteModels(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteModels(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== RiskRule =====

func (h *AgentModelController) ListRules(c *gin.Context) {
	var req dto.AgentRiskRuleSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListRules(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentRiskRuleSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentModelController) GetRule(c *gin.Context) {
	id := c.Query("riskRuleId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetRule(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentModelController) AddRule(c *gin.Context) {
	var req dto.AgentRiskRuleAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentRiskRuleReq
	if _, err := h.svc.AddRule(c, &entity.AgentRiskRule{
		RuleId: r.RuleId, DisplayName: r.DisplayName, Description: r.Description, Pattern: r.Pattern,
		Severity: r.Severity, Action: r.Action, IsEnabled: r.IsEnabled, SortOrder: r.SortOrder,
		CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentModelController) EditRule(c *gin.Context) {
	var req dto.AgentRiskRuleEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentRiskRuleReq
	if err := h.svc.EditRule(c, &entity.AgentRiskRule{
		RiskRuleId: req.RiskRuleId, RuleId: r.RuleId, DisplayName: r.DisplayName, Description: r.Description,
		Pattern: r.Pattern, Severity: r.Severity, Action: r.Action, IsEnabled: r.IsEnabled,
		SortOrder: r.SortOrder, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentModelController) DeleteRules(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteRules(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
