// 对 Tauri Rust 命令的 invoke 封装（Win/Mac 共用同一套命令名）。
import { invoke } from '@tauri-apps/api/core';
import { listen, type UnlistenFn } from '@tauri-apps/api/event';
import type {
  Agent,
  AgentOpResult,
  AgentRuntime,
  ConfigResult,
  EnvInfo,
  ExchangeRes,
  McpSelfView,
} from './types';

export const clientStartLogin = () => invoke<string>('client_start_login');
export const clientExchange = (deviceCode: string) =>
  invoke<ExchangeRes>('client_exchange', { deviceCode });
export const clientGetAgents = () => invoke<Agent[]>('client_get_agents');
export const clientGetEnv = () => invoke<EnvInfo>('client_get_env');
export const clientLogout = () => invoke<void>('client_logout');
export const clientHideWindow = () => invoke<void>('client_hide_window');
export const openLoginPage = (deviceCode: string) =>
  invoke<void>('open_login_page', { deviceCode });
// 防锁屏开关
export const clientGetKeepAwake = () => invoke<boolean>('client_get_keep_awake');
export const clientSetKeepAwake = (enabled: boolean) =>
  invoke<boolean>('client_set_keep_awake', { enabled });

// MCP 分组自助管理（权限/配置重建都在服务端，端上只转发；win/mac 都注册）
export const clientMcpList = () => invoke<McpSelfView>('client_mcp_list');
export const clientMcpBind = (groupId: string) =>
  invoke<void>('client_mcp_bind', { groupId });
export const clientMcpUnbind = (groupId: string) =>
  invoke<void>('client_mcp_unbind', { groupId });

// 平台命令
export const installAgent = (agent: string, spec: unknown) =>
  invoke<AgentOpResult>('install_agent', { agent, spec });
export const startAgent = (agent: string, spec: unknown) =>
  invoke<AgentOpResult>('start_agent', { agent, spec });
export const stopAgent = (agent: string) => invoke<AgentOpResult>('stop_agent', { agent });
export const uninstallAgent = (agent: string) =>
  invoke<AgentOpResult>('uninstall_agent', { agent });
export const getAgentRuntime = (agent: string) =>
  invoke<AgentRuntime>('get_agent_runtime', { agent });
export const saveAgentConfig = (agent: string, content: string) =>
  invoke<ConfigResult>('save_agent_config', { agent, content });
export const readAgentConfig = (agent: string) =>
  invoke<ConfigResult>('read_agent_config', { agent });

// deep-link 事件：Rust 侧 emit("client://deep-link", dc)
export const onDeepLink = (cb: (dc: string) => void): Promise<UnlistenFn> =>
  listen<string>('client://deep-link', (e) => cb(e.payload));

// 安装进度日志事件：Rust 侧 emit("install-log", { agent, line })
export const onInstallLog = (
  cb: (agent: string, line: string) => void
): Promise<UnlistenFn> =>
  listen<{ agent: string; line: string }>('install-log', (e) =>
    cb(e.payload.agent, e.payload.line)
  );

// 强制登出事件：Rust 侧 emit("force-logout")（设备登录态被管理员 revoke，心跳收到 401）
export const onForceLogout = (cb: () => void): Promise<UnlistenFn> =>
  listen('force-logout', () => cb());

// openclaw 版本对齐确认：Rust 侧 emit("openclaw-align-confirm", payload)
// （install 检测到本机已有 openclaw 且版本 ≠ 目标，等待本机用户确认升/降级）
export interface OpenclawAlignFound {
  origin: string; // runtime | npm-global | app
  path: string;
  version: string;
}
export interface OpenclawAlignReq {
  target: string;
  current: string;
  action: string; // upgrade | downgrade | reinstall
  found: OpenclawAlignFound[];
  note?: string;
}
export const onOpenclawAlignConfirm = (cb: (p: OpenclawAlignReq) => void): Promise<UnlistenFn> =>
  listen<OpenclawAlignReq>('openclaw-align-confirm', (e) => cb(e.payload));
export const clientConfirmOpenclawAlign = (accept: boolean) =>
  invoke<boolean>('client_confirm_openclaw_align', { accept });

// 权限清单页（仅 mac / openclaw-desktop 构建注册；win 前端按 isMac 隐藏入口不调）
export interface PermStatus {
  key: string; // screen/accessibility/location/contacts/calendar/reminders/photos/camera/notifications
  state: 'granted' | 'notDetermined' | 'denied' | 'restricted' | 'unknown';
  interactiveSupported: boolean;
}
export const getPermissionStatuses = () =>
  invoke<PermStatus[]>('get_permission_statuses');
export const requestPermission = (key: string) =>
  invoke<string>('request_permission', { key });
export const openPermissionSettings = (key: string) =>
  invoke<void>('open_permission_settings', { key });
export const clientIsDevBuild = () => invoke<boolean>('client_is_dev_build');

// exec 审批：Rust 侧 emit("exec-approval-request", payload)（agent 经 system.run 要执行命令，
// 非 allowlist 命令且 ask 档位开启时弹窗等用户决策；60s 超时自动拒绝）
export interface ExecApprovalReq {
  id: string;
  argv: string[];
  commandText: string;
  cwd?: string;
  resolvedPath?: string;
  security: string;
  ask: string;
  expiresAt: number; // epoch ms
}
export const onExecApprovalRequest = (cb: (p: ExecApprovalReq) => void): Promise<UnlistenFn> =>
  listen<ExecApprovalReq>('exec-approval-request', (e) => cb(e.payload));
export const respondExecApproval = (id: string, decision: 'once' | 'always' | 'deny') =>
  invoke<boolean>('client_respond_exec_approval', { id, decision });

// exec 审批档位（Dashboard 卡片：full/ask/allowlist/deny）
export interface ExecPolicyEntry { pattern: string; source: string; lastUsedCommand?: string }
export interface ExecPolicy {
  mode: 'full' | 'ask' | 'allowlist' | 'deny';
  security: string;
  ask: string;
  path: string;
  exists: boolean;
  allowlistCount: number;
  allowlist: ExecPolicyEntry[];
}
export const clientGetExecPolicy = () => invoke<ExecPolicy>('client_get_exec_policy');
export const clientSetExecPolicy = (mode: string) => invoke<ExecPolicy>('client_set_exec_policy', { mode });
