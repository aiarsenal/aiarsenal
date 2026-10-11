<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Card, Col, Row, Spin, Statistic } from 'ant-design-vue';

import { $t } from '#/locales';
import { getAgentDashboardOverviewApi } from '#/api/system/agent/dashboard';

defineOptions({ name: 'AgentDashboard' });

const loading = ref(true);
const overview = ref({
  instanceTotal: 0,
  instanceRunning: 0,
  teamTotal: 0,
  modelTotal: 0,
  scanJobActive: 0,
  invocation24h: 0,
  cost24h: 0,
});

// 统计卡，主色按 landing【技术栈】色板逐一对应（--c 驱动卡片配色 + hover 交互）。
const cards = computed(() => [
  { key: 'instanceTotal', color: '#6366f1' },
  { key: 'instanceRunning', color: '#06b6d4' },
  { key: 'teamTotal', color: '#10b981' },
  { key: 'modelTotal', color: '#ef4444' },
  { key: 'scanJobActive', color: '#42b983' },
  { key: 'invocation24h', color: '#8b5cf6' },
  { key: 'cost24h', color: '#f59e0b', precision: 2 },
]);

function valueOf(key: string): number {
  return (overview.value as Record<string, number>)[key] ?? 0;
}

async function load() {
  loading.value = true;
  try {
    const res = await getAgentDashboardOverviewApi();
    if (res) overview.value = res;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page :auto-content-height="true">
    <Spin :spinning="loading">
      <div class="mb-4 text-lg font-semibold">
        {{ $t('agent.dashboard_title') }}
      </div>
      <Row :gutter="[16, 16]">
        <Col v-for="c in cards" :key="c.key" :span="6">
          <Card class="stat-card" :style="{ '--c': c.color }">
            <Statistic
              :title="$t(`agent.dashboard_${c.key}`)"
              :value="valueOf(c.key)"
              :precision="c.precision"
              :value-style="{ color: c.color }"
            />
          </Card>
        </Col>
      </Row>
    </Spin>
  </Page>
</template>

<style scoped>
/* 统计卡完全参考 landing【技术栈】卡片：实色基 + 浅彩渐变填充(主色 8%) + 半透主色边框(30%)，
   hover 边框加深(55%) + 主色发光投影 + 上浮 3px，平滑过渡。--c 由各卡 inline 注入。
   antd Card 经 cssinjs 注入底色/边框，故 background-image 与 border-color 用 !important 覆盖。 */
.stat-card {
  --c: #6366f1;
  border-color: color-mix(in srgb, var(--c) 30%, transparent) !important;
  background-image: linear-gradient(
    160deg,
    color-mix(in srgb, var(--c) 8%, transparent),
    transparent 60%
  ) !important;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}
.stat-card:hover {
  border-color: color-mix(in srgb, var(--c) 55%, transparent) !important;
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c) 45%, transparent);
  transform: translateY(-3px);
}
</style>
