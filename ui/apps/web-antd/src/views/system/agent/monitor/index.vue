<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import { computed, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Select } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  deleteAgentAuditLogApi,
  getAgentAuditEventListApi,
  getAgentAuditLogListApi,
  getAgentCostListApi,
  getAgentInvocationListApi,
  getAgentRiskHitListApi,
  type MonitorRecordType,
} from '#/api/system/agent/monitor';

defineOptions({ name: 'SystemAgentMonitor' });

// ===== 记录类型 =====
const TYPE_OPTIONS = computed<{ label: string; value: MonitorRecordType }[]>(() => [
  { label: $t('agent.monitor_type_invocation'), value: 'invocation' },
  { label: $t('agent.monitor_type_auditEvent'), value: 'audit-event' },
  { label: $t('agent.monitor_type_cost'), value: 'cost' },
  { label: $t('agent.monitor_type_riskHit'), value: 'risk-hit' },
  { label: $t('agent.monitor_type_auditLog'), value: 'audit-log' },
]);

const recordType = ref<MonitorRecordType>('invocation');
const userId = ref('');
const extraValue = ref(''); // modelId / eventType / ruleId / action

// 动态附加输入框 placeholder/label
const extraMeta = computed(() => {
  switch (recordType.value) {
    case 'invocation': {
      return { field: 'modelId', placeholder: $t('agent.monitor_extra_modelId') };
    }
    case 'audit-event': {
      return { field: 'eventType', placeholder: $t('agent.monitor_extra_eventType') };
    }
    case 'risk-hit': {
      return { field: 'ruleId', placeholder: $t('agent.monitor_extra_ruleId') };
    }
    case 'audit-log': {
      return { field: 'action', placeholder: $t('agent.monitor_extra_action') };
    }
    default: {
      return null;
    }
  }
});

// ===== 动态列 =====
const baseColumns = computed(() => {
  const t = recordType.value;
  const cols: VxeTableGridOptions['columns'] = [
    { type: 'seq', title: '#', width: 60, fixed: 'left' },
    { field: 'traceId', title: 'TraceID', minWidth: 200 },
    { field: 'userId', title: $t('agent.monitor_user'), width: 140 },
  ];
  if (t === 'invocation') {
    cols.push(
      { field: 'requestedModel', title: $t('agent.monitor_model'), minWidth: 180 },
      {
        field: 'tokens',
        title: 'Tokens',
        width: 140,
        slots: { default: 'tokens' },
      },
      { field: 'latencyMs', title: $t('agent.monitor_latency'), width: 100 },
    );
  } else if (t === 'audit-event') {
    cols.push(
      { field: 'eventType', title: $t('agent.monitor_event'), width: 120 },
      { field: 'message', title: $t('agent.monitor_message'), minWidth: 220 },
    );
  } else if (t === 'cost') {
    cols.push(
      { field: 'modelName', title: $t('agent.monitor_model'), minWidth: 180 },
      { field: 'estimatedCost', title: $t('agent.monitor_cost'), width: 100 },
    );
  } else if (t === 'risk-hit') {
    cols.push(
      { field: 'ruleName', title: $t('agent.monitor_rule'), minWidth: 180 },
      { field: 'action', title: $t('agent.monitor_action2'), width: 90 },
    );
  } else if (t === 'audit-log') {
    cols.push(
      { field: 'action', title: $t('agent.monitor_operation'), width: 120 },
      { field: 'resourceType', title: $t('agent.monitor_resource'), width: 120 },
    );
  }
  // cost 用 recordedAt，其余用 createdAt
  cols.push({
    field: t === 'cost' ? 'recordedAt' : 'createdAt',
    title: $t('agent.monitor_time'),
    width: 170,
  });
  if (t === 'audit-log') {
    cols.push({
      field: 'action-bar',
      title: $t('agent.monitor_operation'),
      width: 90,
      fixed: 'right',
      slots: { default: 'rowAction' },
    });
  }
  return cols;
});

// ===== 列表请求分发 =====
function pickListApi(t: MonitorRecordType) {
  switch (t) {
    case 'invocation': {
      return getAgentInvocationListApi;
    }
    case 'audit-event': {
      return getAgentAuditEventListApi;
    }
    case 'cost': {
      return getAgentCostListApi;
    }
    case 'risk-hit': {
      return getAgentRiskHitListApi;
    }
    default: {
      return getAgentAuditLogListApi;
    }
  }
}

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: baseColumns.value,
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page }: any) => {
        const t = recordType.value;
        const api = pickListApi(t);
        const params: any = {
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
        };
        if (userId.value) params.userId = userId.value;
        if (extraMeta.value && extraValue.value) {
          params[extraMeta.value.field] = extraValue.value;
        }
        const res = await api(params);
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: {
    custom: true,
    refresh: { code: 'query' },
  },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

// 列随类型变化重设
function applyColumns() {
  gridApi.grid?.loadColumn?.(baseColumns.value as any);
}

function onTypeChange(v: any) {
  recordType.value = v;
  extraValue.value = '';
  applyColumns();
  gridApi.query();
}

function onSearch() {
  gridApi.query();
}

// ===== 清理操作日志（批量：当前列表全部）=====
function onClearAuditLog() {
  const tableData = gridApi.grid?.getTableData?.() ?? {};
  const rows: any[] = tableData?.fullData ?? tableData ?? [];
  if (!rows.length) {
    message.warning($t('agent.monitor_emptyList'));
    return;
  }
  const ids = rows
    .map((r: any) => r.auditLogId)
    .filter((id: any) => !!id) as string[];
  if (!ids.length) {
    message.warning($t('agent.monitor_noCleanableId'));
    return;
  }
  Modal.confirm({
    title: $t('agent.monitor_clearConfirmTitle'),
    content: $t('agent.monitor_clearConfirmContent', { count: ids.length }),
    okText: $t('agent.monitor_confirm'),
    cancelText: $t('agent.monitor_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentAuditLogApi(ids);
      message.success($t('agent.monitor_cleared'));
      gridApi.query();
    },
  });
}

async function onDeleteRow(row: any) {
  if (!row.auditLogId) return;
  Modal.confirm({
    title: $t('agent.monitor_deleteRowConfirmTitle'),
    content: $t('agent.monitor_deleteRowConfirmContent'),
    okText: $t('agent.monitor_confirm'),
    cancelText: $t('agent.monitor_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentAuditLogApi([row.auditLogId]);
      message.success($t('agent.monitor_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <span class="mr-2">{{ $t('agent.monitor_recordType') }}</span>
        <Select
          :value="recordType"
          style="width: 160px"
          :options="TYPE_OPTIONS"
          @change="onTypeChange"
        />
        <span class="ml-3 mr-2">{{ $t('agent.monitor_userId') }}</span>
        <Input
          v-model:value="userId"
          style="width: 160px"
          :placeholder="$t('agent.monitor_userIdPlaceholder')"
          allow-clear
          @keyup.enter="onSearch"
        />
        <template v-if="extraMeta">
          <span class="ml-3 mr-2">{{ extraMeta.placeholder }}</span>
          <Input
            v-model:value="extraValue"
            style="width: 160px"
            :placeholder="extraMeta.placeholder"
            allow-clear
            @keyup.enter="onSearch"
          />
        </template>
        <Button class="ml-3" type="primary" @click="onSearch">
          {{ $t('agent.monitor_search') }}
        </Button>
        <Button
          v-if="recordType === 'audit-log'"
          class="ml-2"
          type="primary"
          danger
          @click="onClearAuditLog"
        >
          {{ $t('agent.monitor_clearLog') }}
        </Button>
      </template>
      <template #tokens="{ row }">
        {{ row.promptTokens || 0 }}/{{ row.completionTokens || 0 }}
      </template>
      <template #rowAction="{ row }">
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.monitor_delete') }}
        </Button>
      </template>
    </Grid>
  </Page>
</template>
