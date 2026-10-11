/**
 * Agent 侧用户接口（/api/v1/agent/user/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * Agent 用户 = 实例内 agent（openclaw/hermes）自己的账号体系，与平台 aa_user 无关：
 * 一个 agent 用户可绑定多台实例（instanceIds），凭 agent 侧登录凭据使用被绑实例。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace AgentUserApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    keyword?: string;
    status?: number | '';
  }

  export interface UserItem {
    userId: string;
    username: string;
    displayName: string;
    /** admin=管理员 operator=操作员 member=成员 */
    role: 'admin' | 'member' | 'operator';
    enabled: number; // 1=启用 0=停用
    /** 绑定实例 id 列表 */
    instanceIds: string[];
    /** 绑定实例名（后端富化，展示用） */
    instanceNames: string[];
    remark?: string;
    lastLoginAt?: string;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: UserItem[];
    total: number;
    currentPage?: number;
  }

  /** 新建/编辑请求（edit 带 userId；instanceIds 整组覆盖） */
  export interface UserReq {
    userId?: string;
    username: string;
    displayName: string;
    role: 'admin' | 'member' | 'operator';
    enabled: number;
    instanceIds: string[];
    remark?: string;
  }

  /** 新建返回（初始密码仅返回一次） */
  export interface CreateResult {
    userId: string;
    initialPassword?: string;
  }

  /** 重置密码返回（新密码仅返回一次） */
  export interface ResetPasswordResult {
    password: string;
  }
}

const BASE = '/api/v1/agent/user';

/** Agent 用户列表（分页） */
export function getAgentUserListApi(params: AgentUserApi.ListParams) {
  return requestClient.get<AgentUserApi.ListResult>(`${BASE}/list`, { params });
}

/** 新建 Agent 用户（初始密码仅返回一次） */
export function addAgentUserApi(data: AgentUserApi.UserReq) {
  return requestClient.post<AgentUserApi.CreateResult>(`${BASE}/add`, data);
}

/** 编辑 Agent 用户（body 带 userId） */
export function editAgentUserApi(data: AgentUserApi.UserReq) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteAgentUserApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 重置密码（返回新密码，仅显示一次） */
export function resetAgentUserPasswordApi(userId: string) {
  return requestClient.put<AgentUserApi.ResetPasswordResult>(
    `${BASE}/reset-password`,
    { userId },
  );
}
