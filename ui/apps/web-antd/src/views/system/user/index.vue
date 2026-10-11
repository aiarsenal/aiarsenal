<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  InputPassword,
  message,
  Modal,
  Switch,
} from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addUserApi,
  deleteUserApi,
  editUserApi,
  getDeptTreeApi,
  getEditUserApi,
  getUserListApi,
  getUserParamsApi,
  resetUserPwdApi,
  setUserStatusApi,
} from '#/api/system/user';

defineOptions({ name: 'SystemUser' });

// ===== 部门/角色/岗位 下拉数据（getUserParamsApi）=====
const params = reactive<{
  depts: any[];
  roleList: any[];
  posts: any[];
}>({ depts: [], roleList: [], posts: [] });

async function loadParams() {
  try {
    const [p, d] = await Promise.all([getUserParamsApi(), getDeptTreeApi()]);
    // gin: /user/params → { roleList, posts }；/dept/treeSelect → { deps }
    params.roleList = (p as any)?.roleList ?? [];
    params.posts = (p as any)?.posts ?? [];
    params.depts = (d as any)?.deps ?? [];
  } catch {
    /* 忽略 */
  }
}
onMounted(loadParams);

// 部门树 → TreeSelect {value,label,children}
const deptTreeData = computed(() =>
  mapDeptTree(params.depts),
);
function mapDeptTree(list: any[]): any[] {
  return (list ?? []).map((d: any) => ({
    value: d.deptId,
    label: d.deptName,
    children: d.children?.length ? mapDeptTree(d.children) : undefined,
  }));
}
// 由叶 deptId 反查路径数组 [顶级, ..., 叶]（供 Cascader 回显）
function findDeptPath(deptId: string, list: any[]): string[] {
  for (const d of list ?? []) {
    if (d.deptId === deptId) return [d.deptId];
    if (d.children?.length) {
      const sub = findDeptPath(deptId, d.children);
      if (sub.length) return [d.deptId, ...sub];
    }
  }
  return [];
}
// gin Role json: { id (=roleId), name }
const roleOptions = computed(() =>
  (params.roleList ?? []).map((r: any) => ({ value: r.id, label: r.name })),
);
const postOptions = computed(() =>
  (params.posts ?? []).map((p: any) => ({ value: p.postId, label: p.postName })),
);

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyWords',
    label: $t('sysuser.keywords'),
    componentProps: { placeholder: $t('sysuser.keywordsPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'mobile',
    label: $t('sysuser.mobile'),
    componentProps: { placeholder: $t('sysuser.mobilePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('sysuser.status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('sysuser.enabled'), value: 1 },
        { label: $t('sysuser.disabled'), value: 0 },
      ],
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('sysuser.seq'), width: 60 },
    { field: 'employeeNo', title: $t('sysuser.employeeNo'), minWidth: 100 },
    { field: 'name', title: $t('sysuser.name'), minWidth: 100 },
    { field: 'userName', title: $t('sysuser.userName'), minWidth: 110 },
    { field: 'userNickname', title: $t('sysuser.userNickname'), minWidth: 100 },
    { field: 'dept', title: $t('sysuser.dept'), minWidth: 100, slots: { default: 'dept' } },
    { field: 'mobile', title: $t('sysuser.mobile'), minWidth: 120 },
    {
      field: 'userStatus',
      title: $t('sysuser.status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'createdAt', title: $t('sysuser.createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('sysuser.action'),
      width: 240,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getUserListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        return { items: res?.userList ?? [], total: res?.total ?? 0 };
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
  isEdit.value ? $t('sysuser.modalTitleEdit') : $t('sysuser.modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  ...(isEdit.value
    ? []
    : [
        {
          component: 'Input' as const,
          fieldName: 'userName',
          label: $t('sysuser.userNameLabel'),
          rules: 'required',
          componentProps: { placeholder: $t('sysuser.userNamePlaceholder') },
        },
        {
          component: 'InputPassword' as const,
          fieldName: 'password',
          label: $t('sysuser.password'),
          rules: 'required',
          componentProps: { placeholder: $t('sysuser.passwordPlaceholder') },
        },
      ]),
  {
    component: 'Input' as const,
    fieldName: 'employeeNo',
    label: $t('sysuser.employeeNoLabel'),
    rules: 'required',
    componentProps: { placeholder: $t('sysuser.employeeNoPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('sysuser.nameLabel'),
    rules: 'required',
    componentProps: { placeholder: $t('sysuser.namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'nickName',
    label: $t('sysuser.nickName'),
    rules: 'required',
    componentProps: { placeholder: $t('sysuser.nickNamePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'roleIds',
    label: $t('sysuser.role'),
    componentProps: () => ({
      mode: 'multiple',
      options: roleOptions.value,
      placeholder: $t('sysuser.rolePlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Cascader' as const,
    fieldName: 'deptId',
    label: $t('sysuser.deptLabel'),
    rules: z
      .array(z.string())
      .min(1, $t('sysuser.deptRequired')),
    componentProps: () => ({
      options: deptTreeData.value,
      placeholder: $t('sysuser.deptPlaceholder'),
      changeOnSelect: true,
      allowClear: true,
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'mobile',
    label: $t('sysuser.mobileLabel'),
    rules: 'required',
    componentProps: { placeholder: $t('sysuser.mobileLabelPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'email',
    label: $t('sysuser.email'),
    componentProps: { placeholder: $t('sysuser.emailPlaceholder') },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'sex',
    label: $t('sysuser.sex'),
    componentProps: {
      options: [
        { label: $t('sysuser.sexMale'), value: 1 },
        { label: $t('sysuser.sexFemale'), value: 0 },
        { label: $t('sysuser.sexUnknown'), value: 2 },
      ],
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'postIds',
    label: $t('sysuser.post'),
    componentProps: () => ({
      mode: 'multiple',
      options: postOptions.value,
      placeholder: $t('sysuser.postPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Select' as const,
    fieldName: 'status',
    label: $t('sysuser.status'),
    componentProps: {
      options: [
        { label: $t('sysuser.enabled'), value: 1 },
        { label: $t('sysuser.disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
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
  formApi.setValues({ status: 1, sex: 1 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  try {
    const res: any = await getEditUserApi(row.id);
    // gin: { user, checkedRoleIds, checkedPosts }
    const u = res?.user ?? {};
    modalVisible.value = true;
    await nextTick();
    await formApi.resetForm();
    formApi.setValues({
      employeeNo: u.employeeNo,
      name: u.name,
      nickName: u.userNickname,
      mobile: u.mobile,
      email: u.userEmail,
      deptId: findDeptPath(u.deptId, params.depts),
      sex: u.sex,
      status: u.userStatus,
      roleIds: res?.checkedRoleIds ?? [],
      postIds: res?.checkedPosts ?? [],
    });
  } catch (e) {
    message.error($t('sysuser.getUserInfoFailed'));
  }
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // Cascader 的 deptId 是路径数组，取叶节点 id 提交
  if (Array.isArray(values.deptId)) {
    values.deptId = values.deptId[values.deptId.length - 1];
  }
  try {
    if (isEdit.value) {
      // gin edit 需要 userId（= 行 id）
      await editUserApi({ ...values, userId: editingId.value });
      message.success($t('sysuser.editSuccess'));
    } else {
      await addUserApi(values);
      message.success($t('sysuser.addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('sysuser.selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.id), rows.length);
}
async function onDeleteRow(row: any) {
  await doDelete([row.id], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('sysuser.confirmDeleteTitle'),
    content: $t('sysuser.confirmDeleteContent', { count }),
    okText: $t('sysuser.confirm'),
    cancelText: $t('sysuser.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteUserApi(ids);
      message.success($t('sysuser.deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 状态切换 =====
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  try {
    await setUserStatusApi(row.id, newStatus);
    row.userStatus = newStatus;
    message.success(newStatus === 1 ? $t('sysuser.enableSuccess') : $t('sysuser.disableSuccess'));
  } catch {
    row.userStatus = newStatus === 1 ? 0 : 1;
  }
}

// ===== 重置密码 =====
const resetPwdVisible = ref(false);
const resetPwdRow = ref<any>(null);
const newPwd = ref('');

function onResetPwd(row: any) {
  resetPwdRow.value = row;
  newPwd.value = '';
  resetPwdVisible.value = true;
}
async function onResetPwdConfirm() {
  if (!newPwd.value) {
    message.warning($t('sysuser.pwdRequired'));
    return;
  }
  await resetUserPwdApi(resetPwdRow.value.id, newPwd.value);
  message.success($t('sysuser.resetSuccess'));
  resetPwdVisible.value = false;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('sysuser.add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('sysuser.addBatchDelete') }}</Button>
      </template>
      <template #dept="{ row }">
        {{ row.dept?.name || '-' }}
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.userStatus === 1"
          :checked-children="$t('sysuser.switchEnabled')"
          :un-checked-children="$t('sysuser.switchDisabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('sysuser.edit') }}</Button>
        <Button type="primary" ghost size="small" @click="onResetPwd(row)">
          {{ $t('sysuser.resetPwd') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('sysuser.delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="720"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <Modal
      v-model:open="resetPwdVisible"
      :title="$t('sysuser.resetPwdTitle')"
      :width="420"
      @ok="onResetPwdConfirm"
    >
      <InputPassword
        v-model:value="newPwd"
        :placeholder="$t('sysuser.resetPwdPlaceholder')"
        autocomplete="new-password"
      />
    </Modal>
  </Page>
</template>
