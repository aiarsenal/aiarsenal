<script lang="ts" setup>
// 我的提交：当前用户提交的全部审核工单（统一审核中心，全类型）。
// 类型专属内容在详情弹窗按 orderType 渲染（现只有 llm_temp_budget：临时预算申请）。
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  Modal,
  RadioGroup,
  Table,
  Tag,
  message,
} from 'ant-design-vue';

import { $t } from '#/locales';
import { getMyReviewOrdersApi } from '#/api/system/review/order';
import type { ReviewOrderApi } from '#/api/system/review/order';

defineOptions({ name: 'ReviewMine' });

// ===== 列表 =====
const loading = ref(false);
const status = ref('');
const list = ref<ReviewOrderApi.Item[]>([]);
const pageNum = ref(1);
const pageSize = 20;
const total = ref(0);

const columns = [
  { dataIndex: 'orderType', title: () => $t('review.col_type'), width: 100 },
  { dataIndex: 'title', title: () => $t('review.col_title'), ellipsis: true },
  { dataIndex: 'amount', title: () => $t('review.col_amount'), width: 110 },
  { dataIndex: 'status', title: () => $t('review.col_status'), width: 100 },
  { dataIndex: 'createdAt', title: () => $t('review.col_createdAt'), width: 170 },
  { dataIndex: 'reviewerName', title: () => $t('review.col_reviewer'), width: 110 },
  { key: 'action', title: () => $t('review.col_action'), width: 90, fixed: 'right' as const },
];

async function load(pn?: number) {
  if (pn) pageNum.value = pn;
  loading.value = true;
  try {
    const res = await getMyReviewOrdersApi({
      pageNum: pageNum.value,
      pageSize,
      status: status.value,
    });
    list.value = res?.list ?? [];
    total.value = res?.total ?? 0;
  } catch (e: any) {
    message.error(e?.message || $t('review.loadFailed'));
    list.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(() => load());

// ===== 展示 =====
function fmtMoney(n?: number) {
  return Number.isFinite(n) ? `$${(n as number).toFixed(2)}` : '-';
}

function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

function typeText(t: string) {
  if (t === 'llm_temp_budget') return $t('review.type_llm_temp_budget');
  return t;
}

function statusColor(s: string) {
  switch (s) {
    case 'approved': {
      return 'success';
    }
    case 'rejected': {
      return 'error';
    }
    default: {
      return 'processing';
    }
  }
}

function statusText(s: string) {
  switch (s) {
    case 'approved': {
      return $t('review.status_approved');
    }
    case 'rejected': {
      return $t('review.status_rejected');
    }
    default: {
      return $t('review.status_pending');
    }
  }
}

function fmtDuration(d?: string) {
  switch (d) {
    case '1d': {
      return $t('review.duration_day');
    }
    case '7d': {
      return $t('review.duration_week');
    }
    default: {
      return $t('review.duration_month');
    }
  }
}

// ===== 详情 Modal =====
const detailOpen = ref(false);
const current = ref<ReviewOrderApi.Item | null>(null);

// bodyCell 插槽的 record 是 Record<string, any>，参数用宽松形状接收
function openDetail(r: any) {
  current.value = r as ReviewOrderApi.Item;
  detailOpen.value = true;
}
</script>

<template>
  <Page>
    <Card>
      <template #title>{{ $t('review.mine_title') }}</template>
      <template #extra>
        <RadioGroup
          v-model:value="status"
          size="small"
          option-type="button"
          :options="[
            { label: $t('review.status_all'), value: '' },
            { label: $t('review.status_pending'), value: 'pending' },
            { label: $t('review.status_approved'), value: 'approved' },
            { label: $t('review.status_rejected'), value: 'rejected' },
          ]"
          @change="load(1)"
        />
        <Button :loading="loading" size="small" class="ml-2" @click="load()">
          {{ $t('review.refresh') }}
        </Button>
      </template>

      <Table
        :columns="columns"
        :data-source="list"
        :loading="loading"
        :pagination="{
          current: pageNum,
          pageSize,
          total,
          showSizeChanger: false,
          onChange: (pn: number) => load(pn),
        }"
        row-key="orderId"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'orderType'">
            <Tag color="blue">{{ typeText(record.orderType) }}</Tag>
          </template>
          <template v-else-if="column.dataIndex === 'amount'">
            {{ fmtMoney(record.amount) }}
          </template>
          <template v-else-if="column.dataIndex === 'status'">
            <Tag :color="statusColor(record.status)">
              {{ statusText(record.status) }}
            </Tag>
          </template>
          <template v-else-if="column.dataIndex === 'createdAt'">
            {{ fmtTime(record.createdAt) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <Button type="link" size="small" @click="openDetail(record)">
              {{ $t('review.detail') }}
            </Button>
          </template>
        </template>
        <template #emptyText>
          <Empty :description="$t('review.empty')" class="py-8" />
        </template>
      </Table>
    </Card>

    <!-- 详情：通用字段 + 类型专属块（llm_temp_budget） + 审批信息 -->
    <Modal v-model:open="detailOpen" :title="$t('review.detail_title')" :footer="null">
      <Descriptions v-if="current" :column="1" size="small" class="py-2" bordered>
        <DescriptionsItem :label="$t('review.col_type')">
          {{ typeText(current.orderType) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('review.col_title')">
          {{ current.title }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('review.col_amount')">
          {{ fmtMoney(current.amount) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('review.col_createdAt')">
          {{ fmtTime(current.createdAt) }}
        </DescriptionsItem>

        <!-- llm 临时预算专属：基于预算 / 周期 / 理由 -->
        <template v-if="current.orderType === 'llm_temp_budget'">
          <DescriptionsItem :label="$t('review.detail_base')">
            {{ current.payload?.baseBudgetName || '-' }}
          </DescriptionsItem>
          <DescriptionsItem :label="$t('review.detail_duration')">
            {{ fmtDuration(current.payload?.duration) }}
          </DescriptionsItem>
          <DescriptionsItem :label="$t('review.detail_reason')">
            {{ current.payload?.reason || '-' }}
          </DescriptionsItem>
        </template>

        <DescriptionsItem :label="$t('review.col_status')">
          <Tag :color="statusColor(current.status)">
            {{ statusText(current.status) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem
          v-if="current.status !== 'pending'"
          :label="$t('review.col_reviewer')"
        >
          {{ current.reviewerName || '-' }} · {{ fmtTime(current.reviewedAt) }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="current.status !== 'pending' && current.reviewNote"
          :label="$t('review.col_reviewNote')"
        >
          {{ current.reviewNote }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="current.status === 'approved' && current.resultRef"
          :label="$t('review.detail_resultRef')"
        >
          {{ current.resultRef }}
        </DescriptionsItem>
      </Descriptions>
    </Modal>
  </Page>
</template>
