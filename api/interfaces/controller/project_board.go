package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 看板控制器（列+任务聚合；拖拽改状态）。

type ProjectBoardController struct{ svc *service.ProjectBoardService }

func NewProjectBoardController(s *service.ProjectBoardService) *ProjectBoardController {
	return &ProjectBoardController{svc: s}
}

// Board 返回 { columns:[状态], tasks:[任务] }，前端按 status 分组渲染列。
func (h *ProjectBoardController) Board(c *gin.Context) {
	pid := c.Query("projectId")
	cols, tasks, err := h.svc.Board(c, pid)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.ProjectBoardRes{Columns: cols, Tasks: tasks})
}

// Move 拖拽落点：更新任务 status + sort_order。
func (h *ProjectBoardController) Move(c *gin.Context) {
	var req dto.ProjectTaskMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Move(c, req.TaskId, req.Status, req.SortOrder, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
