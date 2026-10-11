/**
 * gin MCP 系统配置 / 用户级配置接口（/api/v1/mcp/config/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 entity.McpConfig / entity.McpUserConfig 为准（controller 直接 ShouldBindJSON entity）。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace McpConfigApi {
  /** 系统配置（单例 id='default'，entity.McpConfig） */
  export interface SystemConfig {
    id?: string;
    routing?: string;
    install?: string;
    toolResultCompression?: string;
    mcpRouter?: string;
    nameSeparator?: string;
    auth?: string;
    discovery?: string;
    activityLog?: string;
    enableSessionRebuild?: number;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 用户级配置（entity.McpUserConfig；userId 由 JWT 注入，前端不传） */
  export interface UserConfig {
    id?: number;
    userId?: string;
    routing?: string;
    additionalConfig?: string;
    createdAt?: string;
    updatedAt?: string;
  }
}

/** 获取系统配置（gin: GET /mcp/config/get） */
export function getMcpConfigApi() {
  return requestClient.get<McpConfigApi.SystemConfig>(
    '/api/v1/mcp/config/get',
  );
}

/** 保存系统配置（gin: PUT /mcp/config/edit，body: McpConfig） */
export function saveMcpConfigApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/mcp/config/edit', data);
}

/** 获取当前用户级配置（gin: GET /mcp/config/user/get，userId 取自 JWT） */
export function getMcpUserConfigApi() {
  return requestClient.get<McpConfigApi.UserConfig>(
    '/api/v1/mcp/config/user/get',
  );
}

/** 保存当前用户级配置（gin: PUT /mcp/config/user/edit，body: McpUserConfig；userId 后端覆写） */
export function saveMcpUserConfigApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/mcp/config/user/edit', data);
}
