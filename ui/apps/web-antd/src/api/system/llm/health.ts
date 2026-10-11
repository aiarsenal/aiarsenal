/**
 * gin LLM 健康检查接口（/api/v1/llm/health/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 仅只读列表，无 add/edit/delete 端点。
 */
import { requestClient } from '#/api/request';

export namespace LLMHealthApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    modelName?: string;
    status?: string;
  }

  export interface ListItem {
    healthCheckId: string;
    modelName: string;
    modelId: string;
    status: string;
    healthyCount: number;
    unhealthyCount: number;
    errorMessage: string;
    responseTimeMs?: number | null;
    details?: string;
    checkedBy?: string;
    checkedAt?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    currentPage: number;
    total: number;
  }
}

/** 健康检查列表（gin: GET /llm/health/list，query: modelName/status/pageNum/pageSize）*/
export function getLLMHealthListApi(params: LLMHealthApi.ListParams) {
  return requestClient.get<LLMHealthApi.ListResult>(
    '/api/v1/llm/health/list',
    { params },
  );
}
