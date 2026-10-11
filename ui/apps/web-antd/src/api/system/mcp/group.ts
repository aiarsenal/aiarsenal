/**
 * gin MCP 分组接口（/api/v1/mcp/group/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * gin DTO（mcp.go）字段名权威来源：
 *   McpGroupSearchReq  { name, pageNum, pageSize }
 *   McpGroupSearchRes  { list, total, currentPage }
 *   McpGroupReq        { name, description, servers }            // servers 为 jsonb 字符串
 *   McpGroupEditReq    { groupId, name, description, servers }
 *   McpGroupGrantGetReq  { groupId }
 *   McpGroupGrantSaveReq { groupId, visibility, rules }          // rules 为 GrantRow 数组 JSON 文本
 *   McpIdsReq          { ids: [] }
 * 行 entity.McpGroup: { groupId, name, description, servers, visibility, canManage, owner, createdBy, ... }
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpGroupApi {
  export interface ListParams {
    name?: string;
    pageNum: number;
    pageSize: number;
  }

  export interface ListItem {
    groupId: string;
    name: string;
    description?: string;
    servers?: string; // jsonb 数组字符串
    visibility?: number; // 0私有 1按条件 2全员
    canManage?: boolean; // 当前身份可管理（编辑/删除/配授权）
    owner?: string;
    ownerName?: string;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  export interface GroupForm {
    groupId?: string;
    name: string;
    description?: string;
    servers?: string;
  }

  /** 条件行（与 GrantEditor 的 GrantRowLike 对齐；后端 model.GrantRow） */
  export interface GrantRowLike {
    userIds?: string[];
    deptId?: string;
    roleIds?: string[];
    postIds?: string[];
    exclude?: boolean;
    groupId?: string;
    setId?: string;
  }

  /** 授权配置回显（service.McpGroupGrantInfo） */
  export interface GrantInfo {
    visibility: number;
    rules: GrantRowLike[];
  }

  /** 授权配置保存 */
  export interface GrantSaveParams {
    groupId: string;
    visibility: number;
    rules: string; // GrantRow 数组 JSON 文本
  }
}

/** 分组列表 */
export function getMcpGroupListApi(params: McpGroupApi.ListParams) {
  return requestClient.get<McpGroupApi.ListResult>(
    '/api/v1/mcp/group/list',
    { params },
  );
}

/** 全部分组（无分页） */
export function getAllMcpGroupsApi() {
  return requestClient.get<McpGroupApi.ListItem[]>(
    '/api/v1/mcp/group/all',
  );
}

/** 单个分组 */
export function getMcpGroupApi(groupId: string) {
  return requestClient.get<McpGroupApi.ListItem>(
    '/api/v1/mcp/group/get',
    { params: { groupId } },
  );
}

/** 新增 */
export function addMcpGroupApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/mcp/group/add', data);
}

/** 编辑 */
export function editMcpGroupApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/mcp/group/edit', data);
}

/** 删除（批量，body: { ids: [] }，ids = groupId） */
export function deleteMcpGroupApi(ids: string[]) {
  return requestClient.delete('/api/v1/mcp/group/delete', { data: { ids } });
}

/** 授权配置回显 */
export function getMcpGroupGrantApi(groupId: string) {
  return requestClient.get<McpGroupApi.GrantInfo>(
    '/api/v1/mcp/group/grant/get',
    { params: { groupId } },
  );
}

/** 授权配置保存（可见性档位 + 条件行整体重写） */
export function saveMcpGroupGrantApi(data: McpGroupApi.GrantSaveParams) {
  return requestClient.post('/api/v1/mcp/group/grant/save', data);
}
