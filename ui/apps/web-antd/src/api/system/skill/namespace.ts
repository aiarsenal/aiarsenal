/**
 * gin 技能命名空间管理接口（/api/v1/skill/namespace/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段名以 dto.SkillNamespace* / entity.SkillNamespace 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace SkillNamespaceApi {
  // dto.SkillNamespaceSearchReq { slug, name, type, status, pageNum, pageSize }
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    slug?: string;
    name?: string;
    type?: string; // GLOBAL / TEAM
    status?: string;
  }

  // entity.SkillNamespace json 标签
  export interface ListItem {
    namespaceId: string;
    slug: string;
    displayName: string;
    type: string; // GLOBAL / TEAM
    description?: string;
    avatar?: string;
    status: number; // 1 启用 0 停用
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  // dto.SkillNamespaceSearchRes: { list, total, currentPage }
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  // dto.SkillNamespaceReq（Add/Edit 共用业务字段）
  export interface NamespacePayload {
    slug: string;
    displayName: string;
    type: string;
    description?: string;
    avatar?: string;
    status: number;
  }

  // entity.SkillNamespaceMember json 标签
  export interface MemberItem {
    memberId: string;
    namespaceId: string;
    userId: string;
    role: string; // OWNER / ADMIN / MEMBER
    createdAt?: string;
    updatedAt?: string;
  }
}

const BASE = '/api/v1/skill/namespace';

/** 命名空间列表（分页） */
export function getSkillNamespaceListApi(
  params: SkillNamespaceApi.ListParams,
) {
  return requestClient.get<SkillNamespaceApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 全部命名空间（下拉/无分页，gin: { list }） */
export function getAllSkillNamespacesApi() {
  return requestClient.get<{ list: SkillNamespaceApi.ListItem[] }>(
    `${BASE}/all`,
  );
}

/** 详情（query: namespaceId） */
export function getSkillNamespaceApi(namespaceId: string) {
  return requestClient.get<SkillNamespaceApi.ListItem>(`${BASE}/get`, {
    params: { namespaceId },
  });
}

/** 新增（body: dto.SkillNamespaceAddReq → SkillNamespaceReq 字段） */
export function addSkillNamespaceApi(data: SkillNamespaceApi.NamespacePayload) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body: { namespaceId, ...SkillNamespaceReq }） */
export function editSkillNamespaceApi(
  data: SkillNamespaceApi.NamespacePayload & { namespaceId: string },
) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（body: { id }，dto.SkillNamespaceDeleteReq.Id = namespaceId 字符串） */
export function deleteSkillNamespaceApi(id: string) {
  return requestClient.delete(`${BASE}/delete`, { data: { id } });
}

/** 成员列表（query: namespaceId → { list }） */
export function getSkillNamespaceMembersApi(namespaceId: string) {
  return requestClient.get<{ list: SkillNamespaceApi.MemberItem[] }>(
    `${BASE}/member/list`,
    { params: { namespaceId } },
  );
}

/** 添加成员（body: dto.SkillNamespaceMemberAddReq { namespaceId, userId, role }） */
export function addSkillNamespaceMemberApi(data: {
  namespaceId: string;
  userId: string;
  role: string;
}) {
  return requestClient.post(`${BASE}/member/add`, data);
}

/** 移除成员（body: { memberId }，dto.SkillNamespaceMemberDelReq） */
export function removeSkillNamespaceMemberApi(memberId: string) {
  return requestClient.delete(`${BASE}/member/remove`, {
    data: { memberId },
  });
}

/** 重载（供 recordable 工具用） */
export type _SkillNamespaceRecordable = Recordable<any>;
