/**
 * 项目管理 — 里程碑接口（/api/v1/project/milestone/*）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

const BASE = '/api/v1/project/milestone';

export function getProjectMilestoneListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/list`, { params });
}
export function addProjectMilestoneApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function editProjectMilestoneApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
export function deleteProjectMilestoneApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}
