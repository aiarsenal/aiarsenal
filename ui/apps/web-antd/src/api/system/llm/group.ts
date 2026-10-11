/**
 * gin LLM 模型分组接口（/api/v1/llm/group/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go
 *   - LLMModelGroupSearchReq / LLMModelGroupSearchRes
 *   - LLMModelGroupReq / LLMModelGroupAddReq / LLMModelGroupEditReq
 *   - LLMIdsReq（删除）
 * 分组为字典：group_name 即 aa_llm_model.model_group 引用值（软引用）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace LLMModelGroupApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  export interface ListItem {
    id?: number;
    groupId: string;
    groupName: string;
    description?: string;
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

  /** 表单 payload（对应 gin LLMModelGroupReq）。 */
  export interface GroupForm {
    groupId?: string;
    groupName: string;
    description?: string;
    status: number;
  }
}

const BASE = '/api/v1/llm/group';

/** 分组列表 */
export function getLLMGroupListApi(params: LLMModelGroupApi.ListParams) {
  return requestClient.get<LLMModelGroupApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单个分组 */
export function getLLMGroupApi(groupId: string) {
  return requestClient.get<LLMModelGroupApi.ListItem>(`${BASE}/get`, {
    params: { groupId },
  });
}

/** 新增 */
export function addLLMGroupApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body 含 groupId）*/
export function editLLMGroupApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }）*/
export function deleteLLMGroupApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/**
 * 分组下拉数据（复用分组列表端点，取 list 字段）。
 * gin: GET /api/v1/llm/group/list → { list, total }
 */
export function getLLMGroupOptionsApi() {
  return requestClient.get<{ list: LLMModelGroupApi.ListItem[]; total: number }>(
    `${BASE}/list`,
    { params: { pageNum: 1, pageSize: 200 } },
  );
}
