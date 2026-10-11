/**
 * gin LLM 虚拟密钥接口（/api/v1/llm/key/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 注意：list 行的 tokenHash 在 entity 上为 `json:"-"`（见 LLMVirtualKey），
 * 当前后端未外露；但 block/rotate/get/delete 均以 tokenHash 为键，
 * 故页面仍按 row.tokenHash 操作。后端需放开该字段序列化后页面方能正常工作。
 */

import { requestClient } from '#/api/request';

export namespace LLMKeyApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    alias?: string;
    userId?: string;
    teamId?: string;
  }

  export interface ListItem {
    tokenHash: string;
    keyAlias: string;
    userId: string;
    teamId: string;
    projectId?: string;
    orgId?: string;
    spend?: number;
    maxBudget?: number | null;
    softBudget?: number | null;
    budgetId?: string;
    tpmLimit?: number | null;
    rpmLimit?: number | null;
    maxParallelRequests?: number | null;
    budgetDuration?: string;
    allowedModels?: string; // JSON 数组串
    allowedRoutes?: string;
    expires?: string;
    blocked?: number;
    lastActive?: string;
    createdAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  export interface KeyReq {
    keyAlias: string;
    userId?: string;
    teamId?: string;
    projectId?: string;
    orgId?: string;
    models?: string;
    maxBudget?: number;
    softBudget?: number;
    budgetId?: string;
    tpmLimit?: number;
    rpmLimit?: number;
    maxParallelRequests?: number;
    budgetDuration?: string;
    allowedModels?: string; // 提交时 JSON.stringify(string[])
    allowedRoutes?: string;
    expires?: string; // RFC3339
    blocked?: number;
  }

  export interface EditReq extends KeyReq {
    tokenHash: string;
  }

  export interface CreateResult {
    tokenHash: string;
    key: string; // 明文，仅此一次返回
    graceMinutes?: number; // 轮换时旧 key 宽限分钟数
  }
}

const BASE = '/api/v1/llm/key';

/** 虚拟密钥列表 */
export function getLLMKeyListApi(params: LLMKeyApi.ListParams) {
  return requestClient.get<LLMKeyApi.ListResult>(`${BASE}/list`, { params });
}

/** 单条详情（tokenHash 为查询参数） */
export function getLLMKeyApi(tokenHash: string) {
  return requestClient.get<LLMKeyApi.ListItem>(`${BASE}/get`, {
    params: { tokenHash },
  });
}

/** 新增（成功返回明文 key，仅此一次） */
export function addLLMKeyApi(data: LLMKeyApi.KeyReq) {
  return requestClient.post<LLMKeyApi.CreateResult>(`${BASE}/add`, data);
}

/** 编辑（body 含 tokenHash） */
export function editLLMKeyApi(data: LLMKeyApi.EditReq) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [...] }，ids 为 tokenHash 数组） */
export function deleteLLMKeyApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 删除前查该 key 是否被实例使用（前端据此对被使用的 key 做二次确认） */
export function checkLLMKeyUseApi(tokenHash: string) {
  return requestClient.get<{ usedByInstance: boolean; instanceName: string }>(
    `${BASE}/check-use`,
    { params: { tokenHash } },
  );
}

/** 封禁/解封（query: tokenHash, blocked=1|0） */
export function blockLLMKeyApi(tokenHash: string, blocked: boolean) {
  return requestClient.put(`${BASE}/block`, null, {
    params: { tokenHash, blocked: blocked ? 1 : 0 },
  });
}

/** 轮换（query: tokenHash，返回新明文 + 旧 key 5min 宽限） */
export function rotateLLMKeyApi(tokenHash: string) {
  return requestClient.put<LLMKeyApi.CreateResult>(`${BASE}/rotate`, null, {
    params: { tokenHash },
  });
}

/**
 * 表单辅助：已部署模型列表（用于 allowedModels 多选）。
 * 复用 /llm/model/all 端点，返回行含 { modelId, modelName }。
 */
export function getLLMModelAllApi() {
  return requestClient.get<{ list?: any[] } | any[]>(
    '/api/v1/llm/model/all',
  );
}
