<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Card, Empty, Progress, Select, Spin, Statistic, Table, Tag } from 'ant-design-vue';

import { $t } from '#/locales';
import { getProjectReportApi } from '#/api/system/project/project';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectReport' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');
const data = ref<any>(null);
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
    data.value = await getProjectReportApi(projectId.value);
  } finally { loading.value = false; }
}

function statusEntries(obj: any) {
  return obj ? Object.entries(obj).map(([k, v]) => ({ key: k, count: v as number })) : [];
}

const milestoneCols = computed(() => [
  { title: $t('project.ms_name'), dataIndex: 'name', key: 'name' },
  { title: $t('project.report_total'), dataIndex: 'total', key: 'total', width: 90 },
  { title: $t('project.report_done'), dataIndex: 'done', key: 'done', width: 90 },
  { title: $t('project.progress'), key: 'progress', width: 200 },
]);
const memberLoadCols = computed(() => [
  { title: $t('project.member_user'), dataIndex: 'userId', key: 'userId' },
  { title: $t('project.report_task_count'), dataIndex: 'taskCount', key: 'taskCount', width: 110 },
  { title: $t('project.task_estimate'), dataIndex: 'estimateHours', key: 'estimateHours', width: 130 },
  { title: $t('project.report_spent'), dataIndex: 'spentHours', key: 'spentHours', width: 110 },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')" :options="projects.map((p) => ({ label: p.name, value: p.projectId }))" @change="load()" />
    </div>
    <Spin :spinning="loading">
      <Empty v-if="!data" :description="$t('project.report_empty')" />
      <div v-else class="space-y-4">
        <div class="flex flex-wrap gap-4">
          <Card class="min-w-48"><Statistic :title="$t('project.report_total_tasks')" :value="data.totalTasks ?? 0" /></Card>
          <Card class="min-w-64"><div class="text-gray-500 text-sm">{{ $t('project.report_done_rate') }}</div><Progress :percent="data.doneRate ?? 0" /></Card>
          <Card class="min-w-72">
            <div class="text-gray-500 text-sm">{{ $t('project.report_by_status') }}</div>
            <div class="mt-2 flex flex-wrap gap-2">
              <Tag v-for="s in statusEntries(data.byStatus)" :key="s.key">{{ s.key }}: {{ s.count }}</Tag>
            </div>
          </Card>
        </div>

        <Card :title="$t('project.report_milestone')">
          <Table :data-source="data.milestoneRate || []" :columns="milestoneCols" :pagination="false" size="small" row-key="milestoneId">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'progress'"><Progress :percent="record.progress" size="small" /></template>
            </template>
          </Table>
        </Card>

        <Card :title="$t('project.report_member_load')">
          <Table :data-source="data.memberLoad || []" :columns="memberLoadCols" :pagination="false" size="small" row-key="userId" />
        </Card>
      </div>
    </Spin>
  </Page>
</template>
