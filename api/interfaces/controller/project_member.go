package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 成员控制器。

type ProjectMemberController struct{ svc *service.ProjectMemberService }

func NewProjectMemberController(s *service.ProjectMemberService) *ProjectMemberController {
	return &ProjectMemberController{svc: s}
}

func (h *ProjectMemberController) List(c *gin.Context) {
	var req dto.ProjectMemberSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ProjectId, req.Role, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectMemberSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *ProjectMemberController) Add(c *gin.Context) {
	var req dto.ProjectMemberAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectMember{ProjectId: req.ProjectId, UserId: req.UserId, Role: req.Role, CreatedBy: middleware.GetUserId(c)}
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectMemberController) Edit(c *gin.Context) {
	var req dto.ProjectMemberEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Edit(c, req.MemberId, req.Role); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectMemberController) Delete(c *gin.Context) {
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
