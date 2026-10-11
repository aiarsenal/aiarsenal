<script lang="ts" setup>
// 我的审核：待我审批的工单（统一审核中心；超管=全部，否则=本部门）。
// 通过/驳回共用意见弹窗；llm_temp_budget 通过即生成申请人专属临时预算并双通道通知。
import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  RadioGroup,
  Table,
  Tag,
  message,
} from 'ant-design-vue';

import { $t } from '#/locales';
import {
  approveReviewOrderApi,
  getReviewTodoApi,
  rejectReviewOrderApi,
} from '#/api/system/review/order';
import type { ReviewOrderApi } from '#/api/system/review/order';

defineOptions({ name: 'ReviewTodo' });

const Textarea = Input.TextArea;

// ===== 列表 =====
const loading = ref(false);
const status = ref('pending');
const list = ref<ReviewOrderApi.Item[]>([]);
const pageNum = ref(1);
const pageSize = 20;
const total = ref(0);

const columns = [
  { dataIndex: 'orderType', title: () => $t('review.col_type'), width: 100 },
  { dataIndex: 'title', title: () => $t('review.col_title'), ellipsis: true },
  { dataIndex: 'applicantName', title: () => $t('review.col_applicant'), width: 110 },
  { dataIndex: 'amount', title: () => $t('review.col_amount'), width: 110 },
  { dataIndex: 'reason', title: () => $t('review.col_reason'), ellipsis: true },
  { dataIndex: 'createdAt', title: () => $t('review.col_createdAt'), width: 170 },
  { key: 'action', title: () => $t('review.col_action'), width: 130, fixed: 'right' as const },
];

async function load(pn?: number) {
  if (pn) pageNum.value = pn;
  loading.value = true;
  try {
    const res = await getReviewTodoApi({
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

// ===== 审批 Modal（通过/驳回共用） =====
const modalOpen = ref(false);
const saving = ref(false);
const mode = ref<'approve' | 'reject'>('approve');
const current = ref<ReviewOrderApi.Item | null>(null);
const note = reactive({ value: '' });

// bodyCell 插槽的 record 是 Record<string, any>，参数用宽松形状接收
function openReview(m: 'approve' | 'reject', r: any) {
  mode.value = m;
  current.value = r as ReviewOrderApi.Item;
  note.value = '';
  modalOpen.value = true;
}

async function submitReview() {
  if (!current.value) return;
  saving.value = true;
  try {
    const payload = { note: note.value.trim(), orderId: current.value.orderId };
    if (mode.value === 'approve') {
      await approveReviewOrderApi(payload);
      message.success($t('review.approveSuccess'));
    } else {
      await rejectReviewOrderApi(payload);
      message.success($t('review.rejectSuccess'));
    }
    modalOpen.value = false;
    load();
  } catch (e: any) {
    // 「该申请已被处理」（并发双批）等业务错误原样透出
    message.error(e?.message || $t('review.loadFailed'));
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Page>
    <Card>
      <template #title>{{ $t('review.todo_title') }}</template>
      <template #extra>
        <RadioGroup
          v-model:value="status"
          size="small"
          option-type="button"
          :options="[
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
          <template v-else-if="column.dataIndex === 'reason'">
            <span :title="record.payload?.reason">
              {{ record.payload?.reason || '-' }}
            </span>
          </template>
          <template v-else-if="column.dataIndex === 'createdAt'">
            {{ fmtTime(record.createdAt) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <template v-if="record.status === 'pending'">
              <Button type="link" size="small" @click="openReview('approve', record)">
                {{ $t('review.approve') }}
              </Button>
              <Button type="link" size="small" danger @click="openReview('reject', record)">
                {{ $t('review.reject') }}
              </Button>
            </template>
            <template v-else>
              <span class="opacity-60">
                {{ statusText(record.status) }} · {{ record.reviewerName || '-' }} ·
                {{ fmtTime(record.reviewedAt) }}
              </span>
            </template>
          </template>
        </template>
        <template #emptyText>
          <Empty :description="$t('review.empty')" class="py-8" />
        </template>
      </Table>
    </Card>

    <!-- 审批意见 -->
    <Modal
      v-model:open="modalOpen"
      :title="
        mode === 'approve'
          ? $t('review.approveTitle')
          : $t('review.rejectTitle')
      "
      :confirm-loading="saving"
      :ok-text="mode === 'approve' ? $t('review.approve') : $t('review.reject')"
      :ok-button-props="{ danger: mode === 'reject' }"
      @ok="submitReview"
    >
      <div class="flex flex-col gap-3 py-2">
        <div class="text-sm opacity-80">
          {{ current?.applicantName }} · {{ typeText(current?.orderType || '') }} ·
          {{ fmtMoney(current?.amount) }}
        </div>
        <div class="text-sm">{{ current?.title }}</div>
        <div v-if="current?.payload?.reason" class="text-sm opacity-80">
          {{ current.payload.reason }}
        </div>
        <div>
          <div class="mb-1">{{ $t('review.noteLabel') }}</div>
          <Textarea
            v-model:value="note.value"
            :rows="3"
            :maxlength="500"
            show-count
            :placeholder="$t('review.notePlaceholder')"
          />
        </div>
      </div>
    </Modal>
  </Page>
</template>
