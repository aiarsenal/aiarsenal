/**
 * gin 登录日志接口（/api/v1/system/loginLog/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 字段名以后端 DTO 为准：
 *   - dto.LoginLogSearchReq: userName/status/ipaddr/loginLocation/dateRange/pageNum/pageSize
 *   - dto.LoginLogSearchRes: { list, currentPage, total }
 *   - dto.LoginLogDelReq: { ids: []string }（DELETE body，非 query）
 *   - entity.LoginLog: infoId/loginName/ipaddr/loginLocation/browser/os/status(int)/msg/loginTime/module
 *
 * 注意：搜索参数 DTO 的 LoginName 字段 json tag = "userName"（非 loginName），
 * 行内 loginName 是实体字段（登录账号），二者不要混淆。
 */
import { requestClient } from '#/api/request';

export namespace LoginLogApi {
  /** 搜索参数（GET query）*/
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    /** 登录账号（DTO LoginName，json=userName）*/
    userName?: string;
    /** 登录状态："1"=成功 "0"=失败 */
    status?: string;
    ipaddr?: string;
    loginLocation?: string;
    /** [开始, 结束] YYYY-MM-DD */
    dateRange?: string[];
  }

  /** 登录日志行（entity.LoginLog）*/
  export interface ListItem {
    infoId: string;
    loginName: string;
    ipaddr: string;
    loginLocation: string;
    browser: string;
    os: string;
    /** 1=成功 0=失败 */
    status: number;
    msg: string;
    loginTime: string;
    module: string;
  }

  /** 列表响应 */
  export interface ListResult {
    list: ListItem[];
    currentPage: number;
    total: number;
  }
}

/** 登录日志列表 */
export function getLoginLogListApi(params: LoginLogApi.ListParams) {
  return requestClient.get<LoginLogApi.ListResult>(
    '/api/v1/system/loginLog/list',
    { params },
  );
}

/** 批量删除登录日志（DELETE body: { ids: []string }）*/
export function deleteLoginLogApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/loginLog/delete', {
    data: { ids },
  });
}

/** 清空登录日志 */
export function clearLoginLogApi() {
  return requestClient.delete('/api/v1/system/loginLog/clear');
}
