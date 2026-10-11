<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addMcpResourceApi,
  deleteMcpResourceApi,
  editMcpResourceApi,
  getMcpResourceListApi,
} from '#/api/system/mcp/resource';

defineOptions({ name: 'SystemMcpResource' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('mcp.resource_name'),
    componentProps: { placeholder: $t('mcp.resource_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.resource_seq'), width: 60 },
    { field: 'uri', title: $t('mcp.resource_uri'), minWidth: 200 },
    { field: 'name', title: $t('mcp.resource_name'), minWidth: 120 },
    { field: 'mimeType', title: $t('mcp.resource_mimeType'), width: 140 },
    {
      field: 'enabled',
      title: $t('mcp.resource_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'createdAt', title: $t('mcp.resource_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('mcp.resource_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getMcpResourceListApi({
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
  isEdit.value
    ? $t('mcp.resource_modalTitleEdit')
    : $t('mcp.resource_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'uri',
    label: $t('mcp.resource_uri'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.resource_uriPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('mcp.resource_name'),
    componentProps: { placeholder: $t('mcp.resource_namePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('mcp.resource_description'),
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.resource_descriptionPlaceholder'),
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'mimeType',
    label: $t('mcp.resource_mimeType'),
    componentProps: { placeholder: $t('mcp.resource_mimeTypePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'content',
    label: $t('mcp.resource_content'),
    rules: 'required',
    componentProps: { rows: 8, placeholder: $t('mcp.resource_contentPlaceholder') },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('mcp.resource_status'),
    componentProps: {
      options: [
        { label: $t('mcp.resource_enabled'), value: 1 },
        { label: $t('mcp.resource_disabled'), value: 0 },
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
  formApi.setValues({ enabled: 1, mimeType: 'text/plain' });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.resourceId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    uri: row.uri,
    name: row.name,
    description: row.description,
    mimeType: row.mimeType,
    content: row.content,
    enabled: row.enabled,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin edit 需要 resourceId（= 行 resourceId）
      await editMcpResourceApi({ ...values, resourceId: editingId.value });
      message.success($t('mcp.resource_editSuccess'));
    } else {
      await addMcpResourceApi(values);
      message.success($t('mcp.resource_addSuccess'));
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
    message.warning($t('mcp.resource_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.resourceId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.resourceId], 1);
}

async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.resource_confirmDeleteTitle'),
    content: $t('mcp.resource_confirmDeleteContent', { count }),
    okText: $t('mcp.resource_confirm'),
    cancelText: $t('mcp.resource_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpResourceApi(ids);
      message.success($t('mcp.resource_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 状态切换（行内仅展示，无独立 setStatus 端点，编辑走弹窗） =====
function onStatusToggle(row: any, checked: boolean) {
  // 资源无独立 setStatus 接口，提示走编辑
  void row;
  void checked;
  message.info($t('mcp.resource_statusToggleHint'));
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('mcp.resource_add') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.resource_addBatchDelete') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('mcp.resource_switchOn')"
          :un-checked-children="$t('mcp.resource_switchOff')"
          disabled
          @change="(v: any) => onStatusToggle(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('mcp.resource_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('mcp.resource_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="720"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>
  </Page>
</template>
