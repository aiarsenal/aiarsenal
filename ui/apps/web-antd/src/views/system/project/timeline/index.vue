<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Empty, Select, Spin, Table } from 'ant-design-vue';

import { $t } from '#/locales';
import { getProjectTimelineApi } from '#/api/system/project/project';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectTimeline' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');
const tasks = ref<any[]>([]);
const loading = ref(false);

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) {
    projectId.value = projects.value[0]!.projectId;
    await load();
  }
});

async function load() {
  if (!projectId.value) return;
  loading.value = true;
  try {
    const res = await getProjectTimelineApi(projectId.value);
    tasks.value = res?.list ?? [];
  } finally { loading.value = false; }
}

const STATUS_COLOR: Record<string, string> = { todo: 'default', doing: 'processing', done: 'magenta' };

const cols = computed(() => [
  { title: $t('project.task_title'), dataIndex: 'title', key: 'title' },
  { title: $t('project.task_status'), dataIndex: 'status', key: 'status', width: 110 },
  { title: $t('project.task_assignee'), dataIndex: 'assigneeId', key: 'assigneeId', width: 120 },
  { title: $t('project.startDate'), dataIndex: 'startDate', key: 'startDate', width: 140 },
  { title: $t('project.dueDate'), dataIndex: 'dueDate', key: 'dueDate', width: 140 },
  { title: $t('project.timeline_span'), key: 'span', width: 260 },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')" :options="projects.map((p) => ({ label: p.name, value: p.projectId }))" @change="load()" />
    </div>
    <Spin :spinning="loading">
      <Empty v-if="!tasks.length" :description="$t('project.timeline_empty')" />
      <Table v-else :data-source="tasks" :columns="cols" :pagination="{ pageSize: 50 }" size="small" row-key="taskId">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <span :class="['aa-enum', `aa-enum--${STATUS_COLOR[record.status] || 'default'}`]">{{ record.status || '-' }}</span>
          </template>
          <template v-if="column.key === 'span'">
            <div class="h-2 rounded bg-green-500" :title="`${record.startDate} ~ ${record.dueDate}`" />
          </template>
        </template>
      </Table>
    </Spin>
  </Page>
</template>
