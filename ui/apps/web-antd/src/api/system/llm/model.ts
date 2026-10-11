/**
 * gin LLM 模型部署接口（/api/v1/llm/model/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go（LLMModelSearchReq / LLMModelReq / LLMModelEditReq / LLMIdsReq）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace LLMModelApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    provider?: string;
    group?: string;
    status?: number;
  }

  export interface ListItem {
    id?: number;
    modelId: string;
    modelName: string;
    modelGroup: string;
    provider: string;
    providerModel: string;
    credentialId?: string;
    extraParams?: string;
    modelInfo?: string;
    /** 输入单价（$/M，后端毫/百万 tokens JSON 边界自动换算） */
    inputPricePerMillion: number;
    outputPricePerMillion: number;
    cacheReadPricePerMillion?: number;
    cacheCreationPricePerMillion?: number;
    maxInputTokens?: number;
    maxOutputTokens?: number;
    tpmLimit?: number;
    rpmLimit?: number;
    weight: number;
    blocked: number;
    status: number;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 表单 payload（对应 gin LLMModelReq）。价格字段单位 $/M（inputPricePerMillion 等）。 */
  export interface ModelForm {
    modelId?: string;
    modelName: string;
    modelGroup: string;
    provider: string;
    providerModel: string;
    credentialId?: string;
    extraParams?: string;
    modelInfo?: string;
    inputPricePerMillion: number;
    outputPricePerMillion: number;
    cacheReadPricePerMillion?: number;
    cacheCreationPricePerMillion?: number;
    maxInputTokens?: number;
    maxOutputTokens?: number;
    tpmLimit?: number;
    rpmLimit?: number;
    weight: number;
    blocked?: number;
    status: number;
  }
}

const BASE = '/api/v1/llm/model';

/** 模型列表 */
export function getLLMModelListApi(params: LLMModelApi.ListParams) {
  return requestClient.get<LLMModelApi.ListResult>(`${BASE}/list`, { params });
}

/** 全部模型（不分页，gin: {list}）*/
export function getAllLLMModelsApi() {
  return requestClient.get<{ list: LLMModelApi.ListItem[] }>(`${BASE}/all`);
}

/** 单个模型 */
export function getLLMModelApi(modelId: string) {
  return requestClient.get<LLMModelApi.ListItem>(`${BASE}/get`, {
    params: { modelId },
  });
}

/** 新增 */
export function addLLMModelApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body 含 modelId）*/
export function editLLMModelApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }）*/
export function deleteLLMModelApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/**
 * 凭据下拉数据（复用凭据列表端点，取 list 字段）。
 * gin: GET /api/v1/llm/credential/list → { list, total }
 */
export function getLLMCredentialOptionsApi() {
  return requestClient.get<{ list: any[]; total: number }>(
    '/api/v1/llm/credential/list',
    { params: { pageNum: 1, pageSize: 200 } },
  );
}
