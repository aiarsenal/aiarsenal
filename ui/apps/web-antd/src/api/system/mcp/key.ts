/**
 * gin MCP Bearer 密钥接口（/api/v1/mcp/key/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 dto.McpKeyReq / dto.McpKeySearchRes / dto.McpKeyCreateRes 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpKeyApi {
  /** 列表查询参数（dto.McpKeySearchReq） */
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  /** 列表行（entity.McpBearerKey，仅列常用字段） */
  export interface ListItem {
    keyId: string;
    name: string;
    kind: string; // system/user
    enabled: number;
    owner?: string;
    accessType: string; // all/groups/servers/custom
    allowedGroups?: string; // jsonb 数组字符串
    allowedServers?: string; // jsonb 数组字符串
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 列表返回（dto.McpKeySearchRes: { list, total, currentPage }） */
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 新增/编辑表单 payload（dto.McpKeyReq） */
  export interface KeyForm {
    name: string;
    kind: string; // system/user
    accessType: string; // all/groups/servers/custom
    allowedGroups?: string;
    allowedServers?: string;
    enabled: number;
  }

  /** 签发/轮换返回（dto.McpKeyCreateRes: 明文 token 仅返回一次） */
  export interface CreateResult {
    keyId?: string;
    token?: string;
  }
}

const BASE = '/api/v1/mcp/key';

/** 密钥列表（分页） */
export function getMcpKeyListApi(params: McpKeyApi.ListParams) {
  return requestClient.get<McpKeyApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 单条详情 */
export function getMcpKeyApi(keyId: string) {
  return requestClient.get<McpKeyApi.ListItem>(`${BASE}/get`, {
    params: { keyId },
  });
}

/** 签发（body: dto.McpKeyReq）→ 返回 { keyId, token } */
export function addMcpKeyApi(data: Recordable<any>) {
  return requestClient.post<McpKeyApi.CreateResult>(`${BASE}/add`, data);
}

/** 编辑（body: dto.McpKeyEditReq = { keyId, ...McpKeyReq }） */
export function editMcpKeyApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteMcpKeyApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 封禁/解封（params: { keyId, blocked }）*/
export function blockMcpKeyApi(keyId: string, blocked: boolean) {
  return requestClient.put(`${BASE}/block`, {
    params: { keyId, blocked: blocked ? 1 : 0 },
  });
}

/** 轮换（params: { keyId }）→ 返回 { token } 新明文 */
export function rotateMcpKeyApi(keyId: string) {
  return requestClient.put<McpKeyApi.CreateResult>(`${BASE}/rotate`, {
    params: { keyId },
  });
}
