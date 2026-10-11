/**
 * gin 在线用户接口（/api/v1/system/online/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 字段名以后端 DTO 为准：
 *   - dto.UserOnlineSearchReq: userName / ipaddr / pageNum / pageSize
 *     （DTO Username 字段 json tag = "userName"）
 *   - dto.UserOnlineSearchRes: { list, currentPage, total }
 *   - dto.UserOnlineDeleteReq: { uuid: string }（DELETE body，单条，required）
 *     注意：旧 el 页用 forceLogout(ids[]) 调 /forceLogout，与现 gin 路由不一致，
 *     此处以 gin 为准——单条 uuid 调 /online/delete。
 *   - entity.UserOnline: id(=uuid 会话标识) / token / createTime / userName / ip / explorer / os
 */
import { requestClient } from '#/api/request';

export namespace UserOnlineApi {
  /** 搜索参数（GET query）*/
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    /** 登录账号（DTO Username，json=userName）*/
    userName?: string;
    /** 登录 IP */
    ipaddr?: string;
  }

  /** 在线用户行（entity.UserOnline，json 字段）*/
  export interface ListItem {
    /** 会话标识 = uuid（md5 of token），对外即 id */
    id: string;
    token?: string;
    createTime?: string;
    userName: string;
    ip?: string;
    explorer?: string;
    os?: string;
  }

  /** 列表响应 */
  export interface ListResult {
    list: ListItem[];
    currentPage: number;
    total: number;
  }
}

/** 在线用户列表 */
export function getUserOnlineListApi(params: UserOnlineApi.ListParams) {
  return requestClient.get<UserOnlineApi.ListResult>(
    '/api/v1/system/online/list',
    { params },
  );
}

/** 强制下线（DELETE body: { uuid }，单条）*/
export function forceLogoutApi(uuid: string) {
  return requestClient.delete('/api/v1/system/online/delete', {
    data: { uuid },
  });
}
