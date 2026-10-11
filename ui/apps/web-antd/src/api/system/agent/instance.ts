/**
 * gin Agent 实例接口（/api/v1/agent/instance/*）— localdocker 真跑容器
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentInstanceApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    status?: number;
  }
  export interface InstanceItem {
    instanceId: string;
    userId?: string;
    name: string;
    description?: string;
    agent?: string;
    version?: string;
    status: number; // 0=creating 1=running 2=stopped 3=error
    cpuCores?: number;
    memoryGb?: number;
    diskGb?: number;
    imageTag?: string;
    containerId?: string;
    containerName?: string;
    hostPort?: number;
    llmKeyHash?: string; // 实例专属 LLM 密钥 tokenHash（创建时签发，agent 接 LLM 网关用）
    createdAt?: string;
    updatedAt?: string;
  }
  export interface ListResult {
    list: InstanceItem[];
    total: number;
    currentPage?: number;
  }
  export interface CreateReq {
    name: string;
    description?: string;
    agent: string;
    version: string;
    cpuCores?: number;
    memoryGb?: number;
    diskGb?: number;
    imageTag?: string;
    environmentOverridesJson?: string;
    mountPath?: string;
    ports?: { hostPort: number; containerPort: number }[];
  }
  export interface ExecResult {
    output: string;
  }
  export interface StatusResult {
    instanceID: string;
    state: number;
    containerID: string;
    ip: string;
    error: string;
  }
  // 容器内 manager（image/manager）单模块上报：对齐后端 AgentModuleReport。
  export interface RuntimeModuleReport {
    status?: string; // running/stopped/error/creating
    version?: string;
    pidOrContainerId?: string;
    uptimeSeconds?: number;
    lastError?: string;
    configHash?: string;
    configSize?: number;
    configMtime?: string;
    summary?: Record<string, any>;
  }
  // runtime-status 响应：summaryJson 是 manager 上报的原始 modules JSON 字符串。
  export interface RuntimeStatusResult {
    instanceId?: string;
    infraStatus?: number;
    agentStatus?: number;
    agentProcStatus?: number;
    agentProcPid?: string;
    agentProcVersion?: string;
    summaryJson?: string;
    healthJson?: string;
    lastReportedAt?: string;
    updatedAt?: string;
  }
  // 命令项（控制面下发 + 执行结果）。status: 0=pending 2=running 3=done 4=failed。
  export interface CommandItem {
    instanceCommandId: string;
    commandType?: string;
    agentId?: string;
    status?: number;
    payloadJson?: string;
    progressJson?: string; // 运行中进度（微信 QR 走这里：{step: <qrUrl>}）
    resultJson?: string;
    issuedBy?: string;
    issuedAt?: string;
  }
}

const BASE = '/api/v1/agent/instance';

export function getAgentInstanceListApi(params: AgentInstanceApi.ListParams) {
  return requestClient.get<AgentInstanceApi.ListResult>(`${BASE}/list`, {
    params,
  });
}
export function getAgentInstanceAllApi(params: AgentInstanceApi.ListParams) {
  return requestClient.get<AgentInstanceApi.ListResult>(
    `/api/v1/agent/instance/all`,
    { params },
  );
}
export function getAgentInstanceApi(id: string) {
  return requestClient.get<AgentInstanceApi.InstanceItem>(`${BASE}/get`, {
    params: { instanceId: id },
  });
}
export function addAgentInstanceApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function startAgentInstanceApi(id: string) {
  return requestClient.post(`${BASE}/${id}/start`);
}
export function stopAgentInstanceApi(id: string) {
  return requestClient.post(`${BASE}/${id}/stop`);
}
export function restartAgentInstanceApi(id: string) {
  return requestClient.post(`${BASE}/${id}/restart`);
}
export function deleteAgentInstanceApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}
export function statusAgentInstanceApi(id: string) {
  return requestClient.get<AgentInstanceApi.StatusResult>(
    `${BASE}/${id}/status`,
  );
}
// 取容器内 manager（image/manager）心跳上报的运行时状态（含 modules 原始数据）。
export function getAgentRuntimeStatusApi(id: string) {
  return requestClient.get<AgentInstanceApi.RuntimeStatusResult>(
    `${BASE}/${id}/runtime-status`,
  );
}
// 运行时状态历史时间线（状态随时间变化，心跳合并去重后的历史）。
export function getAgentRuntimeStatusHistoryApi(id: string, limit = 50) {
  return requestClient.get<any[]>(`${BASE}/${id}/runtime-status/history`, {
    params: { limit },
  });
}
// 配置变更历史（智能体 configHash 变化才落）。
export function getAgentInstanceConfigChangesApi(id: string, limit = 50) {
  return requestClient.get<any[]>(`${BASE}/${id}/config-changes`, {
    params: { limit },
  });
}
// 命令历史（控制面下发 + 执行结果）。
export function getAgentInstanceCommandsApi(id: string) {
  return requestClient.get<{ list: any[]; total: number }>(
    `${BASE}/${id}/command/list`,
    { params: { pageNum: 1, pageSize: 50 } },
  );
}
// 签发「打开桌面」的短期路径 token，返回同源反代 URL（前端拼 iframe.src）。
export function requestDesktopAccessApi(id: string) {
  return requestClient.post<{ url: string }>(`${BASE}/${id}/desktop/access`);
}
export function execAgentInstanceApi(id: string, cmd: string[]) {
  return requestClient.post<AgentInstanceApi.ExecResult>(
    `${BASE}/${id}/exec`,
    { cmd },
  );
}
// 轮换实例专属 LLM 密钥并重发 agent 运行配置（零停机，5 分钟宽限内切换）。
export function refreshAgentInstanceLlmKeyApi(id: string) {
  return requestClient.post(`${BASE}/${id}/refresh-llm-key`);
}
// 下发命令到容器内 manager（pending→心跳认领→执行）。
// channel_login 用它发起微信扫码登录：openclaw→{agentId:openclaw,channel:openclaw-weixin}；hermes→{agentId:hermes,channel:weixin}。
export function enqueueAgentInstanceCommandApi(
  id: string,
  body: {
    commandType: string;
    agentId?: string;
    payload?: Recordable<any>;
    timeoutSeconds?: number;
  },
) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/${id}/command/enqueue`,
    body,
  );
}

// ===== 实例技能管理 =====

export interface InstanceSkillItem {
  instanceSkillId: string;
  instanceId: string;
  skillId: string;
  slug: string;
  displayName: string;
  summary?: string;
  versionId?: string; // 实际装上的版本 id（install done 时落）；历史行为空=未知（视为可升级）
  version?: string; // 实际装上的版本号
  latestVersionId?: string; // 库最新版本 id（解析口径同安装）
  latestVersion?: string; // 库最新版本号（自由文本，仅做 ≠ 比较不猜大小）
  enabled: number;
  sortOrder: number;
  installedAt?: string;
}

// 列出实例已安装的 skill（富化 aa_skill 详情）。
export function getAgentInstanceSkillsApi(id: string) {
  return requestClient.get<InstanceSkillItem[]>(`${BASE}/${id}/skill/list`);
}

// 安装 skill：下发 install_skill（payload 带 https 取包链接，manager 自行下载解压安装）。
// versionId 空=装库最新；显式指定=升级/回滚到该版本。
export function installAgentInstanceSkillApi(
  id: string,
  skillId: string,
  versionId = '',
) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/${id}/skill/install`,
    { skillId, versionId },
  );
}

// 卸载 skill：下发 uninstall_skill（manager 定位 openclaw 托管目录并删除）。
export function uninstallAgentInstanceSkillApi(id: string, skillId: string) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/${id}/skill/uninstall`,
    { skillId },
  );
}

// ===== 实例 MCP 分组绑定（详情抽屉：绑/解绑自动签专属 key + 重推 openclaw 配置） =====

export interface InstanceMcpGroupItem {
  groupId: string;
  groupName: string;
  description?: string;
  source: 'direct' | 'project'; // direct=直绑 / project=来自项目绑定（只读）
  projectNames?: string[]; // source=project 时的来源项目名
  servers?: { name: string; alias?: string; tools?: unknown }[]; // 组内 server 概览
}

// 实例有效分组列表（直绑 ∪ 项目继承，带来源标注 + server 概览）。
export function getAgentInstanceMcpApi(id: string) {
  return requestClient.get<InstanceMcpGroupItem[]>(`${BASE}/${id}/mcp/list`);
}

// 绑定分组：签/收紧实例专属 MCP key，重推配置注入 mcp.servers。
export function bindAgentInstanceMcpApi(id: string, groupId: string) {
  return requestClient.post(`${BASE}/${id}/mcp/bind`, { groupId });
}

// 解绑直绑分组（项目继承的分组需在项目里解绑）。
export function unbindAgentInstanceMcpApi(id: string, groupId: string) {
  return requestClient.post(`${BASE}/${id}/mcp/unbind`, { groupId });
}
