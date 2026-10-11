/**
 * gin LLM 凭据接口（/api/v1/llm/credential/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go
 *   - LLMCredentialSearchReq / LLMCredentialSearchRes
 *   - LLMCredentialReq / LLMCredentialAddReq / LLMCredentialEditReq
 *   - LLMIdsReq（删除）
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace LLMCredentialApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    providerType?: string;
  }

  export interface ListItem {
    credentialId: string;
    credentialName: string;
    providerType: string;
    baseUrl?: string;
    credentialInfo?: string;
    status: number;
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

  /** 表单 payload（对应 gin LLMCredentialReq）。apiKey 明文，service 加密落库。 */
  export interface CredentialForm {
    credentialId?: string;
    credentialName: string;
    providerType: string;
    baseUrl?: string;
    apiKey?: string;
    credentialInfo?: string;
    status: number;
  }
}

const BASE = '/api/v1/llm/credential';

/** 凭据列表 */
export function getLLMCredentialListApi(
  params: LLMCredentialApi.ListParams,
) {
  return requestClient.get<LLMCredentialApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单个凭据 */
export function getLLMCredentialApi(credentialId: string) {
  return requestClient.get<LLMCredentialApi.ListItem>(`${BASE}/get`, {
    params: { credentialId },
  });
}

/** 新增 */
export function addLLMCredentialApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body 含 credentialId）*/
export function editLLMCredentialApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }）*/
export function deleteLLMCredentialApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 按凭据导入支持模型：调上游 /v1/models 批量写成模型部署。返回 {imported, skipped, models[]} */
export function discoverCredentialModelsApi(data: {
  credentialId: string;
  modelGroup?: string;
}) {
  return requestClient.post<{
    imported: number;
    skipped: number;
    models: string[];
  }>(`${BASE}/discover-models`, data);
}
