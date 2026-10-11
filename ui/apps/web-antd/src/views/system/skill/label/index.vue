<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Input, message, Modal, Switch, Tag } from "ant-design-vue";

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addSkillLabelApi,
  deleteSkillLabelApi,
  editSkillLabelApi,
  getSkillLabelListApi,
  getSkillLabelTranslationsApi,
  upsertSkillLabelTranslationApi,
} from '#/api/system/skill/label';

defineOptions({ name: 'SystemSkillLabel' });

// 类型常量（dto.SkillLabelReq.Type: RECOMMENDED / PRIVILEGED）（computed：随语言切换）
const TYPE_OPTIONS = computed(() => [
  { label: $t('skill.label_typeRecommended'), value: 'RECOMMENDED' },
  { label: $t('skill.label_typePrivileged'), value: 'PRIVILEGED' },
]);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'slug',
    label: $t('skill.label_slug'),
    componentProps: { placeholder: $t('skill.label_slugPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'type',
    label: $t('skill.label_type'),
    componentProps: {
      allowClear: true,
      options: TYPE_OPTIONS.value,
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('skill.label_seq'), width: 60, fixed: 'left' },
    { field: 'slug', title: $t('skill.label_slug'), minWidth: 160 },
    {
      field: 'type',
      title: $t('skill.label_type'),
      width: 110,
      slots: { default: 'type' },
    },
    {
      field: 'visibleInFilter',
      title: $t('skill.label_visibleInFilter'),
      width: 110,
      slots: { default: 'visible' },
    },
    { field: 'sortOrder', title: $t('skill.label_sortOrder'), width: 90 },
    {
      field: 'creatorName',
      title: $t('skill.label_createdBy'),
      minWidth: 120,
      slots: { default: 'creator' },
    },
    { field: 'createdAt', title: $t('skill.label_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('skill.label_action'),
      width: 240,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        // dto.SkillLabelSearchRes: { list, total, currentPage }
        const res = await getSkillLabelListApi({
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
  isEdit.value ? $t('skill.label_modalTitleEdit') : $t('skill.label_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'slug',
    label: $t('skill.label_slug'),
    rules: 'required',
    componentProps: { placeholder: $t('skill.label_slugUniquePlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'type',
    label: $t('skill.label_type'),
    rules: 'required',
    componentProps: {
      options: TYPE_OPTIONS.value,
      placeholder: $t('skill.label_typePlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Switch' as const,
    fieldName: 'visibleInFilter',
    label: $t('skill.label_visibleInFilter'),
    componentProps: {
      checkedChildren: $t('skill.label_visibleShown'),
      unCheckedChildren: $t('skill.label_visibleHidden'),
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'sortOrder',
    label: $t('skill.label_sortOrder'),
    componentProps: { min: 0, style: { width: '100%' } },
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
  formApi.setValues({
    type: 'RECOMMENDED',
    visibleInFilter: 1,
    sortOrder: 0,
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.labelId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    slug: row.slug,
    type: row.type,
    visibleInFilter: row.visibleInFilter,
    sortOrder: row.sortOrder,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // antd Switch 返回 boolean，gin DTO 期望 uint (0/1)
  if (typeof values.visibleInFilter === 'boolean') {
    values.visibleInFilter = values.visibleInFilter ? 1 : 0;
  }
  try {
    if (isEdit.value) {
      // dto.SkillLabelEditReq: { labelId, ...SkillLabelReq }
      await editSkillLabelApi({ ...values, labelId: editingId.value });
      message.success($t('skill.label_editSuccess'));
    } else {
      await addSkillLabelApi(values);
      message.success($t('skill.label_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除（body: { id }，id = labelId 字符串） =====
async function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('skill.label_tipTitle'),
    content: $t('skill.label_confirmDeleteContent', { slug: row.slug }),
    okText: $t('skill.label_confirm'),
    cancelText: $t('skill.label_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteSkillLabelApi(row.labelId);
      message.success($t('skill.label_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 翻译弹窗 =====
const transVisible = ref(false);
const transLabelId = ref('');
const transList = ref<any[]>([]);
const transAdd = reactive({
  labelId: '',
  locale: 'zh-cn',
  displayName: '',
});

async function openTranslations(row: any) {
  transLabelId.value = row.labelId;
  transVisible.value = true;
  transAdd.labelId = row.labelId;
  transAdd.locale = 'zh-cn';
  transAdd.displayName = '';
  await loadTranslations(row.labelId);
}

async function loadTranslations(labelId: string) {
  try {
    const res = await getSkillLabelTranslationsApi(labelId);
    transList.value = res?.list ?? [];
  } catch {
    transList.value = [];
  }
}

async function onUpsertTrans() {
  if (!transAdd.displayName) {
    message.warning($t('skill.label_displayNameRequired'));
    return;
  }
  if (!transAdd.locale) {
    message.warning($t('skill.label_localeRequired'));
    return;
  }
  try {
    await upsertSkillLabelTranslationApi({
      labelId: transAdd.labelId,
      locale: transAdd.locale,
      displayName: transAdd.displayName,
    });
    message.success($t('skill.label_saveSuccess'));
    transAdd.displayName = '';
    await loadTranslations(transAdd.labelId);
  } catch {
    /* 拦截器已提示 */
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('skill.label_add') }}</Button>
      </template>
      <template #type="{ row }">
        <span v-if="row.type === 'RECOMMENDED'">{{ $t('skill.label_typeRecommended') }}</span>
        <span v-else>{{ $t('skill.label_typePrivileged') }}</span>
      </template>
      <template #visible="{ row }">
        <Switch
          :checked="row.visibleInFilter === 1"
          :checked-children="$t('skill.label_visibleShownShort')"
          :un-checked-children="$t('skill.label_visibleHiddenShort')"
          disabled
        />
      </template>
      <template #creator="{ row }">
        <span v-if="row.creatorName">{{ row.creatorName }}</span>
        <Tag v-else color="default">{{ $t('skill.label_builtin') }}</Tag>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('skill.label_edit') }}</Button>
        <Button type="primary" ghost size="small" @click="openTranslations(row)">
          {{ $t('skill.label_translation') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('skill.label_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="520"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <Modal
      v-model:open="transVisible"
      :title="$t('skill.label_translationTitle')"
      :width="640"
      :footer="null"
      :destroy-on-close="true"
    >
      <div class="mb-3 flex gap-2">
        <Input
          v-model:value="transAdd.locale"
          :placeholder="$t('skill.label_localePlaceholder')"
          style="width: 25%"
        />
        <Input
          v-model:value="transAdd.displayName"
          :placeholder="$t('skill.label_displayNamePlaceholder')"
          style="width: 50%"
          @press-enter="onUpsertTrans"
        />
        <Button type="primary" @click="onUpsertTrans">{{ $t('skill.label_save') }}</Button>
      </div>
      <div v-if="transList.length" class="flex flex-col gap-1">
        <div
          v-for="item in transList"
          :key="item.transId || item.locale"
          class="flex justify-between border-b border-gray-100 py-1 text-sm"
        >
          <span>{{ item.locale }}</span>
          <span>{{ item.displayName }}</span>
        </div>
      </div>
      <div v-else class="py-4 text-center text-gray-400">{{ $t('skill.label_translationEmpty') }}</div>
    </Modal>
  </Page>
</template>
