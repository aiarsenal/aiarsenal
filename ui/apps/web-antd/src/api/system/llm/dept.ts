/**
 * gin LLM 部门配置接口（/api/v1/llm/dept/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go（LLMDeptSearchReq / LLMDeptSearchRes / LLMDeptReq / LLMDeptEditReq）。
 * 实体源：api/domain/entity/llm.go LLMDeptProfile（映射 aa_dept，list 行业务 id = deptId）。
 * 注意：dept 仅 list + get + edit，无 add/delete（部门本身由 system/dept 维护，此处只配 LLM 侧字段）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace LLMDeptApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  export interface ListItem {
    deptId: string;
    deptName: string;
    parentId?: string;
    ancestors?: string;
    orderNum?: number;
    leaderId?: string;
    status?: number;
    llmModels?: string;
    llmTpmLimit?: number;
    llmRpmLimit?: number;
    llmMaxParallelRequests?: number;
    llmBudgetDuration?: string;
    llmBudgetResetAt?: string;
    llmRouterSettings?: string;
    llmBlocked?: number;
    spend?: number;
    leaderName?: string;
    leaderEmpNo?: string;
    levelPath?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 编辑 payload（对应 gin LLMDeptEditReq = { deptId, ...LLMDeptReq }）。 */
  export interface DeptEditForm {
    deptId: string;
    llmModels?: string;
    llmTpmLimit?: number;
    llmRpmLimit?: number;
    llmMaxParallelRequests?: number;
    llmBudgetDuration?: string;
    llmRouterSettings?: string;
    llmBlocked?: number;
  }
}

const BASE = '/api/v1/llm/dept';

/** 部门 LLM 配置列表 */
export function getLLMDeptListApi(params: LLMDeptApi.ListParams) {
  return requestClient.get<LLMDeptApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单个部门 LLM 配置（gin: /get?deptId=）*/
export function getLLMDeptApi(deptId: string) {
  return requestClient.get<LLMDeptApi.ListItem>(`${BASE}/get`, {
    params: { deptId },
  });
}

/** 编辑部门 LLM 配置（body 含 deptId）*/
export function editLLMDeptApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
