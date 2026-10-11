<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Progress, Select } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addProjectMilestoneApi,
  deleteProjectMilestoneApi,
  editProjectMilestoneApi,
  getProjectMilestoneListApi,
} from '#/api/system/project/milestone';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectMilestone' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');
onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) projectId.value = projects.value[0]!.projectId;
  gridApi.query();
});

const STATUS_OPTIONS = computed(() => [
  { label: $t('project.ms_open'), value: 'open' },
  { label: $t('project.ms_in_progress'), value: 'in_progress' },
  { label: $t('project.ms_closed'), value: 'closed' },
]);
// 枚举值 → 本地化标签（无匹配回落原值）
const statusLabel = (v: string) => STATUS_OPTIONS.value.find((o) => o.value === v)?.label ?? v;
const MS_COLOR: Record<string, string> = { open: 'default', in_progress: 'processing', closed: 'magenta' };

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { field: 'name', title: $t('project.ms_name'), minWidth: 180 },
    { field: 'status', title: $t('project.status'), width: 110, slots: { default: 'status' } },
    { field: 'progress', title: $t('project.ms_progress'), width: 170, slots: { default: 'progress' } },
    { field: 'startDate', title: $t('project.startDate'), minWidth: 140 },
    { field: 'dueDate', title: $t('project.dueDate'), minWidth: 140 },
    { field: 'sortOrder', title: $t('project.sort'), width: 80 },
    { field: 'action', title: $t('project.action'), width: 160, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page }: any) => {
        if (!projectId.value) return { items: [], total: 0 };
        const res = await getProjectMilestoneListApi({ pageNum: page.currentPage, pageSize: 20, projectId: projectId.value });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() => (isEdit.value ? $t('project.ms_edit_title') : $t('project.ms_add_title')));

const formSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input', fieldName: 'name', label: $t('project.ms_name'), rules: 'required' },
  { component: 'Select', fieldName: 'status', label: $t('project.status'), componentProps: { options: STATUS_OPTIONS.value, style: { width: '100%' } } },
  { component: 'DatePicker', fieldName: 'startDate', label: $t('project.startDate'), componentProps: { style: { width: '100%' } } },
  { component: 'DatePicker', fieldName: 'dueDate', label: $t('project.dueDate'), componentProps: { style: { width: '100%' } } },
  { component: 'InputNumber', fieldName: 'sortOrder', label: $t('project.sort'), componentProps: { style: { width: '100%' } } },
  { component: 'Textarea', fieldName: 'description', label: $t('project.description'), componentProps: { rows: 2, style: { width: '100%' } } },
]);
const [Form, formApi] = useVbenForm(reactive({ commonConfig: { fullWidth: true }, schema: formSchema, showDefaultActions: false }));

async function openAdd() {
  if (!projectId.value) { message.warning($t('project.select_project_first')); return; }
  isEdit.value = false; editingId.value = ''; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  await formApi.setValues({ status: 'open', sortOrder: 0 });
}
async function openEdit(row: any) {
  isEdit.value = true; editingId.value = row.milestoneId; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  await formApi.setValues({ name: row.name, status: row.status ?? 'open', startDate: row.startDate ?? '', dueDate: row.dueDate ?? '', sortOrder: row.sortOrder ?? 0, description: row.description ?? '' });
}
async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) await editProjectMilestoneApi({ ...values, projectId: projectId.value, milestoneId: editingId.value });
    else await addProjectMilestoneApi({ ...values, projectId: projectId.value });
    message.success($t('project.edit_success'));
    modalVisible.value = false; gridApi.query();
  } catch { /* */ }
}
function onDelete(row: any) {
  Modal.confirm({
    title: $t('project.confirm_delete_title'), okType: 'danger', okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => { await deleteProjectMilestoneApi([row.milestoneId]); message.success($t('project.delete_success')); gridApi.query(); },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')" :options="projects.map((p) => ({ label: p.name, value: p.projectId }))" @change="gridApi.query()" />
    </div>
    <Grid>
      <template #toolbar-actions><Button type="primary" @click="openAdd">{{ $t('project.ms_add') }}</Button></template>
      <template #status="{ row }"><span :class="['aa-enum', `aa-enum--${MS_COLOR[row.status] || 'default'}`]">{{ statusLabel(row.status) }}</span></template>
      <template #progress="{ row }">
        <div class="flex items-center gap-2">
          <Progress :percent="row.progress || 0" size="small" style="width: 100px; margin: 0" />
          <span class="text-xs opacity-60">{{ row.taskDone ?? 0 }}/{{ row.taskTotal ?? 0 }}</span>
        </div>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('project.edit') }}</Button>
        <Button type="primary" ghost size="small" danger class="ml-1" @click="onDelete(row)">{{ $t('project.delete') }}</Button>
      </template>
    </Grid>
    <Modal v-model:open="modalVisible" :title="modalTitle" :width="600" :destroy-on-close="true" @ok="onSubmit"><Form /></Modal>
  </Page>
</template>
