// 与 Rust 侧 / 后端 DTO 对齐的前端类型。

export interface EnvInfo {
  env: 'test' | 'prod';
  baseUrl: string;
  loggedIn: boolean;
  userId: string;
  userName: string;
  name?: string;
  employeeNo?: string;
}

export interface ExchangeRes {
  status: 'ok' | 'pending';
  token?: string;
  userId?: string;
  userName?: string;
  name?: string;
  employeeNo?: string;
  isAdmin?: number;
}

export interface AgentVersion {
  versionId: string;
  version: string;
  isDefault: boolean;
  enabled: boolean;
  downloadUrl?: string;
  containerImage?: string;
  channel?: string;
  configTemplate?: unknown;
  envVars?: Record<string, string>;
  extra?: unknown;
  packages?: AgentPackageRef[];
}

export interface AgentPackageRef {
  platform: string;
  filename: string;
  fileSize: number;
  sha256: string;
}

export interface Agent {
  agentId: string;
  key: string;
  name: string;
  intro: string;
  icon: string;
  /** Logo 图片 URL（空=回退打包 logo 或 emoji icon） */
  logo?: string;
  enabled: boolean;
  weigh: number;
  versions: AgentVersion[];
}

export interface AgentsRes {
  list: Agent[];
}

export interface AgentOpResult {
  success: boolean;
  message: string;
  pidOrContainerId: string;
}

export interface AgentRuntime {
  agent: string;
  installed: boolean;
  running: boolean;
  version: string;
  pidOrContainerId: string;
}

export interface ConfigResult {
  success: boolean;
  content: string;
  path: string;
  message: string;
}

// ===== MCP 分组自助管理（设备 token 端点 /mcp；服务端已按当前用户权限过滤） =====

export interface McpGroupOption {
  groupId: string;
  name: string;
}

export interface McpGroupServer {
  name: string;
  alias: string;
}

export interface McpBoundGroup {
  groupId: string;
  groupName: string;
  description: string;
  servers: McpGroupServer[];
  /** 当前用户可解绑（可使用该组 ∨ 本人绑定的行）；false 时隐藏解绑按钮 */
  canUnbind: boolean;
}

export interface McpSelfView {
  bound: McpBoundGroup[];
  options: McpGroupOption[];
}
