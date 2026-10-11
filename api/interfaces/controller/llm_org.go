package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// LLM 组织控制器：团队 / 成员 / 项目 / 终端用户 / 标签。

type LLMOrgController struct{ svc *service.LLMOrgService }

func NewLLMOrgController(s *service.LLMOrgService) *LLMOrgController {
	return &LLMOrgController{svc: s}
}

// ===== Dept LLM 配置 =====
func (h *LLMOrgController) ListDepts(c *gin.Context) {
	var req dto.LLMDeptSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListDepts(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMDeptSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMOrgController) GetDept(c *gin.Context) {
	id := c.Query("deptId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetDept(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMOrgController) EditDeptConfig(c *gin.Context) {
	var req dto.LLMDeptEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMDeptReq
	e := &entity.LLMDeptProfile{
		DeptId: req.DeptId, LlmModels: r.LlmModels,
		LlmBudgetDuration: r.LlmBudgetDuration, LlmRouterSettings: r.LlmRouterSettings, LlmBlocked: r.LlmBlocked,
	}
	if r.LlmTpmLimit > 0 {
		v := r.LlmTpmLimit
		e.LlmTpmLimit = &v
	}
	if r.LlmRpmLimit > 0 {
		v := r.LlmRpmLimit
		e.LlmRpmLimit = &v
	}
	if r.LlmMaxParallelRequests > 0 {
		v := r.LlmMaxParallelRequests
		e.LlmMaxParallelReq = &v
	}
	if err := h.svc.EditDeptConfig(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Project =====
func (h *LLMOrgController) ListProjects(c *gin.Context) {
	var req dto.LLMProjectSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListProjects(c, req.Alias, req.TeamId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMProjectSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMOrgController) GetProject(c *gin.Context) {
	id := c.Query("projectId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetProject(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMOrgController) AddProject(c *gin.Context) {
	var req dto.LLMProjectAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMProjectReq
	if _, err := h.svc.AddProject(c, &entity.LLMProject{
		ProjectAlias: r.ProjectAlias, Description: r.Description, TeamId: r.TeamId, BudgetId: r.BudgetId,
		Models: r.Models, Blocked: r.Blocked, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) EditProject(c *gin.Context) {
	var req dto.LLMProjectEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMProjectReq
	if err := h.svc.EditProject(c, &entity.LLMProject{
		ProjectId: req.ProjectId, ProjectAlias: r.ProjectAlias, Description: r.Description, TeamId: r.TeamId,
		BudgetId: r.BudgetId, Models: r.Models, Blocked: r.Blocked, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) DeleteProjects(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteProjects(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== EndUser =====
func (h *LLMOrgController) ListEndUsers(c *gin.Context) {
	var req dto.LLMEndUserSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListEndUsers(c, req.Alias, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMEndUserSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMOrgController) AddEndUser(c *gin.Context) {
	var req dto.LLMEndUserAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMEndUserReq
	if _, err := h.svc.AddEndUser(c, &entity.LLMEndUser{
		EndUserId: r.EndUserId, Alias: r.Alias, BudgetId: r.BudgetId, AllowedModelRegion: r.AllowedModelRegion,
		DefaultModel: r.DefaultModel, Blocked: r.Blocked,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) EditEndUser(c *gin.Context) {
	var req dto.LLMEndUserEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMEndUserReq
	if err := h.svc.EditEndUser(c, &entity.LLMEndUser{
		EndUserId: r.EndUserId, Alias: r.Alias, BudgetId: r.BudgetId, AllowedModelRegion: r.AllowedModelRegion,
		DefaultModel: r.DefaultModel, Blocked: r.Blocked,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) DeleteEndUsers(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteEndUsers(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Tag =====
func (h *LLMOrgController) ListTags(c *gin.Context) {
	var req dto.LLMTagSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListTags(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMTagSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMOrgController) AddTag(c *gin.Context) {
	var req dto.LLMTagAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMTagReq
	if err := h.svc.AddTag(c, &entity.LLMTag{
		TagName: r.TagName, Description: r.Description, Models: r.Models, BudgetId: r.BudgetId,
		CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) EditTag(c *gin.Context) {
	var req dto.LLMTagEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMTagReq
	if err := h.svc.EditTag(c, &entity.LLMTag{
		TagName: req.TagName, Description: r.Description, Models: r.Models, BudgetId: r.BudgetId,
		UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMOrgController) DeleteTags(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteTags(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
