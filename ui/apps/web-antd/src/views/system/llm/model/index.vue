<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addLLMModelApi,
  deleteLLMModelApi,
  editLLMModelApi,
  getLLMCredentialOptionsApi,
  getLLMModelListApi,
} from '#/api/system/llm/model';
import { getLLMGroupOptionsApi } from '#/api/system/llm/group';

defineOptions({ name: 'SystemLlmModel' });

// ===== 凭据下拉：每次开弹窗都重拉，避免新增凭据后下拉不刷新 =====
const credOptions = ref<{ value: string; label: string; providerType?: string; credentialName?: string }[]>([]);
async function loadCreds() {
  try {
    const res: any = await getLLMCredentialOptionsApi();
    const list = res?.list ?? [];
    credOptions.value = list.map((c: any) => ({
      value: c.credentialId,
      label: c.credentialName ? `${c.credentialName} (${c.providerType})` : c.providerType,
      providerType: c.providerType,
      credentialName: c.credentialName,
    }));
  } catch {
    /* 拦截器已提示 */
  }
}

// 凭据 id → 凭据名（列表「凭据」列回显用；provider 已有独立列，这里只显示名字）
function credLabel(id?: string) {
  if (!id) return '-';
  const c = credOptions.value.find((x) => x.value === id);
  return c?.credentialName || c?.label || id;
}

// openai 兼容族：这些 providerType 共用 OpenAIProvider 翻译。模型 provider 选 openai 时，
// 凭据 providerType 可能是 openai/deepseek/moonshot/groq/openrouter/ollama 之一——都该可见。
const OPENAI_COMPATIBLE = new Set([
  'openai',
  'deepseek',
  'moonshot',
  'groq',
  'openrouter',
  'ollama',
]);
function sameFamily(credType?: string, provider?: string) {
  if (!provider) return true; // 未选 provider 不过滤
  const cf = OPENAI_COMPATIBLE.has(credType || '') ? 'openai' : credType;
  const pf = OPENAI_COMPATIBLE.has(provider) ? 'openai' : provider;
  return cf === pf;
}
// 当前选中 provider：驱动凭据下拉过滤
const selectedProvider = ref('');

// ===== 分组下拉（仅启用分组；来源「分组管理」字典）=====
// 与凭据下拉同策略：每次开弹窗都重拉——tab keep-alive 下组件不重挂载，
// 懒加载一次会导致「分组管理」新增分组后模型弹窗下拉一直是旧列表。
const groupOptions = ref<{ value: string; label: string }[]>([]);
async function loadGroups() {
  try {
    const res: any = await getLLMGroupOptionsApi();
    const list = (res?.list ?? []).filter((g: any) => g.status === 1);
    groupOptions.value = list.map((g: any) => ({
      value: g.groupName,
      label: g.groupName,
    }));
  } catch {
    /* 拦截器已提示 */
  }
}

// 编辑老数据时，model_group 可能不在启用分组里（分组被禁用/删除或历史数据），
// 把当前值补进 options 头部，保证 Select 能回显原值（向后兼容）。
function ensureGroupOption(name: string) {
  if (!name) return;
  if (!groupOptions.value.some((o) => o.value === name)) {
    groupOptions.value = [{ value: name, label: name }, ...groupOptions.value];
  }
}

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
    label: $t('llm.model_name'),
    componentProps: { placeholder: $t('llm.model_namePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'provider',
    label: $t('llm.model_provider'),
    componentProps: {
      showSearch: true,
      allowClear: true,
      options: providerOptions,
      placeholder: $t('llm.model_providerSelectPlaceholder'),
    },
  },
  {
    component: 'Select',
    fieldName: 'group',
    label: $t('llm.model_group'),
    // options 必须走函数懒求值：若直接写 options: groupOptions.value，searchSchema
    // computed 会捕获 groupOptions 依赖；loadGroups() 异步回填后 schema 重算 →
    // useVbenVxeGrid 的 deep watch 把 extends.ts 注入的搜索代理 wrapper 覆盖回原始
    // query（拿不到 getLatestSubmissionValues）→ 搜索条件静默丢失。
    componentProps: () => ({
      showSearch: true,
      allowClear: true,
      options: groupOptions.value,
      placeholder: $t('llm.model_groupSelectPlaceholder'),
    }),
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('llm.model_status'),
    // options 为静态常量（无异步 ref 依赖），直接写不会触发 deep watch 覆盖坑。
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('llm.model_enabled'), value: 1 },
        { label: $t('llm.model_disabled'), value: 0 },
      ],
      placeholder: $t('llm.model_statusSelectPlaceholder'),
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('llm.model_seq'), width: 60 },
    { field: 'modelGroup', title: $t('llm.model_group'), minWidth: 120 },
    { field: 'modelName', title: $t('llm.model_name'), minWidth: 140 },
    { field: 'provider', title: $t('llm.model_provider'), width: 110 },
    { field: 'providerModel', title: $t('llm.model_providerModel'), minWidth: 160 },
    {
      field: 'credentialId',
      title: $t('llm.model_credential'),
      minWidth: 160,
      slots: { default: 'credential' },
    },
    {
      field: 'inputPricePerMillion',
      title: $t('llm.model_price'),
      width: 170,
      slots: { default: 'price' },
    },
    { field: 'weight', title: $t('llm.model_weight'), width: 70 },
    {
      field: 'maxOutputTokens',
      title: $t('llm.model_maxOutputTokens'),
      width: 100,
      formatter: ({ cellValue }: any) => (cellValue ? String(cellValue) : '-'),
    },
    {
      field: 'status',
      title: $t('llm.model_status'),
      width: 90,
      slots: { default: 'status' },
    },
    {
      field: 'action',
      title: $t('llm.model_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMModelListApi({
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
  isEdit.value ? $t('llm.model_modalTitleEdit') : $t('llm.model_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select' as const,
    fieldName: 'modelGroup',
    label: $t('llm.model_group'),
    help: $t('llm.model_groupHelp'),
    rules: 'required',
    componentProps: () => ({
      showSearch: true,
      options: groupOptions.value,
      placeholder: $t('llm.model_groupSelectPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'modelName',
    label: $t('llm.model_name'),
    help: $t('llm.model_nameHelp'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.model_nameInputPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'provider',
    label: $t('llm.model_provider'),
    help: $t('llm.model_providerHelp'),
    rules: 'required',
    componentProps: {
      showSearch: true,
      options: providerOptions,
      placeholder: $t('llm.model_providerInputPlaceholder'),
      style: { width: '100%' },
      onChange: (v: any) => onProviderChange(v),
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'providerModel',
    label: $t('llm.model_providerModel'),
    help: $t('llm.model_providerModelHelp'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.model_providerModelPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'credentialId',
    label: $t('llm.model_credential'),
    help: $t('llm.model_credentialHelp'),
    rules: 'required',
    componentProps: () => ({
      allowClear: true,
      options: credOptions.value.filter((c: any) =>
        sameFamily(c.providerType, selectedProvider.value),
      ),
      placeholder: $t('llm.model_credentialPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'inputPriceM',
    label: $t('llm.model_inputPrice'),
    help: $t('llm.model_inputPriceHelp'),
    componentProps: {
      min: 0,
      precision: 3,
      step: 0.1,
      addonAfter: '$/M tok',
      style: { width: '100%' },
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'outputPriceM',
    label: $t('llm.model_outputPrice'),
    help: $t('llm.model_outputPriceHelp'),
    componentProps: {
      min: 0,
      precision: 3,
      step: 0.1,
      addonAfter: '$/M tok',
      style: { width: '100%' },
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'maxInputTokens',
    label: $t('llm.model_maxInputTokens'),
    help: $t('llm.model_maxInputTokensHelp'),
    componentProps: {
      min: 0,
      step: 1000,
      addonAfter: 'tok',
      style: { width: '100%' },
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'maxOutputTokens',
    label: $t('llm.model_maxOutputTokens'),
    help: $t('llm.model_maxOutputTokensHelp'),
    componentProps: {
      min: 0,
      step: 1000,
      addonAfter: 'tok',
      style: { width: '100%' },
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'weight',
    label: $t('llm.model_weight'),
    help: $t('llm.model_weightHelp'),
    rules: z
      .number()
      .int($t('llm.model_weightInt'))
      .min(1, $t('llm.model_weightMin'))
      .max(100, $t('llm.model_weightMax')),
    componentProps: { min: 1, max: 100, style: { width: '100%' } },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'status',
    label: $t('llm.model_status'),
    help: $t('llm.model_statusHelp'),
    componentProps: {
      options: [
        { label: $t('llm.model_enabled'), value: 1 },
        { label: $t('llm.model_disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    layout: 'horizontal',
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
      labelWidth: 120,
    },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

// 切换 provider：驱动凭据下拉按同族过滤；若已选凭据不同族则清空，避免提交脏数据
async function onProviderChange(v: any) {
  const prev = selectedProvider.value;
  selectedProvider.value = v ?? '';
  if (prev === selectedProvider.value) return;
  const values: any = await formApi.getValues();
  const cred = credOptions.value.find((c: any) => c.value === values?.credentialId);
  if (cred && !sameFamily(cred.providerType, selectedProvider.value)) {
    await formApi.setFieldValue('credentialId', undefined);
  }
}

async function openAdd() {
  await loadCreds();
  await loadGroups();
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  selectedProvider.value = 'openai';
  await formApi.setValues({
    provider: 'openai',
    weight: 1,
    status: 1,
    inputPriceM: 0,
    outputPriceM: 0,
    maxInputTokens: 0,
    maxOutputTokens: 8192,
  });
}

async function openEdit(row: any) {
  await loadCreds();
  await loadGroups();
  // 老数据 model_group 可能不在启用分组里，补进 options 保证回显
  ensureGroupOption(row.modelGroup);
  isEdit.value = true;
  editingId.value = row.modelId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  selectedProvider.value = row.provider || '';
  await formApi.setValues({
    modelGroup: row.modelGroup,
    modelName: row.modelName,
    provider: row.provider,
    providerModel: row.providerModel,
    credentialId: row.credentialId || undefined,
    inputPriceM: +row.inputPricePerMillion, // 后端 JSON 单位即 $/M，直读
    outputPriceM: +row.outputPricePerMillion,
    weight: row.weight,
    status: row.status,
    maxInputTokens: row.maxInputTokens ?? 0,
    maxOutputTokens: row.maxOutputTokens ?? 0,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // 价格单位 $/M 直传（后端 Milli JSON 边界即 $/M）
  const payload: any = {
    modelName: values.modelName,
    modelGroup: values.modelGroup,
    provider: values.provider,
    providerModel: values.providerModel,
    credentialId: values.credentialId || '',
    inputPricePerMillion: +values.inputPriceM,
    outputPricePerMillion: +values.outputPriceM,
    weight: values.weight,
    status: values.status,
    maxInputTokens: values.maxInputTokens ?? 0,
    maxOutputTokens: values.maxOutputTokens ?? 0,
  };
  try {
    if (isEdit.value) {
      await editLLMModelApi({ ...payload, modelId: editingId.value });
      message.success($t('llm.model_editSuccess'));
    } else {
      await addLLMModelApi(payload);
      message.success($t('llm.model_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（per-token 价格原样回填），失败回滚。
// blocked 由健康检查/管理另行设置，置位时开关禁用，避免与 status 冲突。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editLLMModelApi({
      modelId: row.modelId,
      modelName: row.modelName,
      modelGroup: row.modelGroup,
      provider: row.provider,
      providerModel: row.providerModel,
      credentialId: row.credentialId || '',
      inputPricePerMillion: +row.inputPricePerMillion,
      outputPricePerMillion: +row.outputPricePerMillion,
      weight: row.weight,
      status: newStatus,
    });
    message.success($t('llm.model_editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('llm.model_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.modelId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.modelId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('llm.model_confirmTitle'),
    content: $t('llm.model_confirmDeleteContent', { count }),
    okText: $t('llm.model_confirm'),
    cancelText: $t('llm.model_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteLLMModelApi(ids);
      message.success($t('llm.model_deleteSuccess'));
      gridApi.query();
    },
  });
}

onMounted(() => {
  loadGroups(); // 预加载搜索「分组」下拉
  loadCreds(); // 预加载凭据映射：列表「凭据」列用 credOptions 把 id 回显成名字
  gridApi.query();
});
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('llm.model_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('llm.model_batchDelete') }}</Button>
      </template>
      <template #price="{ row }">
        <span>{{ (+row.inputPricePerMillion).toFixed(3) }}</span>
        <span class="mx-1 text-gray-400">/</span>
        <span>{{ (+row.outputPricePerMillion).toFixed(3) }}</span>
        <span class="ml-1 text-gray-400">$/M</span>
      </template>
      <template #credential="{ row }">{{ credLabel(row.credentialId) }}</template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1 && !row.blocked"
          :disabled="!!row.blocked"
          :checked-children="$t('llm.model_enabled')"
          :un-checked-children="$t('llm.model_disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('llm.model_edit') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('llm.model_delete') }}
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
