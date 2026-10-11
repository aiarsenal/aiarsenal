<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Button,
  Input,
  message,
  Modal,
  Select,
  Spin,
  Table,
  Tabs,
  Textarea,
  Tooltip,
  Tree,
  TreeSelect,
  Upload,
} from 'ant-design-vue';
import { renderAsync as renderDocxAsync } from 'docx-preview';
import ExcelJS from 'exceljs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createDirApi,
  deleteDirApi,
  deleteDocApi,
  downloadDocRawApi,
  editDocApi,
  embedDocsApi,
  fetchDocRawApi,
  getDirByRefApi,
  getDirTreeApi,
  getDocAccessApi,
  getDocContentApi,
  getDocGrantApi,
  getDocListApi,
  moveDirApi,
  moveDocsApi,
  renameDirApi,
  saveDocGrantApi,
  searchDocApi,
  updateDocApi,
  uploadDocsApi,
} from '#/api/system/doc';
import GrantEditor, { type GrantRowValue } from '#/components/GrantEditor.vue';
import { $t } from '#/locales';

defineOptions({ name: 'SystemDoc' });

const route = useRoute();

const TYPE_OPTIONS = computed(() => [
  { label: 'PDF', value: 'pdf' },
  { label: 'DOCX', value: 'docx' },
  { label: 'DOC', value: 'doc' },
  { label: 'XLSX', value: 'xlsx' },
  { label: 'XLS', value: 'xls' },
  { label: 'PPTX', value: 'pptx' },
  { label: 'PPT', value: 'ppt' },
  { label: 'OFD', value: 'ofd' },
  { label: 'TXT', value: 'txt' },
  { label: 'MD', value: 'md' },
  { label: 'CSV', value: 'csv' },
  { label: 'HTML', value: 'html' },
  { label: 'JSON', value: 'json' },
  { label: 'ZIP', value: 'zip' },
]);

/** 可入库扩展名（与后端抽取能力对齐）+ 压缩包解包（zip/rar/7z/tar/gz…含嵌套）+ 图片（可预览，无文本层） */
const DOC_EXTS = [
  'pdf',
  'docx',
  'doc',
  'txt',
  'md',
  'csv',
  'log',
  'json',
  'html',
  'htm',
  'ppt',
  'pptx',
  'xls',
  'xlsx',
  'xlsm',
  'ofd',
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'bmp',
  'zip',
  'rar',
  '7z',
  'tar',
  'gz',
  'tgz',
  'bz2',
  'txz',
  'xz',
];

/** 更新原件：单文档类型（压缩包只在上传入口解包，更新不支持包） */
const DOC_SINGLE_EXTS = DOC_EXTS.filter(
  (e) => !['7z', 'bz2', 'gz', 'rar', 'tar', 'tgz', 'txz', 'xz', 'zip'].includes(e),
);

/** 与后端上限一致（controller/doc.go）：单文件 64MB、单次合计 256MB，超限在选择时即拦下 */
const MAX_FILE_SIZE = 64 * 1024 * 1024;
const MAX_TOTAL_SIZE = 256 * 1024 * 1024;

// ===== 目录树（左栏） =====
const dirTree = ref<any[]>([]);
const dirMap = ref<Record<string, any>>({});
const expandedKeys = ref<string[]>([]);
const selectedDirId = ref(''); // ''=全部文档
const totalAllDocs = ref(0); // 全部文档数（根+目录）
let expandOnce = false;

/** 空间固定根（项目空间/个人空间）：系统托管，不提供任何手工操作 */
function isSpaceRoot(n: any): boolean {
  return n?.kind === 'project_space' || n?.kind === 'personal_space';
}

/** 目录图标：空间根区别于普通 📁 */
function dirIcon(n: any): string {
  if (n?.kind === 'project_space') return '🏢';
  if (n?.kind === 'personal_space') return '👤';
  return '📁';
}

// 目录栏可收起（收起成窄条）；状态记 localStorage（前端记忆，刷新不丢）
const dirSideOpen = ref(localStorage.getItem('doc.dirSideOpen') !== '0');
function toggleDirSide() {
  dirSideOpen.value = !dirSideOpen.value;
  localStorage.setItem('doc.dirSideOpen', dirSideOpen.value ? '1' : '0');
}

async function loadDirs() {
  const tree = (await getDirTreeApi()) ?? [];
  dirTree.value = tree;
  const map: Record<string, any> = {};
  const walk = (nodes: any[]) => {
    for (const n of nodes) {
      map[n.dirId] = n;
      if (n.children?.length) walk(n.children);
    }
  };
  walk(tree);
  dirMap.value = map;
  if (!expandOnce) {
    expandedKeys.value = Object.keys(map);
    expandOnce = true;
  }
}

/** 根目录直属文档数 = 全部 − 各顶层目录子树之和 */
const rootDocCount = computed(() => {
  const inDirs = dirTree.value.reduce((s: number, n: any) => s + (n.totalCount || 0), 0);
  return Math.max(0, totalAllDocs.value - inDirs);
});

function selectDir(dirId: string) {
  selectedDirId.value = dirId;
  gridApi.query();
}

// 当前选中目录的祖先链（根→…→当前），面包屑展示用
const dirPathChain = computed(() => {
  const chain: { dirId: string; name: string }[] = [];
  let cur = selectedDirId.value;
  let guard = 0; // 防御环（Move 已拦，双保险）
  while (cur && dirMap.value[cur] && guard < 50) {
    const n = dirMap.value[cur];
    chain.unshift({ dirId: n.dirId, name: n.name });
    cur = n.parentId || '';
    guard++;
  }
  return chain;
});

// 面包屑跳转：选中目录 + 左树展开其祖先链（深层节点默认折叠，不展开看不见）
function jumpToDir(dirId: string) {
  selectedDirId.value = dirId;
  const ancestors: string[] = [];
  let cur = dirId;
  let guard = 0;
  while (cur && dirMap.value[cur] && guard < 50) {
    const p = dirMap.value[cur].parentId || '';
    if (p) ancestors.push(p);
    cur = p;
    guard++;
  }
  if (ancestors.length) {
    expandedKeys.value = [...new Set([...expandedKeys.value, ...ancestors])];
  }
  gridApi.query();
}

// 树数据：key=dirId；title 用插槽渲染（计数+悬浮操作按钮+文档拖入）
const treeData = computed(() =>
  dirTree.value.map(function mapNode(n: any): any {
    return {
      key: n.dirId,
      dirId: n.dirId,
      name: n.name,
      docCount: n.docCount,
      children: (n.children || []).map(mapNode),
    };
  }),
);

// 目录树内部拖拽（目录→目录）：只取新父（同级排序按名称，无位次语义）
function onTreeDrop(info: any) {
  const dragKey = String(info.dragNode?.eventKey ?? info.dragNode?.key ?? '');
  const dropKey = String(info.node?.eventKey ?? info.node?.key ?? '');
  if (!dragKey || !dropKey) return;
  // 系统托管目录（空间根/项目/个人目录）由同步管理，不可拖拽
  if (dirMap.value[dragKey]?.kind) {
    message.warning($t('doc.doc_spaceManagedNoMove'));
    return;
  }
  const parentId = info.dropToGap ? (dirMap.value[dropKey]?.parentId || '') : dropKey;
  // 不能移入空间根（其子目录由系统按项目/员工自动创建）
  if (!info.dropToGap && isSpaceRoot(dirMap.value[dropKey])) {
    message.warning($t('doc.doc_spaceRootNoDropIn'));
    return;
  }
  if (parentId === dragKey) return; // 拖到自己上（无 gap）= 无操作
  moveDirApi({ dirId: dragKey, parentId })
    .then(() => {
      message.success($t('doc.doc_dirMoveOk'));
      loadDirs();
    })
    .catch(() => {
      /* 后端校验（环/不存在）已弹错误 */
    });
}

// ===== 文档行 → 目录拖拽（跨面板 HTML5 DnD） =====
const draggingDocs = ref<string[]>([]);

function onRowDragStart(row: any, e: DragEvent) {
  const sel = (gridApi.grid?.getCheckboxRecords() || []) as any[];
  const ids = sel.some((r) => r.docId === row.docId) ? sel.map((r) => r.docId) : [row.docId];
  draggingDocs.value = ids;
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', ids.join(','));
  }
}

function onDirDragOver(e: DragEvent) {
  if (draggingDocs.value.length > 0) e.preventDefault();
}

async function onDropDocsTo(dirId: string, e: Event) {
  if (draggingDocs.value.length === 0) return; // 目录树自身拖拽走 onTreeDrop
  e.stopPropagation();
  const ids = [...draggingDocs.value];
  draggingDocs.value = [];
  try {
    await moveDocsApi(ids, dirId);
    message.success($t('doc.doc_moveOk'));
    gridApi.query();
    loadDirs();
  } catch {
    /* 错误已弹 */
  }
}

// ===== 目录新建/重命名（共用弹窗；新建可一并配授权） =====
const dirModal = reactive({
  open: false,
  mode: 'create' as 'create' | 'rename',
  dirId: '',
  parentId: '',
  name: '',
  visibility: 2,
});
const dirReadRows = ref<GrantRowValue[]>([]);
const dirWriteRows = ref<GrantRowValue[]>([]);

function openDirCreate(parentId: string) {
  Object.assign(dirModal, { open: true, mode: 'create', dirId: '', parentId, name: '', visibility: 2 });
  dirReadRows.value = [];
  dirWriteRows.value = [];
}

function openDirRename(node: any) {
  Object.assign(dirModal, { open: true, mode: 'rename', dirId: node.dirId, parentId: '', name: node.name });
}

async function onSubmitDir() {
  const name = dirModal.name.trim();
  if (!name) {
    message.warning($t('doc.doc_dirNamePlaceholder'));
    return;
  }
  if (dirModal.mode === 'create') {
    await createDirApi({
      parentId: dirModal.parentId,
      name,
      visibility: dirModal.visibility,
      readRules: JSON.stringify(dirReadRows.value ?? []),
      writeRules: JSON.stringify(dirWriteRows.value ?? []),
    });
  } else {
    await renameDirApi({ dirId: dirModal.dirId, name });
  }
  message.success($t('doc.doc_dirSaveOk'));
  dirModal.open = false;
  loadDirs();
}

function onDeleteDir(node: any) {
  Modal.confirm({
    title: $t('doc.doc_dirDeleteConfirm'),
    content: node.name,
    onOk: async () => {
      await deleteDirApi(node.dirId);
      message.success($t('doc.doc_dirDeleteOk'));
      if (selectedDirId.value === node.dirId) {
        selectedDirId.value = '';
        gridApi.query();
      }
      loadDirs();
    },
  });
}

// ===== 组合授权（文档/目录共用弹窗，读写分离） =====
const VIS_OPTIONS = computed(() => [
  { label: $t('doc.doc_visAll'), value: 2 },
  { label: $t('doc.doc_visCond'), value: 1 },
  { label: $t('doc.doc_visPrivate'), value: 0 },
]);

const grantModal = reactive({
  open: false,
  loading: false,
  targetType: 'doc' as 'dir' | 'doc',
  targetId: '',
  name: '',
  visibility: 2,
});
const grantReadRows = ref<GrantRowValue[]>([]);
const grantWriteRows = ref<GrantRowValue[]>([]);

async function openGrant(targetType: 'dir' | 'doc', targetId: string, name: string) {
  Object.assign(grantModal, { open: true, loading: true, targetType, targetId, name, visibility: 2 });
  grantReadRows.value = [];
  grantWriteRows.value = [];
  try {
    const info = await getDocGrantApi(targetType, targetId);
    grantModal.visibility = info?.visibility ?? 2;
    grantReadRows.value = (info?.readRows ?? []) as GrantRowValue[];
    grantWriteRows.value = (info?.writeRows ?? []) as GrantRowValue[];
  } catch {
    /* 拦截器已提示 */
  } finally {
    grantModal.loading = false;
  }
}

async function onSubmitGrant() {
  grantModal.loading = true;
  try {
    await saveDocGrantApi({
      targetType: grantModal.targetType,
      targetId: grantModal.targetId,
      visibility: grantModal.visibility,
      readRules: JSON.stringify(grantReadRows.value ?? []),
      writeRules: JSON.stringify(grantWriteRows.value ?? []),
    });
    message.success($t('doc.doc_grantSaved'));
    grantModal.open = false;
    gridApi.query(); // canWrite 徽标随授权变化
    loadDirs(); // 目录授权影响子树文档可见性
  } finally {
    grantModal.loading = false;
  }
}

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyWords',
    label: $t('doc.doc_keyword'),
    componentProps: { placeholder: $t('doc.doc_keywordPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'fileType',
    label: $t('doc.doc_fileType'),
    componentProps: {
      allowClear: true,
      options: TYPE_OPTIONS.value,
      placeholder: $t('doc.doc_fileTypeAll'),
      style: { width: '100%' },
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 44, fixed: 'left' },
    { type: 'seq', title: $t('doc.doc_seq'), width: 56, fixed: 'left' },
    {
      field: 'drag',
      title: '',
      width: 36,
      fixed: 'left',
      slots: { default: 'dragHandle' },
    },
    { field: 'title', title: $t('doc.doc_title'), minWidth: 200 },
    { field: 'fileName', title: $t('doc.doc_fileName'), minWidth: 200 },
    {
      field: 'fileType',
      title: $t('doc.doc_fileType'),
      width: 90,
      slots: { default: 'fileType' },
    },
    {
      field: 'fileSize',
      title: $t('doc.doc_fileSize'),
      width: 100,
      slots: { default: 'fileSize' },
    },
    { field: 'charCount', title: $t('doc.doc_charCount'), width: 100 },
    { field: 'chunkCount', title: $t('doc.doc_chunkCount'), width: 90 },
    {
      field: 'embedStatus',
      title: $t('doc.doc_embedStatus'),
      width: 100,
      slots: { default: 'embedStatus' },
    },
    { field: 'creatorName', title: $t('doc.doc_creator'), width: 110 },
    {
      field: 'visibility',
      title: $t('doc.doc_visibility'),
      width: 90,
      slots: { default: 'visibility' },
    },
    { field: 'createdAt', title: $t('doc.doc_createdAt'), width: 170 },
    {
      field: 'action',
      title: $t('doc.doc_action'),
      width: 340,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getDocListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          dirId: selectedDirId.value,
          ...(form as any),
        });
        if (!selectedDirId.value) totalAllDocs.value = res?.total ?? 0;
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

function fmtSize(n: number): string {
  if (!n) return '0 B';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

/** 勾选的文档 id（无勾选提示） */
function selectedDocIds(): null | string[] {
  const sel = (gridApi.grid?.getCheckboxRecords() || []) as any[];
  if (!sel.length) {
    message.warning($t('doc.doc_pickAtLeast'));
    return null;
  }
  return sel.map((r) => r.docId);
}

// ===== 上传（多文件 / 目录 / zip） =====
const uploadVisible = ref(false);
const uploadLoading = ref(false);
const uploadMode = ref<'dir' | 'files'>('files');
const uploadDirId = ref('');
const uploadTitle = ref('');
const uploadSummary = ref('');
const uploadVis = ref(2);
const uploadReadRows = ref<GrantRowValue[]>([]);
const uploadWriteRows = ref<GrantRowValue[]>([]);
const pickedFiles = ref<{ file: File; path: string }[]>([]);
const dirInputRef = ref<HTMLInputElement | null>(null);

function openUpload() {
  uploadMode.value = 'files';
  uploadDirId.value = selectedDirId.value;
  uploadTitle.value = '';
  uploadSummary.value = '';
  uploadVis.value = 2;
  uploadReadRows.value = [];
  uploadWriteRows.value = [];
  pickedFiles.value = [];
  uploadVisible.value = true;
}

/** before-upload 首参即文件本体（antd-vue），return false 阻止自动上传 */
function onPickFile(file: any) {
  if (Number(file.size) > MAX_FILE_SIZE) {
    message.warning($t('doc.doc_fileTooLarge', { name: file.name }));
    return false; // 超限不入场（不可选）
  }
  pickedFiles.value.push({ file: file as File, path: '' });
  return false;
}

function onDirPicked(e: Event) {
  const input = e.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  let oversize = 0;
  for (const f of files) {
    const rel = (f as any).webkitRelativePath || '';
    const ext = rel.split('.').pop()?.toLowerCase() || '';
    if (!DOC_EXTS.includes(ext)) continue; // 跳过图片/二进制等不可入库文件
    if (f.size > MAX_FILE_SIZE) {
      oversize++;
      continue;
    }
    pickedFiles.value.push({ file: f, path: rel });
  }
  if (oversize > 0) {
    message.warning($t('doc.doc_fileTooLargeCount', { n: oversize }));
  }
  input.value = ''; // 允许重复选同一目录
}

async function onSubmitUpload() {
  if (!pickedFiles.value.length) {
    message.warning($t('doc.doc_pickFirst'));
    return;
  }
  const total = pickedFiles.value.reduce((s, f) => s + (f.file.size || 0), 0);
  if (total > MAX_TOTAL_SIZE) {
    message.error($t('doc.doc_totalTooLarge'));
    return;
  }
  uploadLoading.value = true;
  try {
    const single = pickedFiles.value.length === 1;
    const res = await uploadDocsApi(
      pickedFiles.value,
      single ? uploadTitle.value : '',
      single ? uploadSummary.value : '',
      uploadDirId.value,
      {
        visibility: uploadVis.value,
        readRules: JSON.stringify(uploadReadRows.value ?? []),
        writeRules: JSON.stringify(uploadWriteRows.value ?? []),
      },
    );
    message.success(
      $t('doc.doc_uploadResult', { created: res?.created ?? 0, skipped: res?.skipped ?? 0 }),
    );
    if (res?.failed?.length) {
      const first = res.failed[0] as { error: string; name: string; };
      message.error(
        $t('doc.doc_uploadFailedSome', {
          n: res.failed.length,
          detail: `${first.name}：${first.error}`,
        }),
      );
    }
    uploadVisible.value = false;
    gridApi.query();
    loadDirs();
  } finally {
    uploadLoading.value = false;
  }
}

// ===== 更新原件（单文件，独立小弹窗） =====
const updateVisible = ref(false);
const updateLoading = ref(false);
const updatingDoc = ref<any>(null);
const updateFile = ref<File | null>(null);
const updateTitle = ref('');
const updateSummary = ref('');

function openUpdate(row: any) {
  updatingDoc.value = row;
  updateFile.value = null;
  updateTitle.value = '';
  updateSummary.value = '';
  updateVisible.value = true;
}

function onPickUpdateFile(file: any) {
  if (Number(file.size) > MAX_FILE_SIZE) {
    message.warning($t('doc.doc_fileTooLarge', { name: file.name }));
    return false; // 超限不入场（不可选）
  }
  updateFile.value = file as File;
  return false;
}

async function onSubmitUpdate() {
  if (!updateFile.value) {
    message.warning($t('doc.doc_pickFirst'));
    return;
  }
  updateLoading.value = true;
  try {
    await updateDocApi(
      updatingDoc.value.docId,
      updateFile.value,
      updateTitle.value,
      updateSummary.value,
    );
    message.success($t('doc.doc_updateOk'));
    updateVisible.value = false;
    gridApi.query();
  } finally {
    updateLoading.value = false;
  }
}

// ===== 批量移动 =====
const moveVisible = ref(false);
const moveLoading = ref(false);
const moveTarget = ref('');
const moveIds = ref<string[]>([]);

/** 目录选择树（含合成根节点） */
const dirSelectTree = computed(() => [
  {
    dirId: '',
    name: $t('doc.doc_targetDirRoot'),
    children: dirTree.value,
  },
]);

function openMove(ids: string[]) {
  moveIds.value = ids;
  moveTarget.value = selectedDirId.value;
  moveVisible.value = true;
}

/** 工具栏入口：先取勾选（无勾选时已提示并中断） */
function onToolbarMove() {
  const ids = selectedDocIds();
  if (ids) openMove(ids);
}

async function onSubmitMove() {
  moveLoading.value = true;
  try {
    await moveDocsApi(moveIds.value, moveTarget.value || '');
    message.success($t('doc.doc_moveOk'));
    moveVisible.value = false;
    gridApi.query();
    loadDirs();
  } finally {
    moveLoading.value = false;
  }
}

// ===== 重新嵌入（选中文档 或 当前目录子树） =====
const embedLoading = ref(false);

async function onEmbed() {
  const ids = (gridApi.grid?.getCheckboxRecords() || []) as any[];
  const useIds = ids.map((r) => r.docId);
  if (!useIds.length && !selectedDirId.value) {
    message.warning($t('doc.doc_embedNone'));
    return;
  }
  embedLoading.value = true;
  try {
    const res = await embedDocsApi(
      useIds.length ? { docIds: useIds } : { dirId: selectedDirId.value },
    );
    message.success(
      $t('doc.doc_embedOk', { done: res?.done ?? 0, skipped: res?.skipped ?? 0 }),
    );
    gridApi.query();
  } finally {
    embedLoading.value = false;
  }
}

// ===== 批量删除 =====
function onBatchDelete() {
  const ids = selectedDocIds();
  if (!ids) return;
  Modal.confirm({
    title: $t('doc.doc_batchDeleteConfirm', { n: ids.length }),
    onOk: async () => {
      await deleteDocApi(ids!);
      message.success($t('doc.doc_deleteOk'));
      gridApi.query();
      loadDirs();
    },
  });
}

// ===== 编辑标题/摘要 =====
const editVisible = ref(false);
const editForm = reactive({ docId: '', title: '', summary: '' });

function openEdit(row: any) {
  editForm.docId = row.docId;
  editForm.title = row.title;
  editForm.summary = row.summary || '';
  editVisible.value = true;
}

async function onSubmitEdit() {
  await editDocApi({
    docId: editForm.docId,
    title: editForm.title,
    summary: editForm.summary,
  });
  message.success($t('doc.doc_editOk'));
  editVisible.value = false;
  gridApi.query();
}

// ===== 查看（原件预览 + 抽取文本两页签） =====
const viewVisible = ref(false);
const viewLoading = ref(false);
const viewDoc = ref<any>(null);
const viewContent = ref('');
const viewTab = ref('raw');
// 原件预览：按类型分派渲染方式
const viewKind = ref<'docx' | 'img' | 'pdf' | 'text' | 'unsupported' | 'xlsx'>('unsupported');
const viewRawUrl = ref(''); // blob URL（pdf iframe / img src）
const viewRawText = ref(''); // 文本类原件原文
const viewSheets = ref<{ html: string; name: string }[]>([]); // xlsx 各 sheet 渲染结果
const viewSheetIdx = ref(0);
const viewTruncated = ref(false);
const docxContainer = ref<HTMLDivElement | null>(null);

function resetPreview() {
  if (viewRawUrl.value) {
    URL.revokeObjectURL(viewRawUrl.value);
  }
  viewRawUrl.value = '';
  viewRawText.value = '';
  viewSheets.value = [];
  viewSheetIdx.value = 0;
  viewTruncated.value = false;
  viewKind.value = 'unsupported';
  viewTab.value = 'raw';
  viewContent.value = '';
}

watch(viewVisible, (open) => {
  if (!open) {
    resetPreview(); // 关弹窗即释放 blob URL / 清容器
  }
});

async function openView(row: any) {
  viewDoc.value = row;
  resetPreview();
  viewVisible.value = true;
  viewLoading.value = true;
  try {
    // 抽取文本页签数据顺带拉上（轻量）；原件预览按类型渲染
    const [, blob] = await Promise.all([
      getDocContentApi(row.docId)
        .then((res: any) => {
          viewContent.value = res?.content || $t('doc.doc_noContent');
        })
        .catch(() => {
          viewContent.value = $t('doc.doc_noContent');
        }),
      fetchDocRawApi(row.docId).catch(() => null),
    ]);
    // 原件缺失（HTTP 200 + JSON 错误体 blob，或网络失败）：不弹错，回落抽取文本页签
    if (!blob || (blob as Blob).type.includes('json')) {
      viewTab.value = 'text';
      viewKind.value = 'unsupported';
      return;
    }
    await renderPreview(row.fileType, blob as Blob);
  } catch (e: any) {
    viewKind.value = 'unsupported';
    message.error(e?.message || $t('doc.doc_previewUnsupported'));
  } finally {
    viewLoading.value = false;
  }
}

/** 按类型把原件渲染出来（pdf=浏览器原生 / 图=img / docx·xlsx=前端解析 / 文本=原文 / 其余=提示下载） */
async function renderPreview(fileType: string, blob: Blob) {
  const ext = (fileType || '').toLowerCase();
  if (ext === 'pdf') {
    viewKind.value = 'pdf';
    viewRawUrl.value = URL.createObjectURL(blob);
    return;
  }
  if (['bmp', 'gif', 'jpeg', 'jpg', 'png', 'webp'].includes(ext)) {
    viewKind.value = 'img';
    viewRawUrl.value = URL.createObjectURL(blob);
    return;
  }
  if (ext === 'docx') {
    viewKind.value = 'docx';
    await nextTick(); // 等 v-if 渲染出容器
    await renderDocxAsync(await blob.arrayBuffer(), docxContainer.value!, undefined, {
      inWrapper: true,
    });
    return;
  }
  if (ext === 'xlsm' || ext === 'xlsx') {
    viewKind.value = 'xlsx';
    viewSheets.value = await sheetsFromXlsx(await blob.arrayBuffer());
    viewSheetIdx.value = 0;
    return;
  }
  if (['csv', 'htm', 'html', 'json', 'log', 'markdown', 'md', 'txt'].includes(ext)) {
    viewKind.value = 'text';
    viewRawText.value = await blob.text();
    return;
  }
  viewKind.value = 'unsupported';
}

/** xlsx → 各 sheet 的 HTML 表（大表截断保流畅；合并单元格按左上值平铺） */
async function sheetsFromXlsx(buf: ArrayBuffer) {
  const wb = new ExcelJS.Workbook();
  await wb.xlsx.load(buf);
  const maxRows = 1000;
  const out: { html: string; name: string }[] = [];
  wb.eachSheet((ws) => {
    const width = Math.min(ws.columnCount || 0, 100);
    const rows: string[] = [];
    const last = Math.min(ws.rowCount || 0, maxRows);
    for (let r = 1; r <= last; r++) {
      const tds: string[] = new Array(width).fill('<td></td>');
      ws.getRow(r).eachCell({ includeEmpty: false }, (cell, col) => {
        if (col <= width) {
          tds[col - 1] = `<td>${escapeHtml(cellText(cell.value))}</td>`;
        }
      });
      rows.push(`<tr>${tds.join('')}</tr>`);
    }
    if ((ws.rowCount || 0) > maxRows) {
      viewTruncated.value = true;
    }
    out.push({
      name: ws.name || `Sheet${out.length + 1}`,
      html: `<table class="doc-xlsx-table"><tbody>${rows.join('')}</tbody></table>`,
    });
  });
  return out;
}

function cellText(v: ExcelJS.CellValue): string {
  if (v === null || v === undefined) {
    return '';
  }
  if (typeof v === 'object') {
    if ('richText' in v && Array.isArray(v.richText)) {
      return v.richText.map((seg) => seg.text).join('');
    }
    if ('result' in v) {
      return String(v.result ?? '');
    }
    if ('text' in v) {
      return String(v.text ?? '');
    }
    if (v instanceof Date) {
      return v.toISOString().slice(0, 10);
    }
    if ('error' in v) {
      return String(v.error);
    }
    return '';
  }
  return String(v);
}

function escapeHtml(s: string): string {
  return s
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function onDownloadRaw() {
  if (!viewDoc.value) return;
  downloadDocRawApi(viewDoc.value.docId, viewDoc.value.fileName || viewDoc.value.title);
}

// ===== 语义检索 =====
const searchVisible = ref(false);
const searchLoading = ref(false);
const searchQuery = ref('');
const searchResults = ref<any[]>([]);

async function onSearch() {
  if (!searchQuery.value.trim()) {
    message.warning($t('doc.doc_searchPlaceholder'));
    return;
  }
  searchLoading.value = true;
  try {
    const res = await searchDocApi(searchQuery.value.trim(), 10);
    searchResults.value = res?.list ?? [];
  } finally {
    searchLoading.value = false;
  }
}

// ===== 删除（单行） =====
function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('doc.doc_deleteConfirm'),
    content: row.title,
    onOk: async () => {
      await deleteDocApi([row.docId]);
      message.success($t('doc.doc_deleteOk'));
      gridApi.query();
      loadDirs();
    },
  });
}

// ===== 访问记录 =====
const accessVisible = ref(false);
const accessLoading = ref(false);
const accessAction = ref('');
const accessDocId = ref(''); // 行入口带入；空=全部文档
const accessRows = ref<any[]>([]);
const accessTotal = ref(0);
const accessPage = reactive({ num: 1, size: 10 });

const accessActionOptions = computed(() => [
  { label: $t('doc.doc_accessAll'), value: '' },
  { label: $t('doc.doc_accessSearch'), value: 'search' },
  { label: $t('doc.doc_accessRead'), value: 'read' },
]);

const accessColumns = computed(() => [
  { title: $t('doc.doc_createdAt'), dataIndex: 'createdAt', width: 170 },
  {
    title: $t('doc.doc_user'),
    dataIndex: 'userName',
    width: 110,
    customRender: ({ text, record }: any) =>
      text || (record.source === 'mcp' ? 'MCP' : '-'),
  },
  {
    title: $t('doc.doc_actionType'),
    dataIndex: 'action',
    width: 80,
    key: 'action',
  },
  {
    title: $t('doc.doc_keywordCol'),
    key: 'target',
    customRender: ({ record }: any) =>
      record.action === 'search' ? record.keyword : record.docTitle,
  },
  { title: $t('doc.doc_hits'), dataIndex: 'hits', width: 70 },
  {
    title: $t('doc.doc_sourceCol'),
    dataIndex: 'source',
    width: 80,
    key: 'source',
  },
]);

async function loadAccess() {
  accessLoading.value = true;
  try {
    const res = await getDocAccessApi({
      action: accessAction.value,
      docId: accessDocId.value,
      pageNum: accessPage.num,
      pageSize: accessPage.size,
    });
    accessRows.value = res?.list ?? [];
    accessTotal.value = res?.total ?? 0;
  } finally {
    accessLoading.value = false;
  }
}

function openAccess(docId = '') {
  accessDocId.value = docId;
  accessAction.value = '';
  accessPage.num = 1;
  accessVisible.value = true;
  loadAccess();
}

onMounted(async () => {
  await loadDirs();
  await applyRouteIntent();
});

// 路由意图：?dirId=xxx 预选指定目录（项目抽屉「项目文档」跳转）；
// /doc/myspace 入口（我的空间）= 定位个人空间下自己的目录。
async function applyRouteIntent() {
  const dirId = String(route.query.dirId || '');
  if (dirId && dirMap.value[dirId]) {
    jumpToDir(dirId);
    return;
  }
  if (route.path === '/doc/myspace') {
    try {
      const d = await getDirByRefApi('personal', 'me');
      if (d?.dirId) jumpToDir(d.dirId);
    } catch {
      /* 拦截器已提示（目录未生成等） */
    }
  }
}

// 本组件被「文档管理」与「我的空间」两个路由复用（keep-alive 下 onMounted 只走一次），
// 路径/dirId 变化时重放意图
watch(
  () => [route.path, route.query.dirId],
  () => {
    applyRouteIntent();
  },
);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="doc-layout">
      <!-- 左栏：目录树（可收起成窄条） -->
      <div class="doc-side" :class="{ 'doc-side--collapsed': !dirSideOpen }">
        <Button
          v-if="!dirSideOpen"
          size="small"
          type="text"
          class="doc-side-expand"
          :title="$t('doc.doc_dirExpand')"
          @click="toggleDirSide"
        >
»
</Button>
        <div class="doc-side-head">
          <span>{{ $t('doc.doc_dirPanel') }}</span>
          <span class="doc-side-head-ops">
            <Button size="small" type="link" @click="openDirCreate('')">
              {{ $t('doc.doc_newRootDir') }}
            </Button>
            <Button
              size="small"
              type="text"
              :title="$t('doc.doc_dirCollapse')"
              @click="toggleDirSide"
            >«</Button>
          </span>
        </div>

        <!-- 根（全部文档）：也是文档拖拽的「移回根」落点 -->
        <div
          class="doc-side-root"
          :class="{ 'doc-side-root--active': !selectedDirId }"
          @click="selectDir('')"
          @dragover="onDirDragOver"
          @drop="onDropDocsTo('', $event)"
        >
          <span>📄 {{ $t('doc.doc_allDocs') }}</span>
          <span class="doc-count">{{ totalAllDocs }}</span>
        </div>

        <Spin :spinning="false" wrapper-class-name="doc-side-tree">
          <Tree
            v-if="treeData.length"
            :tree-data="treeData"
            :expanded-keys="expandedKeys"
            :selected-keys="selectedDirId ? [selectedDirId] : []"
            block-node
            draggable
            @expand="(keys: any) => (expandedKeys = keys)"
            @select="(keys: any) => selectDir(String(keys[0] || ''))"
            @drop="onTreeDrop"
          >
            <template #title="{ key: k, eventKey: ek }">
              <div
                class="doc-dir-node"
                @dragover="onDirDragOver"
                @drop="onDropDocsTo(dirMap[k || ek]?.dirId || '', $event)"
              >
                <span class="doc-dir-name">
                  {{ dirIcon(dirMap[k || ek]) }} {{ dirMap[k || ek]?.name }}
                  <span class="doc-count">({{ dirMap[k || ek]?.docCount ?? 0 }})</span>
                  <span
                    v-if="dirMap[k || ek]?.visibility === 0"
                    :title="$t('doc.doc_visPrivate')"
                  >🔒</span>
                  <span
                    v-else-if="dirMap[k || ek]?.visibility === 1"
                    :title="$t('doc.doc_visCond')"
                  >🔑</span>
                </span>
                <!-- 空间根无任何手工操作；托管目录（项目/个人）保留授权+建子目录，隐藏改名/删除 -->
                <span
                  v-if="dirMap[k || ek]?.canWrite && !isSpaceRoot(dirMap[k || ek])"
                  class="doc-dir-ops"
                  @click.stop
                >
                  <Button
                    size="small"
                    type="text"
                    :title="$t('doc.doc_grant')"
                    @click="openGrant('dir', dirMap[k || ek]?.dirId, dirMap[k || ek]?.name)"
                  >⚿</Button>
                  <Button
                    size="small"
                    type="text"
                    :title="$t('doc.doc_dirAddChild')"
                    @click="openDirCreate(dirMap[k || ek]?.dirId)"
                  >＋</Button>
                  <Button
                    v-if="!dirMap[k || ek]?.kind"
                    size="small"
                    type="text"
                    :title="$t('doc.doc_dirRename')"
                    @click="openDirRename(dirMap[k || ek])"
                  >✎</Button>
                  <Button
                    v-if="!dirMap[k || ek]?.kind"
                    size="small"
                    type="text"
                    danger
                    :title="$t('doc.doc_dirDelete')"
                    @click="onDeleteDir(dirMap[k || ek])"
                  >✕</Button>
                </span>
              </div>
            </template>
          </Tree>
          <div v-else class="doc-side-empty">{{ $t('doc.doc_dirEmpty') }}</div>
        </Spin>

        <div class="doc-side-tip">{{ $t('doc.doc_sideTip') }}</div>
      </div>

      <!-- 右栏：目录全路径面包屑 + 文档表格 -->
      <div class="doc-main">
        <div class="doc-crumb">
          <a @click="jumpToDir('')">📄 {{ $t('doc.doc_allDocs') }}</a>
          <template v-for="seg in dirPathChain" :key="seg.dirId">
            <span class="doc-crumb-sep">/</span>
            <a
              :class="{ 'doc-crumb-cur': seg.dirId === selectedDirId }"
              @click="jumpToDir(seg.dirId)"
            >{{ seg.name }}</a>
          </template>
        </div>
        <Grid class="doc-grid">
          <template #toolbar-actions>
            <Button type="primary" @click="openUpload">
              {{ $t('doc.doc_upload') }}
            </Button>
            <Button class="ml-2" @click="onToolbarMove">
              {{ $t('doc.doc_move') }}
            </Button>
            <Button class="ml-2" :loading="embedLoading" @click="onEmbed">
              {{ $t('doc.doc_embed') }}
            </Button>
            <Button class="ml-2" danger @click="onBatchDelete">
              {{ $t('doc.doc_batchDelete') }}
            </Button>
            <Button class="ml-2" @click="searchVisible = true">
              {{ $t('doc.doc_search') }}
            </Button>
            <Button class="ml-2" @click="openAccess()">
              {{ $t('doc.doc_accessLog') }}
            </Button>
          </template>
          <template #dragHandle="{ row }">
            <span
              class="doc-drag-handle"
              :title="$t('doc.doc_dragHandle')"
              draggable="true"
              @dragstart="onRowDragStart(row, $event)"
            >⋮⋮</span>
          </template>
          <template #fileType="{ row }">
            <span class="aa-enum aa-enum--blue">{{ (row.fileType || '-').toUpperCase() }}</span>
          </template>
          <template #fileSize="{ row }">{{ fmtSize(row.fileSize) }}</template>
          <template #visibility="{ row }">
            <span
              :class="
                row.visibility === 2
                  ? 'aa-enum aa-enum--default'
                  : row.visibility === 1
                    ? 'aa-enum aa-enum--blue'
                    : 'aa-enum aa-enum--error'
              "
            >
              {{
                row.visibility === 2
                  ? $t('doc.doc_visAll')
                  : row.visibility === 1
                    ? $t('doc.doc_visCond')
                    : $t('doc.doc_visPrivate')
              }}
            </span>
          </template>
          <template #embedStatus="{ row }">
            <!-- embedError：嵌入失败原因 / 空抽取说明（扫描件·老格式），悬停可见 -->
            <Tooltip :title="row.embedError">
              <span v-if="row.embedStatus === 1" class="aa-enum aa-enum--magenta">
                {{ $t('doc.doc_embedDone') }}
              </span>
              <span v-else-if="row.embedStatus === 2" class="aa-enum aa-enum--error">
                {{ $t('doc.doc_embedFailed') }}
              </span>
              <span v-else class="aa-enum aa-enum--default">
                {{ $t('doc.doc_embedPending') }}
              </span>
            </Tooltip>
          </template>
          <template #action="{ row }">
            <Button type="primary" ghost size="small" @click="openView(row)">
              {{ $t('doc.doc_view') }}
            </Button>
            <!-- PDF 不允许编辑 -->
            <Tooltip :title="row.fileType === 'pdf' ? $t('doc.doc_editPdfDisabled') : ''">
              <Button
                v-if="row.canWrite"
                type="primary"
                ghost
                size="small"
                class="ml-1"
                :disabled="row.fileType === 'pdf'"
                @click="openEdit(row)"
              >
                {{ $t('doc.doc_edit') }}
              </Button>
            </Tooltip>
            <Button
              v-if="row.canWrite"
              type="primary"
              ghost
              size="small"
              class="ml-1"
              @click="openUpdate(row)"
            >
              {{ $t('doc.doc_update') }}
            </Button>
            <Button
              v-if="row.canWrite"
              type="primary"
              ghost
              size="small"
              class="ml-1"
              @click="openGrant('doc', row.docId, row.title)"
            >
              {{ $t('doc.doc_grant') }}
            </Button>
            <Button type="primary" ghost size="small" class="ml-1" @click="openAccess(row.docId)">
              {{ $t('doc.doc_accessLogShort') }}
            </Button>
            <Button
              v-if="row.canWrite"
              type="primary"
              ghost
              size="small"
              danger
              class="ml-1"
              @click="onDeleteRow(row)"
            >
              {{ $t('doc.doc_delete') }}
            </Button>
          </template>
        </Grid>
      </div>
    </div>

    <!-- 上传（多文件 / 目录） -->
    <Modal
      v-model:open="uploadVisible"
      :title="$t('doc.doc_modalTitleUpload')"
      :width="620"
      :confirm-loading="uploadLoading"
      :destroy-on-close="true"
      :mask-closable="false"
      @ok="onSubmitUpload"
    >
      <div class="flex flex-col gap-3">
        <div class="flex items-center gap-3">
          <span class="w-20 shrink-0">{{ $t('doc.doc_targetDir') }}</span>
          <TreeSelect
            v-model:value="uploadDirId"
            :tree-data="dirSelectTree"
            :field-names="{ label: 'name', value: 'dirId', children: 'children' }"
            tree-default-expand-all
            tree-line
            allow-clear
            :placeholder="$t('doc.doc_targetDirRoot')"
            style="flex: 1"
          />
        </div>
        <div class="flex items-center gap-3">
          <span class="w-20 shrink-0">{{ $t('doc.doc_uploadMode') }}</span>
          <Select
            v-model:value="uploadMode"
            :options="[
              { label: $t('doc.doc_modeFiles'), value: 'files' },
              { label: $t('doc.doc_modeDir'), value: 'dir' },
            ]"
            style="width: 200px"
          />
        </div>

        <template v-if="uploadMode === 'files'">
          <Upload
            :multiple="true"
            :before-upload="onPickFile"
            :file-list="[]"
            :accept="`.${DOC_EXTS.join(',.')}`"
            :show-upload-list="false"
          >
            <Button>{{ $t('doc.doc_pickFile') }}</Button>
          </Upload>
        </template>
        <template v-else>
          <input
            ref="dirInputRef"
            type="file"
            class="hidden"
            webkitdirectory
            directory
            multiple
            @change="onDirPicked"
          />
          <Button @click="dirInputRef?.click()">
            {{ $t('doc.doc_pickDir') }}
          </Button>
        </template>
        <div class="text-xs opacity-60">{{ $t('doc.doc_sizeLimitTip') }}</div>

        <div v-if="pickedFiles.length" class="doc-picked">
          <div
            v-for="(f, i) in pickedFiles"
            :key="`${f.path}-${i}`"
            class="doc-picked-row"
          >
            <span class="flex-1 truncate">{{ f.path || f.file.name }}</span>
            <span class="opacity-50">{{ fmtSize(f.file.size) }}</span>
          </div>
        </div>

        <div class="flex items-center gap-3" :class="{ 'opacity-50': pickedFiles.length > 1 }">
          <span class="w-20 shrink-0">{{ $t('doc.doc_title') }}</span>
          <Input
            v-model:value="uploadTitle"
            :disabled="pickedFiles.length > 1"
            :placeholder="$t('doc.doc_titlePlaceholder')"
          />
        </div>
        <div class="flex gap-3" :class="{ 'opacity-50': pickedFiles.length > 1 }">
          <span class="w-20 shrink-0">{{ $t('doc.doc_summary') }}</span>
          <Textarea
            v-model:value="uploadSummary"
            :rows="2"
            :disabled="pickedFiles.length > 1"
            :placeholder="$t('doc.doc_summaryPlaceholder')"
          />
        </div>
        <div class="text-xs opacity-60">{{ $t('doc.doc_uploadHelp2') }}</div>

        <!-- 本批新建文档的授权（不给 = 默认全员可读） -->
        <div class="mt-2 border-t border-t-[color-mix(in_srgb,currentColor_12%,transparent)] pt-3">
          <div class="mb-1 font-medium">{{ $t('doc.doc_grant') }}</div>
          <div class="mb-2 flex items-center gap-3">
            <span class="w-20 shrink-0">{{ $t('doc.doc_visibility') }}</span>
            <Select
              v-model:value="uploadVis"
              :options="VIS_OPTIONS"
              style="width: 200px"
            />
          </div>
          <div class="mb-1 text-xs opacity-60">{{ $t('doc.doc_uploadVisTip') }}</div>
          <template v-if="uploadVis === 1">
            <div class="mb-1">{{ $t('doc.doc_readRules') }}</div>
            <GrantEditor
              v-model="uploadReadRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
          </template>
          <div class="mb-1">{{ $t('doc.doc_writeRules') }}</div>
          <GrantEditor
              v-model="uploadWriteRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
        </div>
      </div>
    </Modal>

    <!-- 更新原件 -->
    <Modal
      v-model:open="updateVisible"
      :title="$t('doc.doc_modalTitleUpdate')"
      :width="520"
      :confirm-loading="updateLoading"
      :destroy-on-close="true"
      @ok="onSubmitUpdate"
    >
      <div class="flex flex-col gap-3">
        <Upload
          :max-count="1"
          :before-upload="onPickUpdateFile"
          :file-list="[]"
          :accept="`.${DOC_SINGLE_EXTS.join(',.')}`"
        >
          <Button>{{ $t('doc.doc_pickFile') }}</Button>
        </Upload>
        <div class="text-xs opacity-60">{{ $t('doc.doc_sizeLimitTip') }}</div>
        <div v-if="updateFile" class="text-sm opacity-70">{{ updateFile.name }}</div>
        <div>
          <div class="mb-1">{{ $t('doc.doc_title') }}</div>
          <Input v-model:value="updateTitle" :placeholder="$t('doc.doc_titlePlaceholder')" />
        </div>
        <div>
          <div class="mb-1">{{ $t('doc.doc_summary') }}</div>
          <Textarea v-model:value="updateSummary" :rows="3" :placeholder="$t('doc.doc_summaryPlaceholder')" />
        </div>
      </div>
    </Modal>

    <!-- 批量移动 -->
    <Modal
      v-model:open="moveVisible"
      :title="$t('doc.doc_move')"
      :width="480"
      :confirm-loading="moveLoading"
      :destroy-on-close="true"
      @ok="onSubmitMove"
    >
      <div class="flex flex-col gap-3">
        <div class="text-sm opacity-70">
          {{ $t('doc.doc_moveCount', { n: moveIds.length }) }}
        </div>
        <TreeSelect
          v-model:value="moveTarget"
          :tree-data="dirSelectTree"
          :field-names="{ label: 'name', value: 'dirId', children: 'children' }"
          tree-default-expand-all
          tree-line
          allow-clear
          :placeholder="$t('doc.doc_targetDirRoot')"
        />
      </div>
    </Modal>

    <!-- 目录新建/重命名（新建可一并配授权） -->
    <Modal
      v-model:open="dirModal.open"
      :title="dirModal.mode === 'create' ? $t('doc.doc_dirModalCreate') : $t('doc.doc_dirModalRename')"
      :width="dirModal.mode === 'create' ? 680 : 440"
      :destroy-on-close="true"
      @ok="onSubmitDir"
    >
      <div class="flex flex-col gap-2">
        <div>{{ $t('doc.doc_dirName') }}</div>
        <Input
          v-model:value="dirModal.name"
          :placeholder="$t('doc.doc_dirNamePlaceholder')"
          @press-enter="onSubmitDir"
        />
        <template v-if="dirModal.mode === 'create'">
          <div class="mt-2">{{ $t('doc.doc_visibility') }}</div>
          <Select
            v-model:value="dirModal.visibility"
            :options="VIS_OPTIONS"
            style="width: 200px"
          />
          <template v-if="dirModal.visibility === 1">
            <div class="mt-1">{{ $t('doc.doc_readRules') }}</div>
            <GrantEditor
              v-model="dirReadRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
          </template>
          <div class="mt-1">{{ $t('doc.doc_writeRules') }}</div>
          <GrantEditor
              v-model="dirWriteRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
        </template>
      </div>
    </Modal>

    <!-- 组合授权（文档/目录共用：可见性档位 + 读/写条件行） -->
    <Modal
      v-model:open="grantModal.open"
      :title="$t('doc.doc_grantTitle')"
      :width="680"
      :confirm-loading="grantModal.loading"
      :destroy-on-close="true"
      @ok="onSubmitGrant"
    >
      <div class="flex flex-col gap-2">
        <div class="text-sm opacity-70">{{ grantModal.name }}</div>
        <div>{{ $t('doc.doc_visibility') }}</div>
        <Select
          v-model:value="grantModal.visibility"
          :options="VIS_OPTIONS"
          style="width: 200px"
        />
        <div class="text-xs opacity-60">{{ $t('doc.doc_visTip') }}</div>
        <template v-if="grantModal.visibility === 1">
          <div class="mt-1">{{ $t('doc.doc_readRules') }}</div>
          <GrantEditor
              v-model="grantReadRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
        </template>
        <div class="mt-1">{{ $t('doc.doc_writeRules') }}</div>
        <GrantEditor
              v-model="grantWriteRows"
              :default-mods="['users', 'dept']"
              :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
            />
        <div class="text-xs opacity-60">{{ $t('doc.doc_writeRulesTip') }}</div>
      </div>
    </Modal>

    <!-- 编辑标题/摘要 -->
    <Modal
      v-model:open="editVisible"
      :title="$t('doc.doc_modalTitleEdit')"
      :width="520"
      :destroy-on-close="true"
      @ok="onSubmitEdit"
    >
      <div class="flex flex-col gap-3">
        <div>
          <div class="mb-1">{{ $t('doc.doc_title') }}</div>
          <Input v-model:value="editForm.title" />
        </div>
        <div>
          <div class="mb-1">{{ $t('doc.doc_summary') }}</div>
          <Textarea v-model:value="editForm.summary" :rows="3" />
        </div>
        <div class="text-xs opacity-60">{{ $t('doc.doc_editHelp') }}</div>
      </div>
    </Modal>

    <!-- 查看：原件预览 + 抽取文本 -->
    <Modal
      v-model:open="viewVisible"
      :title="viewDoc?.title"
      :width="1100"
      :footer="null"
      :destroy-on-close="true"
      :body-style="{ paddingTop: '12px' }"
    >
      <div class="doc-view-bar">
        <Tabs v-model:active-key="viewTab" size="small">
          <Tabs.TabPane key="raw" :tab="$t('doc.doc_tabRaw')" />
          <Tabs.TabPane key="text" :tab="$t('doc.doc_tabText')" />
        </Tabs>
        <Button size="small" @click="onDownloadRaw">
          {{ $t('doc.doc_downloadRaw') }}
        </Button>
      </div>
      <Spin :spinning="viewLoading">
        <!-- 原件预览 -->
        <div v-show="viewTab === 'raw'" class="doc-preview">
          <iframe
            v-if="viewKind === 'pdf'"
            :src="viewRawUrl"
            class="doc-preview-frame"
          ></iframe>
          <div v-else-if="viewKind === 'img'" class="doc-preview-imgwrap">
            <img :src="viewRawUrl" :alt="viewDoc?.title" class="doc-preview-img" />
          </div>
          <div v-else-if="viewKind === 'docx'" ref="docxContainer" class="doc-preview-docx"></div>
          <div v-else-if="viewKind === 'xlsx'" class="doc-preview-xlsx">
            <div v-if="viewSheets.length > 1" class="doc-xlsx-tabs">
              <Button
                v-for="(s, i) in viewSheets"
                :key="s.name"
                size="small"
                :type="i === viewSheetIdx ? 'primary' : 'default'"
                @click="viewSheetIdx = i"
              >
{{ s.name }}
</Button>
            </div>
            <div v-if="viewTruncated" class="doc-preview-note">
              {{ $t('doc.doc_previewTruncated') }}
            </div>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div class="doc-xlsx-scroll" v-html="viewSheets[viewSheetIdx]?.html"></div>
          </div>
          <pre v-else-if="viewKind === 'text'" class="doc-content-pre">{{ viewRawText }}</pre>
          <div v-else class="doc-preview-unsupported">
            <div>{{ $t('doc.doc_previewUnsupported') }}</div>
            <Button class="mt-3" @click="onDownloadRaw">
              {{ $t('doc.doc_downloadRaw') }}
            </Button>
          </div>
        </div>
        <!-- 抽取文本（入库检索所见） -->
        <pre v-show="viewTab === 'text'" class="doc-content-pre">{{ viewContent }}</pre>
      </Spin>
    </Modal>

    <!-- 语义检索 -->
    <Modal
      v-model:open="searchVisible"
      :title="$t('doc.doc_searchTitle')"
      :width="760"
      :footer="null"
      :destroy-on-close="true"
    >
      <div class="mb-3 flex gap-2">
        <Input
          v-model:value="searchQuery"
          :placeholder="$t('doc.doc_searchPlaceholder')"
          allow-clear
          @press-enter="onSearch"
        />
        <Button type="primary" :loading="searchLoading" @click="onSearch">
          {{ $t('doc.doc_search') }}
        </Button>
      </div>
      <Spin :spinning="searchLoading">
        <div v-if="searchResults.length === 0" class="py-6 text-center text-sm opacity-60">
          {{ $t('doc.doc_searchEmpty') }}
        </div>
        <div
          v-for="(hit, i) in searchResults"
          :key="`${hit.docId}-${hit.seq}`"
          class="doc-hit"
        >
          <div class="doc-hit-head">
            <span class="aa-enum aa-enum--magenta">#{{ i + 1 }}</span>
            <span class="doc-hit-title">{{ hit.title }}</span>
            <span class="doc-hit-score">
              {{ $t('doc.doc_score') }}: {{ (hit.score ?? 0).toFixed(3) }}
            </span>
          </div>
          <div class="doc-hit-content">{{ hit.content }}</div>
        </div>
      </Spin>
    </Modal>

    <!-- 访问记录 -->
    <Modal
      v-model:open="accessVisible"
      :title="$t('doc.doc_accessLog')"
      :width="860"
      :footer="null"
      :destroy-on-close="true"
    >
      <div class="mb-3 flex items-center gap-2">
        <Select
          v-model:value="accessAction"
          :options="accessActionOptions"
          style="width: 140px"
          @change="() => { accessPage.num = 1; loadAccess(); }"
        />
        <span v-if="accessDocId" class="text-xs opacity-60">{{ $t('doc.doc_filteredByDoc') }}</span>
      </div>
      <Table
        :columns="accessColumns"
        :data-source="accessRows"
        :loading="accessLoading"
        :pagination="{
          current: accessPage.num,
          pageSize: accessPage.size,
          total: accessTotal,
          showSizeChanger: false,
          onChange: (p: number) => { accessPage.num = p; loadAccess(); },
        }"
        size="small"
        :row-key="(_: any, i: number) => i"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            <span
              :class="record.action === 'search' ? 'aa-enum aa-enum--blue' : 'aa-enum aa-enum--magenta'"
            >
              {{ record.action === 'search' ? $t('doc.doc_accessSearch') : $t('doc.doc_accessRead') }}
            </span>
          </template>
          <template v-else-if="column.key === 'source'">
            <span
              :class="record.source === 'mcp' ? 'aa-enum aa-enum--magenta' : 'aa-enum aa-enum--default'"
            >
              {{ record.source === 'mcp' ? 'MCP' : 'UI' }}
            </span>
          </template>
        </template>
      </Table>
    </Modal>
  </Page>
</template>

<style scoped>
.doc-layout {
  display: flex;
  gap: 12px;
  height: 100%;
}
.doc-side {
  display: flex;
  width: 264px;
  flex-direction: column;
  flex-shrink: 0;
  padding: 10px;
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 8px;
  overflow: hidden;
  transition: width 0.2s ease, padding 0.2s ease;
}
/* 收起成窄条：只留展开钮，右栏表格随之变宽 */
.doc-side--collapsed {
  width: 34px;
  min-width: 34px;
  padding: 10px 2px;
  align-items: center;
}
.doc-side--collapsed > :not(.doc-side-expand) {
  display: none;
}
.doc-side-expand {
  flex-shrink: 0;
}
.doc-side-head-ops {
  display: inline-flex;
  align-items: center;
}
.doc-side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
}
.doc-side-root {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 4px;
}
.doc-side-root:hover {
  background: rgba(128, 128, 128, 0.12);
}
.doc-side-root--active {
  background: rgba(22, 119, 255, 0.15);
}
.doc-side-tree {
  flex: 1;
  overflow: auto;
  min-height: 0;
}
.doc-side-empty {
  padding: 12px 8px;
  font-size: 12px;
  opacity: 0.55;
}
.doc-side-tip {
  padding-top: 8px;
  margin-top: 6px;
  border-top: 1px solid rgba(128, 128, 128, 0.2);
  font-size: 12px;
  line-height: 1.5;
  opacity: 0.55;
}
.doc-count {
  font-size: 12px;
  opacity: 0.55;
}
.doc-dir-node {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}
.doc-dir-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.doc-dir-ops {
  display: none;
  flex-shrink: 0;
  gap: 2px;
}
.doc-dir-node:hover .doc-dir-ops {
  display: inline-flex;
}
.doc-main {
  flex: 1;
  height: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
/* 当前目录全路径面包屑：每级可点跳转（与左树联动展开高亮） */
.doc-crumb {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
  padding: 4px 8px;
  margin-bottom: 6px;
  border: 1px solid hsl(var(--border));
  border-radius: 6px;
  background: hsl(var(--card));
  font-size: 13px;
}
.doc-crumb a {
  color: hsl(var(--primary));
  cursor: pointer;
  white-space: nowrap;
}
.doc-crumb a:hover {
  text-decoration: underline;
}
.doc-crumb-sep {
  opacity: 0.35;
  margin: 0 2px;
}
.doc-crumb-cur {
  font-weight: 600;
}
.doc-grid {
  flex: 1;
  min-height: 0;
}
.doc-drag-handle {
  cursor: grab;
  user-select: none;
  opacity: 0.4;
  letter-spacing: -2px;
}
.doc-drag-handle:hover {
  opacity: 1;
}
.doc-picked {
  max-height: 160px;
  overflow: auto;
  padding: 8px;
  border: 1px dashed rgba(128, 128, 128, 0.35);
  border-radius: 6px;
  font-size: 12px;
}
.doc-picked-row {
  display: flex;
  gap: 8px;
  padding: 2px 0;
}
.doc-content-pre {
  max-height: 65vh;
  overflow: auto;
  margin: 0;
  padding: 12px;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 13px;
  line-height: 1.7;
}
/* ===== 原件预览 ===== */
.doc-view-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-right: 8px;
}
.doc-preview {
  max-height: 74vh;
  overflow: auto;
}
.doc-preview-frame {
  width: 100%;
  height: 74vh;
  border: 0;
}
.doc-preview-imgwrap {
  text-align: center;
  padding: 8px 0;
}
.doc-preview-img {
  max-width: 100%;
}
/* docx 恒白底：保真还原原件观感（与 PDF 页面同理，不吃暗色主题） */
.doc-preview-docx {
  min-height: 200px;
  padding: 16px;
  background: #fff;
  border-radius: 8px;
}
.doc-xlsx-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}
.doc-xlsx-scroll {
  max-height: 66vh;
  overflow: auto;
}
.doc-xlsx-table {
  border-collapse: collapse;
  font-size: 12px;
}
.doc-xlsx-table td {
  padding: 3px 8px;
  border: 1px solid rgba(128, 128, 128, 0.35);
  white-space: nowrap;
}
.doc-preview-note {
  margin-bottom: 6px;
  font-size: 12px;
  opacity: 0.6;
}
.doc-preview-unsupported {
  padding: 48px 0;
  text-align: center;
  opacity: 0.75;
}
.doc-hit {
  padding: 10px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
}
.doc-hit + .doc-hit {
  margin-top: 8px;
}
.doc-hit-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.doc-hit-title {
  font-weight: 600;
}
.doc-hit-score {
  margin-left: auto;
  font-size: 12px;
  opacity: 0.65;
}
.doc-hit-content {
  font-size: 13px;
  line-height: 1.7;
  opacity: 0.85;
  max-height: 140px;
  overflow: hidden;
}
</style>
