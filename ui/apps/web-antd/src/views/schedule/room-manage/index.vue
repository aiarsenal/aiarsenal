<script lang="ts" setup>
/**
 * 会议室管理：增删改查（vxe-table + useVbenForm 弹窗，照项目管理模式）。
 * 删除软删：既有事件的 room_id 悬空引用由展示层回退 room_id，不级联。
 */
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch, Tag } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  addRoomApi,
  deleteRoomApi,
  editRoomApi,
  getRoomListApi,
} from '#/api/system/schedule';
import { $t } from '#/locales';

defineOptions({ name: 'ScheduleRoomManage' });

const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('schedule.room_name'),
    componentProps: {
      placeholder: $t('schedule.room_name_placeholder'),
    },
  },
]);

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: '#', width: 55 },
    { field: 'name', title: $t('schedule.room_name'), minWidth: 140 },
    { field: 'location', title: $t('schedule.location'), minWidth: 140 },
    { field: 'capacity', title: $t('schedule.capacity'), width: 90 },
    { field: 'equipment', title: $t('schedule.equipment'), minWidth: 180, slots: { default: 'equipment' } },
    { field: 'enabled', title: $t('schedule.enabled'), width: 90, slots: { default: 'enabled' } },
    { field: 'remark', title: $t('schedule.remark'), minWidth: 160 },
    { field: 'action', title: $t('schedule.action'), width: 150, fixed: 'right', slots: { default: 'action' } },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getRoomListApi({
          pageNum: page.currentPage,
          pageSize: 20,
          ...(form as any),
        });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
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
  isEdit.value ? $t('schedule.room_edit_title') : $t('schedule.room_add_title'),
);
const enabledSwitch = ref(true);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('schedule.room_name'),
    rules: 'required',
    componentProps: {
      placeholder: $t('schedule.room_name_placeholder'),
    },
  },
  {
    component: 'Input',
    fieldName: 'location',
    label: $t('schedule.location'),
    componentProps: {
      placeholder: $t('schedule.location_placeholder'),
    },
  },
  { component: 'InputNumber', fieldName: 'capacity', label: $t('schedule.capacity'), componentProps: { min: 1, style: { width: '100%' } } },
  {
    component: 'Input',
    fieldName: 'equipment',
    label: $t('schedule.equipment'),
    componentProps: {
      placeholder: $t('schedule.equipment_placeholder'),
    },
  },
  { component: 'Input', fieldName: 'remark', label: $t('schedule.remark'), componentProps: { placeholder: $t('schedule.remark_placeholder') } },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  enabledSwitch.value = true;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({ capacity: 6 });
}
async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.roomId;
  enabledSwitch.value = row.enabled === 1;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    name: row.name,
    location: row.location ?? '',
    capacity: row.capacity ?? 0,
    equipment: equipText(row.equipment),
    remark: row.remark ?? '',
  });
}

/** equipment jsonb 文本 ↔ 顿号串 */
function equipText(raw: string): string {
  try {
    const arr = JSON.parse(raw || '[]');
    return Array.isArray(arr) ? arr.join('、') : '';
  } catch {
    return '';
  }
}
function equipJson(text: string): string {
  const arr = text
    .split(/[、,，]/)
    .map((s) => s.trim())
    .filter(Boolean);
  return JSON.stringify(arr);
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  const payload = {
    ...values,
    capacity: Number(values.capacity ?? 0),
    equipment: equipJson(values.equipment ?? ''),
    enabled: enabledSwitch.value ? 1 : 0,
  };
  try {
    if (isEdit.value) {
      await editRoomApi({ ...payload, roomId: editingId.value });
      message.success($t('schedule.room_edit_success'));
    } else {
      await addRoomApi(payload);
      message.success($t('schedule.room_add_success'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('schedule.select_to_delete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.roomId), rows.length);
}
function onDeleteRow(row: any) {
  void doDelete([row.roomId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('schedule.room_confirm_delete_title'),
    content: $t('schedule.room_confirm_delete_content', { count }),
    okText: $t('schedule.confirm'),
    cancelText: $t('schedule.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteRoomApi(ids);
      message.success($t('schedule.delete_success'));
      gridApi.query();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('schedule.room_add') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('schedule.batch_delete') }}
        </Button>
      </template>
      <template #equipment="{ row }">
        <Tag
          v-for="e in (typeof row.equipment === 'string' ? JSON.parse(row.equipment || '[]') : row.equipment)"
          :key="e"
          class="equip-tag"
        >
          {{ e }}
        </Tag>
        <span v-if="equipText(row.equipment) === ''">-</span>
      </template>
      <template #enabled="{ row }">
        <Tag :color="row.enabled === 1 ? 'success' : 'default'">
          {{ row.enabled === 1 ? $t('schedule.enabled_on') : $t('schedule.enabled_off') }}
        </Tag>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('schedule.edit') }}
        </Button>
        <Button
          type="primary"
          ghost
          size="small"
          danger
          class="ml-1"
          @click="onDeleteRow(row)"
        >
          {{ $t('schedule.delete') }}
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
      <div class="enabled-row">
        <span>{{ $t('schedule.enabled') }}</span>
        <Switch v-model:checked="enabledSwitch" />
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.enabled-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
  padding: 0 4px;
}
.equip-tag {
  font-size: 11px;
  line-height: 17px;
  padding: 0 5px;
}
</style>
