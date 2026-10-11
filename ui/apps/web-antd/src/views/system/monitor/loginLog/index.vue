<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, ref } from "vue";

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Tag } from 'ant-design-vue';
import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  clearLoginLogApi,
  deleteLoginLogApi,
  getLoginLogListApi,
} from '#/api/system/monitor/loginLog';

defineOptions({ name: 'SystemMonitorLoginLog' });

// ===== 登录状态字典（admin_login_status: "1"=成功 "0"=失败）=====
const statusOptions = computed(() => [
  { label: $t('monitor.loginLog_success'), value: '1' },
  { label: $t('monitor.loginLog_failure'), value: '0' },
]);

function statusTag(status: number) {
  return status === 1
    ? { color: 'magenta', text: $t('monitor.loginLog_success') }
    : { color: 'red', text: $t('monitor.loginLog_failure') };
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'ipaddr',
    label: $t('monitor.loginLog_ipaddr'),
    componentProps: { placeholder: $t('monitor.loginLog_ipaddrPlaceholder'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'loginLocation',
    label: $t('monitor.loginLog_loginLocation'),
    componentProps: { placeholder: $t('monitor.loginLog_loginLocationPlaceholder'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'userName',
    label: $t('monitor.loginLog_userName'),
    componentProps: { placeholder: $t('monitor.loginLog_userNamePlaceholder'), allowClear: true },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('monitor.loginLog_status'),
    componentProps: {
      allowClear: true,
      options: statusOptions.value,
      placeholder: $t('monitor.loginLog_statusPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'RangePicker',
    fieldName: 'dateRange',
    label: $t('monitor.loginLog_loginTime'),
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
    { type: 'seq', title: $t('monitor.loginLog_seq'), width: 60 },
    { field: 'infoId', title: $t('monitor.loginLog_infoId'), minWidth: 120 },
    { field: 'loginName', title: $t('monitor.loginLog_loginName'), minWidth: 120 },
    { field: 'ipaddr', title: $t('monitor.loginLog_ipaddr'), minWidth: 130 },
    { field: 'loginLocation', title: $t('monitor.loginLog_loginLocation'), minWidth: 120 },
    { field: 'browser', title: $t('monitor.loginLog_browser'), minWidth: 110 },
    { field: 'os', title: $t('monitor.loginLog_os'), minWidth: 110 },
    {
      field: 'status',
      title: $t('monitor.loginLog_status'),
      width: 100,
      slots: { default: 'status' },
    },
    { field: 'msg', title: $t('monitor.loginLog_msg'), minWidth: 160 },
    { field: 'loginTime', title: $t('monitor.loginLog_loginTime'), minWidth: 170 },
    { field: 'module', title: $t('monitor.loginLog_module'), minWidth: 110 },
    {
      field: 'action',
      title: $t('monitor.loginLog_action'),
      width: 100,
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
          ipaddr: form?.ipaddr,
          loginLocation: form?.loginLocation,
          userName: form?.userName,
          status: form?.status,
        };
        // RangePicker valueFormat=YYYY-MM-DD 时返回字符串数组；兜底格式化
        if (Array.isArray(form?.dateRange) && form.dateRange.length === 2) {
          const [s, e] = form.dateRange;
          params.dateRange = [
            dayjs(s).format('YYYY-MM-DD'),
            dayjs(e).format('YYYY-MM-DD'),
          ];
        }
        const res = await getLoginLogListApi(params);
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
    message.warning($t('monitor.loginLog_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.infoId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.infoId], 1);
}

async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('monitor.loginLog_confirmTitle'),
    content: $t('monitor.loginLog_confirmDeleteContent', { count }),
    okText: $t('monitor.loginLog_confirm'),
    cancelText: $t('monitor.loginLog_cancel'),
    okType: 'danger',
    onOk: async () => {
      deleting.value = true;
      try {
        await deleteLoginLogApi(ids);
        message.success($t('monitor.loginLog_deleteSuccess'));
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
    title: $t('monitor.loginLog_confirmTitle'),
    content: $t('monitor.loginLog_clearConfirmContent'),
    okText: $t('monitor.loginLog_confirm'),
    cancelText: $t('monitor.loginLog_cancel'),
    okType: 'danger',
    onOk: async () => {
      try {
        await clearLoginLogApi();
        message.success($t('monitor.loginLog_clearSuccess'));
        gridApi.query();
      } catch {
        /* 拦截器已提示 */
      }
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" :loading="deleting" @click="onDeleteBatch">
          {{ $t('monitor.loginLog_batchDelete') }}
        </Button>
        <Button danger class="ml-2" @click="onClear">
          {{ $t('monitor.loginLog_clearLog') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Tag :color="statusTag(row.status).color">
          {{ statusTag(row.status).text }}
        </Tag>
      </template>
      <template #action="{ row }">
        <Button
          type="primary" ghost
          size="small"
          danger
          :loading="deleting"
          @click="onDeleteRow(row)"
        >
          {{ $t('monitor.loginLog_delete') }}
        </Button>
      </template>
    </Grid>
  </Page>
</template>
