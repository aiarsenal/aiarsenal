/**
 * gin LLM 错误日志接口（/api/v1/llm/error/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go（LLMErrorLogSearchReq / LLMErrorLogSearchRes）。
 *   - PageReq 公共字段：dateRange / pageNum / pageSize。
 *   - ListRes 公共字段：currentPage / total。
 * 实体源：api/domain/entity/llm.go LLMErrorLog（list 行业务 id = requestId）。
 * 控制器：api/interfaces/controller/llm_monitor.go（LLMMonitorController.ListErrorLogs）。
 */

import { requestClient } from '#/api/request';

export namespace LLMErrorApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    model?: string;
    apiKey?: string;
    dateRange?: string[];
  }

  export interface ListItem {
    requestId: string;
    startTime?: string;
    endTime?: string;
    apiBase?: string;
    modelGroup?: string;
    modelName?: string;
    modelId?: string;
    requestKwargs?: string;
    exceptionType?: string;
    exceptionString?: string;
    statusCode?: string;
    apiKey?: string;
    userId?: string;
    teamId?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }
}

const BASE = '/api/v1/llm/error';

/** 错误日志列表（gin: GET /error/list，query 参数） */
export function getLLMErrorListApi(params: LLMErrorApi.ListParams) {
  return requestClient.get<LLMErrorApi.ListResult>(`${BASE}/list`, {
    params,
  });
}
