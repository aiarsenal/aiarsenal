<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Select, Table, Tabs, Tag } from 'ant-design-vue';

import { $t } from '#/locales';
import {
  addProjectFieldApi,
  addProjectStatusApi,
  addProjectTagApi,
  deleteProjectFieldApi,
  deleteProjectStatusApi,
  deleteProjectTagApi,
  editProjectFieldApi,
  editProjectStatusApi,
  editProjectTagApi,
  FIELD_TYPE_OPTIONS,
  getProjectFieldListApi,
  getProjectStatusListApi,
  getProjectTagListApi,
  STATUS_CATEGORY_OPTIONS,
} from '#/api/system/project/config';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectConfig' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');
const activeTab = ref('tag');

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) projectId.value = projects.value[0]!.projectId;
  reloadAll();
});

// ===== 标签 =====
const tags = ref<any[]>([]);
async function loadTags() {
  if (!projectId.value) return (tags.value = []);
  const res = await getProjectTagListApi({ projectId: projectId.value, pageNum: 1, pageSize: 200 });
  tags.value = res?.list ?? [];
}
const tagModal = ref(false);
const tagEdit = ref<any>({});
function openTag(row?: any) {
  tagEdit.value = row
    ? { tagId: row.tagId, name: row.name, color: row.color, sortOrder: row.sortOrder }
    : { name: '', color: 'default', sortOrder: 0 };
  tagModal.value = true;
}
async function submitTag() {
  if (!tagEdit.value.name) { message.warning($t('project.name_required')); return; }
  if (tagEdit.value.tagId) await editProjectTagApi({ ...tagEdit.value, projectId: projectId.value });
  else await addProjectTagApi({ ...tagEdit.value, projectId: projectId.value });
  message.success($t('project.edit_success')); tagModal.value = false; await loadTags();
}
function deleteTag(row: any) {
  Modal.confirm({ title: $t('project.confirm_delete_title'), okType: 'danger', okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => { await deleteProjectTagApi([row.tagId]); message.success($t('project.delete_success')); await loadTags(); } });
}

// ===== 状态流 =====
const statuses = ref<any[]>([]);
async function loadStatus() {
  if (!projectId.value) return (statuses.value = []);
  const res = await getProjectStatusListApi({ projectId: projectId.value, pageNum: 1, pageSize: 200 });
  statuses.value = res?.list ?? [];
}
const statusModal = ref(false);
const statusEdit = ref<any>({});
function openStatus(row?: any) {
  statusEdit.value = row
    ? { statusId: row.statusId, key: row.key, name: row.name, color: row.color, category: row.category, sortOrder: row.sortOrder, isDefault: row.isDefault }
    : { key: '', name: '', color: 'default', category: 'todo', sortOrder: 0, isDefault: 0 };
  statusModal.value = true;
}
async function submitStatus() {
  if (!statusEdit.value.key || !statusEdit.value.name) { message.warning($t('project.status_key_required')); return; }
  if (statusEdit.value.statusId) await editProjectStatusApi({ ...statusEdit.value, projectId: projectId.value });
  else await addProjectStatusApi({ ...statusEdit.value, projectId: projectId.value });
  message.success($t('project.edit_success')); statusModal.value = false; await loadStatus();
}
function deleteStatus(row: any) {
  Modal.confirm({ title: $t('project.confirm_delete_title'), okType: 'danger', okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => { await deleteProjectStatusApi([row.statusId]); message.success($t('project.delete_success')); await loadStatus(); } });
}

// ===== 自定义字段 =====
const fields = ref<any[]>([]);
async function loadFields() {
  if (!projectId.value) return (fields.value = []);
  const res = await getProjectFieldListApi({ projectId: projectId.value, pageNum: 1, pageSize: 200 });
  fields.value = res?.list ?? [];
}
const fieldModal = ref(false);
const fieldEdit = ref<any>({});
function openField(row?: any) {
  fieldEdit.value = row
    ? { fieldId: row.fieldId, name: row.name, type: row.type, options: row.options, sortOrder: row.sortOrder }
    : { name: '', type: 'text', options: '[]', sortOrder: 0 };
  fieldModal.value = true;
}
async function submitField() {
  if (!fieldEdit.value.name || !fieldEdit.value.type) { message.warning($t('project.field_required')); return; }
  if (fieldEdit.value.fieldId) await editProjectFieldApi({ ...fieldEdit.value, projectId: projectId.value });
  else await addProjectFieldApi({ ...fieldEdit.value, projectId: projectId.value });
  message.success($t('project.edit_success')); fieldModal.value = false; await loadFields();
}
function deleteField(row: any) {
  Modal.confirm({ title: $t('project.confirm_delete_title'), okType: 'danger', okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => { await deleteProjectFieldApi([row.fieldId]); message.success($t('project.delete_success')); await loadFields(); } });
}

function reloadAll() { loadTags(); loadStatus(); loadFields(); }
const projectOptions = computed(() => projects.value.map((p) => ({ label: p.name, value: p.projectId })));

const tagCols = computed(() => [
  { title: $t('project.name'), dataIndex: 'name', key: 'name' },
  { title: $t('project.color'), dataIndex: 'color', key: 'color', width: 100 },
  { title: $t('project.sort'), dataIndex: 'sortOrder', key: 'sortOrder', width: 80 },
  { title: $t('project.action'), key: 'action', width: 140 },
]);
const statusCols = computed(() => [
  { title: $t('project.status_key'), dataIndex: 'key', key: 'key' },
  { title: $t('project.name'), dataIndex: 'name', key: 'name' },
  { title: $t('project.color'), dataIndex: 'color', key: 'color', width: 100 },
  { title: $t('project.status_category'), dataIndex: 'category', key: 'category', width: 100 },
  { title: $t('project.action'), key: 'action', width: 140 },
]);
const fieldCols = computed(() => [
  { title: $t('project.name'), dataIndex: 'name', key: 'name' },
  { title: $t('project.field_type'), dataIndex: 'type', key: 'type', width: 120 },
  { title: $t('project.sort'), dataIndex: 'sortOrder', key: 'sortOrder', width: 80 },
  { title: $t('project.action'), key: 'action', width: 140 },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')" :options="projectOptions" @change="reloadAll()" />
    </div>

    <Tabs v-model:active-key="activeTab">
      <!-- 标签 -->
      <Tabs.TabPane key="tag" :tab="$t('project.config_tag')">
        <div class="mb-2"><Button type="primary" size="small" @click="openTag()">{{ $t('project.tag_add') }}</Button></div>
        <Table :data-source="tags" :columns="tagCols" :pagination="false" size="small" row-key="tagId">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'color'"><Tag :color="record.color">{{ record.color }}</Tag></template>
            <template v-if="column.key === 'action'">
              <Button type="primary" ghost size="small" @click="openTag(record)">{{ $t('project.edit') }}</Button>
              <Button type="primary" ghost size="small" danger class="ml-1" @click="deleteTag(record)">{{ $t('project.delete') }}</Button>
            </template>
          </template>
        </Table>
      </Tabs.TabPane>

      <!-- 状态流 -->
      <Tabs.TabPane key="status" :tab="$t('project.config_status')">
        <div class="mb-2"><Button type="primary" size="small" @click="openStatus()">{{ $t('project.status_add') }}</Button></div>
        <Table :data-source="statuses" :columns="statusCols" :pagination="false" size="small" row-key="statusId">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'color'"><Tag :color="record.color">{{ record.color }}</Tag></template>
            <template v-if="column.key === 'action'">
              <Button type="primary" ghost size="small" @click="openStatus(record)">{{ $t('project.edit') }}</Button>
              <Button type="primary" ghost size="small" danger class="ml-1" @click="deleteStatus(record)">{{ $t('project.delete') }}</Button>
            </template>
          </template>
        </Table>
      </Tabs.TabPane>

      <!-- 自定义字段 -->
      <Tabs.TabPane key="field" :tab="$t('project.config_field')">
        <div class="mb-2"><Button type="primary" size="small" @click="openField()">{{ $t('project.field_add') }}</Button></div>
        <Table :data-source="fields" :columns="fieldCols" :pagination="false" size="small" row-key="fieldId">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'action'">
              <Button type="primary" ghost size="small" @click="openField(record)">{{ $t('project.edit') }}</Button>
              <Button type="primary" ghost size="small" danger class="ml-1" @click="deleteField(record)">{{ $t('project.delete') }}</Button>
            </template>
          </template>
        </Table>
      </Tabs.TabPane>
    </Tabs>

    <!-- 标签弹窗 -->
    <Modal v-model:open="tagModal" :title="$t('project.tag_add')" :width="480" :destroy-on-close="true" @ok="submitTag">
      <div class="space-y-2 py-2">
        <input v-model="tagEdit.name" class="w-full rounded border px-2 py-1" :placeholder="$t('project.name')" />
        <input v-model="tagEdit.color" class="w-full rounded border px-2 py-1" :placeholder="$t('project.color')" />
        <input v-model.number="tagEdit.sortOrder" type="number" class="w-full rounded border px-2 py-1" :placeholder="$t('project.sort')" />
      </div>
    </Modal>
    <!-- 状态弹窗 -->
    <Modal v-model:open="statusModal" :title="$t('project.status_add')" :width="520" :destroy-on-close="true" @ok="submitStatus">
      <div class="space-y-2 py-2">
        <input v-model="statusEdit.key" class="w-full rounded border px-2 py-1" :placeholder="$t('project.status_key')" />
        <input v-model="statusEdit.name" class="w-full rounded border px-2 py-1" :placeholder="$t('project.name')" />
        <input v-model="statusEdit.color" class="w-full rounded border px-2 py-1" :placeholder="$t('project.color')" />
        <Select v-model:value="statusEdit.category" :options="STATUS_CATEGORY_OPTIONS" class="w-full" />
        <Select v-model:value="statusEdit.isDefault" :options="[{ label: $t('project.default_no'), value: 0 }, { label: $t('project.default_yes'), value: 1 }]" class="w-full" />
      </div>
    </Modal>
    <!-- 字段弹窗 -->
    <Modal v-model:open="fieldModal" :title="$t('project.field_add')" :width="480" :destroy-on-close="true" @ok="submitField">
      <div class="space-y-2 py-2">
        <input v-model="fieldEdit.name" class="w-full rounded border px-2 py-1" :placeholder="$t('project.name')" />
        <Select v-model:value="fieldEdit.type" :options="FIELD_TYPE_OPTIONS" class="w-full" />
        <input v-model="fieldEdit.options" class="w-full rounded border px-2 py-1" :placeholder='$t("project.field_options_ph")' />
      </div>
    </Modal>
  </Page>
</template>
