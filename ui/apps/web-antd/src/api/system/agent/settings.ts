/**
 * gin Agent 系统设置接口（/api/v1/agent/settings/image/*）
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace AgentSettingsApi {
  export interface ImageItem {
    systemImageId: string;
    agent: string;
    version: string;
    displayName: string;
    image: string;
    isEnabled: number;
    updatedBy?: string;
    updatedAt?: string;
  }
  export interface ImageListResult {
    list: ImageItem[];
  }
  export interface ImageReq {
    agent: string;
    version: string;
    displayName: string;
    image: string;
    isEnabled?: number;
  }
}

const BASE = '/api/v1/agent/settings/image';

export function getAgentImageListApi(params: Recordable<any>) {
  return requestClient.get<AgentSettingsApi.ImageListResult>(`${BASE}/list`, {
    params,
  });
}
export function upsertAgentImageApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/upsert`, data);
}
export function deleteAgentImageApi(agent: string, version: string) {
  return requestClient.delete(`${BASE}/delete`, {
    params: { agent, version },
  });
}
