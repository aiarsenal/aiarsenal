/**
 * 项目管理 — 成员接口（/api/v1/project/member/*）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export const MEMBER_ROLE_OPTIONS = [
  { label: '负责人', value: 'owner' },
  { label: '维护者', value: 'maintainer' },
  { label: '成员', value: 'member' },
  { label: '观察者', value: 'observer' },
];

const BASE = '/api/v1/project/member';

export function getProjectMemberListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/list`, { params });
}
export function addProjectMemberApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function editProjectMemberApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
export function deleteProjectMemberApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}
