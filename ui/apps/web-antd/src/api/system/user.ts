/**
 * gin 用户管理接口（/api/v1/system/user/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace UserApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    keyWords?: string;
    mobile?: string;
    status?: number | string;
    deptId?: string;
    dateRange?: string[];
  }

  export interface ListItem {
    id: string;
    userId?: string;
    userName: string;
    name: string;
    employeeNo?: string;
    userNickname?: string;
    mobile?: string;
    userStatus: number;
    // gin 复用 UserRoleInfoRes：roleId 装部门id，name 装部门名
    dept?: { name?: string; roleId?: string; };
    roleInfo?: { name?: string; roleId?: string; }[];
    createdAt?: string;
  }

  export interface ListResult {
    userList: ListItem[];
    total: number;
  }

  export interface UserParams {
    roleList?: { id: string; name: string }[];
    posts?: { postId: string; postName: string }[];
  }
}

/** 用户列表 */
export function getUserListApi(params: UserApi.ListParams) {
  return requestClient.get<UserApi.ListResult>('/api/v1/system/user/list', {
    params,
  });
}

/** 新增/编辑表单所需的角色、岗位下拉数据（gin: { roleList, posts }，无 depts）*/
export function getUserParamsApi() {
  return requestClient.get<UserApi.UserParams>('/api/v1/system/user/params');
}

/** 部门树（gin: /system/dept/treeSelect → { deps }，节点 deptId/deptName/children）*/
export function getDeptTreeApi() {
  return requestClient.get<{ deps: any[] }>(
    '/api/v1/system/dept/treeSelect',
  );
}

/** 编辑回显 */
export function getEditUserApi(id: string) {
  return requestClient.get('/api/v1/system/user/getEdit', { params: { id } });
}

/** 新增 */
export function addUserApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/system/user/add', data);
}

/** 编辑 */
export function editUserApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/system/user/edit', data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteUserApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/user/delete', { data: { ids } });
}

/** 重置密码（gin: { userId, password }）*/
export function resetUserPwdApi(id: string, password: string) {
  return requestClient.put('/api/v1/system/user/resetPwd', {
    userId: id,
    password,
  });
}

/** 切换状态（gin: { userId, status }）*/
export function setUserStatusApi(id: string, userStatus: number) {
  return requestClient.put('/api/v1/system/user/setStatus', {
    userId: id,
    status: userStatus,
  });
}

/** 人员选择器搜索 */
export function searchUserOptionsApi(params: Recordable<any>) {
  return requestClient.get('/api/v1/system/user/search', { params });
}

export interface UserOptionItem {
  id: string;
  name: string;
  employeeNo?: string;
  mobile?: string;
  userEmail?: string;
  deptName?: string;
}

/** 按用户 id 批量解析姓名/工号/部门（授权条件行已选用户回显；上限 200） */
export function resolveUserOptionsApi(ids: string[]) {
  return requestClient.get<{ list: UserOptionItem[] }>(
    '/api/v1/system/user/resolve',
    { params: { ids: ids.join(',') } },
  );
}
