<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';
import { useUserStore } from '@vben/stores';

import { Button, Descriptions, DescriptionsItem, Modal } from 'ant-design-vue';

import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { getClientStatusListApi } from '#/api/system/client/status';
import { getClientDeviceListApi } from '#/api/system/client/device';

defineOptions({ name: 'SystemClientStatus' });

// 当前登录用户下的设备（下拉选项）：label=设备名，value=deviceId
const userStore = useUserStore();
const deviceOptions = ref<{ label: string; value: string }[]>([]);

async function loadDevices() {
  try {
    const res = await getClientDeviceListApi();
    const uid = userStore.userInfo?.userId ?? '';
    deviceOptions.value = (res?.list ?? [])
      .filter((d) => d.userId === uid)
      .map((d) => ({ label: d.name || d.deviceId, value: d.deviceId }));
  } catch {
    deviceOptions.value = [];
  }
}
onMounted(loadDevices);

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select',
    fieldName: 'deviceId',
    label: $t('client.device_name'),
    componentProps: {
      allowClear: true,
      showSearch: true,
      filterable: true,
      options: deviceOptions.value,
      placeholder: $t('client.device_name'),
    },
  },
  {
    component: 'Select',
    fieldName: 'agent',
    label: $t('client.status_agent'),
    componentProps: {
      allowClear: true,
      options: [
        { label: 'openclaw', value: 'openclaw' },
        { label: 'hermes', value: 'hermes' },
        { label: 'obsidian', value: 'obsidian' },
      ],
    },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('client.status_status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('client.status_running'), value: 'running' },
        { label: $t('client.status_stopped'), value: 'stopped' },
        { label: $t('client.status_installing'), value: 'installing' },
        { label: $t('client.status_error'), value: 'error' },
        { label: $t('client.status_notinstalled'), value: 'notinstalled' },
      ],
    },
  },
]);

// query 运行时通过 gridApiRef 读表单当前值（不依赖 wrapper 注入的 form/latestSubmission，
// 这样刷新/内置刷新/submitOnChange 都用同一份当前表单值，刷新必保留搜索条件）
let gridApiRef: any;
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('client.status_seq'), width: 50, fixed: 'left' },
    { field: 'agent', title: $t('client.status_agent'), width: 120 },
    {
      field: 'status',
      title: $t('client.status_status'),
      width: 120,
      slots: { default: 'status' },
    },
    { field: 'version', title: $t('client.status_version'), width: 110 },
    {
      field: 'uptimeSeconds',
      title: $t('client.status_uptime'),
      width: 150,
      slots: { default: 'uptime' },
    },
    {
      field: 'lastError',
      title: $t('client.status_lastError'),
      minWidth: 200,
      showOverflow: true,
    },
    {
      field: 'extra',
      title: $t('client.status_command'),
      minWidth: 200,
      showOverflow: true,
      slots: { default: 'command' },
    },
    { field: 'reportedAt', title: $t('client.status_reportedAt'), width: 170 },
    { field: 'createdAt', title: $t('client.status_createdAt'), width: 170 },
    {
      field: 'action',
      title: $t('client.status_action'),
      width: 80,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async (params: any) => {
        const form = (await gridApiRef?.formApi?.getValues?.()) ?? {};
        const f = form as { deviceId?: string; agent?: string; status?: string };
        const res = await getClientStatusListApi();
        const allRows = res?.list ?? [];
        // App 自身心跳（kind=app / agent=client）是设备级属性，在设备页展示，
        // 不进本页 agent 状态表格。
        const all = allRows
          .filter((r) => r.kind !== 'app' && r.agent !== 'client')
          .filter((r) => {
            if (f.deviceId && r.deviceId !== f.deviceId) return false;
            if (f.agent && r.agent !== f.agent) return false;
            if (f.status && r.status !== f.status) return false;
            return true;
          })
          .sort((a, b) => (b.reportedAt ?? '').localeCompare(a.reportedAt ?? ''));
        // extra 是后端 jsonb 字符串（Go entity Extra string → JSON string），
        // 不解析则 row.extra.action 恒 undefined，#command 列永远显示 -。
        all.forEach((r) => { r.extra = parseExtra(r.extra); });
        // 前端分页（gin 不分页）：按 vxe-grid 传入的 page 切片
        const page = params?.page;
        const total = all.length;
        const start = page ? (page.currentPage - 1) * page.pageSize : 0;
        const pageSize = page ? page.pageSize : total;
        const items = all.slice(start, start + pageSize);
        return { items, total };
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
  formOptions: { schema: searchSchema, submitOnChange: true },
});
gridApiRef = gridApi;

// 按需刷新：不做定时轮询（每 3s query 会让页面一直跳动）。点右上角刷新图标或改动搜索条件才刷新。


// ===== 状态 Tag：颜色 + 本地化文案 =====
function statusColor(s: string): string {
  switch (s) {
    case 'running': {
      return 'magenta';
    }
    case 'installing': {
      return 'warning';
    }
    case 'error': {
      return 'error';
    }
    default: {
      return 'default';
    }
  }
}

function statusLabel(s: string): string {
  switch (s) {
    case 'running': {
      return $t('client.status_running');
    }
    case 'stopped': {
      return $t('client.status_stopped');
    }
    case 'installing': {
      return $t('client.status_installing');
    }
    case 'error': {
      return $t('client.status_error');
    }
    case 'notinstalled': {
      return $t('client.status_notinstalled');
    }
    default: {
      return s;
    }
  }
}

// 命令动作中文标签
function actionLabel(a: string | undefined): string {
  switch (a) {
    case 'install': { return '安装'; }
    case 'uninstall': { return '卸载'; }
    case 'start': { return '启动'; }
    case 'stop': { return '停止'; }
    case 'restart': { return '重启'; }
    case 'config_push': { return '下发配置'; }
    default: { return a || ''; }
  }
}
function cmdStatusColor(s: string | undefined): string {
  switch (s) {
    case 'running': { return 'processing'; }
    case 'done': { return 'magenta'; }
    case 'failed': { return 'error'; }
    default: { return 'default'; }
  }
}
function cmdStatusLabel(s: string | undefined): string {
  switch (s) {
    case 'running': { return '执行中'; }
    case 'done': { return '成功'; }
    case 'failed': { return '失败'; }
    default: { return s || ''; }
  }
}

// extra 是后端 jsonb 字符串（entity.ClientAgentStatus.Extra 为 Go string → JSON string），
// 需解析成对象模板才能读到 .action/.status/.lastLine/.error；参照 device 页 progressText 的兜底写法。
function parseExtra(raw: any): any {
  if (raw == null || raw === '' || raw === '{}' || raw === 'null') return null;
  if (typeof raw === 'object') return raw;
  try { return JSON.parse(raw); } catch { return null; }
}

// ===== 详情弹窗 =====
const detailVisible = ref(false);
const detailRow = ref<any>({});
function onViewDetail(row: any) {
  detailRow.value = row;
  detailVisible.value = true;
}
function uptimeText(s: number | undefined): string {
  if (!s) return '-';
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return `${d}天${h}时${m}分${sec}秒`;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${statusColor(row.status)}`]">{{ statusLabel(row.status) }}</span>
      </template>
      <template #uptime="{ row }">
        <span :class="{ 'cmd-empty': !row.uptimeSeconds }">{{ uptimeText(row.uptimeSeconds) }}</span>
      </template>
      <template #command="{ row }">
        <template v-if="row.extra && row.extra.action">
          <span :class="['aa-enum', `aa-enum--${cmdStatusColor(row.extra.status)}`, 'cmd-tag']">
            {{ actionLabel(row.extra.action) }}·{{ cmdStatusLabel(row.extra.status) }}
          </span>
          <span v-if="row.extra.lastLine" class="cmd-line">{{ row.extra.lastLine }}</span>
          <span v-else-if="row.extra.error" class="cmd-err">{{ row.extra.error }}</span>
        </template>
        <span v-else class="cmd-empty">-</span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="onViewDetail(row)">
          {{ $t('client.status_detail') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="detailVisible"
      :title="$t('client.status_detailTitle')"
      :width="640"
      :footer="null"
    >
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('client.status_agent')">
          {{ detailRow.agent }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_status')">
          <span :class="['aa-enum', `aa-enum--${statusColor(detailRow.status)}`]">
            {{ statusLabel(detailRow.status) }}
          </span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_version')">
          {{ detailRow.version || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_uptime')">
          {{ uptimeText(detailRow.uptimeSeconds) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_lastError')">
          <span style="white-space: pre-wrap; word-break: break-all">{{
            detailRow.lastError || '-'
          }}</span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_reportedAt')">
          {{ detailRow.reportedAt }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('client.status_createdAt')">
          {{ detailRow.createdAt }}
        </DescriptionsItem>
        <DescriptionsItem label="deviceId">
          {{ detailRow.deviceId }}
        </DescriptionsItem>
        <DescriptionsItem label="statusId">
          {{ detailRow.statusId }}
        </DescriptionsItem>
      </Descriptions>
    </Modal>
  </Page>
</template>

<style scoped>
.cmd-tag {
  margin-right: 6px;
}
.cmd-line {
  color: #595959;
  font-size: 12px;
}
.cmd-err {
  color: #ff4d4f;
  font-size: 12px;
}
.cmd-empty {
  color: #bfbfbf;
}
html.dark .cmd-line {
  color: #c9c9c9;
}
</style>
