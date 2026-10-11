/**
 * gin 角色管理接口（/api/v1/system/role/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 字段名以 gin DTO 为准（api/interfaces/dto/role.go + entity.Role）：
 * - list 行 `id` = roleId（业务 id，对外即 id）；entity.Role json tag: id/status/listOrder/name/remark/createdAt
 * - add 不带 id；edit 体 `id` = roleId
 * - menuIds 为 []string
 */

import { requestClient } from '#/api/request';

export namespace RoleApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    roleName?: string;
    roleStatus?: number | string;
  }

  export interface ListItem {
    /** roleId（业务 id） */
    id: string;
    name: string;
    status: number;
    listOrder: number;
    remark?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage: number;
  }

  /** /role/getParams → { menu: MenuInfoRes[] }（扁平，含 id/pid，前端建树） */
  export interface MenuNode {
    id: string;
    pid: string;
    name?: string;
    title?: string;
    children?: MenuNode[];
    [key: string]: any;
  }

  export interface GetParamsResult {
    menu: MenuNode[];
  }

  /** /role/get → { role, menuIds } */
  export interface GetResult {
    role: ListItem;
    menuIds: string[];
  }

  export interface AddReq {
    name: string;
    status: number;
    listOrder: number;
    remark?: string;
    menuIds: string[];
  }

  export interface EditReq {
    /** roleId */
    id: string;
    name: string;
    status: number;
    listOrder: number;
    remark?: string;
    menuIds: string[];
  }
}

/** 角色列表 */
export function getRoleListApi(params: RoleApi.ListParams) {
  return requestClient.get<RoleApi.ListResult>(
    '/api/v1/system/role/list',
    { params },
  );
}

/** 获取菜单树（扁平列表，前端建树） */
export function getRoleParamsApi() {
  return requestClient.get<RoleApi.GetParamsResult>(
    '/api/v1/system/role/getParams',
  );
}

/** 编辑回显：{ role, menuIds } */
export function getRoleApi(id: string) {
  return requestClient.get<RoleApi.GetResult>(
    '/api/v1/system/role/get',
    { params: { id } },
  );
}

/** 新增 */
export function addRoleApi(data: RoleApi.AddReq) {
  return requestClient.post('/api/v1/system/role/add', data);
}

/** 编辑 */
export function editRoleApi(data: RoleApi.EditReq) {
  return requestClient.put('/api/v1/system/role/edit', data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteRoleApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/role/delete', {
    data: { ids },
  });
}
