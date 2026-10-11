<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, DatePicker, message, Modal, Radio, Table } from 'ant-design-vue';
import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  getLLMSpendAggregateApi,
  getLLMSpendApi,
  getLLMSpendListApi,
} from '#/api/system/llm/spend';

defineOptions({ name: 'SystemLlmSpend' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'model',
    label: $t('llm.spend_model'),
    componentProps: { placeholder: $t('llm.spend_modelPlaceholder'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'userId',
    label: $t('llm.spend_userId'),
    componentProps: { placeholder: $t('llm.spend_userId'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'teamId',
    label: $t('llm.spend_teamId'),
    componentProps: { placeholder: $t('llm.spend_teamId'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'apiKey',
    label: $t('llm.spend_apiKey'),
    componentProps: { placeholder: $t('llm.spend_apiKey'), allowClear: true },
  },
  {
    component: 'RangePicker',
    fieldName: 'dateRange',
    label: $t('llm.spend_dateRange'),
    componentProps: {
      valueFormat: 'YYYY-MM-DD',
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('llm.spend_seq'), width: 60, fixed: 'left' },
    { field: 'startTime', title: $t('llm.spend_startTime'), minWidth: 170 },
    { field: 'model', title: $t('llm.spend_model'), minWidth: 160 },
    { field: 'customLlmProvider', title: $t('llm.spend_provider'), width: 110 },
    { field: 'promptTokens', title: $t('llm.spend_promptTokens'), width: 120 },
    { field: 'completionTokens', title: $t('llm.spend_completionTokens'), width: 130 },
    { field: 'spend', title: $t('llm.spend_spend'), width: 120, slots: { default: 'spend' } },
    {
      field: 'requestDurationMs',
      title: $t('llm.spend_duration'),
      width: 110,
    },
    {
      field: 'requesterIpAddress',
      title: $t('llm.spend_requestIp'),
      minWidth: 140,
    },
    {
      field: 'action',
      title: $t('llm.spend_action'),
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
          userId: form?.userId,
          teamId: form?.teamId,
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
        const res = await getLLMSpendListApi(params);
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

// ===== 详情弹窗（输入/输出内容）=====
const detailVisible = ref(false);
const detailLoading = ref(false);
const detailRow = ref<any>({});

async function onView(row: any) {
  detailRow.value = { ...row };
  detailVisible.value = true;
  // 拉取完整记录，确保 messages/response 字段齐全
  if (!row.requestId) return;
  detailLoading.value = true;
  try {
    const res: any = await getLLMSpendApi(row.requestId);
    if (res) detailRow.value = res;
  } catch {
    message.error($t('llm.spend_fetchDetailError'));
  } finally {
    detailLoading.value = false;
  }
}

// ===== 汇总报表（花费归集：按日 / 密钥 / 用户 / 模型聚合） =====
const aggVisible = ref(false);
const aggLoading = ref(false);
const aggDimension = ref<'date' | 'key' | 'user' | 'model'>('date');
const aggRange = ref<[string, string] | null>(null);
const aggList = ref<any[]>([]);
const aggTotals = ref<any>({});

function openAggregate() {
  aggVisible.value = true;
  loadAggregate();
}

async function loadAggregate() {
  aggLoading.value = true;
  try {
    const res = await getLLMSpendAggregateApi({
      dimension: aggDimension.value,
      dateRange: aggRange.value ?? undefined,
    });
    aggList.value = res?.list ?? [];
    aggTotals.value = res?.totals ?? {};
  } catch {
    message.error($t('llm.spend_aggregateError'));
  } finally {
    aggLoading.value = false;
  }
}

// 维度取值列：date → 日期；key → 别名（无则 hash）；user → 用户ID；model → 模型名
function dimValue(row: any) {
  switch (aggDimension.value) {
    case 'date': {
      return row.date;
    }
    case 'key': {
      return row.keyAlias || row.scopeId;
    }
    case 'user': {
      return row.scopeId;
    }
    default: {
      return row.model;
    }
  }
}

const aggColumns = computed(() => [
  { title: $t('llm.spend_dimension'), dataIndex: 'dim', key: 'dim' },
  { title: $t('llm.spend_spend'), dataIndex: 'spend', key: 'spend' },
  { title: $t('llm.spend_apiRequests'), dataIndex: 'apiRequests', key: 'apiRequests' },
  { title: $t('llm.spend_promptTokens'), dataIndex: 'promptTokens', key: 'promptTokens' },
  { title: $t('llm.spend_completionTokens'), dataIndex: 'completionTokens', key: 'completionTokens' },
]);

const aggDataSource = computed(() =>
  aggList.value.map((r) => ({ ...r, dim: dimValue(r) })),
);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button size="small" @click="openAggregate">{{ $t('llm.spend_aggregate') }}</Button>
      </template>
      <template #spend="{ row }">
        ${{ (row.spend || 0).toFixed(6) }}
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="onView(row)">{{ $t('llm.spend_detail') }}</Button>
      </template>
    </Grid>

    <Modal
      v-model:open="aggVisible"
      :title="$t('llm.spend_aggregateTitle')"
      :width="860"
      :destroy-on-close="true"
      :footer="null"
    >
      <div class="agg-bar">
        <Radio.Group
          v-model:value="aggDimension"
          button-style="solid"
          size="small"
          @change="loadAggregate"
        >
          <Radio.Button value="date">{{ $t('llm.spend_dimensionDate') }}</Radio.Button>
          <Radio.Button value="key">{{ $t('llm.spend_dimensionKey') }}</Radio.Button>
          <Radio.Button value="user">{{ $t('llm.spend_dimensionUser') }}</Radio.Button>
          <Radio.Button value="model">{{ $t('llm.spend_dimensionModel') }}</Radio.Button>
        </Radio.Group>
        <DatePicker.RangePicker
          v-model:value="aggRange"
          size="small"
          value-format="YYYY-MM-DD"
          :allow-clear="true"
          @change="loadAggregate"
        />
      </div>
      <div class="agg-totals">
        <div class="agg-total">
          <div class="agg-total-label">{{ $t('llm.spend_totalSpend') }}</div>
          <div class="agg-total-value">${{ (aggTotals.spend || 0).toFixed(6) }}</div>
        </div>
        <div class="agg-total">
          <div class="agg-total-label">{{ $t('llm.spend_totalRequests') }}</div>
          <div class="agg-total-value">{{ aggTotals.apiRequests ?? 0 }}</div>
        </div>
        <div class="agg-total">
          <div class="agg-total-label">{{ $t('llm.spend_totalPromptTokens') }}</div>
          <div class="agg-total-value">{{ aggTotals.promptTokens ?? 0 }}</div>
        </div>
        <div class="agg-total">
          <div class="agg-total-label">{{ $t('llm.spend_totalCompletionTokens') }}</div>
          <div class="agg-total-value">{{ aggTotals.completionTokens ?? 0 }}</div>
        </div>
      </div>
      <Table
        size="small"
        :columns="aggColumns"
        :data-source="aggDataSource"
        :loading="aggLoading"
        :pagination="{ pageSize: 20, showSizeChanger: false }"
        row-key="dim"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'spend'">
            ${{ (record.spend || 0).toFixed(6) }}
          </template>
        </template>
      </Table>
    </Modal>

    <Modal
      v-model:open="detailVisible"
      :title="$t('llm.spend_detailTitle')"
      :width="860"
      :destroy-on-close="true"
      :footer="null"
    >
      <div class="io-wrap" :class="{ 'io-loading': detailLoading }">
        <div class="io-block">
          <div class="io-title">{{ $t('llm.spend_input') }}</div>
          <pre v-if="detailRow.messages" class="io-pre">{{
            detailRow.messages
          }}</pre>
          <div v-else class="io-empty">{{ $t('llm.spend_empty') }}</div>
        </div>
        <div class="io-block">
          <div class="io-title">{{ $t('llm.spend_output') }}</div>
          <pre v-if="detailRow.response" class="io-pre">{{
            detailRow.response
          }}</pre>
          <div v-else class="io-empty">{{ $t('llm.spend_empty') }}</div>
        </div>
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.agg-bar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}
.agg-totals {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}
.agg-total {
  flex: 1;
  padding: 8px 12px;
  background: var(--ant-color-fill-quaternary);
  border-radius: 4px;
}
.agg-total-label {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
}
.agg-total-value {
  margin-top: 2px;
  font-weight: 600;
}
.io-wrap {
  display: flex;
  gap: 16px;
  padding: 4px 0;
}
.io-block {
  flex: 1;
  min-width: 0;
}
.io-title {
  margin-bottom: 6px;
  font-weight: 600;
  color: var(--ant-color-primary);
}
.io-pre {
  max-height: 420px;
  margin: 0;
  padding: 8px;
  overflow: auto;
  background: var(--ant-color-fill-quaternary);
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
}
.io-empty {
  color: var(--ant-color-text-secondary);
  font-size: 12px;
}
.io-loading {
  opacity: 0.6;
}
</style>
