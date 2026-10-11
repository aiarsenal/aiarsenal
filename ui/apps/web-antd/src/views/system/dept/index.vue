<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addDeptApi,
  deleteDeptApi,
  editDeptApi,
  getDeptListApi,
  getDeptTreeApi,
  searchUserOptionsApi,
} from '#/api/system/dept';

defineOptions({ name: 'SystemDept' });

// ===== 部门树（父级 Cascader 用）=====
const deptTree = ref<any[]>([]);
async function loadDeptTree() {
  try {
    const res = await getDeptTreeApi();
    deptTree.value = (res as any)?.deps ?? [];
  } catch {
    /* 忽略 */
  }
}
onMounted(loadDeptTree);

// 部门树 → Cascader {value,label,children}
const deptCascaderOptions = computed(() => mapDeptTree(deptTree.value));
function mapDeptTree(list: any[]): any[] {
  return (list ?? []).map((d: any) => ({
    value: d.deptId,
    label: d.deptName,
    children: d.children?.length ? mapDeptTree(d.children) : undefined,
  }));
}
// 由叶 deptId 反查路径数组（回显用）
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

// ===== 负责人远程搜索（Select）=====
interface UserOption {
  id: string;
  name: string;
  employeeNo: string;
  mobile: string;
  userEmail: string;
}
const leaderOptions = ref<UserOption[]>([]);
const leaderLoading = ref(false);
async function searchLeaders(kw: string) {
  if (!kw) {
    leaderOptions.value = [];
    return;
  }
  leaderLoading.value = true;
  try {
    const res: any = await searchUserOptionsApi({
      keyWords: kw,
      pageNum: 1,
      pageSize: page.pageSize || 20,
    });
    leaderOptions.value = res?.list ?? [];
  } finally {
    leaderLoading.value = false;
  }
}

// ===== 搜索表单 =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'deptName',
    label: $t('sysdept.deptName'),
    componentProps: { placeholder: $t('sysdept.deptNamePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('sysdept.status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('sysdept.enabled'), value: 1 },
        { label: $t('sysdept.disabled'), value: 0 },
      ],
    },
  },
]);

// ===== 表格（树形）=====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('sysdept.seq'), width: 60, fixed: 'left' },
    {
      field: 'deptName',
      title: $t('sysdept.deptName'),
      minWidth: 200,
      treeNode: true,
    },
    {
      field: 'leaderName',
      title: $t('sysdept.leader'),
      minWidth: 160,
      slots: { default: 'leader' },
    },
    {
      field: 'status',
      title: $t('sysdept.status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'orderNum', title: $t('sysdept.orderNum'), width: 90 },
    { field: 'createdAt', title: $t('sysdept.createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('sysdept.action'),
      width: 220,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: false },
  // rowConfig.keyField 必须设为稳定唯一键(deptId)，否则 vxe-table 的 reserve 会落到内部自增
  // _X_ROW_KEY，重载后对不上 → 展开状态被清空（官方缺此配置会告警 vxe.error.reqProp）。
  rowConfig: { keyField: 'deptId' },
  // reserve:true 配合上面的 keyField，刷新/重载后按 deptId 自动恢复树的展开状态。
  treeConfig: {
    transform: true,
    rowField: 'deptId',
    parentField: 'parentId',
    expandRowKeys: [],
    reserve: true,
  },
  proxyConfig: {
    ajax: {
      query: async ({ form }: any) => {
        const res = await getDeptListApi({ ...(form as any) });
        return { items: res?.deptList ?? [], total: res?.deptList?.length ?? 0 };
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
  isEdit.value ? $t('sysdept.modalTitleEdit') : $t('sysdept.modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Cascader' as const,
    fieldName: 'parentId',
    label: $t('sysdept.parentId'),
    componentProps: () => ({
      options: deptCascaderOptions.value,
      placeholder: $t('sysdept.parentIdPlaceholder'),
      changeOnSelect: true,
      allowClear: true,
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'deptName',
    label: $t('sysdept.deptNameLabel'),
    rules: 'required',
    componentProps: { placeholder: $t('sysdept.deptNameLabelPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'leaderId',
    label: $t('sysdept.leaderLabel'),
    componentProps: () => ({
      showSearch: true,
      // 远程搜索必须禁用本地过滤(filterOption:false)，否则按手机号/邮箱搜到的结果
      // 会因 label 里没有该字段被 Select 本地过滤掉、下拉为空（姓名/工号因在 label 里才正常）。
      filterOption: false,
      allowClear: true,
      options: leaderOptions.value.map((u) => ({
        value: u.id,
        // 补进手机号，便于按手机号/邮箱搜到后辨识具体是哪个人。
        label: u.mobile
          ? `${u.name}（${u.employeeNo}）· ${u.mobile}`
          : `${u.name}（${u.employeeNo}）`,
      })),
      loading: leaderLoading.value,
      placeholder: $t('sysdept.leaderPlaceholder'),
      style: { width: '100%' },
      onSearch: (v: string) => searchLeaders(v),
      onClear: () => {
        leaderOptions.value = [];
      },
    }),
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'orderNum',
    label: $t('sysdept.orderNumLabel'),
    componentProps: { min: 0, max: 999, placeholder: $t('sysdept.orderNumPlaceholder'), style: { width: '100%' } },
  },
  {
    component: 'Switch' as const,
    fieldName: 'status',
    label: $t('sysdept.status'),
    componentProps: {
      checkedValue: 1,
      unCheckedValue: 0,
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
  }) as any,
);

function resetLeaderOptions() {
  leaderOptions.value = [];
}

async function openAdd(parentRow?: any) {
  isEdit.value = false;
  editingId.value = '';
  resetLeaderOptions();
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  const init: any = { status: 1, orderNum: 0, parentId: [] };
  if (parentRow?.deptId) {
    init.parentId = findDeptPath(parentRow.deptId, deptTree.value);
  }
  formApi.setValues(init);
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.deptId;
  resetLeaderOptions();
  // 回显负责人：把列表带出的 leader 塞入选项避免空白
  if (row.leaderId) {
    leaderOptions.value = [
      {
        id: row.leaderId,
        name: row.leaderName ?? '',
        employeeNo: row.leaderEmployeeNo ?? '',
        mobile: '',
        userEmail: '',
      },
    ];
  }
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    deptName: row.deptName,
    parentId: row.parentId ? findDeptPath(row.parentId, deptTree.value) : [],
    leaderId: row.leaderId || undefined,
    orderNum: row.orderNum ?? 0,
    status: row.status ?? 1,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = (JSON.parse(JSON.stringify(await formApi.getValues())));
  // Cascader parentId 是路径数组，取叶节点 id；空数组=顶级
  if (Array.isArray(values.parentId)) {
    values.parentId = values.parentId.length
      ? values.parentId[values.parentId.length - 1]
      : '';
  }
  try {
    if (isEdit.value) {
      await editDeptApi({ ...values, deptId: editingId.value });
      message.success($t('sysdept.editSuccess'));
    } else {
      await addDeptApi(values);
      message.success($t('sysdept.addSuccess'));
    }
    modalVisible.value = false;
    // 部门树结构可能变化，刷新父级选项
    loadDeptTree();
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('sysdept.confirmDeleteTitle'),
    content: $t('sysdept.confirmDeleteContent', { name: row.deptName }),
    okText: $t('sysdept.confirm'),
    cancelText: $t('sysdept.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteDeptApi(row.deptId);
      message.success($t('sysdept.deleteSuccess'));
      loadDeptTree();
      gridApi.query();
    },
  });
}

// ===== 状态切换（列表内联）=====
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const old = row.status;
  try {
    row.status = newStatus;
    await editDeptApi({
      deptId: row.deptId,
      parentId: row.parentId ?? '',
      deptName: row.deptName,
      orderNum: row.orderNum ?? 0,
      leaderId: row.leaderId ?? '',
      status: newStatus,
    });
    message.success(newStatus === 1 ? $t('sysdept.enableSuccess') : $t('sysdept.disableSuccess'));
  } catch {
    row.status = old;
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd()">{{ $t('sysdept.add') }}</Button>
      </template>
      <template #leader="{ row }">
        <span v-if="row.leaderId">
          {{ row.leaderName }}（{{ row.leaderEmployeeNo }}）
        </span>
        <span v-else>-</span>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('sysdept.switchEnabled')"
          :un-checked-children="$t('sysdept.switchDisabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openAdd(row)">{{ $t('sysdept.addChild') }}</Button>
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('sysdept.edit') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('sysdept.delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>
  </Page>
</template>
