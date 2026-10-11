/**
 * gin 技能标签管理接口（/api/v1/skill/label/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段名以 dto.SkillLabel* / entity.SkillLabelDefinition 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace SkillLabelApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    slug?: string;
    type?: string;
  }

  // entity.SkillLabelDefinition json 标签
  export interface ListItem {
    labelId: string;
    slug: string;
    type: string; // RECOMMENDED / PRIVILEGED
    visibleInFilter: number;
    sortOrder: number;
    createdBy?: string;
    creatorName?: string; // service 跨库回填 aa_user.name；为空表示系统内置
    createdAt?: string;
    updatedAt?: string;
  }

  // dto.SkillLabelSearchRes: { list, total, currentPage }
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  // dto.SkillLabelReq (Add/Edit 共用业务字段)
  export interface LabelPayload {
    slug: string;
    type: string;
    visibleInFilter: number;
    sortOrder: number;
  }

  export interface TransItem {
    transId?: string;
    labelId: string;
    locale: string;
    displayName: string;
    createdAt?: string;
    updatedAt?: string;
  }
}

const BASE = '/api/v1/skill/label';

/** 标签列表（分页） */
export function getSkillLabelListApi(params: SkillLabelApi.ListParams) {
  return requestClient.get<SkillLabelApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 全部标签（下拉/无分页，gin: { list }） */
export function getAllSkillLabelsApi() {
  return requestClient.get<{ list: SkillLabelApi.ListItem[] }>(`${BASE}/all`);
}

/** 详情（query: labelId） */
export function getSkillLabelApi(labelId: string) {
  return requestClient.get<SkillLabelApi.ListItem>(`${BASE}/get`, {
    params: { labelId },
  });
}

/** 新增（body: dto.SkillLabelAddReq → SkillLabelReq 字段） */
export function addSkillLabelApi(data: SkillLabelApi.LabelPayload) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body: { labelId, ...SkillLabelReq }） */
export function editSkillLabelApi(
  data: SkillLabelApi.LabelPayload & { labelId: string },
) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（body: { id }，dto.SkillLabelDeleteReq.Id = labelId 字符串） */
export function deleteSkillLabelApi(id: string) {
  return requestClient.delete(`${BASE}/delete`, { data: { id } });
}

/** 翻译列表（query: labelId → { list }） */
export function getSkillLabelTranslationsApi(labelId: string) {
  return requestClient.get<{ list: SkillLabelApi.TransItem[] }>(
    `${BASE}/trans/list`,
    { params: { labelId } },
  );
}

/** 新增/更新翻译（body: dto.SkillLabelTransReq） */
export function upsertSkillLabelTranslationApi(
  data: SkillLabelApi.TransItem,
) {
  return requestClient.post(`${BASE}/trans/upsert`, data);
}

/** 重载（供 recordable 工具用） */
export type _SkillLabelRecordable = Recordable<any>;
