package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// MCP 系统配置 + 用户级配置控制器。

type McpConfigController struct{ svc *service.McpConfigService }

func NewMcpConfigController(s *service.McpConfigService) *McpConfigController {
	return &McpConfigController{svc: s}
}

func (h *McpConfigController) Get(c *gin.Context) {
	v, err := h.svc.GetSystemConfig(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, v)
}

func (h *McpConfigController) Save(c *gin.Context) {
	var e entity.McpConfig
	if err := c.ShouldBindJSON(&e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.SaveSystemConfig(c, &e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpConfigController) GetUserConfig(c *gin.Context) {
	userId := middleware.GetUserId(c)
	if userId == "" {
		middleware.Fail(c, "未登录")
		return
	}
	v, err := h.svc.GetUserConfig(c, userId)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, v)
}

func (h *McpConfigController) SaveUserConfig(c *gin.Context) {
	var e entity.McpUserConfig
	if err := c.ShouldBindJSON(&e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e.UserId = middleware.GetUserId(c)
	if err := h.svc.SaveUserConfig(c, &e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
