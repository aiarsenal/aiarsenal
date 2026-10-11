/**
 * gin Agent 模型管理接口（/api/v1/agent/model/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 api/interfaces/dto/agent.go AgentModelReq / AgentModelSearchReq 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentModelApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    providerType?: string;
  }

  export interface ListItem {
    agentModelId: string;
    displayName: string;
    description?: string;
    providerType: string;
    protocolType?: string;
    baseUrl?: string;
    providerModelName: string;
    apiKeySecretRef?: string;
    isSecure?: number;
    isActive?: number;
    inputPrice?: number;
    outputPrice?: number;
    currency?: string;
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

  export interface ModelForm {
    agentModelId?: string;
    displayName: string;
    description?: string;
    providerType: string;
    protocolType?: string;
    baseUrl?: string;
    providerModelName: string;
    apiKey?: string;
    apiKeySecretRef?: string;
    isSecure?: number;
    isActive?: number;
    inputPrice?: number;
    outputPrice?: number;
    currency?: string;
  }
}

/** 模型列表 */
export function getAgentModelListApi(params: AgentModelApi.ListParams) {
  return requestClient.get<AgentModelApi.ListResult>(
    '/api/v1/agent/model/list',
    { params },
  );
}

/** 详情 */
export function getAgentModelApi(id: string) {
  return requestClient.get<AgentModelApi.ListItem>(
    '/api/v1/agent/model/get',
    { params: { agentModelId: id } },
  );
}

/** 新增 */
export function addAgentModelApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/agent/model/add', data);
}

/** 编辑 */
export function editAgentModelApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/agent/model/edit', data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteAgentModelApi(ids: string[]) {
  return requestClient.delete('/api/v1/agent/model/delete', {
    data: { ids },
  });
}
