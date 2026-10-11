<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Drawer, Input, message, Modal, Switch, Table, Tag } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addMcpOauthClientApi,
  deleteMcpOauthClientApi,
  editMcpOauthClientApi,
  getMcpOauthClientListApi,
  getMcpOauthTokenListApi,
  revokeMcpOauthTokenApi,
  rotateMcpOauthClientSecretApi,
} from '#/api/system/mcp/oauth';
import type { McpOauthApi } from '#/api/system/mcp/oauth';

defineOptions({ name: 'SystemMcpOauth' });

const GRANT_OPTIONS = computed(() => [
  { label: 'authorization_code', value: 'authorization_code' },
  { label: 'client_credentials', value: 'client_credentials' },
  { label: 'refresh_token', value: 'refresh_token' },
]);

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyword',
    label: $t('mcp.oauth_keyword'),
    componentProps: { placeholder: $t('mcp.oauth_keywordPlaceholder') },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.oauth_seq'), width: 60 },
    { field: 'clientName', title: $t('mcp.oauth_name'), minWidth: 140 },
    { field: 'clientId', title: 'Client ID', minWidth: 200, slots: { default: 'clientId' } },
    { field: 'clientType', title: $t('mcp.oauth_type'), width: 110, slots: { default: 'clientType' } },
    { field: 'grantTypes', title: $t('mcp.oauth_grants'), minWidth: 180, slots: { default: 'grants' } },
    { field: 'scopes', title: $t('mcp.oauth_scopes'), minWidth: 140, slots: { default: 'scopes' } },
    { field: 'tokenCount', title: $t('mcp.oauth_tokenCount'), width: 80 },
    { field: 'enabled', title: $t('mcp.oauth_status'), width: 90, slots: { default: 'enabled' } },
    { field: 'createdAt', title: $t('mcp.oauth_createdAt'), width: 160, formatter: ({ cellValue }: any) => fmtTime(cellValue) },
    { field: 'action', title: $t('mcp.oauth_action'), width: 280, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getMcpOauthClientListApi({
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

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value
    ? $t('mcp.oauth_modalTitleEdit')
    : $t('mcp.oauth_modalTitleAdd'),
);

/** 文本域行集合 ⇄ 数组互转（redirectUris / scopes 每行一项） */
function linesToArr(s?: string): string[] {
  return (s ?? '')
    .split(/[\n,]/)
    .map((x) => x.trim())
    .filter(Boolean);
}
function arrToLines(a?: string[]): string {
  return (a ?? []).join('\n');
}

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'clientName',
    label: $t('mcp.oauth_name'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.oauth_namePlaceholder') },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'clientType',
    label: $t('mcp.oauth_type'),
    componentProps: {
      options: [
        { label: $t('mcp.oauth_typeConfidential'), value: 'confidential' },
        { label: $t('mcp.oauth_typePublic'), value: 'public' },
      ],
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'grantTypes',
    label: $t('mcp.oauth_grants'),
    rules: 'selectRequired',
    componentProps: {
      mode: 'multiple',
      options: GRANT_OPTIONS.value,
      placeholder: $t('mcp.oauth_grantsPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'scopes',
    label: $t('mcp.oauth_scopes'),
    componentProps: {
      mode: 'tags',
      open: false,
      tokenSeparators: [',', ' '],
      placeholder: $t('mcp.oauth_scopesPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'redirectUrisText',
    label: $t('mcp.oauth_redirects'),
    dependencies: {
      triggerFields: ['grantTypes'],
      show: (values: any) => (values.grantTypes ?? []).includes('authorization_code'),
    },
    componentProps: {
      rows: 3,
      placeholder: $t('mcp.oauth_redirectsPlaceholder'),
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('mcp.oauth_status'),
    componentProps: {
      options: [
        { label: $t('mcp.oauth_enabled'), value: 1 },
        { label: $t('mcp.oauth_disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    clientType: 'confidential',
    grantTypes: ['authorization_code'],
    scopes: [],
    redirectUrisText: '',
    enabled: 1,
  });
}

async function openEdit(row: McpOauthApi.ClientItem) {
  isEdit.value = true;
  editingId.value = row.clientId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    clientName: row.clientName ?? '',
    clientType: row.clientType ?? 'confidential',
    grantTypes: row.grantTypes ?? [],
    scopes: row.scopes ?? [],
    redirectUrisText: arrToLines(row.redirectUris),
    enabled: row.enabled ?? 1,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const grantTypes: string[] = values.grantTypes ?? [];
  const redirectUris = linesToArr(values.redirectUrisText);
  if (grantTypes.includes('authorization_code') && redirectUris.length === 0) {
    message.warning($t('mcp.oauth_redirectsRequired'));
    return;
  }
  const payload: McpOauthApi.ClientReq = {
    clientName: (values.clientName ?? '').trim(),
    clientType: values.clientType ?? 'confidential',
    grantTypes,
    scopes: values.scopes ?? [],
    redirectUris,
    enabled: values.enabled ?? 1,
  };
  try {
    if (isEdit.value) {
      await editMcpOauthClientApi({ ...payload, clientId: editingId.value });
      message.success($t('mcp.oauth_editSuccess'));
      modalVisible.value = false;
      gridApi.query();
    } else {
      const res: any = await addMcpOauthClientApi(payload);
      message.success($t('mcp.oauth_addSuccess'));
      modalVisible.value = false;
      gridApi.query();
      // secret 仅返回一次；public 客户端无 secret（PKCE）
      if (res?.clientSecret) {
        plainSecret.value = res.clientSecret;
        secretClientId.value = res.clientId ?? '';
        secretVisible.value = true;
      } else if (res?.clientId) {
        plainSecret.value = '';
        secretClientId.value = res.clientId;
        secretVisible.value = true;
      }
    }
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内启停：edit 整行（镜像 openEdit 字段），失败回滚
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.enabled;
  row.enabled = newEnabled;
  try {
    await editMcpOauthClientApi({
      clientId: row.clientId,
      clientName: row.clientName ?? '',
      clientType: row.clientType ?? 'confidential',
      grantTypes: row.grantTypes ?? [],
      scopes: row.scopes ?? [],
      redirectUris: row.redirectUris ?? [],
      enabled: newEnabled,
    });
    message.success($t('mcp.oauth_editSuccess'));
  } catch {
    row.enabled = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('mcp.oauth_selectToDelete'));
    return;
  }
  doDelete(rows.map((r: any) => r.clientId), rows.length);
}
async function onDeleteRow(row: any) {
  doDelete([row.clientId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.oauth_confirmDeleteTitle'),
    content: $t('mcp.oauth_confirmDeleteContent', { count }),
    okText: $t('mcp.oauth_confirm'),
    cancelText: $t('mcp.oauth_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpOauthClientApi(ids);
      message.success($t('mcp.oauth_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 轮换密钥（新 secret 仅显示一次） =====
function onRotateSecret(row: any) {
  Modal.confirm({
    title: $t('mcp.oauth_confirmRotateTitle'),
    content: $t('mcp.oauth_confirmRotateContent', { name: row.clientName }),
    okText: $t('mcp.oauth_confirm'),
    cancelText: $t('mcp.oauth_cancel'),
    okType: 'danger',
    onOk: async () => {
      const res: any = await rotateMcpOauthClientSecretApi(row.clientId);
      message.success($t('mcp.oauth_rotated'));
      gridApi.query();
      if (res?.clientSecret) {
        plainSecret.value = res.clientSecret;
        secretClientId.value = res.clientId ?? row.clientId;
        secretVisible.value = true;
      }
    },
  });
}

// ===== 一次性 secret 展示弹窗 =====
const secretVisible = ref(false);
const plainSecret = ref('');
const secretClientId = ref('');

function copySecret() {
  navigator.clipboard?.writeText(plainSecret.value);
  message.success($t('mcp.oauth_copied'));
}

// ===== 令牌抽屉（某客户端的有效/历史令牌，可吊销） =====
const tokensVisible = ref(false);
const tokensLoading = ref(false);
const tokenClient = ref<McpOauthApi.ClientItem | null>(null);
const tokens = ref<McpOauthApi.TokenItem[]>([]);
const tokensTotal = ref(0);
const tokenPage = reactive({ pageNum: 1, pageSize: 10 });

async function openTokens(row: McpOauthApi.ClientItem) {
  tokenClient.value = row;
  tokenPage.pageNum = 1;
  tokensVisible.value = true;
  await loadTokens();
}

async function loadTokens() {
  if (!tokenClient.value) return;
  tokensLoading.value = true;
  try {
    const res = await getMcpOauthTokenListApi({
      clientId: tokenClient.value.clientId,
      pageNum: tokenPage.pageNum,
      pageSize: tokenPage.pageSize,
    });
    tokens.value = res?.list ?? [];
    tokensTotal.value = res?.total ?? 0;
  } catch {
    tokens.value = [];
    tokensTotal.value = 0;
  } finally {
    tokensLoading.value = false;
  }
}

const tokensPagination = computed(() => ({
  current: tokenPage.pageNum,
  onChange: (p: number, s: number) => {
    tokenPage.pageNum = p;
    tokenPage.pageSize = s;
    loadTokens();
  },
  pageSize: tokenPage.pageSize,
  showSizeChanger: false,
  total: tokensTotal.value,
}));

const tokenColumns = computed(() => [
  { dataIndex: 'subject', title: () => $t('mcp.oauth_tkSubject'), width: 140 },
  { dataIndex: 'scope', title: () => $t('mcp.oauth_scopes'), minWidth: 140 },
  { dataIndex: 'accessTokenPreview', title: () => $t('mcp.oauth_tkPreview'), width: 130 },
  { dataIndex: 'issuedAt', title: () => $t('mcp.oauth_tkIssuedAt'), width: 160 },
  { dataIndex: 'expiresAt', title: () => $t('mcp.oauth_tkExpiresAt'), width: 160 },
  { dataIndex: 'lastUsedAt', title: () => $t('mcp.oauth_tkLastUsedAt'), width: 160 },
  { dataIndex: 'status', title: () => $t('mcp.oauth_tkStatus'), width: 90 },
  { key: 'op', title: () => $t('mcp.oauth_action'), width: 90 },
]);

function onRevokeToken(row: McpOauthApi.TokenItem) {
  Modal.confirm({
    title: $t('mcp.oauth_tkRevokeTitle'),
    content: $t('mcp.oauth_tkRevokeContent'),
    okText: $t('mcp.oauth_confirm'),
    cancelText: $t('mcp.oauth_cancel'),
    okType: 'danger',
    onOk: async () => {
      await revokeMcpOauthTokenApi(row.tokenId);
      message.success($t('mcp.oauth_tkRevoked'));
      loadTokens();
    },
  });
}

// ===== 展示 =====
function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('mcp.oauth_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.oauth_batchDelete') }}
        </Button>
      </template>
      <template #clientId="{ row }">
        <span class="font-mono text-xs" :title="row.clientId">
          {{ row.clientId }}
        </span>
      </template>
      <template #clientType="{ row }">
        <Tag :color="row.clientType === 'confidential' ? 'geekblue' : 'default'">
          {{ row.clientType === 'confidential' ? $t('mcp.oauth_typeConfidential') : $t('mcp.oauth_typePublic') }}
        </Tag>
      </template>
      <template #grants="{ row }">
        <Tag v-for="g in row.grantTypes ?? []" :key="g" class="!mr-1 font-mono">
          {{ g }}
        </Tag>
        <span v-if="!row.grantTypes?.length" class="text-xs opacity-50">-</span>
      </template>
      <template #scopes="{ row }">
        <span class="font-mono text-xs">{{ (row.scopes ?? []).join(' ') || '-' }}</span>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('mcp.oauth_enabled')"
          :un-checked-children="$t('mcp.oauth_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row as McpOauthApi.ClientItem)">
          {{ $t('mcp.oauth_edit') }}
        </Button>
        <Button type="primary" ghost size="small" @click="openTokens(row as McpOauthApi.ClientItem)">
          {{ $t('mcp.oauth_tokens') }}
        </Button>
        <Button
          v-if="row.clientType === 'confidential'"
          type="primary"
          ghost
          size="small"
          @click="onRotateSecret(row)"
        >
          {{ $t('mcp.oauth_rotate') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('mcp.oauth_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="600"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <!-- client_secret（仅显示一次） -->
    <Modal
      v-model:open="secretVisible"
      :title="$t('mcp.oauth_secretModalTitle')"
      :width="560"
      :ok-text="$t('mcp.oauth_secretModalClose')"
    >
      <div class="mb-2 text-amber-500">{{ $t('mcp.oauth_secretWarning') }}</div>
      <div class="mb-2 text-xs opacity-60">
        {{ $t('mcp.oauth_secretClient') }}: <span class="font-mono">{{ secretClientId }}</span>
      </div>
      <Input v-if="plainSecret" :value="plainSecret" readonly>
        <template #addonAfter>
          <Button size="small" @click="copySecret">
            {{ $t('mcp.oauth_copy') }}
          </Button>
        </template>
      </Input>
      <div v-else class="text-sm">
        {{ $t('mcp.oauth_publicNoSecret') }}
      </div>
    </Modal>

    <!-- 令牌列表抽屉 -->
    <Drawer
      v-model:open="tokensVisible"
      :title="`${$t('mcp.oauth_tokensTitle')}${tokenClient ? ' · ' + tokenClient.clientName : ''}`"
      :width="960"
      :footer="null"
      :destroy-on-close="true"
    >
      <Table
        :columns="tokenColumns"
        :data-source="tokens"
        :loading="tokensLoading"
        :pagination="tokensPagination"
        row-key="tokenId"
        size="small"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'accessTokenPreview'">
            <span class="font-mono text-xs">{{ record.accessTokenPreview || '-' }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'scope'">
            <span class="font-mono text-xs">{{ record.scope || '-' }}</span>
          </template>
          <template v-else-if="column.dataIndex === 'status'">
            <Tag v-if="record.revoked === 1" color="error">{{ $t('mcp.oauth_tkRevoked') }}</Tag>
            <Tag v-else color="success">{{ $t('mcp.oauth_tkActive') }}</Tag>
          </template>
          <template v-else-if="column.key === 'op'">
            <Button
              v-if="record.revoked !== 1"
              size="small"
              danger
              @click="onRevokeToken(record as McpOauthApi.TokenItem)"
            >
              {{ $t('mcp.oauth_tkRevoke') }}
            </Button>
          </template>
        </template>
      </Table>
    </Drawer>
  </Page>
</template>
