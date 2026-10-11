<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addMcpPromptApi,
  deleteMcpPromptApi,
  editMcpPromptApi,
  getMcpPromptListApi,
} from '#/api/system/mcp/prompt';

defineOptions({ name: 'SystemMcpPrompt' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('mcp.prompt_name'),
    componentProps: { placeholder: $t('mcp.prompt_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.prompt_seq'), width: 60 },
    { field: 'name', title: $t('mcp.prompt_name'), minWidth: 120 },
    { field: 'title', title: $t('mcp.prompt_title'), minWidth: 120 },
    { field: 'description', title: $t('mcp.prompt_description'), minWidth: 180 },
    {
      field: 'enabled',
      title: $t('mcp.prompt_status'),
      width: 90,
      slots: { default: 'enabled' },
    },
    { field: 'createdAt', title: $t('mcp.prompt_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('mcp.prompt_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getMcpPromptListApi({
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
    ? $t('mcp.prompt_modalTitleEdit')
    : $t('mcp.prompt_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('mcp.prompt_name'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.prompt_namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'title',
    label: $t('mcp.prompt_title'),
    componentProps: { placeholder: $t('mcp.prompt_titlePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('mcp.prompt_description'),
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.prompt_descriptionPlaceholder'),
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'template',
    label: $t('mcp.prompt_template'),
    rules: 'required',
    componentProps: { rows: 6, placeholder: $t('mcp.prompt_templatePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'arguments',
    label: $t('mcp.prompt_arguments'),
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.prompt_argumentsPlaceholder'),
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('mcp.prompt_status'),
    componentProps: {
      options: [
        { label: $t('mcp.prompt_enabled'), value: 1 },
        { label: $t('mcp.prompt_disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
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
  formApi.setValues({ enabled: 1, arguments: '[]' });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.promptId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    name: row.name,
    title: row.title,
    description: row.description,
    template: row.template,
    arguments: row.arguments ?? '[]',
    enabled: row.enabled,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editMcpPromptApi({ ...values, promptId: editingId.value });
      message.success($t('mcp.prompt_editSuccess'));
    } else {
      await addMcpPromptApi(values);
      message.success($t('mcp.prompt_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
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
    await editMcpPromptApi({
      name: row.name,
      title: row.title,
      description: row.description,
      template: row.template,
      arguments: row.arguments ?? '[]',
      enabled: newEnabled,
      promptId: row.promptId,
    });
    message.success($t('mcp.prompt_editSuccess'));
  } catch {
    row.enabled = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('mcp.prompt_selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.promptId), rows.length);
}

async function onDeleteRow(row: any) {
  await doDelete([row.promptId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.prompt_confirmDeleteTitle'),
    content: $t('mcp.prompt_confirmDeleteContent', { count }),
    okText: $t('mcp.prompt_confirm'),
    cancelText: $t('mcp.prompt_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpPromptApi(ids);
      message.success($t('mcp.prompt_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('mcp.prompt_add') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.prompt_addBatchDelete') }}
        </Button>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('mcp.prompt_enabled')"
          :un-checked-children="$t('mcp.prompt_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('mcp.prompt_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('mcp.prompt_delete') }}
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
