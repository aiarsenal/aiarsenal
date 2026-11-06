<script lang="ts" setup>
import { onMounted, reactive, ref } from 'vue';

import {
  Button,
  Drawer,
  Empty,
  Input,
  message,
  Modal,
  Radio,
  RadioGroup,
  Select,
  Spin,
  Switch,
  Tag,
} from 'ant-design-vue';

import {
  addCalendarSyncAccountApi,
  deleteCalendarSyncAccountApi,
  discoverCalendarsApi,
  editCalendarSyncAccountApi,
  getCalendarSyncAccountsApi,
  runCalendarSyncApi,
} from '#/api/system/schedule';
import type { ScheduleApi } from '#/api/system/schedule';

defineOptions({ name: 'CalendarSyncDrawer' });

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ 'update:open': [boolean] }>();

const loading = ref(false);
const accounts = ref<ScheduleApi.CalendarAccount[]>([]);
const syncRunning = ref('');
const formOpen = ref(false);
const saving = ref(false);
const discovering = ref(false);
const editingId = ref('');
const calendars = ref<ScheduleApi.DiscoveredCalendar[]>([]);

const form = reactive({
  name: '',
  serverUrl: '',
  calendarUrl: '',
  username: '',
  password: '',
  enabled: true,
  syncIntervalMinutes: 15,
  conflictPolicy: 'manual' as 'local' | 'manual' | 'remote',
});

function resetForm() {
  Object.assign(form, {
    name: '',
    serverUrl: '',
    calendarUrl: '',
    username: '',
    password: '',
    enabled: true,
    syncIntervalMinutes: 15,
    conflictPolicy: 'manual',
  });
  calendars.value = [];
}

async function loadAccounts() {
  loading.value = true;
  try {
    const result = await getCalendarSyncAccountsApi();
    accounts.value = result?.list ?? [];
  } catch (error: any) {
    message.error(error?.message || '读取日历同步账号失败');
  } finally {
    loading.value = false;
  }
}

function openAdd() {
  editingId.value = '';
  resetForm();
  formOpen.value = true;
}

function openEdit(account: ScheduleApi.CalendarAccount) {
  editingId.value = account.accountId;
  Object.assign(form, {
    name: account.name,
    serverUrl: account.serverUrl,
    calendarUrl: account.calendarUrl,
    username: account.username,
    password: '',
    enabled: account.enabled === 1,
    syncIntervalMinutes: account.syncIntervalMinutes,
    conflictPolicy: account.conflictPolicy,
  });
  calendars.value = [
    { displayName: account.name, url: account.calendarUrl },
  ];
  formOpen.value = true;
}

async function discover() {
  if (!form.serverUrl || !form.username || !form.password) {
    message.warning('请先填写 CalDAV 地址、用户名和密码');
    return;
  }
  discovering.value = true;
  try {
    const result = await discoverCalendarsApi({
      password: form.password,
      serverUrl: form.serverUrl,
      username: form.username,
    });
    calendars.value = result?.list ?? [];
    if (calendars.value.length === 1) {
      form.calendarUrl = calendars.value[0]!.url;
      if (!form.name) form.name = calendars.value[0]!.displayName;
    }
    if (calendars.value.length === 0) {
      message.warning('未发现可写日历，请检查账号权限或直接填写日历地址');
    } else {
      message.success(`发现 ${calendars.value.length} 个日历`);
    }
  } catch (error: any) {
    message.error(error?.message || '发现 CalDAV 日历失败');
  } finally {
    discovering.value = false;
  }
}

async function save() {
  if (
    !form.name.trim() ||
    !form.serverUrl.trim() ||
    !form.calendarUrl.trim() ||
    !form.username.trim()
  ) {
    message.warning('请完整填写账号名称、服务地址、日历地址和用户名');
    return;
  }
  if (!editingId.value && !form.password) {
    message.warning('新增账号需要填写密码');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      ...form,
      enabled: form.enabled ? 1 : 0,
    };
    if (editingId.value) {
      await editCalendarSyncAccountApi({
        ...payload,
        accountId: editingId.value,
      });
      message.success('日历同步账号已更新');
    } else {
      const result = await addCalendarSyncAccountApi(payload);
      message.success('CalDAV 日历已绑定，开始首次同步');
      if (result?.accountId) {
        await runSync(result.accountId, false);
      }
    }
    formOpen.value = false;
    await loadAccounts();
  } catch (error: any) {
    message.error(error?.message || '保存日历同步账号失败');
  } finally {
    saving.value = false;
  }
}

async function runSync(accountId: string, showMessage = true) {
  syncRunning.value = accountId;
  try {
    const result = await runCalendarSyncApi(accountId);
    if (showMessage) {
      message.success(
        `同步完成：拉取 ${result.pulled}，推送 ${result.pushed}，跳过 ${result.skipped ?? 0}，冲突 ${result.conflicts}`,
      );
    }
    await loadAccounts();
  } catch (error: any) {
    message.error(error?.message || '日历同步失败');
  } finally {
    syncRunning.value = '';
  }
}

function remove(account: ScheduleApi.CalendarAccount) {
  Modal.confirm({
    title: '删除日历同步账号？',
    content:
      '仅解除同步绑定，已经写入本系统的日程会保留。不会删除远端日历中的事件。',
    okText: '解除绑定',
    okType: 'danger',
    cancelText: '取消',
    onOk: async () => {
      await deleteCalendarSyncAccountApi(account.accountId);
      message.success('已解除日历同步');
      await loadAccounts();
    },
  });
}

function formatTime(value?: string) {
  if (!value) return '尚未同步';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function statusMeta(status: string) {
  switch (status) {
    case 'success': {
      return { color: 'success', label: '同步正常' };
    }
    case 'running': {
      return { color: 'processing', label: '同步中' };
    }
    case 'conflict': {
      return { color: 'warning', label: '存在冲突' };
    }
    case 'error': {
      return { color: 'error', label: '同步失败' };
    }
    default: {
      return { color: 'default', label: '等待同步' };
    }
  }
}

onMounted(loadAccounts);
</script>

<template>
  <Drawer
    :open="props.open"
    :width="760"
    title="CalDAV 双向日历同步"
    @close="emit('update:open', false)"
  >
    <div class="calendar-sync-panel">
      <header class="sync-intro">
        <div>
          <span>CALDAV / TWO-WAY</span>
          <h3>手机日历与日程台账双向同步</h3>
          <p>
            支持 iCloud（App 专用密码）、Nextcloud、Radicale 等 CalDAV
            服务。当前用户组织的日程会推送到绑定日历，远端变更也会回写本系统。
          </p>
        </div>
        <Button type="primary" @click="openAdd">绑定日历</Button>
      </header>

      <div class="sync-note">
        <strong>冲突保护</strong>
        默认策略为“保留双方、暂停该条映射”，不会静默覆盖本地或远端事件。可在账号设置中改为本地优先或远端优先。
      </div>

      <Spin :spinning="loading">
        <div v-if="accounts.length" class="account-list">
          <article
            v-for="account in accounts"
            :key="account.accountId"
            class="account-card"
          >
            <div class="account-top">
              <div>
                <span class="account-provider">CALDAV</span>
                <h4>{{ account.name }}</h4>
                <p>{{ account.username }} · {{ account.calendarUrl }}</p>
              </div>
              <Tag :color="statusMeta(account.lastSyncStatus).color">
                {{ statusMeta(account.lastSyncStatus).label }}
              </Tag>
            </div>
            <div class="account-metrics">
              <span>
                <b>{{ account.eventCount }}</b>
                已映射事件
              </span>
              <span :class="{ danger: account.conflictCount > 0 }">
                <b>{{ account.conflictCount }}</b>
                冲突
              </span>
              <span>
                <b>{{ account.syncIntervalMinutes }}</b>
                分钟/次
              </span>
              <span>
                <b>{{ account.conflictPolicy === 'manual' ? '手动' : account.conflictPolicy === 'local' ? '本地' : '远端' }}</b>
                冲突策略
              </span>
            </div>
            <div class="account-foot">
              <span>上次同步：{{ formatTime(account.lastSyncAt) }}</span>
              <div>
                <Button size="small" @click="openEdit(account)">设置</Button>
                <Button
                  :loading="syncRunning === account.accountId"
                  class="ml-1"
                  size="small"
                  type="primary"
                  @click="runSync(account.accountId)"
                >
                  立即同步
                </Button>
                <Button
                  class="ml-1"
                  danger
                  size="small"
                  type="text"
                  @click="remove(account)"
                >
                  解除
                </Button>
              </div>
            </div>
            <p v-if="account.lastSyncError" class="sync-error">
              {{ account.lastSyncError }}
            </p>
          </article>
        </div>
        <Empty v-else description="尚未绑定 CalDAV 日历" class="sync-empty" />
      </Spin>
    </div>

    <Modal
      v-model:open="formOpen"
      :confirm-loading="saving"
      :title="editingId ? '编辑 CalDAV 日历' : '绑定 CalDAV 日历'"
      :width="620"
      ok-text="保存"
      cancel-text="取消"
      @ok="save"
    >
      <div class="sync-form">
        <label>
          <span>账号名称</span>
          <Input v-model:value="form.name" placeholder="例如：iPhone 日历" />
        </label>
        <label>
          <span>CalDAV 服务地址</span>
          <Input
            v-model:value="form.serverUrl"
            placeholder="例如：https://caldav.icloud.com"
          />
        </label>
        <div class="form-row">
          <label>
            <span>用户名</span>
            <Input
              v-model:value="form.username"
              placeholder="Apple ID 或 CalDAV 用户名"
            />
          </label>
          <label>
            <span>密码 / App 专用密码</span>
            <Input.Password
              v-model:value="form.password"
              :placeholder="editingId ? '留空表示不修改' : 'iCloud 请使用 App 专用密码'"
            />
          </label>
        </div>
        <div class="discover-row">
          <Button :loading="discovering" @click="discover">发现日历</Button>
          <span>手机端需用同一 CalDAV 账号登录，系统日历才会同步。</span>
        </div>
        <label v-if="calendars.length">
          <span>选择日历</span>
          <Select
            v-model:value="form.calendarUrl"
            :options="
              calendars.map((calendar) => ({
                label: calendar.displayName,
                value: calendar.url,
              }))
            "
            placeholder="请选择可写日历"
            style="width: 100%"
          />
        </label>
        <label>
          <span>日历地址</span>
          <Input
            v-model:value="form.calendarUrl"
            placeholder="发现后自动填入，也可直接填写 calendar collection URL"
          />
        </label>
        <div class="form-row compact">
          <label>
            <span>同步间隔</span>
            <Select
              v-model:value="form.syncIntervalMinutes"
              :options="[
                { label: '每 5 分钟', value: 5 },
                { label: '每 15 分钟', value: 15 },
                { label: '每 30 分钟', value: 30 },
                { label: '每 60 分钟', value: 60 },
              ]"
              style="width: 100%"
            />
          </label>
          <label class="toggle-label">
            <span>启用自动同步</span>
            <Switch v-model:checked="form.enabled" />
          </label>
        </div>
        <label>
          <span>冲突策略</span>
          <RadioGroup v-model:value="form.conflictPolicy">
            <Radio value="manual">保留双方，暂停该条</Radio>
            <Radio value="local">本地优先</Radio>
            <Radio value="remote">远端优先</Radio>
          </RadioGroup>
        </label>
      </div>
    </Modal>
  </Drawer>
</template>

<style scoped>
.calendar-sync-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sync-intro {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px;
  color: #f7f2e8;
  background:
    radial-gradient(circle at 100% 0, rgba(66, 185, 131, 0.24), transparent 35%),
    #111827;
  border-radius: 10px;
}

.sync-intro span {
  color: #78d9ae;
  font-family: Georgia, serif;
  font-size: 10px;
  letter-spacing: 0.16em;
}

.sync-intro h3 {
  margin: 6px 0;
  color: #fff;
  font-size: 20px;
}

.sync-intro p {
  max-width: 540px;
  margin: 0;
  color: rgba(255, 255, 255, 0.62);
  font-size: 12px;
  line-height: 1.7;
}

.sync-note {
  padding: 11px 14px;
  color: #7a5b22;
  font-size: 12px;
  line-height: 1.6;
  background: #fff8df;
  border: 1px solid #f0dfaa;
  border-radius: 7px;
}

.sync-note strong {
  margin-right: 8px;
}

.account-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.account-card {
  padding: 17px 18px;
  background: var(--ant-color-bg-container, #fff);
  border: 1px solid var(--ant-color-border-secondary, #f0f0f0);
  border-radius: 10px;
}

.account-top,
.account-foot {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.account-provider {
  color: #42b983;
  font-family: Georgia, serif;
  font-size: 9px;
  letter-spacing: 0.14em;
}

.account-card h4 {
  margin: 3px 0;
  font-size: 16px;
}

.account-card p {
  max-width: 520px;
  margin: 0;
  overflow: hidden;
  color: var(--ant-color-text-secondary, #888);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  margin: 15px 0;
  border-top: 1px solid var(--ant-color-border-secondary, #f0f0f0);
  border-bottom: 1px solid var(--ant-color-border-secondary, #f0f0f0);
}

.account-metrics span {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  color: var(--ant-color-text-secondary, #888);
  font-size: 10px;
  border-right: 1px solid var(--ant-color-border-secondary, #f0f0f0);
}

.account-metrics span:last-child {
  border-right: 0;
}

.account-metrics b {
  color: var(--ant-color-text, #222);
  font-size: 16px;
}

.account-metrics .danger b {
  color: #d24b35;
}

.account-foot {
  color: var(--ant-color-text-secondary, #888);
  font-size: 11px;
}

.sync-error {
  margin-top: 10px !important;
  padding: 8px 10px;
  color: #b33a28 !important;
  line-height: 1.6;
  white-space: normal !important;
  background: #fff0ed;
  border-radius: 5px;
}

.sync-empty {
  padding: 48px 0;
}

.sync-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 6px 0;
}

.sync-form label {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 6px;
}

.sync-form label > span {
  color: var(--ant-color-text-secondary, #666);
  font-size: 12px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.form-row.compact {
  align-items: center;
}

.toggle-label {
  flex-direction: row !important;
  align-items: center;
  justify-content: space-between;
  padding-top: 22px;
}

.discover-row {
  display: flex;
  gap: 12px;
  align-items: center;
  color: var(--ant-color-text-secondary, #888);
  font-size: 11px;
}

@media (max-width: 700px) {
  .sync-intro,
  .account-top,
  .account-foot,
  .form-row {
    align-items: stretch;
    flex-direction: column;
  }

  .account-metrics {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
