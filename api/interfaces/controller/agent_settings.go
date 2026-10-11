package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentSettingsController 系统设置：admin 镜像卡片（system_image_setting）。
type AgentSettingsController struct{ svc *service.AgentSettingsService }

func NewAgentSettingsController(s *service.AgentSettingsService) *AgentSettingsController {
	return &AgentSettingsController{svc: s}
}

func (h *AgentSettingsController) ListImages(c *gin.Context) {
	var req dto.AgentSystemImageSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.ListImages(c, req.Agent, req.Version)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.AgentSystemImageSearchRes{List: list})
}

func (h *AgentSettingsController) UpsertImage(c *gin.Context) {
	var req dto.AgentSystemImageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.UpsertImage(c, &entity.AgentSystemImageSetting{
		Agent: req.Agent, Version: req.Version,
		DisplayName: req.DisplayName, Image: req.Image, IsEnabled: req.IsEnabled,
		UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentSettingsController) DeleteImage(c *gin.Context) {
	agentVal := c.Query("agent")
	versionVal := c.Query("version")
	if agentVal == "" || versionVal == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	if err := h.svc.DeleteImage(c, agentVal, versionVal); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
