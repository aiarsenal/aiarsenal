<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { getAgentInstanceAllApi } from '#/api/system/agent/instance';
import {
  addAgentUserApi,
  deleteAgentUserApi,
  editAgentUserApi,
  getAgentUserListApi,
  resetAgentUserPasswordApi,
} from '#/api/system/agent/user';
import type { AgentUserApi } from '#/api/system/agent/user';

defineOptions({ name: 'SystemAgentUser' });

const ROLE_OPTIONS = computed(() => [
  { label: $t('agent.au_role_admin'), value: 'admin' },
  { label: $t('agent.au_role_operator'), value: 'operator' },
  { label: $t('agent.au_role_member'), value: 'member' },
]);

// ===== 绑定实例选项（全部实例，名 + id） =====
const instanceOptions = ref<{ label: string; value: string }[]>([]);
async function loadInstanceOptions() {
  try {
    const res = await getAgentInstanceAllApi({ pageNum: 1, pageSize: 200 });
    instanceOptions.value = (res?.list ?? []).map((i: any) => ({
      label: i.name,
      value: i.instanceId,
    }));
  } catch {
    instanceOptions.value = [];
  }
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyword',
    label: $t('agent.au_keyword'),
    componentProps: { placeholder: $t('agent.au_keywordPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('agent.au_status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('agent.au_enabled'), value: 1 },
        { label: $t('agent.au_disabled'), value: 0 },
      ],
      placeholder: $t('agent.au_statusPlaceholder'),
      style: { width: '100%' },
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.au_seq'), width: 60 },
    { field: 'username', title: $t('agent.au_username'), minWidth: 120 },
    { field: 'displayName', title: $t('agent.au_displayName'), minWidth: 120 },
    { field: 'role', title: $t('agent.au_role'), width: 100, slots: { default: 'role' } },
    { field: 'instanceNames', title: $t('agent.au_instances'), minWidth: 180, slots: { default: 'instances' } },
    { field: 'enabled', title: $t('agent.au_status'), width: 90, slots: { default: 'enabled' } },
    { field: 'lastLoginAt', title: $t('agent.au_lastLogin'), width: 160, formatter: ({ cellValue }: any) => fmtTime(cellValue) },
    { field: 'createdAt', title: $t('agent.au_createdAt'), width: 160, formatter: ({ cellValue }: any) => fmtTime(cellValue) },
    { field: 'action', title: $t('agent.au_action'), width: 240, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentUserListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          // -1=全部（后端 status>=0 才过滤），与实例列表同口径
          status: -1,
          ...(form as any),
        });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('agent.au_modalTitleEdit') : $t('agent.au_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'username',
    label: $t('agent.au_username'),
    rules: 'required',
    componentProps: {
      disabled: isEdit.value,
      placeholder: $t('agent.au_usernamePlaceholder'),
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'displayName',
    label: $t('agent.au_displayName'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.au_displayNamePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'role',
    label: $t('agent.au_role'),
    rules: 'selectRequired',
    componentProps: {
      options: ROLE_OPTIONS.value,
      placeholder: $t('agent.au_rolePlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'instanceIds',
    label: $t('agent.au_instances'),
    componentProps: {
      mode: 'multiple',
      options: instanceOptions.value,
      placeholder: $t('agent.au_instancesPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('agent.au_status'),
    componentProps: {
      options: [
        { label: $t('agent.au_enabled'), value: 1 },
        { label: $t('agent.au_disabled'), value: 0 },
      ],
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'remark',
    label: $t('agent.au_remark'),
    componentProps: { rows: 2, placeholder: $t('agent.au_remarkPlaceholder') },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await loadInstanceOptions();
  await formApi.resetForm();
  await formApi.setValues({ role: 'member', enabled: 1, instanceIds: [] });
}

async function openEdit(row: AgentUserApi.UserItem) {
  isEdit.value = true;
  editingId.value = row.userId;
  modalVisible.value = true;
  await nextTick();
  await loadInstanceOptions();
  await formApi.resetForm();
  await formApi.setValues({
    username: row.username ?? '',
    displayName: row.displayName ?? '',
    role: row.role ?? 'member',
    instanceIds: row.instanceIds ?? [],
    enabled: row.enabled ?? 1,
    remark: row.remark ?? '',
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const payload: AgentUserApi.UserReq = {
    username: (values.username ?? '').trim(),
    displayName: (values.displayName ?? '').trim(),
    role: values.role ?? 'member',
    enabled: values.enabled ?? 1,
    instanceIds: values.instanceIds ?? [],
    remark: values.remark ?? '',
  };
  try {
    if (isEdit.value) {
      await editAgentUserApi({ ...payload, userId: editingId.value });
      message.success($t('agent.au_editSuccess'));
      modalVisible.value = false;
      gridApi.query();
    } else {
      const res: any = await addAgentUserApi(payload);
      message.success($t('agent.au_addSuccess'));
      modalVisible.value = false;
      gridApi.query();
      if (res?.initialPassword) {
        plainPassword.value = res.initialPassword;
        passwordModalTitle.value = $t('agent.au_initialPasswordTitle');
        passwordVisible.value = true;
      }
    }
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内启停：edit 整行（镜像 openEdit 字段），失败回滚
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.enabled;
  row.enabled = newEnabled;
  try {
    await editAgentUserApi({
      userId: row.userId,
      username: row.username ?? '',
      displayName: row.displayName ?? '',
      role: row.role ?? 'member',
      enabled: newEnabled,
      instanceIds: row.instanceIds ?? [],
      remark: row.remark ?? '',
    });
    message.success($t('agent.au_editSuccess'));
  } catch {
    row.enabled = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('agent.au_selectToDelete'));
    return;
  }
  doDelete(rows.map((r: any) => r.userId), rows.length);
}
async function onDeleteRow(row: any) {
  doDelete([row.userId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('agent.au_confirmDeleteTitle'),
    content: $t('agent.au_confirmDeleteContent', { count }),
    okText: $t('agent.au_confirm'),
    cancelText: $t('agent.au_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentUserApi(ids);
      message.success($t('agent.au_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 重置密码（新密码仅显示一次） =====
function onResetPassword(row: any) {
  Modal.confirm({
    title: $t('agent.au_resetTitle'),
    content: $t('agent.au_resetContent', { name: row.displayName || row.username }),
    okText: $t('agent.au_confirm'),
    cancelText: $t('agent.au_cancel'),
    onOk: async () => {
      try {
        const res: any = await resetAgentUserPasswordApi(row.userId);
        message.success($t('agent.au_resetSuccess'));
        plainPassword.value = res?.password ?? '';
        passwordModalTitle.value = $t('agent.au_newPasswordTitle');
        passwordVisible.value = true;
      } catch {
        /* 拦截器已提示 */
      }
    },
  });
}

// ===== 一次性密码展示弹窗 =====
const passwordVisible = ref(false);
const plainPassword = ref('');
const passwordModalTitle = ref('');

function copyPassword() {
  navigator.clipboard?.writeText(plainPassword.value);
  message.success($t('agent.au_copied'));
}

// ===== 展示 =====
function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

function roleText(role?: string) {
  switch (role) {
    case 'admin': {
      return $t('agent.au_role_admin');
    }
    case 'operator': {
      return $t('agent.au_role_operator');
    }
    case 'member': {
      return $t('agent.au_role_member');
    }
    default: {
      return role || '-';
    }
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.au_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.au_batchDelete') }}
        </Button>
      </template>
      <template #role="{ row }">
        <span :class="['aa-enum', row.role === 'admin' ? 'aa-enum--red' : row.role === 'operator' ? 'aa-enum--blue' : 'aa-enum--default']">
          {{ roleText(row.role) }}
        </span>
      </template>
      <template #instances="{ row }">
        <template v-if="row.instanceNames?.length">
          <span
            v-for="n in row.instanceNames"
            :key="n"
            class="mr-1 inline-block max-w-40 truncate rounded border border-solid border-border px-1 text-xs"
            :title="n"
          >{{ n }}</span>
        </template>
        <span v-else class="text-xs opacity-50">-</span>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('agent.au_enabled')"
          :un-checked-children="$t('agent.au_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row as AgentUserApi.UserItem)">
          {{ $t('agent.au_edit') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onResetPassword(row as AgentUserApi.UserItem)">
          {{ $t('agent.au_reset') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.au_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="560"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <!-- 初始/重置密码（仅显示一次） -->
    <Modal
      v-model:open="passwordVisible"
      :title="passwordModalTitle"
      :width="520"
      :ok-text="$t('agent.au_close')"
    >
      <div class="mb-2 text-amber-500">{{ $t('agent.au_passwordWarning') }}</div>
      <Input :value="plainPassword" readonly>
        <template #addonAfter>
          <Button size="small" @click="copyPassword">
            {{ $t('agent.au_copy') }}
          </Button>
        </template>
      </Input>
    </Modal>
  </Page>
</template>
