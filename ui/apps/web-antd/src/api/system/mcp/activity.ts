/**
 * gin MCP 调用审计接口（/api/v1/mcp/activity/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 dto.McpActivitySearchReq / dto.McpActivitySearchRes 为准。
 * 只读模块：仅有 list 端点，无 add/edit/delete。
 */
import { requestClient } from '#/api/request';

export namespace McpActivityApi {
  /** 列表查询参数（dto.McpActivitySearchReq） */
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    serverName?: string;
    tool?: string;
    status?: string;
    userId?: string;
    dateRange?: string[];
  }

  /** 列表行（entity.McpActivity） */
  export interface ListItem {
    serverName: string;
    tool: string;
    durationMs: number;
    status: string; // success | error
    input?: string;
    output?: string;
    groupName?: string;
    userId?: string;
    keyId?: string;
    keyName?: string;
    sourceIp?: string;
    errorMessage?: string;
    ts?: string;
  }

  /** 列表返回（dto.McpActivitySearchRes: { list, total, currentPage }） */
  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }
}

const BASE = '/api/v1/mcp/activity';

/** 调用审计列表（分页，只读） */
export function getMcpActivityListApi(params: McpActivityApi.ListParams) {
  return requestClient.get<McpActivityApi.ListResult>(`${BASE}/list`, {
    params,
  });
}
