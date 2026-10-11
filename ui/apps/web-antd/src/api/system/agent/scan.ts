/**
 * gin Agent 安全扫描 Job 接口（/api/v1/agent/scan/job/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentScanApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    assetType?: string;
    status?: string;
  }

  export interface ListItem {
    scanJobId: string;
    assetType: string;
    scanMode: string;
    status: number;
    requestedBy?: string;
    scopeJson?: string;
    totalItems: number;
    completedItems: number;
    failedItems: number;
    currentItemName?: string;
    startedAt?: string;
    finishedAt?: string;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
    total: number;
    currentPage?: number;
  }

  export interface ScanJobReq {
    assetType: string;
    scanMode?: string;
    status?: number;
    scopeJson?: string;
    totalItems?: number;
  }
}

/** 扫描任务列表 */
export function getAgentScanJobListApi(params: AgentScanApi.ListParams) {
  return requestClient.get<AgentScanApi.ListResult>(
    '/api/v1/agent/scan/job/list',
    { params },
  );
}

/** 扫描任务详情 */
export function getAgentScanJobApi(scanJobId: string) {
  return requestClient.get('/api/v1/agent/scan/job/get', {
    params: { scanJobId },
  });
}

/** 新增扫描任务 */
export function addAgentScanJobApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/agent/scan/job/add', data);
}

/** 编辑扫描任务 */
export function editAgentScanJobApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/agent/scan/job/edit', data);
}

/** 删除扫描任务（批量，body: { ids: [] }） */
export function deleteAgentScanJobApi(ids: string[]) {
  return requestClient.delete('/api/v1/agent/scan/job/delete', {
    data: { ids },
  });
}
