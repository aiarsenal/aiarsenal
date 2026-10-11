/**
 * gin 部门管理接口（/api/v1/system/dept/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */

import { requestClient } from '#/api/request';

export namespace DeptApi {
  /** 部门列表查询参数（gin: DeptSearchReq）*/
  export interface ListParams {
    deptName?: string;
    status?: number | string;
  }

  /** 部门列表行（gin: model.DeptInfo，扁平，前端按 parentId 转树）*/
  export interface ListItem {
    deptId: string;
    parentId: string;
    deptName: string;
    orderNum: number;
    leaderId: string;
    leaderName: string;
    leaderEmployeeNo: string;
    status: number;
    createdAt: string;
  }

  /** 部门列表结果（gin: DeptSearchRes → { deptList }）*/
  export interface ListResult {
    deptList: ListItem[];
  }

  /** 部门树节点（gin: DeptTreeNode）*/
  export interface TreeNode {
    deptId: string;
    parentId: string;
    deptName: string;
    children?: TreeNode[];
  }

  /** 部门树结果（gin: DeptTreeSelectRes → { deps }）*/
  export interface TreeResult {
    deps: TreeNode[];
  }

  /** 新增/编辑表单（gin: DeptAddReq / DeptEditReq）*/
  export interface WritePayload {
    deptId?: string;
    parentId: string;
    deptName: string;
    orderNum: number;
    leaderId: string;
    status: number;
  }
}

/** 部门列表（扁平）*/
export function getDeptListApi(params?: DeptApi.ListParams) {
  return requestClient.get<DeptApi.ListResult>(
    '/api/v1/system/dept/list',
    { params },
  );
}

/** 部门树（用于父级选择）*/
export function getDeptTreeApi() {
  return requestClient.get<DeptApi.TreeResult>(
    '/api/v1/system/dept/treeSelect',
  );
}

/** 新增 */
export function addDeptApi(data: DeptApi.WritePayload) {
  return requestClient.post('/api/v1/system/dept/add', data);
}

/** 编辑 */
export function editDeptApi(data: DeptApi.WritePayload) {
  return requestClient.put('/api/v1/system/dept/edit', data);
}

/** 删除（gin: DeptDeleteReq → { id }）*/
export function deleteDeptApi(id: string) {
  return requestClient.delete('/api/v1/system/dept/delete', { data: { id } });
}

/** 人员选择器搜索（复用 user 模块，负责人远程搜索）*/
export { searchUserOptionsApi } from '#/api/system/user';
