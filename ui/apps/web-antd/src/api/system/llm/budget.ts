/**
 * gin LLM 预算接口（/api/v1/llm/budget/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/llm.go（LLMBudgetSearchReq / LLMBudgetReq / LLMBudgetEditReq / LLMIdsReq）。
 * 实体源：api/domain/entity/llm.go LLMBudget（list 行 id = budgetId 业务 id）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace LLMBudgetApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  export interface ListItem {
    budgetId: string;
    budgetName: string;
    maxBudget?: number;
    softBudget?: number;
    maxParallelRequests?: number;
    tpmLimit?: number;
    rpmLimit?: number;
    modelMaxBudget?: string;
    budgetDuration?: string;
    budgetResetAt?: string;
    allowedModels?: string;
    /** 适用范围条件行 JSON（[]GrantRow；空 '[]'=不圈人） */
    grantRules?: string;
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

  /** 表单 payload（对应 gin LLMBudgetReq）。 */
  export interface BudgetForm {
    budgetId?: string;
    budgetName: string;
    maxBudget?: number;
    softBudget?: number;
    maxParallelRequests?: number;
    tpmLimit?: number;
    rpmLimit?: number;
    modelMaxBudget?: string;
    budgetDuration?: string;
    allowedModels?: string;
    /** 适用范围条件行 JSON 字符串（GrantEditor 序列化产物） */
    grantRules?: string;
  }
}

const BASE = '/api/v1/llm/budget';

/** 预算列表 */
export function getLLMBudgetListApi(params: LLMBudgetApi.ListParams) {
  return requestClient.get<LLMBudgetApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单个预算 */
export function getLLMBudgetApi(budgetId: string) {
  return requestClient.get<LLMBudgetApi.ListItem>(`${BASE}/get`, {
    params: { budgetId },
  });
}

/** 新增 */
export function addLLMBudgetApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body 含 budgetId）*/
export function editLLMBudgetApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }）*/
export function deleteLLMBudgetApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 我的预算：当前用户命中的圈人预算 + 本周期消费/剩余（DTO 源 dto/llm.go LLMMyBudgetItem） */
export namespace LLMBudgetApi {
  export interface MyBudgetItem {
    budgetId: string;
    budgetName: string;
    /** nil/undefined = 不限额 */
    maxBudget?: number;
    softBudget?: number;
    /** 本人周期内消费（展示层计算：user 维度日聚合，不写预算数据） */
    spend: number;
    /** 上限 − 本人周期内消费；nil/undefined = 不限额 */
    remaining?: number;
    /** 0-100（本人消费口径） */
    usedPercent: number;
    budgetDuration?: string;
    budgetResetAt?: string;
    /** 本人已用 ≥90% */
    isLow: boolean;
    /** 本人已用 ≥100% */
    isExhausted: boolean;
  }

  /** 本人自然月消费汇总（与预算窗口解耦的真实消费） */
  export interface MyUsageSummary {
    /** YYYY-MM */
    month: string;
    spend: number;
    promptTokens: number;
    completionTokens: number;
    requests: number;
  }

  export interface MyBudgetResult {
    list: MyBudgetItem[];
    /** 命中预算（生效上限最大的一条，唯一展示已用/剩余/进度与提醒的对象） */
    hitBudgetId?: string;
    usage: MyUsageSummary;
  }
}

/** 我的预算（个人视角只读） */
export function getMyBudgetsApi() {
  return requestClient.get<LLMBudgetApi.MyBudgetResult>(`${BASE}/mine`);
}

/** 历史周期消费（DTO 源 dto/llm.go LLMSpendHistoryRes） */
export namespace LLMBudgetApi {
  export interface SpendHistoryPoint {
    /** day→2026-09-05 / week→2026-W36 / month→2026-09 */
    period: string;
    spend: number;
  }

  export interface SpendHistoryResult {
    period: string;
    points: SpendHistoryPoint[];
  }
}

// 临时预算申请（create/list/approve/reject）已上收统一审核中心：
// api/system/review/order.ts（order_type=llm_temp_budget），进度见 审核系统·我的提交/我的审核。

/** 历史周期消费（period=day/week/month，缺省 month；本人 scope 日聚合再分桶） */
export function getMySpendHistoryApi(period: string) {
  return requestClient.get<LLMBudgetApi.SpendHistoryResult>(
    `${BASE}/spend/history`,
    { params: { period } },
  );
}
