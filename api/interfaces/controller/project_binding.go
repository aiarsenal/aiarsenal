package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 资源绑定控制器（项目 ↔ Agent/Skill/KB/LLM/MCP）。

type ProjectBindingController struct {
	svc *service.ProjectBindingService
}

func NewProjectBindingController(s *service.ProjectBindingService) *ProjectBindingController {
	return &ProjectBindingController{svc: s}
}

func (h *ProjectBindingController) List(c *gin.Context) {
	var req dto.ProjectBindingSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ProjectId, req.Kind, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectBindingSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *ProjectBindingController) Get(c *gin.Context) {
	id := c.Query("bindingId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *ProjectBindingController) Add(c *gin.Context) {
	var req dto.ProjectBindingAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectBinding{ProjectId: req.ProjectId, Kind: req.Kind, TargetId: req.TargetId, TargetName: req.TargetName, Config: req.Config, Enabled: req.Enabled, CreatedBy: middleware.GetUserId(c)}
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectBindingController) Edit(c *gin.Context) {
	var req dto.ProjectBindingEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectBinding{ProjectId: req.ProjectId, Kind: req.Kind, TargetId: req.TargetId, TargetName: req.TargetName, Config: req.Config, Enabled: req.Enabled, BindingId: req.BindingId}
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectBindingController) Delete(c *gin.Context) {
	var req dto.ProjectIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
