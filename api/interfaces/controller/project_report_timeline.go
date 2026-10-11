package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 报表 + 时间线 控制器（均只读聚合）。

type ProjectReportController struct{ svc *service.ProjectReportService }

func NewProjectReportController(s *service.ProjectReportService) *ProjectReportController {
	return &ProjectReportController{svc: s}
}

func (h *ProjectReportController) Overview(c *gin.Context) {
	pid := c.Query("projectId")
	if pid == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Overview(c, pid)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, v)
}

type ProjectTimelineController struct {
	svc *service.ProjectTimelineService
}

func NewProjectTimelineController(s *service.ProjectTimelineService) *ProjectTimelineController {
	return &ProjectTimelineController{svc: s}
}

func (h *ProjectTimelineController) List(c *gin.Context) {
	pid := c.Query("projectId")
	if pid == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.List(c, pid)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}
