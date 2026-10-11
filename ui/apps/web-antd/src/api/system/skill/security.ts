/**
 * gin 技能安全审计接口（/api/v1/skill/security/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 端点（api/interfaces/router/router.go → SkillSecurityController）：
 *   GET  /skill/security/list    → ListAudits  (query: versionId/status/pageNum/pageSize)
 *   POST /skill/security/rescan  → Rescan     (body : { versionId })
 *
 * DTO（api/interfaces/dto/skill.go）：
 *   SkillAuditSearchReq  { versionId, status, PageReq }
 *   SkillAuditSearchRes  { ListRes(CurrentPage/Total), List []*SkillSecurityAudit }
 *   SkillAuditRescanReq  { versionId (binding:required) }
 *
 * 实体 api/domain/entity/skill_security.go 字段：
 *   auditId / skillVersionId / status(0=PENDING,1=PASSED,2=FAILED)
 *   / severity(0~4) / findingsJson / scannerRaw / scannedAt / createdAt
 */
import { requestClient } from '#/api/request';

export namespace SkillSecurityApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    versionId?: string;
    status?: number | string;
  }

  export interface ListItem {
    auditId: string;
    skillVersionId: string;
    status: number; // 0=PENDING 1=PASSED 2=FAILED
    severity: number; // 0~4
    findingsJson?: any;
    scannerRaw?: string;
    scannedAt?: string;
    createdAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  export interface RescanResult {
    auditId: string;
  }
}

/** 安全审计列表 */
export function getSkillAuditListApi(
  params: SkillSecurityApi.ListParams,
) {
  return requestClient.get<SkillSecurityApi.ListResult>(
    '/api/v1/skill/security/list',
    { params },
  );
}

/** 重新扫描（gin body: { versionId }）*/
export function rescanSkillAuditApi(versionId: string) {
  return requestClient.post<SkillSecurityApi.RescanResult>(
    '/api/v1/skill/security/rescan',
    { versionId },
  );
}
