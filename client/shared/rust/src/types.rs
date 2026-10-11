//! 与后端 `/api/v1/system/client/*` DTO 对齐的类型。

use serde::{Deserialize, Serialize};

/// 后端统一响应外壳：`{ code, data, message }`。code=0 成功。
#[derive(Debug, Deserialize)]
pub struct ApiEnvelope<T> {
    pub code: i32,
    pub data: Option<T>,
    pub message: String,
}

/// exchange 响应：status=ok 时附带 token + 用户信息；pending 表示尚未绑定。
#[derive(Debug, Deserialize, Serialize)]
pub struct ExchangeRes {
    pub status: String, // "ok" | "pending"
    #[serde(default)]
    pub token: String,
    #[serde(default, rename = "userId")]
    pub user_id: String,
    #[serde(default, rename = "userName")]
    pub user_name: String,
    #[serde(default)]
    pub name: String, // 真实姓名（aa_user.name）
    #[serde(default, rename = "employeeNo")]
    pub employee_no: String, // 工号（aa_user.employee_no）
    #[serde(default, rename = "isAdmin")]
    pub is_admin: i32,
}

/// agent 的单个版本规格（每版本各自安装+配置）。
/// 字段对齐后端 ClientAgentVersionVO，全部可选以容忍后端 jsonb 差异。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct AgentVersion {
    #[serde(default, rename = "versionId")]
    pub version_id: String,
    #[serde(default)]
    pub version: String,
    #[serde(default, rename = "isDefault")]
    pub is_default: bool,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default, rename = "downloadUrl")]
    pub download_url: String,
    #[serde(default, rename = "containerImage")]
    pub container_image: String,
    #[serde(default)]
    pub channel: String,
    #[serde(default, rename = "configTemplate")]
    pub config_template: serde_json::Value,
    #[serde(default, rename = "envVars")]
    pub env_vars: serde_json::Value,
    #[serde(default)]
    pub extra: serde_json::Value,
    /// 已缓存的安装包（按平台）。App 优先从本服务下载，不直连官网。
    #[serde(default)]
    pub packages: Vec<ClientPackageRef>,
}

/// 版本下已缓存安装包引用。App 据此构造下载 URL：
/// `{base_url}/api/v1/system/client/packages/{agent}/{version}/{platform}`
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct ClientPackageRef {
    #[serde(default)]
    pub platform: String,
    #[serde(default)]
    pub filename: String,
    #[serde(default, rename = "fileSize")]
    pub file_size: i64,
    #[serde(default)]
    pub sha256: String,
}

/// 远程指令（管理员下发，App 认领执行）。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct ClientCommand {
    #[serde(default, rename = "commandId")]
    pub command_id: String,
    #[serde(default, rename = "deviceId")]
    pub device_id: String,
    #[serde(default)]
    pub action: String, // install/uninstall/start/stop/restart/config_push/collect_data/appperm_reset
    #[serde(default)]
    pub agent: String,
    #[serde(default)]
    pub version: String,
    #[serde(default)]
    pub payload: serde_json::Value,
    #[serde(default)]
    pub status: String,
}

// ===== 心跳通道（状态+日志+配置变更+命令进度合一） =====

/// 单条日志（增量上报）。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct LogEntry {
    #[serde(default)]
    pub line: String,
    #[serde(default)]
    pub level: String,
    #[serde(default)]
    pub ts: String,
}

/// 当前执行中命令的进度/结果。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct RunningCommandReport {
    #[serde(default, rename = "commandId")]
    pub command_id: String,
    #[serde(default)]
    pub action: String, // install/uninstall/start/stop/restart/config_push/appperm_reset
    #[serde(default)]
    pub status: String, // running/done/failed
    #[serde(default)]
    pub progress: String,
    #[serde(default)]
    pub percent: i32,
    #[serde(default, rename = "lastLine")]
    pub last_line: String,
    #[serde(default)]
    pub result: String,
    #[serde(default)]
    pub error: String,
}

/// 渠道登录进行中状态（App 端 channel_login 命令持有，心跳读取回传后端）。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct ChannelLoginState {
    #[serde(default, rename = "channel")]
    pub channel: String,
    #[serde(default, rename = "loginStatus")]
    pub login_status: String, // qr_pending/connected/disconnected/failed
    #[serde(default, rename = "qrDataUrl")]
    pub qr_data_url: String,
    #[serde(default, rename = "accountAlias")]
    pub account_alias: String,
    #[serde(default)]
    pub message: String,
}

/// 心跳上报的单个渠道状态（对应后端 ChannelStateReport）。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct ChannelStateReport {
    #[serde(default, rename = "channel")]
    pub channel: String,
    #[serde(default, rename = "accountId")]
    pub account_id: String,
    #[serde(default, rename = "loginStatus")]
    pub login_status: String,
    #[serde(default, rename = "qrDataUrl")]
    pub qr_data_url: String,
    #[serde(default, rename = "accountAlias")]
    pub account_alias: String,
    #[serde(default)]
    pub message: String,
}

/// 单客户端模块上报。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct ModuleReport {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub version: String,
    #[serde(default, rename = "pidOrContainerId")]
    pub pid_or_container_id: String,
    #[serde(default, rename = "uptimeSeconds")]
    pub uptime_seconds: i64,
    #[serde(default, rename = "lastError")]
    pub last_error: String,
    #[serde(default)]
    pub logs: Vec<LogEntry>,
    #[serde(default, rename = "configHash")]
    pub config_hash: String,
    #[serde(default, rename = "configSize")]
    pub config_size: i64,
    #[serde(default, rename = "configMtime")]
    pub config_mtime: String,
    #[serde(default, rename = "configContent")]
    pub config_content: String,
    #[serde(default, rename = "runningCommand")]
    pub running_command: Option<RunningCommandReport>,
    /// 环境探测（当前仅 openclaw 用）：本机已有安装的多来源版本 + 是否需要版本对齐。
    /// 服务端合进状态行 extra.env 展示。None = 该模块不探测（不序列化）。
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<serde_json::Value>,
}

/// 心跳请求。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct HeartbeatReq {
    #[serde(default, rename = "deviceId")]
    pub device_id: String,
    #[serde(default)]
    pub modules: std::collections::HashMap<String, ModuleReport>,
    /// 该设备各渠道登录/连接状态（含扫码期 QR），对应后端 ChannelStateReport。
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub channels: Vec<ChannelStateReport>,
    /// App 授权状态快照（已授权/已拒绝 bundle id 集）。None=与上次已发一致不带；
    /// 变更后或启动首跳 Some，发送成功由引擎 mark_uploaded 收口。
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub perm_state: Option<PermStateReport>,
    /// 已装技能快照（~/.openclaw/skills 扫描，含手装）。不用 skip_serializing_if：
    /// 空数组也要发 []——后端靠 nil（旧客户端未带字段，保留存量）与空数组（确实没装，清空）区分。
    #[serde(default)]
    pub skills: Vec<SkillStateReport>,
}

/// 设备已装技能快照项（对应后端 ClientSkillState）：扫 ~/.openclaw/skills 各技能
/// SKILL.md frontmatter（name/description/version）；slug=托管目录下的目录名。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct SkillStateReport {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub slug: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub version: String,
}

/// App 授权状态上报（aa.json granted_perms/denied_perms 的镜像，变更时主动上报）。
#[derive(Debug, Serialize, Deserialize, Clone, Default, PartialEq)]
pub struct PermStateReport {
    #[serde(default, rename = "granted_perms")]
    pub granted_perms: Vec<String>,
    #[serde(default, rename = "denied_perms")]
    pub denied_perms: Vec<String>,
}

/// 心跳响应。
#[derive(Debug, Deserialize, Default)]
pub struct HeartbeatRes {
    #[serde(default)]
    pub stored: bool,
    #[serde(default)]
    pub commands: Vec<ClientCommand>,
    /// 该设备操作过的 agent：App 据此补全本地已激活集合（事实来源在服务端命令历史）
    #[serde(default, rename = "activatedAgents")]
    pub activated_agents: Vec<String>,
    /// 该设备配置的 macOS 自动化授权 App 清单（管理后台「App 权限」按设备配置）。
    /// 空=未配置（客户端仅内置 System Events 授权）；旧服务端无此字段同样得到空。
    /// alias 兼容旧服务端的 camelCase 字段。
    #[serde(default, rename = "app_perms", alias = "appPerms")]
    pub app_perms: Vec<AppPerm>,
}

/// 服务端下发的授权 App（bundle_id 喂 osascript `application id`，app_name 仅供日志展示）。
#[derive(Debug, Deserialize, Serialize, Clone, Default, PartialEq)]
pub struct AppPerm {
    #[serde(default, rename = "bundle_id", alias = "bundleId")]
    pub bundle_id: String,
    #[serde(default, rename = "app_name", alias = "appName")]
    pub app_name: String,
}

/// 可安装 agent（类型+介绍）+ 其版本列表。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct Agent {
    #[serde(default, rename = "agentId")]
    pub agent_id: String,
    #[serde(default)]
    pub key: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub intro: String,
    #[serde(default)]
    pub icon: String,
    /// Logo 图片 URL（空=客户端回退打包 logo 或 emoji icon）。
    #[serde(default)]
    pub logo: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub weigh: i64,
    #[serde(default)]
    pub versions: Vec<AgentVersion>,
}

#[derive(Debug, Deserialize, Serialize, Default)]
pub struct AgentsRes {
    #[serde(default)]
    pub list: Vec<Agent>,
}

/// App 采集上报的设备信息（exchange 时携带，后端落 aa_client_device）。
/// 字段对齐后端 DeviceInfo（camelCase）。
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct DeviceInfo {
    #[serde(rename = "deviceId")]
    pub device_id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub platform: String,
    #[serde(default)]
    pub os: String,
    #[serde(default)]
    pub arch: String,
    #[serde(default, rename = "appVersion")]
    pub app_version: String,
    #[serde(default, rename = "macAddress")]
    pub mac_address: String,
    #[serde(default, rename = "macExtra")]
    pub mac_extra: Vec<String>,
    #[serde(default)]
    pub ip: String,
}

/// 当前环境与登录态信息（供前端展示）。
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct EnvInfo {
    pub env: String, // "test" | "prod"
    #[serde(rename = "baseUrl")]
    pub base_url: String,
    #[serde(rename = "loggedIn")]
    pub logged_in: bool,
    #[serde(rename = "userId")]
    pub user_id: String,
    #[serde(rename = "userName")]
    pub user_name: String,
    #[serde(default)]
    pub name: String, // 真实姓名（aa_user.name）
    #[serde(default, rename = "employeeNo")]
    pub employee_no: String, // 工号（aa_user.employee_no）
}

// ===== MCP 分组自助管理（设备 token 端点 /mcp，桌面客户端 MCP 管理模块） =====

/// 可绑分组选项（服务端已按当前用户权限过滤，仅 id+名）。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct McpGroupOption {
    #[serde(default, rename = "groupId")]
    pub group_id: String,
    #[serde(default)]
    pub name: String,
}

/// 已绑分组的 server 概览（name+alias 即可；tools 等裁剪配置不下发客户端）。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct McpGroupServer {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub alias: String,
}

/// 已绑分组行（canUnbind=当前用户可解：可使用该组 ∨ 本人绑的）。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct McpBoundGroup {
    #[serde(default, rename = "groupId")]
    pub group_id: String,
    #[serde(default, rename = "groupName")]
    pub group_name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub servers: Vec<McpGroupServer>,
    #[serde(default, rename = "canUnbind")]
    pub can_unbind: bool,
}

/// 设备自助 MCP 视图（一次返回 bound + options，省一次往返）。
#[derive(Debug, Deserialize, Serialize, Clone, Default)]
pub struct McpSelfView {
    #[serde(default)]
    pub bound: Vec<McpBoundGroup>,
    #[serde(default)]
    pub options: Vec<McpGroupOption>,
}
