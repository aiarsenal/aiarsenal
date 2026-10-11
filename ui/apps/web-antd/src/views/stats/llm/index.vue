<script lang="ts" setup>
import type { EchartsUIType } from '@vben/plugins/echarts';

import type { StatsLlmApi } from '#/api/system/stats/llm';

import { computed, onMounted, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Card, Empty, RadioGroup, Spin, Table, Tag } from 'ant-design-vue';

import { EchartsUI, useEcharts } from '@vben/plugins/echarts';

import {
  getStatsLlmOverviewApi,
  getStatsLlmTopApi,
  getStatsLlmTrendApi,
} from '#/api/system/stats/llm';
import { $t } from '#/locales';

defineOptions({ name: 'StatsLlmData' });

// ===== 总览 =====
const loading = ref(false);
const overview = ref<StatsLlmApi.Overview | undefined>();

async function loadOverview() {
  loading.value = true;
  try {
    overview.value = await getStatsLlmOverviewApi();
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

// ===== 趋势 =====
const trendDays = ref<'14' | '30' | '7'>('7');
const trendLoading = ref(false);
const trendPoints = ref<StatsLlmApi.TrendPoint[]>([]);
const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

async function loadTrend() {
  trendLoading.value = true;
  try {
    const res = await getStatsLlmTrendApi(Number(trendDays.value));
    trendPoints.value = res?.points ?? [];
  } catch {
    trendPoints.value = [];
  } finally {
    trendLoading.value = false;
  }
}

/** tokens（柱）+ 花费（线，右轴）双轴趋势 */
function renderTrend() {
  renderEcharts({
    legend: { data: [$t('stats.llm_tokens'), $t('stats.llm_spend')] },
    series: [
      {
        data: trendPoints.value.map((p) => p.totalTokens),
        itemStyle: { borderRadius: [3, 3, 0, 0] },
        name: $t('stats.llm_tokens'),
        type: 'bar',
        yAxisIndex: 0,
      },
      {
        areaStyle: { opacity: 0.12 },
        data: trendPoints.value.map((p) => Number(p.spend.toFixed(4))),
        itemStyle: { color: '#f59e0b' },
        name: $t('stats.llm_spend'),
        smooth: true,
        type: 'line',
        yAxisIndex: 1,
      },
    ],
    tooltip: {
      axisPointer: { type: 'cross' },
      trigger: 'axis',
    },
    xAxis: {
      axisTick: { show: false },
      boundaryGap: true,
      data: trendPoints.value.map((p) => p.date),
      type: 'category',
    },
    yAxis: [
      {
        axisTick: { show: false },
        name: $t('stats.llm_tokens'),
        splitNumber: 4,
        type: 'value',
      },
      {
        axisLabel: { formatter: (v: number) => `$${v}` },
        axisTick: { show: false },
        name: $t('stats.llm_spend'),
        splitLine: { show: false },
        splitNumber: 4,
        type: 'value',
      },
    ],
  });
}

watch(trendPoints, renderTrend);

function onTrendDaysChange(e: any) {
  trendDays.value = e.target.value;
  loadTrend();
}

// ===== Top 榜（模型 / 虚拟Key / 用户） =====
const topDays = ref<'14' | '30' | '7'>('7');
const topDimension = ref<StatsLlmApi.TopDimension>('model');
const topLoading = ref(false);
const topList = ref<StatsLlmApi.TopItem[]>([]);

async function loadTop() {
  topLoading.value = true;
  try {
    const res = await getStatsLlmTopApi(
      topDimension.value,
      Number(topDays.value),
      10,
    );
    topList.value = res?.list ?? [];
  } catch {
    topList.value = [];
  } finally {
    topLoading.value = false;
  }
}

function onTopDimensionChange(key: any) {
  topDimension.value = key as StatsLlmApi.TopDimension;
  loadTop();
}

function onTopDaysChange(e: any) {
  topDays.value = e.target.value;
  loadTop();
}

const topColumns = computed(() => [
  { dataIndex: 'name', title: topDimensionTitle(topDimension.value) },
  { dataIndex: 'requests', title: () => $t('stats.llm_requests'), width: 100 },
  { dataIndex: 'totalTokens', title: () => $t('stats.llm_tokens'), width: 120 },
  { dataIndex: 'spend', title: () => $t('stats.llm_spend'), width: 110 },
  { dataIndex: 'errorCount', title: () => $t('stats.llm_errors'), width: 90 },
  { dataIndex: 'errorRate', title: () => $t('stats.llm_errorRate'), width: 100 },
]);

function topDimensionTitle(d: StatsLlmApi.TopDimension) {
  return d === 'model'
    ? $t('stats.llm_topModel')
    : d === 'virtualKey'
      ? $t('stats.llm_topVirtualKey')
      : $t('stats.llm_topUser');
}

// ===== 展示 =====
function fmtMoney(n?: number) {
  return Number.isFinite(n) ? `$${(n as number).toFixed(2)}` : '-';
}

function fmtInt(n?: number) {
  return Number.isFinite(n) ? (n as number).toLocaleString() : '-';
}

function fmtRate(n?: number) {
  return Number.isFinite(n) ? `${(n as number).toFixed(2)}%` : '-';
}

function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

/** 错误率分档 tag：<1% 正常 → <5% 关注 → 其余告警 */
function errorRateTag(rate?: number) {
  const r = rate ?? 0;
  if (r >= 5) return { color: 'error', text: fmtRate(r) };
  if (r >= 1) return { color: 'warning', text: fmtRate(r) };
  return { color: 'success', text: fmtRate(r) };
}

const dayOptions = [
  { label: $t('stats.llm_days7'), value: '7' },
  { label: $t('stats.llm_days14'), value: '14' },
  { label: $t('stats.llm_days30'), value: '30' },
];

const summaryCards = computed(() => [
  {
    hint: $t('stats.llm_todayRequestsHint'),
    key: 'requests',
    title: $t('stats.llm_todayRequests'),
    value: fmtInt(overview.value?.today?.requests),
  },
  {
    hint: $t('stats.llm_todayTokensHint'),
    key: 'tokens',
    title: $t('stats.llm_todayTokens'),
    value: fmtInt(overview.value?.today?.totalTokens),
  },
  {
    hint: $t('stats.llm_todaySpendHint'),
    key: 'spend',
    title: $t('stats.llm_todaySpend'),
    value: fmtMoney(overview.value?.today?.spend),
  },
  {
    hint: $t('stats.llm_todayErrorHint'),
    key: 'errors',
    tag: errorRateTag(overview.value?.today?.errorRate),
    title: $t('stats.llm_todayErrorRate'),
  },
]);

onMounted(() => {
  loadOverview();
  loadTrend();
  loadTop();
});
</script>

<template>
  <Page :auto-content-height="true">
    <div class="space-y-4">
      <!-- 概览卡（今日口径） -->
      <Spin :spinning="loading">
        <div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <Card v-for="c in summaryCards" :key="c.key" size="small">
            <div class="flex flex-col gap-1">
              <span class="text-xs opacity-60">{{ c.title }}</span>
              <span class="text-2xl font-semibold">{{ c.value }}</span>
              <span v-if="c.tag" class="flex items-center gap-1 text-xs">
                <Tag :color="c.tag.color" class="!mr-0">
                  {{ c.tag.text }}
                </Tag>
              </span>
              <span v-if="c.hint" class="text-xs opacity-40">
                {{ c.hint }}
              </span>
            </div>
          </Card>
        </div>
      </Spin>

      <!-- 用量趋势 -->
      <Card>
        <template #title>
          <span>{{ $t('stats.llm_trendTitle') }}</span>
        </template>
        <template #extra>
          <RadioGroup
            :value="trendDays"
            size="small"
            option-type="button"
            :options="dayOptions"
            @change="onTrendDaysChange"
          />
        </template>
        <Spin :spinning="trendLoading">
          <Empty
            v-if="!trendLoading && trendPoints.length === 0"
            :description="$t('stats.llm_trendEmpty')"
            class="py-12"
          />
          <div v-else class="h-[320px]">
            <EchartsUI ref="chartRef" height="100%" />
          </div>
        </Spin>
      </Card>

      <!-- Top 榜 + 健康状态 -->
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-4">
        <Card class="xl:col-span-3">
          <template #title>
            <span>{{ $t('stats.llm_topTitle') }}</span>
          </template>
          <template #extra>
            <RadioGroup
              :value="topDays"
              size="small"
              option-type="button"
              :options="dayOptions"
              @change="onTopDaysChange"
            />
          </template>
          <RadioGroup
            :value="topDimension"
            class="mb-3"
            size="small"
            option-type="button"
            @change="(e: any) => onTopDimensionChange(e.target.value)"
          >
            <RadioGroup.Button value="model">
              {{ $t('stats.llm_topModel') }}
            </RadioGroup.Button>
            <RadioGroup.Button value="virtualKey">
              {{ $t('stats.llm_topVirtualKey') }}
            </RadioGroup.Button>
            <RadioGroup.Button value="user">
              {{ $t('stats.llm_topUser') }}
            </RadioGroup.Button>
          </RadioGroup>
          <Table
            :columns="topColumns"
            :data-source="topList"
            :loading="topLoading"
            :pagination="false"
            row-key="key"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'spend'">
                {{ fmtMoney(record.spend) }}
              </template>
              <template v-else-if="column.dataIndex === 'totalTokens'">
                {{ fmtInt(record.totalTokens) }}
              </template>
              <template v-else-if="column.dataIndex === 'requests'">
                {{ fmtInt(record.requests) }}
              </template>
              <template v-else-if="column.dataIndex === 'errorCount'">
                {{ fmtInt(record.errorCount) }}
              </template>
              <template v-else-if="column.dataIndex === 'errorRate'">
                <Tag :color="errorRateTag(record.errorRate).color">
                  {{ errorRateTag(record.errorRate).text }}
                </Tag>
              </template>
            </template>
          </Table>
        </Card>

        <!-- 凭证健康卡 -->
        <Card>
          <template #title>
            <span>{{ $t('stats.llm_healthTitle') }}</span>
          </template>
          <template #extra>
            <Button size="small" :loading="loading" @click="loadOverview">
              {{ $t('stats.llm_refresh') }}
            </Button>
          </template>
          <div class="flex flex-col gap-3 text-sm">
            <div class="flex items-center justify-between">
              <span class="opacity-60">{{ $t('stats.llm_healthTotal') }}</span>
              <b>{{ fmtInt(overview?.health?.totalCredentials) }}</b>
            </div>
            <div class="flex items-center justify-between">
              <span class="opacity-60">{{ $t('stats.llm_healthHealthy') }}</span>
              <b class="text-emerald-500">
                {{ fmtInt(overview?.health?.healthyCredentials) }}
              </b>
            </div>
            <div class="flex items-center justify-between">
              <span class="opacity-60">
                {{ $t('stats.llm_healthUnhealthy') }}
              </span>
              <b class="text-red-500">
                {{ fmtInt(overview?.health?.unhealthyCredentials) }}
              </b>
            </div>
            <div class="flex items-center justify-between">
              <span class="opacity-60">{{ $t('stats.llm_healthLatency') }}</span>
              <b>{{ fmtInt(overview?.health?.avgLatencyMs) }} ms</b>
            </div>
            <div class="text-xs opacity-40">
              {{ $t('stats.llm_healthCheckedAt') }}:
              {{ fmtTime(overview?.health?.checkedAt) }}
            </div>
            <div class="text-xs opacity-40">
              {{ $t('stats.llm_window7Hint') }}
              {{ $t('stats.llm_window7Value', { n: fmtInt(overview?.last7Days?.requests) }) }}
              ·
              {{ $t('stats.llm_window7Spend', { amount: fmtMoney(overview?.last7Days?.spend) }) }}
            </div>
          </div>
        </Card>
      </div>
    </div>
  </Page>
</template>
