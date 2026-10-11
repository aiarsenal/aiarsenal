<script lang="ts" setup>
/**
 * 我的日程：日/周/月三视图（自绘轻日历，无第三方日历组件依赖）。
 * 数据=窗口内场次（后端周期展开+例外应用），点击场次看详情；
 * 参会人可回执，组织者可编辑/取消（整系列或单场）。
 */
import { computed, ref } from 'vue';
import dayjs, { type Dayjs } from 'dayjs';

import { Page } from '@vben/common-ui';

import {
  Button,
  Empty,
  message,
  Modal,
  Popconfirm,
  Spin,
  Tag,
  Tooltip,
} from 'ant-design-vue';

import {
  addScheduleExceptionApi,
  deleteScheduleEventApi,
  getMyScheduleApi,
  respondScheduleApi,
  type ScheduleApi,
} from '#/api/system/schedule';
import { $t } from '#/locales';
import EventModal from '../_shared/event-modal.vue';

defineOptions({ name: 'ScheduleMy' });

type ViewMode = 'day' | 'week' | 'month';

const view = ref<ViewMode>('week');
const anchor = ref<Dayjs>(dayjs());
const loading = ref(false);
const occurrences = ref<ScheduleApi.Occurrence[]>([]);

const eventModalRef = ref<InstanceType<typeof EventModal>>();

// ===== 窗口与格子 =====
function startOfWeek(d: Dayjs): Dayjs {
  const wd = d.day();
  return d.subtract(wd === 0 ? 6 : wd - 1, 'day').startOf('day');
}

const windowRange = computed<{ from: string; to: string }>(() => {
  if (view.value === 'day') {
    return {
      from: anchor.value.format('YYYY-MM-DD'),
      to: anchor.value.add(1, 'day').format('YYYY-MM-DD'),
    };
  }
  if (view.value === 'week') {
    const s = startOfWeek(anchor.value);
    return {
      from: s.format('YYYY-MM-DD'),
      to: s.add(7, 'day').format('YYYY-MM-DD'),
    };
  }
  const s = startOfWeek(anchor.value.startOf('month'));
  return {
    from: s.format('YYYY-MM-DD'),
    to: s.add(42, 'day').format('YYYY-MM-DD'),
  };
});

const WEEKDAYS = computed(() => [
  $t('schedule.day_mo'), $t('schedule.day_tu'), $t('schedule.day_we'),
  $t('schedule.day_th'), $t('schedule.day_fr'), $t('schedule.day_sa'),
  $t('schedule.day_su'),
]);

/** 周视图列（或月视图格）：日期 + 当天场次 */
interface DayCell {
  key: string;
  day: Dayjs;
  inMonth: boolean;
  isToday: boolean;
  items: ScheduleApi.Occurrence[];
}

function cellOf(d: Dayjs): DayCell {
  const key = d.format('YYYY-MM-DD');
  return {
    key,
    day: d,
    inMonth: d.month() === anchor.value.month() || view.value !== 'month',
    isToday: d.isSame(dayjs(), 'day'),
    items: occurrences.value.filter((o) => o.start.slice(0, 10) === key),
  };
}

const dayCells = computed<DayCell[]>(() => {
  const { from } = windowRange.value;
  const start = dayjs(from);
  const n = view.value === 'week' ? 7 : view.value === 'month' ? 42 : 1;
  return Array.from({ length: n }, (_, i) => cellOf(start.add(i, 'day')));
});

async function load() {
  loading.value = true;
  try {
    const res = await getMyScheduleApi(windowRange.value);
    occurrences.value = res?.list ?? [];
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

function shift(dir: 1 | -1) {
  const unit = view.value === 'month' ? 'month' : view.value === 'week' ? 'week' : 'day';
  anchor.value = anchor.value.add(dir, unit);
  void load();
}
function goToday() {
  anchor.value = dayjs();
  void load();
}
function setView(v: ViewMode) {
  view.value = v;
  void load();
}

// ===== 展示辅助 =====
const TYPE_COLOR: Record<number, string> = {
  0: 'var(--sched-c-personal)',
  1: 'var(--sched-c-meeting)',
  2: 'var(--sched-c-online)',
  3: 'var(--sched-c-hybrid)',
};
const TYPE_LABEL = computed<Record<number, string>>(() => ({
  0: $t('schedule.type_personal'),
  1: $t('schedule.type_meeting'),
  2: $t('schedule.type_online'),
  3: $t('schedule.type_hybrid'),
}));
function hm(s: string) {
  return s.slice(11, 16);
}
function placeText(o: ScheduleApi.Occurrence) {
  const parts: string[] = [];
  if (o.roomName) parts.push(o.roomName);
  if (o.location) parts.push(o.location);
  if (o.onlineUrl) parts.push($t('schedule.online_tag'));
  return parts.join(' · ');
}
const MY_STATUS = computed<Record<number, { text: string; color: string }>>(() => ({
  0: { text: $t('schedule.status_pending'), color: 'warning' },
  1: { text: $t('schedule.status_accepted'), color: 'success' },
  2: { text: $t('schedule.status_declined'), color: 'error' },
}));

const rangeTitle = computed(() => {
  const a = anchor.value;
  if (view.value === 'day') return a.format('YYYY-MM-DD dddd');
  if (view.value === 'week') {
    const s = startOfWeek(a);
    return `${s.format('YYYY-MM-DD')} ~ ${s.add(6, 'day').format('YYYY-MM-DD')}`;
  }
  return a.format('YYYY-MM');
});

// ===== 详情弹窗 =====
const detailVisible = ref(false);
const detail = ref<ScheduleApi.Occurrence | null>(null);
function openDetail(o: ScheduleApi.Occurrence) {
  detail.value = o;
  detailVisible.value = true;
}

async function respond(status: 1 | 2) {
  if (!detail.value) return;
  try {
    await respondScheduleApi({ eventId: detail.value.eventId, status });
    message.success(status === 1 ? $t('schedule.accept_success') : $t('schedule.decline_success'));
    detailVisible.value = false;
    void load();
  } catch { /* 拦截器已提示 */ }
}

function confirmCancelSeries() {
  if (!detail.value) return;
  const id = detail.value.eventId;
  const title = detail.value.title;
  Modal.confirm({
    title: $t('schedule.cancel_series_title'),
    content: $t('schedule.cancel_series_content', { title }),
    okType: 'danger',
    okText: $t('schedule.confirm'),
    cancelText: $t('schedule.cancel'),
    onOk: async () => {
      await deleteScheduleEventApi([id]);
      message.success($t('schedule.cancel_success'));
      detailVisible.value = false;
      void load();
    },
  });
}

function confirmCancelSingle() {
  if (!detail.value) return;
  const { eventId, date, title } = detail.value;
  Modal.confirm({
    title: $t('schedule.cancel_single_title'),
    content: $t('schedule.cancel_single_content', { title, date }),
    okType: 'danger',
    okText: $t('schedule.confirm'),
    cancelText: $t('schedule.cancel'),
    onOk: async () => {
      await addScheduleExceptionApi({ eventId, date, status: 0 });
      message.success($t('schedule.cancel_success'));
      detailVisible.value = false;
      void load();
    },
  });
}

function openEditFromDetail() {
  if (!detail.value) return;
  detailVisible.value = false;
  eventModalRef.value?.open({ eventId: detail.value.eventId });
}

function onSaved() {
  void load();
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="sched-page">
      <!-- 工具栏 -->
      <div class="sched-toolbar">
        <div class="sched-toolbar-left">
          <div class="sched-view-switch">
            <Button
              v-for="v in (['day', 'week', 'month'] as ViewMode[])"
              :key="v"
              :type="view === v ? 'primary' : 'default'"
              size="small"
              @click="setView(v)"
            >
              {{ $t(`schedule.view_${v}`) }}
            </Button>
          </div>
          <Button size="small" @click="shift(-1)">‹</Button>
          <span class="sched-range-title">{{ rangeTitle }}</span>
          <Button size="small" @click="shift(1)">›</Button>
          <Button size="small" @click="goToday">{{ $t('schedule.today') }}</Button>
        </div>
        <Button type="primary" @click="eventModalRef?.open()">
          {{ $t('schedule.add') }}
        </Button>
      </div>

      <!-- 图例 -->
      <div class="sched-legend">
        <span
          v-for="(label, t) in TYPE_LABEL"
          :key="t"
          class="sched-legend-item"
        >
          <i :style="{ background: TYPE_COLOR[t] }" />
          {{ label }}
        </span>
        <span class="sched-legend-item">
          <i class="sched-dot" />
          {{ $t('schedule.legend_recurring') }}
        </span>
      </div>

      <Spin :spinning="loading">
        <!-- 日/周视图 -->
        <div
          v-if="view !== 'month'"
          class="sched-week"
          :class="{ 'sched-week--day': view === 'day' }"
        >
          <div v-for="cell in dayCells" :key="cell.key" class="sched-day-col">
            <div
              class="sched-day-head"
              :class="{
                'is-today': cell.isToday,
                'out-month': !cell.inMonth,
              }"
              @click="anchor = cell.day; view = 'day'; load()"
            >
              <span class="sched-day-dow">
                {{ WEEKDAYS[cell.day.day() === 0 ? 6 : cell.day.day() - 1] }}
              </span>
              <span class="sched-day-num">{{ cell.day.date() }}</span>
            </div>
            <div class="sched-day-body">
              <Empty
                v-if="cell.items.length === 0"
                :image="Empty.PRESENTED_IMAGE_SIMPLE"
                :description="$t('schedule.empty_day')"
                :image-style="{ height: '28px' }"
              />
              <Tooltip
                v-for="o in cell.items"
                :key="o.eventId + o.date + o.start"
              >
                <template #title>
                  {{ o.title }}（{{ hm(o.start) }}-{{ hm(o.end) }}）
                  <template v-if="placeText(o)"> · {{ placeText(o) }}</template>
                </template>
                <div
                  class="sched-occ"
                  :style="{ '--c': TYPE_COLOR[o.eventType] }"
                  @click="openDetail(o)"
                >
                  <span v-if="o.isRecurring === 1" class="sched-occ-rec">↻</span>
                  <span v-if="o.modified" class="sched-occ-rec">✎</span>
                  <span class="sched-occ-time">{{ hm(o.start) }}</span>
                  <span class="sched-occ-title">{{ o.title }}</span>
                  <Tag
                    v-if="!o.isOrganizer && o.myStatus === 0"
                    class="sched-occ-tag"
                    color="warning"
                  >
                    {{ $t('schedule.status_pending') }}
                  </Tag>
                </div>
              </Tooltip>
            </div>
          </div>
        </div>

        <!-- 月视图 -->
        <div v-else class="sched-month">
          <div class="sched-month-head">
            <span v-for="w in WEEKDAYS" :key="w">{{ w }}</span>
          </div>
          <div class="sched-month-grid">
            <div
              v-for="cell in dayCells"
              :key="cell.key"
              class="sched-month-cell"
              :class="{
                'out-month': !cell.inMonth,
                'is-today': cell.isToday,
              }"
              @click="anchor = cell.day; view = 'day'; load()"
            >
              <div class="sched-month-date">{{ cell.day.date() }}</div>
              <div
                v-for="o in cell.items.slice(0, 3)"
                :key="o.eventId + o.date + o.start"
                class="sched-mini"
                :style="{ '--c': TYPE_COLOR[o.eventType] }"
                @click.stop="openDetail(o)"
              >
                {{ hm(o.start) }} {{ o.title }}
              </div>
              <div
                v-if="cell.items.length > 3"
                class="sched-mini-more"
                @click.stop="anchor = cell.day; view = 'day'; load()"
              >
                {{ $t('schedule.more_items', { count: cell.items.length - 3 }) }}
              </div>
            </div>
          </div>
        </div>
      </Spin>
    </div>

    <!-- 详情弹窗 -->
    <Modal
      v-model:open="detailVisible"
      :title="detail?.title ?? ''"
      :width="560"
      :footer="null"
    >
      <div v-if="detail" class="sched-detail">
        <p>
          <b>{{ $t('schedule.event_type') }}：</b>
          {{ TYPE_LABEL[detail.eventType] }}
          <Tag v-if="detail.isRecurring === 1" class="ml-1">
            {{ detail.repeatDesc }}
          </Tag>
          <Tag v-if="detail.modified" color="orange">
            {{ $t('schedule.modified_tag') }}
          </Tag>
        </p>
        <p>
          <b>{{ $t('schedule.time') }}：</b>
          {{ detail.start }} ~ {{ hm(detail.end) }}
        </p>
        <p v-if="placeText(detail)">
          <b>{{ $t('schedule.place') }}：</b>{{ placeText(detail) }}
        </p>
        <p v-if="detail.onlineUrl" class="sched-break">
          <b>{{ $t('schedule.online_url') }}：</b>
          <a :href="detail.onlineUrl" target="_blank" rel="noopener">
            {{ detail.onlineUrl }}
          </a>
        </p>
        <p>
          <b>{{ $t('schedule.organizer') }}：</b>{{ detail.organizerName || detail.organizerId }}
        </p>
        <p v-if="detail.remark" class="sched-break">
          <b>{{ $t('schedule.remark') }}：</b>{{ detail.remark }}
        </p>
        <div v-if="detail.attendees.length > 0">
          <b>{{ $t('schedule.attendees') }}：</b>
          <div class="sched-att-list">
            <span
              v-for="a in detail.attendees"
              :key="a.userId"
              class="sched-att"
            >
              {{ a.userName || a.userId }}
              <Tag
                v-if="MY_STATUS[a.status]"
                :color="MY_STATUS[a.status]!.color"
                class="sched-att-tag"
              >
                {{ MY_STATUS[a.status]!.text }}
              </Tag>
            </span>
          </div>
        </div>

        <div class="sched-detail-actions">
          <template v-if="!detail.isOrganizer && detail.myStatus === 0">
            <Popconfirm
              :title="$t('schedule.confirm_accept')"
              @confirm="respond(1)"
            >
              <Button type="primary" size="small">
                {{ $t('schedule.accept') }}
              </Button>
            </Popconfirm>
            <Popconfirm
              :title="$t('schedule.confirm_decline')"
              @confirm="respond(2)"
            >
              <Button danger size="small" class="ml-2">
                {{ $t('schedule.decline') }}
              </Button>
            </Popconfirm>
          </template>
          <template v-if="detail.isOrganizer">
            <Button type="primary" size="small" @click="openEditFromDetail">
              {{ $t('schedule.edit') }}
            </Button>
            <Button
              v-if="detail.isRecurring === 1"
              danger
              size="small"
              class="ml-2"
              @click="confirmCancelSingle"
            >
              {{ $t('schedule.cancel_single') }}
            </Button>
            <Button danger size="small" class="ml-2" @click="confirmCancelSeries">
              {{ $t('schedule.cancel_series') }}
            </Button>
          </template>
        </div>
      </div>
    </Modal>

    <EventModal ref="eventModalRef" @saved="onSaved" />
  </Page>
</template>

<style scoped>
.sched-page {
  --sched-c-personal: hsl(var(--primary) / 0.9);
  --sched-c-meeting: hsl(152 60% 38%);
  --sched-c-online: hsl(210 75% 45%);
  --sched-c-hybrid: hsl(280 55% 50%);
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  min-height: 0;
}

.sched-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.sched-toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.sched-view-switch {
  display: inline-flex;
  margin-right: 4px;
}
.sched-view-switch .ant-btn + .ant-btn {
  margin-left: -1px;
}
.sched-range-title {
  min-width: 170px;
  text-align: center;
  font-weight: 600;
}

.sched-legend {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  font-size: 12px;
  opacity: 0.85;
}
.sched-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.sched-legend-item i {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  display: inline-block;
}
.sched-dot {
  border-radius: 50%;
  background: hsl(var(--muted-foreground) / 0.8);
}

/* ===== 周/日 ===== */
.sched-week {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 6px;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.sched-week--day {
  grid-template-columns: minmax(0, 720px);
  justify-content: center;
}
.sched-day-col {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 240px;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--card));
  overflow: hidden;
}
.sched-day-head {
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 6px;
  padding: 6px 4px;
  cursor: pointer;
  border-bottom: 1px solid hsl(var(--border));
  background: color-mix(in srgb, hsl(var(--muted)) 55%, transparent);
}
.sched-day-head.is-today .sched-day-num {
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
  border-radius: 50%;
  min-width: 22px;
  text-align: center;
  color: white;
}
.sched-day-head.out-month {
  opacity: 0.45;
}
.sched-day-dow {
  font-size: 12px;
  opacity: 0.75;
}
.sched-day-num {
  font-size: 15px;
  font-weight: 600;
}
.sched-day-body {
  flex: 1;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow: auto;
}

.sched-occ {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px 6px;
  border-radius: 6px;
  border-left: 3px solid var(--c);
  background: color-mix(in srgb, var(--c) 12%, transparent);
  cursor: pointer;
  font-size: 12px;
  min-width: 0;
}
.sched-occ:hover {
  background: color-mix(in srgb, var(--c) 22%, transparent);
}
.sched-occ-rec {
  font-size: 11px;
  opacity: 0.8;
}
.sched-occ-time {
  font-weight: 600;
  white-space: nowrap;
}
.sched-occ-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sched-occ-tag {
  margin-left: auto;
  font-size: 11px;
  line-height: 16px;
  padding: 0 4px;
}

/* ===== 月 ===== */
.sched-month {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--card));
  overflow: hidden;
}
.sched-month-head {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  border-bottom: 1px solid hsl(var(--border));
  background: color-mix(in srgb, hsl(var(--muted)) 55%, transparent);
}
.sched-month-head span {
  padding: 6px 4px;
  text-align: center;
  font-size: 12px;
  font-weight: 600;
}
.sched-month-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  grid-auto-rows: minmax(88px, 1fr);
  overflow: auto;
}
.sched-month-cell {
  border-top: 1px solid hsl(var(--border));
  border-left: 1px solid hsl(var(--border));
  padding: 3px 4px;
  min-width: 0;
  overflow: hidden;
  cursor: pointer;
}
.sched-month-cell:nth-child(7n + 1) {
  border-left: none;
}
.sched-month-cell:nth-child(-n + 7) {
  border-top: none;
}
.sched-month-cell.out-month {
  opacity: 0.4;
}
.sched-month-cell.is-today .sched-month-date {
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
  border-radius: 50%;
  display: inline-flex;
  min-width: 20px;
  height: 20px;
  align-items: center;
  justify-content: center;
}
.sched-month-date {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 2px;
}
.sched-mini {
  font-size: 11px;
  border-left: 2px solid var(--c);
  background: color-mix(in srgb, var(--c) 12%, transparent);
  border-radius: 4px;
  padding: 0 4px;
  margin-bottom: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sched-mini:hover {
  background: color-mix(in srgb, var(--c) 24%, transparent);
}
.sched-mini-more {
  font-size: 11px;
  opacity: 0.7;
}

/* ===== 详情 ===== */
.sched-detail p {
  margin-bottom: 8px;
}
.sched-break {
  word-break: break-all;
}
.sched-att-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}
.sched-att {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.sched-att-tag {
  font-size: 11px;
  line-height: 16px;
  padding: 0 4px;
}
.sched-detail-actions {
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px solid hsl(var(--border));
}

@media (max-width: 900px) {
  .sched-week {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
