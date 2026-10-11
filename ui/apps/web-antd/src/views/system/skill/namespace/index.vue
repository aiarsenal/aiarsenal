<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Select, Switch, Table } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addSkillNamespaceApi,
  addSkillNamespaceMemberApi,
  deleteSkillNamespaceApi,
  editSkillNamespaceApi,
  getSkillNamespaceListApi,
  getSkillNamespaceMembersApi,
  removeSkillNamespaceMemberApi,
} from '#/api/system/skill/namespace';

defineOptions({ name: 'SystemSkillNamespace' });

// 类型/状态 下拉（computed：随语言切换实时变更 label）
const typeOptions = computed(() => [
  { label: $t('skill.ns_typeTeam'), value: 'TEAM' },
  { label: $t('skill.ns_typeGlobal'), value: 'GLOBAL' },
]);
const statusOptions = computed(() => [
  { label: $t('skill.ns_enabled'), value: 1 },
  { label: $t('skill.ns_disabled'), value: 0 },
]);
const roleOptions = computed(() => [
  { label: $t('skill.ns_roleOwner'), value: 'OWNER' },
  { label: $t('skill.ns_roleAdmin'), value: 'ADMIN' },
  { label: $t('skill.ns_roleMember'), value: 'MEMBER' },
]);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'slug',
    label: $t('skill.ns_slug'),
    componentProps: { placeholder: $t('skill.ns_slugPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('skill.ns_name'),
    componentProps: { placeholder: $t('skill.ns_namePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'type',
    label: $t('skill.ns_type'),
    componentProps: {
      allowClear: true,
      options: typeOptions.value,
      style: { width: '100%' },
    },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('skill.ns_status'),
    componentProps: {
      allowClear: true,
      options: statusOptions.value,
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('skill.ns_seq'), width: 60 },
    { field: 'slug', title: $t('skill.ns_slug'), minWidth: 140 },
    { field: 'displayName', title: $t('skill.ns_name'), minWidth: 140 },
    { field: 'type', title: $t('skill.ns_type'), width: 100, slots: { default: 'type' } },
    {
      field: 'status',
      title: $t('skill.ns_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'createdAt', title: $t('skill.ns_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('skill.ns_action'),
      width: 220,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getSkillNamespaceListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin: dto.SkillNamespaceSearchRes → { list, total }
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: {
    custom: true,
    refresh: { code: 'query' },
  },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('skill.ns_modalTitleEdit') : $t('skill.ns_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  ...(isEdit.value
    ? []
    : [
        {
          component: 'Input' as const,
          fieldName: 'slug',
          label: $t('skill.ns_slug'),
          rules: 'required',
          componentProps: { placeholder: $t('skill.ns_slugPlaceholder') },
        },
      ]),
  {
    component: 'Input' as const,
    fieldName: 'displayName',
    label: $t('skill.ns_name'),
    rules: 'required',
    componentProps: { placeholder: $t('skill.ns_namePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'type',
    label: $t('skill.ns_type'),
    rules: 'required',
    componentProps: () => ({
      disabled: isEdit.value, // 编辑时不可改类型
      options: typeOptions.value,
      placeholder: $t('skill.ns_typePlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'description',
    label: $t('skill.ns_description'),
    componentProps: { placeholder: $t('skill.ns_descriptionPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'status',
    label: $t('skill.ns_status'),
    componentProps: {
      options: statusOptions.value,
      style: { width: '100%' },
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ type: 'TEAM', status: 1 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.namespaceId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    slug: row.slug,
    displayName: row.displayName,
    type: row.type,
    description: row.description,
    status: row.status,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin edit: { namespaceId, ...SkillNamespaceReq }
      await editSkillNamespaceApi({ ...values, namespaceId: editingId.value });
      message.success($t('skill.ns_editSuccess'));
    } else {
      await addSkillNamespaceApi(values);
      message.success($t('skill.ns_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（业务字段原样回填），失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editSkillNamespaceApi({
      namespaceId: row.namespaceId,
      slug: row.slug,
      displayName: row.displayName,
      type: row.type,
      description: row.description || '',
      status: newStatus,
    });
    message.success($t('skill.ns_editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('skill.ns_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.namespaceId),
    rows.length,
  );
}
async function onDeleteRow(row: any) {
  await doDelete([row.namespaceId], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('skill.ns_tipTitle'),
    content: $t('skill.ns_confirmDeleteContent', { count }),
    okText: $t('skill.ns_confirm'),
    cancelText: $t('skill.ns_cancel'),
    okType: 'danger',
    onOk: async () => {
      // gin: DELETE body { id }（单条），批量逐条调用
      await Promise.all(ids.map((id) => deleteSkillNamespaceApi(id)));
      message.success($t('skill.ns_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 成员管理弹窗 =====
const memberVisible = ref(false);
const currentNs = ref<any>(null);
const members = ref<any[]>([]);
const memberCols = computed(() => [
  { title: $t('skill.ns_memberUserId'), dataIndex: 'userId', key: 'userId' },
  { title: $t('skill.ns_memberRole'), dataIndex: 'role', key: 'role', width: 120 },
  { title: $t('skill.ns_memberJoinedAt'), dataIndex: 'createdAt', key: 'createdAt', width: 180 },
  { title: $t('skill.ns_action'), key: 'action', width: 100 },
]);
const memberAdd = reactive({
  namespaceId: '',
  userId: '',
  role: 'MEMBER',
});

async function openMembers(row: any) {
  currentNs.value = row;
  memberAdd.namespaceId = row.namespaceId;
  memberAdd.userId = '';
  memberAdd.role = 'MEMBER';
  memberVisible.value = true;
  await loadMembers();
}
async function loadMembers() {
  try {
    const res = await getSkillNamespaceMembersApi(currentNs.value.namespaceId);
    members.value = res?.list ?? [];
  } catch {
    /* 忽略 */
  }
}
async function onAddMember() {
  if (!memberAdd.userId) {
    message.warning($t('skill.ns_memberUserIdRequired'));
    return;
  }
  try {
    await addSkillNamespaceMemberApi({
      namespaceId: memberAdd.namespaceId,
      userId: memberAdd.userId,
      role: memberAdd.role,
    });
    message.success($t('skill.ns_memberAddSuccess'));
    memberAdd.userId = '';
    memberAdd.role = 'MEMBER';
    await loadMembers();
  } catch {
    /* 拦截器已提示 */
  }
}
async function onRemoveMember(row: any) {
  Modal.confirm({
    title: $t('skill.ns_tipTitle'),
    content: $t('skill.ns_memberRemoveContent', { userId: row.userId }),
    okText: $t('skill.ns_confirm'),
    cancelText: $t('skill.ns_cancel'),
    okType: 'danger',
    onOk: async () => {
      await removeSkillNamespaceMemberApi(row.memberId);
      message.success($t('skill.ns_memberRemoveSuccess'));
      await loadMembers();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('skill.ns_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('skill.ns_batchDelete') }}</Button>
      </template>
      <template #type="{ row }">
        <span v-if="row.type === 'GLOBAL'" class="aa-enum aa-enum--warning">{{ $t('skill.ns_typeGlobal') }}</span>
        <span v-else class="aa-enum aa-enum--default">{{ $t('skill.ns_typeTeam') }}</span>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('skill.ns_enabled')"
          :un-checked-children="$t('skill.ns_disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('skill.ns_edit') }}</Button>
        <Button type="primary" ghost size="small" @click="openMembers(row)">
          {{ $t('skill.ns_member') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('skill.ns_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="560"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <Modal
      v-model:open="memberVisible"
      :title="$t('skill.ns_memberManageTitle', { name: currentNs?.displayName || '' })"
      :width="680"
      :destroy-on-close="true"
      :footer="null"
    >
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="memberAdd.userId"
          :placeholder="$t('skill.ns_memberUserIdPlaceholder')"
          style="width: 50%"
          allow-clear
        />
        <Select
          v-model:value="memberAdd.role"
          :options="roleOptions"
          style="width: 22%"
        />
        <Button type="primary" @click="onAddMember">{{ $t('skill.ns_memberAdd') }}</Button>
      </div>
      <Table
        :data-source="members"
        :columns="memberCols"
        :pagination="false"
        row-key="memberId"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            <Button
              type="primary" ghost
              size="small"
              danger
              @click="onRemoveMember(record)"
            >
              {{ $t('skill.ns_memberRemove') }}
            </Button>
          </template>
        </template>
      </Table>
    </Modal>
  </Page>
</template>
