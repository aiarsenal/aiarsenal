<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Card, Empty, message, Select, Spin, Tag } from 'ant-design-vue';

import { $t } from '#/locales';
import { getProjectBoardApi, moveProjectTaskApi } from '#/api/system/project/project';
import { ensureProjects, priorityLabel } from '../_shared';

defineOptions({ name: 'SystemProjectBoard' });

const PRIORITY_COLOR: Record<string, string> = { urgent: 'error', high: 'warning', medium: 'processing', low: 'default' };

const projects = ref<any[]>([]);
const projectId = ref<string>('');
const columns = ref<any[]>([]);
const tasks = ref<any[]>([]);
const loading = ref(false);

const draggedTaskId = ref<string>('');

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) {
    projectId.value = projects.value[0]!.projectId;
    await loadBoard();
  }
});

async function loadBoard() {
  if (!projectId.value) return;
  loading.value = true;
  try {
    const res = await getProjectBoardApi(projectId.value);
    columns.value = res?.columns ?? [];
    tasks.value = res?.tasks ?? [];
  } finally {
    loading.value = false;
  }
}

function tasksOf(key: string) {
  return tasks.value.filter((t) => (t.status || '') === key);
}

function onDragStart(taskId: string) {
  draggedTaskId.value = taskId;
}
async function onDrop(statusKey: string) {
  const id = draggedTaskId.value;
  draggedTaskId.value = '';
  if (!id) return;
  const task = tasks.value.find((t) => t.taskId === id);
  if (!task || task.status === statusKey) return;
  try {
    await moveProjectTaskApi({ taskId: id, status: statusKey, sortOrder: tasksOf(statusKey).length });
    // 乐观更新
    task.status = statusKey;
    message.success($t('project.board_moved'));
  } catch {
    await loadBoard();
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select
        v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')"
        :options="projects.map((p) => ({ label: p.name, value: p.projectId }))"
        @change="loadBoard()"
      />
    </div>

    <Spin :spinning="loading">
      <div v-if="columns.length" class="flex gap-3 overflow-x-auto pb-3">
        <div
          v-for="col in columns"
          :key="col.key"
          class="w-72 shrink-0"
          @dragover.prevent
          @drop="onDrop(col.key)"
        >
          <div class="mb-2 flex items-center gap-2">
            <Tag :color="col.color || 'default'">{{ col.name }}</Tag>
            <span class="text-gray-400 text-xs">({{ tasksOf(col.key).length }})</span>
          </div>
          <div class="min-h-40 space-y-2 rounded bg-gray-100 p-2 dark:bg-gray-800">
            <Card
              v-for="t in tasksOf(col.key)"
              :key="t.taskId"
              size="small"
              class="cursor-move"
              draggable="true"
              @dragstart="onDragStart(t.taskId)"
            >
              <div class="font-medium">{{ t.title }}</div>
              <div class="mt-1 flex items-center gap-2">
                <span v-if="t.priority" :class="['aa-enum', `aa-enum--${PRIORITY_COLOR[t.priority] || 'default'}`]">{{ priorityLabel(t.priority) }}</span>
                <span v-if="t.dueDate" class="text-gray-400 text-xs">{{ t.dueDate }}</span>
                <span v-if="t.assigneeId" class="text-gray-400 text-xs">{{ t.assigneeName || t.assigneeId }}</span>
              </div>
            </Card>
            <Empty v-if="!tasksOf(col.key).length" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
          </div>
        </div>
      </div>
      <Empty v-else :description="$t('project.board_empty')" />
    </Spin>
  </Page>
</template>
