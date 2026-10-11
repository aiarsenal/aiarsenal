<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Switch, Tag } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addLLMCredentialApi,
  deleteLLMCredentialApi,
  discoverCredentialModelsApi,
  editLLMCredentialApi,
  getLLMCredentialListApi,
} from '#/api/system/llm/credential';

defineOptions({ name: 'SystemLlmCredential' });

const providerOptions = [
  'openai',
  'anthropic',
  'gemini',
  'deepseek',
  'moonshot',
  'groq',
  'ollama',
  'openrouter',
  'bedrock',
].map((v) => ({ value: v, label: v }));

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('llm.credential_name'),
    componentProps: { placeholder: $t('llm.credential_namePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'providerType',
    label: $t('llm.credential_provider'),
    componentProps: {
      showSearch: true,
      allowClear: true,
      options: providerOptions,
      placeholder: $t('llm.credential_providerPlaceholder'),
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('llm.credential_seq'), width: 60 },
    { field: 'credentialName', title: $t('llm.credential_name'), minWidth: 140 },
    { field: 'providerType', title: $t('llm.credential_provider'), width: 120 },
    { field: 'baseUrl', title: $t('llm.credential_baseUrl'), minWidth: 200 },
    {
      field: 'apiKey',
      title: $t('llm.credential_apiKey'),
      width: 110,
      slots: { default: 'apiKey' },
    },
    {
      field: 'status',
      title: $t('llm.credential_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'createdAt', title: $t('llm.credential_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('llm.credential_action'),
      width: 260,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMCredentialListApi({
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
const modalTitle = computed(
  () =>
    isEdit.value
      ? $t('llm.credential_modalTitleEdit')
      : $t('llm.credential_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'credentialName',
    label: $t('llm.credential_name'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.credential_namePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'providerType',
    label: $t('llm.credential_provider'),
    rules: 'required',
    componentProps: {
      showSearch: true,
      options: providerOptions,
      placeholder: $t('llm.credential_providerInputPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'baseUrl',
    label: $t('llm.credential_baseUrl'),
    componentProps: { placeholder: $t('llm.credential_baseUrlPlaceholder') },
  },
  {
    component: 'InputPassword' as const,
    fieldName: 'apiKey',
    label: $t('llm.credential_apiKey'),
    componentProps: () => ({
      placeholder: isEdit.value
        ? $t('llm.credential_apiKeyEditPlaceholder')
        : $t('llm.credential_apiKeyAddPlaceholder'),
      autocomplete: 'new-password',
    }),
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'status',
    label: $t('llm.credential_status'),
    componentProps: {
      options: [
        { label: $t('llm.credential_enabled'), value: 1 },
        { label: $t('llm.credential_disabled'), value: 0 },
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
    providerType: 'openai',
    status: 1,
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.credentialId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    credentialName: row.credentialName,
    providerType: row.providerType,
    baseUrl: row.baseUrl || '',
    apiKey: '',
    status: row.status,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // 编辑时 apiKey 留空则不下发该字段（service 不改密文）
  const payload: any = {
    credentialName: values.credentialName,
    providerType: values.providerType,
    baseUrl: values.baseUrl || '',
    credentialInfo: '',
    status: values.status,
  };
  if (values.apiKey) payload.apiKey = values.apiKey;
  try {
    if (isEdit.value) {
      await editLLMCredentialApi({
        ...payload,
        credentialId: editingId.value,
      });
      message.success($t('llm.credential_editSuccess'));
    } else {
      await addLLMCredentialApi(payload);
      message.success($t('llm.credential_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（apiKey 留空 service 不改密文），失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editLLMCredentialApi({
      credentialId: row.credentialId,
      credentialName: row.credentialName,
      providerType: row.providerType,
      baseUrl: row.baseUrl || '',
      credentialInfo: '',
      status: newStatus,
    });
    message.success($t('llm.credential_editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('llm.credential_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.credentialId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.credentialId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('llm.credential_confirmTitle'),
    content: $t('llm.credential_confirmDeleteContent', { count }),
    okText: $t('llm.credential_confirm'),
    cancelText: $t('llm.credential_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteLLMCredentialApi(ids);
      message.success($t('llm.credential_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 导入支持模型 =====
const importVisible = ref(false);
const importCredId = ref('');
const importCredName = ref('');
const importGroup = ref('auto');
const importing = ref(false);

function openImport(row: any) {
  importCredId.value = row.credentialId;
  importCredName.value = row.credentialName || row.providerType || '';
  importGroup.value = 'auto';
  importVisible.value = true;
}

async function onImportSubmit() {
  if (!importGroup.value?.trim()) {
    message.warning($t('llm.credential_importGroupRequired'));
    return;
  }
  importing.value = true;
  try {
    const res: any = await discoverCredentialModelsApi({
      credentialId: importCredId.value,
      modelGroup: importGroup.value.trim(),
    });
    message.success(
      $t('llm.credential_importResult', {
        imported: res?.imported ?? 0,
        skipped: res?.skipped ?? 0,
      }),
    );
    importVisible.value = false;
  } catch {
    /* 拦截器已提示 */
  } finally {
    importing.value = false;
  }
}

onMounted(() => {
  gridApi.query();
});
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('llm.credential_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('llm.credential_batchDelete') }}</Button>
      </template>
      <template #apiKey>
        <Tag color="default">{{ $t('llm.credential_encrypted') }}</Tag>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('llm.credential_enabled')"
          :un-checked-children="$t('llm.credential_disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('llm.credential_edit') }}</Button>
        <Button type="primary" ghost size="small" @click="openImport(row)">
          {{ $t('llm.credential_importModels') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('llm.credential_delete') }}
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
      v-model:open="importVisible"
      :title="$t('llm.credential_importTitle')"
      :width="480"
      :destroy-on-close="true"
      :confirm-loading="importing"
      @ok="onImportSubmit"
    >
      <div class="mb-2 text-gray-500">
        {{ $t('llm.credential_importHint', { name: importCredName }) }}
      </div>
      <div class="mb-1">{{ $t('llm.credential_model_group') }}</div>
      <Input
        v-model:value="importGroup"
        :placeholder="$t('llm.credential_importGroupPlaceholder')"
        @press-enter="onImportSubmit"
      />
    </Modal>
  </Page>
</template>
