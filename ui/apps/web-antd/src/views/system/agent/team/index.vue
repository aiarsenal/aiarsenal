<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addAgentTeamApi,
  deleteAgentTeamApi,
  editAgentTeamApi,
  getAgentTeamListApi,
} from '#/api/system/agent/team';

defineOptions({ name: 'AgentTeam' });

const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('agent.team_name'),
    componentProps: { placeholder: $t('agent.team_namePlaceholder') },
  },
]);

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.team_seq'), width: 60 },
    { field: 'name', title: $t('agent.team_name'), minWidth: 160 },
    { field: 'communicationMode', title: $t('agent.team_communicationMode'), width: 140 },
    { field: 'description', title: $t('agent.team_description'), minWidth: 160 },
    { field: 'status', title: $t('agent.team_status'), width: 90 },
    { field: 'createdAt', title: $t('agent.team_createdAt'), minWidth: 160 },
    { field: 'action', title: $t('agent.team_action'), width: 150, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentTeamListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
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

const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('agent.team_modalTitleEdit') : $t('agent.team_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('agent.team_name'),
    rules: 'required',
    componentProps: { placeholder: $t('agent.team_namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'communicationMode',
    label: $t('agent.team_communicationMode'),
    componentProps: { placeholder: 'leader_only / broadcast' },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('agent.team_description'),
    componentProps: { placeholder: $t('agent.team_descriptionPlaceholder'), rows: 2 },
  },
  {
    component: 'Input' as const,
    fieldName: 'sharedVolumeName',
    label: 'Shared Volume',
    componentProps: { placeholder: 'docker volume (Phase2)' },
  },
  {
    component: 'Input' as const,
    fieldName: 'sharedMountPath',
    label: 'Mount Path',
    componentProps: { placeholder: '/team' },
  },
]);

const [Form, formApi] = useVbenForm(
  computed(() => ({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: formSchema,
    showDefaultActions: false,
  })) as any,
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ communicationMode: 'leader_only', sharedMountPath: '/team' });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.teamId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    name: row.name,
    communicationMode: row.communicationMode,
    description: row.description,
    sharedVolumeName: row.sharedVolumeName,
    sharedMountPath: row.sharedMountPath,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editAgentTeamApi({ ...values, teamId: editingId.value });
      message.success($t('agent.team_editSuccess'));
    } else {
      await addAgentTeamApi(values);
      message.success($t('agent.team_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('agent.team_selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.teamId), rows.length);
}
async function onDeleteRow(row: any) {
  await doDelete([row.teamId], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('agent.team_confirmDeleteTitle'),
    content: $t('agent.team_confirmDeleteContent', { count }),
    okText: $t('agent.team_confirm'),
    cancelText: $t('agent.team_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentTeamApi(ids);
      message.success($t('agent.team_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.team_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.team_batchDelete') }}
        </Button>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('agent.team_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('agent.team_delete') }}
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
