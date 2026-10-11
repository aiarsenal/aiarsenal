/**
 * gin Agent Team 协作接口（/api/v1/agent/team/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentTeamApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }
  export interface TeamItem {
    teamId: string;
    userId?: string;
    name: string;
    description?: string;
    status: number;
    communicationMode?: string;
    sharedVolumeName?: string;
    sharedMountPath?: string;
    redisUrl?: string;
    teamToken?: string;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }
  export interface ListResult {
    list: TeamItem[];
    total: number;
    currentPage?: number;
  }
  export interface TeamReq {
    teamId?: string;
    name: string;
    description?: string;
    communicationMode?: string;
    sharedVolumeName?: string;
    sharedMountPath?: string;
    redisUrl?: string;
    teamToken?: string;
  }
  export interface MemberListParams {
    pageNum: number;
    pageSize: number;
    teamId: string;
  }
  export interface TaskListParams {
    pageNum: number;
    pageSize: number;
    teamId: string;
    status?: number;
  }
  export interface EventListParams {
    pageNum: number;
    pageSize: number;
    teamId: string;
  }
}

const BASE = '/api/v1/agent/team';

export function getAgentTeamListApi(params: AgentTeamApi.ListParams) {
  return requestClient.get<AgentTeamApi.ListResult>(`${BASE}/list`, { params });
}
export function getAgentTeamApi(id: string) {
  return requestClient.get<AgentTeamApi.TeamItem>(`${BASE}/get`, {
    params: { teamId: id },
  });
}
export function addAgentTeamApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}
export function editAgentTeamApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}
export function deleteAgentTeamApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

export function getAgentTeamMemberListApi(params: AgentTeamApi.MemberListParams) {
  return requestClient.get<AgentTeamApi.ListResult>(`${BASE}/member/list`, {
    params,
  });
}
export function addAgentTeamMemberApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/member/add`, data);
}
export function editAgentTeamMemberApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/member/edit`, data);
}
export function deleteAgentTeamMemberApi(ids: string[]) {
  return requestClient.delete(`${BASE}/member/delete`, { data: { ids } });
}

export function getAgentTeamTaskListApi(params: AgentTeamApi.TaskListParams) {
  return requestClient.get<AgentTeamApi.ListResult>(`${BASE}/task/list`, {
    params,
  });
}
export function dispatchAgentTeamTaskApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/task/dispatch`, data);
}

export function getAgentTeamEventListApi(params: AgentTeamApi.EventListParams) {
  return requestClient.get<AgentTeamApi.ListResult>(`${BASE}/event/list`, {
    params,
  });
}

export function getAgentTeamWorkItemListApi(params: any) {
  return requestClient.get<AgentTeamApi.ListResult>(`${BASE}/work-item/list`, {
    params,
  });
}
