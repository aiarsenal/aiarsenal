<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, ref } from 'vue';

import { Page } from '@vben/common-ui';
import { Button, Descriptions, DescriptionsItem, message, Modal, Table } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  getSkillAuditListApi,
  rescanSkillAuditApi,
} from '#/api/system/skill/security';

defineOptions({ name: 'SystemSkillSecurity' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'versionId',
    label: $t('skill.sec_versionId'),
    componentProps: { placeholder: $t('skill.sec_versionIdPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('skill.sec_status'),
    componentProps: {
      allowClear: true,
      style: { width: '100%' },
      options: [
        { label: $t('skill.sec_statusPending'), value: 0 },
        { label: $t('skill.sec_statusPass'), value: 1 },
        { label: $t('skill.sec_statusFail'), value: 2 },
        { label: $t('skill.sec_statusScanning'), value: 3 },
        { label: $t('skill.sec_statusError'), value: 4 },
      ],
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('skill.sec_seq'), width: 60, fixed: 'left' },
    { field: 'auditId', title: $t('skill.sec_auditId'), minWidth: 180 },
    { field: 'skillVersionId', title: $t('skill.sec_skillVersionId'), minWidth: 180 },
    {
      field: 'status',
      title: $t('skill.sec_status'),
      width: 100,
      slots: { default: 'status' },
    },
    { field: 'severity', title: $t('skill.sec_severity'), width: 90 },
    { field: 'scannerRaw', title: $t('skill.sec_scanner'), minWidth: 140 },
    { field: 'scannedAt', title: $t('skill.sec_scannedAt'), minWidth: 160 },
    { field: 'createdAt', title: $t('skill.sec_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('skill.sec_action'),
      width: 170,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getSkillAuditListApi({
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

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

// ===== 重新扫描 =====
async function onRescan(row: any) {
  if (!row?.skillVersionId) {
    message.warning($t('skill.sec_versionIdMissing'));
    return;
  }
  try {
    await rescanSkillAuditApi(row.skillVersionId);
    message.success($t('skill.sec_rescanSubmitted'));
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 状态 tag 颜色（5 态：待扫描/扫描中/安全/不安全/扫描失败）
function statusTag(status: number) {
  if (status === 1) return { color: 'success', text: $t('skill.sec_statusPass') };
  if (status === 2) return { color: 'error', text: $t('skill.sec_statusFail') };
  if (status === 3) return { color: 'blue', text: $t('skill.sec_statusScanning') };
  if (status === 4) return { color: 'orange', text: $t('skill.sec_statusError') };
  return { color: 'default', text: $t('skill.sec_statusPending') };
}

// ===== 不安全原因（findingsJson：file/rule/severity/message 数组） =====
const findingsVisible = ref(false);
const findingsRow = ref<any>(null);
const findingsList = computed(() => {
  const raw = findingsRow.value?.findingsJson;
  if (!raw) return [];
  try {
    const arr = typeof raw === 'string' ? JSON.parse(raw) : raw;
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
});
// 严重度 0~4 本地化（INFO/LOW/MEDIUM/HIGH/CRITICAL）
function severityText(sev: number) {
  return (
    ['INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'][sev] ?? `SEV${sev}`
  );
}
function severityColor(sev: number) {
  if (sev >= 3) return 'error';
  if (sev === 2) return 'warning';
  return 'default';
}
function onShowFindings(row: any) {
  findingsRow.value = row;
  findingsVisible.value = true;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="gridApi.query()">{{ $t('skill.sec_refresh') }}</Button>
      </template>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${statusTag(row.status).color}`]">
          {{ statusTag(row.status).text }}
        </span>
      </template>
      <template #action="{ row }">
        <!-- 不安全/扫描失败时看原因：findings 明细 + scannerRaw -->
        <Button
          v-if="row.status === 2 || row.status === 4"
          type="primary" ghost
          size="small"
          @click="onShowFindings(row)"
        >
          {{ $t('skill.sec_findings') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onRescan(row)">
          {{ $t('skill.sec_rescan') }}
        </Button>
      </template>
    </Grid>

    <!-- 不安全原因弹窗：findings（文件/规则/严重度/说明）+ 扫描器原始输出 -->
    <Modal
      v-model:open="findingsVisible"
      :title="$t('skill.sec_findingsTitle')"
      :width="720"
      :footer="null"
    >
      <Descriptions v-if="findingsRow" :column="2" size="small" bordered class="mb-3">
        <DescriptionsItem :label="$t('skill.sec_skillVersionId')">
          {{ findingsRow.skillVersionId }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.sec_status')">
          <span :class="['aa-enum', `aa-enum--${statusTag(findingsRow.status).color}`]">
            {{ statusTag(findingsRow.status).text }}
          </span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.sec_severity')">
          {{ severityText(findingsRow.severity) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.sec_scannedAt')">
          {{ findingsRow.scannedAt || '—' }}
        </DescriptionsItem>
      </Descriptions>
      <div v-if="findingsList.length" class="findings-title">
        {{ $t('skill.sec_findings') }}（{{ findingsList.length }}）
      </div>
      <Table
        v-if="findingsList.length"
        :data-source="findingsList"
        :pagination="false"
        size="small"
        :columns="[
          { title: $t('skill.sec_findingFile'), dataIndex: 'file', ellipsis: true },
          { title: $t('skill.sec_findingRule'), dataIndex: 'rule', width: 150 },
          { title: $t('skill.sec_severity'), key: 'sev', width: 100 },
          { title: $t('skill.sec_findingMsg'), dataIndex: 'message', ellipsis: true },
        ]"
        :row-key="(_r: any, i: number) => i"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'sev'">
            <span :class="['aa-enum', `aa-enum--${severityColor(record.severity)}`]">
              {{ severityText(record.severity) }}
            </span>
          </template>
        </template>
      </Table>
      <div v-else class="findings-empty">{{ $t('skill.sec_findingsEmpty') }}</div>
      <div v-if="findingsRow?.scannerRaw" class="findings-raw">
        <div class="findings-title">scanner_raw</div>
        <pre class="findings-raw-pre">{{ findingsRow.scannerRaw }}</pre>
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.findings-title {
  margin: 8px 0 6px;
  font-weight: 600;
  font-size: 13px;
}
.findings-empty {
  padding: 16px 0;
  color: #8a94a6;
  text-align: center;
}
.findings-raw-pre {
  margin: 0;
  padding: 8px 10px;
  background: #1f2933;
  color: #c8d1dd;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.mb-3 {
  margin-bottom: 12px;
}
</style>
