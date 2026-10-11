/**
 * MCP OAuth 客户端接口（/api/v1/mcp/oauth/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * OAuth 客户端 = 第三方应用接入 MCP 网关 OAuth 2.0 的身份（client_id/client_secret），
 * 与 Bearer 密钥（mcp/key）并行的一套鉴权通道；令牌（token）= 客户端换出的访问凭据。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace McpOauthApi {
  /** 客户端列表行 */
  export interface ClientItem {
    clientId: string;
    clientName: string;
    /** confidential=带密钥机密客户端 public=公共客户端（PKCE） */
    clientType: 'confidential' | 'public';
    redirectUris: string[];
    /** authorization_code / client_credentials / refresh_token */
    grantTypes: string[];
    scopes: string[];
    enabled: number; // 1=启用 0=停用
    /** 有效令牌数（后端富化） */
    tokenCount: number;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListParams {
    pageNum: number;
    pageSize: number;
    keyword?: string;
  }

  export interface ListResult {
    list: ClientItem[];
    total: number;
    currentPage?: number;
  }

  /** 新建请求 */
  export interface ClientReq {
    clientName: string;
    clientType: 'confidential' | 'public';
    redirectUris: string[];
    grantTypes: string[];
    scopes: string[];
    enabled: number;
  }

  /** 新建/轮换密钥返回（clientSecret 仅返回一次） */
  export interface SecretResult {
    clientId: string;
    clientSecret?: string;
  }

  /** 令牌列表行 */
  export interface TokenItem {
    tokenId: string;
    clientId: string;
    clientName?: string;
    /** 授权主体（用户/服务账号，后端富化展示名） */
    subject?: string;
    scope?: string;
    /** 访问令牌前 8 位（脱敏） */
    accessTokenPreview?: string;
    issuedAt?: string;
    expiresAt?: string;
    lastUsedAt?: string;
    /** 1=已吊销 0=有效 */
    revoked: number;
  }

  export interface TokenListResult {
    list: TokenItem[];
    total: number;
    currentPage?: number;
  }
}

const CLIENT_BASE = '/api/v1/mcp/oauth/client';
const TOKEN_BASE = '/api/v1/mcp/oauth/token';

/** OAuth 客户端列表（分页） */
export function getMcpOauthClientListApi(params: McpOauthApi.ListParams) {
  return requestClient.get<McpOauthApi.ListResult>(`${CLIENT_BASE}/list`, {
    params,
  });
}

/** 新建客户端（client_secret 仅返回一次；public 类型不含 secret） */
export function addMcpOauthClientApi(data: McpOauthApi.ClientReq) {
  return requestClient.post<McpOauthApi.SecretResult>(`${CLIENT_BASE}/add`, data);
}

/** 编辑客户端（body 带 clientId；redirectUris/grantTypes/scopes 整组覆盖） */
export function editMcpOauthClientApi(
  data: McpOauthApi.ClientReq & { clientId: string },
) {
  return requestClient.put(`${CLIENT_BASE}/edit`, data);
}

/** 删除客户端（批量，body: { ids: [] }；级联吊销其全部令牌） */
export function deleteMcpOauthClientApi(ids: string[]) {
  return requestClient.delete(`${CLIENT_BASE}/delete`, { data: { ids } });
}

/** 轮换 client_secret（旧 secret 立即失效，新 secret 仅返回一次） */
export function rotateMcpOauthClientSecretApi(clientId: string) {
  return requestClient.post<McpOauthApi.SecretResult>(
    `${CLIENT_BASE}/rotate-secret`,
    { clientId },
  );
}

/** 令牌列表（clientId 空=全部客户端） */
export function getMcpOauthTokenListApi(params: {
  clientId?: string;
  pageNum: number;
  pageSize: number;
}) {
  return requestClient.get<McpOauthApi.TokenListResult>(`${TOKEN_BASE}/list`, {
    params,
  });
}

/** 吊销令牌（body: { tokenId }） */
export function revokeMcpOauthTokenApi(tokenId: string) {
  return requestClient.post(`${TOKEN_BASE}/revoke`, { tokenId });
}
