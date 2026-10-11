<script lang="ts" setup>
import type { EchartsUIType } from '@vben/plugins/echarts';

import type { StatsSkillApi } from '#/api/system/stats/skill';

import { computed, onMounted, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Card, Empty, Progress, RadioGroup, Spin, Tag } from 'ant-design-vue';

import { EchartsUI, useEcharts } from '@vben/plugins/echarts';

import {
  getStatsSkillInstallTrendApi,
  getStatsSkillOverviewApi,
} from '#/api/system/stats/skill';
import { $t } from '#/locales';

defineOptions({ name: 'StatsSkillData' });

// ===== 总览 =====
const loading = ref(false);
const overview = ref<StatsSkillApi.Overview | undefined>();

async function loadOverview() {
  loading.value = true;
  try {
    overview.value = await getStatsSkillOverviewApi();
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

// ===== 安装趋势 =====
const trendDays = ref<'30' | '7' | '90'>('30');
const trendLoading = ref(false);
const trendPoints = ref<StatsSkillApi.InstallTrendPoint[]>([]);
const trendChartRef = ref<EchartsUIType>();
const { renderEcharts: renderTrendChart } = useEcharts(trendChartRef);

async function loadTrend() {
  trendLoading.value = true;
  try {
    const res = await getStatsSkillInstallTrendApi(Number(trendDays.value));
    trendPoints.value = res?.points ?? [];
  } catch {
    trendPoints.value = [];
  } finally {
    trendLoading.value = false;
  }
}

/** 安装（柱）+ 卸载（线）趋势 */
function renderTrend() {
  renderTrendChart({
    legend: {
      data: [$t('stats.skill_installs'), $t('stats.skill_uninstalls')],
    },
    series: [
      {
        data: trendPoints.value.map((p) => p.installs),
        itemStyle: { borderRadius: [3, 3, 0, 0] },
        name: $t('stats.skill_installs'),
        type: 'bar',
      },
      {
        areaStyle: { opacity: 0.1 },
        data: trendPoints.value.map((p) => p.uninstalls),
        itemStyle: { color: '#ef4444' },
        name: $t('stats.skill_uninstalls'),
        smooth: true,
        type: 'line',
      },
    ],
    tooltip: { trigger: 'axis' },
    xAxis: {
      axisTick: { show: false },
      data: trendPoints.value.map((p) => p.date),
      type: 'category',
    },
    yAxis: [{ axisTick: { show: false }, splitNumber: 4, type: 'value' }],
  });
}

watch(trendPoints, renderTrend);

function onTrendDaysChange(e: any) {
  trendDays.value = e.target.value;
  loadTrend();
}

// ===== 来源分布（饼图） =====
const pieChartRef = ref<EchartsUIType>();
const { renderEcharts: renderPieChart } = useEcharts(pieChartRef);

const PALETTE = ['#42b983', '#5ab1ef', '#f59e0b', '#f97316', '#a78bfa', '#38bdf8', '#f472b6'];

function renderPie() {
  const sources = overview.value?.sources ?? [];
  renderPieChart({
    legend: { orient: 'vertical', right: 0, top: 'middle' },
    series: [
      {
        avoidLabelOverlap: true,
        data: sources.map((s, i) => ({
          itemStyle: { color: PALETTE[i % PALETTE.length] },
          name: s.label || s.source,
          value: s.count,
        })),
        itemStyle: { borderRadius: 4, borderColor: '#fff', borderWidth: 1 },
        label: { show: false },
        radius: ['40%', '70%'],
        type: 'pie',
      },
    ],
    tooltip: { trigger: 'item' },
  });
}

watch(
  () => overview.value?.sources,
  () => renderPie(),
);

// ===== 展示 =====
function fmtInt(n?: number) {
  return Number.isFinite(n) ? (n as number).toLocaleString() : '-';
}

/** 安全扫描分档色：≥95% emerald → ≥80% amber → <80% red */
function securityColor(rate?: number) {
  const r = rate ?? 0;
  if (r >= 95) return '#10b981';
  if (r >= 80) return '#f59e0b';
  return '#ef4444';
}

const summaryCards = computed(() => [
  {
    key: 'total',
    title: $t('stats.skill_total'),
    value: fmtInt(overview.value?.totalSkills),
  },
  {
    key: 'published',
    title: $t('stats.skill_published'),
    value: fmtInt(overview.value?.publishedSkills),
  },
  {
    key: 'draft',
    title: $t('stats.skill_draft'),
    value: fmtInt(overview.value?.draftSkills),
  },
  {
    key: 'pending',
    tag:
      (overview.value?.pendingReview ?? 0) > 0
        ? { color: 'warning', text: fmtInt(overview.value?.pendingReview) }
        : undefined,
    title: $t('stats.skill_pendingReview'),
    value: fmtInt(overview.value?.pendingReview),
  },
  {
    key: 'installed',
    title: $t('stats.skill_installed'),
    value: fmtInt(overview.value?.installedCount),
  },
  {
    key: 'versions',
    title: $t('stats.skill_versions'),
    value: fmtInt(overview.value?.totalVersions),
  },
]);

const dayOptions = [
  { label: $t('stats.skill_days7'), value: '7' },
  { label: $t('stats.skill_days30'), value: '30' },
  { label: $t('stats.skill_days90'), value: '90' },
];

onMounted(() => {
  loadOverview();
  loadTrend();
});
</script>

<template>
  <Page :auto-content-height="true">
    <div class="space-y-4">
      <!-- 计数卡 -->
      <Spin :spinning="loading">
        <div class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-6">
          <Card v-for="c in summaryCards" :key="c.key" size="small">
            <div class="flex flex-col gap-1">
              <span class="text-xs opacity-60">{{ c.title }}</span>
              <span class="text-2xl font-semibold">{{ c.value }}</span>
              <span v-if="c.tag" class="flex items-center gap-1 text-xs">
                <Tag :color="c.tag.color" class="!mr-0">
                  {{ c.tag.text }}
                </Tag>
              </span>
            </div>
          </Card>
        </div>
      </Spin>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <!-- 安装趋势 -->
        <Card class="xl:col-span-2">
          <template #title>
            <span>{{ $t('stats.skill_trendTitle') }}</span>
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
              :description="$t('stats.skill_trendEmpty')"
              class="py-12"
            />
            <div v-else class="h-[300px]">
              <EchartsUI ref="trendChartRef" height="100%" />
            </div>
          </Spin>
        </Card>

        <!-- 安全扫描通过率 -->
        <Card>
          <template #title>
            <span>{{ $t('stats.skill_securityTitle') }}</span>
          </template>
          <template #extra>
            <Button size="small" :loading="loading" @click="loadOverview">
              {{ $t('stats.skill_refresh') }}
            </Button>
          </template>
          <div class="flex flex-col items-center gap-2 py-2">
            <Progress
              type="dashboard"
              :percent="Math.min(100, Math.round(overview?.security?.passRate ?? 0))"
              :stroke-color="securityColor(overview?.security?.passRate)"
              :format="(p?: number) => `${p ?? 0}%`"
            />
            <div class="grid w-full grid-cols-2 gap-x-6 gap-y-1 text-sm">
              <div class="flex items-center justify-between">
                <span class="opacity-60">{{ $t('stats.skill_secScanned') }}</span>
                <b>{{ fmtInt(overview?.security?.scanned) }}</b>
              </div>
              <div class="flex items-center justify-between">
                <span class="opacity-60">{{ $t('stats.skill_secPassed') }}</span>
                <b class="text-emerald-500">
                  {{ fmtInt(overview?.security?.passed) }}
                </b>
              </div>
              <div class="flex items-center justify-between">
                <span class="opacity-60">{{ $t('stats.skill_secFailed') }}</span>
                <b class="text-red-500">{{ fmtInt(overview?.security?.failed) }}</b>
              </div>
              <div class="flex items-center justify-between">
                <span class="opacity-60">{{ $t('stats.skill_secScanning') }}</span>
                <b>{{ fmtInt(overview?.security?.scanning) }}</b>
              </div>
            </div>
          </div>
        </Card>
      </div>

      <!-- 市场来源分布 -->
      <Card>
        <template #title>
          <span>{{ $t('stats.skill_sourceTitle') }}</span>
        </template>
        <Empty
          v-if="!loading && (overview?.sources?.length ?? 0) === 0"
          :description="$t('stats.skill_sourceEmpty')"
          class="py-12"
        />
        <div v-else class="h-[280px]">
          <EchartsUI ref="pieChartRef" height="100%" />
        </div>
      </Card>
    </div>
  </Page>
</template>
