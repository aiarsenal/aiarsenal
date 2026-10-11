/**
 * gin MCP 服务器配置接口（/api/v1/mcp/server/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 dto.McpServerReq / dto.McpServerSearchRes 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpServerApi {
  /** 列表查询参数（dto.McpServerSearchReq） */
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
    type?: string;
  }

  /** 列表行（entity.McpServer，仅列常用字段） */
  export interface ListItem {
    serverId: string;
    name: string;
    type: string;
    description?: string;
    url?: string;
    command?: string;
    args?: string;
    env?: string;
    headers?: string;
    options?: string;
    oauth?: string;
    proxy?: string;
    openapi?: string;
    passthroughHeaders?: string;
    enabled: number;
    owner?: string;
    visibility?: string;
    enableKeepAlive?: number;
    keepAliveInterval?: number;
    tools?: string;
    prompts?: string;
    resources?: string;
    createdBy?: string;
    createdAt?: string;
  }

  /** 列表返回（dto.McpServerSearchRes: { list, total, currentPage }） */
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  /** 新增/编辑表单 payload（dto.McpServerReq） */
  export interface ServerForm {
    name: string;
    type: string;
    description?: string;
    url?: string;
    command?: string;
    args?: string;
    env?: string;
    headers?: string;
    options?: string;
    oauth?: string;
    proxy?: string;
    openapi?: string;
    passthroughHeaders?: string;
    enabled: number;
    visibility: string;
    enableKeepAlive?: number;
    keepAliveInterval?: number;
    tools?: string;
    prompts?: string;
    resources?: string;
  }

  /** 连接测试参数（dto.McpServerTestReq：serverId 与表单二选一） */
  export interface TestParams {
    serverId?: string;
    [key: string]: any;
  }

  /** 工具预览行（dto.McpToolPreview） */
  export interface ToolPreview {
    name: string;
    description?: string;
    inputSchema?: any;
  }

  /** 连接测试返回（dto.McpServerTestRes） */
  export interface TestResult {
    ok: boolean;
    message?: string;
    durationMs: number;
    tools: ToolPreview[];
    prompts: { name: string; description?: string }[];
    resources: {
      name: string;
      description?: string;
      mimeType?: string;
      uri?: string;
    }[];
  }

  /** 试调用返回（dto.McpServerCallRes） */
  export interface CallResult {
    durationMs: number;
    text: string;
  }
}

const BASE = '/api/v1/mcp/server';

/** 服务器列表（分页） */
export function getMcpServerListApi(params: McpServerApi.ListParams) {
  return requestClient.get<McpServerApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 全部服务器（不分页，供下拉/分组引用） */
export function getAllMcpServersApi() {
  return requestClient.get<McpServerApi.ListItem[]>(`${BASE}/all`);
}

/** 单条详情 */
export function getMcpServerApi(serverId: string) {
  return requestClient.get<McpServerApi.ListItem>(`${BASE}/get`, {
    params: { serverId },
  });
}

/** 新增（body: dto.McpServerReq） */
export function addMcpServerApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/add`, data);
}

/** 编辑（body: dto.McpServerEditReq = { serverId, ...McpServerReq }） */
export function editMcpServerApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteMcpServerApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 连接测试 + 能力发现（body: dto.McpServerTestReq） */
export function testMcpServerApi(data: McpServerApi.TestParams) {
  return requestClient.post<McpServerApi.TestResult>(`${BASE}/test`, data);
}

/** 在线试调用（body: dto.McpServerCallReq） */
export function callMcpServerToolApi(data: {
  serverId: string;
  tool: string;
  arguments: Record<string, any>;
}) {
  return requestClient.post<McpServerApi.CallResult>(`${BASE}/callTool`, data);
}
