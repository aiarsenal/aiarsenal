<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  deleteAgentImageApi,
  getAgentImageListApi,
  upsertAgentImageApi,
} from '#/api/system/agent/settings';
import { AGENT_OPTIONS } from '#/constants/agent-options';

defineOptions({ name: 'AgentSettings' });

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('agent.settings_image_seq'), width: 60, fixed: 'left' },
    { field: 'agent', title: $t('agent.settings_image_agent'), width: 110 },
    { field: 'version', title: $t('agent.settings_image_version'), width: 100 },
    { field: 'displayName', title: $t('agent.settings_image_displayName'), minWidth: 150 },
    { field: 'image', title: $t('agent.settings_image_image'), minWidth: 260 },
    {
      field: 'isEnabled',
      title: $t('agent.settings_image_enabled'),
      width: 90,
      slots: { default: 'enabled' },
    },
    { field: 'action', title: $t('agent.settings_image_action'), width: 150, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async () => {
        const res = await getAgentImageListApi({
          agent: '',
          version: '',
        });
        return { items: res?.list ?? [], total: res?.list?.length ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
});

const isEdit = ref(false);
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value
    ? $t('agent.settings_image_modalTitleEdit')
    : $t('agent.settings_image_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select' as const,
    fieldName: 'agent',
    label: $t('agent.settings_image_agent'),
    rules: 'required',
    // width 必须显式给：antd-vue 的 placeholder 是 flex:1（flex-basis:0），
    // 空值时 Select 固有宽度塌缩到只剩内边距+箭头，placeholder 被裁到只剩一个字母
    componentProps: {
      options: AGENT_OPTIONS,
      placeholder: 'openclaw/hermes/common',
      style: { width: '100%' },
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'version',
    label: $t('agent.settings_image_version'),
    rules: 'required',
    componentProps: { placeholder: 'test / v1 / 2026.07' },
  },
  {
    component: 'Input' as const,
    fieldName: 'displayName',
    label: $t('agent.settings_image_displayName'),
    rules: 'required',
  },
  {
    component: 'Input' as const,
    fieldName: 'image',
    label: $t('agent.settings_image_image'),
    rules: 'required',
    componentProps: { placeholder: 'registry/image:tag' },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isEnabled',
    label: $t('agent.settings_image_enabled'),
    componentProps: {
      options: [
        { label: $t('agent.settings_image_enabled'), value: 1 },
        { label: $t('agent.settings_image_disabled'), value: 0 },
      ],
    },
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
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({ agent: '', version: '', isEnabled: 1 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    agent: row.agent ?? '',
    version: row.version ?? '',
    displayName: row.displayName ?? '',
    image: row.image ?? '',
    isEnabled: row.isEnabled ?? 1,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    await upsertAgentImageApi(values);
    message.success($t('agent.settings_image_saveSuccess'));
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内启用/禁用切换：upsert 整行（二元组定位），失败回滚。
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.isEnabled;
  row.isEnabled = newEnabled;
  try {
    await upsertAgentImageApi({
      agent: row.agent,
      version: row.version,
      displayName: row.displayName,
      image: row.image,
      isEnabled: newEnabled,
    });
    message.success($t('agent.settings_image_saveSuccess'));
  } catch {
    row.isEnabled = prev;
  }
}

async function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('agent.team_confirmDeleteTitle'),
    content: $t('agent.settings_image_confirmDelete'),
    okText: $t('agent.team_confirm'),
    cancelText: $t('agent.team_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentImageApi(row.agent, row.version);
      message.success($t('agent.settings_image_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('agent.settings_image_add') }}
        </Button>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.isEnabled === 1"
          :checked-children="$t('agent.settings_image_enabled')"
          :un-checked-children="$t('agent.settings_image_disabled')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('agent.team_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.settings_image_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="600"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>
  </Page>
</template>
