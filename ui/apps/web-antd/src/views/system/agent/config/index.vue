<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addAgentResourceApi,
  deleteAgentResourceApi,
  editAgentResourceApi,
  getAgentResourceApi,
  getAgentResourceListApi,
} from '#/api/system/agent/config';

defineOptions({ name: 'AgentConfigResource' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('agent.config_name'),
    componentProps: { placeholder: $t('agent.config_namePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'resourceType',
    label: $t('agent.config_resourceType'),
    componentProps: { placeholder: $t('agent.config_resourceTypePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.config_seq'), width: 60 },
    { field: 'name', title: $t('agent.config_name'), minWidth: 140 },
    { field: 'resourceType', title: $t('agent.config_resourceType'), width: 130 },
    { field: 'resourceKey', title: $t('agent.config_resourceKey'), minWidth: 160 },
    { field: 'version', title: $t('agent.config_version'), width: 80 },
    {
      field: 'enabled',
      title: $t('agent.config_enabled'),
      width: 90,
      slots: { default: 'enabled' },
    },
    { field: 'createdAt', title: $t('agent.config_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('agent.config_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentResourceListApi({
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
  isEdit.value ? $t('agent.config_modalTitleEdit') : $t('agent.config_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('agent.config_name'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.config_namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'resourceType',
    label: $t('agent.config_resourceType'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.config_resourceTypePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'resourceKey',
    label: $t('agent.config_resourceKey'),
    componentProps: { placeholder: $t('agent.config_resourceKeyPlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('agent.config_description'),
    componentProps: { placeholder: $t('agent.config_descriptionPlaceholder'), rows: 2 },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'contentJson',
    label: $t('agent.config_contentJson'),
    rules: z
      .string()
      .optional()
      .refine(
        (v) => {
          if (!v) return true;
          try {
            JSON.parse(v);
            return true;
          } catch {
            return false;
          }
        },
        $t('agent.config_contentJsonInvalid'),
      ),
    componentProps: { placeholder: $t('agent.config_contentJsonPlaceholder'), rows: 4 },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'version',
    label: $t('agent.config_versionLabel'),
    rules: 'required',
    componentProps: { min: 1, style: { width: '100%' } },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('agent.config_enabled'),
    componentProps: {
      options: [
        { label: $t('agent.config_enabled'), value: 1 },
        { label: $t('agent.config_disabled'), value: 0 },
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
  formApi.setValues({ version: 1, enabled: 1, contentJson: '{}' });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.configResourceId;
  try {
    const res: any = await getAgentResourceApi(row.configResourceId);
    const v = res ?? row;
    modalVisible.value = true;
    await nextTick();
    await formApi.resetForm();
    formApi.setValues({
      name: v.name,
      resourceType: v.resourceType,
      resourceKey: v.resourceKey,
      description: v.description,
      contentJson: v.contentJson ?? '{}',
      version: v.version,
      enabled: v.enabled,
    });
  } catch {
    message.error($t('agent.config_loadFailed'));
  }
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editAgentResourceApi({
        ...values,
        configResourceId: editingId.value,
      });
      message.success($t('agent.config_editSuccess'));
    } else {
      await addAgentResourceApi(values);
      message.success($t('agent.config_addSuccess'));
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
    message.warning($t('agent.config_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.configResourceId),
    rows.length,
  );
}
async function onDeleteRow(row: any) {
  await doDelete([row.configResourceId], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('agent.config_confirmDeleteTitle'),
    content: $t('agent.config_confirmDeleteContent', { count }),
    okText: $t('agent.config_confirm'),
    cancelText: $t('agent.config_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentResourceApi(ids);
      message.success($t('agent.config_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 启用切换（行内仅展示，写库需 edit 端点）=====
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.enabled;
  row.enabled = newEnabled;
  try {
    await editAgentResourceApi({
      configResourceId: row.configResourceId,
      resourceType: row.resourceType,
      resourceKey: row.resourceKey,
      name: row.name,
      description: row.description,
      enabled: newEnabled,
      version: row.version,
      tagsJson: row.tagsJson,
      contentJson: row.contentJson,
    });
    message.success(
      $t('agent.config_toggleSuccess', {
        action:
          newEnabled === 1
            ? $t('agent.config_enabled')
            : $t('agent.config_disabled'),
      }),
    );
  } catch {
    row.enabled = prev;
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.config_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.config_batchDelete') }}
        </Button>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('agent.config_enabled')"
          :un-checked-children="$t('agent.config_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('agent.config_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.config_delete') }}
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
