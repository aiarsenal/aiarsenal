/**
 * gin 服务监控接口（/api/v1/system/monitor/server）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * gin 后端 application/service/monitor_service.go Server() 返回一个扁平 map，
 * 字段以后端 DTO 为准（非旧 el 页的 sysInfo.* 字段名）。
 */
import { requestClient } from '#/api/request';

export namespace MonitorServerApi {
  /** GET /api/v1/system/monitor/server 返回的服务监控信息（扁平 map） */
  export interface ServerInfo {
    // CPU
    cpuNum: number;
    cpuUsed: number; // %
    cpuAvg5: number;
    cpuAvg15: number;
    // 内存
    memTotal: number; // bytes
    memUsed: number; // bytes
    memPercent: number; // %
    // 主机
    hostName: string;
    os: string;
    kernelArch: string;
    kernelVersion: string;
    uptime: number; // seconds
    bootTime: number; // unix seconds
    // 磁盘（/）
    diskTotal: number; // bytes
    diskUsed: number; // bytes
    diskPercent: number; // %
    // Go 运行时
    goVersion: string;
    goroutines: number;
    startTime: string; // "2006-01-02 15:04:05"
    pid: number;
    runTime: string; // duration string
  }
}

/** 获取服务监控信息 */
export function getServerInfoApi() {
  return requestClient.get<MonitorServerApi.ServerInfo>(
    '/api/v1/system/monitor/server',
  );
}
