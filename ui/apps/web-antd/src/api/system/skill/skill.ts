/**
 * gin 技能管理接口（/api/v1/skill/skill/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段名以 dto/skill.go 为准。
 */

import { requestClient } from '#/api/request';

export namespace SkillApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    namespaceId?: string;
    slug?: string;
    name?: string;
    status?: number | string;
    keyWords?: string;
  }

  export interface ListItem {
    skillId: string;
    namespaceId: string;
    slug: string;
    displayName: string;
    summary?: string;
    ownerId?: string;
    ownerName?: string;
    ownerEmpNo?: string;
    sourceSkillId?: string;
    visibility: number; // 0=PUBLIC 1=PRIVATE
    status: number; // 1=ACTIVE 0=INACTIVE
    latestVersionId?: string;
    hidden: number; // 0/1
    downloadCount?: number;
    starCount?: number;
    ratingAvg?: number;
    ratingCount?: number;
    subscriptionCount?: number;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    currentPage?: number;
    total: number;
  }

  /** SkillReq（dto/skill.go）—— add/edit payload 公共字段 */
  export interface SkillReq {
    namespaceId: string;
    slug: string;
    displayName: string;
    summary?: string;
    visibility: number;
    status: number;
  }

  export interface SkillEditReq extends SkillReq {
    skillId: string;
  }

  export interface NamespaceItem {
    namespaceId: string;
    slug: string;
    displayName: string;
    type?: string;
    description?: string;
    avatar?: string;
    status: number;
  }

  export interface VersionItem {
    versionId: string;
    skillId: string;
    version: string;
    changelog?: string;
    status: number; // 0=DRAFT 1=PUBLISHED 2=YANKED
    // 最新安全审计状态（后端回填）：1=安全 2=不安全 3=扫描中 4=扫描失败；缺省=无审计记录（门禁同拦）
    auditStatus?: number;
    createdAt?: string;
  }

  /** 技能包预解析结果（回填弹窗字段） */
  export interface PackageParseResult {
    tempId: string;
    slug: string;
    name: string;
    summary: string;
    version?: string;
    license?: string;
    fileCount: number;
    totalSize: number;
    metadata?: Record<string, any>;
  }

  /** 技能包提交创建请求（凭 tempId） */
  export interface PackageCreateReq {
    tempId: string;
    namespaceId: string;
    slug: string;
    displayName: string;
    summary?: string;
    visibility: number;
    status: number;
    version: string;
    changelog?: string;
  }

  /** 版本内文件 */
  export interface SkillFileItem {
    fileId: string;
    versionId: string;
    filePath: string;
    fileSize: number;
    contentType: string;
    sha256: string;
    storageKey: string;
    createdAt?: string;
  }
}

/** 技能列表 */
export function getSkillListApi(params: SkillApi.ListParams) {
  return requestClient.get<SkillApi.ListResult>(
    '/api/v1/skill/skill/list',
    { params },
  );
}

/** 技能详情 */
export function getSkillApi(skillId: string) {
  return requestClient.get<SkillApi.ListItem>(
    '/api/v1/skill/skill/get',
    { params: { skillId } },
  );
}

/** 新增 */
export function addSkillApi(data: SkillApi.SkillReq) {
  return requestClient.post('/api/v1/skill/skill/add', data);
}

/** 编辑（gin: { skillId, ...SkillReq }） */
export function editSkillApi(data: SkillApi.SkillEditReq) {
  return requestClient.put('/api/v1/skill/skill/edit', data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteSkillApi(ids: string[]) {
  return requestClient.delete('/api/v1/skill/skill/delete', {
    data: { ids },
  });
}

/** 切换隐藏（gin: { skillId, hidden }） */
export function setSkillHiddenApi(skillId: string, hidden: number) {
  return requestClient.put('/api/v1/skill/skill/setHidden', {
    skillId,
    hidden,
  });
}

/** 命名空间下拉（gin: /skill/namespace/all → { list }） */
export function getAllSkillNamespacesApi() {
  return requestClient.get<{ list: SkillApi.NamespaceItem[] }>(
    '/api/v1/skill/namespace/all',
  );
}

/** 技能版本列表（gin: { list }） */
export function getSkillVersionsApi(skillId: string) {
  return requestClient.get<{ list: SkillApi.VersionItem[] }>(
    '/api/v1/skill/skill/version/list',
    { params: { skillId } },
  );
}

/** 新增版本（gin: { skillId, version, changelog }） */
export function addSkillVersionApi(data: {
  skillId: string;
  version: string;
  changelog?: string;
}) {
  return requestClient.post('/api/v1/skill/skill/version/add', data);
}

/** 发布版本（gin: { versionId }） */
export function publishSkillVersionApi(versionId: string) {
  return requestClient.post('/api/v1/skill/skill/version/publish', {
    versionId,
  });
}

/** 撤回版本（gin: { versionId, reason }） */
export function yankSkillVersionApi(versionId: string, reason: string) {
  return requestClient.put('/api/v1/skill/skill/version/yank', {
    versionId,
    reason,
  });
}

/** 预解析技能包 zip：解析 SKILL.md 回填字段 + 暂存到 staging（multipart: file） */
export function parseSkillPackageApi(file: File) {
  const form = new FormData();
  form.append('file', file);
  return requestClient.post<SkillApi.PackageParseResult>(
    '/api/v1/skill/skill/package/parse',
    form,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  );
}

/** 提交创建 skill + 版本（凭预解析返回的 tempId） */
export function createSkillPackageApi(data: SkillApi.PackageCreateReq) {
  return requestClient.post<{ skillId: string; versionId: string }>(
    '/api/v1/skill/skill/package/create',
    data,
  );
}

/** 版本文件列表 */
export function getSkillFilesApi(versionId: string) {
  return requestClient.get<{ list: SkillApi.SkillFileItem[] }>(
    '/api/v1/skill/skill/version/file/list',
    { params: { versionId } },
  );
}

/** 下载版本内单个文件（blob，触发浏览器下载） */
export async function downloadSkillFileApi(versionId: string, filePath: string) {
  const blob = await requestClient.download('/api/v1/skill/skill/file/download', {
    params: { versionId, filePath },
  });
  triggerBrowserDownload(blob as Blob, filePath.split('/').pop() || 'download');
}

/** 下载整包 zip */
export async function downloadSkillVersionApi(versionId: string, filename: string) {
  const blob = await requestClient.download('/api/v1/skill/skill/version/download', {
    params: { versionId },
  });
  triggerBrowserDownload(blob as Blob, filename);
}

function triggerBrowserDownload(blob: Blob, filename: string) {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
}
