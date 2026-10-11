<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  deleteClientPackageApi,
  downloadClientPackageApi,
  fetchClientPackageApi,
  getClientPackageListApi,
} from '#/api/system/client/package';

defineOptions({ name: 'SystemClientPackage' });

// ===== 搜索表单（gin 不分页，前端本地过滤）=====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select',
    fieldName: 'agent',
    label: $t('client.package_agent'),
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
    fieldName: 'platform',
    label: $t('client.package_platform'),
    componentProps: {
      allowClear: true,
      options: [
        { label: 'Windows', value: 'win' },
        { label: 'macOS', value: 'mac' },
        { label: 'Linux', value: 'linux' },
        { label: $t('client.package_platformCommon'), value: 'common' },
      ],
    },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('client.package_status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('client.package_statusReady'), value: 'ready' },
        { label: $t('client.package_statusFailed'), value: 'failed' },
      ],
    },
  },
]);

// ===== 全量数据（本地过滤）=====
const allRows = ref<any[]>([]);

function applyFilter(form: any): any[] {
  return allRows.value.filter((r: any) => {
    if (form?.agent && r.agent !== form.agent) return false;
    if (form?.platform && r.platform !== form.platform) return false;
    if (form?.status && r.status !== form.status) return false;
    return true;
  });
}

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('client.package_seq'), width: 60, fixed: 'left' },
    {
      field: 'agent',
      title: $t('client.package_agent'),
      minWidth: 130,
      slots: { default: 'agent' },
    },
    { field: 'version', title: $t('client.package_version'), minWidth: 120 },
    { field: 'platform', title: $t('client.package_platform'), width: 90 },
    { field: 'filename', title: $t('client.package_filename'), minWidth: 200 },
    { field: 'fileSize', title: $t('client.package_size'), width: 110, slots: { default: 'size' } },
    { field: 'sha256', title: 'SHA256', minWidth: 160 },
    { field: 'officialUrl', title: $t('client.package_officialUrl'), minWidth: 220 },
    {
      field: 'status',
      title: $t('client.package_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'fetchedAt', title: $t('client.package_fetchedAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('client.package_action'),
      width: 150,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async ({ form }: any) => {
        const res = await getClientPackageListApi();
        allRows.value = res?.list ?? [];
        const items = applyFilter(form);
        return { items, total: items.length };
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

function formatSize(n: number) {
  if (!n) return '-';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// ===== 拉取弹窗（相当于新增）=====
const modalVisible = ref(false);
const submitting = ref(false);
const modalTitle = computed(() => $t('client.package_fetchTitle'));

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Alert',
    fieldName: '_tip',
    label: '',
    componentProps: {
      type: 'info',
      showIcon: true,
      message: $t('client.package_fetchTip'),
      style: { marginBottom: '12px' },
    },
  },
  {
    component: 'Select',
    fieldName: 'agent',
    label: $t('client.package_agent'),
    rules: 'required',
    componentProps: () => ({
      options: [
        { label: 'openclaw', value: 'openclaw' },
        { label: 'hermes', value: 'hermes' },
        { label: 'obsidian', value: 'obsidian' },
      ],
      placeholder: $t('client.package_agentPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input',
    fieldName: 'version',
    label: $t('client.package_version'),
    rules: 'required',
    componentProps: { placeholder: $t('client.package_versionPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'platform',
    label: $t('client.package_platform'),
    rules: 'required',
    componentProps: () => ({
      options: [
        { label: 'Windows', value: 'win' },
        { label: 'macOS', value: 'mac' },
        { label: 'Linux', value: 'linux' },
        { label: $t('client.package_platformCommonCross'), value: 'common' },
      ],
      placeholder: $t('client.package_platformPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'InputTextArea',
    fieldName: 'officialUrl',
    label: $t('client.package_officialUrlLabel'),
    rules: z
      .string()
      .url($t('client.package_urlInvalid'))
      .or(z.literal('')),
    componentProps: {
      rows: 3,
      placeholder: $t('client.package_urlPlaceholder'),
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: formSchema,
    showDefaultActions: false,
  }) as any,
);

async function openFetch() {
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ agent: 'obsidian', platform: 'mac' });
}

async function onSubmitFetch() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  if (!values.officialUrl) {
    message.warning($t('client.package_fillRequired'));
    return;
  }
  submitting.value = true;
  try {
    await fetchClientPackageApi({
      agent: values.agent,
      version: values.version,
      platform: values.platform,
      officialUrl: values.officialUrl,
    });
    message.success($t('client.package_fetchSuccess'));
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  } finally {
    submitting.value = false;
  }
}

// ===== 下载到本地（管理员从服务缓存下到浏览器）=====
async function onDownloadRow(row: any) {
  try {
    await downloadClientPackageApi(
      row.agent,
      row.version,
      row.platform,
      row.filename,
    );
    message.success($t('client.package_downloadSuccess'));
  } catch {
    message.error($t('client.package_downloadFailed'));
  }
}

// ===== 删除 =====
async function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.package_deleteContent', {
      agent: row.agent,
      version: row.version,
      platform: row.platform,
    }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteClientPackageApi(row.packageId);
      message.success($t('client.package_deleteSuccess'));
      gridApi.query();
    },
  });
}

// 顶部“从官网拉取”按钮的 loading 状态
const fetchBtnLoading = computed(() => submitting.value);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" :loading="fetchBtnLoading" @click="openFetch">
          {{ $t('client.package_fetchBtn') }}
        </Button>
      </template>
      <template #agent="{ row }">
        <span class="pkg-agent-cell">
          <img v-if="row.logo" class="pkg-logo" :src="row.logo" :alt="row.agent" />
          <span class="pkg-agent-name">{{ row.agent }}</span>
        </span>
      </template>
      <template #size="{ row }">
        {{ formatSize(row.fileSize) }}
      </template>
      <template #status="{ row }">
        <span v-if="row.status === 'ready'" class="aa-enum aa-enum--magenta">
          {{ $t('client.package_statusReady') }}
        </span>
        <span v-else class="aa-enum aa-enum--error">
          {{ $t('client.package_statusFailed') }}
        </span>
      </template>
      <template #action="{ row }">
        <Button
          type="primary"
          ghost
          size="small"
          :disabled="row.status !== 'ready'"
          style="margin-right: 8px"
          @click="onDownloadRow(row)"
        >
          {{ $t('client.package_download') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('client.package_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="560"
      :destroy-on-close="true"
      :confirm-loading="submitting"
      @ok="onSubmitFetch"
    >
      <Form />
    </Modal>
  </Page>
</template>

<style scoped>
/* logo + name 强制同一行（flex 容器，居中对齐）*/
.pkg-agent-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.pkg-logo {
  width: 18px;
  height: 18px;
  object-fit: contain;
  flex-shrink: 0;
}
.pkg-agent-name {
  line-height: 1;
}
</style>
