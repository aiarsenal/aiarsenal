/**
 * gin 操作日志接口（/api/v1/system/operLog/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 字段名以后端 DTO 为准：
 *   - dto.OperLogSearchReq: title/requestMethod/operName/dateRange/pageNum/pageSize
 *   - dto.OperLogSearchRes: { list, currentPage, total }
 *   - dto.OperLogGetReq: { operId }（GET query）
 *   - dto.OperLogDeleteReq: { operIds: []string }（DELETE body，非 query）
 *   - dto.OperLogClearReq: {} （DELETE 无体）
 *   - model.OperLogListRes: operId/title/requestMethod/operName/deptName/operUrl/operIp/operLocation/operParam/operTime
 *   - model.OperLogInfoRes: 在 list 基础上增加 businessType/method/operatorType/errorMsg
 */
import { requestClient } from '#/api/request';

export namespace OperLogApi {
  /** 搜索参数（GET query）*/
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    /** 系统模块 */
    title?: string;
    /** 请求方式：GET/POST/PUT/DELETE */
    requestMethod?: string;
    /** 操作人员 */
    operName?: string;
    /** [开始, 结束] YYYY-MM-DD */
    dateRange?: string[];
  }

  /** 操作日志行（model.OperLogListRes）*/
  export interface ListItem {
    operId: string;
    title: string;
    requestMethod: string;
    operName: string;
    deptName: string;
    operUrl: string;
    operIp: string;
    operLocation: string;
    operParam: string;
    operTime: string;
  }

  /** 操作日志详情（model.OperLogInfoRes，比 list 多 businessType/method/operatorType/errorMsg）*/
  export interface InfoData extends ListItem {
    businessType: number;
    method: string;
    operatorType: number;
    errorMsg: string;
  }

  /** 列表响应 */
  export interface ListResult {
    list: ListItem[];
    currentPage: number;
    total: number;
  }
}

/** 操作日志列表 */
export function getOperLogListApi(params: OperLogApi.ListParams) {
  return requestClient.get<OperLogApi.ListResult>(
    '/api/v1/system/operLog/list',
    { params },
  );
}

/** 操作日志详情（GET query: { operId }）*/
export function getOperLogApi(operId: string) {
  return requestClient.get<OperLogApi.InfoData>(
    '/api/v1/system/operLog/get',
    { params: { operId } },
  );
}

/** 批量删除操作日志（DELETE body: { operIds: []string }）*/
export function deleteOperLogApi(operIds: string[]) {
  return requestClient.delete('/api/v1/system/operLog/delete', {
    data: { operIds },
  });
}

/** 清空操作日志 */
export function clearOperLogApi() {
  return requestClient.delete('/api/v1/system/operLog/clear');
}
