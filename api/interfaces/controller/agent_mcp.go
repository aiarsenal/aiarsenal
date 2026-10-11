package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentMcpController 实例 MCP 分组绑定（详情抽屉）：列表/绑/解绑，后台自动签 key + 重推配置。
type AgentMcpController struct{ svc *service.AgentMcpService }

func NewAgentMcpController(s *service.AgentMcpService) *AgentMcpController {
	return &AgentMcpController{svc: s}
}

// List GET /agent/instance/:id/mcp/list （有效分组=直绑∪项目继承，带来源与 server 概览）
func (h *AgentMcpController) List(c *gin.Context) {
	instanceId := c.Param("id")
	if instanceId == "" {
		middleware.Fail(c, "缺少 instanceId")
		return
	}
	list, err := h.svc.List(c, instanceId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, list)
}

// Bind POST /agent/instance/:id/mcp/bind
func (h *AgentMcpController) Bind(c *gin.Context) {
	instanceId := c.Param("id")
	var req dto.AgentInstanceMcpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if instanceId == "" {
		instanceId = req.InstanceId
	}
	if instanceId == "" || req.GroupId == "" {
		middleware.Fail(c, "缺少 instanceId/groupId")
		return
	}
	if err := h.svc.Bind(c, instanceId, req.GroupId, middleware.GetUserId(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Unbind POST /agent/instance/:id/mcp/unbind
func (h *AgentMcpController) Unbind(c *gin.Context) {
	instanceId := c.Param("id")
	var req dto.AgentInstanceMcpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if instanceId == "" {
		instanceId = req.InstanceId
	}
	if instanceId == "" || req.GroupId == "" {
		middleware.Fail(c, "缺少 instanceId/groupId")
		return
	}
	if err := h.svc.Unbind(c, instanceId, req.GroupId, middleware.GetUserId(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
