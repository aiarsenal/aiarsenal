/**
 * LLM 数据看板接口（/api/v1/stats/llm/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 只读聚合口径：数据源 = aa_llm_spend_log（请求/token/花费）+ aa_llm_health_check（健康），
 * 虚拟 Key 维度即 aa_llm_key（网关虚拟 Key），用户维度 = 发起请求的 aa_user。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace StatsLlmApi {
  /** 一个时间窗的用量汇总（今日/近 N 天共用） */
  export interface UsageSummary {
    requests: number;
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    spend: number;
    errorCount: number;
    /** 错误率（%，0-100，保留 2 位） */
    errorRate: number;
  }

  /** 总览响应（GET /stats/llm/overview） */
  export interface Overview {
    today: UsageSummary;
    last7Days: UsageSummary;
    /** 凭证健康卡（来自健康检查最近一轮） */
    health: {
      totalCredentials: number;
      healthyCredentials: number;
      unhealthyCredentials: number;
      /** 平均响应时延（ms） */
      avgLatencyMs: number;
      checkedAt?: string;
    };
  }

  /** 趋势点（GET /stats/llm/trend?days=7，按日聚合） */
  export interface TrendPoint {
    date: string; // YYYY-MM-DD
    requests: number;
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    spend: number;
    errorCount: number;
    /** 错误率（%，0-100） */
    errorRate: number;
  }

  /** Top 榜行（GET /stats/llm/top?dimension=model|virtualKey|user&days=7&limit=10） */
  export interface TopItem {
    /** 维度值：模型名 / 虚拟Key id / 用户 id */
    key: string;
    /** 展示名：模型名 / 虚拟Key 别名 / 用户昵称（后端负责富化） */
    name: string;
    requests: number;
    totalTokens: number;
    spend: number;
    errorCount: number;
    /** 错误率（%，0-100） */
    errorRate: number;
  }

  export interface TopResult {
    list: TopItem[];
  }

  export type TopDimension = 'model' | 'user' | 'virtualKey';
}

const BASE = '/api/v1/stats/llm';

/** LLM 用量总览（今日 + 近 7 天 + 凭证健康） */
export function getStatsLlmOverviewApi() {
  return requestClient.get<StatsLlmApi.Overview>(`${BASE}/overview`);
}

/** LLM 用量趋势（按日，days 默认 7，上限 90） */
export function getStatsLlmTrendApi(days = 7) {
  return requestClient.get<{ points: StatsLlmApi.TrendPoint[] }>(
    `${BASE}/trend`,
    { params: { days } },
  );
}

/** Top 榜（模型 / 虚拟Key / 用户，days 默认 7，limit 默认 10，上限 50） */
export function getStatsLlmTopApi(
  dimension: StatsLlmApi.TopDimension,
  days = 7,
  limit = 10,
) {
  return requestClient.get<StatsLlmApi.TopResult>(`${BASE}/top`, {
    params: { days, dimension, limit },
  });
}
