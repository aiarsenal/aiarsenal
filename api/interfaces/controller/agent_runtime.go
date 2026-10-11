package controller

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentRuntimeController Phase3 控制面：容器内 agent 的心跳/认领/回传 + 管理端命令下发。
type AgentRuntimeController struct{ svc *service.AgentRuntimeService }

func NewAgentRuntimeController(s *service.AgentRuntimeService) *AgentRuntimeController {
	return &AgentRuntimeController{svc: s}
}

// Heartbeat POST /agent/runtime/heartbeat （InstanceAuth）
func (h *AgentRuntimeController) Heartbeat(c *gin.Context) {
	instanceId := middleware.GetInstanceId(c)
	if instanceId == "" {
		middleware.Fail(c, "实例未识别")
		return
	}
	var req dto.AgentRuntimeHeartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body 心跳
		req = dto.AgentRuntimeHeartbeatReq{}
	}
	raw, _ := json.Marshal(req)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	stored, commands, supportAgents, err := h.svc.Heartbeat(c, instanceId, m)
	if err != nil {
		middleware.Fail(c, "心跳处理失败")
		return
	}
	middleware.Success(c, dto.AgentRuntimeHeartbeatRes{Stored: stored, Commands: commands, SupportAgents: supportAgents})
}

// EnqueueCommand POST /agent/instance/:id/command/enqueue （JWTAuth+CasbinAuth 管理端）
func (h *AgentRuntimeController) EnqueueCommand(c *gin.Context) {
	instanceId := c.Param("id")
	if instanceId == "" {
		instanceId = c.Query("instanceId")
	}
	var req dto.AgentEnqueueCommandReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if instanceId == "" {
		instanceId = req.InstanceId
	}
	if instanceId == "" {
		middleware.Fail(c, "缺少 instanceId")
		return
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 300
	}
	issuedBy := middleware.GetUserId(c)
	cmdId, err := h.svc.EnqueueCommand(c, instanceId, req.CommandType, req.AgentId, issuedBy, req.Payload, req.TimeoutSeconds)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.AgentEnqueueCommandRes{CommandId: cmdId})
}

// ListCommands GET /agent/instance/:id/command/list （管理端）
func (h *AgentRuntimeController) ListCommands(c *gin.Context) {
	instanceId := c.Param("id")
	status := -1
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			status = v
		}
	}
	pageNum, _ := strconv.Atoi(c.Query("pageNum"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	list, total, err := h.svc.ListCommands(c, instanceId, status, pageNum, pageSize)
	if err != nil {
		middleware.Fail(c, "获取失败")
		return
	}
	pn := pageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentCommandListRes{ListRes: dto.ListRes{CurrentPage: pn, Total: total}, List: list})
}

// GetRuntimeStatus GET /agent/instance/:id/runtime-status （管理端）
func (h *AgentRuntimeController) GetRuntimeStatus(c *gin.Context) {
	instanceId := c.Param("id")
	st, err := h.svc.GetRuntimeStatus(c, instanceId)
	if err != nil {
		middleware.Fail(c, "未找到运行状态")
		return
	}
	middleware.Success(c, st)
}

// ListStatusHistory GET /agent/instance/:id/runtime-status/history （管理端，状态时间线）
func (h *AgentRuntimeController) ListStatusHistory(c *gin.Context) {
	instanceId := c.Param("id")
	limit := parseLimit(c.DefaultQuery("limit", "50"))
	list, err := h.svc.ListStatusHistory(c, instanceId, limit)
	if err != nil {
		middleware.Fail(c, "获取失败")
		return
	}
	middleware.Success(c, list)
}

// ListConfigChanges GET /agent/instance/:id/config-changes （管理端，配置变更历史）
func (h *AgentRuntimeController) ListConfigChanges(c *gin.Context) {
	instanceId := c.Param("id")
	limit := parseLimit(c.DefaultQuery("limit", "50"))
	list, err := h.svc.ListConfigChanges(c, instanceId, limit)
	if err != nil {
		middleware.Fail(c, "获取失败")
		return
	}
	middleware.Success(c, list)
}

// ListInstanceSkills GET /agent/instance/:id/skill/list （管理端，实例已安装 skill）
func (h *AgentRuntimeController) ListInstanceSkills(c *gin.Context) {
	instanceId := c.Param("id")
	list, err := h.svc.ListInstanceSkills(c, instanceId)
	if err != nil {
		middleware.Fail(c, "获取失败")
		return
	}
	middleware.Success(c, list)
}

// InstallSkill POST /agent/instance/:id/skill/install （管理端，下发 install_skill）
func (h *AgentRuntimeController) InstallSkill(c *gin.Context) {
	instanceId := c.Param("id")
	var req dto.AgentInstanceSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if instanceId == "" {
		instanceId = req.InstanceId
	}
	if instanceId == "" || req.SkillId == "" {
		middleware.Fail(c, "缺少 instanceId/skillId")
		return
	}
	cmdId, err := h.svc.InstallSkill(c, instanceId, req.SkillId, req.VersionId, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.AgentInstanceSkillOpRes{CommandId: cmdId})
}

// UninstallSkill POST /agent/instance/:id/skill/uninstall （管理端，下发 uninstall_skill）
func (h *AgentRuntimeController) UninstallSkill(c *gin.Context) {
	instanceId := c.Param("id")
	var req dto.AgentInstanceSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if instanceId == "" {
		instanceId = req.InstanceId
	}
	if instanceId == "" || req.SkillId == "" {
		middleware.Fail(c, "缺少 instanceId/skillId")
		return
	}
	cmdId, err := h.svc.UninstallSkill(c, instanceId, req.SkillId, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.AgentInstanceSkillOpRes{CommandId: cmdId})
}

// parseLimit 解析 limit 查询参数，默认 50，上限 500。
func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 50
	}
	if n > 500 {
		return 500
	}
	return n
}
