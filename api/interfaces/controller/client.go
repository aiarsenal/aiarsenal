package controller

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// ClientController 桌面客户端支持：device-code 登录中转 / 可安装 agent 列表 / 运行情况上报 / 后台配置。
type ClientController struct{ svc *service.ClientService }

func NewClientController(s *service.ClientService) *ClientController {
	return &ClientController{svc: s}
}

// Bind  POST /api/v1/system/client/bind  （JWTAuth，无 casbin）
// 浏览器登录后把 deviceCode 绑定到当前登录用户。
func (h *ClientController) Bind(c *gin.Context) {
	var req dto.ClientBindReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, "参数错误: deviceCode 必填")
		return
	}
	u := middleware.GetUser(c)
	if u == nil {
		middleware.Unauthorized(c, "未登录")
		return
	}
	expired, err := h.svc.Bind(c, u.Id, u.UserName, u.IsAdmin, req.DeviceCode)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"bound": !expired, "expired": expired})
}

// Exchange  POST /api/v1/system/client/exchange  （公开）
// App 端用 deviceCode 换 JWT；未绑定返回 pending。deviceInfo 落设备记录。
func (h *ClientController) Exchange(c *gin.Context) {
	var req dto.ClientExchangeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, "参数错误: deviceCode 必填")
		return
	}
	info := service.DeviceInfo{
		DeviceId:   req.DeviceInfo.DeviceId,
		Name:       req.DeviceInfo.Name,
		Platform:   req.DeviceInfo.Platform,
		Os:         req.DeviceInfo.Os,
		Arch:       req.DeviceInfo.Arch,
		AppVersion: req.DeviceInfo.AppVersion,
		MacAddress: req.DeviceInfo.MacAddress,
		MacExtra:   req.DeviceInfo.MacExtra,
		Ip:         req.DeviceInfo.Ip,
	}
	token, userId, userName, name, employeeNo, isAdmin, ok, err := h.svc.Exchange(c, req.DeviceCode, &info, c.ClientIP(), deriveLLMBaseURL(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if !ok {
		middleware.Success(c, dto.ClientExchangeRes{Status: "pending"})
		return
	}
	middleware.Success(c, dto.ClientExchangeRes{
		Status:     "ok",
		Token:      token,
		UserId:     userId,
		UserName:   userName,
		Name:       name,
		EmployeeNo: employeeNo,
		IsAdmin:    isAdmin,
	})
}

// GetAgents  GET /api/v1/system/client/agents  （JWTAuth，无 casbin）
// 返回可安装 agent 列表 + 各 agent 的 enabled 版本（App 首页用）。
func (h *ClientController) GetAgents(c *gin.Context) {
	list, err := h.svc.ListAgents(c)
	if err != nil {
		middleware.Fail(c, "获取 agent 列表失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// Heartbeat  POST /api/v1/system/client/heartbeat  （JWTAuth，无 casbin）
// 一条 5s 心跳合一：上行多模块状态(含日志/配置变更/命令进度)，下行存储结果+最新命令。
func (h *ClientController) Heartbeat(c *gin.Context) {
	var req service.HeartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	u := middleware.GetUser(c)
	if u == nil || u.DeviceId == "" {
		middleware.Fail(c, "设备未绑定，无法心跳（请重新登录）")
		return
	}
	// deviceId 以 token 为准，防伪造
	req.DeviceId = u.DeviceId
	userId := u.Id
	res, err := h.svc.Heartbeat(c, userId, &req)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// AdminListLogs  GET /api/v1/system/client/admin/device/logs?deviceId=&agent=&commandId=&limit=
// commandId 非空时返回该命令的分步日志（正序），用于 UI 命令历史展开看每一步。
func (h *ClientController) AdminListLogs(c *gin.Context) {
	limit := 200
	if cmdId := c.Query("commandId"); cmdId != "" {
		list, err := h.svc.AdminListCommandLogs(c, cmdId, limit)
		// [诊断] 确认 commandId 分支被命中 + 返回条数（指令历史展开无日志排查）
		log.Printf("[cmdlog] AdminListLogs commandId=%q count=%d err=%v", cmdId, len(list), err)
		if err != nil {
			middleware.Fail(c, "获取命令日志失败")
			return
		}
		middleware.Success(c, gin.H{"list": list})
		return
	}
	deviceId := c.Query("deviceId")
	agent := c.Query("agent")
	list, err := h.svc.AdminListLogs(c, deviceId, agent, limit)
	if err != nil {
		middleware.Fail(c, "获取日志失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminListConfigChanges  GET /api/v1/system/client/admin/device/config-changes?deviceId=
func (h *ClientController) AdminListConfigChanges(c *gin.Context) {
	deviceId := c.Query("deviceId")
	list, err := h.svc.AdminListConfigChanges(c, deviceId, 50)
	if err != nil {
		middleware.Fail(c, "获取配置变更失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminGetOpenclawConfig  GET /api/v1/system/client/admin/device/openclaw-config?deviceId=
// 返回该设备 openclaw 最新配置（库中存的主体 + aa_llm_model 最新可用模型），2 空格美化。
// 供后台「手动下发配置」输入框预填。
func (h *ClientController) AdminGetOpenclawConfig(c *gin.Context) {
	deviceId := c.Query("deviceId")
	cfg, err := h.svc.AdminPreviewOpenclawConfig(c, deviceId)
	if err != nil {
		middleware.Fail(c, "获取 openclaw 配置失败")
		return
	}
	middleware.Success(c, gin.H{"config": cfg})
}

// ===== 管理后台（JWTAuth + CasbinAuth，路由 /client/admin/*） =====

// AdminListAgents  GET /api/v1/system/client/admin/agent/list
func (h *ClientController) AdminListAgents(c *gin.Context) {
	list, err := h.svc.AdminListAgents(c)
	if err != nil {
		middleware.Fail(c, "获取 agent 列表失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminListApps  GET /api/v1/system/client/admin/app/list
func (h *ClientController) AdminListApps(c *gin.Context) {
	list, err := h.svc.AdminListApps(c)
	if err != nil {
		middleware.Fail(c, "获取 App 目录失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminSuggestApps  GET /api/v1/system/client/admin/app/suggest?keyword=
// 按 App 名/关键词联想 Bundle ID（Apple iTunes Search 查 Mac App Store，目录录入辅助）。
// 查不到/无网返回空列表（前端降级手填），不报错。
func (h *ClientController) AdminSuggestApps(c *gin.Context) {
	list := h.svc.AdminSuggestApps(c.Request.Context(), c.Query("keyword"))
	middleware.Success(c, gin.H{"list": list})
}

// AdminSaveApp  PUT /api/v1/system/client/admin/app/save
func (h *ClientController) AdminSaveApp(c *gin.Context) {
	var req dto.ClientAppSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ClientApp{
		AppId:    req.AppId,
		BundleId: req.BundleId,
		AppName:  req.AppName,
		Category: req.Category,
		Logo:     req.Logo,
		Weigh:    req.Weigh,
		Remarks:  req.Remarks,
	}
	if err := h.svc.AdminSaveApp(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"saved": true, "appId": e.AppId})
}

// AdminDeleteApp  DELETE /api/v1/system/client/admin/app/delete
func (h *ClientController) AdminDeleteApp(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeleteApp(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"deleted": true})
}

// AdminGetDeviceAppPerms  GET /api/v1/system/client/admin/appperm/get?deviceId=
func (h *ClientController) AdminGetDeviceAppPerms(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		middleware.Fail(c, "deviceId 不能为空")
		return
	}
	perms, err := h.svc.AdminGetDeviceAppPerms(c, deviceId)
	if err != nil {
		middleware.Fail(c, "获取设备 App 权限配置失败")
		return
	}
	ids := make([]string, 0, len(perms))
	for _, p := range perms {
		ids = append(ids, p.BundleId)
	}
	// bundleIds 供勾选回显；perms 带客户端上报的 granted/denied 徽标数据
	middleware.Success(c, gin.H{"bundleIds": ids, "perms": perms})
}

// AdminResetDeviceAppPerm  POST /api/v1/system/client/admin/appperm/reset
// 重置某设备的 App 自动化授权（tccutil 全量重置，清单内所有 App 授权框重弹）。
func (h *ClientController) AdminResetDeviceAppPerm(c *gin.Context) {
	var req dto.DeviceAppPermResetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	commandId, err := h.svc.AdminResetDeviceAppPerm(c, req.DeviceId, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": commandId})
}

// AdminSetDeviceAppPerms  PUT /api/v1/system/client/admin/appperm/save
func (h *ClientController) AdminSetDeviceAppPerms(c *gin.Context) {
	var req dto.DeviceAppPermSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminSetDeviceAppPerms(c, req.DeviceId, req.BundleIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"saved": true})
}

// AdminSaveAgent  PUT /api/v1/system/client/admin/agent/save
func (h *ClientController) AdminSaveAgent(c *gin.Context) {
	var req dto.ClientAgentSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ClientAgent{
		AgentId: req.AgentId,
		Agent:   req.Agent,
		Name:    req.Name,
		Intro:   req.Intro,
		Icon:    req.Icon,
		Logo:    req.Logo,
		Enabled: req.Enabled,
		Weigh:   req.Weigh,
	}
	if err := h.svc.AdminSaveAgent(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"saved": true, "agentId": e.AgentId})
}

// AdminDeleteAgent  DELETE /api/v1/system/client/admin/agent/delete
func (h *ClientController) AdminDeleteAgent(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeleteAgent(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"deleted": true})
}

// AdminSaveVersion  PUT /api/v1/system/client/admin/version/save
func (h *ClientController) AdminSaveVersion(c *gin.Context) {
	var req dto.ClientAgentVersionSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ClientAgentVersion{
		VersionId:      req.VersionId,
		AgentId:        req.AgentId,
		Version:        req.Version,
		IsDefault:      req.IsDefault,
		Enabled:        req.Enabled,
		DownloadUrl:    req.DownloadUrl,
		ContainerImage: req.ContainerImage,
		Channel:        req.Channel,
		ConfigTemplate: req.ConfigTemplate,
		EnvVars:        req.EnvVars,
		Extra:          req.Extra,
	}
	if err := h.svc.AdminSaveVersion(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"saved": true, "versionId": e.VersionId})
}

// AdminDeleteVersion  DELETE /api/v1/system/client/admin/version/delete
func (h *ClientController) AdminDeleteVersion(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeleteVersion(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"deleted": true})
}

// AdminListStatus  GET /api/v1/system/client/admin/status/list
func (h *ClientController) AdminListStatus(c *gin.Context) {
	list, err := h.svc.AdminListStatus(c)
	if err != nil {
		middleware.Fail(c, "获取运行状态失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminListStatusByDevice  GET /api/v1/system/client/admin/device/status-history?deviceId=
// 某设备全部客户端状态历史（时间线）；设备页 App 上下线历史用。
func (h *ClientController) AdminListStatusByDevice(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		middleware.Fail(c, "deviceId 不能为空")
		return
	}
	list, err := h.svc.AdminListStatusByDevice(c, deviceId)
	if err != nil {
		middleware.Fail(c, "获取状态历史失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// ===== 远程指令（管理后台） =====

// AdminEnqueueCommand  POST /api/v1/system/client/admin/device/command
func (h *ClientController) AdminEnqueueCommand(c *gin.Context) {
	var req dto.ClientCommandEnqueueReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	// install openclaw 需把 LLM 网关地址随配置下发：从请求 Host 推导（api.aiarsenal.cc→llm.aiarsenal.cc）
	// 复用 deriveLLMBaseURL（scheme 跟随 X-Forwarded-Proto：测试 HTTP / 生产 HTTPS）。
	llmBaseURL := deriveLLMBaseURL(c)
	id, err := h.svc.AdminEnqueueCommand(c, req.DeviceId, req.Action, req.Agent, req.Version, req.Payload, userId, llmBaseURL)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminListCommands  GET /api/v1/system/client/admin/device/commands/:deviceId
func (h *ClientController) AdminListCommands(c *gin.Context) {
	deviceId := c.Param("deviceId")
	list, err := h.svc.AdminListCommands(c, deviceId, 100)
	if err != nil {
		middleware.Fail(c, "获取指令历史失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminGetDeviceDetail  GET /api/v1/system/client/admin/device/detail?deviceId=...
func (h *ClientController) AdminGetDeviceDetail(c *gin.Context) {
	deviceId := c.Query("deviceId")
	dev, statuses, selected, err := h.svc.AdminGetDeviceDetail(c, deviceId)
	if err != nil {
		middleware.Fail(c, "获取设备详情失败")
		return
	}
	// 字段名沿用 commandedAgents 兼容前端，语义已改为「选中的 agent」
	middleware.Success(c, gin.H{"device": dev, "statuses": statuses, "commandedAgents": selected})
}

// AdminSetSelectedAgents  PUT /api/v1/system/client/admin/device/selected-agents
// 设置某设备选中的 agent（App 心跳据此上报；下发指令 tab 也只显示这些）。
func (h *ClientController) AdminSetSelectedAgents(c *gin.Context) {
	var req dto.ClientDeviceSelectedAgentsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminSetSelectedAgents(c, req.DeviceId, req.Agents); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"ok": true})
}

// ===== 管理后台：渠道绑定（微信等） =====

// AdminListChannels  GET /api/v1/system/client/admin/device/channels?deviceId=...
func (h *ClientController) AdminListChannels(c *gin.Context) {
	list, err := h.svc.AdminListChannels(c, c.Query("deviceId"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminSetChannel  PUT /api/v1/system/client/admin/device/channel
func (h *ClientController) AdminSetChannel(c *gin.Context) {
	var req dto.ClientChannelSetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminSetChannel(c, req.DeviceId, req.Channel, req.Enabled, req.DmPolicy, req.GroupPolicy); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"ok": true})
}

// AdminGetChannelQr  GET /api/v1/system/client/admin/device/channel/qr?deviceId=...&channel=...
func (h *ClientController) AdminGetChannelQr(c *gin.Context) {
	st, err := h.svc.AdminGetChannelQr(c, c.Query("deviceId"), c.Query("channel"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, st)
}

// AdminChannelLogin  POST /api/v1/system/client/admin/device/channel/login
func (h *ClientController) AdminChannelLogin(c *gin.Context) {
	var req dto.ClientChannelActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	id, err := h.svc.AdminStartChannelLogin(c, req.DeviceId, req.Channel, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminChannelLogout  POST /api/v1/system/client/admin/device/channel/logout
func (h *ClientController) AdminChannelLogout(c *gin.Context) {
	var req dto.ClientChannelActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	id, err := h.svc.AdminChannelLogout(c, req.DeviceId, req.Channel, userId, deriveLLMBaseURL(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminChannelInstallPlugin  POST /api/v1/system/client/admin/device/channel/install-plugin
func (h *ClientController) AdminChannelInstallPlugin(c *gin.Context) {
	var req dto.ClientChannelActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.Spec == "" {
		middleware.Fail(c, "spec 不能为空")
		return
	}
	userId := middleware.GetUserId(c)
	id, err := h.svc.AdminInstallPlugin(c, req.DeviceId, req.Spec, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminListDeviceSkills  GET /api/v1/system/client/admin/device/skills?deviceId=
// 设备已装技能列表（客户端心跳上报的 ~/.openclaw/skills 扫描快照）。
func (h *ClientController) AdminListDeviceSkills(c *gin.Context) {
	list, err := h.svc.AdminListDeviceSkills(c, c.Query("deviceId"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminInstallDeviceSkill  POST /api/v1/system/client/admin/device/skill/install
// 从技能库选技能装到设备：下发 skill_install，App 下载 zip 解压后按 agent 落地
// （openclaw skills install --global / hermes 落盘 $HERMES_HOME/skills）。
func (h *ClientController) AdminInstallDeviceSkill(c *gin.Context) {
	var req dto.ClientSkillInstallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	id, err := h.svc.AdminInstallDeviceSkill(c, req.DeviceId, req.SkillId, req.VersionId, req.Agent, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminUninstallDeviceSkill  POST /api/v1/system/client/admin/device/skill/uninstall
// 卸载设备上的技能：下发 skill_uninstall，App 删目标 agent 技能目录下对应 slug 目录（幂等）。
func (h *ClientController) AdminUninstallDeviceSkill(c *gin.Context) {
	var req dto.ClientSkillUninstallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	id, err := h.svc.AdminUninstallDeviceSkill(c, req.DeviceId, req.Slug, req.Agent, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"commandId": id})
}

// AdminChannelUnbind  POST /api/v1/system/client/admin/device/channel/unbind
// 解绑渠道：只清当前账号绑定（渠道功能保留，可直接重新扫码绑其它账号）。
func (h *ClientController) AdminChannelUnbind(c *gin.Context) {
	var req dto.ClientChannelActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	if err := h.svc.AdminUnbindChannel(c, req.DeviceId, req.Channel, userId, deriveLLMBaseURL(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"ok": true})
}

// ===== 管理后台：设备管理 =====

// AdminListDevices  GET /api/v1/system/client/admin/device/list
func (h *ClientController) AdminListDevices(c *gin.Context) {
	list, err := h.svc.AdminListDevices(c)
	if err != nil {
		middleware.Fail(c, "获取设备列表失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminGetDevice  GET /api/v1/system/client/admin/device/get
func (h *ClientController) AdminGetDevice(c *gin.Context) {
	id := c.Query("id")
	dev, err := h.svc.AdminGetDevice(c, id)
	if err != nil {
		middleware.Fail(c, "获取设备失败")
		return
	}
	middleware.Success(c, gin.H{"device": dev})
}

// AdminRevokeDevice  PUT /api/v1/system/client/admin/device/revoke
func (h *ClientController) AdminRevokeDevice(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminRevokeDevice(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"revoked": true})
}

// AdminBlockDevice  PUT /api/v1/system/client/admin/device/block
func (h *ClientController) AdminBlockDevice(c *gin.Context) {
	var req dto.ClientDeviceBlockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminSetDeviceBlocked(c, req.Id, req.Blocked); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"blocked": req.Blocked})
}

// AdminDeleteDevice  DELETE /api/v1/system/client/admin/device/delete
func (h *ClientController) AdminDeleteDevice(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeleteDevice(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"deleted": true})
}

// ===== 管理后台：安装包缓存 =====

// AdminListPackages  GET /api/v1/system/client/admin/package/list
func (h *ClientController) AdminListPackages(c *gin.Context) {
	list, err := h.svc.AdminListPackages(c)
	if err != nil {
		middleware.Fail(c, "获取安装包列表失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// AdminFetchPackage  PUT /api/v1/system/client/admin/package/fetch
// 从 officialUrl 拉取安装包到本地（必须官网来源）。
func (h *ClientController) AdminFetchPackage(c *gin.Context) {
	var req dto.ClientPackageFetchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	pkg, err := h.svc.FetchPackage(c, req.Agent, req.Version, req.Platform, req.OfficialUrl, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"packageId": pkg.PackageId, "filename": pkg.Filename, "fileSize": pkg.FileSize, "sha256": pkg.Sha256})
}

// AdminDeletePackage  DELETE /api/v1/system/client/admin/package/delete
func (h *ClientController) AdminDeletePackage(c *gin.Context) {
	var req dto.ClientIdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeletePackage(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"deleted": true})
}

// DownloadPackage  GET /api/v1/system/client/packages/:agent/:version/:platform （JWTAuth，无 casbin）
// App 端从本服务下载已缓存的安装包（替代直连官网）。
func (h *ClientController) DownloadPackage(c *gin.Context) {
	agent := c.Param("agent")
	version := c.Param("version")
	platform := c.Param("platform")
	pkg, err := h.svc.GetPackageByCVP(c, agent, version, platform)
	if err != nil {
		middleware.Fail(c, "安装包不存在")
		return
	}
	abs := h.svc.PackageLocalPath(pkg)
	if _, err := os.Stat(abs); err != nil {
		middleware.Fail(c, "安装包文件缺失")
		return
	}
	c.FileAttachment(abs, pkg.Filename)
}
