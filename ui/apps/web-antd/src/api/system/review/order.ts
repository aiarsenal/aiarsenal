/**
 * gin 统一审核工单接口（/api/v1/review/order/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * DTO 源：api/interfaces/dto/review.go（ReviewOrderCreateReq / ReviewOrderListReq / ReviewOrderItem）。
 * 所有提交类申请（llm 临时预算等）统一走这里，审核系统「我的提交/我的审核」两页面消费。
 */
import { requestClient } from '#/api/request';

export namespace ReviewOrderApi {
  export interface ListParams {
    pageNum?: number;
    pageSize?: number;
    /** mine：空=全部；review：空=默认 pending */
    status?: string;
  }

  /** llm_temp_budget 类型的 payload 快照（service/review_type_llm.go llmTempPayload） */
  export interface LlmTempBudgetPayload {
    baseBudgetId?: string;
    baseBudgetName?: string;
    duration?: string;
    reason?: string;
  }

  export interface Item {
    orderId: string;
    orderType: string;
    title: string;
    applicantId: string;
    applicantName: string;
    deptId: string;
    /** 金额（主币浮点，后端毫存储自动换算；无金额类型=0） */
    amount: number;
    /** 类型专属快照（按 orderType 解析；llm_temp_budget 见 LlmTempBudgetPayload） */
    payload: Record<string, any>;
    /** pending/approved/rejected */
    status: string;
    reviewerId?: string;
    reviewerName?: string;
    reviewNote?: string;
    /** 审批产物引用（llm=生成的临时预算 budgetId） */
    resultRef?: string;
    reviewedAt?: string;
    createdAt?: string;
  }

  export interface ListResult {
    list: Item[];
    total: number;
    currentPage?: number;
  }

  /** 提交 payload：type 区分审核类型，params 为类型专属原始参数 */
  export interface CreatePayload {
    type: string;
    params: Record<string, any>;
  }
}

const BASE = '/api/v1/review/order';

/** llm 临时预算申请 params（baseBudgetId 可缺省：后端自动取命中本人且上限最大的预算） */
export function createLlmTempBudgetApi(data: {
  amount: number;
  baseBudgetId?: string;
  duration?: string;
  reason: string;
}) {
  const { amount, baseBudgetId, duration, reason } = data;
  return createReviewOrderApi({
    params: {
      amount,
      ...(baseBudgetId ? { baseBudgetId } : {}),
      duration: duration || '30d',
      reason,
    },
    type: 'llm_temp_budget',
  });
}

/** 提交审核工单（通用入口） */
export function createReviewOrderApi(data: ReviewOrderApi.CreatePayload) {
  return requestClient.post<ReviewOrderApi.Item>(`${BASE}/create`, data);
}

/** 我的提交（全类型） */
export function getMyReviewOrdersApi(params: ReviewOrderApi.ListParams) {
  return requestClient.get<ReviewOrderApi.ListResult>(`${BASE}/mine`, {
    params,
  });
}

/** 待我审批（超管=全部；否则=本部门） */
export function getReviewTodoApi(params: ReviewOrderApi.ListParams) {
  return requestClient.get<ReviewOrderApi.ListResult>(`${BASE}/review`, {
    params,
  });
}

/** 通过审批（llm：生成申请人专属临时预算并通知） */
export function approveReviewOrderApi(data: { note?: string; orderId: string }) {
  return requestClient.post(`${BASE}/approve`, data);
}

/** 驳回审批（通知申请人） */
export function rejectReviewOrderApi(data: { note?: string; orderId: string }) {
  return requestClient.post(`${BASE}/reject`, data);
}
