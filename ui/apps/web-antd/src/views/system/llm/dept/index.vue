<script lang="ts" setup>
/**
 * LLM 部门配置（仅 list + edit，无 add/delete）。
 * 旧页：ui/src/views/system/llm/dept/index.vue（Element Plus）。
 * gin: /api/v1/llm/dept/{list,get,edit}，DTO 见 api/interfaces/dto/llm.go。
 */
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  editLLMDeptApi,
  getLLMDeptListApi,
  type LLMDeptApi,
} from '#/api/system/llm/dept';

defineOptions({ name: 'SystemLLMDept' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('llm.dept_deptName'),
    componentProps: { placeholder: $t('llm.dept_deptNamePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('llm.dept_seq'), width: 60, fixed: 'left' },
    { field: 'deptName', title: $t('llm.dept_dept'), minWidth: 140 },
    {
      field: 'levelPath',
      title: $t('llm.dept_levelPath'),
      minWidth: 180,
      slots: { default: 'levelPath' },
    },
    {
      field: 'leaderName',
      title: $t('llm.dept_leader'),
      width: 180,
      slots: { default: 'leader' },
    },
    {
      field: 'spend',
      title: $t('llm.dept_spend'),
      width: 140,
      slots: { default: 'spend' },
    },
    {
      field: 'llmBlocked',
      title: $t('llm.dept_status'),
      width: 90,
      slots: { default: 'status' },
    },
    {
      field: 'action',
      title: $t('llm.dept_action'),
      width: 120,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMDeptListApi({
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

// ===== 编辑弹窗 =====
const modalVisible = ref(false);
const editingDeptName = ref('');
const editingDeptId = ref('');

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Textarea' as const,
    fieldName: 'llmModels',
    label: $t('llm.dept_models'),
    componentProps: {
      placeholder: $t('llm.dept_modelsPlaceholder'),
      rows: 3,
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'llmTpmLimit',
    label: $t('llm.dept_tpmLimit'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'llmRpmLimit',
    label: $t('llm.dept_rpmLimit'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'llmMaxParallelRequests',
    label: $t('llm.dept_maxParallel'),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'Select' as const,
    fieldName: 'llmBudgetDuration',
    label: $t('llm.dept_budgetDuration'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('llm.dept_durationNone'), value: '' },
        { label: $t('llm.dept_durationDay'), value: '1d' },
        { label: $t('llm.dept_durationWeek'), value: '7d' },
        { label: $t('llm.dept_durationMonth'), value: '30d' },
      ],
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'llmRouterSettings',
    label: $t('llm.dept_routerSettings'),
    componentProps: {
      placeholder: $t('llm.dept_routerSettingsPlaceholder'),
      rows: 3,
    },
  },
  {
    component: 'Switch' as const,
    fieldName: 'llmBlocked',
    label: $t('llm.dept_blockedLabel'),
    componentProps: {
      checkedValue: 1,
      unCheckedValue: 0,
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

async function openEdit(row: LLMDeptApi.ListItem) {
  editingDeptId.value = row.deptId;
  editingDeptName.value = row.deptName || '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    llmModels: row.llmModels || '[]',
    llmTpmLimit: row.llmTpmLimit ?? 0,
    llmRpmLimit: row.llmRpmLimit ?? 0,
    llmMaxParallelRequests: row.llmMaxParallelRequests ?? 0,
    llmBudgetDuration: row.llmBudgetDuration || '',
    llmRouterSettings: row.llmRouterSettings || '{}',
    llmBlocked: row.llmBlocked ?? 0,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    await editLLMDeptApi({ ...values, deptId: editingDeptId.value });
    message.success($t('llm.dept_saveSuccess'));
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

const modalTitle = computed(() =>
  editingDeptName.value
    ? $t('llm.dept_modalTitleWithName', { name: editingDeptName.value })
    : $t('llm.dept_modalTitle'),
);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #levelPath="{ row }">
        <span v-if="row.levelPath">{{ row.levelPath }}</span>
        <span v-else>—</span>
      </template>
      <template #leader="{ row }">
        <span v-if="row.leaderName">{{
          $t('llm.dept_leaderFormat', { name: row.leaderName, empNo: row.leaderEmpNo })
        }}</span>
        <span v-else>—</span>
      </template>
      <template #spend="{ row }">
        ${{ Number(row.spend || 0).toFixed(6) }}
      </template>
      <template #status="{ row }">
        <span v-if="row.llmBlocked" class="aa-enum aa-enum--error">{{ $t('llm.dept_blocked') }}</span>
        <span v-else class="aa-enum aa-enum--magenta">{{ $t('llm.dept_normal') }}</span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('llm.dept_config') }}</Button>
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
