/**
 * 项目管理 — 项目接口（/api/v1/project/*）。
 * 响应壳 {code:0,data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace ProjectApi {
  export interface ListItem {
    projectId: string;
    name: string;
    code?: string;
    description?: string;
    status: string;
    priority?: string;
    progress?: number;
    owner?: string;
    visibility?: string;
    deptId?: string;
    startDate?: string;
    dueDate?: string;
    color?: string;
    tags?: string;
    archived?: number;
    createdBy?: string;
    createdAt?: string;
  }
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }
  export interface Overview {
    project: ListItem;
    memberCount: number;
    [k: string]: any;
  }
}

const BASE = '/api/v1/project';

export function getProjectListApi(params: Recordable<any>) {
  return requestClient.get<ProjectApi.ListResult>(`${BASE}/list`, { params });
}
export function getAllProjectsApi() {
  return requestClient.get<ProjectApi.ListItem[]>(`${BASE}/all`);
}
export function getProjectApi(projectId: string) {
  return requestClient.get<ProjectApi.ListItem>(`${BASE}/get`, { params: { projectId } });
}
export function getProjectOverviewApi(projectId: string) {
  return requestClient.get<ProjectApi.Overview>(`${BASE}/overview`, { params: { projectId } });
}
export function addProjectApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function editProjectApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
export function deleteProjectApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

// ===== 资源绑定 =====
export const BINDING_KIND_OPTIONS = [
  { label: 'Agent', value: 'agent' },
  { label: 'Skill', value: 'skill' },
  { label: '知识库', value: 'kb' },
  { label: 'LLM 预算', value: 'llm_budget' },
  { label: 'LLM 密钥', value: 'llm_key' },
  { label: 'MCP 分组', value: 'mcp_group' },
];
export function getProjectBindingListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/binding/list`, { params });
}
export function addProjectBindingApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/binding/add`, data);
}
export function editProjectBindingApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/binding/edit`, data);
}
export function deleteProjectBindingApi(ids: string[]) {
  return requestClient.delete(`${BASE}/binding/delete`, { data: { ids } });
}

// ===== 看板 =====
export function getProjectBoardApi(projectId: string) {
  return requestClient.get<{ columns: any[]; tasks: any[] }>(`${BASE}/board`, { params: { projectId } });
}
export function moveProjectTaskApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/task/move`, data);
}

// ===== 报表 / 时间线 / 动态 =====
export function getProjectReportApi(projectId: string) {
  return requestClient.get<Recordable<any>>(`${BASE}/report/overview`, { params: { projectId } });
}
export function getProjectTimelineApi(projectId: string) {
  return requestClient.get<{ list: any[] }>(`${BASE}/timeline/list`, { params: { projectId } });
}
export function getProjectActivityListApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/activity/list`, { params });
}
