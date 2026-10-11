<script lang="ts" setup>
/**
 * 会议室当日时间轴：每房一行 08:00-22:00 横轴，占用区间色块（悬浮看会议详情），
 * 点击空白处从该时刻起预订该房间（复用日程新建弹窗）。
 */
import { computed, onMounted, ref } from 'vue';
import dayjs, { type Dayjs } from 'dayjs';

import { Page } from '@vben/common-ui';

import { Button, DatePicker, Empty, Spin, Tag, Tooltip } from 'ant-design-vue';

import { getRoomOverviewApi, type ScheduleApi } from '#/api/system/schedule';
import { $t } from '#/locales';
import EventModal from '../_shared/event-modal.vue';

defineOptions({ name: 'ScheduleRoom' });

const DAY_START = 8 * 60; // 08:00
const DAY_END = 22 * 60; // 22:00
const SPAN = DAY_END - DAY_START;

const date = ref<Dayjs>(dayjs());
const loading = ref(false);
const rooms = ref<ScheduleApi.RoomDay[]>([]);
const eventModalRef = ref<InstanceType<typeof EventModal>>();

const hours = Array.from({ length: 15 }, (_, i) => DAY_START / 60 + i); // 8..22

const dateStr = computed(() => date.value.format('YYYY-MM-DD'));

async function load() {
  loading.value = true;
  try {
    const res = await getRoomOverviewApi(dateStr.value);
    rooms.value = res?.list ?? [];
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

function minutesOf(t: string): number {
  const m = /(\d{2}):(\d{2}):?\d{0,2}/.exec(t);
  if (!m) return DAY_START;
  return Number(m[1]) * 60 + Number(m[2]);
}

function pct(min: number): string {
  return `${((min - DAY_START) / SPAN) * 100}%`;
}

function equipList(r: ScheduleApi.RoomDay): string[] {
  try {
    return JSON.parse(r.equipment || '[]');
  } catch {
    return [];
  }
}

/** 点击时间轴空白 → 该时刻（取整 15 分钟）起新建预订 */
function onTrackClick(room: ScheduleApi.RoomDay, ev: MouseEvent) {
  const track = ev.currentTarget as HTMLElement;
  const rect = track.getBoundingClientRect();
  const x = ev.clientX - rect.left;
  const min = DAY_START + (x / rect.width) * SPAN;
  const clamped = Math.min(Math.max(Math.round(min / 15) * 15, DAY_START), DAY_END - 30);
  eventModalRef.value?.open({
    date: dateStr.value,
    startMinute: clamped,
    roomId: room.roomId,
  });
}

onMounted(load);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="room-page">
      <div class="room-toolbar">
        <DatePicker
          :value="date"
          :format="'YYYY-MM-DD'"
          :allow-clear="false"
          @change="(v: any) => { if (v) { date = v; load(); } }"
        />
        <Button
          size="small"
          @click="date = dayjs(); load()"
        >
          {{ $t('schedule.today') }}
        </Button>
        <span class="room-hint">{{ $t('schedule.room_click_hint') }}</span>
      </div>

      <Spin :spinning="loading">
        <Empty
          v-if="!loading && rooms.length === 0"
          :description="$t('schedule.room_empty')"
        />
        <div v-else class="room-list">
          <div class="room-head-row">
            <div class="room-name-col" />
            <div class="room-track-head">
              <span
                v-for="h in hours"
                :key="h"
                class="room-hour"
                :style="{ left: pct(h * 60) }"
              >
                {{ String(h).padStart(2, '0') }}:00
              </span>
            </div>
          </div>

          <div
            v-for="r in rooms"
            :key="r.roomId"
            class="room-row"
          >
            <div class="room-name-col">
              <div class="room-name">{{ r.name }}</div>
              <div class="room-meta">
                {{ $t('schedule.capacity') }} {{ r.capacity }}
                <template v-if="r.location"> · {{ r.location }}</template>
              </div>
              <div v-if="equipList(r).length" class="room-equip">
                <Tag v-for="e in equipList(r)" :key="e" class="room-equip-tag">
                  {{ e }}
                </Tag>
              </div>
            </div>
            <div class="room-track" @click="onTrackClick(r, $event)">
              <span
                v-for="h in hours"
                :key="h"
                class="room-grid"
                :style="{ left: pct(h * 60) }"
              />
              <Tooltip
                v-for="b in r.busy"
                :key="b.eventId + b.start"
              >
                <template #title>
                  {{ b.title }}（{{ b.start.slice(11, 16) }}-{{ b.end.slice(11, 16) }}）
                  <template v-if="b.organizerName"> · {{ b.organizerName }}</template>
                </template>
                <div
                  class="room-busy"
                  :style="{
                    left: pct(minutesOf(b.start)),
                    width: `${((minutesOf(b.end) - minutesOf(b.start)) / SPAN) * 100}%`,
                  }"
                  @click.stop
                >
                  {{ b.title }}
                </div>
              </Tooltip>
            </div>
          </div>
        </div>
      </Spin>

      <EventModal ref="eventModalRef" @saved="load" />
    </div>
  </Page>
</template>

<style scoped>
.room-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
}
.room-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.room-hint {
  font-size: 12px;
  opacity: 0.7;
}

.room-list {
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--card));
  overflow: auto;
}
.room-head-row,
.room-row {
  display: flex;
  align-items: stretch;
}
.room-head-row {
  position: sticky;
  top: 0;
  z-index: 2;
  background: hsl(var(--card));
  border-bottom: 1px solid hsl(var(--border));
}
.room-name-col {
  flex: 0 0 200px;
  min-width: 0;
  padding: 8px 10px;
  border-right: 1px solid hsl(var(--border));
}
.room-name {
  font-weight: 600;
}
.room-meta {
  font-size: 12px;
  opacity: 0.75;
}
.room-equip {
  margin-top: 2px;
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
.room-equip-tag {
  font-size: 11px;
  line-height: 16px;
  padding: 0 4px;
}

.room-track-head {
  position: relative;
  flex: 1;
  height: 26px;
}
.room-hour {
  position: absolute;
  top: 4px;
  transform: translateX(-50%);
  font-size: 11px;
  opacity: 0.7;
}
.room-track {
  position: relative;
  flex: 1;
  min-height: 64px;
  cursor: copy;
  background:
    repeating-linear-gradient(
      to right,
      transparent 0,
      transparent calc(100% / 14 - 1px),
      hsl(var(--border) / 0.4) calc(100% / 14 - 1px),
      hsl(var(--border) / 0.4) calc(100% / 14)
    );
}
.room-track:hover {
  background-color: color-mix(in srgb, hsl(var(--primary)) 4%, transparent);
}
.room-grid {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: hsl(var(--border) / 0.5);
}
.room-busy {
  position: absolute;
  top: 10px;
  bottom: 10px;
  border-radius: 6px;
  background: color-mix(in srgb, hsl(var(--primary)) 22%, transparent);
  border: 1px solid color-mix(in srgb, hsl(var(--primary)) 55%, transparent);
  color: hsl(var(--foreground));
  font-size: 12px;
  line-height: 1.2;
  padding: 2px 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: default;
}
.room-row + .room-row {
  border-top: 1px solid hsl(var(--border));
}

@media (max-width: 720px) {
  .room-name-col {
    flex-basis: 120px;
  }
}
</style>
