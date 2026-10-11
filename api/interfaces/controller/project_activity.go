package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 动态（活动流，只读）。

type ProjectActivityController struct {
	svc *service.ProjectActivityService
}

func NewProjectActivityController(s *service.ProjectActivityService) *ProjectActivityController {
	return &ProjectActivityController{svc: s}
}

func (h *ProjectActivityController) List(c *gin.Context) {
	var req dto.ProjectActivitySearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ProjectId, req.TaskId, req.Action, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectActivitySearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}
