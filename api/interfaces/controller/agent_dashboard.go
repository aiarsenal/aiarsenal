package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// AgentDashboardController 概览：聚合统计。
type AgentDashboardController struct {
	svc *service.AgentDashboardService
}

func NewAgentDashboardController(s *service.AgentDashboardService) *AgentDashboardController {
	return &AgentDashboardController{svc: s}
}

func (h *AgentDashboardController) Overview(c *gin.Context) {
	uid := middleware.GetUserId(c)
	res, err := h.svc.Overview(c, uid)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, res)
}
