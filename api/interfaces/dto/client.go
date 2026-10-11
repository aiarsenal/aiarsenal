package dto

import "encoding/json"

// Client 模块请求/响应 DTO（镜像 agent.go 写法）。

// ClientBindReq 浏览器登录后绑定 deviceCode → 当前用户
type ClientBindReq struct {
	DeviceCode string `json:"deviceCode" binding:"required"`
}

// ClientExchangeReq App 端用 deviceCode 换 JWT
type ClientExchangeReq struct {
	DeviceCode string     `json:"deviceCode" binding:"required"`
	DeviceInfo DeviceInfo `json:"deviceInfo"`
}

// DeviceInfo App 采集上报的设备信息（exchange 时携带，落 aa_client_device）。
type DeviceInfo struct {
	DeviceId   string   `json:"deviceId"`
	Name       string   `json:"name"`
	Platform   string   `json:"platform"`
	Os         string   `json:"os"`
	Arch       string   `json:"arch"`
	AppVersion string   `json:"appVersion"`
	MacAddress string   `json:"macAddress"`
	MacExtra   []string `json:"macExtra"`
	Ip         string   `json:"ip"`
}

// ClientExchangeRes exchange 结果：status=ok 时附带 token + userInfo
type ClientExchangeRes struct {
	Status     string `json:"status"` // ok / pending
	Token      string `json:"token,omitempty"`
	UserId     string `json:"userId,omitempty"`
	UserName   string `json:"userName,omitempty"`
	Name       string `json:"name,omitempty"`       // 真实姓名（aa_user.name）
	EmployeeNo string `json:"employeeNo,omitempty"` // 工号（aa_user.employee_no）
	IsAdmin    int    `json:"isAdmin,omitempty"`
}

// ===== 管理后台 =====

// ClientAgentSaveReq 管理员保存 agent（类型+介绍）
type ClientAgentSaveReq struct {
	AgentId string `json:"agentId"`                  // 空=新建
	Agent   string `json:"agent" binding:"required"` // agent key，唯一
	Name    string `json:"name" binding:"required"`
	Intro   string `json:"intro"`
	Icon    string `json:"icon"`
	Logo    string `json:"logo"`
	Enabled uint   `json:"enabled"`
	Weigh   int    `json:"weigh"`
}

// ClientAgentVersionSaveReq 管理员保存版本
type ClientAgentVersionSaveReq struct {
	VersionId      string `json:"versionId"` // 空=新建
	AgentId        string `json:"agentId" binding:"required"`
	Version        string `json:"version" binding:"required"`
	IsDefault      uint   `json:"isDefault"`
	Enabled        uint   `json:"enabled"`
	DownloadUrl    string `json:"downloadUrl"`
	ContainerImage string `json:"containerImage"`
	Channel        string `json:"channel"`
	ConfigTemplate string `json:"configTemplate"`
	EnvVars        string `json:"envVars"`
	Extra          string `json:"extra"`
}

// ClientIdReq 按 id 删除（agent / version / app 通用）
type ClientIdReq struct {
	Id string `json:"id" binding:"required"`
}

// ClientAppSaveReq 管理员保存 App 目录条目（App 权限）
type ClientAppSaveReq struct {
	AppId    string `json:"appId"`                       // 空=新建
	BundleId string `json:"bundleId" binding:"required"` // macOS bundle id，唯一；编辑不改
	AppName  string `json:"appName" binding:"required"`
	Category string `json:"category"` // 类别 key（comm/docs/tools/browser/ai/system）
	Logo     string `json:"logo"`     // emoji logo
	Weigh    int    `json:"weigh"`
	Remarks  string `json:"remarks"`
}

// DeviceAppPermSaveReq 管理员整组覆盖某设备的 App 权限清单
type DeviceAppPermSaveReq struct {
	DeviceId  string   `json:"deviceId" binding:"required"`
	BundleIds []string `json:"bundleIds"` // 勾选的 bundle id 全集
}

// DeviceAppPermResetReq 管理员重置某设备的 App 自动化授权（下发 appperm_reset 指令）
type DeviceAppPermResetReq struct {
	DeviceId string `json:"deviceId" binding:"required"`
}

// ClientDeviceBlockReq 封禁/解封设备
type ClientDeviceBlockReq struct {
	Id      string `json:"id" binding:"required"` // device_id
	Blocked uint   `json:"blocked" binding:"required"`
}

// ClientPackageFetchReq 管理员从官网拉取安装包
type ClientPackageFetchReq struct {
	Agent       string `json:"agent" binding:"required"`
	Version     string `json:"version" binding:"required"`
	Platform    string `json:"platform" binding:"required"` // win/mac/linux/common
	OfficialUrl string `json:"officialUrl" binding:"required"`
}

// ClientCommandEnqueueReq 管理员下发远程指令
type ClientCommandEnqueueReq struct {
	DeviceId string          `json:"deviceId" binding:"required"`
	Action   string          `json:"action" binding:"required"` // install/uninstall/start/stop/restart/config_push/collect_data
	Agent    string          `json:"agent"`                     // agent key（设备级动作可空）
	Version  string          `json:"version"`                   // install 用
	Payload  json.RawMessage `json:"payload"`                   // 动作专属 json（spec/configContent/...）
}

// ClientDeviceMcpReq 设备 MCP 分组绑定/解绑请求。
type ClientDeviceMcpReq struct {
	DeviceId string `json:"deviceId" form:"deviceId" binding:"required"`
	GroupId  string `json:"groupId" form:"groupId" binding:"required"`
}

// ClientDeviceMcpSelfReq 设备自助 MCP 绑定/解绑请求（deviceId 取 JWT，不收前端参数）。
type ClientDeviceMcpSelfReq struct {
	GroupId string `json:"groupId" form:"groupId" binding:"required"`
}

// ClientDeviceSelectedAgentsReq 设置某设备选中的 agent（App 据此上报）
type ClientDeviceSelectedAgentsReq struct {
	DeviceId string   `json:"deviceId" binding:"required"`
	Agents   []string `json:"agents"` // agent key 列表
}

// ClientChannelSetReq 启用/禁用某设备某渠道（微信等）
type ClientChannelSetReq struct {
	DeviceId    string `json:"deviceId" binding:"required"`
	Channel     string `json:"channel" binding:"required"` // openclaw-weixin
	Enabled     bool   `json:"enabled"`
	DmPolicy    string `json:"dmPolicy"`    // pairing/allowlist/open/disabled
	GroupPolicy string `json:"groupPolicy"` // allowlist/open/disabled
}

// ClientChannelActionReq 渠道登录/登出/装插件
type ClientChannelActionReq struct {
	DeviceId string `json:"deviceId" binding:"required"`
	Channel  string `json:"channel"` // channel_login/channel_logout 用
	Spec     string `json:"spec"`    // plugin_install 用（npm spec）
}

// ClientSkillInstallReq 向设备下发 skill_install（从技能库选版本装到设备）
type ClientSkillInstallReq struct {
	DeviceId  string `json:"deviceId" binding:"required"`
	SkillId   string `json:"skillId" binding:"required"`
	VersionId string `json:"versionId"` // 空=自动解析（latest_version_id 回退最新非 yanked）
	Agent     string `json:"agent"`     // openclaw/hermes；空=按设备已启用 agent 推断
}

// ClientSkillUninstallReq 向设备下发 skill_uninstall（按装进托管目录的 slug 删目录）
type ClientSkillUninstallReq struct {
	DeviceId string `json:"deviceId" binding:"required"`
	Slug     string `json:"slug" binding:"required"`
	Agent    string `json:"agent"` // openclaw/hermes；空=按设备已启用 agent 推断
}
