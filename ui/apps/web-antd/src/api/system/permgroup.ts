/**
 * gin 权限分组管理接口（/api/v1/system/permgroup/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * gin（controller/permgroup.go + dto/permgroup.go）：
 *   group/list   {name}                    → {list: []*entity.PermGroup}
 *   group/add    {id?, name, remark, terms}（terms = GrantRow[] JSON 文本）
 *   group/edit   同 add（id 非空 = 编辑）
 *   group/delete {ids []string}（groupId 业务 id）
 *   set/*        同构（members = 工号数组 JSON 文本）
 *   options      → {groups:[{id,name}], sets:[{id,name}]}（瘦投影，登录即可见）
 *
 * entity 行字段：groupId/name/remark/terms/createdBy/createdAt/updatedAt；
 *               setId/name/remark/empNos/…
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace PermGroupApi {
  export interface GroupItem {
    groupId: string;
    name: string;
    remark?: string;
    terms?: string;
    createdBy?: string;
    createdByDesc?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface SetItem {
    setId: string;
    name: string;
    remark?: string;
    empNos?: string;
    createdBy?: string;
    createdByDesc?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 集合成员明细项（工号 → 姓名/部门；解析不到 = 在册外） */
  export interface MemberInfo {
    employeeNo: string;
    name: string;
    deptName: string;
  }

  /** GrantEditor 选择器瘦投影项 */
  export interface OptionItem {
    id: string;
    name: string;
  }

  export interface OptionsResult {
    groups: OptionItem[];
    sets: OptionItem[];
  }
}

/** 权限分组列表 */
export function getPermGroupListApi(params: Recordable<any>) {
  return requestClient.get<{ list: PermGroupApi.GroupItem[] }>(
    '/api/v1/system/permgroup/group/list',
    { params },
  );
}

/** 新增权限分组（terms 为 GrantRow[] JSON 文本） */
export function addPermGroupApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/system/permgroup/group/add', data);
}
export function editPermGroupApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/system/permgroup/group/edit', data);
}

/** 删除权限分组（body: { ids: [] }，ids 为 groupId 业务 id） */
export function deletePermGroupApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/permgroup/group/delete', {
    data: { ids },
  });
}

/** 员工集合列表 */
export function getEmpSetListApi(params: Recordable<any>) {
  return requestClient.get<{ list: PermGroupApi.SetItem[] }>(
    '/api/v1/system/permgroup/set/list',
    { params },
  );
}

/** 新增/编辑员工集合（id 空 = 新增；members 为工号数组 JSON 文本） */
export function addEmpSetApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/system/permgroup/set/add', data);
}
export function editEmpSetApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/system/permgroup/set/edit', data);
}

/** 删除员工集合（body: { ids: [] }，ids 为 setId 业务 id） */
export function deleteEmpSetApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/permgroup/set/delete', {
    data: { ids },
  });
}

/** 工号批量解析姓名/部门（集合弹窗成员明细；上限 500） */
export function resolveEmpNosApi(empNos: string[]) {
  return requestClient.post<{ list: PermGroupApi.MemberInfo[] }>(
    '/api/v1/system/permgroup/set/resolve',
    { empNos },
  );
}

/** 分组/集合瘦投影（GrantEditor 选项；登录即可见） */
export function getPermGroupOptionsApi() {
  return requestClient.get<PermGroupApi.OptionsResult>(
    '/api/v1/system/permgroup/options',
  );
}
