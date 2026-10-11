/**
 * gin 桌面客户端 App 权限接口（/api/v1/system/client/admin/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * App 目录（aa_client_app，种子预置常用 App）+ 设备 App 权限配置
 * （aa_client_app_perm，整组覆盖式保存，变更写配置变更记录）。
 */
import { requestClient } from '#/api/request';

const BASE = '/api/v1/system/client/admin';

/** 类别 key 有序清单（展示顺序；i18n key = client.appperm_cat_<key>，空值兜底 appperm_cat_other）*/
export const APP_PERM_CATS = [
  'comm',
  'docs',
  'tools',
  'browser',
  'ai',
  'system',
] as const;

export namespace ClientAppPermApi {
  /** App 目录条目（aa_client_app）*/
  export interface App {
    appId: string;
    bundleId: string;
    appName: string;
    category: string; // comm/docs/tools/browser/ai/system；空=未分类
    logo: string;
    weigh: number;
    remarks: string;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 保存目录 App 请求体（gin: ClientAppSaveReq）*/
  export interface AppSaveReq {
    appId?: string; // 空=新建
    bundleId: string; // 编辑时不可改
    appName: string;
    category?: string;
    logo?: string;
    weigh?: number;
    remarks?: string;
  }

  /** 设备 App 权限保存请求体（gin: DeviceAppPermSaveReq）*/
  export interface DeviceAppPermSaveReq {
    deviceId: string;
    bundleIds: string[];
  }

  /** 设备 App 权限行（含客户端上报的授权状态）*/
  export interface DevicePermInfo {
    bundleId: string;
    granted: boolean; // 已授权（探测成功/事件已送达）
    denied: boolean; // 已拒绝（TCC 缓存拒绝 -1743）
  }

  /** Bundle ID 联想候选（本机已装 App 优先 + Apple iTunes Search 查 Mac App Store）*/
  export interface AppSuggest {
    bundleId: string;
    trackName: string; // 展示名（本机=App 名；MAS=上架名）
    artistName: string; // 开发者（仅 MAS 候选）
    artworkUrl: string; // 图标（仅 MAS 候选）
    source: 'local' | 'mas'; // local=本机已安装 mas=Mac App Store
  }
}

/** App 目录全量 */
export function getClientAppListApi() {
  return requestClient.get<{ list: ClientAppPermApi.App[] }>(
    `${BASE}/app/list`,
  );
}

/** 按 App 名/关键词联想 Bundle ID（查不到返回空列表，降级手填）*/
export function suggestClientAppsApi(keyword: string) {
  return requestClient.get<{ list: ClientAppPermApi.AppSuggest[] }>(
    `${BASE}/app/suggest`,
    { params: { keyword } },
  );
}

/** 保存目录 App（新建/编辑）*/
export function saveClientAppApi(data: ClientAppPermApi.AppSaveReq) {
  return requestClient.put(`${BASE}/app/save`, data);
}

/** 删除目录 App（被设备引用时后端拦截）*/
export function deleteClientAppApi(id: string) {
  return requestClient.delete(`${BASE}/app/delete`, { data: { id } });
}

/** 某设备勾选中的授权 App（bundleIds 供勾选回显；perms 带授权状态徽标数据）*/
export function getDeviceAppPermsApi(deviceId: string) {
  return requestClient.get<{
    bundleIds: string[];
    perms: ClientAppPermApi.DevicePermInfo[];
  }>(`${BASE}/appperm/get`, {
    params: { deviceId },
  });
}

/** 整组覆盖某设备的 App 权限清单 */
export function saveDeviceAppPermsApi(
  data: ClientAppPermApi.DeviceAppPermSaveReq,
) {
  return requestClient.put(`${BASE}/appperm/save`, data);
}

/** 重置某设备的 App 自动化授权（下发 appperm_reset：tccutil 全量重置，清单内所有 App 授权框重弹）*/
export function resetDeviceAppPermApi(data: { deviceId: string }) {
  return requestClient.post<{ commandId: string }>(
    `${BASE}/appperm/reset`,
    data,
  );
}
