package entity

import (
	"encoding/json"

	"aiarsenal/domain/types"
)

// Client 模块实体（client 库，表前缀 aa_client_）。
// 桌面客户端（Win/Mac App）：可安装 agent（类型+介绍）→ 多个 version（各自配置，全局）+ 运行情况上报（每用户）。
// 跨库软引用：user_id → aiarsenal.aa_user.user_id（varchar(32) biz_id，无实际 FK）。

// ClientAgent agent 类型级：每 agent 一行（agent key 唯一，可由管理员自由新增）。
type ClientAgent struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AgentId   string      `gorm:"column:agent_id;uniqueIndex" json:"agentId"`
	Agent     string      `gorm:"column:agent;uniqueIndex" json:"agent"` // agent key（openclaw/hermes/obsidian/...）
	Name      string      `gorm:"column:name" json:"name"`
	Intro     string      `gorm:"column:intro" json:"intro"`     // 介绍/说明
	Icon      string      `gorm:"column:icon" json:"icon"`       // emoji 或图标名
	Logo      string      `gorm:"column:logo" json:"logo"`       // Logo 图片 URL（空=客户端 App 回退到打包 logo 或 emoji）
	Enabled   uint        `gorm:"column:enabled" json:"enabled"` // 0/1
	Weigh     int         `gorm:"column:weigh" json:"weigh"`     // 排序，大者靠前
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClientAgent) TableName() string { return "aa_client_agent" }

// ClientAgentVersion agent 版本级：每 agent 多行，每版本各自安装+配置规格。
type ClientAgentVersion struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	VersionId      string      `gorm:"column:version_id;uniqueIndex" json:"versionId"`
	AgentId        string      `gorm:"column:agent_id;uniqueIndex:idx_agent_ver" json:"agentId"` // 软引用 aa_client_agent.agent_id
	Version        string      `gorm:"column:version;uniqueIndex:idx_agent_ver" json:"version"`  // npm 包版本 / 镜像 tag / dmg 版本
	IsDefault      uint        `gorm:"column:is_default" json:"isDefault"`                       // 0/1，每 agent 至多一条
	Enabled        uint        `gorm:"column:enabled" json:"enabled"`                            // 0/1
	DownloadUrl    string      `gorm:"column:download_url" json:"downloadUrl"`                   // obsidian 安装包 / openclaw npm 包（空=走 npm）
	ContainerImage string      `gorm:"column:container_image" json:"containerImage"`             // hermes 专用镜像
	Channel        string      `gorm:"column:channel" json:"channel"`                            // openclaw channel 等
	ConfigTemplate string      `gorm:"column:config_template;type:jsonb" json:"configTemplate"`  // 配置文件模板（对象）
	EnvVars        string      `gorm:"column:env_vars;type:jsonb" json:"envVars"`                // 环境变量（hermes 容器）
	Extra          string      `gorm:"column:extra;type:jsonb" json:"extra"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClientAgentVersion) TableName() string { return "aa_client_agent_version" }

// ClientAgentStatus 桌面客户端运行状态（每设备每客户端一行）。
type ClientAgentStatus struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	StatusId   string      `gorm:"column:status_id;uniqueIndex" json:"statusId"`
	DeviceId   string      `gorm:"column:device_id;uniqueIndex:idx_device_agent" json:"deviceId"`
	UserId     string      `gorm:"column:user_id;index" json:"userId"`
	Agent      string      `gorm:"column:agent;uniqueIndex:idx_device_agent" json:"agent"` // agent key；client=App 自身心跳
	Kind       string      `gorm:"column:kind" json:"kind"`                                // agent | app
	Status     string      `gorm:"column:status" json:"status"`                            // running / stopped / installing / error / notinstalled
	Version    string      `gorm:"column:version" json:"version"`
	LastError  string      `gorm:"column:last_error" json:"lastError"`
	Extra      string      `gorm:"column:extra;type:jsonb" json:"extra"`
	ReportedAt *types.Time `gorm:"column:reported_at" json:"reportedAt"`
	// CreatedAt 行插入时间（不随合并刷新）。20s 活跃期判断用它，而非 reported_at。
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	// UptimeSeconds 仅响应回填：running 且最近上报过（活跃）时 = now − created_at，
	// 后端读时计算，不落库（客户端从不真实上报，旧 uptime_seconds 列已 drop）。
	UptimeSeconds int64 `gorm:"-" json:"uptimeSeconds"`
}

func (ClientAgentStatus) TableName() string { return "aa_client_agent_status" }

// ClientDevice 设备登录记录（每设备一行，App 登录 exchange 时上报并 upsert）。
type ClientDevice struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DeviceId    string      `gorm:"column:device_id;uniqueIndex" json:"deviceId"`
	UserId      string      `gorm:"column:user_id;index" json:"userId"`
	UserName    string      `gorm:"column:user_name" json:"userName"`
	Name        string      `gorm:"column:name" json:"name"`         // 主机名/设备名
	Platform    string      `gorm:"column:platform" json:"platform"` // win / mac
	Os          string      `gorm:"column:os" json:"os"`
	Arch        string      `gorm:"column:arch" json:"arch"`
	AppVersion  string      `gorm:"column:app_version" json:"appVersion"`
	MacAddress  string      `gorm:"column:mac_address" json:"macAddress"`
	MacExtra    string      `gorm:"column:mac_extra;type:jsonb" json:"macExtra"`
	Ip          string      `gorm:"column:ip" json:"ip"`
	Status      string      `gorm:"column:status" json:"status"`   // active / revoked
	Blocked     uint        `gorm:"column:blocked" json:"blocked"` // 0/1
	LastLoginAt *types.Time `gorm:"column:last_login_at" json:"lastLoginAt"`
	LastLoginIp string      `gorm:"column:last_login_ip" json:"lastLoginIp"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	Extra       string      `gorm:"column:extra;type:jsonb" json:"extra"`
	Channels    string      `gorm:"column:channels;type:jsonb" json:"channels"` // 渠道绑定状态数组 JSON（ClientChannelState）
	Skills      string      `gorm:"column:skills;type:jsonb" json:"skills"`     // 已装技能快照数组 JSON（ClientSkillState）
	McpKeyPlain string      `gorm:"column:mcp_key_plain" json:"-"`              // 设备专属 MCP Bearer key 明文（绑分组时签发；配置下发随时要用，密钥库只存哈希故落实例行，永不回显）
}

func (ClientDevice) TableName() string { return "aa_client_device" }

// ClientDeviceMcpGroup 设备↔MCP 分组关联（绑分组→自动签收紧密钥+配置下发注入 mcp.servers）。
type ClientDeviceMcpGroup struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DeviceMcpId string      `gorm:"column:device_mcp_id;uniqueIndex" json:"deviceMcpId"`
	DeviceId    string      `gorm:"column:device_id" json:"deviceId"`
	GroupId     string      `gorm:"column:group_id" json:"groupId"`     // → aa_mcp_group.group_id
	GroupName   string      `gorm:"column:group_name" json:"groupName"` // 缓存展示名（网关 scope 即组名）
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClientDeviceMcpGroup) TableName() string { return "aa_client_device_mcp_group" }

// ClientPackage 安装包缓存（每 agent+version+platform 一行，后端从官网拉取落盘供 App 下载）。
type ClientPackage struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PackageId   string      `gorm:"column:package_id;uniqueIndex" json:"packageId"`
	Agent       string      `gorm:"column:agent;uniqueIndex:idx_cvp" json:"agent"`
	Version     string      `gorm:"column:version;uniqueIndex:idx_cvp" json:"version"`
	Platform    string      `gorm:"column:platform;uniqueIndex:idx_cvp" json:"platform"` // win / mac / linux / common
	Filename    string      `gorm:"column:filename" json:"filename"`
	FileSize    int64       `gorm:"column:file_size" json:"fileSize"`
	Sha256      string      `gorm:"column:sha256" json:"sha256"`
	OfficialUrl string      `gorm:"column:official_url" json:"officialUrl"`
	LocalPath   string      `gorm:"column:local_path" json:"localPath"`
	Status      string      `gorm:"column:status" json:"status"` // ready / failed
	FetchedBy   string      `gorm:"column:fetched_by" json:"fetchedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	FetchedAt   *types.Time `gorm:"column:fetched_at" json:"fetchedAt"`
	// Logo 仅响应回填（管理后台列表展示用），不落库。AdminListPackages 用 agent→logo 映射填入。
	Logo string `gorm:"-" json:"logo,omitempty"`
}

func (ClientPackage) TableName() string { return "aa_client_package" }

// ClientCommand 远程指令队列（管理员入队，App 认领执行回传结果）。
type ClientCommand struct {
	Id         uint            `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	CommandId  string          `gorm:"column:command_id;uniqueIndex" json:"commandId"`
	DeviceId   string          `gorm:"column:device_id;index" json:"deviceId"`
	Action     string          `gorm:"column:action" json:"action"`   // install/uninstall/start/stop/restart/config_push/collect_data
	Agent      string          `gorm:"column:agent" json:"agent"`     // agent key
	Version    string          `gorm:"column:version" json:"version"` // install 用
	Payload    json.RawMessage `gorm:"column:payload;type:jsonb" json:"payload"`
	Status     string          `gorm:"column:status" json:"status"` // pending/running/done/failed
	Result     string          `gorm:"column:result;type:jsonb" json:"result"`
	Progress   string          `gorm:"column:progress;type:jsonb" json:"progress"`
	Error      string          `gorm:"column:error" json:"error"`
	ChainId    string          `gorm:"column:chain_id" json:"chainId"` // 安装链 id：install→config→start 同一链；某步失败则取消后续
	CreatedBy  string          `gorm:"column:created_by" json:"createdBy"`
	CreatedAt  *types.Time     `gorm:"column:created_at" json:"createdAt"`
	StartedAt  *types.Time     `gorm:"column:started_at" json:"startedAt"`
	FinishedAt *types.Time     `gorm:"column:finished_at" json:"finishedAt"`
	// 超时（idle 语义）：timeout_seconds>0 覆盖 action 默认；last_progress_at 为进度最近变化时间
	// （认领时=started_at，UpdateProgress 检测到 progress 变化才刷新）。连续无进度变化超过 timeout 才判超时，
	// 非命令总时长——只要 heartbeat 一直上报新进度就不超时。
	TimeoutSeconds int         `gorm:"column:timeout_seconds" json:"timeoutSeconds"`
	LastProgressAt *types.Time `gorm:"column:last_progress_at" json:"lastProgressAt"`
}

func (ClientCommand) TableName() string { return "aa_client_command" }

// ClientLog 运行日志（心跳上行增量，落库审计）。
type ClientLog struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	LogId     string      `gorm:"column:log_id;uniqueIndex" json:"logId"`
	DeviceId  string      `gorm:"column:device_id;index" json:"deviceId"`
	Agent     string      `gorm:"column:agent" json:"agent"`
	CommandId string      `gorm:"column:command_id" json:"commandId"`
	Level     string      `gorm:"column:level" json:"level"`
	Line      string      `gorm:"column:line" json:"line"`
	Ts        *types.Time `gorm:"column:ts" json:"ts"`
}

func (ClientLog) TableName() string { return "aa_client_log" }

// ClientConfigChange 配置文件变更（hash 变化时落库全文）。
type ClientConfigChange struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ChangeId    string      `gorm:"column:change_id;uniqueIndex" json:"changeId"`
	DeviceId    string      `gorm:"column:device_id;index" json:"deviceId"`
	Agent       string      `gorm:"column:agent" json:"agent"`
	ConfigHash  string      `gorm:"column:config_hash" json:"configHash"`
	ConfigSize  int64       `gorm:"column:config_size" json:"configSize"`
	ConfigMtime *types.Time `gorm:"column:config_mtime" json:"configMtime"`
	Content     string      `gorm:"column:content" json:"content"`
	// 变更来源归因（落库时判定）：
	//   admin_push / admin_install / admin_restart / admin_start / admin_plugin —— 后台命令写入
	//     （config_push/install/restart/start 携带 configContent；plugin_install 合并 channels）；
	//   llm_model —— 差异仅落在 models.*（可用模型清单变化触发）；
	//   local_change —— 无后台命令关联，本机 openclaw/客户端自行写入。
	Source       string      `gorm:"column:source" json:"source"`
	SourceDetail string      `gorm:"column:source_detail" json:"sourceDetail"`
	ChangedAt    *types.Time `gorm:"column:changed_at" json:"changedAt"`
}

func (ClientConfigChange) TableName() string { return "aa_client_config_change" }

// ClientApp App 目录：管理员可选的自动化目标 App 字典（macOS bundle id + emoji logo，
// 种子预置常用 App；供设备 App 权限配置勾选）。
type ClientApp struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AppId     string      `gorm:"column:app_id;uniqueIndex" json:"appId"`
	BundleId  string      `gorm:"column:bundle_id;uniqueIndex" json:"bundleId"` // macOS bundle id，如 com.googlecode.iterm2
	AppName   string      `gorm:"column:app_name" json:"appName"`
	Category  string      `gorm:"column:category" json:"category"` // 类别 key（comm/docs/tools/browser/ai/system；前端 i18n 渲染）
	Logo      string      `gorm:"column:logo" json:"logo"`         // emoji logo（与 aa_menu.icon 同风格）
	Weigh     int         `gorm:"column:weigh" json:"weigh"`       // 排序，大者靠前
	Remarks   string      `gorm:"column:remarks" json:"remarks"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClientApp) TableName() string { return "aa_client_app" }

// ClientAppPerm 设备 App 权限配置：某设备要授权自动化权限（AppleEvents TCC）的 App。
// 存在即启用；后台整组覆盖式保存，变更写 aa_client_config_change（source=app_perm）。
// Granted/Denied 是客户端上报的事实（心跳 perm_state 快照落库），后台保存清单不动它们。
type ClientAppPerm struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PermId    string      `gorm:"column:perm_id;uniqueIndex" json:"permId"`
	DeviceId  string      `gorm:"column:device_id;uniqueIndex:idx_app_perm_dev_bundle" json:"deviceId"` // 软引用 aa_client_device
	BundleId  string      `gorm:"column:bundle_id;uniqueIndex:idx_app_perm_dev_bundle" json:"bundleId"` // 软引用 aa_client_app.bundle_id
	Granted   bool        `gorm:"column:granted" json:"granted"`                                        // 已授权（探测成功/事件已送达）
	Denied    bool        `gorm:"column:denied" json:"denied"`                                          // 已拒绝（TCC 缓存拒绝 -1743）
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClientAppPerm) TableName() string { return "aa_client_app_perm" }

// ClientChannelState 渠道绑定状态（不落独立表，作为 aa_client_device.channels jsonb 数组元素）。
// 后台启用 → 写入 channels + 下发 plugin_install/restart → openclaw.json 合并 plugins.entries/channels；
// App 端 channel_login 通过 openclaw gateway call web.login.start 拿 qrDataUrl 回传；
// 登录成功 loginStatus=connected、accountAlias 填微信昵称，qrDataUrl 清空。
type ClientChannelState struct {
	Channel      string `json:"channel"`      // openclaw-weixin
	AccountId    string `json:"accountId"`    // 默认 default
	Enabled      bool   `json:"enabled"`      // 插件是否启用
	DmPolicy     string `json:"dmPolicy"`     // pairing/allowlist/open/disabled
	GroupPolicy  string `json:"groupPolicy"`  // allowlist/open/disabled
	LoginStatus  string `json:"loginStatus"`  // idle/qr_pending/connected/disconnected/failed
	AccountAlias string `json:"accountAlias"` // 登录后微信昵称
	QrDataUrl    string `json:"qrDataUrl"`    // 扫码期 QR data URL（登录成功后清空）
	LastMessage  string `json:"lastMessage"`  // 最近登录/状态消息
}

// ClientSkillState 设备已装技能快照（不落独立表，作为 aa_client_device.skills jsonb 数组元素）。
// App 心跳扫描 ~/.openclaw/skills（openclaw 全局托管目录）解析 SKILL.md frontmatter 上报；
// 手工装进该目录的技能同样会被扫到。列表只认这份设备真实状态，不做管理端关联表。
type ClientSkillState struct {
	Name        string `json:"name"`        // 技能名（frontmatter name，缺省目录名）
	Slug        string `json:"slug"`        // 安装目录名（安装时 --as 指定的 slug）
	Description string `json:"description"` // frontmatter 描述
	Version     string `json:"version"`     // frontmatter 版本
}

// DeviceSkillView 已装技能列表行视图：设备快照 + 技能库对照信息。
// 库内无此 slug（手工安装/库已删）时对照字段为空，前端不展示「库最新/升级」。
type DeviceSkillView struct {
	ClientSkillState
	SkillId         string `json:"skillId,omitempty"`         // 技能库 skill_id（按 slug 回查）
	LatestVersionId string `json:"latestVersionId,omitempty"` // 库最新版本 id（解析口径同 install）
	LatestVersion   string `json:"latestVersion,omitempty"`   // 库最新版本号（版本是自由文本，仅做 ≠ 比较不猜大小）
}
