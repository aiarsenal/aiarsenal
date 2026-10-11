/**
 * Agent 实例工作区文件接口（/api/v1/agent/instance/:id/workspace/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 工作区 = 实例容器内挂载的用户可见目录（openclaw 为容器内 /workspace，由后端限定根，
 * path 一律为相对根的 POSIX 路径，''/./ 视为根）。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace AgentWorkspaceApi {
  export interface EntryItem {
    name: string;
    /** 相对工作区根的完整路径（含自身名） */
    path: string;
    type: 'dir' | 'file';
    /** 字节（目录=0 或聚合值，仅展示用） */
    size: number;
    /** 八进制权限，如 0644（可选） */
    mode?: string;
    modifiedAt?: string;
  }

  /** 列目录响应 */
  export interface ListResult {
    path: string;
    list: EntryItem[];
  }

  /** 上传返回 */
  export interface UploadResult {
    path: string;
    size: number;
  }
}

/** 列目录（path 空=根；后端按名称排序，目录在前） */
export function getAgentWorkspaceListApi(id: string, path = '') {
  return requestClient.get<AgentWorkspaceApi.ListResult>(
    `/api/v1/agent/instance/${id}/workspace/list`,
    { params: { path } },
  );
}

/** 新建目录（path 为新目录相对路径） */
export function mkdirAgentWorkspaceApi(id: string, path: string) {
  return requestClient.post(
    `/api/v1/agent/instance/${id}/workspace/mkdir`,
    { path },
  );
}

/** 删除文件/目录（目录递归删；paths 批量） */
export function deleteAgentWorkspaceApi(id: string, paths: string[]) {
  return requestClient.delete(
    `/api/v1/agent/instance/${id}/workspace/delete`,
    { data: { paths } },
  );
}

/** 重命名/移动（from/to 均为相对根路径；to 已存在则拒绝） */
export function renameAgentWorkspaceApi(id: string, from: string, to: string) {
  return requestClient.post(
    `/api/v1/agent/instance/${id}/workspace/rename`,
    { from, to },
  );
}

/** 上传文件（multipart: file + path=目标目录，空=根；同名覆盖） */
export function uploadAgentWorkspaceApi(id: string, file: File, path = '') {
  const fd = new FormData();
  fd.append('file', file);
  fd.append('path', path);
  return requestClient.post<AgentWorkspaceApi.UploadResult>(
    `/api/v1/agent/instance/${id}/workspace/upload`,
    fd,
    { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 5 * 60 * 1000 },
  );
}

/** 下载文件（blob；目录不允许下载） */
export async function downloadAgentWorkspaceApi(
  id: string,
  path: string,
  filename: string,
) {
  const blob = await requestClient.download(
    `/api/v1/agent/instance/${id}/workspace/download`,
    { params: { path } },
  );
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob as Blob);
  a.download = filename || 'download';
  a.click();
  URL.revokeObjectURL(a.href);
}
