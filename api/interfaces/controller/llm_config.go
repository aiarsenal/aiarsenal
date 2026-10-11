package controller

import (
	"context"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// LLM 配置控制器：凭据 / 模型 / 预算。

type LLMConfigController struct {
	svc              *service.LLMConfigService
	agentInstanceSvc *service.AgentInstanceService // 模型可用性下降时异步通知受影响实例刷新本地清单
}

func NewLLMConfigController(s *service.LLMConfigService, ais *service.AgentInstanceService) *LLMConfigController {
	return &LLMConfigController{svc: s, agentInstanceSvc: ais}
}

// ===== Credential =====

func (h *LLMConfigController) ListCredentials(c *gin.Context) {
	var req dto.LLMCredentialSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListCredentials(c, req.Name, req.ProviderType, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMCredentialSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMConfigController) GetCredential(c *gin.Context) {
	id := c.Query("credentialId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetCredential(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMConfigController) AddCredential(c *gin.Context) {
	var req dto.LLMCredentialAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMCredentialReq
	if _, err := h.svc.AddCredential(c, &entity.LLMCredential{
		CredentialName: r.CredentialName, ProviderType: r.ProviderType, BaseURL: r.BaseURL,
		CredentialInfo: r.CredentialInfo, Status: r.Status, CreatedBy: middleware.GetUserId(c),
	}, r.APIKey); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) EditCredential(c *gin.Context) {
	var req dto.LLMCredentialEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMCredentialReq
	if err := h.svc.EditCredential(c, &entity.LLMCredential{
		CredentialId: req.CredentialId, CredentialName: r.CredentialName, ProviderType: r.ProviderType,
		BaseURL: r.BaseURL, CredentialInfo: r.CredentialInfo, Status: r.Status, UpdatedBy: middleware.GetUserId(c),
	}, r.APIKey); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) DeleteCredentials(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteCredentials(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// DiscoverModels 按凭据调上游 /v1/models，批量导入支持模型为部署。
func (h *LLMConfigController) DiscoverModels(c *gin.Context) {
	var req dto.LLMDiscoverModelsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	res, err := h.svc.DiscoverModels(c, req.CredentialId, req.ModelGroup, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// ===== Model =====

func (h *LLMConfigController) ListModels(c *gin.Context) {
	var req dto.LLMModelSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListModels(c, req.Name, req.Provider, req.Group, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMModelSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMConfigController) AllModels(c *gin.Context) {
	list, err := h.svc.AllModels(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *LLMConfigController) GetModel(c *gin.Context) {
	id := c.Query("modelId")
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

func (h *LLMConfigController) AddModel(c *gin.Context) {
	var req dto.LLMModelAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMModelReq
	if _, err := h.svc.AddModel(c, modelFromReq(r, "", middleware.GetUserId(c), "")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) EditModel(c *gin.Context) {
	var req dto.LLMModelEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMModelReq
	// 取旧值判断模型可用性是否下降（曾在线→禁用/封禁），下降才需通知受影响实例删掉这条
	old, _ := h.svc.GetModel(c, req.ModelId)
	issuedBy := middleware.GetUserId(c) // 同步取出，异步 goroutine 不再碰 gin.Context
	if err := h.svc.EditModel(c, modelFromReq(r, req.ModelId, "", issuedBy)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
	notifyModelUnavailableIfDropped(h, old, r, issuedBy)
	// 模型编辑后刷新受影响实例的运行配置（max/model_name/provider 等都可能影响下发内容）。
	// 不限定字段变化：编辑即异步重推（ak 不变 best-effort），保证 openclaw.json 始终反映最新 DB，
	// 也兜住「DB 已改但 openclaw.json 还是创建实例时旧值」的历史不一致。
	if h.agentInstanceSvc != nil && r.ModelGroup != "" && r.ModelName != "" {
		ais := h.agentInstanceSvc
		group, name := r.ModelGroup, r.ModelName
		go func() {
			ais.NotifyModelConfigRefresh(context.Background(), group, name, issuedBy)
		}()
	}
}

func (h *LLMConfigController) DeleteModels(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 删前取出曾在线模型的 group/name，用于事后通知受影响实例
	var toNotify [][2]string
	for _, id := range req.Ids {
		m, _ := h.svc.GetModel(c, id)
		if m != nil && m.Status == 1 && m.Blocked == 0 && m.ModelGroup != "" && m.ModelName != "" {
			toNotify = append(toNotify, [2]string{m.ModelGroup, m.ModelName})
		}
	}
	if err := h.svc.DeleteModels(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
	if h.agentInstanceSvc != nil && len(toNotify) > 0 {
		issuedBy := middleware.GetUserId(c)
		ais := h.agentInstanceSvc
		go func() {
			defer func() { _ = recover() }()
			for _, mi := range toNotify {
				ais.NotifyModelUnavailable(context.Background(), mi[0], mi[1], issuedBy)
			}
		}()
	}
}

// notifyModelUnavailableIfDropped：模型曾在线但本次编辑后不再可用（status→0 或 blocked→1）时，
// 异步通知 allowed_models 含该模型的 key 所绑定的实例从其运行配置删掉这条（ak 不变）。
func notifyModelUnavailableIfDropped(h *LLMConfigController, old *entity.LLMModel, r *dto.LLMModelReq, issuedBy string) {
	if h.agentInstanceSvc == nil || old == nil || r == nil {
		return
	}
	if old.ModelGroup == "" || old.ModelName == "" {
		return
	}
	wasAvailable := old.Status == 1 && old.Blocked == 0
	nowAvailable := r.Status == 1 && r.Blocked == 0
	if !wasAvailable || nowAvailable {
		return // 仅可用性下降才通知（重新启用走 add 路径，本方案不含）
	}
	group, name := old.ModelGroup, old.ModelName
	go func() {
		defer func() { _ = recover() }()
		h.agentInstanceSvc.NotifyModelUnavailable(context.Background(), group, name, issuedBy)
	}()
}

func modelFromReq(r *dto.LLMModelReq, modelId, createdBy, updatedBy string) *entity.LLMModel {
	return &entity.LLMModel{
		ModelId: modelId, ModelName: r.ModelName, ModelGroup: r.ModelGroup, Provider: r.Provider,
		ProviderModel: r.ProviderModel, CredentialId: r.CredentialId, ExtraParams: r.ExtraParams,
		ModelInfo: r.ModelInfo, InputPricePerMillion: r.InputPricePerMillion, OutputPricePerMillion: r.OutputPricePerMillion,
		CacheReadPricePerMillion: r.CacheReadPricePerMillion, CacheCreationPricePerMillion: r.CacheCreationPricePerMillion,
		MaxInputTokens: r.MaxInputTokens, MaxOutputTokens: r.MaxOutputTokens, TpmLimit: r.TpmLimit, RpmLimit: r.RpmLimit,
		Weight: r.Weight, Blocked: r.Blocked, Status: r.Status, CreatedBy: createdBy, UpdatedBy: updatedBy,
	}
}

// ===== Budget =====

func (h *LLMConfigController) ListBudgets(c *gin.Context) {
	var req dto.LLMBudgetSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListBudgets(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMBudgetSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMConfigController) GetBudget(c *gin.Context) {
	id := c.Query("budgetId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetBudget(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMConfigController) AddBudget(c *gin.Context) {
	var req dto.LLMBudgetAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMBudgetReq
	if _, err := h.svc.AddBudget(c, budgetFromReq(r, "", middleware.GetUserId(c), "")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) EditBudget(c *gin.Context) {
	var req dto.LLMBudgetEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMBudgetReq
	if err := h.svc.EditBudget(c, budgetFromReq(r, req.BudgetId, "", middleware.GetUserId(c))); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) DeleteBudgets(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteBudgets(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// MyBudgets 我的预算：当前登录用户命中的圈人预算 + 本周期消费/剩余 + 本月消费汇总（个人视角，只读）。
func (h *LLMConfigController) MyBudgets(c *gin.Context) {
	res, err := h.svc.MyBudgets(c, middleware.GetIdentity(c))
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, res)
}

// MySpendHistory 历史周期消费：本人日/周/月分桶消费曲线（【我的预算】页下半区）。
func (h *LLMConfigController) MySpendHistory(c *gin.Context) {
	res, err := h.svc.MySpendHistory(c, middleware.GetUserId(c), c.Query("period"))
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, res)
}

func budgetFromReq(r *dto.LLMBudgetReq, budgetId, createdBy, updatedBy string) *entity.LLMBudget {
	e := &entity.LLMBudget{
		BudgetId: budgetId, BudgetName: r.BudgetName, ModelMaxBudget: r.ModelMaxBudget,
		BudgetDuration: r.BudgetDuration, AllowedModels: r.AllowedModels,
		GrantRules: r.GrantRules,
		CreatedBy: createdBy, UpdatedBy: updatedBy,
	}
	if r.MaxBudget > 0 {
		v := r.MaxBudget
		e.MaxBudget = &v
	}
	if r.SoftBudget > 0 {
		v := r.SoftBudget
		e.SoftBudget = &v
	}
	if r.MaxParallelRequests > 0 {
		v := r.MaxParallelRequests
		e.MaxParallelRequests = &v
	}
	if r.TpmLimit > 0 {
		v := r.TpmLimit
		e.TpmLimit = &v
	}
	if r.RpmLimit > 0 {
		v := r.RpmLimit
		e.RpmLimit = &v
	}
	return e
}

// ===== ModelGroup（分组字典）=====

func (h *LLMConfigController) ListGroups(c *gin.Context) {
	var req dto.LLMModelGroupSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListGroups(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMModelGroupSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMConfigController) GetGroup(c *gin.Context) {
	id := c.Query("groupId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetGroup(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMConfigController) AddGroup(c *gin.Context) {
	var req dto.LLMModelGroupAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMModelGroupReq
	if _, err := h.svc.AddGroup(c, &entity.LLMModelGroup{
		GroupName: r.GroupName, Description: r.Description, Status: r.Status, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) EditGroup(c *gin.Context) {
	var req dto.LLMModelGroupEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMModelGroupReq
	if err := h.svc.EditGroup(c, &entity.LLMModelGroup{
		GroupId: req.GroupId, GroupName: r.GroupName, Description: r.Description, Status: r.Status, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMConfigController) DeleteGroups(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteGroups(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
