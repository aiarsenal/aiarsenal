/**
 * gin 桌面客户端设备管理接口（/api/v1/system/client/admin/device/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 字段来源：entity.ClientDevice / ClientAgentStatus / ClientCommand /
 *          ClientLog / ClientConfigChange（api/domain/entity/client.go）
 * DTO：api/interfaces/dto/client.go
 *   - ClientIdReq        { id }
 *   - ClientDeviceBlockReq { id, blocked }
 *   - ClientCommandEnqueueReq { deviceId, action, agent, version, payload }
 *
 * 端点（router.go clientAdmin 组，JWTAuth + CasbinAuth）：
 *   GET    /device/list                 → { list: ClientDevice[] }
 *   GET    /device/get?id=               → { device }
 *   GET    /device/detail?deviceId=      → { device, statuses: ClientAgentStatus[] }
 *   PUT    /device/revoke        body{id} → { revoked: true }
 *   PUT    /device/block         body{id, blocked} → { blocked }
 *   DELETE /device/delete        body{id} → { deleted: true }
 *   POST   /device/command       body{deviceId,action,agent,version,payload} → { commandId }
 *   GET    /device/commands/:deviceId    → { list: ClientCommand[] }
 *   GET    /device/logs?deviceId=&agent= → { list: ClientLog[] }
 *   GET    /device/config-changes?deviceId= → { list: ClientConfigChange[] }
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace ClientDeviceApi {
  /** 设备登录记录（aa_client_device）*/
  export interface Device {
    /** 业务 id（列表行 id）*/
    deviceId: string;
    userId: string;
    userName: string;
    /** 主机名/设备名 */
    name: string;
    /** win / mac */
    platform: string;
    os: string;
    arch: string;
    appVersion: string;
    macAddress: string;
    macExtra: string;
    ip: string;
    /** active / revoked */
    status: string;
    /** 0 / 1 */
    blocked: number;
    lastLoginAt: string;
    lastLoginIp: string;
    createdAt: string;
    updatedAt: string;
    extra: string;
  }

  /** 客户端运行状态（aa_client_agent_status）*/
  export interface AgentStatus {
    statusId: string;
    deviceId: string;
    userId: string;
    /** agent key；client=App 自身心跳 */
    agent: string;
    /** agent=agent 上报 | app=App 自身心跳（agent=client） */
    kind: string;
    /** running / stopped / installing / error / notinstalled */
    status: string;
    version: string;
    uptimeSeconds: number;
    lastError: string;
    extra: string;
    reportedAt: string;
  }

  /** 远程指令（aa_client_command）。payload/result/progress 是裸 JSON 对象（后端 json.RawMessage）*/
  export interface Command {
    commandId: string;
    deviceId: string;
    /** install/uninstall/start/stop/restart/config_push/collect_data */
    action: string;
    agent: string;
    version: string;
    payload: Record<string, any> | null;
    /** pending / running / done / failed */
    status: string;
    result: Record<string, any> | null;
    progress: Record<string, any> | null;
    error: string;
    createdBy: string;
    createdAt: string;
    startedAt: string;
    finishedAt: string;
  }

  /** 运行日志（aa_client_log）*/
  export interface Log {
    logId: string;
    deviceId: string;
    agent: string;
    commandId: string;
    level: string;
    line: string;
    ts: string;
  }

  /** 配置变更（aa_client_config_change）*/
  export interface ConfigChange {
    changeId: string;
    deviceId: string;
    agent: string;
    configHash: string;
    configSize: number;
    configMtime: string;
    content: string;
    /** 来源归因：admin_config_push/admin_install/admin_restart/admin_start/admin_plugin/llm_model/local_change */
    source?: string;
    /** 归因详情（操作人/命令 id/差异摘要）*/
    sourceDetail?: string;
    changedAt: string;
  }

  /** 远程指令请求体（ClientCommandEnqueueReq）。payload 是裸 JSON 对象（后端 json.RawMessage）*/
  export interface CommandReq {
    deviceId: string;
    action: string;
    agent?: string;
    version?: string;
    payload?: Record<string, any>;
  }

  export interface ListResult {
    list: Device[];
  }

  export interface DetailResult {
    device: Device;
    statuses: AgentStatus[];
    commandedAgents: string[];
  }

  /** 渠道绑定状态（aa_client_device.channels jsonb 数组元素）*/
  export interface Channel {
    channel: string; // openclaw-weixin
    accountId: string;
    enabled: boolean;
    dmPolicy: string;
    groupPolicy: string;
    loginStatus: string; // idle/qr_pending/connected/disconnected/failed
    accountAlias: string;
    qrDataUrl: string;
    lastMessage: string;
  }

  /** 已装技能快照（aa_client_device.skills jsonb 数组元素，客户端扫 ~/.openclaw/skills 上报）；
   * skillId/latestVersion* 为服务端按 slug 回查技能库附的对照信息（库内无此 slug 时为空，如手装技能）*/
  export interface Skill {
    name: string; // frontmatter name，缺省目录名
    slug: string; // 托管目录下的目录名（安装时 --as 指定）
    description: string;
    version: string;
    skillId?: string;
    latestVersionId?: string;
    latestVersion?: string; // 库最新版本号（自由文本，仅做 ≠ 比较不猜大小）
  }
}

const BASE = '/api/v1/system/client/admin/device';

/** 设备列表（gin 不分页，直接返回全部）*/
export function getClientDeviceListApi() {
  return requestClient.get<ClientDeviceApi.ListResult>(`${BASE}/list`);
}

/** 单个设备 */
export function getClientDeviceApi(id: string) {
  return requestClient.get<{ device: ClientDeviceApi.Device }>(
    `${BASE}/get`,
    { params: { id } },
  );
}

/** 设备详情（设备信息 + 三客户端运行状态）*/
export function getClientDeviceDetailApi(deviceId: string) {
  return requestClient.get<ClientDeviceApi.DetailResult>(
    `${BASE}/detail`,
    { params: { deviceId } },
  );
}

/** 撤销登录态（body: { id }）*/
export function revokeClientDeviceApi(id: string) {
  return requestClient.put(`${BASE}/revoke`, { id });
}

/** 封禁/解封（body: { id, blocked }）*/
export function blockClientDeviceApi(id: string, blocked: number) {
  return requestClient.put(`${BASE}/block`, { id, blocked });
}

/** 删除设备（body: { id }）*/
export function deleteClientDeviceApi(id: string) {
  return requestClient.delete(`${BASE}/delete`, { data: { id } });
}

/** 下发远程指令（body: CommandReq）→ { commandId } */
export function enqueueClientCommandApi(data: ClientDeviceApi.CommandReq) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/command`,
    data as Recordable<any>,
  );
}

/** 设置某设备选中的 agent（App 心跳据此上报；下发指令 tab 也只显示这些）→ { ok } */
export function setClientDeviceSelectedAgentsApi(
  deviceId: string,
  agents: string[],
) {
  return requestClient.put<{ ok: boolean }>(`${BASE}/selected-agents`, {
    deviceId,
    agents,
  });
}

/** 指令历史（path: deviceId）→ { list: Command[] } */
export function getClientDeviceCommandsApi(deviceId: string) {
  return requestClient.get<{ list: ClientDeviceApi.Command[] }>(
    `${BASE}/commands/${encodeURIComponent(deviceId)}`,
  );
}

/** 运行日志（query: deviceId, agent）→ { list: Log[] }；commandId 非空时返回该命令的分步日志（正序） */
export function getClientDeviceLogsApi(
  deviceId: string,
  agent = '',
  limit = 200,
  commandId = '',
) {
  return requestClient.get<{ list: ClientDeviceApi.Log[] }>(`${BASE}/logs`, {
    params: commandId ? { commandId, limit } : { deviceId, agent, limit },
  });
}

/** 配置变更历史（query: deviceId）→ { list: ConfigChange[] } */
export function getClientDeviceConfigChangesApi(deviceId: string) {
  return requestClient.get<{ list: ClientDeviceApi.ConfigChange[] }>(
    `${BASE}/config-changes`,
    { params: { deviceId } },
  );
}

/**
 * 该设备 openclaw 最新配置预览：库中存的配置主体（含真实 apiKey）+ aa_llm_model 当前
 * 活跃模型集，2 空格美化。供「手动下发配置」输入框预填。无配置时 config 为空串。
 *   GET /device/openclaw-config?deviceId= → { config: string }
 */
export function getClientOpenclawConfigApi(deviceId: string) {
  return requestClient.get<{ config: string }>(`${BASE}/openclaw-config`, {
    params: { deviceId },
  });
}

/** 某设备全部客户端状态历史（时间线）→ { list: AgentStatus[] }；设备页 App 上下线历史用 */
export function getClientDeviceStatusHistoryApi(deviceId: string) {
  return requestClient.get<{ list: ClientDeviceApi.AgentStatus[] }>(
    `${BASE}/status-history`,
    { params: { deviceId } },
  );
}

// ===== 渠道绑定（微信等） =====

/** 某设备全部渠道绑定状态 → { list: Channel[] } */
export function getClientDeviceChannelsApi(deviceId: string) {
  return requestClient.get<{ list: ClientDeviceApi.Channel[] }>(
    `${BASE}/channels`,
    { params: { deviceId } },
  );
}

/** 启用/禁用某渠道 */
export function setClientDeviceChannelApi(
  deviceId: string,
  channel: string,
  enabled: boolean,
  dmPolicy = 'pairing',
  groupPolicy = 'disabled',
) {
  return requestClient.put<{ ok: boolean }>(`${BASE}/channel`, {
    deviceId,
    channel,
    enabled,
    dmPolicy,
    groupPolicy,
  });
}

/** 取某渠道当前 QR/登录态（管理后台扫码用） */
export function getClientDeviceChannelQrApi(deviceId: string, channel: string) {
  return requestClient.get<ClientDeviceApi.Channel>(`${BASE}/channel/qr`, {
    params: { deviceId, channel },
  });
}

/** 下发渠道扫码登录命令 → { commandId } */
export function loginClientDeviceChannelApi(deviceId: string, channel: string) {
  return requestClient.post<{ commandId: string }>(`${BASE}/channel/login`, {
    deviceId,
    channel,
  });
}

/** 下发渠道登出命令 → { commandId } */
export function logoutClientDeviceChannelApi(deviceId: string, channel: string) {
  return requestClient.post<{ commandId: string }>(`${BASE}/channel/logout`, {
    deviceId,
    channel,
  });
}

/** 解绑渠道：禁用 + logout（清账号）+ restart → { ok } */
export function unbindClientDeviceChannelApi(deviceId: string, channel: string) {
  return requestClient.post<{ ok: boolean }>(`${BASE}/channel/unbind`, {
    deviceId,
    channel,
  });
}

/** 下发装渠道插件命令（spec 如 @tencent-weixin/openclaw-weixin）→ { commandId } */
export function installPluginClientDeviceApi(deviceId: string, spec: string) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/channel/install-plugin`,
    { deviceId, spec },
  );
}

// ===== 技能管理（装/卸走远程指令队列，列表读心跳上报的 skills 快照） =====

/** 某设备已装技能列表 → { list: Skill[] } */
export function getClientDeviceSkillsApi(deviceId: string) {
  return requestClient.get<{ list: ClientDeviceApi.Skill[] }>(
    `${BASE}/skills`,
    { params: { deviceId } },
  );
}

/** 下发安装技能命令（versionId 空=自动解析最新可用版本）→ { commandId } */
export function installClientDeviceSkillApi(
  deviceId: string,
  skillId: string,
  versionId = '',
) {
  return requestClient.post<{ commandId: string }>(`${BASE}/skill/install`, {
    deviceId,
    skillId,
    versionId,
  });
}

/** 下发卸载技能命令（按装进设备托管目录的 slug）→ { commandId } */
export function uninstallClientDeviceSkillApi(deviceId: string, slug: string) {
  return requestClient.post<{ commandId: string }>(`${BASE}/skill/uninstall`, {
    deviceId,
    slug,
  });
}

// ===== 设备 MCP 分组绑定（设备详情：绑/解绑自动签专属 key + 尝试 restart 生效） =====

export interface DeviceMcpGroupItem {
  groupId: string;
  groupName: string;
  description?: string;
  servers?: { name: string; alias?: string; tools?: unknown }[]; // 组内 server 概览
}

/** 设备已绑 MCP 分组列表 */
export function getClientDeviceMcpApi(deviceId: string) {
  return requestClient.get<DeviceMcpGroupItem[]>(`${BASE}/mcp`, {
    params: { deviceId },
  });
}

/** 绑定 MCP 分组（设备已装 openclaw 时自动 enqueue restart 让配置重建带上 mcp.servers） */
export function bindClientDeviceMcpApi(deviceId: string, groupId: string) {
  return requestClient.post(`${BASE}/mcp/bind`, { deviceId, groupId });
}

/** 解绑 MCP 分组 */
export function unbindClientDeviceMcpApi(deviceId: string, groupId: string) {
  return requestClient.post(`${BASE}/mcp/unbind`, { deviceId, groupId });
}
