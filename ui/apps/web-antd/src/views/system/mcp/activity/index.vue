<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed } from 'vue';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { getMcpActivityListApi } from '#/api/system/mcp/activity';

defineOptions({ name: 'SystemMcpActivity' });

const STATUS_OPTIONS = computed(() => [
  { label: $t('mcp.activity_statusSuccess'), value: 'success' },
  { label: $t('mcp.activity_statusError'), value: 'error' },
]);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'serverName',
    label: $t('mcp.activity_server'),
    componentProps: { placeholder: $t('mcp.activity_serverPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'tool',
    label: $t('mcp.activity_tool'),
    componentProps: { placeholder: $t('mcp.activity_toolPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('mcp.activity_status'),
    componentProps: {
      allowClear: true,
      options: STATUS_OPTIONS.value,
      placeholder: $t('mcp.activity_statusAll'),
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('mcp.activity_seq'), width: 60, fixed: 'left' },
    { field: 'ts', title: $t('mcp.activity_ts'), minWidth: 170 },
    { field: 'serverName', title: $t('mcp.activity_server'), minWidth: 140 },
    { field: 'tool', title: $t('mcp.activity_tool'), minWidth: 140 },
    { field: 'durationMs', title: $t('mcp.activity_durationMs'), width: 100 },
    {
      field: 'status',
      title: $t('mcp.activity_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'keyName', title: $t('mcp.activity_keyName'), width: 120 },
    { field: 'sourceIp', title: $t('mcp.activity_sourceIp'), width: 130 },
    { field: 'errorMessage', title: $t('mcp.activity_errorMessage'), minWidth: 160 },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getMcpActivityListApi({
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
        <span v-if="row.status === 'success'" class="aa-enum aa-enum--magenta">
          {{ $t('mcp.activity_statusSuccess') }}
        </span>
        <span v-else class="aa-enum aa-enum--error">{{ $t('mcp.activity_statusError') }}</span>
      </template>
    </Grid>
  </Page>
</template>
