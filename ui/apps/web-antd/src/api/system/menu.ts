/**
 * gin 菜单管理接口（/api/v1/system/menu/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段名以 dto/menu.go 为准。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace MenuApi {
  export interface SearchParams {
    menuName?: string;
    component?: string;
  }

  /** 菜单/规则投影（model.MenuInfoRes），无搜索时 rules 为树，有搜索时为扁平 */
  export interface MenuItem {
    id: string;
    pid: string;
    name: string;
    title: string;
    enName?: string;
    icon?: string;
    condition?: string;
    remark?: string;
    menuType: number;
    weigh: number;
    isHide: number;
    isCached?: number;
    isAffix?: number;
    path?: string;
    redirect?: string;
    component?: string;
    isIframe?: number;
    isLink?: number;
    linkUrl?: string;
    children?: MenuItem[];
  }

  export interface ListResult {
    rules: MenuItem[];
  }

  export interface ParamsResult {
    roles: { id: string; name: string }[];
    menus: MenuItem[];
  }

  export interface InfoResult {
    rule: MenuItem;
    roleIds: string[];
  }

  export interface AddReq {
    menuType: number;
    parentId: string;
    name: string;
    menuName: string;
    enName?: string;
    icon?: string;
    menuSort?: number;
    condition?: string;
    remark?: string;
    isHide?: number;
    path?: string;
    redirect?: string;
    component?: string;
    isLink?: number;
    isIframe?: number;
    isKeepAlive?: number;
    isAffix?: number;
    linkUrl?: string;
    /** 组合授权条件行 JSON（[]{userIds,deptId,roleIds,postIds,exclude}） */
    grantRules?: string;
  }

  export interface UpdateReq extends AddReq {
    id: string;
  }
}

/** 菜单列表（无搜索条件 → 树；有条件 → 扁平） */
export function getMenuListApi(params: MenuApi.SearchParams) {
  return requestClient.get<MenuApi.ListResult>(
    '/api/v1/system/menu/list',
    { params },
  );
}

/** 新增/编辑表单所需的角色、上级菜单（gin: { roles, menus }，menus 为扁平）*/
export function getMenuParamsApi() {
  return requestClient.get<MenuApi.ParamsResult>(
    '/api/v1/system/menu/getParams',
  );
}

/** 编辑回显（gin: { rule, roleIds }）*/
export function getMenuInfoApi(id: string) {
  return requestClient.get<MenuApi.InfoResult>(
    '/api/v1/system/menu/get',
    { params: { id } },
  );
}

/** 新增 */
export function addMenuApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/system/menu/add', data);
}

/** 修改 */
export function updateMenuApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/system/menu/update', data);
}

/** 删除（批量，body: { ids: [] }） */
export function deleteMenuApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/menu/delete', {
    data: { ids },
  });
}
