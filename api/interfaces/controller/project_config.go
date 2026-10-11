package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 设置控制器（标签 / 状态流 / 自定义字段 三类配置）。

type ProjectConfigController struct{ svc *service.ProjectConfigService }

func NewProjectConfigController(s *service.ProjectConfigService) *ProjectConfigController {
	return &ProjectConfigController{svc: s}
}

// ===== 标签 =====
func (h *ProjectConfigController) TagList(c *gin.Context) {
	var req dto.ProjectTagSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListTags(c, req.ProjectId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectTagSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}
func (h *ProjectConfigController) TagAdd(c *gin.Context) {
	var req dto.ProjectTagAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectTag{ProjectId: req.ProjectId, Name: req.Name, Color: req.Color, SortOrder: req.SortOrder, CreatedBy: middleware.GetUserId(c)}
	if _, err := h.svc.AddTag(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) TagEdit(c *gin.Context) {
	var req dto.ProjectTagEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectTag{ProjectId: req.ProjectId, Name: req.Name, Color: req.Color, SortOrder: req.SortOrder, TagId: req.TagId}
	if err := h.svc.EditTag(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) TagDelete(c *gin.Context) {
	var req dto.ProjectIdsReq
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

// ===== 状态流 =====
func (h *ProjectConfigController) StatusList(c *gin.Context) {
	var req dto.ProjectStatusSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListStatus(c, req.ProjectId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectStatusSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}
func (h *ProjectConfigController) StatusAdd(c *gin.Context) {
	var req dto.ProjectStatusAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectStatus{ProjectId: req.ProjectId, Key: req.Key, Name: req.Name, Color: req.Color, Category: req.Category, SortOrder: req.SortOrder, IsDefault: req.IsDefault, CreatedBy: middleware.GetUserId(c)}
	if _, err := h.svc.AddStatus(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) StatusEdit(c *gin.Context) {
	var req dto.ProjectStatusEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectStatus{ProjectId: req.ProjectId, Key: req.Key, Name: req.Name, Color: req.Color, Category: req.Category, SortOrder: req.SortOrder, IsDefault: req.IsDefault, StatusId: req.StatusId}
	if err := h.svc.EditStatus(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) StatusDelete(c *gin.Context) {
	var req dto.ProjectIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteStatus(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 自定义字段 =====
func (h *ProjectConfigController) FieldList(c *gin.Context) {
	var req dto.ProjectCustomFieldSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListFields(c, req.ProjectId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectCustomFieldSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}
func (h *ProjectConfigController) FieldAdd(c *gin.Context) {
	var req dto.ProjectCustomFieldAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectCustomField{ProjectId: req.ProjectId, Name: req.Name, Type: req.Type, Options: req.Options, SortOrder: req.SortOrder, CreatedBy: middleware.GetUserId(c)}
	if _, err := h.svc.AddField(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) FieldEdit(c *gin.Context) {
	var req dto.ProjectCustomFieldEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectCustomField{ProjectId: req.ProjectId, Name: req.Name, Type: req.Type, Options: req.Options, SortOrder: req.SortOrder, FieldId: req.FieldId}
	if err := h.svc.EditField(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
func (h *ProjectConfigController) FieldDelete(c *gin.Context) {
	var req dto.ProjectIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteFields(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
