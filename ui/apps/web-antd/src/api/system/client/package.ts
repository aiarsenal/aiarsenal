/**
 * gin 桌面安装包缓存接口（/api/v1/system/client/admin/package/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * 端点：
 *   GET    /package/list   → { list: ClientPackage[] }（无分页）
 *   PUT    /package/fetch  → { packageId, filename, fileSize, sha256 }（从官网拉取，长耗时）
 *   DELETE /package/delete → { deleted: true }（body: { id }，id=packageId）
 *
 * 字段来源：entity.ClientPackage（api/domain/entity/client.go）
 *           DTO：dto.ClientPackageFetchReq（agent/version/platform/officialUrl）
 *                dto.ClientIdReq（id）
 */
import { useAppConfig } from '@vben/hooks';

import { requestClient } from '#/api/request';

export namespace ClientPackageApi {
  /** 安装包缓存行（每 agent+version+platform 一行）*/
  export interface ListItem {
    /** 业务 id（删除用此字段，传给 DELETE /package/delete 的 id）*/
    packageId: string;
    /** agent key：openclaw / hermes / obsidian */
    agent: string;
    /** agent 官方 logo（相对路径，由 UI 静态目录提供）；后端 AdminListPackages 回填 */
    logo?: string;
    version: string;
    /** win / mac / linux / common */
    platform: string;
    filename: string;
    /** 字节数 */
    fileSize: number;
    sha256: string;
    officialUrl: string;
    localPath: string;
    /** ready / failed */
    status: string;
    fetchedBy: string;
    createdAt?: string;
    updatedAt?: string;
    fetchedAt?: string;
  }

  export interface ListResult {
    list: ListItem[];
  }

  /** PUT /package/fetch 请求体（dto.ClientPackageFetchReq）*/
  export interface FetchReq {
    agent: string;
    version: string;
    platform: string;
    officialUrl: string;
  }

  export interface FetchResult {
    packageId: string;
    filename: string;
    fileSize: number;
    sha256: string;
  }
}

/** 安装包列表（无分页，gin 返回 { list }）*/
export function getClientPackageListApi() {
  return requestClient.get<ClientPackageApi.ListResult>(
    '/api/v1/system/client/admin/package/list',
  );
}

/** 从官网拉取安装包（长耗时，后端下载落盘）*/
export function fetchClientPackageApi(data: ClientPackageApi.FetchReq) {
  return requestClient.put<ClientPackageApi.FetchResult>(
    '/api/v1/system/client/admin/package/fetch',
    data,
    { timeout: 600000 },
  );
}

/** 删除安装包（body: { id }，id=packageId）*/
export function deleteClientPackageApi(id: string) {
  return requestClient.delete(
    '/api/v1/system/client/admin/package/delete',
    { data: { id } },
  );
}

/**
 * 下载安装包到本地（管理员浏览器另存）。
 * 端点 GET /client/packages/:agent/:version/:platform 公开（无鉴权，安装包公开分发）。
 * 用 fetch 绕过 requestClient 的 {code,data} JSON 拦截器，直接拿二进制 blob 触发浏览器下载。
 */
export async function downloadClientPackageApi(
  agent: string,
  version: string,
  platform: string,
  filename: string,
) {
  const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);
  const resp = await fetch(
    `${apiURL}/api/v1/system/client/packages/${agent}/${version}/${platform}`,
  );
  if (!resp.ok) {
    throw new Error(`下载失败 HTTP ${resp.status}`);
  }
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
