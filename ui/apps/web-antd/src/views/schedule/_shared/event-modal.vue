<script lang="ts" setup>
/**
 * 日程/会议 新建·编辑弹窗（我的日程与会议室时间轴共用）。
 * 提交流程：预检 → 房间冲突时内嵌冲突明细 +「仍要预订」（ignoreConflicts 重提）→ 成功通知。
 * 周期子集与后端对齐：repeat none|daily|weekly|monthly + byday + count/until。
 */
import { computed, reactive, ref } from 'vue';
import dayjs, { type Dayjs } from 'dayjs';

import {
  Alert,
  Button,
  Checkbox,
  DatePicker,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Spin,
} from 'ant-design-vue';

import {
  addScheduleEventApi,
  editScheduleEventApi,
  getRoomListApi,
  getScheduleDetailApi,
  type ScheduleApi,
} from '#/api/system/schedule';
import { searchUserOptionsApi } from '#/api/system/user';
import { $t } from '#/locales';

const emit = defineEmits<{ saved: [] }>();

type Repeat = 'none' | 'daily' | 'weekly' | 'monthly';

const visible = ref(false);
const saving = ref(false);
const loadingDetail = ref(false);
const isEdit = ref(false);
const editingId = ref('');

const EVENT_TYPE_OPTIONS = [
  { label: $t('schedule.type_personal'), value: 0 },
  { label: $t('schedule.type_meeting'), value: 1 },
  { label: $t('schedule.type_online'), value: 2 },
  { label: $t('schedule.type_hybrid'), value: 3 },
];
const WEEKDAY_OPTIONS = [
  { label: $t('schedule.day_mo'), value: 'MO' },
  { label: $t('schedule.day_tu'), value: 'TU' },
  { label: $t('schedule.day_we'), value: 'WE' },
  { label: $t('schedule.day_th'), value: 'TH' },
  { label: $t('schedule.day_fr'), value: 'FR' },
  { label: $t('schedule.day_sa'), value: 'SA' },
  { label: $t('schedule.day_su'), value: 'SU' },
];

const form = reactive({
  title: '',
  eventType: 0,
  range: [null, null] as [Dayjs | null, Dayjs | null],
  repeat: 'none' as Repeat,
  byday: [] as string[],
  count: undefined as number | undefined,
  until: undefined as Dayjs | undefined,
  roomId: '',
  onlineUrl: '',
  location: '',
  attendees: [] as string[],
  remark: '',
});

// 冲突确认（Created=false 且有 Conflicts 时展示；「仍要预订」→ ignoreConflicts 重提）
const conflicts = ref<ScheduleApi.Conflict[]>([]);
let pendingPayload: ScheduleApi.EventInput | null = null;

// ===== 参会人远程搜索 =====
const userOptions = ref<{ label: string; value: string }[]>([]);
const userSearching = ref(false);
let userSearchTimer: ReturnType<typeof setTimeout> | undefined;
function onUserSearch(q: string) {
  if (userSearchTimer) clearTimeout(userSearchTimer);
  userSearchTimer = setTimeout(async () => {
    if (!q.trim()) return;
    userSearching.value = true;
    try {
      const res: any = await searchUserOptionsApi({
        keyWords: q, pageNum: 1, pageSize: 20,
      });
      const found = (res?.list ?? []).map((u: any) => ({
        value: String(u.userId ?? ''),
        label: `${u.name}${u.deptName ? ` / ${u.deptName}` : ''}`,
      })).filter((o: any) => o.value);
      // 与已选合并去重（保留已选 label）
      const known = new Map(userOptions.value.map((o) => [o.value, o]));
      for (const f of found) known.set(f.value, f);
      userOptions.value = [...known.values()];
    } catch {
      /* 拦截器已提示 */
    } finally {
      userSearching.value = false;
    }
  }, 300);
}

// ===== 会议室选项 =====
const roomOptions = ref<{ label: string; value: string }[]>([]);
async function loadRooms() {
  try {
    const res = await getRoomListApi({ pageNum: 1, pageSize: 200, enabledOnly: true });
    roomOptions.value = (res?.list ?? []).map((r) => ({
      value: r.roomId,
      label: `${r.name}（${$t('schedule.capacity')} ${r.capacity}）`,
    }));
  } catch { /* 拦截器已提示 */ }
}

const showByday = computed(() => form.repeat === 'weekly');
const showCount = computed(() => form.repeat !== 'none');
const modalTitle = computed(() =>
  isEdit.value ? $t('schedule.edit_title') : $t('schedule.add_title'),
);

/** 打开：preset 可带 eventId（编辑）/ date+minute（时间轴点空白预订）/ roomId / fullDay */
function open(preset?: {
  eventId?: string;
  date?: string; // YYYY-MM-DD
  startMinute?: number; // 当日分钟数（时间轴点击位置）
  durationMinute?: number;
  roomId?: string;
}) {
  visible.value = true;
  conflicts.value = [];
  pendingPayload = null;
  isEdit.value = false;
  editingId.value = '';
  Object.assign(form, {
    title: '', eventType: 0, range: [], repeat: 'none', byday: [],
    count: undefined, until: undefined, roomId: '', onlineUrl: '',
    location: '', attendees: [], remark: '',
  });
  loadRooms();
  if (preset?.eventId) {
    void fillForEdit(preset.eventId);
    return;
  }
  // 新建缺省：今天此刻起 1 小时（或时间轴点击位）
  const day = preset?.date ? dayjs(preset.date) : dayjs();
  const base = day.startOf('minute');
  const sm = preset?.startMinute;
  const start = sm === undefined
    ? base.add(Math.ceil(dayjs().diff(base, 'minute') / 15) * 15, 'minute')
    : base.minute(0).add(Math.round(sm / 15) * 15, 'minute');
  const dur = preset?.durationMinute ?? 60;
  form.range = [start, start.add(dur, 'minute')];
  if (preset?.roomId) {
    form.roomId = preset.roomId;
    form.eventType = 1;
  }
}

async function fillForEdit(eventId: string) {
  isEdit.value = true;
  editingId.value = eventId;
  loadingDetail.value = true;
  try {
    const d = await getScheduleDetailApi(eventId);
    form.title = d.title;
    form.eventType = d.eventType;
    form.range = [
      d.start ? dayjs(d.start) : null,
      d.end ? dayjs(d.end) : null,
    ];
    form.byday = [];
    form.count = undefined;
    form.until = undefined;
    form.repeat = 'none';
    // rrule 原文解析回表单（后端视图带 rrule 字段）
    const raw = (d as any).rrule;
    if (raw) {
      try {
        const r = typeof raw === 'string' ? JSON.parse(raw) : raw;
        if (r?.freq) {
          form.repeat = String(r.freq).toLowerCase() as Repeat;
          form.byday = Array.isArray(r.byday) ? r.byday : [];
          form.count = r.count > 0 ? r.count : undefined;
          form.until = r.until ? dayjs(r.until) : undefined;
        }
      } catch { /* 忽略坏 rrule */ }
    }
    form.roomId = d.roomId ?? '';
    form.onlineUrl = d.onlineUrl ?? '';
    form.location = d.location ?? '';
    form.attendees = (d.attendees ?? []).map((a) => a.userId);
    form.remark = d.remark ?? '';
    // 已选参会人回显
    const known = new Map(userOptions.value.map((o) => [o.value, o]));
    for (const a of d.attendees ?? []) {
      if (a.userId && !known.has(a.userId)) {
        known.set(a.userId, {
          value: a.userId,
          label: a.userName || a.userId,
        });
      }
    }
    userOptions.value = [...known.values()];
  } catch {
    /* 拦截器已提示 */
    visible.value = false;
  } finally {
    loadingDetail.value = false;
  }
}

function buildRrule(): string {
  if (form.repeat === 'none') return '';
  const r: Record<string, any> = { freq: form.repeat.toUpperCase() };
  if (form.repeat === 'weekly' && form.byday.length > 0) {
    r.byday = [...form.byday];
  }
  if (form.count && form.count > 0) {
    r.count = form.count;
  } else if (form.until) {
    r.until = form.until.format('YYYY-MM-DD');
  }
  return JSON.stringify(r);
}

function buildPayload(ignoreConflicts = false): ScheduleApi.EventInput | null {
  const [s, e] = form.range;
  if (!s || !e) {
    message.warning($t('schedule.time_required'));
    return null;
  }
  if (!form.title.trim()) {
    message.warning($t('schedule.title_required'));
    return null;
  }
  if (!s.isBefore(e)) {
    message.warning($t('schedule.time_order'));
    return null;
  }
  return {
    eventId: isEdit.value ? editingId.value : undefined,
    title: form.title.trim(),
    eventType: form.eventType,
    startAt: s.format('YYYY-MM-DD HH:mm'),
    endAt: e.format('YYYY-MM-DD HH:mm'),
    rrule: buildRrule(),
    roomId: form.roomId,
    onlineUrl: form.onlineUrl.trim(),
    location: form.location.trim(),
    attendees: form.attendees,
    remark: form.remark,
    ignoreConflicts,
  };
}

async function submit(ignoreConflicts = false) {
  const payload = buildPayload(ignoreConflicts);
  if (!payload) return;
  saving.value = true;
  conflicts.value = [];
  try {
    const res = isEdit.value
      ? await editScheduleEventApi(payload)
      : await addScheduleEventApi(payload);
    if (!res?.created && res?.conflicts?.length) {
      pendingPayload = payload;
      conflicts.value = res.conflicts;
      return;
    }
    message.success(
      isEdit.value ? $t('schedule.edit_success') : $t('schedule.add_success'),
    );
    visible.value = false;
    emit('saved');
  } catch {
    /* 拦截器已提示 */
  } finally {
    saving.value = false;
  }
}

/** 忽略冲突强制提交（用户明示后） */
function forceSubmit() {
  if (!pendingPayload) return;
  void submit(true);
}

function onRangeChange(v: any) {
  form.range = v ?? [null, null];
}

defineExpose({ open });
</script>

<template>
  <Modal
    v-model:open="visible"
    :title="modalTitle"
    :width="680"
    :mask-closable="false"
    @ok="submit(false)"
    :ok-text="$t('schedule.submit')"
    :cancel-text="$t('schedule.cancel')"
    :confirm-loading="saving"
  >
    <Spin :spinning="loadingDetail">
      <Form layout="vertical" class="sched-event-form">
        <Form.Item :label="$t('schedule.title')" required>
          <Input
            v-model:value="form.title"
            :placeholder="$t('schedule.title_placeholder')"
            :maxlength="255"
          />
        </Form.Item>
        <div class="sched-form-row">
          <Form.Item :label="$t('schedule.event_type')" class="grow">
            <Select
              v-model:value="form.eventType"
              :options="EVENT_TYPE_OPTIONS"
            />
          </Form.Item>
          <Form.Item :label="$t('schedule.time')" required class="w-double">
            <DatePicker.RangePicker
              :value="(form.range as any)"
              show-time
              :format="'YYYY-MM-DD HH:mm'"
              :minute-step="5"
              style="width: 100%"
              @change="onRangeChange"
            />
          </Form.Item>
        </div>
        <div class="sched-form-row">
          <Form.Item :label="$t('schedule.repeat')" class="grow">
            <Select v-model:value="form.repeat">
              <Select.Option value="none">{{ $t('schedule.repeat_none') }}</Select.Option>
              <Select.Option value="daily">{{ $t('schedule.repeat_daily') }}</Select.Option>
              <Select.Option value="weekly">{{ $t('schedule.repeat_weekly') }}</Select.Option>
              <Select.Option value="monthly">{{ $t('schedule.repeat_monthly') }}</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            v-if="showByday"
            :label="$t('schedule.repeat_byday')"
            class="w-double"
          >
            <Checkbox.Group
              v-model:value="form.byday"
              :options="WEEKDAY_OPTIONS"
            />
          </Form.Item>
        </div>
        <div class="sched-form-row" v-if="showCount">
          <Form.Item :label="$t('schedule.repeat_count')" class="grow">
            <InputNumber
              v-model:value="form.count"
              :min="1"
              :max="999"
              style="width: 100%"
              :placeholder="$t('schedule.repeat_count_placeholder')"
            />
          </Form.Item>
          <Form.Item :label="$t('schedule.repeat_until')" class="grow">
            <DatePicker
              v-model:value="form.until"
              :format="'YYYY-MM-DD'"
              style="width: 100%"
              :placeholder="$t('schedule.repeat_until_placeholder')"
            />
          </Form.Item>
        </div>
        <div class="sched-form-row">
          <Form.Item :label="$t('schedule.room')" class="grow">
            <Select
              v-model:value="form.roomId"
              allow-clear
              :options="roomOptions"
              :placeholder="$t('schedule.room_placeholder')"
            />
          </Form.Item>
          <Form.Item :label="$t('schedule.online_url')" class="grow">
            <Input
              v-model:value="form.onlineUrl"
              :placeholder="$t('schedule.online_url_placeholder')"
            />
          </Form.Item>
        </div>
        <Form.Item :label="$t('schedule.location')">
          <Input
            v-model:value="form.location"
            :placeholder="$t('schedule.location_placeholder')"
          />
        </Form.Item>
        <Form.Item :label="$t('schedule.attendees')">
          <Select
            v-model:value="form.attendees"
            mode="multiple"
            :options="userOptions"
            :filter-option="false"
            :loading="userSearching"
            :placeholder="$t('schedule.attendees_placeholder')"
            @search="onUserSearch"
          />
        </Form.Item>
        <Form.Item :label="$t('schedule.remark')">
          <Input.TextArea
            v-model:value="form.remark"
            :rows="2"
            :maxlength="2000"
            :placeholder="$t('schedule.remark_placeholder')"
          />
        </Form.Item>
      </Form>

      <Alert
        v-if="conflicts.length"
        type="warning"
        show-icon
        class="mt-2"
      >
        <template #message>
          {{ $t('schedule.conflict_title', { count: conflicts.length }) }}
        </template>
        <template #description>
          <div
            v-for="c in conflicts.slice(0, 5)"
            :key="c.eventId + c.start"
            class="sched-conflict-line"
          >
            {{ c.title }}：{{ c.start }} ~ {{ c.end }}
            <span v-if="c.roomName">（{{ c.roomName }}）</span>
          </div>
          <div v-if="conflicts.length > 5" class="sched-conflict-line">
            …{{ $t('schedule.conflict_more', { count: conflicts.length - 5 }) }}
          </div>
          <Button danger size="small" class="mt-2" @click="forceSubmit">
            {{ $t('schedule.conflict_force') }}
          </Button>
        </template>
      </Alert>
    </Spin>
  </Modal>
</template>

<style scoped>
.sched-event-form :deep(.ant-form-item) {
  margin-bottom: 12px;
}
.sched-form-row {
  display: flex;
  gap: 12px;
}
.sched-form-row .grow {
  flex: 1;
  min-width: 0;
}
.sched-form-row .w-double {
  flex: 2;
  min-width: 0;
}
.sched-conflict-line {
  line-height: 1.7;
}
@media (max-width: 640px) {
  .sched-form-row {
    flex-direction: column;
    gap: 0;
  }
}
</style>
