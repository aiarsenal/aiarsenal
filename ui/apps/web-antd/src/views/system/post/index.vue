<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from "ant-design-vue";

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addPostApi,
  deletePostApi,
  editPostApi,
  getPostListApi,
} from '#/api/system/post';

defineOptions({ name: 'SystemPost' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'postName',
    label: $t('post.postName'),
    componentProps: { placeholder: $t('post.postNamePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'postCode',
    label: $t('post.postCode'),
    componentProps: { placeholder: $t('post.postCodePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('post.status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('post.enabled'), value: 1 },
        { label: $t('post.disabled'), value: 0 },
      ],
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('post.seq'), width: 60 },
    { field: 'postCode', title: $t('post.postCode'), minWidth: 120 },
    { field: 'postName', title: $t('post.postName'), minWidth: 120 },
    { field: 'postSort', title: $t('post.postSort'), width: 90 },
    {
      field: 'status',
      title: $t('post.status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'remark', title: $t('post.remark'), minWidth: 160 },
    { field: 'createdAt', title: $t('post.createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('post.action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getPostListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin: { postList, total }
        return { items: res?.postList ?? [], total: res?.total ?? 0 };
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
  isEdit.value ? $t('post.modalTitleEdit') : $t('post.modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'postName',
    label: $t('post.postName'),
    rules: 'required',
    componentProps: { placeholder: $t('post.postNamePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'postCode',
    label: $t('post.postCode'),
    rules: 'required',
    componentProps: { placeholder: $t('post.postCodePlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'postSort',
    label: $t('post.postSortLabel'),
    rules: z.number({ required_error: $t('post.postSortRequired') }),
    componentProps: { min: 0, style: { width: '100%' } },
  },
  {
    component: 'Switch' as const,
    fieldName: 'status',
    label: $t('post.status'),
    componentProps: {
      checkedValue: 1,
      unCheckedValue: 0,
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'remark',
    label: $t('post.remark'),
    componentProps: { placeholder: $t('post.remarkPlaceholder'), rows: 3 },
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
  formApi.setValues({ status: 1, postSort: 0 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.postId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  // gin PostEditReq 字段与行一致：postId/postCode/postName/postSort/status/remark
  formApi.setValues({
    postName: row.postName,
    postCode: row.postCode,
    postSort: row.postSort,
    status: row.status,
    remark: row.remark ?? '',
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin PostEditReq = postId + PostReq
      await editPostApi({ ...values, postId: editingId.value });
      message.success($t('post.editSuccess'));
    } else {
      await addPostApi(values);
      message.success($t('post.addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（镜像 openEdit 字段），失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editPostApi({
      postName: row.postName,
      postCode: row.postCode,
      postSort: row.postSort,
      remark: row.remark ?? '',
      status: newStatus,
      postId: row.postId,
    });
    message.success($t('post.editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('post.selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.postId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.postId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('post.confirmDeleteTitle'),
    content: $t('post.confirmDeleteContent', { count }),
    okText: $t('post.confirm'),
    cancelText: $t('post.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deletePostApi(ids);
      message.success($t('post.deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('post.add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('post.addBatchDelete') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('post.enabled')"
          :un-checked-children="$t('post.disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('post.edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('post.delete') }}
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
