/**
 * gin Agent 概览接口（/api/v1/agent/dashboard/*）
 */
import { requestClient } from '#/api/request';

export namespace AgentDashboardApi {
  export interface Overview {
    instanceTotal: number;
    instanceRunning: number;
    teamTotal: number;
    modelTotal: number;
    scanJobActive: number;
    invocation24h: number;
    cost24h: number;
  }
}

export function getAgentDashboardOverviewApi() {
  return requestClient.get<AgentDashboardApi.Overview>(
    '/api/v1/agent/dashboard/overview',
  );
}
