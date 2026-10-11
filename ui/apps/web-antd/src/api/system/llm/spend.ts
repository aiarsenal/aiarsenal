/**
 * gin LLM 用量日志接口（/api/v1/llm/spend/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go（LLMSpendLogSearchReq / LLMSpendLogSearchRes）。
 *   - PageReq 公共字段：dateRange / pageNum / pageSize。
 *   - ListRes 公共字段：currentPage / total。
 * 实体源：api/domain/entity/llm.go LLMSpendLog（list 行业务 id = requestId）。
 * 控制器：api/interfaces/controller/llm_monitor.go（LLMMonitorController.ListSpendLogs / GetSpendLog）。
 */

import { requestClient } from '#/api/request';

export namespace LLMSpendApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    userId?: string;
    teamId?: string;
    model?: string;
    apiKey?: string;
    dateRange?: string[];
  }

  export interface ListItem {
    requestId: string;
    callType?: string;
    apiKey?: string;
    spend?: number;
    totalTokens?: number;
    promptTokens?: number;
    completionTokens?: number;
    cacheReadInputTokens?: number;
    cacheCreationInputTokens?: number;
    startTime?: string;
    endTime?: string;
    requestDurationMs?: number;
    completionStartTime?: string;
    model?: string;
    modelId?: string;
    modelGroup?: string;
    customLlmProvider?: string;
    apiBase?: string;
    userId?: string;
    teamId?: string;
    orgId?: string;
    projectId?: string;
    endUser?: string;
    sessionId?: string;
    requesterIpAddress?: string;
    cacheHit?: string;
    cacheKey?: string;
    requestTags?: string;
    metadata?: string;
    messages?: string;
    response?: string;
    status?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 花费归集聚合行（维度值按 dimension 落在 date / keyAlias|scopeId / model 之一） */
  export interface AggregateItem {
    date?: string;
    scopeId?: string;
    keyAlias?: string;
    model?: string;
    spend: number;
    apiRequests: number;
    promptTokens: number;
    completionTokens: number;
  }

  export interface AggregateTotals {
    spend: number;
    apiRequests: number;
    promptTokens: number;
    completionTokens: number;
  }

  export interface AggregateResult {
    list: AggregateItem[];
    totals: AggregateTotals;
  }

  export interface AggregateParams {
    dimension: 'date' | 'key' | 'user' | 'model';
    dateRange?: string[];
  }
}

const BASE = '/api/v1/llm/spend';

/** 用量日志列表 */
export function getLLMSpendListApi(params: LLMSpendApi.ListParams) {
  return requestClient.get<LLMSpendApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单条用量日志（gin: params { requestId }）*/
export function getLLMSpendApi(requestId: string) {
  return requestClient.get<LLMSpendApi.ListItem>(`${BASE}/get`, {
    params: { requestId },
  });
}

/** 花费归集报表（按日/密钥/用户/模型维度聚合，含窗口合计） */
export function getLLMSpendAggregateApi(
  params: LLMSpendApi.AggregateParams,
) {
  return requestClient.get<LLMSpendApi.AggregateResult>(
    `${BASE}/aggregate`,
    { params },
  );
}
