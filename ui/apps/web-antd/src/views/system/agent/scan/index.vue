<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addAgentScanJobApi,
  deleteAgentScanJobApi,
  editAgentScanJobApi,
  getAgentScanJobListApi,
} from '#/api/system/agent/scan';

defineOptions({ name: 'AgentScan' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'assetType',
    label: $t('agent.scan_assetType'),
    componentProps: { placeholder: $t('agent.scan_assetTypePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'status',
    label: $t('agent.scan_status'),
    componentProps: { placeholder: $t('agent.scan_statusPlaceholder') },
  },
]);

// ===== 状态映射 =====
const statusOptions = [
  { label: 'pending', value: 0 },
  { label: 'running', value: 1 },
  { label: 'done', value: 2 },
  { label: 'failed', value: 3 },
];
function statusText(v: number) {
  return statusOptions.find((s) => s.value === v)?.label ?? 'unknown';
}
function statusColor(v: number): string {
  return ['default', 'processing', 'magenta', 'error'][v] ?? 'default';
}

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.scan_seq'), width: 60 },
    { field: 'assetType', title: $t('agent.scan_assetType'), width: 120 },
    { field: 'scanMode', title: $t('agent.scan_scanMode'), width: 100 },
    {
      field: 'status',
      title: $t('agent.scan_status'),
      width: 100,
      slots: { default: 'status' },
    },
    {
      field: 'progress',
      title: $t('agent.scan_progress'),
      width: 180,
      slots: { default: 'progress' },
    },
    {
      field: 'currentItemName',
      title: $t('agent.scan_currentItem'),
      minWidth: 160,
      showOverflow: true,
    },
    { field: 'createdAt', title: $t('agent.scan_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('agent.scan_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentScanJobListApi({
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
  isEdit.value ? $t('agent.scan_modalTitleEdit') : $t('agent.scan_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'assetType',
    label: $t('agent.scan_assetType'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.scan_assetTypePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'scanMode',
    label: $t('agent.scan_scanMode'),
    componentProps: { placeholder: $t('agent.scan_scanModePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'status',
    label: $t('agent.scan_status'),
    componentProps: {
      options: statusOptions,
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'scopeJson',
    label: $t('agent.scan_scopeJson'),
    componentProps: { rows: 2, placeholder: $t('agent.scan_scopeJsonPlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'totalItems',
    label: $t('agent.scan_totalItems'),
    componentProps: { min: 0, style: { width: '100%' } },
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
  }) as any,
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ status: 0, totalItems: 0 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.scanJobId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    assetType: row.assetType,
    scanMode: row.scanMode,
    status: row.status,
    scopeJson: row.scopeJson,
    totalItems: row.totalItems,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editAgentScanJobApi({ ...values, scanJobId: editingId.value });
      message.success($t('agent.scan_editSuccess'));
    } else {
      await addAgentScanJobApi(values);
      message.success($t('agent.scan_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('agent.scan_selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.scanJobId), rows.length);
}
async function onDeleteRow(row: any) {
  await doDelete([row.scanJobId], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('agent.scan_confirmDeleteTitle'),
    content: $t('agent.scan_confirmDeleteContent', { count }),
    okText: $t('agent.scan_confirm'),
    cancelText: $t('agent.scan_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentScanJobApi(ids);
      message.success($t('agent.scan_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.scan_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.scan_batchDelete') }}
        </Button>
      </template>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${statusColor(row.status)}`]">{{ statusText(row.status) }}</span>
      </template>
      <template #progress="{ row }">
        <span>{{ row.completedItems }}/{{ row.totalItems }}</span>
        <span v-if="row.failedItems" style="color: #ff4d4f">
          {{ $t('agent.scan_failedItems', { n: row.failedItems }) }}
        </span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('agent.scan_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.scan_delete') }}
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
  </Page>
</template>
