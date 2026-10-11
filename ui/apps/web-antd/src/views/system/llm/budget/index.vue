<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import GrantEditor, { type GrantRowValue } from '#/components/GrantEditor.vue';
import { $t } from '#/locales';
import {
  addLLMBudgetApi,
  deleteLLMBudgetApi,
  editLLMBudgetApi,
  getLLMBudgetListApi,
} from '#/api/system/llm/budget';

defineOptions({ name: 'SystemLlmBudget' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('llm.budget_name'),
    componentProps: { placeholder: $t('llm.budget_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('llm.budget_seq'), width: 60 },
    { field: 'budgetName', title: $t('llm.budget_name'), minWidth: 140 },
    { field: 'maxBudget', title: $t('llm.budget_maxBudget'), width: 120, slots: { default: 'maxBudget' } },
    { field: 'softBudget', title: $t('llm.budget_softBudget'), width: 120, slots: { default: 'softBudget' } },
    { field: 'tpmLimit', title: $t('llm.budget_tpm'), width: 100 },
    { field: 'rpmLimit', title: $t('llm.budget_rpm'), width: 100 },
    { field: 'budgetDuration', title: $t('llm.budget_duration'), width: 100 },
    { field: 'budgetResetAt', title: $t('llm.budget_resetAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('llm.budget_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMBudgetListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin LLMBudgetSearchRes → { list, total }
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
  isEdit.value ? $t('llm.budget_modalTitleEdit') : $t('llm.budget_modalTitleAdd'),
);

// 周期下拉（gin budgetDuration: "" / "1d" / "7d" / "30d"）
const durationOptions = computed(() => [
  { label: $t('llm.budget_durationNone'), value: '' },
  { label: $t('llm.budget_durationDay'), value: '1d' },
  { label: $t('llm.budget_durationWeek'), value: '7d' },
  { label: $t('llm.budget_durationMonth'), value: '30d' },
]);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'budgetName',
    label: $t('llm.budget_name'),
    help: $t('llm.budget_nameHelp'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.budget_namePlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'maxBudget',
    label: $t('llm.budget_maxBudget'),
    help: $t('llm.budget_maxBudgetHelp'),
    componentProps: { min: 0, precision: 2, addonAfter: '$', style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'softBudget',
    label: $t('llm.budget_softBudget'),
    help: $t('llm.budget_softBudgetHelp'),
    componentProps: { min: 0, precision: 2, addonAfter: '$', style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'tpmLimit',
    label: $t('llm.budget_tpm'),
    help: $t('llm.budget_tpmHelp'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'rpmLimit',
    label: $t('llm.budget_rpm'),
    help: $t('llm.budget_rpmHelp'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'maxParallelRequests',
    label: $t('llm.budget_parallel'),
    help: $t('llm.budget_parallelHelp'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'Select' as const,
    fieldName: 'budgetDuration',
    label: $t('llm.budget_duration'),
    help: $t('llm.budget_durationHelp'),
    componentProps: {
      options: durationOptions.value,
      style: { width: '100%' },
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

// ===== 适用范围（GrantEditor 条件行）：预算圈人，命中者共享/受限此额度 =====
// 七键必须齐全（GrantEditor 契约），回显时对旧数据做规范化补齐。
const grantRows = ref<GrantRowValue[]>([]);

function parseGrantRules(s: unknown): GrantRowValue[] {
  let rows: any;
  try {
    rows = typeof s === 'string' && s ? JSON.parse(s) : [];
  } catch {
    rows = [];
  }
  if (!Array.isArray(rows)) return [];
  return rows.map((r: any) => ({
    userIds: Array.isArray(r.userIds) ? r.userIds : [],
    deptId: r.deptId ?? '',
    roleIds: Array.isArray(r.roleIds) ? r.roleIds : [],
    postIds: Array.isArray(r.postIds) ? r.postIds : [],
    exclude: !!r.exclude,
    groupId: r.groupId ?? '',
    setId: r.setId ?? '',
  }));
}

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ budgetDuration: '' });
  grantRows.value = [];
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.budgetId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  // gin list 行字段即表单字段（budgetId 除外，提交时补）
  formApi.setValues({
    budgetName: row.budgetName,
    maxBudget: row.maxBudget,
    softBudget: row.softBudget,
    tpmLimit: row.tpmLimit,
    rpmLimit: row.rpmLimit,
    maxParallelRequests: row.maxParallelRequests,
    budgetDuration: row.budgetDuration ?? '',
  });
  grantRows.value = parseGrantRules(row.grantRules);
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const payload = { ...values, grantRules: JSON.stringify(grantRows.value ?? []) };
  try {
    if (isEdit.value) {
      // gin edit payload 需 budgetId
      await editLLMBudgetApi({ ...payload, budgetId: editingId.value });
      message.success($t('llm.budget_editSuccess'));
    } else {
      await addLLMBudgetApi(payload);
      message.success($t('llm.budget_addSuccess'));
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
    message.warning($t('llm.budget_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.budgetId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.budgetId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('llm.budget_confirmTitle'),
    content: $t('llm.budget_confirmDeleteContent', { count }),
    okText: $t('llm.budget_confirm'),
    cancelText: $t('llm.budget_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteLLMBudgetApi(ids);
      message.success($t('llm.budget_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 格式化 =====
function fmtMoney(v: any): string {
  if (v === null || v === undefined || v === '') return '-';
  const n = Number(v);
  return Number.isFinite(n) ? `$${n.toFixed(2)}` : '-';
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('llm.budget_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('llm.budget_batchDelete') }}</Button>
      </template>
      <template #maxBudget="{ row }">
        <span v-if="row.maxBudget">{{ fmtMoney(row.maxBudget) }}</span>
        <span v-else>∞</span>
      </template>
      <template #softBudget="{ row }">
        {{ fmtMoney(row.softBudget) }}
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('llm.budget_edit') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('llm.budget_delete') }}
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
      <!-- 适用范围：留空=不圈人（仅经虚拟密钥挂靠生效）；圈定后网关按用户身份判定额度 -->
      <div class="mt-3">
        <div class="mb-1 font-medium">{{ $t('llm.budget_grantRules') }}</div>
        <div class="mb-2 text-xs opacity-70">
          {{ $t('llm.budget_grantRulesHelp') }}
        </div>
        <GrantEditor
          v-model="grantRows"
          :default-mods="['group']"
          :mod-order="['group', 'users', 'dept', 'roles', 'posts', 'set']"
        />
      </div>
    </Modal>
  </Page>
</template>
