<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Select } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { getProjectActivityListApi } from '#/api/system/project/project';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectActivity' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');

const ACTION_COLOR: Record<string, string> = {
  created: 'magenta', updated: 'processing', commented: 'default',
  status_changed: 'warning', assigned: 'blue', archived: 'error',
};

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) projectId.value = projects.value[0]!.projectId;
  gridApi.query();
});

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { field: 'createdAt', title: $t('project.activity_time'), minWidth: 160 },
    { field: 'action', title: $t('project.activity_action'), width: 130, slots: { default: 'action' } },
    { field: 'userId', title: $t('project.activity_user'), width: 130 },
    { field: 'target', title: $t('project.activity_target'), minWidth: 200 },
    { field: 'detail', title: $t('project.activity_detail'), minWidth: 200 },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page }: any) => {
        if (!projectId.value) return { items: [], total: 0 };
        const res = await getProjectActivityListApi({ pageNum: page.currentPage, pageSize: 20, projectId: projectId.value });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3 flex items-center">
      <span class="mr-2">{{ $t('project.project_label') }}</span>
      <Select v-model:value="projectId" style="width: 280px" :placeholder="$t('project.select_project')" :options="projects.map((p) => ({ label: p.name, value: p.projectId }))" @change="gridApi.query()" />
    </div>
    <Grid>
      <template #action="{ row }"><span :class="['aa-enum', `aa-enum--${ACTION_COLOR[row.action] || 'default'}`]">{{ row.action }}</span></template>
    </Grid>
  </Page>
</template>
