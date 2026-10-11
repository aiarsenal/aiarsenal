<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addMcpKeyApi,
  deleteMcpKeyApi,
  editMcpKeyApi,
  getMcpKeyListApi,
  rotateMcpKeyApi,
} from '#/api/system/mcp/key';
import ConnectGuide from '../components/connect-guide.vue';

defineOptions({ name: 'SystemMcpKey' });

const KIND_OPTIONS = [
  { label: 'user', value: 'user' },
  { label: 'system', value: 'system' },
];
const ACCESS_TYPE_OPTIONS = computed(() => [
  { label: $t('mcp.key_accessTypeAll'), value: 'all' },
  { label: $t('mcp.key_accessTypeGroups'), value: 'groups' },
  { label: $t('mcp.key_accessTypeServers'), value: 'servers' },
  { label: $t('mcp.key_accessTypeCustom'), value: 'custom' },
]);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('mcp.key_name'),
    componentProps: { placeholder: $t('mcp.key_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.key_seq'), width: 60 },
    { field: 'name', title: $t('mcp.key_name'), minWidth: 140 },
    { field: 'kind', title: $t('mcp.key_kind'), width: 90 },
    { field: 'accessType', title: $t('mcp.key_accessType'), width: 110 },
    {
      field: 'enabled',
      title: $t('mcp.key_status'),
      width: 90,
      slots: { default: 'enabled' },
    },
    { field: 'createdAt', title: $t('mcp.key_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('mcp.key_action'),
      width: 280,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        // gin: dto.McpKeySearchRes → { list, total, currentPage }
        const res = await getMcpKeyListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: {
    custom: true,
    refresh: { code: 'query' },
  },
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
  isEdit.value ? $t('mcp.key_modalTitleEdit') : $t('mcp.key_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('mcp.key_name'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.key_namePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'kind',
    label: $t('mcp.key_kind'),
    componentProps: {
      options: KIND_OPTIONS,
      placeholder: $t('mcp.key_kindPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'accessType',
    label: $t('mcp.key_accessType'),
    componentProps: {
      options: ACCESS_TYPE_OPTIONS.value,
      placeholder: $t('mcp.key_accessTypePlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'allowedGroups',
    label: $t('mcp.key_allowedGroups'),
    dependencies: {
      triggerFields: ['accessType'],
      show: (values: any) =>
        values.accessType === 'groups' || values.accessType === 'custom',
    },
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.key_allowedGroupsPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'allowedServers',
    label: $t('mcp.key_allowedServers'),
    dependencies: {
      triggerFields: ['accessType'],
      show: (values: any) =>
        values.accessType === 'servers' || values.accessType === 'custom',
    },
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.key_allowedServersPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('mcp.key_status'),
    componentProps: {
      options: [
        { label: $t('mcp.key_enabled'), value: 1 },
        { label: $t('mcp.key_disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
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
    kind: 'user',
    accessType: 'all',
    allowedGroups: '[]',
    allowedServers: '[]',
    enabled: 1,
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.keyId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    name: row.name ?? '',
    kind: row.kind ?? 'user',
    accessType: row.accessType ?? 'all',
    allowedGroups: row.allowedGroups ?? '[]',
    allowedServers: row.allowedServers ?? '[]',
    enabled: row.enabled ?? 1,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin edit: dto.McpKeyEditReq = { keyId, ...McpKeyReq }
      await editMcpKeyApi({ ...values, keyId: editingId.value });
      message.success($t('mcp.key_editSuccess'));
      modalVisible.value = false;
      gridApi.query();
    } else {
      // gin add: dto.McpKeyCreateRes → { keyId, token }
      const res: any = await addMcpKeyApi(values);
      message.success($t('mcp.key_addSuccess'));
      modalVisible.value = false;
      gridApi.query();
      if (res?.token) {
        plainToken.value = res.token;
        tokenVisible.value = true;
      }
    }
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内启用切换：edit 整行（镜像 openEdit 字段），失败回滚。
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.enabled;
  row.enabled = newEnabled;
  try {
    await editMcpKeyApi({
      name: row.name ?? '',
      kind: row.kind ?? 'user',
      accessType: row.accessType ?? 'all',
      allowedGroups: row.allowedGroups ?? '[]',
      allowedServers: row.allowedServers ?? '[]',
      enabled: newEnabled,
      keyId: row.keyId,
    });
    message.success($t('mcp.key_editSuccess'));
  } catch {
    row.enabled = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('mcp.key_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.keyId),
    rows.length,
  );
}
async function onDeleteRow(row: any) {
  await doDelete([row.keyId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.key_confirmDeleteTitle'),
    content: $t('mcp.key_confirmDeleteContent', { count }),
    okText: $t('mcp.key_confirm'),
    cancelText: $t('mcp.key_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpKeyApi(ids);
      message.success($t('mcp.key_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 轮换（返回新明文，仅显示一次） =====
function onRotate(row: any) {
  Modal.confirm({
    title: $t('mcp.key_confirmRotateTitle'),
    content: $t('mcp.key_confirmRotateContent', { name: row.name }),
    okText: $t('mcp.key_confirm'),
    cancelText: $t('mcp.key_cancel'),
    okType: 'danger',
    onOk: async () => {
      const res: any = await rotateMcpKeyApi(row.keyId);
      message.success($t('mcp.key_rotated'));
      gridApi.query();
      if (res?.token) {
        plainToken.value = res.token;
        tokenVisible.value = true;
      }
    },
  });
}

// ===== 明文 token 展示弹窗（签发/轮换后仅显示一次） =====
const tokenVisible = ref(false);
const plainToken = ref('');

function copyToken() {
  navigator.clipboard?.writeText(plainToken.value);
  message.success($t('mcp.key_copied'));
}

// ===== 接入引导（key 无 scope → /mcp 全部聚合；签发/轮换后可带 token 预填） =====
const guideVisible = ref(false);
const guideInitialToken = ref('');

function onGuide() {
  guideInitialToken.value = '';
  guideVisible.value = true;
}
function onGuideWithToken() {
  guideInitialToken.value = plainToken.value;
  tokenVisible.value = false;
  guideVisible.value = true;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('mcp.key_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.key_addBatchDelete') }}
        </Button>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('mcp.key_enabled')"
          :un-checked-children="$t('mcp.key_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('mcp.key_edit') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onRotate(row)">
          {{ $t('mcp.key_rotate') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onGuide">
          {{ $t('mcp.connect_guide') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('mcp.key_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="560"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <Modal
      v-model:open="tokenVisible"
      :title="$t('mcp.key_tokenModalTitle')"
      :width="560"
      :ok-text="$t('mcp.key_tokenModalClose')"
    >
      <div style="color: #faad14; margin-bottom: 8px">
        {{ $t('mcp.key_tokenModalWarning') }}
      </div>
      <Input :value="plainToken" readonly>
        <template #addonAfter>
          <Button size="small" @click="copyToken">
            {{ $t('mcp.key_copy') }}
          </Button>
        </template>
      </Input>
      <Button size="small" class="mt-3" @click="onGuideWithToken">
        {{ $t('mcp.key_goConnect') }}
      </Button>
    </Modal>

    <!-- 客户端接入引导 -->
    <ConnectGuide
      v-model:open="guideVisible"
      :initial-token="guideInitialToken"
    />
  </Page>
</template>
