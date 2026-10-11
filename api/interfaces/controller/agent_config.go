package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentConfigController 配置中心：resource / bundle / bundle-item / bundle-skill。
type AgentConfigController struct{ svc *service.AgentConfigService }

func NewAgentConfigController(s *service.AgentConfigService) *AgentConfigController {
	return &AgentConfigController{svc: s}
}

// ===== ConfigResource =====

func (h *AgentConfigController) ListResources(c *gin.Context) {
	var req dto.AgentConfigResourceSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListResources(c, req.Name, req.ResourceType, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentConfigResourceSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentConfigController) GetResource(c *gin.Context) {
	id := c.Query("configResourceId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetResource(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentConfigController) AddResource(c *gin.Context) {
	var req dto.AgentConfigResourceAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigResourceReq
	if _, err := h.svc.AddResource(c, &entity.AgentConfigResource{
		ResourceType: r.ResourceType, ResourceKey: r.ResourceKey, Name: r.Name, Description: r.Description,
		Enabled: r.Enabled, Version: r.Version, TagsJson: r.TagsJson, ContentJson: r.ContentJson,
		CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) EditResource(c *gin.Context) {
	var req dto.AgentConfigResourceEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigResourceReq
	if err := h.svc.EditResource(c, &entity.AgentConfigResource{
		ConfigResourceId: req.ConfigResourceId, ResourceType: r.ResourceType, ResourceKey: r.ResourceKey,
		Name: r.Name, Description: r.Description, Enabled: r.Enabled, Version: r.Version,
		TagsJson: r.TagsJson, ContentJson: r.ContentJson, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) DeleteResources(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteResources(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== ConfigBundle =====

func (h *AgentConfigController) ListBundles(c *gin.Context) {
	var req dto.AgentConfigBundleSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListBundles(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentConfigBundleSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentConfigController) GetBundle(c *gin.Context) {
	id := c.Query("configBundleId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetBundle(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentConfigController) AddBundle(c *gin.Context) {
	var req dto.AgentConfigBundleAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleReq
	if _, err := h.svc.AddBundle(c, &entity.AgentConfigBundle{
		Name: r.Name, Description: r.Description, Enabled: r.Enabled, Version: r.Version,
		CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) EditBundle(c *gin.Context) {
	var req dto.AgentConfigBundleEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleReq
	if err := h.svc.EditBundle(c, &entity.AgentConfigBundle{
		ConfigBundleId: req.ConfigBundleId, Name: r.Name, Description: r.Description,
		Enabled: r.Enabled, Version: r.Version, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) DeleteBundles(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteBundles(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== BundleItem =====

func (h *AgentConfigController) ListBundleItems(c *gin.Context) {
	var req dto.AgentConfigBundleItemSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListBundleItems(c, req.BundleId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentConfigBundleItemSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentConfigController) GetBundleItem(c *gin.Context) {
	id := c.Query("bundleItemId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetBundleItem(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentConfigController) AddBundleItem(c *gin.Context) {
	var req dto.AgentConfigBundleItemAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleItemReq
	if _, err := h.svc.AddBundleItem(c, &entity.AgentConfigBundleItem{
		BundleId: r.BundleId, ResourceId: r.ResourceId, SortOrder: r.SortOrder,
		Required: r.Required, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) EditBundleItem(c *gin.Context) {
	var req dto.AgentConfigBundleItemEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleItemReq
	if err := h.svc.EditBundleItem(c, &entity.AgentConfigBundleItem{
		BundleItemId: req.BundleItemId, BundleId: r.BundleId, ResourceId: r.ResourceId,
		SortOrder: r.SortOrder, Required: r.Required,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) DeleteBundleItems(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteBundleItems(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== BundleSkill =====

func (h *AgentConfigController) ListBundleSkills(c *gin.Context) {
	var req dto.AgentConfigBundleSkillSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListBundleSkills(c, req.BundleId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentConfigBundleSkillSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentConfigController) GetBundleSkill(c *gin.Context) {
	id := c.Query("bundleSkillId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetBundleSkill(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentConfigController) AddBundleSkill(c *gin.Context) {
	var req dto.AgentConfigBundleSkillAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleSkillReq
	if _, err := h.svc.AddBundleSkill(c, &entity.AgentConfigBundleSkill{
		BundleId: r.BundleId, SkillId: r.SkillId, SortOrder: r.SortOrder,
		Required: r.Required, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) EditBundleSkill(c *gin.Context) {
	var req dto.AgentConfigBundleSkillEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentConfigBundleSkillReq
	if err := h.svc.EditBundleSkill(c, &entity.AgentConfigBundleSkill{
		BundleSkillId: req.BundleSkillId, BundleId: r.BundleId, SkillId: r.SkillId,
		SortOrder: r.SortOrder, Required: r.Required,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentConfigController) DeleteBundleSkills(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteBundleSkills(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
