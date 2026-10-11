/**
 * gin Agent 配置中心接口（/api/v1/agent/config/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段名以 gin DTO（api/interfaces/dto/agent.go）为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentConfigApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    resourceType?: string;
  }

  export interface ListItem {
    configResourceId: string;
    userId?: string;
    resourceType: string;
    resourceKey: string;
    name: string;
    description?: string;
    enabled: number;
    version: number;
    tagsJson?: string;
    contentJson?: string;
    createdBy?: string;
    updatedBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 提交体（add/edit 共用，edit 携带 configResourceId） */
  export interface ResourceReq {
    configResourceId?: string;
    resourceType: string;
    resourceKey?: string;
    name: string;
    description?: string;
    enabled: number;
    version: number;
    tagsJson?: string;
    contentJson?: string;
  }
}

const BASE = '/api/v1/agent/config/resource';

/** 资源列表 */
export function getAgentResourceListApi(params: AgentConfigApi.ListParams) {
  return requestClient.get<AgentConfigApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 资源详情 */
export function getAgentResourceApi(id: string) {
  return requestClient.get<AgentConfigApi.ListItem>(`${BASE}/get`, {
    params: { configResourceId: id },
  });
}

/** 新增资源 */
export function addAgentResourceApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑资源 */
export function editAgentResourceApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除资源（批量，body: { ids: [] }） */
export function deleteAgentResourceApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}
