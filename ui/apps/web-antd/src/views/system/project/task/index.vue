<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Select, Tag, Textarea } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { getProjectStatusListApi } from '#/api/system/project/config';
import { getProjectMemberListApi } from '#/api/system/project/member';
import { getProjectMilestoneListApi } from '#/api/system/project/milestone';
import {
  addProjectTaskApi,
  addProjectTaskCommentApi,
  deleteProjectTaskApi,
  editProjectTaskApi,
  getProjectTaskCommentsApi,
  getProjectTaskListApi,
} from '#/api/system/project/task';
import { $t } from '#/locales';
import { ensureProjects, PRIORITY_OPTIONS, priorityLabel } from '../_shared';

defineOptions({ name: 'SystemProjectTask' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');

// 项目内引用数据（状态流/成员/里程碑/可选父任务），随项目切换重拉
const statuses = ref<any[]>([]);
const members = ref<any[]>([]);
const milestones = ref<any[]>([]);
const parentTasks = ref<any[]>([]);

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length && !projectId.value) projectId.value = projects.value[0]!.projectId;
  await loadProjectRefs();
  gridApi.query();
});

async function loadProjectRefs() {
  if (!projectId.value) {
    statuses.value = [];
    members.value = [];
    milestones.value = [];
    parentTasks.value = [];
    return;
  }
  const pid = projectId.value;
  const [st, mb, ms, tk] = await Promise.all([
    getProjectStatusListApi({ projectId: pid, pageNum: 1, pageSize: 100 }).catch(() => null),
    getProjectMemberListApi({ projectId: pid, pageNum: 1, pageSize: 200 }).catch(() => null),
    getProjectMilestoneListApi({ projectId: pid, pageNum: 1, pageSize: 100 }).catch(() => null),
    getProjectTaskListApi({ projectId: pid, pageNum: 1, pageSize: 200 }).catch(() => null),
  ]);
  // 竞态守卫：响应回来时项目已再切换则丢弃
  if (pid !== projectId.value) return;
  statuses.value = st?.list ?? [];
  members.value = mb?.list ?? [];
  milestones.value = ms?.list ?? [];
  parentTasks.value = tk?.list ?? [];
}

watch(projectId, () => {
  loadProjectRefs();
  gridApi.query();
});

const statusOptions = computed(() =>
  statuses.value.map((s: any) => ({ label: s.name, value: s.key })),
);
const statusMap = computed(() => {
  const m: Record<string, any> = {};
  for (const s of statuses.value) m[s.key] = s;
  return m;
});
const memberOptions = computed(() =>
  members.value.map((m: any) => ({ label: m.userName || m.userId, value: m.userId })),
);
const milestoneOptions = computed(() =>
  milestones.value.map((m: any) => ({ label: m.name, value: m.milestoneId })),
);
const parentTaskOptions = computed(() =>
  parentTasks.value
    .filter((t: any) => !t.parentTaskId)
    .map((t: any) => ({ label: t.title, value: t.taskId })),
);
const milestoneName = (id: string) =>
  (id && milestones.value.find((m: any) => m.milestoneId === id)?.name) || '';

const PRIORITY_COLOR: Record<string, string> = { urgent: 'error', high: 'warning', medium: 'processing', low: 'default' };

const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select', fieldName: 'status', label: $t('project.task_status'),
    componentProps: { allowClear: true, options: statusOptions.value, placeholder: $t('project.all'), style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'priority', label: $t('project.priority'),
    componentProps: { allowClear: true, options: PRIORITY_OPTIONS.value, placeholder: $t('project.all'), style: { width: '100%' } },
  },
]);

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { field: 'title', title: $t('project.task_title'), minWidth: 200 },
    { field: 'status', title: $t('project.task_status'), width: 110, slots: { default: 'status' } },
    { field: 'priority', title: $t('project.priority'), width: 90, slots: { default: 'priority' } },
    { field: 'assigneeId', title: $t('project.task_assignee'), width: 110, slots: { default: 'assignee' } },
    { field: 'milestoneId', title: $t('project.task_milestone'), width: 130, slots: { default: 'milestone' } },
    { field: 'dueDate', title: $t('project.dueDate'), minWidth: 150 },
    { field: 'estimateHours', title: $t('project.task_estimate'), width: 90 },
    { field: 'action', title: $t('project.action'), width: 210, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        if (!projectId.value) return { items: [], total: 0 };
        const res = await getProjectTaskListApi({
          pageNum: page.currentPage, pageSize: 20, projectId: projectId.value, ...(form as any),
        });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions: { schema: searchSchema } });

// ===== 新增/编辑 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() => (isEdit.value ? $t('project.task_edit_title') : $t('project.task_add_title')));

const formSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input', fieldName: 'title', label: $t('project.task_title'), rules: 'required' },
  {
    component: 'Select', fieldName: 'milestoneId', label: $t('project.task_milestone'),
    componentProps: { allowClear: true, options: milestoneOptions.value, placeholder: $t('project.task_milestone_ph'), style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'status', label: $t('project.task_status'),
    componentProps: { allowClear: true, options: statusOptions.value, placeholder: $t('project.task_status_ph'), style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'priority', label: $t('project.priority'),
    componentProps: { options: PRIORITY_OPTIONS.value, style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'assigneeId', label: $t('project.task_assignee'),
    componentProps: { allowClear: true, showSearch: true, optionFilterProp: 'label', options: memberOptions.value, placeholder: $t('project.task_assignee_ph'), style: { width: '100%' } },
  },
  {
    component: 'Select', fieldName: 'parentTaskId', label: $t('project.task_parent'),
    componentProps: { allowClear: true, options: parentTaskOptions.value, placeholder: $t('project.task_parent_ph'), style: { width: '100%' } },
  },
  { component: 'DatePicker', fieldName: 'startDate', label: $t('project.startDate'), componentProps: { style: { width: '100%' } } },
  { component: 'DatePicker', fieldName: 'dueDate', label: $t('project.dueDate'), componentProps: { style: { width: '100%' } } },
  { component: 'InputNumber', fieldName: 'estimateHours', label: $t('project.task_estimate'), componentProps: { min: 0, style: { width: '100%' } } },
  { component: 'InputNumber', fieldName: 'spentHours', label: $t('project.task_spent'), componentProps: { min: 0, style: { width: '100%' } } },
  {
    component: 'Select', fieldName: 'tags', label: $t('project.task_tags'),
    componentProps: { mode: 'tags', open: false, tokenSeparators: [','], placeholder: $t('project.task_tags_ph'), style: { width: '100%' } },
  },
  { component: 'Textarea', fieldName: 'description', label: $t('project.description'), componentProps: { rows: 2, style: { width: '100%' } } },
]);

const [Form, formApi] = useVbenForm(
  reactive({ commonConfig: { fullWidth: true, hideLabel: false }, schema: formSchema, showDefaultActions: false }),
);

/** 表单 tags 值（数组）↔ 后端 jsonb 字符串 */
function parseTags(v: any): string[] {
  if (Array.isArray(v)) return v;
  try {
    const arr = JSON.parse(v || '[]');
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

async function openAdd() {
  if (!projectId.value) { message.warning($t('project.select_project_first')); return; }
  isEdit.value = false; editingId.value = ''; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  // 默认状态取项目状态流的 isDefault 项（后端也会兜底）
  const def = statuses.value.find((s: any) => s.isDefault === 1)?.key ?? '';
  await formApi.setValues({ priority: 'medium', status: def, tags: [] });
}
async function openEdit(row: any) {
  isEdit.value = true; editingId.value = row.taskId; modalVisible.value = true;
  await nextTick(); await formApi.resetForm();
  await formApi.setValues({
    title: row.title, milestoneId: row.milestoneId ?? '', priority: row.priority ?? 'medium', status: row.status ?? '',
    assigneeId: row.assigneeId ?? '', parentTaskId: row.parentTaskId ?? '',
    startDate: row.startDate ?? '', dueDate: row.dueDate ?? '',
    estimateHours: row.estimateHours ?? 0, spentHours: row.spentHours ?? 0,
    tags: parseTags(row.tags), description: row.description ?? '',
  });
}
async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const payload = { ...values, tags: JSON.stringify(values.tags ?? []) };
  try {
    if (isEdit.value) {
      await editProjectTaskApi({ ...payload, projectId: projectId.value, taskId: editingId.value });
      message.success($t('project.edit_success'));
    } else {
      await addProjectTaskApi({ ...payload, projectId: projectId.value });
      message.success($t('project.add_success'));
    }
    modalVisible.value = false;
    gridApi.query();
    loadProjectRefs(); // 父任务下拉选项随新建任务更新
  } catch { /* */ }
}
function onDelete(row: any) {
  Modal.confirm({
    title: $t('project.confirm_delete_title'), okType: 'danger',
    okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => {
      await deleteProjectTaskApi([row.taskId], projectId.value);
      message.success($t('project.delete_success'));
      gridApi.query();
      loadProjectRefs();
    },
  });
}

// ===== 评论抽屉 =====
const commentVisible = ref(false);
const commentTaskId = ref('');
const comments = ref<any[]>([]);
const commentText = ref('');
async function openComments(row: any) {
  commentTaskId.value = row.taskId;
  commentVisible.value = true;
  commentText.value = '';
  const res = await getProjectTaskCommentsApi(row.taskId);
  comments.value = res?.list ?? [];
}
async function postComment() {
  if (!commentText.value.trim()) return;
  await addProjectTaskCommentApi({ taskId: commentTaskId.value, projectId: projectId.value, content: commentText.value });
  commentText.value = '';
  const res = await getProjectTaskCommentsApi(commentTaskId.value);
  comments.value = res?.list ?? [];
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select
        v-model:value="projectId" :options="projects.map((p) => ({ label: p.name, value: p.projectId }))"
        style="width: 280px" :placeholder="$t('project.select_project')"
      />
    </div>
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('project.task_add') }}</Button>
      </template>
      <template #status="{ row }">
        <Tag :color="statusMap[row.status]?.color || 'blue'">
          {{ statusMap[row.status]?.name || row.status || '-' }}
        </Tag>
      </template>
      <template #priority="{ row }">
        <span :class="['aa-enum', `aa-enum--${PRIORITY_COLOR[row.priority] || 'default'}`]">{{ priorityLabel(row.priority) }}</span>
      </template>
      <template #assignee="{ row }">{{ row.assigneeName || row.assigneeId || '-' }}</template>
      <template #milestone="{ row }">{{ milestoneName(row.milestoneId) || '-' }}</template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openComments(row)">{{ $t('project.task_comment') }}</Button>
        <Button type="primary" ghost size="small" class="ml-1" @click="openEdit(row)">{{ $t('project.edit') }}</Button>
        <Button type="primary" ghost size="small" danger class="ml-1" @click="onDelete(row)">{{ $t('project.delete') }}</Button>
      </template>
    </Grid>

    <Modal v-model:open="modalVisible" :title="modalTitle" :width="640" :destroy-on-close="true" @ok="onSubmit">
      <Form />
    </Modal>

    <Modal v-model:open="commentVisible" :title="$t('project.task_comment_title')" :width="600" :footer="null" :destroy-on-close="true">
      <div class="mb-3">
        <Textarea v-model:value="commentText" :rows="3" :placeholder="$t('project.task_comment_ph')" />
        <div class="mt-2 text-right">
          <Button type="primary" size="small" @click="postComment">{{ $t('project.task_comment_post') }}</Button>
        </div>
      </div>
      <div class="space-y-2">
        <div v-for="c in comments" :key="c.commentId" class="rounded border p-2">
          <div class="text-gray-500 text-xs">{{ c.userName || c.userId }} · {{ c.createdAt }}</div>
          <div class="mt-1 whitespace-pre-wrap">{{ c.content }}</div>
        </div>
        <div v-if="!comments.length" class="text-center text-gray-400">{{ $t('project.task_comment_empty') }}</div>
      </div>
    </Modal>
  </Page>
</template>
