<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { Page } from '@vben/common-ui';

import { Button, Modal } from 'ant-design-vue';
import dayjs from 'dayjs';
import { computed, ref } from 'vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { getLLMErrorListApi } from '#/api/system/llm/error';

defineOptions({ name: 'SystemLlmError' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'model',
    label: $t('llm.error_model'),
    componentProps: { placeholder: $t('llm.error_modelPlaceholder'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'apiKey',
    label: $t('llm.error_apiKey'),
    componentProps: { placeholder: $t('llm.error_apiKey'), allowClear: true },
  },
  {
    component: 'RangePicker',
    fieldName: 'dateRange',
    label: $t('llm.error_dateRange'),
    componentProps: {
      valueFormat: 'YYYY-MM-DD',
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('llm.error_seq'), width: 60, fixed: 'left' },
    { field: 'startTime', title: $t('llm.error_time'), minWidth: 170 },
    { field: 'modelGroup', title: $t('llm.error_modelGroup'), minWidth: 140 },
    { field: 'modelName', title: $t('llm.error_modelColumn'), minWidth: 160 },
    { field: 'statusCode', title: $t('llm.error_statusCode'), width: 90 },
    {
      field: 'exceptionType',
      title: $t('llm.error_exceptionType'),
      minWidth: 160,
      slots: { default: 'exceptionType' },
    },
    {
      field: 'exceptionString',
      title: $t('llm.error_exceptionString'),
      minWidth: 220,
      slots: { default: 'exceptionString' },
    },
    { field: 'userId', title: $t('llm.error_user'), minWidth: 160 },
    {
      field: 'action',
      title: $t('llm.error_action'),
      width: 90,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const params: any = {
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          model: form?.model,
          apiKey: form?.apiKey,
        };
        // RangePicker valueFormat=YYYY-MM-DD 时返回字符串数组；兜底格式化
        if (Array.isArray(form?.dateRange) && form.dateRange.length === 2) {
          const [s, e] = form.dateRange;
          params.dateRange = [
            dayjs(s).format('YYYY-MM-DD'),
            dayjs(e).format('YYYY-MM-DD'),
          ];
        }
        const res = await getLLMErrorListApi(params);
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

// ===== 详情弹窗（异常信息/请求参数）=====
const detailVisible = ref(false);
const detailRow = ref<any>({});

function onView(row: any) {
  detailRow.value = { ...row };
  detailVisible.value = true;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #exceptionType="{ row }">
        <span>{{ row.exceptionType || '-' }}</span>
      </template>
      <template #exceptionString="{ row }">
        <span class="ellipsis-cell" :title="row.exceptionString">{{
          row.exceptionString || '-'
        }}</span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="onView(row)">{{ $t('llm.error_detail') }}</Button>
      </template>
    </Grid>

    <Modal
      v-model:open="detailVisible"
      :title="$t('llm.error_detailTitle')"
      :width="720"
      :destroy-on-close="true"
      :footer="null"
    >
      <div class="detail-wrap">
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_time') }}</div>
          <div class="detail-value">{{ detailRow.startTime || '-' }}</div>
        </div>
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_modelGroup') }}</div>
          <div class="detail-value">{{ detailRow.modelGroup || '-' }}</div>
        </div>
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_modelColumn') }}</div>
          <div class="detail-value">
            {{ detailRow.modelName || '-' }}
          </div>
        </div>
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_statusCode') }}</div>
          <div class="detail-value">{{ detailRow.statusCode || '-' }}</div>
        </div>
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_exceptionType') }}</div>
          <div class="detail-value">
            {{ detailRow.exceptionType || '-' }}
          </div>
        </div>
        <div class="detail-row">
          <div class="detail-label">{{ $t('llm.error_user') }}</div>
          <div class="detail-value">{{ detailRow.userId || '-' }}</div>
        </div>
        <div class="detail-block">
          <div class="detail-label">{{ $t('llm.error_exceptionString') }}</div>
          <pre v-if="detailRow.exceptionString" class="detail-pre">{{
            detailRow.exceptionString
          }}</pre>
          <div v-else class="detail-empty">{{ $t('llm.error_empty') }}</div>
        </div>
        <div class="detail-block">
          <div class="detail-label">{{ $t('llm.error_requestParams') }}</div>
          <pre v-if="detailRow.requestKwargs" class="detail-pre">{{
            detailRow.requestKwargs
          }}</pre>
          <div v-else class="detail-empty">{{ $t('llm.error_empty') }}</div>
        </div>
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.ellipsis-cell {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
.detail-wrap {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0;
}
.detail-row {
  display: flex;
  gap: 12px;
  font-size: 13px;
}
.detail-label {
  flex: 0 0 80px;
  color: var(--ant-color-text-secondary);
}
.detail-value {
  flex: 1;
  min-width: 0;
  word-break: break-all;
}
.detail-block .detail-label {
  margin-bottom: 6px;
}
.detail-pre {
  max-height: 240px;
  margin: 0;
  padding: 8px;
  overflow: auto;
  background: var(--ant-color-fill-quaternary);
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
}
.detail-empty {
  color: var(--ant-color-text-secondary);
  font-size: 12px;
}
</style>
