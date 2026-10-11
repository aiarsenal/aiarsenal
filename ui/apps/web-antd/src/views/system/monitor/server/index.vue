<script lang="ts" setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Progress,
  Row,
  Statistic,
  Table,
} from 'ant-design-vue';

import { $t } from '#/locales';
import { getServerInfoApi } from '#/api/system/monitor/server';
import type { MonitorServerApi } from '#/api/system/monitor/server';

defineOptions({ name: 'SystemMonitorServer' });

const info = ref<Partial<MonitorServerApi.ServerInfo>>({});
const loading = ref(false);
let timer: ReturnType<typeof setInterval> | null = null;

// showLoading=true 仅首次加载显示骨架；定时轮询传 false → 静默刷新数据，页面/卡片不闪。
async function load(showLoading = false) {
  if (showLoading) loading.value = true;
  try {
    info.value = await getServerInfoApi();
  } finally {
    if (showLoading) loading.value = false;
  }
}

onMounted(() => {
  load(true);
  // 每 5 秒静默刷新数据（不显示骨架、不重新渲染整卡，仅响应式更新数值）
  timer = setInterval(() => load(false), 5000);
});

onUnmounted(() => {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
});

// ===== 单位换算（bytes → 人类可读）=====
function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return '-';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(2)} ${units[i]}`;
}

// uptime 秒 → "x天 y时 z分"
function formatUptime(sec?: number): string {
  if (sec === undefined || sec === null || Number.isNaN(sec)) return '-';
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  const parts: string[] = [];
  if (d > 0) parts.push(`${d}${$t('monitor.server_day')}`);
  if (h > 0) parts.push(`${h}${$t('monitor.server_hour')}`);
  if (m > 0) parts.push(`${m}${$t('monitor.server_minute')}`);
  parts.push(`${s}${$t('monitor.server_second')}`);
  return parts.join(' ');
}

function formatBootTime(ts?: number): string {
  if (!ts) return '-';
  const d = new Date(ts * 1000);
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString();
}

// ===== 派生展示值 =====
const cpuPercent = computed(() => Number(info.value.cpuUsed ?? 0));
const memPercent = computed(() => Number(info.value.memPercent ?? 0));
const diskPercent = computed(() => Number(info.value.diskPercent ?? 0));

const diskRows = computed(() => {
  const d = info.value;
  if (d?.diskTotal === undefined) return [];
  return [
    {
      key: '/',
      path: '/',
      total: d.diskTotal ?? 0,
      used: d.diskUsed ?? 0,
      free: (d.diskTotal ?? 0) - (d.diskUsed ?? 0),
      percent: d.diskPercent ?? 0,
    },
  ];
});

const diskColumns = computed(() => [
  { title: $t('monitor.server_mountPath'), dataIndex: 'path', key: 'path' },
  { title: $t('monitor.server_total'), dataIndex: 'total', key: 'total' },
  { title: $t('monitor.server_used'), dataIndex: 'used', key: 'used' },
  { title: $t('monitor.server_free'), dataIndex: 'free', key: 'free' },
  { title: $t('monitor.server_usage'), dataIndex: 'percent', key: 'percent' },
]);
</script>

<template>
  <Page :content-style="{ padding: '16px' }">
    <Row :gutter="[16, 16]">
      <!-- 运行时信息 -->
      <Col :xs="24" :md="12">
        <Card
          :title="$t('monitor.server_runtimeInfo')"
          :loading="loading"
          class="mod-card"
          :style="{ '--c': '#6366f1' }"
        >
          <Descriptions :column="1" size="small" bordered>
            <DescriptionsItem :label="$t('monitor.server_hostName')">
              {{ info.hostName ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_os')">
              {{ info.os ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_arch')">
              {{ info.kernelArch ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_kernelVersion')">
              {{ info.kernelVersion ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_goVersion')">
              {{ info.goVersion ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_goroutines')">
              {{ info.goroutines ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_pid')">
              {{ info.pid ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_startTime')">
              {{ info.startTime ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_runTime')">
              {{ info.runTime ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_uptime')">
              {{ formatUptime(info.uptime) }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('monitor.server_bootTime')">
              {{ formatBootTime(info.bootTime) }}
            </DescriptionsItem>
          </Descriptions>
        </Card>
      </Col>

      <!-- CPU -->
      <Col :xs="24" :md="12">
        <Card
          :title="$t('monitor.server_cpu')"
          :loading="loading"
          class="mod-card"
          :style="{ '--c': '#06b6d4' }"
        >
          <Row :gutter="16" align="middle">
            <Col :xs="24" :md="10" style="text-align: center">
              <Progress
                type="circle"
                :percent="cpuPercent"
                :format="() => `${cpuPercent.toFixed(2)}%`"
              />
            </Col>
            <Col :xs="24" :md="14">
              <Descriptions :column="1" size="small" bordered>
                <DescriptionsItem :label="$t('monitor.server_cpuNum')">
                  {{ info.cpuNum ?? '-' }}
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_cpuUsage')">
                  <span :class="['aa-enum', `aa-enum--${cpuPercent > 80 ? 'red' : 'magenta'}`]">
                    {{ cpuPercent.toFixed(2) }}%
                  </span>
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_cpuAvg5')">
                  {{ info.cpuAvg5 ?? '-' }}
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_cpuAvg15')">
                  {{ info.cpuAvg15 ?? '-' }}
                </DescriptionsItem>
              </Descriptions>
            </Col>
          </Row>
        </Card>
      </Col>

      <!-- 内存 -->
      <Col :xs="24" :md="12">
        <Card
          :title="$t('monitor.server_memory')"
          :loading="loading"
          class="mod-card"
          :style="{ '--c': '#10b981' }"
        >
          <Row :gutter="16" align="middle">
            <Col :xs="24" :md="10" style="text-align: center">
              <Progress
                type="circle"
                :percent="memPercent"
                :format="() => `${memPercent.toFixed(2)}%`"
              />
            </Col>
            <Col :xs="24" :md="14">
              <Descriptions :column="1" size="small" bordered>
                <DescriptionsItem :label="$t('monitor.server_total')">
                  {{ formatBytes(info.memTotal) }}
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_used')">
                  {{ formatBytes(info.memUsed) }}
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_free')">
                  {{ formatBytes((info.memTotal ?? 0) - (info.memUsed ?? 0)) }}
                </DescriptionsItem>
                <DescriptionsItem :label="$t('monitor.server_usage')">
                  <span :class="['aa-enum', `aa-enum--${memPercent > 80 ? 'red' : 'magenta'}`]">
                    {{ memPercent.toFixed(2) }}%
                  </span>
                </DescriptionsItem>
              </Descriptions>
            </Col>
          </Row>
        </Card>
      </Col>

      <!-- 磁盘 -->
      <Col :xs="24" :md="12">
        <Card
          :title="$t('monitor.server_disk')"
          :loading="loading"
          class="mod-card"
          :style="{ '--c': '#3b82f6' }"
        >
          <Row :gutter="16" align="middle" style="margin-bottom: 12px">
            <Col :xs="24" :md="10" style="text-align: center">
              <Progress
                type="circle"
                :percent="diskPercent"
                :format="() => `${diskPercent.toFixed(2)}%`"
              />
            </Col>
            <Col :xs="24" :md="14">
              <Statistic
                :title="$t('monitor.server_rootPartitionUsage')"
                :value="diskPercent"
                :precision="2"
                suffix="%"
              />
            </Col>
          </Row>
          <Table
            :columns="diskColumns"
            :data-source="diskRows"
            :pagination="false"
            size="small"
            :loading="loading"
            bordered
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'total'">
                {{ formatBytes(record.total) }}
              </template>
              <template v-else-if="column.dataIndex === 'used'">
                {{ formatBytes(record.used) }}
              </template>
              <template v-else-if="column.dataIndex === 'free'">
                {{ formatBytes(record.free) }}
              </template>
              <template v-else-if="column.dataIndex === 'percent'">
                <span :class="['aa-enum', `aa-enum--${record.percent > 80 ? 'red' : 'magenta'}`]">
                  {{ Number(record.percent).toFixed(2) }}%
                </span>
              </template>
            </template>
          </Table>
        </Card>
      </Col>
    </Row>
  </Page>
</template>

<style scoped>
/* 各模块参考 landing【技术栈】卡片：透明基 + 浅彩渐变填充(主色 8%) + 半透主色边框(30%)，
   hover 边框加深(55%) + 主色发光投影 + 上浮 3px。--c 由各卡 inline 注入。
   标题框取主色浅底 + 主色标题字，给每个模块色彩标识。整体紧凑（小 padding、小标题框）。*/
.mod-card {
  --c: #6366f1;
  height: 100%;
  border-color: color-mix(in srgb, var(--c) 30%, transparent) !important;
  background-image: linear-gradient(
    160deg,
    color-mix(in srgb, var(--c) 8%, transparent),
    transparent 60%
  ) !important;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}
.mod-card :deep(.ant-card-head) {
  min-height: 38px;
  padding: 0 14px;
  background: color-mix(in srgb, var(--c) 14%, transparent) !important;
  border-bottom: 1px solid color-mix(in srgb, var(--c) 22%, transparent);
}
.mod-card :deep(.ant-card-head-title) {
  padding: 8px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--c) !important;
}
.mod-card :deep(.ant-card-body) {
  padding: 12px 14px;
}
.mod-card:hover {
  border-color: color-mix(in srgb, var(--c) 55%, transparent) !important;
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c) 45%, transparent);
  transform: translateY(-3px);
}
</style>
