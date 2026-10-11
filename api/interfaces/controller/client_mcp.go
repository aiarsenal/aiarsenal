package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// ClientMcpController 设备 MCP 分组绑定（设备详情）：列表/绑/解绑，后台自动签 key +
// 尝试 restart 让全量重建的配置带上 mcp 段。
type ClientMcpController struct{ svc *service.ClientMcpService }

func NewClientMcpController(s *service.ClientMcpService) *ClientMcpController {
	return &ClientMcpController{svc: s}
}

// List GET /system/client/admin/device/mcp?deviceId=...
func (h *ClientMcpController) List(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		middleware.Fail(c, "缺少 deviceId")
		return
	}
	list, err := h.svc.List(c, deviceId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, list)
}

// Bind POST /system/client/admin/device/mcp/bind
func (h *ClientMcpController) Bind(c *gin.Context) {
	var req dto.ClientDeviceMcpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.DeviceId == "" || req.GroupId == "" {
		middleware.Fail(c, "缺少 deviceId/groupId")
		return
	}
	// restart 生效需要随配置下发 LLM 网关地址（同 AdminEnqueueCommand 的推导方式）。
	if err := h.svc.Bind(c, req.DeviceId, req.GroupId, middleware.GetUserId(c), deriveLLMBaseURL(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Unbind POST /system/client/admin/device/mcp/unbind
func (h *ClientMcpController) Unbind(c *gin.Context) {
	var req dto.ClientDeviceMcpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.DeviceId == "" || req.GroupId == "" {
		middleware.Fail(c, "缺少 deviceId/groupId")
		return
	}
	if err := h.svc.Unbind(c, req.DeviceId, req.GroupId, middleware.GetUserId(c), deriveLLMBaseURL(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// ===== 设备自助端点（桌面客户端 MCP 管理模块）=====
// deviceId 取 JWT（ClientDeviceAuth 已验设备登录态）；web token（DeviceId 空）一律 403 拒绝。

// ListSelf GET /system/client/mcp
func (h *ClientMcpController) ListSelf(c *gin.Context) {
	deviceId := middleware.GetUser(c).DeviceId
	if deviceId == "" {
		middleware.FailWith(c, 403, "仅桌面客户端可用")
		return
	}
	v, err := h.svc.SelfView(c, deviceId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, v)
}

// BindSelf POST /system/client/mcp/bind {groupId}
func (h *ClientMcpController) BindSelf(c *gin.Context) {
	deviceId := middleware.GetUser(c).DeviceId
	if deviceId == "" {
		middleware.FailWith(c, 403, "仅桌面客户端可用")
		return
	}
	var req dto.ClientDeviceMcpSelfReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.GroupId == "" {
		middleware.Fail(c, "缺少 groupId")
		return
	}
	if err := h.svc.Bind(c, deviceId, req.GroupId, middleware.GetUserId(c), deriveLLMBaseURL(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// UnbindSelf POST /system/client/mcp/unbind {groupId}
func (h *ClientMcpController) UnbindSelf(c *gin.Context) {
	deviceId := middleware.GetUser(c).DeviceId
	if deviceId == "" {
		middleware.FailWith(c, 403, "仅桌面客户端可用")
		return
	}
	var req dto.ClientDeviceMcpSelfReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.GroupId == "" {
		middleware.Fail(c, "缺少 groupId")
		return
	}
	if err := h.svc.Unbind(c, deviceId, req.GroupId, middleware.GetUserId(c), deriveLLMBaseURL(c), middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
