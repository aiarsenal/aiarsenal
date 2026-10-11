<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Progress, Select, Table, Tabs } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { getDirByRefApi } from '#/api/system/doc';
import { $t } from '#/locales';
import {
  addProjectApi,
  deleteProjectApi,
  editProjectApi,
  getProjectListApi,
  getProjectOverviewApi,
} from '#/api/system/project/project';
import {
  addProjectBindingApi,
  BINDING_KIND_OPTIONS,
  deleteProjectBindingApi,
  editProjectBindingApi,
  getProjectBindingListApi,
} from '#/api/system/project/project';

defineOptions({ name: 'SystemProjectList' });

const STATUS_OPTIONS = computed(() => [
  { label: $t('project.status_planning'), value: 'planning' },
  { label: $t('project.status_active'), value: 'active' },
  { label: $t('project.status_paused'), value: 'paused' },
  { label: $t('project.status_closed'), value: 'closed' },
  { label: $t('project.status_archived'), value: 'archived' },
]);
const PRIORITY_OPTIONS = computed(() => [
  { label: $t('project.priority_low'), value: 'low' },
  { label: $t('project.priority_medium'), value: 'medium' },
  { label: $t('project.priority_high'), value: 'high' },
  { label: $t('project.priority_urgent'), value: 'urgent' },
]);
const VISIBILITY_OPTIONS = computed(() => [
  { label: $t('project.vis_private'), value: 'private' },
  { label: $t('project.vis_dept'), value: 'dept' },
  { label: $t('project.vis_team'), value: 'team' },
  { label: $t('project.vis_public'), value: 'public' },
]);
// 枚举值 → 本地化标签（表格/抽屉展示；无匹配回落原值）
const statusLabel = (v: string) =>
  STATUS_OPTIONS.value.find((o) => o.value === v)?.label ?? v;
const priorityLabel = (v: string) =>
  PRIORITY_OPTIONS.value.find((o) => o.value === v)?.label ?? v;
const visLabel = (v: string) =>
  VISIBILITY_OPTIONS.value.find((o) => o.value === v)?.label ?? v;
const PRIORITY_COLOR: Record<string, string> = {
  urgent: 'error',
  high: 'warning',
  medium: 'processing',
  low: 'default',
};
const STATUS_COLOR: Record<string, string> = {
  planning: 'default',
  active: 'processing',
  paused: 'warning',
  closed: 'magenta',
  archived: 'default',
};

const searchSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input', fieldName: 'name', label: $t('project.name'), componentProps: { placeholder: $t('project.name_placeholder') } },
  {
    component: 'Select', fieldName: 'status', label: $t('project.status'),
    componentProps: { allowClear: true, options: STATUS_OPTIONS.value, placeholder: $t('project.all'), style: { width: '100%' } },
  },
]);

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: '#', width: 55 },
    { field: 'name', title: $t('project.name'), minWidth: 160 },
    { field: 'code', title: $t('project.code'), width: 110 },
    { field: 'status', title: $t('project.status'), width: 100, slots: { default: 'status' } },
    { field: 'priority', title: $t('project.priority'), width: 90, slots: { default: 'priority' } },
    { field: 'progress', title: $t('project.progress'), width: 140, slots: { default: 'progress' } },
    { field: 'visibility', title: $t('project.visibility'), width: 90, slots: { default: 'visibility' } },
    { field: 'createdAt', title: $t('project.createdAt'), minWidth: 150 },
    { field: 'action', title: $t('project.action'), width: 210, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getProjectListApi({ pageNum: page.currentPage, pageSize: 20, ...(form as any) });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions: { schema: searchSchema } });

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() => (isEdit.value ? $t('project.edit_title') : $t('project.add_title')));

const formSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input', fieldName: 'name', label: $t('project.name'), rules: 'required', componentProps: { placeholder: $t('project.name_placeholder') } },
  { component: 'Input', fieldName: 'code', label: $t('project.code'), componentProps: { placeholder: $t('project.code_placeholder') } },
  {
    component: 'Select', fieldName: 'status', label: $t('project.status'),
    componentProps: { options: STATUS_OPTIONS.value, placeholder: $t('project.status_placeholder'), style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'priority', label: $t('project.priority'),
    componentProps: { options: PRIORITY_OPTIONS.value, placeholder: $t('project.priority_placeholder'), style: { width: '100%' } },
  },
  { component: 'InputNumber', fieldName: 'progress', label: $t('project.progress'), componentProps: { min: 0, max: 100, style: { width: '100%' } } },
  {
    component: 'Select', fieldName: 'visibility', label: $t('project.visibility'),
    componentProps: { options: VISIBILITY_OPTIONS.value, placeholder: $t('project.visibility_placeholder'), style: { width: '100%' } },
  },
  { component: 'Textarea', fieldName: 'description', label: $t('project.description'), componentProps: { rows: 2, placeholder: $t('project.description_placeholder'), style: { width: '100%' } } },
  { component: 'DatePicker', fieldName: 'startDate', label: $t('project.startDate'), componentProps: { style: { width: '100%' } } },
  { component: 'DatePicker', fieldName: 'dueDate', label: $t('project.dueDate'), componentProps: { style: { width: '100%' } } },
]);

const [Form, formApi] = useVbenForm(
  reactive({ commonConfig: { fullWidth: true, hideLabel: false }, schema: formSchema, showDefaultActions: false }),
);

async function openAdd() {
  isEdit.value = false; editingId.value = ''; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  await formApi.setValues({ status: 'planning', priority: 'medium', progress: 0, visibility: 'private' });
}
async function openEdit(row: any) {
  isEdit.value = true; editingId.value = row.projectId; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  await formApi.setValues({
    name: row.name, code: row.code ?? '', status: row.status ?? 'planning', priority: row.priority ?? 'medium',
    progress: row.progress ?? 0, visibility: row.visibility ?? 'private', description: row.description ?? '',
    startDate: row.startDate ?? '', dueDate: row.dueDate ?? '',
  });
}
async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      await editProjectApi({ ...values, projectId: editingId.value });
      message.success($t('project.edit_success'));
    } else {
      await addProjectApi(values);
      message.success($t('project.add_success'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch { /* 拦截器已提示 */ }
}

async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) { message.warning($t('project.select_to_delete')); return; }
  await doDelete(rows.map((r: any) => r.projectId), rows.length);
}
async function onDeleteRow(row: any) { await doDelete([row.projectId], 1); }
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('project.confirm_delete_title'),
    content: $t('project.confirm_delete_content', { count }),
    okText: $t('project.confirm'), cancelText: $t('project.cancel'), okType: 'danger',
    onOk: async () => {
      await deleteProjectApi(ids);
      message.success($t('project.delete_success'));
      gridApi.query();
    },
  });
}

// ===== 详情抽屉：概览 + 资源绑定 =====
const detailVisible = ref(false);
const detailProject = ref<any>(null);
const detailLoading = ref(false);
const bindings = ref<any[]>([]);
const bindModalVisible = ref(false);
const bindEditingId = ref('');
const bindForm = reactive({ kind: 'agent', targetId: '', targetName: '', enabled: 1 });

async function openDetail(row: any) {
  detailVisible.value = true;
  detailProject.value = row;
  detailLoading.value = true;
  try {
    const ov = await getProjectOverviewApi(row.projectId);
    detailProject.value = ov?.project ?? row;
  } finally { detailLoading.value = false; }
  await loadBindings(row.projectId);
}

// 项目空间跳转：byref 反查项目托管目录 → 文档页预选该目录
const router = useRouter();
async function openProjectDocs() {
  try {
    const d = await getDirByRefApi('project', detailProject.value.projectId);
    if (d?.dirId) {
      router.push({ path: '/doc/document/list', query: { dirId: d.dirId } });
    }
  } catch {
    /* 拦截器已提示（无权限/目录未生成） */
  }
}
async function loadBindings(projectId: string) {
  const res = await getProjectBindingListApi({ projectId, pageNum: 1, pageSize: 200 });
  bindings.value = res?.list ?? [];
}
function openBindAdd() {
  bindEditingId.value = '';
  Object.assign(bindForm, { kind: 'agent', targetId: '', targetName: '', enabled: 1 });
  bindModalVisible.value = true;
}
function openBindEdit(row: any) {
  bindEditingId.value = row.bindingId;
  Object.assign(bindForm, { kind: row.kind, targetId: row.targetId, targetName: row.targetName ?? '', enabled: row.enabled ?? 1 });
  bindModalVisible.value = true;
}
async function onBindSubmit() {
  if (!bindForm.targetId) { message.warning($t('project.binding_target_required')); return; }
  try {
    if (bindEditingId.value) {
      await editProjectBindingApi({ ...bindForm, bindingId: bindEditingId.value, projectId: detailProject.value.projectId });
    } else {
      await addProjectBindingApi({ ...bindForm, projectId: detailProject.value.projectId });
    }
    message.success($t('project.binding_save_success'));
    bindModalVisible.value = false;
    await loadBindings(detailProject.value.projectId);
  } catch { /* */ }
}
function onBindDelete(row: any) {
  Modal.confirm({
    title: $t('project.binding_unbind_title'), okType: 'danger',
    okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => {
      await deleteProjectBindingApi([row.bindingId]);
      message.success($t('project.binding_unbind_success'));
      await loadBindings(detailProject.value.projectId);
    },
  });
}

const bindingCols = computed(() => [
  { title: $t('project.binding_kind'), dataIndex: 'kind', key: 'kind', width: 110 },
  { title: $t('project.binding_target'), dataIndex: 'targetName', key: 'targetName' },
  { title: $t('project.binding_enabled'), dataIndex: 'enabled', key: 'enabled', width: 90 },
  { title: $t('project.action'), key: 'action', width: 150 },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('project.add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('project.batch_delete') }}</Button>
      </template>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${STATUS_COLOR[row.status] || 'default'}`]">{{ statusLabel(row.status) }}</span>
      </template>
      <template #priority="{ row }">
        <span :class="['aa-enum', `aa-enum--${PRIORITY_COLOR[row.priority] || 'default'}`]">{{ priorityLabel(row.priority) }}</span>
      </template>
      <template #visibility="{ row }">{{ visLabel(row.visibility) }}</template>
      <template #progress="{ row }">
        <Progress :percent="row.progress || 0" size="small" />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openDetail(row)">{{ $t('project.detail') }}</Button>
        <Button type="primary" ghost size="small" class="ml-1" @click="openEdit(row)">{{ $t('project.edit') }}</Button>
        <Button type="primary" ghost size="small" danger class="ml-1" @click="onDeleteRow(row)">{{ $t('project.delete') }}</Button>
      </template>
    </Grid>

    <Modal v-model:open="modalVisible" :title="modalTitle" :width="640" :destroy-on-close="true" @ok="onSubmit">
      <Form />
    </Modal>

    <Modal
      v-model:open="detailVisible" :title="$t('project.detail_title')" :width="820" :footer="null" :destroy-on-close="true"
    >
      <Tabs>
        <Tabs.TabPane key="overview" :tab="$t('project.tab_overview')">
          <div v-if="detailProject" class="space-y-2">
            <p><b>{{ $t('project.name') }}：</b>{{ detailProject.name }}</p>
            <p><b>{{ $t('project.code') }}：</b>{{ detailProject.code || '-' }}</p>
            <p><b>{{ $t('project.status') }}：</b>{{ statusLabel(detailProject.status) }}</p>
            <p><b>{{ $t('project.progress') }}：</b>{{ detailProject.progress || 0 }}%</p>
            <p><b>{{ $t('project.member_count') }}：</b>{{ (detailProject as any).memberCount ?? '-' }}</p>
            <p><b>{{ $t('project.description') }}：</b>{{ detailProject.description || '-' }}</p>
            <div class="mt-2">
              <Button type="primary" ghost size="small" @click="openProjectDocs">
                {{ $t('project.openDocs') }}
              </Button>
            </div>
          </div>
        </Tabs.TabPane>
        <Tabs.TabPane key="binding" :tab="$t('project.tab_binding')">
          <div class="mb-2">
            <Button type="primary" size="small" @click="openBindAdd">{{ $t('project.binding_add') }}</Button>
          </div>
          <Table :data-source="bindings" :columns="bindingCols" :pagination="false" size="small" row-key="bindingId" :loading="detailLoading">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'enabled'">
                <span :class="['aa-enum', `aa-enum--${record.enabled === 1 ? 'magenta' : 'default'}`]">{{ record.enabled === 1 ? $t('project.on') : $t('project.off') }}</span>
              </template>
              <template v-if="column.key === 'action'">
                <Button type="primary" ghost size="small" @click="openBindEdit(record)">{{ $t('project.edit') }}</Button>
                <Button type="primary" ghost size="small" danger class="ml-1" @click="onBindDelete(record)">{{ $t('project.binding_unbind') }}</Button>
              </template>
            </template>
          </Table>
        </Tabs.TabPane>
      </Tabs>
    </Modal>

    <Modal v-model:open="bindModalVisible" :title="$t('project.binding_add')" :width="520" :destroy-on-close="true" @ok="onBindSubmit">
      <div class="space-y-3 py-2">
        <div><b>{{ $t('project.binding_kind') }}</b>
          <Select v-model:value="bindForm.kind" :options="BINDING_KIND_OPTIONS" class="w-full" />
        </div>
        <div><b>{{ $t('project.binding_target_id') }}</b>
          <Input v-model:value="bindForm.targetId" :placeholder="$t('project.binding_target_id_ph')" />
        </div>
        <div><b>{{ $t('project.binding_target_name') }}</b>
          <Input v-model:value="bindForm.targetName" :placeholder="$t('project.binding_target_name_ph')" />
        </div>
      </div>
    </Modal>
  </Page>
</template>
