/**
 * SKILL 数据看板接口（/api/v1/stats/skill/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 口径：技能总数/已发布 = aa_skill + aa_skill_version（version.status=1 为已发布）；
 * 待审核 = 版本安全审计未出结论（auditStatus 缺省/3=扫描中）或审核工单挂起数；
 * 安装趋势 = aa_agent_instance_skill（实例安装流水，含卸载回滚）按日聚合；
 * 来源分布 = skill.source（local / market / import 等）。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace StatsSkillApi {
  /** 总览响应（GET /stats/skill/overview） */
  export interface Overview {
    totalSkills: number;
    /** 存在已发布版本（version.status=1）的技能数 */
    publishedSkills: number;
    /** 仅有草稿版本（version.status=0）的技能数 */
    draftSkills: number;
    /** 待审核：待安全扫描/待人工审核的版本或工单数 */
    pendingReview: number;
    totalVersions: number;
    /** 实例侧当前安装总量（去重后的安装关系数） */
    installedCount: number;
    /** 安全扫描通过率卡 */
    security: {
      /** 已有审计结论的版本数 */
      scanned: number;
      /** auditStatus=1 安全 */
      passed: number;
      /** auditStatus=2 不安全 */
      failed: number;
      /** auditStatus=3 扫描中 */
      scanning: number;
      /** 通过率（%，passed/scanned，保留 1 位） */
      passRate: number;
    };
    /** 来源分布（市场来源/本地创建/导入，饼图数据） */
    sources: SourceItem[];
  }

  /** 来源分布行 */
  export interface SourceItem {
    /** 来源标识：local / market:<marketId> / import 等 */
    source: string;
    /** 展示名（后端富化，市场项带市场名） */
    label: string;
    count: number;
  }

  /** 安装趋势点（GET /stats/skill/install-trend?days=30，按日聚合） */
  export interface InstallTrendPoint {
    date: string; // YYYY-MM-DD
    installs: number;
    uninstalls: number;
  }
}

const BASE = '/api/v1/stats/skill';

/** 技能数据总览（计数 + 安全扫描 + 来源分布） */
export function getStatsSkillOverviewApi() {
  return requestClient.get<StatsSkillApi.Overview>(`${BASE}/overview`);
}

/** 技能安装趋势（按日，days 默认 30，上限 90） */
export function getStatsSkillInstallTrendApi(days = 30) {
  return requestClient.get<{ points: StatsSkillApi.InstallTrendPoint[] }>(
    `${BASE}/install-trend`,
    { params: { days } },
  );
}
