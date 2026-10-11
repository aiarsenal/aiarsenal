<script lang="ts" setup>
import type { EchartsUIType } from '@vben/plugins/echarts';

import { computed, onMounted, reactive, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Progress,
  RadioGroup,
  Select,
  Spin,
  Table,
  Tag,
  Tooltip,
  message,
} from 'ant-design-vue';

import { CircleHelp } from '@vben/icons';
import { EchartsUI, useEcharts } from '@vben/plugins/echarts';

import { $t } from '#/locales';
import { getMyBudgetsApi, getMySpendHistoryApi } from '#/api/system/llm/budget';
import type { LLMBudgetApi } from '#/api/system/llm/budget';
import { createLlmTempBudgetApi } from '#/api/system/review/order';

defineOptions({ name: 'SystemLlmMyBudget' });

const Textarea = Input.TextArea;

// ===== 我的预算 =====
const loading = ref(false);
const list = ref<LLMBudgetApi.MyBudgetItem[]>([]);
const usage = ref<LLMBudgetApi.MyUsageSummary | undefined>();
/** 命中预算 id（生效上限最大的一条，后端判定）：唯一展示已用/剩余/进度与提醒的对象 */
const hitId = ref('');

async function load() {
  loading.value = true;
  try {
    const res = await getMyBudgetsApi();
    list.value = res?.list ?? [];
    usage.value = res?.usage;
    hitId.value = res?.hitBudgetId ?? '';
    syncHistoryDefault();
  } catch (e: any) {
    message.error(e?.message || $t('llm.mine_loadFailed'));
  } finally {
    loading.value = false;
  }
}

onMounted(load);

// ===== 历史周期消费 =====
const historyPeriod = ref<'day' | 'month' | 'week'>('month');
/** 用户手动切过周期后不再随预算数据重置默认 */
let historyTouched = false;
const historyLoading = ref(false);
const historyPoints = ref<LLMBudgetApi.SpendHistoryPoint[]>([]);
const chartRef = ref<EchartsUIType>();
const { renderEcharts } = useEcharts(chartRef);

/** 默认粒度跟首个命中预算的周期走（1d→日 / 7d→周 / 其余→月）；无预算保持月 */
function syncHistoryDefault() {
  if (historyTouched) return;
  const d = sortedList.value[0]?.budgetDuration;
  historyPeriod.value =
    d === '1d' ? 'day' : d === '7d' ? 'week' : ('month' as 'day' | 'week');
}

async function loadHistory() {
  historyLoading.value = true;
  try {
    const res = await getMySpendHistoryApi(historyPeriod.value);
    historyPoints.value = res?.points ?? [];
  } catch {
    historyPoints.value = [];
  } finally {
    historyLoading.value = false;
  }
}

function onHistoryChange(e: any) {
  historyTouched = true;
  historyPeriod.value = e.target.value;
  loadHistory();
}

function renderHistory() {
  renderEcharts({
    grid: { bottom: 0, containLabel: true, left: '1%', right: '2%', top: '5%' },
    series: [
      {
        areaStyle: { opacity: 0.15 },
        data: historyPoints.value.map((p) => Number(p.spend.toFixed(4))),
        smooth: true,
        type: 'line',
      },
    ],
    tooltip: {
      axisPointer: { lineStyle: { width: 1 }, type: 'line' },
      trigger: 'axis',
      valueFormatter: (v: number) => `$${v.toFixed(2)}`,
    },
    xAxis: {
      axisTick: { show: false },
      boundaryGap: false,
      data: historyPoints.value.map((p) => p.period),
      type: 'category',
    },
    yAxis: [
      {
        axisTick: { show: false },
        splitNumber: 4,
        type: 'value',
      },
    ],
  });
}

watch(historyPoints, renderHistory);
onMounted(loadHistory);

// ===== 申请临时预算 Modal（提交走统一审核中心，进度在 审核系统·我的提交 查看） =====
const reqModalOpen = ref(false);
const reqSaving = ref(false);
const reqForm = reactive({
  amount: undefined as number | undefined,
  baseBudgetId: '',
  baseBudgetName: '',
  duration: '30d',
  reason: '',
});

/** 个人功能，与具体某条预算无关：基础预算自动取命中预算（后端 hitBudgetId 判定） */
function openReqModal() {
  const hit = list.value.find((b) => b.budgetId === hitId.value);
  reqForm.baseBudgetId = hit?.budgetId ?? '';
  reqForm.baseBudgetName = hit?.budgetName ?? '';
  reqForm.amount = undefined;
  reqForm.reason = '';
  reqForm.duration = '30d';
  reqModalOpen.value = true;
}

const durationOptions = computed(() => [
  { label: $t('llm.budget_durationDay'), value: '1d' },
  { label: $t('llm.budget_durationWeek'), value: '7d' },
  { label: $t('llm.budget_durationMonth'), value: '30d' },
]);

async function submitReq() {
  const amount = Number(reqForm.amount);
  if (!Number.isFinite(amount) || amount <= 0) {
    message.warning($t('llm.req_amountInvalid'));
    return;
  }
  const reason = reqForm.reason.trim();
  if (!reason) {
    message.warning($t('llm.req_reasonRequired'));
    return;
  }
  reqSaving.value = true;
  try {
    const res = await createLlmTempBudgetApi({
      amount,
      // 后端可自动取命中最大预算；这里带上弹窗里展示的那条，保证所见即所录
      baseBudgetId: reqForm.baseBudgetId || undefined,
      duration: reqForm.duration,
      reason,
    });
    // 最高职级申请人免审：提交即通过、临时预算当场生成
    message.success(
      res?.status === 'approved'
        ? $t('llm.req_autoApproved')
        : $t('llm.req_submitSuccess'),
    );
    reqModalOpen.value = false;
  } catch (e: any) {
    message.error(e?.message || $t('llm.mine_loadFailed'));
  } finally {
    reqSaving.value = false;
  }
}

// ===== 派生 =====
/** 命中行（后端 hitBudgetId 判定，与临时预算基础预算同口径） */
const hitItem = computed(() =>
  list.value.find((b) => b.budgetId === hitId.value),
);

/** 申请临时预算门槛：只有被命中的预算、且本周期本人有消耗，才允许点击（后端同口径拦截） */
const canRequestTemp = computed(
  () => !!hitItem.value && (hitItem.value.spend ?? 0) > 0,
);

/** 命中预算本人已用 ≥90% → 整页顶部警示（只看命中那条） */
const hasAlert = computed(
  () => !!hitItem.value && (hitItem.value.isLow || hitItem.value.isExhausted),
);

/** 命中预算剩余（个人口径；不限额=∞，无命中=0） */
const totalRemaining = computed(() => {
  const h = hitItem.value;
  if (!h) return 0;
  return h.maxBudget == null ? Number.POSITIVE_INFINITY : (h.remaining ?? 0);
});

/** 列表按生效上限降序（命中行自然在最上），再按名称稳定 */
const sortedList = computed(() =>
  [...list.value].sort((a, b) => {
    const capA = a.maxBudget ?? Number.POSITIVE_INFINITY;
    const capB = b.maxBudget ?? Number.POSITIVE_INFINITY;
    if (capA !== capB) return capB - capA;
    return (a.budgetName ?? '').localeCompare(b.budgetName ?? '');
  }),
);

const budgetColumns = [
  { dataIndex: 'budgetName', title: () => $t('llm.budget_name') },
  { dataIndex: 'budgetDuration', title: () => $t('llm.budget_duration'), width: 70 },
  { dataIndex: 'maxBudget', title: () => $t('llm.budget_maxBudget'), width: 100 },
  { dataIndex: 'softBudget', title: () => $t('llm.budget_softBudget'), width: 100 },
  { dataIndex: 'spend', title: () => $t('llm.mine_used'), width: 100 },
  { dataIndex: 'remaining', title: () => $t('llm.mine_remaining'), width: 100 },
  { key: 'progress', title: () => $t('llm.mine_progress'), width: 170 },
  { dataIndex: 'budgetResetAt', title: () => $t('llm.budget_resetAt'), width: 170 },
  { dataIndex: 'status', title: () => $t('llm.key_status'), width: 100 },
];

// ===== 展示 =====
function fmtMoney(n?: number) {
  return Number.isFinite(n) ? `$${(n as number).toFixed(2)}` : '-';
}

function fmtInt(n?: number) {
  return Number.isFinite(n) ? (n as number).toLocaleString() : '-';
}

/** 周期文案：无/日/周/月（与预算管理页同映射） */
function fmtDuration(d?: string) {
  switch (d) {
    case '1d': {
      return $t('llm.budget_durationDay');
    }
    case '7d': {
      return $t('llm.budget_durationWeek');
    }
    case '1mo':
    case '30d': {
      return $t('llm.budget_durationMonth');
    }
    default: {
      return $t('llm.budget_durationNone');
    }
  }
}

/** headerCell 插槽拿到的是原始 column.title（本项目为随语言切换定义为 () => $t(...) 函数），
 *  插槽接管渲染后须手动调用，否则插值输出函数源码 */
function headerTitle(title: unknown) {
  return typeof title === 'function' ? title() : title;
}

function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

/**
 * 进度条分档配色（色源 /client/device 运行详情模块色，明暗主题通用中间色调）：
 * <60% emerald → ≥60% amber → ≥85% orange → ≥95%/用尽 red。
 * 参数用宽松形状（bodyCell 插槽 record 是 Record<string, any>）。
 */
function progressColor(b: {
  isExhausted?: boolean;
  isLow?: boolean;
  usedPercent?: number;
}) {
  const pct = b.usedPercent ?? 0;
  if (b.isExhausted || pct >= 95) return '#ef4444';
  if (b.isLow || pct >= 85) return '#f97316';
  if (pct >= 60) return '#f59e0b';
  return '#10b981';
}
</script>

<template>
  <Page>
    <Card>
      <template #title>{{ $t('llm.mine_title') }}</template>
      <template #extra>
        <div class="flex gap-2">
          <!-- 个人功能，不绑定某条预算；基础预算由后端自动取命中最大者。
               门槛：命中预算本周期有本人消耗才可申请（canRequestTemp）。 -->
          <Button
            type="primary"
            :disabled="!canRequestTemp"
            @click="openReqModal()"
          >
            {{ $t('llm.req_apply') }}
          </Button>
          <Button :loading="loading" @click="load">
            {{ $t('llm.mine_refresh') }}
          </Button>
        </div>
      </template>

      <Spin :spinning="loading">
        <!-- 低额警示 -->
        <Alert
          v-if="hasAlert"
          type="error"
          show-icon
          class="mb-4"
          :message="$t('llm.mine_alertTitle')"
          :description="$t('llm.mine_alertDesc')"
        />

        <!-- 本月消费汇总条 -->
        <div
          class="mb-4 flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-border pb-4 text-sm"
        >
          <div>
            <span class="opacity-70">{{ $t('llm.mine_usageSpend') }}</span>
            <b class="ml-2 text-base">{{ fmtMoney(usage?.spend) }}</b>
          </div>
          <div>
            <span class="opacity-70">{{ $t('llm.mine_usagePrompt') }}</span>
            <b class="ml-2">{{ fmtInt(usage?.promptTokens) }}</b>
          </div>
          <div>
            <span class="opacity-70">{{ $t('llm.mine_usageCompletion') }}</span>
            <b class="ml-2">{{ fmtInt(usage?.completionTokens) }}</b>
          </div>
          <div>
            <span class="opacity-70">{{ $t('llm.mine_usageRequests') }}</span>
            <b class="ml-2">{{ fmtInt(usage?.requests) }}</b>
          </div>
          <div>
            <span class="opacity-70">{{ $t('llm.mine_summaryCount', { count: list.length }) }}</span>
            <template v-if="hitItem">
              ·
              {{ $t('llm.mine_summaryRemaining', { amount: hitItem.maxBudget == null ? '∞' : fmtMoney(totalRemaining) }) }}
            </template>
          </div>
        </div>

        <!-- 口径说明：每行三列是「该预算共享池」全体命中用户的累计，非个人消费 -->
        <div
          v-if="sortedList.length > 0"
          class="mb-2 text-xs opacity-60"
        >
          {{ $t('llm.mine_poolHint') }}
        </div>

        <!-- 预算列表 -->
        <Table
          v-if="sortedList.length > 0"
          :columns="budgetColumns"
          :data-source="sortedList"
          :pagination="false"
          row-key="budgetId"
          size="small"
        >
          <!-- 硬/软上限表头：? 浮层解释字段含义（个人口径：本人本周期消费 vs 上限） -->
          <template #headerCell="{ column, title }">
            <template v-if="column.dataIndex === 'maxBudget'">
              {{ headerTitle(title) }}
              <Tooltip :title="$t('llm.mine_maxBudgetTip')">
                <CircleHelp
                  class="ml-0.5 inline-block size-3.5 cursor-help align-[-2px] opacity-50 hover:opacity-100"
                />
              </Tooltip>
            </template>
            <template v-else-if="column.dataIndex === 'softBudget'">
              {{ headerTitle(title) }}
              <Tooltip :title="$t('llm.mine_softBudgetTip')">
                <CircleHelp
                  class="ml-0.5 inline-block size-3.5 cursor-help align-[-2px] opacity-50 hover:opacity-100"
                />
              </Tooltip>
            </template>
            <template v-else>{{ headerTitle(title) }}</template>
          </template>
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'budgetName'">
              <span :class="{ 'opacity-60': record.isExhausted }">
                {{ record.budgetName }}
              </span>
              <Tag v-if="record.budgetId === hitId" color="blue" class="ml-2">
                {{ $t('llm.mine_hit') }}
              </Tag>
            </template>
            <template v-else-if="column.dataIndex === 'budgetDuration'">
              {{ fmtDuration(record.budgetDuration) }}
            </template>
            <template v-else-if="column.dataIndex === 'maxBudget'">
              {{ record.maxBudget == null ? '∞' : fmtMoney(record.maxBudget) }}
            </template>
            <template v-else-if="column.dataIndex === 'softBudget'">
              {{ record.softBudget == null ? '-' : fmtMoney(record.softBudget) }}
            </template>
            <template v-else-if="column.dataIndex === 'spend'">
              <span v-if="record.budgetId === hitId">{{ fmtMoney(record.spend) }}</span>
              <span v-else class="opacity-50">-</span>
            </template>
            <template v-else-if="column.dataIndex === 'remaining'">
              <template v-if="record.budgetId === hitId">
                <span
                  :class="{
                    'font-medium text-red-500': record.isExhausted,
                  }"
                >
                  {{ record.maxBudget == null ? '∞' : fmtMoney(record.remaining) }}
                </span>
              </template>
              <span v-else class="opacity-50">-</span>
            </template>
            <template v-else-if="column.key === 'progress'">
              <Progress
                v-if="record.budgetId === hitId && record.maxBudget != null"
                :percent="Math.min(100, Math.round(record.usedPercent ?? 0))"
                :stroke-color="progressColor(record)"
                size="small"
              />
              <span v-else class="opacity-50">-</span>
            </template>
            <template v-else-if="column.dataIndex === 'budgetResetAt'">
              {{ fmtTime(record.budgetResetAt) }}
            </template>
            <template v-else-if="column.dataIndex === 'status'">
              <!-- 状态只对命中预算判定（其余行口径同已用/剩余，显示 -）；
                   健康（含不限额）也要给出「正常」，不能整列空着像坏掉 -->
              <template v-if="record.budgetId === hitId">
                <Tag v-if="record.isExhausted" color="error">
                  {{ $t('llm.mine_exhausted') }}
                </Tag>
                <Tag v-else-if="record.isLow" color="warning">
                  {{ $t('llm.mine_low') }}
                </Tag>
                <Tag v-else color="success">
                  {{ $t('llm.mine_normal') }}
                </Tag>
              </template>
              <span v-else class="opacity-50">-</span>
            </template>
          </template>
        </Table>

        <!-- 空态 -->
        <Empty
          v-else-if="!loading"
          :description="$t('llm.mine_empty')"
          class="py-12"
        />
      </Spin>
    </Card>

    <!-- 历史周期消费 -->
    <Card class="mt-4">
      <template #title>{{ $t('llm.mine_history_title') }}</template>
      <template #extra>
        <RadioGroup
          :value="historyPeriod"
          size="small"
          option-type="button"
          :options="[
            { label: $t('llm.mine_history_day'), value: 'day' },
            { label: $t('llm.mine_history_week'), value: 'week' },
            { label: $t('llm.mine_history_month'), value: 'month' },
          ]"
          @change="onHistoryChange"
        />
      </template>
      <Spin :spinning="historyLoading">
        <Empty
          v-if="!historyLoading && historyPoints.length === 0"
          :description="$t('llm.mine_history_empty')"
          class="py-12"
        />
        <div v-else class="h-[280px]">
          <EchartsUI ref="chartRef" height="100%" />
        </div>
      </Spin>
    </Card>

    <!-- 申请临时预算（个人功能，基础预算自动选取，只读展示） -->
    <Modal
      v-model:open="reqModalOpen"
      :title="$t('llm.req_modalTitle')"
      :confirm-loading="reqSaving"
      :ok-text="$t('llm.req_submit')"
      @ok="submitReq"
    >
      <div class="flex flex-col gap-4 py-2">
        <div class="text-sm opacity-70">
          {{ $t('llm.req_base') }}：{{ reqForm.baseBudgetName || '-' }}
          <span class="ml-1">（{{ $t('llm.req_baseAuto') }}）</span>
        </div>
        <div>
          <div class="mb-1">{{ $t('llm.req_amount') }}</div>
          <InputNumber
            v-model:value="reqForm.amount"
            :min="0.01"
            :precision="2"
            :placeholder="$t('llm.req_amountPlaceholder')"
            class="w-full"
          />
        </div>
        <div>
          <div class="mb-1">{{ $t('llm.budget_duration') }}</div>
          <Select v-model:value="reqForm.duration" :options="durationOptions" class="w-full" />
        </div>
        <div>
          <div class="mb-1">{{ $t('llm.req_reason') }}</div>
          <Textarea
            v-model:value="reqForm.reason"
            :rows="3"
            :maxlength="500"
            show-count
            :placeholder="$t('llm.req_reasonPlaceholder')"
          />
        </div>
      </div>
    </Modal>
  </Page>
</template>
