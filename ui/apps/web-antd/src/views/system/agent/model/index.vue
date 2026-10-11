<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch, Tag } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addAgentModelApi,
  deleteAgentModelApi,
  editAgentModelApi,
  getAgentModelListApi,
} from '#/api/system/agent/model';

defineOptions({ name: 'AgentModel' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('agent.model_name'),
    componentProps: { placeholder: $t('agent.model_namePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'providerType',
    label: $t('agent.model_providerType'),
    componentProps: { placeholder: $t('agent.model_providerTypePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.model_seq'), width: 60 },
    { field: 'displayName', title: $t('agent.model_name'), minWidth: 140 },
    { field: 'providerType', title: $t('agent.model_providerType'), width: 120 },
    { field: 'providerModelName', title: $t('agent.model_providerModelName'), minWidth: 160 },
    { field: 'inputPrice', title: $t('agent.model_inputPrice'), width: 90 },
    { field: 'outputPrice', title: $t('agent.model_outputPrice'), width: 90 },
    { field: 'currency', title: $t('agent.model_currency'), width: 80 },
    {
      field: 'apiKey',
      title: $t('agent.model_apiKey'),
      width: 100,
      slots: { default: 'apiKey' },
    },
    {
      field: 'isActive',
      title: $t('agent.model_status'),
      width: 90,
      slots: { default: 'status' },
    },
    {
      field: 'action',
      title: $t('agent.model_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentModelListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin: { list, total, currentPage }
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
  isEdit.value ? $t('agent.model_modalTitleEdit') : $t('agent.model_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'displayName',
    label: $t('agent.model_name'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.model_namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'providerType',
    label: $t('agent.model_providerType'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.model_providerTypePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'providerModelName',
    label: $t('agent.model_modelName'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.model_modelNamePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'protocolType',
    label: $t('agent.model_protocolType'),
    componentProps: { placeholder: $t('agent.model_protocolTypePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'baseUrl',
    label: $t('agent.model_baseUrl'),
    componentProps: { placeholder: $t('agent.model_baseUrlPlaceholder') },
  },
  {
    component: 'InputPassword' as const,
    fieldName: 'apiKey',
    label: $t('agent.model_apiKeyLabel'),
    componentProps: () => ({
      placeholder: isEdit.value
        ? $t('agent.model_apiKeyPlaceholderEdit')
        : $t('agent.model_apiKeyPlaceholderAdd'),
      autocomplete: 'new-password',
    }),
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'inputPrice',
    label: $t('agent.model_inputPrice'),
    componentProps: { min: 0, step: 0.01, style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'outputPrice',
    label: $t('agent.model_outputPrice'),
    componentProps: { min: 0, step: 0.01, style: { width: '100%' } },
  },
  {
    component: 'Input' as const,
    fieldName: 'currency',
    label: $t('agent.model_currency'),
    componentProps: { placeholder: $t('agent.model_currencyPlaceholder') },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isActive',
    label: $t('agent.model_enabled'),
    componentProps: {
      options: [
        { label: $t('agent.model_enabled'), value: 1 },
        { label: $t('agent.model_disabled'), value: 0 },
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
  }) as any,
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    isActive: 1,
    currency: 'USD',
    inputPrice: 0,
    outputPrice: 0,
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.agentModelId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  // apiKey 落库加密且不外露，编辑回显置空
  formApi.setValues({
    displayName: row.displayName,
    description: row.description,
    providerType: row.providerType,
    protocolType: row.protocolType,
    baseUrl: row.baseUrl,
    providerModelName: row.providerModelName,
    apiKeySecretRef: row.apiKeySecretRef,
    isSecure: row.isSecure,
    isActive: row.isActive,
    inputPrice: row.inputPrice,
    outputPrice: row.outputPrice,
    currency: row.currency,
    apiKey: '',
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editAgentModelApi({ ...values, agentModelId: editingId.value });
      message.success($t('agent.model_editSuccess'));
    } else {
      await addAgentModelApi(values);
      message.success($t('agent.model_addSuccess'));
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
    message.warning($t('agent.model_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.agentModelId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.agentModelId], 1);
}

async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('agent.model_confirmDeleteTitle'),
    content: $t('agent.model_confirmDeleteContent', { count }),
    okText: $t('agent.model_confirm'),
    cancelText: $t('agent.model_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentModelApi(ids);
      message.success($t('agent.model_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.model_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.model_batchDelete') }}
        </Button>
      </template>
      <template #apiKey>
        <Tag color="default">{{ $t('agent.model_encrypted') }}</Tag>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.isActive === 1"
          :checked-children="$t('agent.model_enabled')"
          :un-checked-children="$t('agent.model_disabled')"
          disabled
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('agent.model_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.model_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>
  </Page>
</template>
