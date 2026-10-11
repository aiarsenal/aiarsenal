<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed } from 'vue';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { getLLMHealthListApi } from '#/api/system/llm/health';

defineOptions({ name: 'SystemLlmHealth' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'modelName',
    label: $t('llm.health_model'),
    componentProps: { placeholder: $t('llm.health_modelPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('llm.health_status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('llm.health_healthy'), value: 'healthy' },
        { label: $t('llm.health_unhealthy'), value: 'unhealthy' },
      ],
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('llm.health_seq'), width: 60, fixed: 'left' },
    { field: 'checkedAt', title: $t('llm.health_checkedAt'), minWidth: 160 },
    { field: 'modelName', title: $t('llm.health_modelName'), minWidth: 140 },
    { field: 'modelId', title: $t('llm.health_modelId'), minWidth: 160 },
    {
      field: 'status',
      title: $t('llm.health_status'),
      width: 100,
      slots: { default: 'status' },
    },
    { field: 'responseTimeMs', title: $t('llm.health_responseTime'), width: 120 },
    { field: 'healthyCount', title: $t('llm.health_healthyCount'), width: 100 },
    { field: 'unhealthyCount', title: $t('llm.health_unhealthyCount'), width: 110 },
    { field: 'errorMessage', title: $t('llm.health_errorMessage'), minWidth: 200 },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMHealthListApi({
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

const [Grid] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${row.status === 'healthy' ? 'magenta' : 'red'}`]">
          {{ row.status }}
        </span>
      </template>
    </Grid>
  </Page>
</template>
