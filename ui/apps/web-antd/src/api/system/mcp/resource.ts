/**
 * gin MCP 内置 Resource 接口（/api/v1/mcp/resource/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 dto.McpResourceSearchReq / dto.McpResourceReq / entity.McpBuiltinResource 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpResourceApi {
  /** 列表查询参数（dto.McpResourceSearchReq） */
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  /** 列表行（entity.McpBuiltinResource） */
  export interface ListItem {
    resourceId: string;
    uri: string;
    name: string;
    description?: string;
    mimeType?: string;
    content?: string;
    enabled: number;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 列表返回（dto.McpResourceSearchRes: { list, total, currentPage }） */
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 新增/编辑表单 payload（dto.McpResourceReq） */
  export interface ResourceForm {
    uri: string;
    name?: string;
    description?: string;
    mimeType?: string;
    content: string;
    enabled: number;
  }
}

const BASE = '/api/v1/mcp/resource';

/** 资源列表（分页） */
export function getMcpResourceListApi(params: McpResourceApi.ListParams) {
  return requestClient.get<McpResourceApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单条详情 */
export function getMcpResourceApi(resourceId: string) {
  return requestClient.get<McpResourceApi.ListItem>(`${BASE}/get`, {
    params: { resourceId },
  });
}

/** 新增（body: dto.McpResourceReq） */
export function addMcpResourceApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body: dto.McpResourceEditReq = { resourceId, ...McpResourceReq }） */
export function editMcpResourceApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteMcpResourceApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}
