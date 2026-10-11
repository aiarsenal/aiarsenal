/**
 * 项目管理 — 设置接口（/api/v1/project/config/*）：标签 / 状态流 / 自定义字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

const BASE = '/api/v1/project/config';

// ===== 标签 =====
export function getProjectTagListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/tag/list`, { params });
}
export function addProjectTagApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/tag/add`, data);
}
export function editProjectTagApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/tag/edit`, data);
}
export function deleteProjectTagApi(ids: string[]) {
  return requestClient.delete(`${BASE}/tag/delete`, { data: { ids } });
}

// ===== 状态流 =====
export const STATUS_CATEGORY_OPTIONS = [
  { label: '待办', value: 'todo' },
  { label: '进行中', value: 'doing' },
  { label: '已完成', value: 'done' },
];
export function getProjectStatusListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/status/list`, { params });
}
export function addProjectStatusApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/status/add`, data);
}
export function editProjectStatusApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/status/edit`, data);
}
export function deleteProjectStatusApi(ids: string[]) {
  return requestClient.delete(`${BASE}/status/delete`, { data: { ids } });
}

// ===== 自定义字段 =====
export const FIELD_TYPE_OPTIONS = [
  { label: '文本', value: 'text' },
  { label: '数字', value: 'number' },
  { label: '日期', value: 'date' },
  { label: '单选', value: 'select' },
  { label: '多选', value: 'multiselect' },
  { label: '用户', value: 'user' },
  { label: '链接', value: 'url' },
];
export function getProjectFieldListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/field/list`, { params });
}
export function addProjectFieldApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/field/add`, data);
}
export function editProjectFieldApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/field/edit`, data);
}
export function deleteProjectFieldApi(ids: string[]) {
  return requestClient.delete(`${BASE}/field/delete`, { data: { ids } });
}
