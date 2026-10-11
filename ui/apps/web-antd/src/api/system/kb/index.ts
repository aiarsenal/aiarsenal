/**
 * 知识库管理接口（/api/v1/kb/document/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * 字段以 dto.KbSearchReq / entity.KbDocument 为准。
 * 上传即抽取→分块→嵌入（离线哈希嵌入，同步完成）；更新原件/改标题摘要自动重嵌。
 */
import { requestClient } from '#/api/request';

export namespace KbApi {
  /** 列表查询参数（dto.KbSearchReq） */
  export interface ListParams {
    keyWords?: string;
    fileType?: string;
    dirId?: string; // ''=全部；选中目录=含子树
    pageNum: number;
    pageSize: number;
  }

  /** 列表/详情行（entity.KbDocument） */
  export interface DocItem {
    dirId?: string;
    docId: string;
    visibility: number; // 0私有 1按条件 2全员（徽标）
    canWrite?: boolean; // 当前身份可写（编辑/更新/删除/移动/重嵌/配授权按钮显隐）
    title: string;
    summary?: string;
    fileName: string;
    fileType: string;
    fileSize: number;
    charCount: number;
    chunkCount: number;
    embedStatus: number; // 0待嵌入 1已嵌入 2失败
    source: string; // ui/mcp
    creatorName?: string;
    createdBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  /** 列表返回（dto.KbListRes: { list, total, currentPage }） */
  export interface ListResult {
    list: DocItem[];
    total: number;
    currentPage?: number;
  }

  /** 语义检索命中行（块级打分带文档信息） */
  export interface SearchItem {
    docId: string;
    title: string;
    fileName: string;
    fileType: string;
    seq: number;
    content: string;
    score: number;
  }

  /** 目录树节点（dto.KbDirNode） */
  export interface DirNode {
    dirId: string;
    name: string;
    parentId: string;
    visibility: number; // 0私有 1按条件 2全员（徽标）
    canWrite: boolean; // 当前身份可写（改名/删除/配授权按钮显隐）
    docCount: number;
    totalCount: number;
    creatorName?: string;
    createdAt?: string;
    children?: DirNode[];
  }

  /** 访问记录行（entity.KbAccessLog） */
  export interface AccessItem {
    docId: string;
    docTitle: string;
    userName: string; // ''=MCP（agent）侧
    action: string; // search | read
    keyword: string;
    hits: number;
    source: string; // ui | mcp
    createdAt?: string;
  }

  /** 批量上传结果（service.UploadResult） */
  export interface UploadResult {
    created: number;
    skipped: number;
    failed: { name: string; error: string }[];
  }

  /** 授权配置回显（service.DocGrantInfo） */
  export interface GrantInfo {
    visibility: number; // 0私有 1按条件 2全员
    readRows: GrantRow[];
    writeRows: GrantRow[];
  }

  /** 条件行（model.GrantRow；与菜单 GrantEditor 同构） */
  export interface GrantRow {
    userIds?: string[];
    deptId?: string;
    roleIds?: string[];
    postIds?: string[];
    exclude?: boolean;
  }
}

const BASE = '/api/v1/kb/document';
const DIR_BASE = '/api/v1/kb/dir';

/** 文档列表（分页） */
export function getKbListApi(params: KbApi.ListParams) {
  return requestClient.get<KbApi.ListResult>(`${BASE}/list`, { params });
}

/** 文档详情（元数据） */
export function getKbApi(docId: string) {
  return requestClient.get<KbApi.DocItem>(`${BASE}/get`, {
    params: { docId },
  });
}

/** 文档全文（抽取清洗后） */
export function getKbContentApi(docId: string) {
  return requestClient.get<{ docId: string; content: string }>(
    `${BASE}/content`,
    { params: { docId } },
  );
}

/** 拉原件字节（blob）：预览用（pdf iframe / img / docx·xlsx 解析），不触发下载 */
export function fetchKbRawApi(docId: string) {
  return requestClient.download(`${BASE}/raw`, {
    params: { docId },
  }) as Promise<Blob>;
}

/** 下载原件（attachment） */
export async function downloadKbRawApi(docId: string, filename: string) {
  const blob = await requestClient.download(`${BASE}/raw`, {
    params: { docId, download: 1 },
  });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob as Blob);
  a.download = filename || 'document';
  a.click();
  URL.revokeObjectURL(a.href);
}

/** 语义检索（向量 top-k 块） */
export function searchKbApi(query: string, limit: number = 10) {
  return requestClient.get<{ list: KbApi.SearchItem[] }>(`${BASE}/search`, {
    params: { query, limit },
  });
}

/** 上传文档（multipart；同 sha256 内容自动跳过） */
export function uploadKbApi(
  file: File,
  title: string = '',
  summary: string = '',
) {
  const fd = new FormData();
  fd.append('file', file);
  if (title) fd.append('title', title);
  if (summary) fd.append('summary', summary);
  return requestClient.post<{ docId: string; created: boolean }>(
    `${BASE}/upload`,
    fd,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  );
}

/** 更新文档原件（内容变了全量重嵌） */
export function updateKbApi(
  docId: string,
  file: File,
  title: string = '',
  summary: string = '',
) {
  const fd = new FormData();
  fd.append('docId', docId);
  fd.append('file', file);
  if (title) fd.append('title', title);
  if (summary) fd.append('summary', summary);
  return requestClient.post(`${BASE}/update`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
}

/** 编辑标题/摘要（自动重嵌） */
export function editKbApi(data: { docId: string; title?: string; summary?: string }) {
  return requestClient.put(`${BASE}/edit`, data);
}

/** 删除文档（连同分块与原件） */
export function deleteKbApi(ids: string[]) {
  return requestClient.delete(`${BASE}/delete`, { data: { ids } });
}

/** 批量上传（多文件/目录上传/zip 统一入口；path 为目录上传的相对层级）。
 * visibility + 条件行 = 对每个新建文档落授权（不给 = 默认全员）。 */
export function uploadKbsApi(
  files: { file: File; path?: string }[],
  title = '',
  summary = '',
  dirId = '',
  vis?: { visibility: number; readRules: string; writeRules: string },
) {
  const fd = new FormData();
  for (const f of files) {
    fd.append('files', f.file);
    fd.append('paths', f.path || '');
  }
  if (title) fd.append('title', title);
  if (summary) fd.append('summary', summary);
  if (dirId) fd.append('dirId', dirId);
  if (vis) {
    fd.append('visibility', String(vis.visibility));
    fd.append('readRules', vis.readRules);
    fd.append('writeRules', vis.writeRules);
  }
  return requestClient.post<KbApi.UploadResult>(`${BASE}/upload`, fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 10 * 60 * 1000, // 目录/压缩包批量入库较慢
  });
}

/** 批量移动文档（dirId ''=移回根） */
export function moveKbsApi(docIds: string[], dirId: string) {
  return requestClient.post(`${BASE}/move`, { docIds, dirId });
}

/** 重新嵌入（docIds 指定文档 / dirId 整目录含子树） */
export function embedKbsApi(data: { docIds?: string[]; dirId?: string }) {
  return requestClient.post<{ done: number; skipped: number; error: string }>(
    `${BASE}/embed`,
    data,
    { timeout: 10 * 60 * 1000 },
  );
}

/** 访问记录（谁检索/阅读过；action ''=全部） */
export function getKbAccessApi(params: {
  action?: string;
  docId?: string;
  pageNum: number;
  pageSize: number;
}) {
  return requestClient.get<{
    list: KbApi.AccessItem[];
    total: number;
    current: number;
  }>(`${BASE}/access`, { params });
}

/** 目录树（含直属/子树文档计数） */
export function getKbDirTreeApi() {
  return requestClient.get<KbApi.DirNode[]>(`${DIR_BASE}/list`);
}

/** 新建目录（parentId ''=根；visibility + 条件行 = 创建即配授权） */
export function createKbDirApi(data: {
  parentId: string;
  name: string;
  visibility: number;
  readRules: string; // GrantRow[] JSON
  writeRules: string; // GrantRow[] JSON
}) {
  return requestClient.post(`${DIR_BASE}/create`, data);
}

/** 重命名目录 */
export function renameKbDirApi(data: { dirId: string; name: string }) {
  return requestClient.put(`${DIR_BASE}/rename`, data);
}

/** 移动目录（parentId ''=根；不得移入自身子树） */
export function moveKbDirApi(data: { dirId: string; parentId: string }) {
  return requestClient.post(`${DIR_BASE}/move`, data);
}

/** 删除目录（须空目录） */
export function deleteKbDirApi(dirId: string) {
  return requestClient.delete(`${DIR_BASE}/delete`, { data: { dirId } });
}

/** 查询文档/目录的授权配置（回显与保存同门槛：超管或目标可写） */
export function getKbGrantApi(targetType: 'dir' | 'doc', targetId: string) {
  return requestClient.get<KbApi.GrantInfo>('/api/v1/kb/grant/get', {
    params: { targetType, targetId },
  });
}

/** 保存授权配置（可见性档位 + 读/写条件行整体重写） */
export function saveKbGrantApi(data: {
  targetType: 'dir' | 'doc';
  targetId: string;
  visibility: number;
  readRules: string; // GrantRow[] JSON
  writeRules: string; // GrantRow[] JSON
}) {
  return requestClient.post('/api/v1/kb/grant/save', data);
}
