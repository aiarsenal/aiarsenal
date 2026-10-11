/**
 * gin MCP 内置 Prompt 接口（/api/v1/mcp/prompt/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpPromptApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    name?: string;
  }

  export interface ListItem {
    promptId: string;
    name: string;
    title?: string;
    description?: string;
    template: string;
    arguments?: string;
    enabled: number;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage: number;
  }

  export interface PromptPayload {
    name: string;
    title?: string;
    description?: string;
    template: string;
    arguments?: string;
    enabled: number;
  }
}

/** 列表 */
export function getMcpPromptListApi(params: McpPromptApi.ListParams) {
  return requestClient.get<McpPromptApi.ListResult>(
    '/api/v1/mcp/prompt/list',
    { params },
  );
}

/** 详情 */
export function getMcpPromptApi(promptId: string) {
  return requestClient.get<McpPromptApi.ListItem>(
    '/api/v1/mcp/prompt/get',
    { params: { promptId } },
  );
}

/** 新增 */
export function addMcpPromptApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/mcp/prompt/add', data);
}

/** 编辑（gin: { promptId, ...McpPromptReq }）*/
export function editMcpPromptApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/mcp/prompt/edit', data);
}

/** 删除（批量，body: { ids: [] }）*/
export function deleteMcpPromptApi(ids: string[]) {
  return requestClient.delete('/api/v1/mcp/prompt/delete', {
    data: { ids },
  });
}
