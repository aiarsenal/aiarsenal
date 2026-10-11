/**
 * gin 治理监控接口（/api/v1/agent/monitor/*，只读）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 5 类记录：invocation / audit-event / cost / risk-hit / audit-log
 * audit-log 支持批量清理。
 */
import { requestClient } from '#/api/request';

const BASE = '/api/v1/agent/monitor';

export type MonitorRecordType =
  | 'audit-event'
  | 'audit-log'
  | 'cost'
  | 'invocation'
  | 'risk-hit';

export interface MonitorListParams {
  pageNum: number;
  pageSize: number;
  userId?: string;
  modelId?: string;
  eventType?: string;
  ruleId?: string;
  action?: string;
}

export interface MonitorListResult {
  list: any[];
  total: number;
  currentPage?: number;
}

/** 模型调用记录 */
export function getAgentInvocationListApi(params: MonitorListParams) {
  return requestClient.get<MonitorListResult>(`${BASE}/invocation/list`, {
    params,
  });
}

/** 审计事件 */
export function getAgentAuditEventListApi(params: MonitorListParams) {
  return requestClient.get<MonitorListResult>(`${BASE}/audit-event/list`, {
    params,
  });
}

/** 成本记录 */
export function getAgentCostListApi(params: MonitorListParams) {
  return requestClient.get<MonitorListResult>(`${BASE}/cost/list`, {
    params,
  });
}

/** 风险命中 */
export function getAgentRiskHitListApi(params: MonitorListParams) {
  return requestClient.get<MonitorListResult>(`${BASE}/risk-hit/list`, {
    params,
  });
}

/** 操作日志 */
export function getAgentAuditLogListApi(params: MonitorListParams) {
  return requestClient.get<MonitorListResult>(`${BASE}/audit-log/list`, {
    params,
  });
}

/** 清理操作日志（批量，body: { ids: [] }） */
export function deleteAgentAuditLogApi(ids: string[]) {
  return requestClient.delete(`${BASE}/audit-log/delete`, { data: { ids } });
}
