/**
 * gin 桌面客户端运行状态接口（/api/v1/system/client/admin/status/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 仅一个只读端点：GET /status/list → { list: ClientAgentStatus[] }
 * 字段来源：entity.ClientAgentStatus（api/domain/entity/client.go）
 */
import { requestClient } from '#/api/request';

export namespace ClientStatusApi {
  /** 桌面客户端运行状态行（每设备每客户端一行）*/
  export interface ListItem {
    /** 业务 id（= biz id，列表行 id）*/
    statusId: string;
    /** 设备持久 id */
    deviceId: string;
    /** 用户 id */
    userId: string;
    /** agent key：openclaw / hermes / obsidian 等 */
    agent: string;
    /** running / stopped / installing / error / notinstalled */
    status: string;
    version: string;
    /** 运行时长（秒）：running 且活跃时后端按 now−createdAt 计算，其余为 0 */
    uptimeSeconds: number;
    /** 最近错误 */
    lastError: string;
    /** 执行中命令（有命令上报时填充，普通心跳为空）*/
    extra?: {
      commandId?: string;
      action?: string; // install/uninstall/start/stop/restart/config_push
      status?: string; // running/done/failed
      lastLine?: string;
      result?: string;
      error?: string;
      percent?: number;
      /** openclaw 环境探测（client 心跳随行上报）：本机多来源版本 + 是否需对齐 */
      env?: {
        targetVersion?: string;
        aligned?: boolean | null;
        found?: { origin: string; path: string; version: string }[];
        configExists?: boolean;
        gatewayPortAlive?: boolean;
      };
    } | null;
    /** 上报时间 */
    reportedAt: string;
    /** 插入时间（行创建时间，不随合并刷新）*/
    createdAt: string;
  }

  export interface ListResult {
    list: ListItem[];
  }

  /** 前端搜索参数（gin 不分页，前端本地过滤）*/
  export interface ListParams {
    userId?: string;
    agent?: string;
    status?: string;
  }
}

/** 客户端运行状态列表 */
export function getClientStatusListApi(params?: ClientStatusApi.ListParams) {
  return requestClient.get<ClientStatusApi.ListResult>(
    '/api/v1/system/client/admin/status/list',
    { params },
  );
}
