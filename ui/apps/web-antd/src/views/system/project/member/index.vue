<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Select } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addProjectMemberApi,
  deleteProjectMemberApi,
  editProjectMemberApi,
  getProjectMemberListApi,
  MEMBER_ROLE_OPTIONS,
} from '#/api/system/project/member';
import { ensureProjects } from '../_shared';

defineOptions({ name: 'SystemProjectMember' });

const projects = ref<any[]>([]);
const projectId = ref<string>('');
const addVisible = ref(false);
const addUser = ref<string>('');
const addRole = ref<string>('member');

const ROLE_COLOR: Record<string, string> = { owner: 'error', maintainer: 'warning', member: 'processing', observer: 'default' };

onMounted(async () => {
  projects.value = await ensureProjects();
  if (projects.value.length) projectId.value = projects.value[0]!.projectId;
  gridApi.query();
});

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { field: 'userId', title: $t('project.member_user'), minWidth: 180 },
    { field: 'role', title: $t('project.member_role'), width: 120, slots: { default: 'role' } },
    { field: 'joinedAt', title: $t('project.member_joined'), minWidth: 160 },
    { field: 'action', title: $t('project.action'), width: 200, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page }: any) => {
        if (!projectId.value) return { items: [], total: 0 };
        const res = await getProjectMemberListApi({ pageNum: page.currentPage, pageSize: 20, projectId: projectId.value });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

function openAdd() {
  if (!projectId.value) { message.warning($t('project.select_project_first')); return; }
  addUser.value = ''; addRole.value = 'member'; addVisible.value = true;
}
async function onSubmitAdd() {
  if (!addUser.value) { message.warning($t('project.member_user_required')); return; }
  await addProjectMemberApi({ projectId: projectId.value, userId: addUser.value, role: addRole.value });
  message.success($t('project.add_success'));
  addVisible.value = false; gridApi.query();
}
function onRoleChange(row: any, role: string) {
  editProjectMemberApi({ memberId: row.memberId, role }).then(() => {
    row.role = role; message.success($t('project.edit_success'));
  });
}
function onDelete(row: any) {
  Modal.confirm({
    title: $t('project.confirm_delete_title'), okType: 'danger', okText: $t('project.confirm'), cancelText: $t('project.cancel'),
    onOk: async () => { await deleteProjectMemberApi([row.memberId]); message.success($t('project.delete_success')); gridApi.query(); },
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
      <template #toolbar-actions><Button type="primary" @click="openAdd">{{ $t('project.member_add') }}</Button></template>
      <template #role="{ row }"><span :class="['aa-enum', `aa-enum--${ROLE_COLOR[row.role] || 'default'}`]">{{ row.role }}</span></template>
      <template #action="{ row }">
        <Select :value="row.role" size="small" style="width: 110px" :options="MEMBER_ROLE_OPTIONS" @change="(v: any) => onRoleChange(row, v)" />
        <Button type="primary" ghost size="small" danger class="ml-1" @click="onDelete(row)">{{ $t('project.member_remove') }}</Button>
      </template>
    </Grid>

    <Modal v-model:open="addVisible" :title="$t('project.member_add')" :width="480" :destroy-on-close="true" @ok="onSubmitAdd">
      <div class="space-y-3 py-2">
        <div><b>{{ $t('project.member_user_id') }}</b><input v-model="addUser" class="mt-1 w-full rounded border px-2 py-1" :placeholder="$t('project.member_user_id_ph')" /></div>
        <div><b>{{ $t('project.member_role') }}</b>
          <Select v-model:value="addRole" :options="MEMBER_ROLE_OPTIONS" class="mt-1 w-full" />
        </div>
      </div>
    </Modal>
  </Page>
</template>
