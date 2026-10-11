<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Descriptions, DescriptionsItem, message, Modal } from 'ant-design-vue';
import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  clearOperLogApi,
  deleteOperLogApi,
  getOperLogApi,
  getOperLogListApi,
} from '#/api/system/monitor/operLog';

defineOptions({ name: 'SystemMonitorOperLog' });

// ===== 请求方式字典（sys_oper_log_type: GET/POST/PUT/DELETE）=====
const requestMethodOptions = computed(() => [
  { label: $t('monitor.operLog_methodGet'), value: 'GET' },
  { label: $t('monitor.operLog_methodPost'), value: 'POST' },
  { label: $t('monitor.operLog_methodPut'), value: 'PUT' },
  { label: $t('monitor.operLog_methodDelete'), value: 'DELETE' },
]);

function requestMethodTag(method: string) {
  const map: Record<string, string> = {
    GET: 'blue',
    POST: 'magenta',
    PUT: 'orange',
    DELETE: 'red',
  };
  const opt = requestMethodOptions.value.find((o) => o.value === method);
  return { color: map[method] ?? 'default', text: opt ? `${method}（${opt.label}）` : method };
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'title',
    label: $t('monitor.operLog_module'),
    componentProps: { placeholder: $t('monitor.operLog_modulePlaceholder'), allowClear: true },
  },
  {
    component: 'Select',
    fieldName: 'requestMethod',
    label: $t('monitor.operLog_requestMethod'),
    componentProps: {
      allowClear: true,
      options: requestMethodOptions.value,
      placeholder: $t('monitor.operLog_requestMethodPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Input',
    fieldName: 'operName',
    label: $t('monitor.operLog_operName'),
    componentProps: { placeholder: $t('monitor.operLog_operNamePlaceholder'), allowClear: true },
  },
  {
    component: 'RangePicker',
    fieldName: 'dateRange',
    label: $t('monitor.operLog_operTime'),
    componentProps: {
      valueFormat: 'YYYY-MM-DD',
      style: { width: '100%' },
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('monitor.operLog_seq'), width: 60 },
    { field: 'operId', title: $t('monitor.operLog_logId'), minWidth: 160 },
    { field: 'title', title: $t('monitor.operLog_module'), minWidth: 140 },
    {
      field: 'requestMethod',
      title: $t('monitor.operLog_requestMethod'),
      width: 130,
      slots: { default: 'requestMethod' },
    },
    { field: 'operName', title: $t('monitor.operLog_operName'), minWidth: 120 },
    { field: 'deptName', title: $t('monitor.operLog_deptName'), minWidth: 120 },
    { field: 'operUrl', title: $t('monitor.operLog_operUrl'), minWidth: 260 },
    { field: 'operIp', title: $t('monitor.operLog_operIp'), minWidth: 130 },
    { field: 'operLocation', title: $t('monitor.operLog_operLocation'), minWidth: 120 },
    { field: 'operTime', title: $t('monitor.operLog_operTime'), minWidth: 170 },
    {
      field: 'action',
      title: $t('monitor.operLog_action'),
      width: 140,
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
          title: form?.title,
          requestMethod: form?.requestMethod,
          operName: form?.operName,
        };
        // RangePicker valueFormat=YYYY-MM-DD 时返回字符串数组；兜底格式化
        if (Array.isArray(form?.dateRange) && form.dateRange.length === 2) {
          const [s, e] = form.dateRange;
          params.dateRange = [
            dayjs(s).format('YYYY-MM-DD'),
            dayjs(e).format('YYYY-MM-DD'),
          ];
        }
        const res = await getOperLogListApi(params);
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
  formOptions: {
    schema: searchSchema,
  },
});

// ===== 删除（批量 / 单行）=====
const deleting = ref(false);

async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('monitor.operLog_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.operId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.operId], 1);
}

async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('monitor.operLog_confirmTitle'),
    content: $t('monitor.operLog_confirmDeleteContent', { count }),
    okText: $t('monitor.operLog_confirm'),
    cancelText: $t('monitor.operLog_cancel'),
    okType: 'danger',
    onOk: async () => {
      deleting.value = true;
      try {
        await deleteOperLogApi(ids);
        message.success($t('monitor.operLog_deleteSuccess'));
        gridApi.query();
      } catch {
        /* 拦截器已提示 */
      } finally {
        deleting.value = false;
      }
    },
  });
}

// ===== 清空日志 =====
async function onClear() {
  Modal.confirm({
    title: $t('monitor.operLog_confirmTitle'),
    content: $t('monitor.operLog_clearConfirmContent'),
    okText: $t('monitor.operLog_confirm'),
    cancelText: $t('monitor.operLog_cancel'),
    okType: 'danger',
    onOk: async () => {
      try {
        await clearOperLogApi();
        message.success($t('monitor.operLog_clearSuccess'));
        gridApi.query();
      } catch {
        /* 拦截器已提示 */
      }
    },
  });
}

// ===== 详情抽屉 =====
const detailVisible = ref(false);
const detailLoading = ref(false);
const detailData = ref<any>({});

function formatTime(t?: string) {
  if (!t) return '-';
  const d = dayjs(t);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : t;
}

async function onView(row: any) {
  detailVisible.value = true;
  detailLoading.value = true;
  detailData.value = {};
  try {
    const res = await getOperLogApi(row.operId);
    detailData.value = res ?? {};
  } catch {
    /* 拦截器已提示 */
  } finally {
    detailLoading.value = false;
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" :loading="deleting" @click="onDeleteBatch">
          {{ $t('monitor.operLog_batchDelete') }}
        </Button>
        <Button danger class="ml-2" @click="onClear">
          {{ $t('monitor.operLog_clearLog') }}
        </Button>
      </template>
      <template #requestMethod="{ row }">
        <span :class="['aa-enum', `aa-enum--${requestMethodTag(row.requestMethod).color}`]">
          {{ requestMethodTag(row.requestMethod).text }}
        </span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="onView(row)">
          {{ $t('monitor.operLog_view') }}
        </Button>
        <Button
          type="primary" ghost
          size="small"
          danger
          :loading="deleting"
          @click="onDeleteRow(row)"
        >
          {{ $t('monitor.operLog_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="detailVisible"
      :title="$t('monitor.operLog_detailTitle')"
      :width="820"
      :footer="null"
      :destroy-on-close="true"
    >
      <Descriptions
        :column="2"
        bordered
        size="small"
        :loading="detailLoading"
      >
        <DescriptionsItem :label="$t('monitor.operLog_logId')">
          {{ detailData.operId ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_module')">
          {{ detailData.title ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_detailMethod')">
          {{ detailData.method ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_requestMethod')">
          <span
            v-if="detailData.requestMethod"
            :class="['aa-enum', `aa-enum--${requestMethodTag(detailData.requestMethod).color}`]"
          >
            {{ requestMethodTag(detailData.requestMethod).text }}
          </span>
          <span v-else>-</span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_operName')">
          {{ detailData.operName ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_deptName')">
          {{ detailData.deptName ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_operUrl')" :span="2">
          {{ detailData.operUrl ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_operIp')">
          {{ detailData.operIp ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_operLocation')">
          {{ detailData.operLocation ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_detailOperParam')" :span="2">
          {{ detailData.operParam ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_detailErrorMsg')" :span="2">
          {{ detailData.errorMsg ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('monitor.operLog_operTime')" :span="2">
          {{ formatTime(detailData.operTime) }}
        </DescriptionsItem>
      </Descriptions>
    </Modal>
  </Page>
</template>
