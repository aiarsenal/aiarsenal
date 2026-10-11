<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  forceLogoutApi,
  getUserOnlineListApi,
} from '#/api/system/monitor/userOnline';

defineOptions({ name: 'SystemMonitorUserOnline' });

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'ipaddr',
    label: $t('monitor.userOnline_ip'),
    componentProps: { placeholder: $t('monitor.userOnline_ipPlaceholder'), allowClear: true },
  },
  {
    component: 'Input',
    fieldName: 'userName',
    label: $t('monitor.userOnline_userName'),
    componentProps: { placeholder: $t('monitor.userOnline_userNamePlaceholder'), allowClear: true },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('monitor.userOnline_seq'), width: 60 },
    { field: 'id', title: $t('monitor.userOnline_id'), minWidth: 220 },
    { field: 'userName', title: $t('monitor.userOnline_userName'), minWidth: 120 },
    { field: 'ip', title: $t('monitor.userOnline_ip'), minWidth: 130 },
    { field: 'explorer', title: $t('monitor.userOnline_browser'), minWidth: 140 },
    { field: 'os', title: $t('monitor.userOnline_os'), minWidth: 120 },
    { field: 'createTime', title: $t('monitor.userOnline_createTime'), minWidth: 170 },
    {
      field: 'action',
      title: $t('monitor.userOnline_action'),
      width: 110,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getUserOnlineListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ipaddr: form?.ipaddr,
          userName: form?.userName,
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
  formOptions: {
    schema: searchSchema,
  },
});

// ===== 强制下线 =====
// gin Delete 仅支持单条 uuid（body {uuid}），批量时逐条调用。
const deleting = ref(false);

async function doForceLogout(uuid: string, count = 1) {
  Modal.confirm({
    title: $t('monitor.userOnline_confirmTitle'),
    content:
      count > 1
        ? $t('monitor.userOnline_confirmBatchContent', { count })
        : $t('monitor.userOnline_confirmSingleContent'),
    okText: $t('monitor.userOnline_confirm'),
    cancelText: $t('monitor.userOnline_cancel'),
    okType: 'danger',
    onOk: async () => {
      deleting.value = true;
      try {
        await forceLogoutApi(uuid);
        message.success($t('monitor.userOnline_forceLogoutSuccess'));
        gridApi.query();
      } catch {
        /* 拦截器已提示 */
      } finally {
        deleting.value = false;
      }
    },
  });
}

async function onForceLogoutBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('monitor.userOnline_selectToForceLogout'));
    return;
  }
  // gin 仅支持单条 uuid，批量逐条调用
  Modal.confirm({
    title: $t('monitor.userOnline_confirmTitle'),
    content: $t('monitor.userOnline_confirmBatchContent', { count: rows.length }),
    okText: $t('monitor.userOnline_confirm'),
    cancelText: $t('monitor.userOnline_cancel'),
    okType: 'danger',
    onOk: async () => {
      deleting.value = true;
      try {
        await Promise.all(
          rows.map((r: any) => forceLogoutApi(r.id)),
        );
        message.success($t('monitor.userOnline_forceLogoutSuccess'));
        gridApi.query();
      } catch {
        /* 拦截器已提示 */
      } finally {
        deleting.value = false;
      }
    },
  });
}

async function onForceLogoutRow(row: any) {
  await doForceLogout(row.id, 1);
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" danger :loading="deleting" @click="onForceLogoutBatch">
          {{ $t('monitor.userOnline_batchForceLogout') }}
        </Button>
      </template>
      <template #action="{ row }">
        <Button
          type="primary" ghost
          size="small"
          danger
          :loading="deleting"
          @click="onForceLogoutRow(row)"
        >
          {{ $t('monitor.userOnline_forceLogout') }}
        </Button>
      </template>
    </Grid>
  </Page>
</template>
