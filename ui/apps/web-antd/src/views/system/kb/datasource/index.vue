<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Drawer,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Switch,
  Table,
  Tag,
  Textarea,
  Tooltip,
  TreeSelect,
} from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  addKbDatasourceApi,
  cancelKbDatasourceRunApi,
  deleteKbDatasourceApi,
  editKbDatasourceApi,
  getKbDatasourceListApi,
  getKbDatasourceRunsApi,
  triggerKbDatasourceApi,
} from '#/api/system/kb/datasource';
import type { KbDatasourceApi } from '#/api/system/kb/datasource';
import { getKbDirTreeApi } from '#/api/system/kb';
import { $t } from '#/locales';

defineOptions({ name: 'SystemKbDatasource' });

// ===== 类型元数据（tag 色 + 目标摘要） =====
const TYPE_META: Record<string, { color: string; key: string }> = {
  api: { color: 'geekblue', key: 'kb.ds_type_api' },
  database: { color: 'purple', key: 'kb.ds_type_database' },
  external_kb: { color: 'cyan', key: 'kb.ds_type_externalKb' },
  object_storage: { color: 'orange', key: 'kb.ds_type_objectStorage' },
  web_crawl: { color: 'blue', key: 'kb.ds_type_webCrawl' },
};

const TYPE_OPTIONS = computed(() =>
  Object.entries(TYPE_META).map(([value, m]) => ({
    label: $t(m.key),
    value,
  })),
);

function typeTag(type?: string) {
  const m = (type && TYPE_META[type]) || TYPE_META.api!;
  return { color: m.color, text: $t(m.key) };
}

/** 各类型目标摘要（表格「同步对象」列） */
function targetSummary(row: KbDatasourceApi.DatasourceItem): string {
  const c: any = row.config ?? {};
  switch (row.type) {
    case 'api': {
      return c.url || '-';
    }
    case 'database': {
      return c.host ? `${c.host}:${c.port || ''}/${c.database}` : '-';
    }
    case 'external_kb': {
      return c.baseUrl || '-';
    }
    case 'object_storage': {
      return c.bucket ? `${c.provider || ''}/${c.bucket}` : '-';
    }
    case 'web_crawl': {
      return (c.startUrls || [])[0] || '-';
    }
    default: {
      return '-';
    }
  }
}

const RUN_TAG: Record<string, { color: string; key: string }> = {
  failed: { color: 'error', key: 'kb.ds_run_failed' },
  partial: { color: 'warning', key: 'kb.ds_run_partial' },
  pending: { color: 'processing', key: 'kb.ds_run_pending' },
  running: { color: 'processing', key: 'kb.ds_run_running' },
  success: { color: 'success', key: 'kb.ds_run_success' },
};

function runTag(s?: string) {
  const m = (s && RUN_TAG[s]) || RUN_TAG.pending!;
  return { color: m.color, text: $t(m.key) };
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyword',
    label: $t('kb.ds_keyword'),
    componentProps: { placeholder: $t('kb.ds_keywordPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'type',
    label: $t('kb.ds_type'),
    componentProps: {
      allowClear: true,
      options: TYPE_OPTIONS.value,
      placeholder: $t('kb.ds_typePlaceholder'),
      style: { width: '100%' },
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('kb.ds_seq'), width: 60 },
    { field: 'name', title: $t('kb.ds_name'), minWidth: 140 },
    { field: 'type', title: $t('kb.ds_type'), width: 110, slots: { default: 'type' } },
    { field: 'target', title: $t('kb.ds_target'), minWidth: 180, slots: { default: 'target' } },
    { field: 'cron', title: $t('kb.ds_cron'), width: 130, slots: { default: 'cron' } },
    { field: 'enabled', title: $t('kb.ds_enabled'), width: 80, slots: { default: 'enabled' } },
    { field: 'lastRun', title: $t('kb.ds_lastRun'), width: 110, slots: { default: 'lastRun' } },
    { field: 'lastRunAt', title: $t('kb.ds_lastRunAt'), width: 160, formatter: ({ cellValue }: any) => fmtTime(cellValue) },
    { field: 'action', title: $t('kb.ds_action'), width: 300, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getKbDatasourceListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
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

// ===== 展示工具 =====
function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

function fmtDuration(ms?: number) {
  if (!Number.isFinite(ms) || !ms) return '-';
  if (ms < 1000) return `${ms}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  return `${Math.floor(s / 60)}m${s % 60}s`;
}

// ===== 新增/编辑弹窗（各类型动态字段，antd Form 直排） =====
const modalVisible = ref(false);
const isEdit = ref(false);
const saving = ref(false);
const formState = reactive<KbDatasourceApi.DatasourceReq & { config: any }>({
  datasourceId: '',
  name: '',
  type: 'web_crawl',
  dirId: '',
  description: '',
  enabled: 1,
  cron: '',
  // 函数声明提升可用：按默认类型生成对应骨架，避免空对象与 DatasourceConfig 联合类型不匹配
  config: defaultConfig('web_crawl'),
});

const dirTree = ref<any[]>([]);
async function loadDirTree() {
  try {
    const res = await getKbDirTreeApi();
    dirTree.value = res ?? [];
  } catch {
    dirTree.value = [];
  }
}

const dirTreeFieldNames = { children: 'children', label: 'name', value: 'dirId' };

/** 类型切换：重置 config 为该类型默认骨架 */
function onTypeChange(value: any) {
  formState.type = value;
  formState.config = defaultConfig(value);
}

function defaultConfig(type: string): any {
  switch (type) {
    case 'api': {
      return {
        authType: 'none',
        contentField: '',
        headers: '',
        listPath: '',
        method: 'GET',
        params: '',
        titleField: '',
        url: '',
      };
    }
    case 'database': {
      return {
        database: '',
        driver: 'postgres',
        host: '',
        incrementColumn: '',
        password: '',
        port: 5432,
        query: '',
        username: '',
      };
    }
    case 'external_kb': {
      return {
        accessToken: '',
        appId: '',
        appSecret: '',
        baseUrl: '',
        docTypes: [],
        provider: 'feishu',
        spaceId: '',
      };
    }
    case 'object_storage': {
      return {
        accessKey: '',
        bucket: '',
        endpoint: '',
        fileTypes: [],
        prefix: '',
        provider: 's3',
        region: '',
        secretKey: '',
      };
    }
    default: {
      return {
        excludePatterns: [],
        includePatterns: [],
        maxDepth: 2,
        maxPages: 50,
        selector: '',
        startUrls: [],
      };
    }
  }
}

function openAdd() {
  isEdit.value = false;
  Object.assign(formState, {
    datasourceId: '',
    name: '',
    type: 'web_crawl',
    dirId: '',
    description: '',
    enabled: 1,
    cron: '',
    config: defaultConfig('web_crawl'),
  });
  loadDirTree();
  modalVisible.value = true;
}

function openEdit(row: KbDatasourceApi.DatasourceItem) {
  isEdit.value = true;
  Object.assign(formState, {
    datasourceId: row.datasourceId,
    name: row.name,
    type: row.type,
    dirId: row.dirId || '',
    description: row.description || '',
    enabled: row.enabled,
    cron: row.cron || '',
    config: defaultConfig(row.type),
  });
  // 后端回传的 config 按字段并入骨架（缺字段给默认值，多余字段丢弃）
  Object.assign(formState.config, row.config ?? {});
  loadDirTree();
  modalVisible.value = true;
}

function validateForm(): string {
  if (!formState.name.trim()) return $t('kb.ds_form_nameRequired');
  const c = formState.config;
  switch (formState.type) {
    case 'api': {
      if (!c.url?.trim()) return $t('kb.ds_form_urlRequired');
      break;
    }
    case 'database': {
      if (!c.host?.trim() || !c.database?.trim() || !c.query?.trim()) {
        return $t('kb.ds_form_dbRequired');
      }
      break;
    }
    case 'external_kb': {
      if (!c.baseUrl?.trim()) return $t('kb.ds_form_urlRequired');
      break;
    }
    case 'object_storage': {
      if (!c.endpoint?.trim() || !c.bucket?.trim()) {
        return $t('kb.ds_form_storageRequired');
      }
      break;
    }
    default: {
      if (!c.startUrls?.length) return $t('kb.ds_form_urlsRequired');
    }
  }
  if (formState.cron?.trim()) {
    // 5 段 cron 粗校验（秒/分/时/日/月 与 分/时/日/月/周 两种均放行到后端细校验）
    const segs = formState.cron.trim().split(/\s+/);
    if (segs.length !== 5) return $t('kb.ds_form_cronInvalid');
  }
  return '';
}

async function onSave() {
  const err = validateForm();
  if (err) {
    message.warning(err);
    return;
  }
  saving.value = true;
  try {
    const payload: KbDatasourceApi.DatasourceReq = {
      name: formState.name.trim(),
      type: formState.type,
      dirId: formState.dirId || '',
      description: formState.description?.trim(),
      enabled: formState.enabled,
      cron: formState.cron?.trim() || '',
      config: formState.config,
    };
    if (isEdit.value) {
      await editKbDatasourceApi({ ...payload, datasourceId: formState.datasourceId });
      message.success($t('kb.ds_editSuccess'));
    } else {
      await addKbDatasourceApi(payload);
      message.success($t('kb.ds_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  } finally {
    saving.value = false;
  }
}

// 行内启停：edit 整行，失败回滚
async function onEnabledChange(row: any, checked: boolean) {
  const prev = row.enabled;
  row.enabled = checked ? 1 : 0;
  try {
    await editKbDatasourceApi({
      datasourceId: row.datasourceId,
      name: row.name,
      type: row.type,
      dirId: row.dirId || '',
      description: row.description,
      enabled: checked ? 1 : 0,
      cron: row.cron || '',
      config: row.config,
    });
    message.success($t('kb.ds_toggleSuccess'));
  } catch {
    row.enabled = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('kb.ds_selectToDelete'));
    return;
  }
  doDelete(rows.map((r: any) => r.datasourceId), rows.length);
}
async function onDeleteRow(row: any) {
  doDelete([row.datasourceId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('kb.ds_confirmDeleteTitle'),
    content: $t('kb.ds_confirmDeleteContent', { count }),
    okText: $t('kb.ds_confirm'),
    cancelText: $t('kb.ds_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteKbDatasourceApi(ids);
      message.success($t('kb.ds_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 手动同步 =====
async function onTrigger(row: any) {
  try {
    await triggerKbDatasourceApi(row.datasourceId);
    message.success($t('kb.ds_triggerSuccess'));
    gridApi.query();
    // 若运行历史抽屉正开着这条数据源，顺手刷新
    if (runsDatasource.value?.datasourceId === row.datasourceId) {
      loadRuns();
    }
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 运行历史抽屉 =====
const runsVisible = ref(false);
const runsLoading = ref(false);
const runsDatasource = ref<KbDatasourceApi.DatasourceItem | null>(null);
const runs = ref<KbDatasourceApi.RunItem[]>([]);
const runsTotal = ref(0);
const runsPage = reactive({ pageNum: 1, pageSize: 10 });

async function openRuns(row: KbDatasourceApi.DatasourceItem) {
  runsDatasource.value = row;
  runsPage.pageNum = 1;
  runsVisible.value = true;
  await loadRuns();
}

async function loadRuns() {
  if (!runsDatasource.value) return;
  runsLoading.value = true;
  try {
    const res = await getKbDatasourceRunsApi(runsDatasource.value.datasourceId, {
      pageNum: runsPage.pageNum,
      pageSize: runsPage.pageSize,
    });
    runs.value = res?.list ?? [];
    runsTotal.value = res?.total ?? 0;
  } catch {
    runs.value = [];
    runsTotal.value = 0;
  } finally {
    runsLoading.value = false;
  }
}

const runsPagination = computed(() => ({
  current: runsPage.pageNum,
  onChange: (p: number, s: number) => {
    runsPage.pageNum = p;
    runsPage.pageSize = s;
    loadRuns();
  },
  pageSize: runsPage.pageSize,
  showSizeChanger: false,
  total: runsTotal.value,
}));

async function onCancelRun(run: KbDatasourceApi.RunItem) {
  try {
    await cancelKbDatasourceRunApi(run.runId);
    message.success($t('kb.ds_runCancelSuccess'));
    loadRuns();
  } catch {
    /* 拦截器已提示 */
  }
}

const runColumns = computed(() => [
  { dataIndex: 'status', title: () => $t('kb.ds_run_status'), width: 100 },
  { dataIndex: 'trigger', title: () => $t('kb.ds_run_trigger'), width: 90 },
  { dataIndex: 'startedAt', title: () => $t('kb.ds_run_startedAt'), width: 160, customRender: ({ text }: any) => fmtTime(text) },
  { dataIndex: 'durationMs', title: () => $t('kb.ds_run_duration'), width: 90, customRender: ({ text }: any) => fmtDuration(text) },
  { dataIndex: 'stats', title: () => $t('kb.ds_run_stats'), minWidth: 220 },
  { dataIndex: 'error', title: () => $t('kb.ds_run_error'), minWidth: 160 },
  { key: 'op', title: () => $t('kb.ds_action'), width: 90 },
]);

function statsText(s?: KbDatasourceApi.RunItem['stats']) {
  if (!s) return '-';
  return $t('kb.ds_run_statsText', {
    created: s.docsCreated ?? 0,
    chunks: s.chunksCreated ?? 0,
    failed: s.failed ?? 0,
    fetched: s.fetched ?? 0,
    skipped: s.skipped ?? 0,
    updated: s.docsUpdated ?? 0,
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('kb.ds_add') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('kb.ds_batchDelete') }}
        </Button>
      </template>
      <template #type="{ row }">
        <span :class="['aa-enum', `aa-enum--${typeTag(row.type).color}`]">
          {{ typeTag(row.type).text }}
        </span>
      </template>
      <template #target="{ row }">
        <span class="font-mono text-xs break-all">{{ targetSummary(row as KbDatasourceApi.DatasourceItem) }}</span>
      </template>
      <template #cron="{ row }">
        <code v-if="row.cron" class="text-xs">{{ row.cron }}</code>
        <span v-else class="text-xs opacity-50">{{ $t('kb.ds_cronManual') }}</span>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('kb.ds_enabledOn')"
          :un-checked-children="$t('kb.ds_enabledOff')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #lastRun="{ row }">
        <span
          v-if="row.lastRunStatus"
          :class="['aa-enum', `aa-enum--${runTag(row.lastRunStatus).color}`]"
        >
          {{ runTag(row.lastRunStatus).text }}
        </span>
        <span v-else class="text-xs opacity-50">-</span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="onTrigger(row as KbDatasourceApi.DatasourceItem)">
          {{ $t('kb.ds_sync') }}
        </Button>
        <Button type="primary" ghost size="small" @click="openRuns(row as KbDatasourceApi.DatasourceItem)">
          {{ $t('kb.ds_runs') }}
        </Button>
        <Button type="primary" ghost size="small" @click="openEdit(row as KbDatasourceApi.DatasourceItem)">
          {{ $t('kb.ds_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('kb.ds_delete') }}
        </Button>
      </template>
    </Grid>

    <!-- 新增/编辑弹窗 -->
    <Modal
      v-model:open="modalVisible"
      :title="isEdit ? $t('kb.ds_modalTitleEdit') : $t('kb.ds_modalTitleAdd')"
      :width="680"
      :confirm-loading="saving"
      @ok="onSave"
    >
      <Form layout="vertical" class="pt-2">
        <div class="flex gap-3">
          <Form.Item :label="$t('kb.ds_name')" required class="flex-1">
            <Input v-model:value="formState.name" :maxlength="100" :placeholder="$t('kb.ds_namePlaceholder')" />
          </Form.Item>
          <Form.Item :label="$t('kb.ds_type')" required class="flex-1">
            <Select
              :value="formState.type"
              :options="TYPE_OPTIONS"
              :disabled="isEdit"
              :placeholder="$t('kb.ds_typePlaceholder')"
              @change="onTypeChange"
            />
          </Form.Item>
        </div>
        <Form.Item :label="$t('kb.ds_dir')">
          <TreeSelect
            v-model:value="formState.dirId"
            allow-clear
            tree-default-expand-all
            :field-names="dirTreeFieldNames"
            :placeholder="$t('kb.ds_dirPlaceholder')"
            :tree-data="dirTree"
          />
        </Form.Item>
        <Form.Item :label="$t('kb.ds_desc')">
          <Textarea v-model:value="formState.description" :rows="2" :maxlength="200" />
        </Form.Item>
        <div class="flex gap-3">
          <Form.Item :label="$t('kb.ds_cron')" class="flex-1">
            <Input v-model:value="formState.cron" :placeholder="$t('kb.ds_cronPlaceholder')" />
          </Form.Item>
          <Form.Item :label="$t('kb.ds_enabled')" class="w-32">
            <Switch
              :checked="formState.enabled === 1"
              :checked-children="$t('kb.ds_enabledOn')"
              :un-checked-children="$t('kb.ds_enabledOff')"
              @change="(v: any) => (formState.enabled = v ? 1 : 0)"
            />
          </Form.Item>
        </div>

        <!-- web_crawl：网页爬取 -->
        <template v-if="formState.type === 'web_crawl'">
          <Form.Item :label="$t('kb.ds_wc_startUrls')" required>
            <Select
              v-model:value="formState.config.startUrls"
              mode="tags"
              :token-separators="[',', ' ']"
              :open="false"
              :placeholder="$t('kb.ds_wc_startUrlsPh')"
            />
          </Form.Item>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_wc_maxDepth')" class="w-40">
              <InputNumber v-model:value="formState.config.maxDepth" :min="1" :max="10" class="w-full" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_wc_maxPages')" class="w-40">
              <InputNumber v-model:value="formState.config.maxPages" :min="1" :max="10_000" class="w-full" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_wc_selector')" class="flex-1">
              <Input v-model:value="formState.config.selector" :placeholder="$t('kb.ds_wc_selectorPh')" />
            </Form.Item>
          </div>
          <Form.Item :label="$t('kb.ds_wc_include')">
            <Select v-model:value="formState.config.includePatterns" mode="tags" :open="false" :placeholder="$t('kb.ds_wc_includePh')" />
          </Form.Item>
          <Form.Item :label="$t('kb.ds_wc_exclude')">
            <Select v-model:value="formState.config.excludePatterns" mode="tags" :open="false" :placeholder="$t('kb.ds_wc_excludePh')" />
          </Form.Item>
        </template>

        <!-- database：数据库 -->
        <template v-else-if="formState.type === 'database'">
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_db_driver')" class="w-40">
              <Select
                v-model:value="formState.config.driver"
                :options="[
                  { label: 'PostgreSQL', value: 'postgres' },
                  { label: 'MySQL', value: 'mysql' },
                  { label: 'SQLite', value: 'sqlite' },
                  { label: 'Oracle', value: 'oracle' },
                ]"
              />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_db_host')" required class="flex-1">
              <Input v-model:value="formState.config.host" placeholder="127.0.0.1" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_db_port')" class="w-32">
              <InputNumber v-model:value="formState.config.port" :min="1" :max="65_535" class="w-full" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_db_database')" required class="flex-1">
              <Input v-model:value="formState.config.database" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_db_username')" class="flex-1">
              <Input v-model:value="formState.config.username" autocomplete="off" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_db_password')" class="flex-1">
              <Input.Password v-model:value="formState.config.password" autocomplete="new-password" :placeholder="isEdit ? $t('kb.ds_secretKeep') : ''" />
            </Form.Item>
          </div>
          <Form.Item :label="$t('kb.ds_db_query')" required>
            <Textarea v-model:value="formState.config.query" :rows="3" placeholder="SELECT id, title, content, updated_at FROM docs" />
          </Form.Item>
          <Form.Item :label="$t('kb.ds_db_increment')">
            <Input v-model:value="formState.config.incrementColumn" placeholder="updated_at" />
          </Form.Item>
        </template>

        <!-- api：API 拉取 -->
        <template v-else-if="formState.type === 'api'">
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_api_method')" class="w-28">
              <Select
                v-model:value="formState.config.method"
                :options="[
                  { label: 'GET', value: 'GET' },
                  { label: 'POST', value: 'POST' },
                ]"
              />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_api_url')" required class="flex-1">
              <Input v-model:value="formState.config.url" placeholder="https://…" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_api_auth')" class="w-40">
              <Select
                v-model:value="formState.config.authType"
                :options="[
                  { label: 'None', value: 'none' },
                  { label: 'Bearer', value: 'bearer' },
                  { label: 'Basic', value: 'basic' },
                ]"
              />
            </Form.Item>
            <Form.Item v-if="formState.config.authType === 'bearer'" :label="$t('kb.ds_api_token')" class="flex-1">
              <Input.Password v-model:value="formState.config.authToken" autocomplete="new-password" :placeholder="isEdit ? $t('kb.ds_secretKeep') : ''" />
            </Form.Item>
          </div>
          <Form.Item :label="$t('kb.ds_api_headers')">
            <Textarea v-model:value="formState.config.headers" :rows="2" placeholder='{"X-Token":"..."}' />
          </Form.Item>
          <Form.Item :label="$t('kb.ds_api_params')">
            <Textarea v-model:value="formState.config.params" :rows="2" placeholder='{"limit":100}' />
          </Form.Item>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_api_listPath')" class="flex-1">
              <Input v-model:value="formState.config.listPath" placeholder="data.list" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_api_titleField')" class="flex-1">
              <Input v-model:value="formState.config.titleField" placeholder="title" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_api_contentField')" class="flex-1">
              <Input v-model:value="formState.config.contentField" placeholder="content" />
            </Form.Item>
          </div>
        </template>

        <!-- object_storage：对象存储 -->
        <template v-else-if="formState.type === 'object_storage'">
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_os_provider')" class="w-40">
              <Select
                v-model:value="formState.config.provider"
                :options="[
                  { label: 'S3', value: 's3' },
                  { label: 'OSS', value: 'oss' },
                  { label: 'COS', value: 'cos' },
                  { label: 'MinIO', value: 'minio' },
                ]"
              />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_os_endpoint')" required class="flex-1">
              <Input v-model:value="formState.config.endpoint" placeholder="https://s3.example.com" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_os_region')" class="w-40">
              <Input v-model:value="formState.config.region" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_os_bucket')" required class="flex-1">
              <Input v-model:value="formState.config.bucket" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_os_prefix')" class="flex-1">
              <Input v-model:value="formState.config.prefix" placeholder="docs/" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_os_accessKey')" class="flex-1">
              <Input v-model:value="formState.config.accessKey" autocomplete="off" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_os_secretKey')" class="flex-1">
              <Input.Password v-model:value="formState.config.secretKey" autocomplete="new-password" :placeholder="isEdit ? $t('kb.ds_secretKeep') : ''" />
            </Form.Item>
          </div>
          <Form.Item :label="$t('kb.ds_os_fileTypes')">
            <Select v-model:value="formState.config.fileTypes" mode="tags" :open="false" :placeholder="$t('kb.ds_os_fileTypesPh')" />
          </Form.Item>
        </template>

        <!-- external_kb：外部知识库 -->
        <template v-else-if="formState.type === 'external_kb'">
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_ek_provider')" class="w-40">
              <Select
                v-model:value="formState.config.provider"
                :options="[
                  { label: '飞书', value: 'feishu' },
                  { label: '钉钉', value: 'dingtalk' },
                  { label: 'Notion', value: 'notion' },
                  { label: 'Confluence', value: 'confluence' },
                  { label: $t('kb.ds_ek_custom'), value: 'custom' },
                ]"
              />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_ek_baseUrl')" required class="flex-1">
              <Input v-model:value="formState.config.baseUrl" placeholder="https://…" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_ek_appId')" class="flex-1">
              <Input v-model:value="formState.config.appId" autocomplete="off" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_ek_appSecret')" class="flex-1">
              <Input.Password v-model:value="formState.config.appSecret" autocomplete="new-password" :placeholder="isEdit ? $t('kb.ds_secretKeep') : ''" />
            </Form.Item>
          </div>
          <div class="flex gap-3">
            <Form.Item :label="$t('kb.ds_ek_token')" class="flex-1">
              <Input.Password v-model:value="formState.config.accessToken" autocomplete="new-password" :placeholder="isEdit ? $t('kb.ds_secretKeep') : ''" />
            </Form.Item>
            <Form.Item :label="$t('kb.ds_ek_spaceId')" class="flex-1">
              <Input v-model:value="formState.config.spaceId" />
            </Form.Item>
          </div>
          <Form.Item :label="$t('kb.ds_ek_docTypes')">
            <Select v-model:value="formState.config.docTypes" mode="tags" :open="false" :placeholder="$t('kb.ds_ek_docTypesPh')" />
          </Form.Item>
        </template>
      </Form>
    </Modal>

    <!-- 运行历史抽屉 -->
    <Drawer
      v-model:open="runsVisible"
      :title="`${$t('kb.ds_runsTitle')}${runsDatasource ? ' · ' + runsDatasource.name : ''}`"
      :width="900"
      :footer="null"
      :destroy-on-close="true"
    >
      <Card v-if="runsDatasource" size="small" class="mb-3">
        <Descriptions size="small" :column="3">
          <DescriptionsItem :label="$t('kb.ds_type')">
            {{ typeTag(runsDatasource.type).text }}
          </DescriptionsItem>
          <DescriptionsItem :label="$t('kb.ds_cron')">
            <code v-if="runsDatasource.cron" class="text-xs">{{ runsDatasource.cron }}</code>
            <span v-else class="text-xs opacity-50">{{ $t('kb.ds_cronManual') }}</span>
          </DescriptionsItem>
          <DescriptionsItem :label="$t('kb.ds_nextRunAt')">
            {{ fmtTime(runsDatasource.nextRunAt) }}
          </DescriptionsItem>
        </Descriptions>
      </Card>
      <div class="mb-2 flex items-center gap-2">
        <Button size="small" type="primary" ghost @click="loadRuns">
          {{ $t('kb.ds_runsRefresh') }}
        </Button>
        <Tooltip :title="$t('kb.ds_runsRefreshTip')">
          <span class="text-xs opacity-50">{{ $t('kb.ds_runsRefreshTip') }}</span>
        </Tooltip>
      </div>
      <Table
        :columns="runColumns"
        :data-source="runs"
        :loading="runsLoading"
        :pagination="runsPagination"
        row-key="runId"
        size="small"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'status'">
            <span :class="['aa-enum', `aa-enum--${runTag(record.status).color}`]">
              {{ runTag(record.status).text }}
            </span>
          </template>
          <template v-else-if="column.dataIndex === 'trigger'">
            <Tag v-if="record.trigger === 'manual'" color="blue">{{ $t('kb.ds_trigger_manual') }}</Tag>
            <Tag v-else-if="record.trigger === 'cron'" color="purple">{{ $t('kb.ds_trigger_cron') }}</Tag>
            <Tag v-else color="default">{{ $t('kb.ds_trigger_api') }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'startedAt'">
            {{ fmtTime(record.startedAt) }}
          </template>
          <template v-else-if="column.dataIndex === 'durationMs'">
            {{ fmtDuration(record.durationMs) }}
          </template>
          <template v-else-if="column.dataIndex === 'stats'">
            <span class="text-xs">{{ statsText(record.stats) }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'error'">
            <span v-if="record.error" class="text-xs text-red-500 break-all">{{ record.error }}</span>
            <span v-else class="text-xs opacity-40">-</span>
          </template>
          <template v-else-if="column.key === 'op'">
            <Button
              v-if="record.status === 'pending' || record.status === 'running'"
              size="small"
              danger
              @click="onCancelRun(record as KbDatasourceApi.RunItem)"
            >
              {{ $t('kb.ds_runCancel') }}
            </Button>
          </template>
        </template>
      </Table>
    </Drawer>
  </Page>
</template>
