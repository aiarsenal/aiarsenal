/**
 * gin 桌面客户端配置接口（/api/v1/system/client/admin/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * agent（类型+介绍）+ 各 agent 的版本 CRUD。
 * 列表接口 GET /agent/list 返回 {list: ClientAgentVO[]}，
 * 每个 agent 的 versions 已内嵌（jsonb 字段解析为对象）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

const BASE = '/api/v1/system/client/admin';

export namespace ClientConfigApi {
  export interface AgentVersion {
    versionId: string;
    agentId: string;
    version: string;
    isDefault: boolean;
    enabled: boolean;
    downloadUrl: string;
    containerImage: string;
    channel: string;
    configTemplate: unknown;
    envVars: unknown;
    extra: unknown;
    packages?: { platform: string; filename: string; fileSize: number; sha256: string }[];
  }

  export interface Agent {
    agentId: string;
    key: string; // agent key（= agent 列）
    name: string;
    intro: string;
    icon: string;
    logo: string;
    enabled: boolean;
    weigh: number;
    versions: AgentVersion[];
  }

  /** 保存 agent 请求体（gin: ClientAgentSaveReq）*/
  export interface AgentSaveReq {
    agentId?: string; // 空=新建
    agent: string;
    name: string;
    intro?: string;
    icon?: string;
    logo?: string;
    enabled: number;
    weigh: number;
  }

  /** 保存版本请求体（gin: ClientAgentVersionSaveReq）*/
  export interface VersionSaveReq {
    versionId?: string; // 空=新建
    agentId: string;
    version: string;
    isDefault: number;
    enabled: number;
    downloadUrl?: string;
    containerImage?: string;
    channel?: string;
    configTemplate?: string;
    envVars?: string;
    extra?: string;
  }
}

/** agent 列表（含版本子项）*/
export function getClientAgentListApi() {
  return requestClient.get<{ list: ClientConfigApi.Agent[] }>(
    `${BASE}/agent/list`,
  );
}

/** 保存 agent（新建/编辑）*/
export function saveClientAgentApi(data: ClientConfigApi.AgentSaveReq) {
  return requestClient.put(`${BASE}/agent/save`, data);
}

/** 删除 agent（连带其全部版本）*/
export function deleteClientAgentApi(id: string) {
  return requestClient.delete(`${BASE}/agent/delete`, { data: { id } });
}

/** 保存版本（新建/编辑）*/
export function saveClientAgentVersionApi(
  data: ClientConfigApi.VersionSaveReq | Recordable<any>,
) {
  return requestClient.put(`${BASE}/version/save`, data);
}

/** 删除版本 */
export function deleteClientAgentVersionApi(id: string) {
  return requestClient.delete(`${BASE}/version/delete`, { data: { id } });
}
