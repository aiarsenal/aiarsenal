/**
 * 第三方 skill 市场接口（/api/v1/skill/market/*）。
 * 链接导入 + 外部市场目录 CRUD（site/*）+ 站内搜索（search）+ 目录一键同步（sync）。
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import { requestClient } from '#/api/request';

export namespace SkillMarketApi {
  /** 链接导入请求（zip 直链或详情页链接；可一并记原作者/打标签）。 */
  export interface ImportReq {
    url: string;
    namespaceId: string;
    upstreamAuthor?: string;
    tags?: string[];
  }

  export interface ImportResult {
    skillId: string;
    versionId: string;
  }

  /** 外部市场目录项（后端 entity.SkillMarketSite，aa_skill_market_site）。 */
  export interface Site {
    marketId: string;
    name: string;
    siteUrl: string;
    description: string;
    icon: string; // emoji 兜底
    logoUrl: string; // logo 图片 URL（空则用 icon）
    themeColor: string;
    status: number; // 1=显示 0=隐藏
    sort: number;
    createdAt?: number;
    updatedAt?: number;
  }

  export interface SiteListParams {
    keyword?: string;
    status?: string;
    pageNum?: number;
    pageSize?: number;
  }

  export interface SiteListResult {
    list: Site[];
    total: number;
  }

  /** 目录新建/编辑请求。 */
  export interface SiteReq {
    marketId?: string; // 编辑必填
    name: string;
    siteUrl: string;
    description?: string;
    icon?: string;
    logoUrl?: string;
    themeColor?: string;
    status?: number;
    sort?: number;
  }

  /** 站内搜索命中行（GET /skill/market/search?keyword=）。
   *  后端聚合各启用市场的公开目录/开放接口，imported 标注是否已入库。 */
  export interface SearchItem {
    /** 市场侧唯一标识（slug/id，用于二次定位详情） */
    externalId: string;
    title: string;
    slug?: string;
    description?: string;
    author?: string;
    version?: string;
    marketId: string;
    marketName: string;
    detailUrl?: string;
    /** zip 直链（有则可直接走 /market/import 导入） */
    downloadUrl?: string;
    installs?: number;
    stars?: number;
    updatedAt?: string;
    /** 是否已导入本地技能库 */
    imported: boolean;
    localSkillId?: string;
  }

  export interface SearchResult {
    list: SearchItem[];
    total: number;
  }

  /** 一键同步响应（POST /skill/market/sync { marketId? }） */
  export interface SyncResult {
    /** 触发的市场数 */
    synced: number;
    created: number;
    updated: number;
    failed: number;
    errors: { marketId: string; error: string }[];
  }
}

const BASE = '/api/v1/skill/market';

/** 链接导入：zip 直链或详情页链接 → 保存到本地技能库（记原作者/标签溯源）。 */
export function importMarketUrlApi(data: SkillMarketApi.ImportReq) {
  return requestClient.post<SkillMarketApi.ImportResult>(`${BASE}/import`, data);
}

/** 启用中的外部市场目录全量（第三方市场页卡片展示）。 */
export function listMarketSitesApi() {
  return requestClient.get<{ list: SkillMarketApi.Site[] }>(`${BASE}/site/all`);
}

/** 外部市场目录分页列表（目录管理）。 */
export function listMarketSitePageApi(params: SkillMarketApi.SiteListParams) {
  return requestClient.get<SkillMarketApi.SiteListResult>(`${BASE}/site/list`, {
    params,
  });
}

/** 新增外部市场目录项。 */
export function addMarketSiteApi(data: SkillMarketApi.SiteReq) {
  return requestClient.post<{ marketId: string }>(`${BASE}/site/add`, data);
}

/** 编辑外部市场目录项。 */
export function editMarketSiteApi(data: SkillMarketApi.SiteReq) {
  return requestClient.post(`${BASE}/site/edit`, data);
}

/** 删除外部市场目录项。 */
export function deleteMarketSiteApi(marketId: string) {
  return requestClient.post(`${BASE}/site/delete`, { marketId });
}

/** 站内搜索：聚合各启用市场的技能（keyword 必填；marketId 可选收窄单市场） */
export function searchMarketApi(params: {
  keyword: string;
  marketId?: string;
  page?: number;
  pageSize?: number;
}) {
  return requestClient.get<SkillMarketApi.SearchResult>(`${BASE}/search`, {
    params,
  });
}

/** 一键同步：拉取（marketId 指定单市场 / 空=全部启用市场）最新目录，返回计数 */
export function syncMarketApi(marketId = '') {
  return requestClient.post<SkillMarketApi.SyncResult>(`${BASE}/sync`, {
    marketId,
  });
}
