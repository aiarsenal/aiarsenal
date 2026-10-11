<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addLLMGroupApi,
  deleteLLMGroupApi,
  editLLMGroupApi,
  getLLMGroupListApi,
} from '#/api/system/llm/group';

defineOptions({ name: 'SystemLlmGroup' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('llm.group_name'),
    componentProps: { placeholder: $t('llm.group_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('llm.group_seq'), width: 60 },
    { field: 'groupName', title: $t('llm.group_name'), minWidth: 160 },
    { field: 'description', title: $t('llm.group_description'), minWidth: 220 },
    {
      field: 'status',
      title: $t('llm.group_status'),
      width: 100,
      slots: { default: 'status' },
    },
    {
      field: 'action',
      title: $t('llm.group_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMGroupListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin LLMModelGroupSearchRes → { list, total }
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
  isEdit.value ? $t('llm.group_modalTitleEdit') : $t('llm.group_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'groupName',
    label: $t('llm.group_name'),
    help: $t('llm.group_nameHelp'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.group_namePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('llm.group_description'),
    help: $t('llm.group_descHelp'),
    componentProps: {
      placeholder: $t('llm.group_descPlaceholder'),
      rows: 3,
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'status',
    label: $t('llm.group_status'),
    componentProps: {
      options: [
        { label: $t('llm.group_enabled'), value: 1 },
        { label: $t('llm.group_disabled'), value: 0 },
      ],
    },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
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
  await formApi.setValues({ status: 1 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.groupId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    groupName: row.groupName,
    description: row.description ?? '',
    status: row.status,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const payload: any = {
    groupName: values.groupName,
    description: values.description ?? '',
    status: values.status,
  };
  try {
    if (isEdit.value) {
      await editLLMGroupApi({ ...payload, groupId: editingId.value });
      message.success($t('llm.group_editSuccess'));
    } else {
      await addLLMGroupApi(payload);
      message.success($t('llm.group_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（groupName/description 原样回填），失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editLLMGroupApi({
      groupId: row.groupId,
      groupName: row.groupName,
      description: row.description ?? '',
      status: newStatus,
    });
    message.success($t('llm.group_editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('llm.group_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.groupId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.groupId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('llm.group_confirmTitle'),
    content: $t('llm.group_confirmDeleteContent', { count }),
    okText: $t('llm.group_confirm'),
    cancelText: $t('llm.group_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteLLMGroupApi(ids);
      message.success($t('llm.group_deleteSuccess'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('llm.group_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('llm.group_batchDelete') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('llm.group_enabled')"
          :un-checked-children="$t('llm.group_disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('llm.group_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('llm.group_delete') }}
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
  </Page>
</template>
