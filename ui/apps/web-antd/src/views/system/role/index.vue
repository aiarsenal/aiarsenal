<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch, Tree } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addRoleApi,
  deleteRoleApi,
  editRoleApi,
  getRoleApi,
  getRoleListApi,
  getRoleParamsApi,
} from '#/api/system/role';

defineOptions({ name: 'SystemRole' });

// ===== 菜单树（getRoleParamsApi → { menu }，扁平 id/pid，前端建树）=====
const menuRaw = ref<any[]>([]);
async function loadMenuTree() {
  try {
    const res: any = await getRoleParamsApi();
    menuRaw.value = res?.menu ?? [];
  } catch {
    /* 忽略 */
  }
}
onMounted(loadMenuTree);

// 扁平 list → 树 {key, title, children}
const menuTreeData = computed(() =>
  buildTree(menuRaw.value, 'id', 'pid', 'title'),
);
function buildTree(
  list: any[],
  idKey: string,
  pidKey: string,
  titleKey: string,
): any[] {
  const map = new Map<string, any>();
  const roots: any[] = [];
  list.forEach((it) => {
    map.set(it[idKey], {
      key: it[idKey],
      title: it[titleKey] || it.name || it[idKey],
      raw: it,
      children: [],
    });
  });
  list.forEach((it) => {
    const node = map.get(it[idKey])!;
    const parent = map.get(it[pidKey]);
    if (parent) parent.children.push(node);
    else roots.push(node);
  });
  return roots;
}

// 当前选中菜单 id（checkStrictly 模式，回显 1:1，无半选状态）
const checkedMenuKeys = ref<string[]>([]);
function onMenuCheck(checked: any) {
  // checkStrictly 下 antd 返回 { checked, halfChecked }；否则为 array
  if (Array.isArray(checked)) {
    checkedMenuKeys.value = checked as string[];
  } else {
    checkedMenuKeys.value = (checked?.checked ?? []) as string[];
  }
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'roleName',
    label: $t('sysrole.roleName'),
    componentProps: { placeholder: $t('sysrole.roleNamePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'roleStatus',
    label: $t('sysrole.status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('sysrole.enabled'), value: 1 },
        { label: $t('sysrole.disabled'), value: 0 },
      ],
    },
  },
]);

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('sysrole.seq'), width: 60 },
    { field: 'name', title: $t('sysrole.roleName'), minWidth: 120 },
    { field: 'listOrder', title: $t('sysrole.listOrder'), width: 90 },
    {
      field: 'status',
      title: $t('sysrole.status'),
      width: 100,
      slots: { default: 'status' },
    },
    { field: 'remark', title: $t('sysrole.remark'), minWidth: 160 },
    { field: 'createdAt', title: $t('sysrole.createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('sysrole.action'),
      width: 180,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getRoleListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
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
  isEdit.value ? $t('sysrole.modalTitleEdit') : $t('sysrole.modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('sysrole.roleName'),
    rules: 'required',
    componentProps: { placeholder: $t('sysrole.roleNamePlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'listOrder',
    label: $t('sysrole.listOrder'),
    componentProps: { min: 0, placeholder: $t('sysrole.listOrderPlaceholder'), style: { width: '100%' } },
  },
  {
    component: 'Select' as const,
    fieldName: 'status',
    label: $t('sysrole.status'),
    componentProps: {
      options: [
        { label: $t('sysrole.enabled'), value: 1 },
        { label: $t('sysrole.disabled'), value: 0 },
      ],
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'remark',
    label: $t('sysrole.remark'),
    componentProps: {
      placeholder: $t('sysrole.remarkPlaceholder'),
      maxlength: 150,
      rows: 3,
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
  checkedMenuKeys.value = [];
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ status: 1, listOrder: 0 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  try {
    const res: any = await getRoleApi(row.id);
    const role: any = res?.role ?? {};
    modalVisible.value = true;
    await nextTick();
    await formApi.resetForm();
    formApi.setValues({
      name: role.name,
      listOrder: role.listOrder,
      status: role.status,
      remark: role.remark,
    });
    checkedMenuKeys.value = (res?.menuIds ?? []) as string[];
  } catch {
    message.error($t('sysrole.getRoleInfoFailed'));
  }
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const menuIds = [...checkedMenuKeys.value];
  try {
    if (isEdit.value) {
      await editRoleApi({
        ...values,
        id: editingId.value,
        menuIds,
      });
      message.success($t('sysrole.editSuccess'));
    } else {
      await addRoleApi({ ...values, menuIds });
      message.success($t('sysrole.addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：先取详情保留 menuIds，再 edit 改 status，失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    const res: any = await getRoleApi(row.id);
    const role: any = res?.role ?? {};
    await editRoleApi({
      name: role.name,
      listOrder: role.listOrder,
      remark: role.remark,
      status: newStatus,
      id: row.id,
      menuIds: res?.menuIds ?? [],
    });
    message.success($t('sysrole.editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('sysrole.selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.id), rows.length);
}
async function onDeleteRow(row: any) {
  await doDelete([row.id], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('sysrole.confirmDeleteTitle'),
    content: $t('sysrole.confirmDeleteContent', { count }),
    okText: $t('sysrole.confirm'),
    cancelText: $t('sysrole.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteRoleApi(ids);
      message.success($t('sysrole.deleteSuccess'));
      gridApi.query();
    },
  });
}

</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('sysrole.add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('sysrole.addBatchDelete') }}</Button>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('sysrole.enabled')"
          :un-checked-children="$t('sysrole.disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('sysrole.edit') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('sysrole.delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="680"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
      <div class="menu-tree-wrap">
        <div class="menu-tree-label">{{ $t('sysrole.menuPermissions') }}</div>
        <Tree
          :tree-data="menuTreeData"
          :checked-keys="checkedMenuKeys"
          :field-names="{ children: 'children', title: 'title', key: 'key' }"
          checkable
          :check-strictly="true"
          :default-expand-all="true"
          @check="onMenuCheck"
        />
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.menu-tree-wrap {
  margin-top: 16px;
  border: 1px solid #e5e6e7;
  border-radius: 4px;
  padding: 12px;
  max-height: 360px;
  overflow: auto;
}
.menu-tree-label {
  margin-bottom: 8px;
  font-weight: 500;
}
</style>
