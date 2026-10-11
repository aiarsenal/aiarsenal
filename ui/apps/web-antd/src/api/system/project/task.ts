/**
 * 项目管理 — 任务/评论接口（/api/v1/project/task/*）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace ProjectTaskApi {
  export interface ListItem {
    taskId: string;
    projectId: string;
    parentTaskId?: string;
    milestoneId?: string;
    title: string;
    description?: string;
    status: string;
    priority?: string;
    assigneeId?: string;
    assigneeName?: string;
    reporterId?: string;
    startDate?: string;
    dueDate?: string;
    doneAt?: string;
    sortOrder?: number;
    tags?: string;
    customFields?: string;
    estimateHours?: number;
    spentHours?: number;
    createdBy?: string;
    createdAt?: string;
  }
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }
}

const BASE = '/api/v1/project/task';

export function getProjectTaskListApi(params: Recordable<any>) {
  return requestClient.get<ProjectTaskApi.ListResult>(`${BASE}/list`, { params });
}
export function getProjectTaskApi(taskId: string) {
  return requestClient.get<ProjectTaskApi.ListItem>(`${BASE}/get`, { params: { taskId } });
}
export function addProjectTaskApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function editProjectTaskApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
export function deleteProjectTaskApi(ids: string[], projectId?: string) {
  return requestClient.delete(`${BASE}/delete`, { params: projectId ? { projectId } : {}, data: { ids } });
}

// ===== 评论 =====
export function getProjectTaskCommentsApi(taskId: string) {
  return requestClient.get<{ list: any[] }>(`${BASE}/comment/list`, { params: { taskId } });
}
export function addProjectTaskCommentApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/comment/add`, data);
}
export function deleteProjectTaskCommentApi(ids: string[]) {
  return requestClient.delete(`${BASE}/comment/delete`, { data: { ids } });
}
