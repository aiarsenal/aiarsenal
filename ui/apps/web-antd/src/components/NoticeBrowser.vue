<script lang="ts" setup>
import type { Dayjs } from 'dayjs';

import { ref, watch } from 'vue';

import { Button, DatePicker, Drawer, List, ListItem, message, Pagination, Select, Spin, Tag } from 'ant-design-vue';

import { deleteNoticeApi, getNoticeListApi, readNoticeApi } from '#/api/system/notice';

defineOptions({ name: 'NoticeBrowser' });
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ 'update:open': [boolean]; changed: [] }>();

const { RangePicker } = DatePicker;

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(10);

// 筛选
const fIsRead = ref<string | undefined>(undefined);
const fType = ref<string | undefined>(undefined);
const fRange = ref<[Dayjs, Dayjs] | undefined>();

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '未读', value: '0' },
  { label: '已读', value: '1' },
];
const typeOptions = [
  { label: '全部类型', value: '' },
  { label: '信息', value: 'info' },
  { label: '警告', value: 'warning' },
  { label: '成功', value: 'success' },
  { label: '错误', value: 'error' },
];

function showTotal(total: number) {
  return `共 ${total} 条`;
}

function typeTag(t?: string) {
  switch (t) {
    case 'warning': {
      return { color: 'gold', text: '警告' };
    }
    case 'success': {
      return { color: 'magenta', text: '成功' };
    }
    case 'error': {
      return { color: 'red', text: '错误' };
    }
    default: {
      return { color: 'blue', text: '信息' };
    }
  }
}

async function load() {
  loading.value = true;
  try {
    const params: any = { pageNum: page.value, pageSize: pageSize.value };
    if (fIsRead.value) params.isRead = fIsRead.value;
    if (fType.value) params.type = fType.value;
    if (fRange.value && fRange.value.length === 2) {
      params.startTime = fRange.value[0]!.format('YYYY-MM-DD HH:mm:ss');
      params.endTime = fRange.value[1]!.format('YYYY-MM-DD HH:mm:ss');
    }
    const res = await getNoticeListApi(params);
    list.value = res?.noticeList ?? [];
    total.value = res?.total ?? 0;
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

function onSearch() {
  page.value = 1;
  load();
}
function onReset() {
  fIsRead.value = undefined;
  fType.value = undefined;
  fRange.value = undefined;
  page.value = 1;
  load();
}
function onPageChange(p: number) {
  page.value = p;
  load();
}
async function markRead(id: string) {
  try {
    await readNoticeApi(id);
    const it = list.value.find((n) => n.noticeId === id);
    if (it) it.isRead = 1;
    emit('changed');
  } catch {
    /* 忽略 */
  }
}
async function remove(id: string) {
  try {
    await deleteNoticeApi(id);
    list.value = list.value.filter((n) => n.noticeId !== id);
    total.value = Math.max(0, total.value - 1);
    emit('changed');
    message.success('已删除');
  } catch {
    /* 忽略 */
  }
}

// 打开时重置筛选 + 加载
watch(
  () => props.open,
  (v) => {
    if (v) {
      onReset();
    }
  },
);

function close() {
  emit('update:open', false);
}
</script>

<template>
  <Drawer :open="open" title="全部消息" :width="560" :footer="null" @close="close">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <Select
        v-model:value="fIsRead"
        style="width: 120px"
        :options="statusOptions"
        placeholder="状态"
        allow-clear
      />
      <Select
        v-model:value="fType"
        style="width: 120px"
        :options="typeOptions"
        placeholder="类型"
        allow-clear
      />
      <RangePicker v-model:value="fRange" show-time format="YYYY-MM-DD HH:mm:ss" />
      <Button type="primary" @click="onSearch">查询</Button>
      <Button @click="onReset">重置</Button>
    </div>
    <Spin :spinning="loading">
      <List :data-source="list" :locale="{ emptyText: '暂无消息' }">
        <template #renderItem="{ item }">
          <ListItem>
            <div class="w-full">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Tag :color="typeTag(item.type).color">
                    {{ typeTag(item.type).text }}
                  </Tag>
                  <span :class="['font-medium', item.isRead === 1 ? 'text-gray-400' : '']">
                    {{ item.title }}
                  </span>
                  <Tag v-if="item.isRead !== 1" color="blue">未读</Tag>
                </div>
                <span class="text-xs text-gray-400">{{ item.createdAt }}</span>
              </div>
              <div class="mt-1 text-sm text-gray-600">{{ item.content }}</div>
              <div class="mt-2 flex gap-2">
                <Button
                  v-if="item.isRead !== 1"
                  size="small"
                  type="link"
                  @click="markRead(item.noticeId)"
                >
                  标为已读
                </Button>
                <Button size="small" type="link" danger @click="remove(item.noticeId)">
                  删除
                </Button>
              </div>
            </div>
          </ListItem>
        </template>
      </List>
      <Pagination
        class="mt-3 text-right"
        :current="page"
        :page-size="pageSize"
        :total="total"
        :show-total="showTotal"
        size="small"
        @change="onPageChange"
      />
    </Spin>
  </Drawer>
</template>
