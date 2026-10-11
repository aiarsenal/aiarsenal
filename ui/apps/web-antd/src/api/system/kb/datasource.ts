/**
 * 知识库数据源接口（/api/v1/kb/datasource/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 数据源 = 向知识库灌数的外部通道：网页爬取 / 数据库 / API 拉取 / 对象存储 / 外部知识库。
 * config 为各类型差异化字段的 jsonb（前端按类型动态渲染，后端整包存储校验）。
 * 运行（run）= 一次同步执行记录；手动触发走 trigger，定时触发由 cron 调度器落 run。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace KbDatasourceApi {
  export type DatasourceType =
    | 'api' // API 拉取
    | 'database' // 数据库
    | 'external_kb' // 外部知识库
    | 'object_storage' // 对象存储
    | 'web_crawl'; // 网页爬取

  export type RunStatus = 'failed' | 'partial' | 'pending' | 'running' | 'success';
  export type RunTrigger = 'api' | 'cron' | 'manual';

  // ===== 各类型 config（jsonb，按 type 二选一） =====

  /** web_crawl：网页爬取 */
  export interface WebCrawlConfig {
    startUrls: string[];
    maxDepth?: number; // 默认 2
    maxPages?: number; // 默认 50
    includePatterns?: string[]; // URL 白名单正则
    excludePatterns?: string[]; // URL 黑名单正则
    selector?: string; // 正文 CSS 选择器（可选）
  }

  /** database：数据库拉取 */
  export interface DatabaseConfig {
    driver: 'mysql' | 'oracle' | 'postgres' | 'sqlite';
    host: string;
    port?: number;
    database: string;
    username: string;
    password: string; // 后端加密存储，编辑回显为占位（不回传明文时前端不改动）
    query: string; // 拉取 SQL（SELECT）
    incrementColumn?: string; // 增量列（如 updated_at）
  }

  /** api：API 拉取 */
  export interface ApiPullConfig {
    method: 'GET' | 'POST';
    url: string;
    headers?: string; // JSON 字符串
    params?: string; // JSON 字符串
    bodyTemplate?: string; // POST body 模板
    authType?: 'bearer' | 'basic' | 'none'; // 默认 none
    authToken?: string;
    listPath?: string; // 响应数据路径（如 data.list，缺省=根数组）
    titleField?: string; // 文档标题字段
    contentField?: string; // 正文字段
  }

  /** object_storage：对象存储 */
  export interface ObjectStorageConfig {
    provider: 'cos' | 'minio' | 'oss' | 's3';
    endpoint: string;
    region?: string;
    bucket: string;
    accessKey: string;
    secretKey: string; // 后端加密存储
    prefix?: string; // 对象 key 前缀过滤
    fileTypes?: string[]; // 如 ['pdf','docx']，空=全部支持类型
  }

  /** external_kb：外部知识库 */
  export interface ExternalKbConfig {
    provider: 'confluence' | 'custom' | 'dingtalk' | 'feishu' | 'notion';
    baseUrl: string;
    appId?: string; // feishu/dingtalk: app id
    appSecret?: string; // 后端加密存储
    accessToken?: string; // notion/confluence token
    spaceId?: string; // 空间/库 id
    docTypes?: string[]; // 拉取的文档类型
  }

  export type DatasourceConfig =
    | ApiPullConfig
    | DatabaseConfig
    | ExternalKbConfig
    | ObjectStorageConfig
    | WebCrawlConfig;

  // ===== 列表/详情行 =====
  export interface DatasourceItem {
    datasourceId: string;
    name: string;
    type: DatasourceType;
    /** 灌入的知识库目录（kb dir，空=根） */
    dirId?: string;
    description?: string;
    enabled: number; // 1=启用（参与 cron 调度） 0=停用
    /** cron 表达式（5 段；空=仅手动同步） */
    cron?: string;
    /** 调度器推算的下次执行时间（后端富化） */
    nextRunAt?: string;
    config: DatasourceConfig;
    /** 最近一次运行（后端富化，冗余展示用） */
    lastRunAt?: string;
    lastRunStatus?: RunStatus;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListParams {
    pageNum: number;
    pageSize: number;
    keyword?: string;
    type?: DatasourceType | '';
  }

  export interface ListResult {
    list: DatasourceItem[];
    total: number;
    currentPage?: number;
  }

  /** 新建/编辑请求（config 整包提交；edit 带 datasourceId） */
  export interface DatasourceReq {
    datasourceId?: string;
    name: string;
    type: DatasourceType;
    dirId?: string;
    description?: string;
    enabled: number;
    cron?: string;
    config: DatasourceConfig;
  }

  // ===== 运行历史 =====
  export interface RunItem {
    runId: string;
    datasourceId: string;
    datasourceName?: string;
    status: RunStatus;
    trigger: RunTrigger;
    startedAt?: string;
    finishedAt?: string;
    durationMs?: number;
    /** 统计（按类型语义不同：web=页/对象存储=文件/数据库·API=行） */
    stats?: {
      /** 拉取的原始条目数 */
      fetched: number;
      docsCreated: number;
      docsUpdated: number;
      chunksCreated: number;
      /** 跳过（未变化/不支持） */
      skipped: number;
      failed: number;
    };
    error?: string;
    createdAt?: string;
  }

  export interface RunListResult {
    list: RunItem[];
    total: number;
    currentPage?: number;
  }

  /** 手动触发返回 */
  export interface TriggerResult {
    runId: string;
  }
}

const BASE = '/api/v1/kb/datasource';

/** 数据源列表（分页） */
export function getKbDatasourceListApi(params: KbDatasourceApi.ListParams) {
  return requestClient.get<KbDatasourceApi.ListResult>(`${BASE}/list`, {
    params,
  });
}

/** 数据源详情 */
export function getKbDatasourceApi(datasourceId: string) {
  return requestClient.get<KbDatasourceApi.DatasourceItem>(`${BASE}/get`, {
    params: { datasourceId },
  });
}

/** 新建数据源 */
export function addKbDatasourceApi(data: KbDatasourceApi.DatasourceReq) {
  return requestClient.post<{ datasourceId: string }>(`${BASE}/add`, data);
}

/** 编辑数据源（body 带 datasourceId；config 整包覆盖） */
export function editKbDatasourceApi(data: KbDatasourceApi.DatasourceReq) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除数据源（批量，body: { ids: [] }；不删已灌入的文档） */
export function deleteKbDatasourceApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 手动触发同步（立即起一次 run） */
export function triggerKbDatasourceApi(datasourceId: string) {
  return requestClient.post<KbDatasourceApi.TriggerResult>(
    `${BASE}/${datasourceId}/trigger`,
  );
}

/** 运行历史（分页，倒序） */
export function getKbDatasourceRunsApi(
  datasourceId: string,
  params: { pageNum: number; pageSize: number },
) {
  return requestClient.get<KbDatasourceApi.RunListResult>(
    `${BASE}/${datasourceId}/runs`,
    { params },
  );
}

/** 取消运行中的 run（pending/running 可取消） */
export function cancelKbDatasourceRunApi(runId: string) {
  return requestClient.post(`${BASE}/runs/cancel`, { runId });
}
