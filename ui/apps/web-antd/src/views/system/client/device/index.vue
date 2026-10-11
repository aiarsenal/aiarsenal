<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, h, onBeforeUnmount, reactive, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  CheckboxGroup,
  Descriptions,
  DescriptionsItem,
  Drawer,
  Input,
  Modal,
  Pagination,
  RangePicker,
  Select,
  Segmented,
  Table,
  Textarea,
  Timeline,
  TimelineItem,
  Tooltip,
  message,
} from 'ant-design-vue';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import McpBindDrawer from '#/components/McpBindDrawer.vue';
import QRCode from 'qrcode';
import {
  blockClientDeviceApi,
  deleteClientDeviceApi,
  enqueueClientCommandApi,
  getClientDeviceChannelsApi,
  getClientDeviceChannelQrApi,
  getClientDeviceCommandsApi,
  getClientDeviceConfigChangesApi,
  getClientDeviceDetailApi,
  getClientDeviceListApi,
  getClientDeviceLogsApi,
  getClientDeviceSkillsApi,
  getClientDeviceStatusHistoryApi,
  getClientOpenclawConfigApi,
  installClientDeviceSkillApi,
  loginClientDeviceChannelApi,
  logoutClientDeviceChannelApi,
  revokeClientDeviceApi,
  setClientDeviceChannelApi,
  setClientDeviceSelectedAgentsApi,
  unbindClientDeviceChannelApi,
  uninstallClientDeviceSkillApi,
} from '#/api/system/client/device';
import type { ClientDeviceApi } from '#/api/system/client/device';
import type { SkillApi } from '#/api/system/skill/skill';
import { getSkillListApi, getSkillVersionsApi } from '#/api/system/skill/skill';
import type { SkillNamespaceApi } from '#/api/system/skill/namespace';
import { getAllSkillNamespacesApi } from '#/api/system/skill/namespace';
import { getClientAgentListApi } from '#/api/system/client/config';
import {
  getClientAppListApi,
  getDeviceAppPermsApi,
  resetDeviceAppPermApi,
  APP_PERM_CATS,
  saveDeviceAppPermsApi,
} from '#/api/system/client/appperm';

defineOptions({ name: 'SystemClientDevice' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyWords',
    label: $t('client.device_keyword'),
    componentProps: { placeholder: $t('client.device_keywordPlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'platform',
    label: $t('client.device_platform'),
    componentProps: {
      allowClear: true,
      options: [
        { label: 'Windows', value: 'win' },
        { label: 'macOS', value: 'mac' },
        { label: 'Linux', value: 'linux' },
      ],
    },
  },
  {
    component: 'Select',
    fieldName: 'blocked',
    label: $t('client.device_blocked'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('client.device_blockedYes'), value: 1 },
        { label: $t('client.device_blockedNo'), value: 0 },
      ],
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('client.device_seq'), width: 60, fixed: 'left' },
    { field: 'name', title: $t('client.device_name'), minWidth: 140 },
    { field: 'platform', title: $t('client.device_platform'), width: 80 },
    { field: 'macAddress', title: $t('client.device_mac'), minWidth: 160 },
    { field: 'appVersion', title: $t('client.device_appVersion'), width: 100 },
    { field: 'lastLoginIp', title: $t('client.device_lastLoginIp'), width: 130 },
    { field: 'lastLoginAt', title: $t('client.device_lastLoginAt'), width: 160 },
    {
      field: 'status',
      title: $t('client.device_status'),
      width: 90,
      slots: { default: 'status' },
    },
    {
      field: 'action',
      title: $t('client.device_action'),
      width: 320,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      // vben 表单值在 query 第 2 参（wrapper 注入），非 vxe-grid params.form（那个为空）
      query: async (params: any, form: any) => {
        const page = params?.page;
        const res = await getClientDeviceListApi();
        let items: ClientDeviceApi.Device[] = res?.list ?? [];
        // 前端本地过滤（gin 不分页、不接受搜索参数）
        const kw = (form?.keyWords ?? '').trim().toLowerCase();
        if (kw) {
          items = items.filter(
            (d) =>
              (d.name ?? '').toLowerCase().includes(kw) ||
              (d.userName ?? '').toLowerCase().includes(kw) ||
              (d.macAddress ?? '').toLowerCase().includes(kw),
          );
        }
        if (form?.platform) {
          items = items.filter((d) => d.platform === form.platform);
        }
        if (form?.blocked !== undefined && form?.blocked !== null) {
          items = items.filter(
            (d) => Number(d.blocked) === Number(form.blocked),
          );
        }
        const total = items.length;
        const start = (page.currentPage - 1) * page.pageSize;
        const paged = items.slice(start, start + page.pageSize);
        return { items: paged, total };
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

// ===== 状态标签 =====
function statusTagOf(row: { blocked?: number; status?: string }) {
  if (Number(row.blocked) === 1) {
    return { color: 'red', text: $t('client.device_statusBlocked') };
  }
  if (row.status === 'active') {
    return { color: 'magenta', text: $t('client.device_statusOnline') };
  }
  if (row.status === 'pending') {
    return { color: 'orange', text: $t('client.device_statusLogging') };
  }
  return { color: 'default', text: $t('client.device_statusRevoked') };
}

// ===== 操作：撤销 / 封禁 / 解封 / 删除 =====
function onRevoke(row: ClientDeviceApi.Device) {
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.device_revokeContent', {
      name: row.name || row.deviceId,
    }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    onOk: async () => {
      await revokeClientDeviceApi(row.deviceId);
      message.success($t('client.device_revoked'));
      gridApi.query();
    },
  });
}

function onBlock(row: ClientDeviceApi.Device, blocked: number) {
  const label =
    blocked === 1
      ? $t('client.device_block')
      : $t('client.device_unblock');
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.device_blockContent', {
      label,
      name: row.name || row.deviceId,
    }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    okType: blocked === 1 ? 'danger' : 'primary',
    onOk: async () => {
      await blockClientDeviceApi(row.deviceId, blocked);
      message.success(
        blocked === 1
          ? $t('client.device_blockSuccess')
          : $t('client.device_unblockSuccess'),
      );
      gridApi.query();
    },
  });
}

function onDelete(row: ClientDeviceApi.Device) {
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.device_deleteContent', {
      name: row.name || row.deviceId,
    }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteClientDeviceApi(row.deviceId);
      message.success($t('client.device_deleted'));
      gridApi.query();
    },
  });
}

// ===== 详情/控制 Drawer =====
const drawerVisible = ref(false);
const drawerLoading = ref(false);
const device = ref<ClientDeviceApi.Device | null>(null);
// ===== MCP 分组绑定抽屉（共享组件，设备=直绑，后台自动签 key + 尝试 restart） =====
const mcpVisible = ref(false);
const mcpDeviceName = ref('');
function openMcpManage() {
  if (!device.value) return;
  mcpDeviceName.value = device.value.name || device.value.deviceId;
  mcpVisible.value = true;
}
const statuses = ref<ClientDeviceApi.AgentStatus[]>([]);
const commandedAgents = ref<string[]>([]);
const commands = ref<ClientDeviceApi.Command[]>([]);
const configChanges = ref<ClientDeviceApi.ConfigChange[]>([]);
const logs = ref<ClientDeviceApi.Log[]>([]);

// ===== App 权限管理（设备级勾选；App 目录在 App 权限管理 页全局维护，mac TCC 专属） =====
const appPermApps = ref<any[]>([]);
const appPermSelected = ref<string[]>([]);
// 打开抽屉时的服务端清单快照——保存时以它为基线算「本次编辑的增/减」，
// 叠加到重新拉取的最新清单上，避免旧快照整组覆盖吞掉编辑期间的其他保存。
const appPermLoadedSelected = ref<string[]>([]);
// 客户端上报的授权状态（bundleId → {granted, denied}），驱动卡片徽标与「已授权 n/m」
const appPermStates = ref<Record<string, { denied: boolean; granted: boolean }>>({});
const appPermKeyword = ref('');
const appPermSaving = ref(false);
const appPermLoading = ref(false);
// 真实图标 public/app-logos/<bundleId>.png；加载失败回退 emoji（logo 字段/📦）
const appPermLogoErr = reactive<Record<string, boolean>>({});

// App 权限管理仅对启用 openclaw 的设备生效：openclaw 桌面控制（node 协议）与
// aa-desktop MCP 工具都过客户端 app_gate 闸门、受此清单约束；hermes 自带
// terminal/write_file 等工具直接跑在用户账户下、不经闸门，清单约束不了它。
// 未启用 openclaw 时整块置灰（启用后恢复可配），与后端保存拦截同口径。
const appPermDisabled = computed(() => !commandedAgents.value.includes('openclaw'));
const hasHermesAgent = computed(() => commandedAgents.value.includes('hermes'));
const appPermFiltered = computed(() => {
  const k = appPermKeyword.value.trim().toLowerCase();
  if (!k) return appPermApps.value;
  return appPermApps.value.filter(
    (a: any) =>
      String(a.appName || '').toLowerCase().includes(k) ||
      String(a.bundleId || '').toLowerCase().includes(k),
  );
});
// 按类别分组展示（固定顺序 comm→docs→tools→browser→ai→system；空/未知类别兜底「其他」排最后）
const appPermGroups = computed(() => {
  const map = new Map<string, any[]>();
  for (const a of appPermFiltered.value) {
    const c = APP_PERM_CATS.includes(a?.category) ? a.category : '';
    if (!map.has(c)) map.set(c, []);
    map.get(c)!.push(a);
  }
  return [...APP_PERM_CATS, '']
    .filter((c) => (map.get(c)?.length ?? 0) > 0)
    .map((c) => ({
      key: c,
      label: c
        ? $t(`client.appperm_cat_${c}`)
        : $t('client.appperm_cat_other'),
      apps: map.get(c)!,
    }));
});
// 目录进程内缓存（每次开 drawer 只拉一次），设备已选清单随 drawer 拉取
async function loadAppPerms(deviceId: string) {
  if (!deviceId) return;
  appPermLoading.value = true;
  try {
    const listReq =
      appPermApps.value.length > 0
        ? Promise.resolve(appPermApps.value)
        : getClientAppListApi().then((r) => (r?.list ?? []) as any[]);
    const [apps, sel] = await Promise.all([
      listReq,
      getDeviceAppPermsApi(deviceId),
    ]);
    appPermApps.value = apps;
    appPermSelected.value = (sel?.bundleIds ?? []) as string[];
    appPermLoadedSelected.value = [...appPermSelected.value];
    const states: Record<string, { denied: boolean; granted: boolean }> = {};
    for (const p of sel?.perms ?? []) states[p.bundleId] = { denied: !!p.denied, granted: !!p.granted };
    appPermStates.value = states;
  } catch {
    appPermSelected.value = [];
    appPermLoadedSelected.value = [];
  } finally {
    appPermLoading.value = false;
  }
}
function toggleAppPerm(bundleId: string) {
  if (appPermDisabled.value) return; // 未启用 openclaw：整块置灰，不可勾选
  const set = new Set(appPermSelected.value);
  if (set.has(bundleId)) {
    set.delete(bundleId);
  } else {
    set.add(bundleId);
  }
  appPermSelected.value = [...set];
}
// 已授权 n/m（n=已授权数，m=勾选总数）
const appPermGrantedCount = computed(
  () =>
    appPermSelected.value.filter((b) => appPermStates.value[b]?.granted)
      .length,
);
// 重新授权（对已拒绝的 App）：下发 appperm_reset——tccutil 全量重置本 App 的自动化
// 授权对（macOS 无按目标 App 粒度），清单内所有 App 的授权框会重新弹一遍。
function regrantAppPerm() {
  if (!device.value?.deviceId) return;
  Modal.confirm({
    title: $t('client.appperm_regrant'),
    content: $t('client.appperm_resetConfirm'),
    okText: $t('client.appperm_confirm'),
    cancelText: $t('client.appperm_cancel'),
    onOk: async () => {
      try {
        await resetDeviceAppPermApi({
          deviceId: device.value!.deviceId,
        });
        message.success($t('client.appperm_resetQueued'));
        // 指令已入队——刷新命令历史可见 appperm_reset
        loadCommands();
      } catch (e: any) {
        message.error(e?.message || $t('client.appperm_resetFailed'));
      }
    },
  });
}
// 刷新：重拉勾选与授权徽标（客户端新授权的 App 点一下立即显示）。
// 编辑中刷新不丢未保存的改动——把本次勾选的增/减（相对上次加载快照）叠加到
// 最新清单上，与保存同一套合并语义；无改动时等同纯重载。
async function refreshAppPerms() {
  if (!device.value?.deviceId) return;
  appPermLoading.value = true;
  try {
    const loaded = new Set(appPermLoadedSelected.value);
    const edited = new Set(appPermSelected.value);
    const added = [...edited].filter((b) => !loaded.has(b));
    const removed = [...loaded].filter((b) => !edited.has(b));
    const [fresh] = await Promise.all([
      getDeviceAppPermsApi(device.value.deviceId),
      // 目录不沿用进程缓存——别的管理员新加的目录 App 刷新后也能看到
      getClientAppListApi().then((r) => {
        appPermApps.value = (r?.list ?? []) as any[];
      }),
    ]);
    const freshIds = (fresh?.bundleIds ?? []) as string[];
    const merged = new Set(freshIds);
    for (const b of added) {
      merged.add(b);
    }
    for (const b of removed) {
      merged.delete(b);
    }
    appPermSelected.value = [...merged];
    appPermLoadedSelected.value = [...freshIds];
    const states: Record<string, { denied: boolean; granted: boolean }> = {};
    for (const p of fresh?.perms ?? []) states[p.bundleId] = { denied: !!p.denied, granted: !!p.granted };
    appPermStates.value = states;
  } catch (e: any) {
    message.error(e?.message || $t('client.appperm_refreshFailed'));
  } finally {
    appPermLoading.value = false;
  }
}
async function saveAppPerms() {
  if (!device.value?.deviceId) return;
  if (appPermDisabled.value) return; // 未启用 openclaw：后端同口径拒绝，前端不发起
  appPermSaving.value = true;
  try {
    // 基于最新快照保存：编辑期间清单可能已被改过（他人保存/本页另开抽屉），直接把
    // 打开时的快照 + 本次勾选整组覆盖会吞掉期间的变化。先重拉最新清单，把「本次
    // 编辑的增/减」（相对打开时快照）叠加到最新清单上再保存。
    const loaded = new Set(appPermLoadedSelected.value);
    const edited = new Set(appPermSelected.value);
    const added = [...edited].filter((b) => !loaded.has(b));
    const removed = [...loaded].filter((b) => !edited.has(b));
    const fresh = await getDeviceAppPermsApi(device.value.deviceId);
    const merged = new Set((fresh?.bundleIds ?? []) as string[]);
    const changedUnderneath =
      [...merged].some((b) => !loaded.has(b)) ||
      [...loaded].some((b) => !merged.has(b));
    for (const b of added) {
      merged.add(b);
    }
    for (const b of removed) {
      merged.delete(b);
    }
    const final = [...merged];
    await saveDeviceAppPermsApi({
      deviceId: device.value.deviceId,
      bundleIds: final,
    });
    if (changedUnderneath) {
      message.info($t('client.appperm_merged'));
    }
    message.success($t('client.appperm_saved'));
    // 保存后重拉：勾选/徽标回到服务端真实状态（新勾选 App 立即显示「待授权」），
    // 下一次编辑也从最新快照开始。
    await loadAppPerms(device.value.deviceId);
    // 保存即写一条 aa_client_config_change（source=app_perm）——顺手刷新，
    // 「配置变更」模块不用重开抽屉就能看到这条 ±diff。
    loadConfigChanges();
  } catch (e: any) {
    message.error(e?.message || $t('client.appperm_saveFailed'));
  } finally {
    appPermSaving.value = false;
  }
}

// ===== 模块卡折叠（运行详情各模块标题左侧 ▸/▾） =====
// 全部模块默认收起；用户点过的展开/折叠状态记入 localStorage（前端记忆），
// 刷新/重进页面后恢复上次选择。未记忆过的模块回落默认收起。
const SEC_COLLAPSE_KEY = 'client-device-secs-collapsed';
const SEC_KEYS = [
  'info', 'agents', 'app', 'appperm', 'status', 'switch',
  'cmd', 'channel', 'skills', 'mcp', 'history', 'logs', 'config',
];
function loadCollapsedSecs(): Record<string, boolean> {
  const state: Record<string, boolean> = {};
  for (const k of SEC_KEYS) state[k] = true;
  try {
    const raw = localStorage.getItem(SEC_COLLAPSE_KEY);
    if (raw) Object.assign(state, JSON.parse(raw) ?? {});
  } catch {
    /* 损坏串/隐私模式读失败：回落全收起默认 */
  }
  return state;
}
const collapsedSecs = reactive<Record<string, boolean>>(loadCollapsedSecs());
function toggleSec(key: string) {
  collapsedSecs[key] = !collapsedSecs[key];
  try {
    localStorage.setItem(SEC_COLLAPSE_KEY, JSON.stringify(collapsedSecs));
  } catch {
    /* 隐私模式写入失败：仅本次会话生效 */
  }
}

// ===== 渠道绑定（微信/钉钉/企微/飞书/Telegram 等） =====
// 可绑定的渠道目录。channel 前缀=agent key（openclaw-*/hermes-*），后端据此派 agent。
// openclaw 走 plugin_install + 配置下发（pluginSpec 给出包名）；hermes 的 messaging extra
// 已随 install 装好，无 pluginSpec。图标沿用 public/client-logos 现有资源——该目录只有
// agent 级（hermes/openclaw/obsidian）素材、无渠道级素材，故统一用通用 emoji 兜底。
const CHANNEL_CATALOG = [
  { channel: 'openclaw-weixin', icon: '💬', name: '微信', pluginSpec: '@tencent-weixin/openclaw-weixin' },
  { channel: 'openclaw-dingtalk', icon: '💬', name: '钉钉' },
  { channel: 'openclaw-wecom', icon: '💬', name: '企业微信' },
  { channel: 'openclaw-feishu', icon: '💬', name: '飞书' },
  { channel: 'openclaw-telegram', icon: '✈️', name: 'Telegram' },
  { channel: 'hermes-weixin', icon: '💬', name: '微信(hermes)' },
  { channel: 'hermes-telegram', icon: '✈️', name: 'Telegram(hermes)' },
];
// 仅展示【当前选中 agent】对应的渠道条目（channel 前缀=agent key）。
// activeAgent 由外层统一切换器驱动，切 agent 即只看该 agent 的渠道。
const visibleChannelCatalog = computed(() =>
  CHANNEL_CATALOG.filter((e) => e.channel.split('-')[0] === activeAgent.value),
);
const channels = ref<ClientDeviceApi.Channel[]>([]);
// 扫码登录态：当前展示 QR 的渠道 + 轮询定时器
const qrChannel = ref<string>('');
const qrPollTimer = ref<number | null>(null);
const channelBusy = ref<string>('');
// 渠道 QR 渲染图：设备回传的 qrDataUrl 是 QR 内容 URL（微信，非 data:image）时，
// 用 qrcode 库生成 data URL 存这；whatsapp 等 data:image 直接用。key=channel。
const qrRendered = ref<Record<string, string>>({});
const qrRenderedSrc = ref<Record<string, string>>({});
function isDataImg(s: string | undefined): boolean {
  return !!s && s.startsWith('data:image');
}
// 为某渠道生成 QR 图。raw 是 QR 内容（URL 或 data:image）。源没变则跳过。
async function generateQr(channel: string, raw: string) {
  if (!raw) {
    delete qrRendered.value[channel];
    delete qrRenderedSrc.value[channel];
    return;
  }
  if (isDataImg(raw)) {
    qrRendered.value[channel] = raw;
    qrRenderedSrc.value[channel] = raw;
    return;
  }
  if (qrRenderedSrc.value[channel] === raw) return;
  try {
    const dataUrl = await QRCode.toDataURL(raw, { width: 240, margin: 1 });
    qrRendered.value[channel] = dataUrl;
    qrRenderedSrc.value[channel] = raw;
  } catch (e) {
    console.error('[channel] QR 生成失败', channel, e);
  }
}
function qrImgSrc(channel: string): string {
  return qrRendered.value[channel] || '';
}
function loginStatusTag(s: string): { color: string; text: string } {
  switch (s) {
    case 'connected': { return { color: 'magenta', text: '已连接' }; }
    case 'qr_pending': { return { color: 'processing', text: '扫码中' }; }
    case 'disconnected': { return { color: 'warning', text: '已断开' }; }
    case 'failed': { return { color: 'error', text: '失败' }; }
    default: { return { color: 'default', text: '未登录' }; }
  }
}
function channelOf(channel: string): ClientDeviceApi.Channel | undefined {
  return channels.value.find((c) => c.channel === channel);
}
async function loadChannels() {
  if (!device.value) return;
  try {
    const res = await getClientDeviceChannelsApi(device.value.deviceId);
    channels.value = res?.list ?? [];
  } catch {
    channels.value = [];
  }
}
async function onToggleChannel(channel: string, checked: boolean) {
  if (!device.value) return;
  channelBusy.value = channel;
  try {
    // 1. 写 channels（启用/禁用）
    await setClientDeviceChannelApi(device.value.deviceId, channel, checked);
    await loadChannels();
    const entry = CHANNEL_CATALOG.find((c) => c.channel === channel);
    const agent = channel.split('-')[0]; // openclaw | hermes
    if (checked && entry) {
      if (entry.pluginSpec) {
        // openclaw：微信插件随 install openclaw 自动装（@tencent-weixin/openclaw-weixin），
        // 启用只需 restart 推配置（channels.*.enabled）+ 加载插件。
        await enqueueClientCommandApi({
          deviceId: device.value.deviceId,
          action: 'restart',
          agent,
          payload: {},
        });
        message.success(`已启用 ${entry.name}，重启加载配置后点「扫码登录」`);
      } else {
        // hermes：messaging extra 已随 install 装好，启用仅需落 channels，直接可扫码。
        // openclaw 无 pluginSpec 的渠道（钉钉/企微/飞书/Telegram）同样只落 channels。
        message.success(`已启用 ${entry.name}，点「扫码登录」绑定`);
      }
    } else if (!checked) {
      // 禁用：openclaw 推一次 restart 让配置（不含渠道）生效；hermes 仅落 channels（解绑走 onChannelUnbind）。
      if (entry?.pluginSpec) {
        await enqueueClientCommandApi({
          deviceId: device.value.deviceId,
          action: 'restart',
          agent,
          payload: {},
        });
      }
      stopQrPoll();
      message.success('已禁用');
    }
  } catch (e: any) {
    message.error(e?.message || '操作失败');
    await loadChannels();
  } finally {
    channelBusy.value = '';
  }
}
async function onChannelLogin(channel: string) {
  if (!device.value) return;
  // 该渠道已在扫码中（qrChannel 命中且轮询活着）→ 不重复下发命令，复用现有轮询。
  // 否则非阻塞 channel_login 会再 spawn 一个 channels login 进程，与上一轮争抢会话。
  if (qrChannel.value === channel && qrPollTimer.value !== null) {
    message.info('该渠道扫码进行中，请在下方扫码');
    return;
  }
  channelBusy.value = channel;
  try {
    await loginClientDeviceChannelApi(device.value.deviceId, channel);
    message.info('已下发扫码登录命令，等待设备回传二维码');
    startQrPoll(channel);
  } catch (e: any) {
    message.error(e?.message || '下发失败');
  } finally {
    channelBusy.value = '';
  }
}
async function onChannelLogout(channel: string) {
  if (!device.value) return;
  channelBusy.value = channel;
  try {
    await logoutClientDeviceChannelApi(device.value.deviceId, channel);
    message.success('已下发登出命令');
    if (qrChannel.value === channel) stopQrPoll();
    // 服务端入队时已同步清 loginStatus（resetChannelLogin）；本地快照不重拉会残留旧
    // connected，把「扫码登录」按钮锁死到手动点刷新为止——与解绑（onChannelUnbind）同构回手刷新。
    await loadChannels();
  } catch (e: any) {
    message.error(e?.message || '下发失败');
  } finally {
    channelBusy.value = '';
  }
}
async function onChannelUnbind(channel: string) {
  if (!device.value) return;
  channelBusy.value = channel;
  try {
    await unbindClientDeviceChannelApi(device.value.deviceId, channel);
    if (qrChannel.value === channel) stopQrPoll();
    await loadChannels();
    message.success('已解绑：清当前账号中，完成后可直接扫码绑定新微信');
  } catch (e: any) {
    message.error(e?.message || '解绑失败');
  } finally {
    channelBusy.value = '';
  }
}
function startQrPoll(channel: string) {
  stopQrPoll();
  // stopQrPoll 会清 qrChannel，这里重新设上——否则模板 v-if="qrChannel===entry.channel" 永不成立，QR 不渲染
  qrChannel.value = channel;
  // 立即先取一次，不用等 2s
  void pollChannelQr(channel);
  qrPollTimer.value = window.setInterval(() => pollChannelQr(channel), 2000);
}
async function pollChannelQr(channel: string) {
  if (!device.value) return;
  try {
    const st = await getClientDeviceChannelQrApi(device.value.deviceId, channel);
    if (!st) return;
    // 合并 QR/状态到本地 channels。设备 channels 列表可能没这条（未绑/未上报），
    // 此时直接用 st 补建——否则 channelOf() 找不到，模板 v-if 永不成立，QR 不渲染。
    const idx = channels.value.findIndex((c) => c.channel === channel);
    if (idx >= 0) channels.value[idx] = { ...channels.value[idx], ...st };
    else channels.value.push({ ...st });
    // 直接生成 QR 图（不靠 watch，避免响应性遗漏）
    if (st.qrDataUrl) {
      void generateQr(channel, st.qrDataUrl);
    } else {
      delete qrRendered.value[channel];
      delete qrRenderedSrc.value[channel];
    }
    if (st.loginStatus === 'connected' || st.loginStatus === 'failed') {
      stopQrPoll();
      if (st.loginStatus === 'connected') message.success('渠道已连接');
      else if (st.loginStatus === 'failed') message.error('登录失败');
    }
  } catch {
    /* 忽略轮询瞬时错误 */
  }
}
function stopQrPoll() {
  if (qrPollTimer.value !== null) {
    clearInterval(qrPollTimer.value);
    qrPollTimer.value = null;
  }
  qrChannel.value = '';
}

// ===== 技能管理（装到设备 openclaw 全局托管目录 ~/.openclaw/skills） =====
// 列表=设备心跳上报的真实快照（含手装技能）；安装=从技能库选一个下发 skill_install；
// 卸载=按 slug 下发 skill_uninstall（设备删托管目录，幂等）。
const deviceSkills = ref<ClientDeviceApi.Skill[]>([]);
// 安装选择器：远程搜索技能库（keyWords 模糊匹配 name/slug），可先选命名空间收窄范围
const skillOptions = ref<SkillApi.ListItem[]>([]);
const skillSearching = ref(false);
const selectedSkillId = ref<string>('');
const skillBusy = ref(false);
// 版本选择（''=最新，服务端按 latest_version_id/最新非 yanked 解析）：选中技能后拉其版本列表，
// 可指定装旧版（=回滚能力）；跳过已 yanked，草稿可装（与 install 解析口径一致）
const skillVersionOptions = ref<SkillApi.VersionItem[]>([]);
const selectedVersionId = ref('');
// 命名空间筛选（''=全部）：取启用中的空间做下拉；切换后按当前条件重搜
const skillNsOptions = ref<SkillNamespaceApi.ListItem[]>([]);
const selectedNsId = ref<string>('');
// 已装列表本地搜索（name/slug/description 包含即命中，大小写不敏感）
const installedSkillQuery = ref('');
const filteredDeviceSkills = computed(() => {
  const q = installedSkillQuery.value.trim().toLowerCase();
  if (!q) return deviceSkills.value;
  return deviceSkills.value.filter(
    (s) =>
      s.name.toLowerCase().includes(q) ||
      s.slug.toLowerCase().includes(q) ||
      (s.description ?? '').toLowerCase().includes(q),
  );
});
// 已装 slug 集：下拉选项标「已安装」；选中已装技能时禁用「安装」（重复安装无意义，重装先卸载）
const installedSlugs = computed(
  () => new Set(deviceSkills.value.map((s) => s.slug)),
);
// 当前选中待装技能（展示介绍详情用）
const selectedSkillDetail = computed(
  () =>
    skillOptions.value.find((s) => s.skillId === selectedSkillId.value) ?? null,
);
const selectedSkillInstalled = computed(() => {
  const sk = selectedSkillDetail.value;
  return !!sk && installedSlugs.value.has(sk.slug);
});
// 安装按钮禁用条件：未选技能禁；已装且用默认「最新」禁（重复装无意义，升级走行内按钮/
// 重装先卸载）；已装但另选了不同版本放行——装指定旧版=回滚能力
const installDisabled = computed(() => {
  if (!selectedSkillId.value) return true;
  if (!selectedSkillInstalled.value) return false;
  if (!selectedVersionId.value) return true;
  const sk = selectedSkillDetail.value;
  const inst = deviceSkills.value.find((s) => s.slug === sk?.slug);
  const pick = skillVersionOptions.value.find(
    (v) => v.versionId === selectedVersionId.value,
  );
  return !!inst && !!pick && inst.version === pick.version;
});
// 已装技能详情弹窗（名称/slug/版本/完整介绍——表格里描述是截断的）
function onSkillDetail(sk: ClientDeviceApi.Skill) {
  const row = (k: string, v: string) =>
    h('p', { style: 'margin:0 0 6px;white-space:pre-wrap;word-break:break-word' }, [
      h('b', null, `${k}：`),
      v,
    ]);
  Modal.info({
    title: `${$t('client.device_skillDetailTitle')}：${sk.name}`,
    width: 560,
    content: h('div', null, [
      row('slug', sk.slug),
      row($t('client.device_version'), sk.version || '—'),
      row($t('client.device_skillIntro'), sk.description || $t('client.device_skillNoDesc')),
    ]),
  });
}
async function loadSkillNamespaces() {
  if (skillNsOptions.value.length > 0) return;
  try {
    const res = await getAllSkillNamespacesApi();
    skillNsOptions.value = (res?.list ?? []).filter((n) => n.status === 1);
  } catch {
    skillNsOptions.value = [];
  }
}
async function loadSkills() {
  if (!device.value) return;
  try {
    const res = await getClientDeviceSkillsApi(device.value.deviceId);
    deviceSkills.value = res?.list ?? [];
  } catch {
    deviceSkills.value = [];
  }
}
async function searchSkills(keywords: string) {
  skillSearching.value = true;
  try {
    const res = await getSkillListApi({
      pageNum: 1,
      pageSize: 50,
      keyWords: keywords || undefined,
      namespaceId: selectedNsId.value || undefined,
    });
    // 只列 ACTIVE 且有 slug 的（slug 是安装目录名，缺失装不了）
    skillOptions.value = (res?.list ?? []).filter(
      (s) => s.status === 1 && !!s.slug,
    );
  } catch {
    skillOptions.value = [];
  } finally {
    skillSearching.value = false;
  }
}
// 切换命名空间后立即重搜（Select 内部搜索词取不到，用空关键字重拉该空间全集）
function onSkillNsChange() {
  selectedSkillId.value = '';
  void searchSkills('');
}
// 选中技能后拉版本列表（供选版本装；列表拉不到不阻断——还可用「最新」）
watch(selectedSkillId, async (id) => {
  selectedVersionId.value = '';
  skillVersionOptions.value = [];
  if (!id) return;
  try {
    const res = await getSkillVersionsApi(id);
    skillVersionOptions.value = (res?.list ?? []).filter((v) => v.status !== 2);
  } catch {
    skillVersionOptions.value = [];
  }
});
async function onInstallSkill() {
  if (!device.value || !selectedSkillId.value) return;
  const sk = skillOptions.value.find((s) => s.skillId === selectedSkillId.value);
  skillBusy.value = true;
  try {
    const res = await installClientDeviceSkillApi(
      device.value.deviceId,
      selectedSkillId.value,
      selectedVersionId.value,
    );
    message.success(`已下发安装命令：${sk?.displayName ?? '技能'}（完成后列表自动刷新）`);
    selectedSkillId.value = '';
    pollCommandCompletion(res.commandId);
  } catch (e: any) {
    message.error(e?.message || '下发安装失败');
  } finally {
    skillBusy.value = false;
  }
}
async function onUninstallSkill(sk: ClientDeviceApi.Skill) {
  if (!device.value) return;
  Modal.confirm({
    title: '卸载技能',
    content: `确定从设备卸载「${sk.name}」（${sk.slug}）？卸载后该技能在本机立即失效。`,
    okText: '卸载',
    okType: 'danger',
    cancelText: '取消',
    onOk: async () => {
      try {
        const res = await uninstallClientDeviceSkillApi(
          device.value!.deviceId,
          sk.slug,
        );
        message.success('已下发卸载命令（完成后列表自动刷新）');
        pollCommandCompletion(res.commandId);
      } catch (e: any) {
        message.error(e?.message || '下发卸载失败');
      }
    },
  });
}
// 行内升级：已装版本 ≠ 库最新（版本是自由文本，不做大小推断）→ 下发 skill_install 带
// 显式 latestVersionId 复用整条安装管线；设备端 install 前挪走旧目录，失败自动挪回。
function skillUpgradable(sk: ClientDeviceApi.Skill) {
  return (
    !!sk.skillId &&
    !!sk.latestVersionId &&
    !!sk.latestVersion &&
    sk.latestVersion !== sk.version
  );
}
function onUpgradeSkill(sk: ClientDeviceApi.Skill) {
  if (!device.value || !skillUpgradable(sk)) return;
  Modal.confirm({
    title: '升级技能',
    content: `确定将「${sk.name}」（${sk.slug}）从 ${sk.version || '—'} 升级到 ${sk.latestVersion}？升级期间该技能短暂不可用；失败将自动保留旧版本。`,
    okText: '升级',
    cancelText: '取消',
    onOk: async () => {
      try {
        const res = await installClientDeviceSkillApi(
          device.value!.deviceId,
          sk.skillId!,
          sk.latestVersionId!,
        );
        message.success('已下发升级命令（完成后列表自动刷新）');
        pollCommandCompletion(res.commandId);
      } catch (e: any) {
        message.error(e?.message || '下发升级失败');
      }
    },
  });
}

// 已装技能列表列（antd-vue 4.x 自定义列走 #bodyCell 分发）
const skillColumns = computed(() => [
  { title: $t('client.device_skillColName'), dataIndex: 'name', key: 'name' },
  { title: $t('client.device_skillColSlug'), dataIndex: 'slug', key: 'slug', width: 180 },
  { title: $t('client.device_version'), dataIndex: 'version', key: 'version', width: 90 },
  { title: $t('client.device_skillColLatest'), key: 'latest', width: 110 },
  { title: $t('client.device_skillColActions'), key: 'actions', width: 176 },
]);

// 命令完成轮询：下发后每 3s 拉一次命令状态 + agent 状态，命令到终态（done/failed/skipped）才停。
// 否则 agent 状态不会从 stopped→running 翻转，「启动」成功后 start 仍可点、
// stop/restart 仍灰的问题就是下发后只在下发那一刻刷了一次状态、之后不刷导致的。
const cmdPollTimer = ref<number | null>(null);
function stopCmdPoll() {
  if (cmdPollTimer.value !== null) {
    clearInterval(cmdPollTimer.value);
    cmdPollTimer.value = null;
  }
}
async function pollCommandCompletion(commandId: string) {
  stopCmdPoll();
  const startedAt = Date.now();
  // 不设固定停止时长：服务端超时清理（cleanupStaleRunning）从最后一次进度上报起算，
  // 卡住的命令转 failed 的时刻没有「自下发起算」的上界——固定时长轮询必然先于服务端
  // 终态自杀（install 卡死实锤：UI 21min 停轮询，服务端 23min 才标 failed，此后无人刷新，
  // 按钮一直禁用到手动重开窗口）。服务端才是权威：有 pending/running 命令时新命令本来
  // 就被拒，按钮禁用与服务端一致，轮询跟着服务端状态走到终态即可。
  // 21min（原固定停止点）后降频 3s→15s：长命令只差终态一跳，无需高频打接口。
  const FAST_INTERVAL = 3000;
  const SLOW_INTERVAL = 15000;
  const SLOW_AFTER = 1260000;
  const isTerminal = (st?: string) =>
    st === 'done' || st === 'failed' || st === 'skipped';
  let slowed = false;
  const tick = async () => {
    try {
      await loadCommands();
      await loadDetail(device.value!.deviceId);
      await loadSkills(); // 技能命令终态后列表随真实快照刷新
    } catch {
      /* ignore 单次失败，下次 tick 再试 */
    }
    const cmd = commands.value.find((c) => c.commandId === commandId);
    // 命令已从列表消失（清理/换设备）且设备无其它执行中命令 → 一并收尾，防幽灵轮询
    if (isTerminal(cmd?.status) || (!cmd && !runningCommand.value)) {
      stopCmdPoll();
      // 终态时补拉一次日志，确保分步日志落齐
      try {
        await loadLogs();
      } catch {
        /* ignore */
      }
      return;
    }
    if (!slowed && Date.now() - startedAt > SLOW_AFTER) {
      slowed = true;
      stopCmdPoll();
      cmdPollTimer.value = window.setInterval(tick, SLOW_INTERVAL);
    }
  };
  await tick(); // 立即跑一次，避免首帧延迟
  cmdPollTimer.value = window.setInterval(tick, FAST_INTERVAL);
}

// 窗口重新可见/获得焦点时补拉一次命令+状态：命令轮询之外的兜底——命令在后台转终态
// 而本页没开轮询时（如轮询期间页面休眠、或命令由他处触发），按钮/标签要等这次补拉才解锁。
let lastFocusRefresh = 0;
const refreshOnFocus = async () => {
  if (!device.value) return;
  const now = Date.now();
  if (now - lastFocusRefresh < 2000) return; // focus+visibilitychange 连发去抖
  lastFocusRefresh = now;
  try {
    await loadCommands();
    await loadDetail(device.value.deviceId);
  } catch {
    /* ignore */
  }
};
const onVisibilityChange = () => {
  if (document.visibilityState === 'visible') void refreshOnFocus();
};
window.addEventListener('focus', refreshOnFocus);
document.addEventListener('visibilitychange', onVisibilityChange);

onBeforeUnmount(() => {
  stopQrPoll();
  stopCmdPoll();
  window.removeEventListener('focus', refreshOnFocus);
  document.removeEventListener('visibilitychange', onVisibilityChange);
});

const statusColumns = computed(() => [
  { title: $t('client.device_agent'), dataIndex: 'agent', width: 140, key: 'agent' },
  { title: $t('client.device_status'), dataIndex: 'status', width: 100, key: 'status' },
  { title: $t('client.device_version'), dataIndex: 'version', width: 130, key: 'version' },
  { title: $t('client.device_reportedAt'), dataIndex: 'reportedAt', width: 160 },
]);

// 客户端运行状态值本地化（与 status 页一致）
function agentStatusLabel(s: string): string {
  switch (s) {
    case 'running': { return $t('client.status_running'); }
    case 'stopped': { return $t('client.status_stopped'); }
    case 'installing': { return $t('client.status_installing'); }
    case 'error': { return $t('client.status_error'); }
    case 'notinstalled': { return $t('client.status_notinstalled'); }
    default: { return s; }
  }
}
function agentStatusColor(s: string): string {
  switch (s) {
    case 'running': { return 'magenta'; }
    case 'installing': { return 'warning'; }
    case 'error': { return 'error'; }
    default: { return 'default'; }
  }
}
// 安装中状态：探测 version 此时尚未就绪，从最近一条 install 命令取正在安装的版本，展示「安装中 vX」
function installingVersionLabel(agent: string): string {
  const cmd = commands.value
    .filter((c) => c.agent === agent && c.action === 'install')
    .sort((a, b) => (b.createdAt ?? '').localeCompare(a.createdAt ?? ''))[0];
  const v = cmd?.version ?? '';
  const vs = v && !v.startsWith('v') ? `v${v}` : v;
  return vs ? `${$t('client.status_installing')} ${vs}` : `${$t('client.status_installing')}…`;
}

// openclaw 环境探测的来源标签（extra.env.found[].origin）
function envOriginLabel(origin: string): string {
  switch (origin) {
    case 'runtime': { return $t('client.device_envRuntime'); }
    case 'npm-global': { return $t('client.device_envNpmGlobal'); }
    case 'app': { return $t('client.device_envApp'); }
    default: { return origin; }
  }
}

const commandColumns = computed(() => [
  { title: $t('client.device_action'), dataIndex: 'action', width: 90 },
  { title: $t('client.device_agent'), dataIndex: 'agent', width: 90 },
  { title: $t('client.device_version'), dataIndex: 'version', width: 100, key: 'version' },
  { title: $t('client.device_status'), dataIndex: 'status', width: 90, key: 'status' },
  { title: $t('client.device_error'), dataIndex: 'error', ellipsis: true, key: 'error' },
  { title: $t('client.device_progress'), dataIndex: 'progress', minWidth: 180, key: 'progress', ellipsis: true },
  { title: $t('client.device_createdAt'), dataIndex: 'createdAt', width: 160 },
]);

const configColumns = computed(() => [
  { title: $t('client.device_agent'), dataIndex: 'agent', width: 90 },
  { title: $t('client.device_configSource'), dataIndex: 'source', width: 110, key: 'source' },
  { title: $t('client.device_configHash'), dataIndex: 'configHash', width: 130, ellipsis: true },
  { title: $t('client.device_size'), dataIndex: 'configSize', width: 70 },
  { title: $t('client.device_changedAt'), dataIndex: 'changedAt', width: 150 },
  { title: $t('client.device_content'), key: 'content', width: 70 },
]);

function commandStatusTag(status: string) {
  if (status === 'done') return { color: 'magenta', text: $t('client.device_cmdDone') };
  if (status === 'pending') return { color: 'default', text: $t('client.device_cmdPending') };
  if (status === 'running') return { color: 'blue', text: $t('client.device_cmdRunning') };
  if (status === 'skipped') return { color: 'default', text: $t('client.device_cmdSkipped') };
  return { color: 'red', text: $t('client.device_cmdFailed') };
}

async function loadDetail(deviceId: string) {
  drawerLoading.value = true;
  try {
    const res = await getClientDeviceDetailApi(deviceId);
    device.value = res?.device ?? null;
    statuses.value = res?.statuses ?? [];
    commandedAgents.value = res?.commandedAgents ?? [];
  } finally {
    drawerLoading.value = false;
  }
}

async function loadCommands() {
  if (!device.value) return;
  const res = await getClientDeviceCommandsApi(device.value.deviceId);
  commands.value = res?.list ?? [];
}

async function loadLogs() {
  if (!device.value) return;
  const res = await getClientDeviceLogsApi(
    device.value.deviceId,
    activeAgent.value,
  );
  logs.value = res?.list ?? [];
}

async function loadConfigChanges() {
  if (!device.value) return;
  const res = await getClientDeviceConfigChangesApi(device.value.deviceId);
  configChanges.value = res?.list ?? [];
}

// ===== 命令分步日志 =====
// 抽屉打开时一次性取该设备最近 200 条日志（不限 agent），按 command_id 分组缓存到 cmdStepLogs。
// 展开指令行直接读缓存——不依赖 ant 的 onExpand 回调是否触发（曾因回调未触发 / 浏览器缓存旧 bundle
// 导致「展开无日志」）。onExpand 仅作兜底：命令较老、落在 200 条窗口外时再单条拉取。
const cmdStepLogs = ref<Record<string, ClientDeviceApi.Log[]>>({});
async function loadStepLogs() {
  if (!device.value) return;
  const res = await getClientDeviceLogsApi(device.value.deviceId, '', 200);
  const grouped: Record<string, ClientDeviceApi.Log[]> = {};
  for (const l of res?.list ?? []) {
    const cid = (l as any).commandId;
    if (!cid) continue;
    (grouped[cid] ??= []).push(l);
  }
  for (const cid of Object.keys(grouped)) {
    grouped[cid]!.sort((a, b) => String(a.ts ?? '').localeCompare(String(b.ts ?? '')));
  }
  cmdStepLogs.value = grouped;
}
// 单条命令分步日志：幂等缓存。undefined=未拉取，[]=已拉取但无日志。
const cmdStepLoading = new Set<string>();
async function loadCommandLogs(commandId: string) {
  if (!commandId) return;
  if (cmdStepLogs.value[commandId] !== undefined) return; // 已缓存（含空数组=确认无日志）
  if (cmdStepLoading.has(commandId)) return; // 拉取中
  cmdStepLoading.add(commandId);
  try {
    const res = await getClientDeviceLogsApi('', '', 200, commandId);
    // 整体替换引用，保证 stepLogsText 的响应式依赖触发重渲染
    cmdStepLogs.value = { ...cmdStepLogs.value, [commandId]: res?.list ?? [] };
  } finally {
    cmdStepLoading.delete(commandId);
  }
}
async function onExpandCommand(expanded: boolean, record: any) {
  // onExpand 回调在本表配置下实测不触发，仅作兜底；真正拉取由 stepLogsText 渲染时驱动。
  if (!expanded || !record?.commandId) return;
  await loadCommandLogs(record.commandId);
}
// 渲染时读缓存；未命中则触发一次拉取（fire-and-forget），到了响应式自动重渲染。
// 关键：挂在 expandedRowRender 插槽的渲染上（插槽必渲染），不依赖 onExpand 回调是否触发——
// 这正是之前「展开新指令无日志」的根因（onExpand 不触发 + eager load 是打开抽屉时的快照，
// 覆盖不到之后才下发的指令）。
function stepLogsText(commandId: string): string {
  const logs = cmdStepLogs.value[commandId];
  if (logs === undefined) {
    loadCommandLogs(commandId);
    return $t('client.device_stepEmpty');
  }
  if (!logs.length) return $t('client.device_stepEmpty');
  return logs.map((l) => `[${l.ts ?? ''}] ${l.line ?? ''}`).join('\n');
}
// progress：jsonb 字符串 {"step":"…","percent":N}。客户端目前不上报 percent（恒 0），故按命令状态
// 给示意百分比——完成/跳过/失败=100%、执行中=50%、待执行=0%。失败=100% 但显示为红色（终止态），
// 原因见「错误」列。若客户端将来上报真实 percent（>0），优先用真实值。step 文本作为进度条 title 保留。
function progressPercent(rec: any): number {
  const p = rec?.progress;
  if (p != null && p !== '' && p !== '{}' && p !== 'null') {
    try {
      const obj = typeof p === 'string' ? JSON.parse(p) : p;
      const pct = Number(obj?.percent) || 0;
      if (pct > 0) return Math.min(100, Math.round(pct));
    } catch {
      /* fallthrough 到状态映射 */
    }
  }
  const m: Record<string, number> = { done: 100, skipped: 100, failed: 100, running: 50, pending: 0 };
  return m[rec?.status] ?? 0;
}
function progressStep(rec: any): string {
  const p = rec?.progress;
  if (p == null || p === '' || p === '{}' || p === 'null') return '';
  try {
    const obj = typeof p === 'string' ? JSON.parse(p) : p;
    return obj?.step ?? '';
  } catch {
    return String(p);
  }
}
function progressColor(status: string): string {
  switch (status) {
    case 'done': return 'ok';
    case 'skipped': return 'skip';
    case 'failed': return 'err';
    case 'running': return 'run';
    default: return 'wait';
  }
}

// ===== App 在线状态（设备级：该设备 App 自身心跳，agent=client / kind=app）=====
// detail 的 statuses（ListLatestByDevice，每 agent 最新一条）含 client 行，
// 单独提为 App 在线状态；agent 状态表格排除它（避免把 App 心跳当 agent 展示）。
const appStatusRow = computed(() =>
  statuses.value.find((r) => r.kind === 'app' || r.agent === 'client') ?? null,
);
const agentStatuses = computed(() =>
  statuses.value.filter((r) => r.kind !== 'app' && r.agent !== 'client'),
);
// App 上下线历史时间线
const appTimelineVisible = ref(false);
const appTimelineRows = ref<ClientDeviceApi.AgentStatus[]>([]);
async function openAppTimeline() {
  if (!device.value) return;
  try {
    const res = await getClientDeviceStatusHistoryApi(device.value.deviceId);
    appTimelineRows.value = (res?.list ?? [])
      .filter((r) => r.kind === 'app' || r.agent === 'client')
      .sort((a, b) => (b.reportedAt ?? '').localeCompare(a.reportedAt ?? ''));
  } catch {
    appTimelineRows.value = [];
  }
  appTimelinePage.value = 1;
  appTimelineVisible.value = true;
}

// ===== 历史数据时间筛选 + 分页（前端做：后端历史接口返回全量） =====
const cmdTimeRange = ref<[Dayjs, Dayjs] | undefined>();
const configTimeRange = ref<[Dayjs, Dayjs] | undefined>();
const appTimeRange = ref<[Dayjs, Dayjs] | undefined>();

// 时间戳是否落在所选区间内（区间为空=不过滤；按日取舍到当天起止）。
function tsInRange(ts: string | undefined, range?: [Dayjs, Dayjs]): boolean {
  if (!range || !range[0] || !range[1]) return true;
  if (!ts) return false;
  const t = dayjs(ts);
  if (!t.isValid()) return false;
  return !t.isBefore(range[0].startOf('day')) && !t.isAfter(range[1].endOf('day'));
}

const filteredCommands = computed(() =>
  commands.value
    .filter((c) => tsInRange(c.createdAt, cmdTimeRange.value))
    .filter((c) => c.agent === activeAgent.value),
);
const filteredConfigChanges = computed(() =>
  configChanges.value
    .filter((c) => tsInRange(c.changedAt, configTimeRange.value))
    // agent=client 的变更（如 App 权限 app_perm）是设备级配置，不属于任何
    // agent 页签——'client' 也不在 Agent 切换器里，按页签过滤会让它永久不可见。
    .filter((c) => c.agent === activeAgent.value || c.agent === 'client'),
);

// App 上下线历史：筛选 + 分页（Timeline 手动分页；改时间区间回到第 1 页）
const appTimelinePage = ref(1);
const appTimelinePageSize = ref(10);
const filteredAppTimelineRows = computed(() =>
  appTimelineRows.value.filter((r) => tsInRange(r.reportedAt, appTimeRange.value)),
);
const pagedAppTimelineRows = computed(() => {
  const start = (appTimelinePage.value - 1) * appTimelinePageSize.value;
  return filteredAppTimelineRows.value.slice(start, start + appTimelinePageSize.value);
});
function onAppTimeRangeChange() {
  appTimelinePage.value = 1;
}

const logText = computed(() =>
  (logs.value ?? [])
    .slice()
    .reverse()
    .map(
      (l) =>
        `[${l.ts ?? ''}] ${l.agent ?? ''} ${l.level ?? ''} ${l.line ?? ''}`,
    )
    .join('\n'),
);

// 配置内容查看弹窗（放大版：元信息头 + 全文滚动）
const configViewVisible = ref(false);
const configViewRow = ref<ClientDeviceApi.ConfigChange | null>(null);
// 尝试 2 空格美化 JSON；解析失败原样展示
const configViewText = computed(() => {
  const raw = configViewRow.value?.content || '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
});
function onViewConfig(row: any) {
  configViewRow.value = row as ClientDeviceApi.ConfigChange;
  configViewVisible.value = true;
}

// ===== 配置变更来源归因展示 =====
// 后端落库时判定（client_service.go attributeConfigChange）：admin_* = 后台命令写入（带操作人），
// llm_model = 差异仅为模型清单，local_change = 无命令关联的本机写入。
const CONFIG_SOURCE_META: Record<string, { color: string; key: string }> = {
  admin_config_push: { color: 'geekblue', key: 'device_srcAdminPush' },
  admin_install: { color: 'geekblue', key: 'device_srcAdminInstall' },
  admin_restart: { color: 'geekblue', key: 'device_srcAdminRestart' },
  admin_start: { color: 'geekblue', key: 'device_srcAdminStart' },
  admin_plugin: { color: 'geekblue', key: 'device_srcAdminPlugin' },
  app_perm: { color: 'geekblue', key: 'device_srcAppPerm' },
  app_perm_applied: { color: 'green', key: 'device_srcAppPermApplied' },
  llm_model: { color: 'purple', key: 'device_srcLlmModel' },
  local_change: { color: 'orange', key: 'device_srcLocal' },
};
function configSourceMeta(source?: string) {
  return CONFIG_SOURCE_META[source || ''] ?? { color: 'default', key: 'device_srcUnknown' };
}

// ===== 快照勾选（最多 2 条）+ 行级 LCS 对比 =====
const selectedChangeIds = ref<string[]>([]);
const diffVisible = ref(false);
const diffOnlyChanged = ref(true);
interface DiffRow {
  type: 'add' | 'del' | 'same';
  aIdx: number; // A 侧行号（1 起在下标换算），-1 表示无
  bIdx: number;
  text: string;
}
const diffRows = ref<DiffRow[]>([]);
const diffMetaA = ref<ClientDeviceApi.ConfigChange | null>(null);
const diffMetaB = ref<ClientDeviceApi.ConfigChange | null>(null);

const configRowSelection = computed(() => ({
  selectedRowKeys: selectedChangeIds.value,
  // 超过 2 条时挤掉最早的勾选，保持「最近两条」语义
  onChange: (keys: any[]) => {
    const ks = keys.map(String);
    selectedChangeIds.value = ks.length > 2 ? ks.slice(ks.length - 2) : ks;
  },
}));

function onCompareConfigs() {
  const rows = configChanges.value.filter((c) =>
    selectedChangeIds.value.includes(c.changeId),
  );
  if (rows.length !== 2) return;
  // A=较早，B=较晚（快照演进方向）；prettyJson 为文件后侧已有实现（声明提升，可直接用）
  const [a, b] = [...rows].sort(
    (x, y) => new Date(x.changedAt).getTime() - new Date(y.changedAt).getTime(),
  ) as [ClientDeviceApi.ConfigChange, ClientDeviceApi.ConfigChange];
  diffMetaA.value = a;
  diffMetaB.value = b;
  diffRows.value = lcsLines(
    prettyJson(a.content).split('\n'),
    prettyJson(b.content).split('\n'),
  );
  diffVisible.value = true;
}

// lcsLines 行级 LCS diff：O(n·m) DP（配置文件几百行，量级安全；超长输入先裁共同前后缀再封顶）。
function lcsLines(a: string[], b: string[]): DiffRow[] {
  // 裁共同前缀
  let pre = 0;
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;
  // 裁共同后缀
  let suf = 0;
  while (
    suf < a.length - pre &&
    suf < b.length - pre &&
    a[a.length - 1 - suf] === b[b.length - 1 - suf]
  )
    suf++;
  const MAX = 4000; // DP 上限：超出截断（实际配置远小于此）
  const ta = a.slice(pre, Math.min(a.length - suf, pre + MAX));
  const tb = b.slice(pre, Math.min(b.length - suf, pre + MAX));

  const n = ta.length;
  const m = tb.length;
  const dp: Uint32Array[] = Array.from({ length: n + 1 }, () =>
    new Uint32Array(m + 1),
  );
  for (let i = n - 1; i >= 0; i--) {
    const cur = dp[i]!;
    const next = dp[i + 1]!;
    for (let j = m - 1; j >= 0; j--) {
      cur[j] =
        ta[i] === tb[j] ? next[j + 1]! + 1 : Math.max(next[j]!, cur[j + 1]!);
    }
  }
  const mid: DiffRow[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (ta[i] === tb[j]) {
      mid.push({ type: 'same', aIdx: pre + i, bIdx: pre + j, text: ta[i]! });
      i++;
      j++;
    } else if (dp[i + 1]![j]! >= dp[i]![j + 1]!) {
      mid.push({ type: 'del', aIdx: pre + i, bIdx: -1, text: ta[i]! });
      i++;
    } else {
      mid.push({ type: 'add', aIdx: -1, bIdx: pre + j, text: tb[j]! });
      j++;
    }
  }
  while (i < n) mid.push({ type: 'del', aIdx: pre + i, bIdx: -1, text: ta[i++]! });
  while (j < m) mid.push({ type: 'add', aIdx: -1, bIdx: pre + j, text: tb[j++]! });

  // 前后缀作为 same 行补回（保证行号连续）
  const head: DiffRow[] = a
    .slice(0, pre)
    .map((t, k) => ({ type: 'same' as const, aIdx: k, bIdx: k, text: t }));
  const tail: DiffRow[] = a
    .slice(a.length - suf)
    .map((t, k) => ({
      type: 'same' as const,
      aIdx: a.length - suf + k,
      bIdx: b.length - suf + k,
      text: t,
    }));
  return [...head, ...mid, ...tail];
}

// 仅看变更行：保留 add/del 及其上下各 2 行上下文
const diffVisibleRows = computed(() => {
  const rows = diffRows.value;
  if (!diffOnlyChanged.value) return rows;
  const keep = new Set<number>();
  rows.forEach((r, k) => {
    if (r.type === 'same') return;
    for (let x = Math.max(0, k - 2); x <= Math.min(rows.length - 1, k + 2); x++)
      keep.add(x);
  });
  return rows.filter((_, k) => keep.has(k));
});
const diffAddCount = computed(
  () => diffRows.value.filter((r) => r.type === 'add').length,
);
const diffDelCount = computed(
  () => diffRows.value.filter((r) => r.type === 'del').length,
);
const diffIsIdentical = computed(() => diffRows.value.every((r) => r.type === 'same'));

// ===== 手动下发配置弹窗（点下发指令动作行「配置」按钮弹出，与 启动 等按钮同尺寸） =====
const configModalVisible = ref(false);
const configModalAgent = ref('');
const configModalContent = computed({
  get: () => cmdState[configModalAgent.value]?.configContent ?? '',
  set: (v: string) => {
    if (configModalAgent.value) cmdState[configModalAgent.value]!.configContent = v;
  },
});
const configModalTitle = computed(() =>
  $t('client.device_configModalTitle', { agent: configModalAgent.value }),
);
function openConfigModal(agent: string) {
  configModalAgent.value = agent;
  configModalVisible.value = true;
}
// 编辑框内容 2 空格美化（非法 JSON 原样保留并提示）
function formatConfigModal() {
  const cur = configModalContent.value;
  try {
    configModalContent.value = JSON.stringify(JSON.parse(cur), null, 2);
  } catch {
    message.warning($t('client.device_configFormatInvalid'));
  }
}
async function onConfigPush() {
  const agent = configModalAgent.value;
  if (!agent) return;
  configModalVisible.value = false;
  await sendCommand(agent, 'config_push');
}

// ===== 下发指令（Tab 切换 agent + 操作按钮） =====
const CLIENT_TABS = [
  { key: 'openclaw', label: 'openclaw' },
  { key: 'hermes', label: 'hermes' },
  { key: 'obsidian', label: 'obsidian' },
];

// 选中的 agent → 下发指令 tab 只展示这些（替代旧的「下过命令的」）
const commandedTabs = computed(() => {
  const nameOf = (k: string) => agentOptions.value.find((a) => a.value === k)?.label ?? k;
  return CLIENT_TABS.filter((c) => commandedAgents.value.includes(c.key))
    .map((c) => ({ key: c.key, label: nameOf(c.key) }));
});

// activeAgent：外层统一 agent 切换器的唯一真实源。5 个 section（下发指令/渠道绑定/指令历史/运行日志/配置变更）
// 全部绑它，切 agent 后联动切到对应 agent 视图。
const activeAgent = ref('openclaw');
// Segmented 选项：本设备选中的 agents（与原下发指令 Tabs 同源 commandedTabs）。
// 每个 option 的 label 用 VNode 渲染：Logo + 名字。
const agentTabOptions = computed(() =>
  commandedTabs.value.map((c) => {
    const logo = agentMeta.value[c.key]?.logo;
    return {
      value: c.key,
      label: h('span', { class: 'seg-agent' }, [
        logo ? h('img', { src: logo, class: 'seg-agent-logo', alt: c.label }) : null,
        c.label,
      ]),
    };
  }),
);
// 本设备使用 Agent 复选框选项：Logo + 名字（VNode label）。
// agentOptions 保持纯字符串（nameOf 等处用），这里单独派生带 logo 的选项。
const agentCheckOptions = computed(() =>
  agentOptions.value.map((o) => {
    const logo = agentMeta.value[o.value]?.logo;
    return {
      value: o.value,
      label: h('span', { class: 'chk-agent' }, [
        logo ? h('img', { src: logo, class: 'chk-agent-logo', alt: o.label }) : null,
        o.label,
      ]),
    };
  }),
);
// activeAgent 是否为「已勾选」的 agent：未勾选任何 agent 时为 false。
// 下发指令整块据此门禁——只有勾选了的 agent 才能切、能操作；否则命令虽能下发，
// 但后端心跳按勾选集过滤（client_service.go Heartbeat），未勾选 agent 的进度/日志会被丢弃，
// 造成「指令历史有命令、却无进度/步骤日志/运行日志」。
const activeAgentSelected = computed(
  () => !!activeAgent.value && commandedAgents.value.includes(activeAgent.value),
);
// 运行日志服务端按 agent 过滤，切 agent 需重拉；其余 section 前端 computed 自动响应。
watch(activeAgent, () => {
  loadLogs();
});

// 各 agent 的命令输入态：版本（仅 install 用）、配置内容（config_push 用）
const cmdState = reactive<Record<string, { version: string; configContent: string }>>({
  openclaw: { version: '', configContent: '{}' },
  hermes: { version: '', configContent: '{}' },
  obsidian: { version: '', configContent: '{}' },
});

// agent 版本目录：agent → { versions, defaultVersion }
const agentMap = ref<Record<string, { versions: { label: string; value: string }[]; defaultVersion: string }>>({});
// 全部 agent 列表（供「本设备使用 agent」多选框选项）：值=key，标签=name
const agentOptions = ref<{ label: string; value: string }[]>([]);
// agent 元信息：key → { name, logo }（供运行状态表/Agent 切换器展示 Logo+名字）
const agentMeta = ref<Record<string, { name: string; logo: string }>>({});

async function loadAgents() {
  try {
    const res = await getClientAgentListApi();
    const map: Record<string, { versions: { label: string; value: string }[]; defaultVersion: string }> = {};
    const opts: { label: string; value: string }[] = [];
    const meta: Record<string, { name: string; logo: string }> = {};
    for (const a of res?.list ?? []) {
      if (!a.enabled) continue;
      const vs = (a.versions ?? []).filter((v) => v.enabled).map((v) => ({ label: v.version, value: v.version }));
      const def = (a.versions ?? []).find((v) => v.isDefault);
      map[a.key] = { versions: vs, defaultVersion: def?.version ?? vs[0]?.value ?? '' };
      opts.push({ label: a.name || a.key, value: a.key });
      meta[a.key] = { name: a.name || a.key, logo: a.logo || '' };
    }
    agentMap.value = map;
    agentOptions.value = opts;
    agentMeta.value = meta;
  } catch {
    agentMap.value = {};
    agentOptions.value = [];
    agentMeta.value = {};
  }
}

// 保存选中的 agent（多选框 change 触发）
const selectedAgentsSaving = ref(false);
async function onSaveSelectedAgents(agents: string[]) {
  if (!device.value) return;
  selectedAgentsSaving.value = true;
  try {
    await setClientDeviceSelectedAgentsApi(device.value.deviceId, agents);
    commandedAgents.value = agents;
    // 选中的第一个 agent 作为当前 tab（若当前 tab 被取消选择）
    if (agents.length && !agents.includes(activeAgent.value)) {
      activeAgent.value = agents[0]!;
    }
    message.success($t('client.device_selectedAgentsSaved'));
  } catch {
    message.error($t('client.device_selectedAgentsSaveFail'));
  } finally {
    selectedAgentsSaving.value = false;
  }
}

// 取某 agent 在本设备的当前状态
function agentStatusOf(agent?: string): string {
  if (!agent) return '';
  return statuses.value.find((s) => s.agent === agent)?.status ?? '';
}

// 设备级串行：服务端 AdminEnqueueCommand 在该设备有 pending/running 命令时拒绝新命令
// （"有命令执行中"）。前端据此禁用整组动作按钮——既防同一条命令被重复点（如安装中再点安装），
// 也防跨 agent 冲突下发被服务端拒。命令到终态（done/failed/skipped）即解禁：失败后可重试。
const runningCommand = computed<ClientDeviceApi.Command | null>(() =>
  commands.value.find((c) => c.status === 'pending' || c.status === 'running') ??
    null,
);

// 按 agent 状态筛可用操作：未装只能装；已装未跑能启动/卸载/配置；跑着能停/重启/卸载/配置
// 没有状态记录（从未下过命令的 agent）视为 notinstalled，让「安装」可用
function allowedActions(agent?: string): string[] {
  const allow: Record<string, string[]> = {
    notinstalled: ['install'],
    stopped: ['start', 'uninstall', 'config_push'],
    running: ['stop', 'restart', 'uninstall', 'config_push'],
    installing: [],
    error: ['stop', 'restart', 'uninstall', 'config_push'],
  };
  const st = agentStatusOf(agent) || 'notinstalled';
  return allow[st] ?? [];
}

function isActionAllowed(agent: string, action: string): boolean {
  // 设备有执行中命令 → 全部动作禁用（串行约束）。状态门禁（allowedActions）作为二级兜底。
  if (runningCommand.value) return false;
  return allowedActions(agent).includes(action);
}

function versionOptionsFor(agent?: string) {
  return agentMap.value[agent ?? '']?.versions ?? [];
}
function defaultVersionFor(agent?: string) {
  return agentMap.value[agent ?? '']?.defaultVersion ?? '';
}

// 操作按钮（config_push 单独配 textarea，不在此列）
const ACTION_BUTTONS = computed(() => [
  { value: 'install', label: $t('client.device_actionInstall'), danger: false, confirm: false },
  { value: 'start', label: $t('client.device_actionStart'), danger: false, confirm: false },
  { value: 'stop', label: $t('client.device_actionStop'), danger: false, confirm: false },
  { value: 'restart', label: $t('client.device_actionRestart'), danger: false, confirm: false },
  { value: 'uninstall', label: $t('client.device_actionUninstall'), danger: true, confirm: true },
]);

// 动作中文化（运行中命令提示用）：复用 ACTION_BUTTONS 标签，config_push 用「配置」
function actionLabel(action: string): string {
  const btn = ACTION_BUTTONS.value.find((b) => b.value === action);
  if (btn) return btn.label;
  if (action === 'config_push') return $t('client.device_actionConfig');
  return action;
}

const cmdSendingAgent = ref('');
const cmdSendingAction = ref('');

async function sendCommand(agent: string, action: string) {
  if (!device.value) return;
  // 前端去重锁：按钮 :disabled 基于 commands 刷新（要等下发往返 + loadCommands 二次请求），
  // 在那 ~1s 窗口内快速双击仍能穿透 → 连发两条（曾出现连续 2 条 restart）。入口同步拦截，
  // 不依赖 DOM 刷新时机；配合后端同 (agent,action) pending 去重双保险。
  if (cmdSendingAgent.value || cmdSendingAction.value) return;
  let payload: Record<string, any> = {};
  let version = '';
  if (action === 'config_push') {
    payload = { configContent: cmdState[agent]?.configContent ?? '{}' };
  } else if (action === 'install') {
    version = cmdState[agent]?.version || defaultVersionFor(agent);
    payload = { version };
  }
  cmdSendingAgent.value = agent;
  cmdSendingAction.value = action;
  try {
    const res = await enqueueClientCommandApi({
      deviceId: device.value.deviceId,
      action,
      agent,
      version,
      payload,
    });
    message.success($t('client.device_commandSent'));
    await loadCommands();
    // 下发后刷新状态（操作可能改变 agent 状态，进而改变可用按钮）
    await loadDetail(device.value.deviceId);
    // 命令异步执行：轮询直到终态，让 agent 状态随命令完成自动翻转、按钮可用性跟着刷新
    if (res?.commandId) void pollCommandCompletion(res.commandId);
  } finally {
    cmdSendingAgent.value = '';
    cmdSendingAction.value = '';
  }
}

function onClickAction(agent: string, action: string) {
  const btn = ACTION_BUTTONS.value.find((b) => b.value === action);
  if (btn?.confirm) {
    Modal.confirm({
      title: $t('client.device_tip'),
      content: $t('client.device_actionConfirm', { action: btn.label, agent }),
      okText: $t('client.device_confirm'),
      cancelText: $t('client.device_cancel'),
      okType: 'danger',
      onOk: () => sendCommand(agent, action),
    });
  } else {
    sendCommand(agent, action);
  }
}

async function openControl(row: ClientDeviceApi.Device) {
  drawerVisible.value = true;
  device.value = { ...row };
  statuses.value = [];
  commands.value = [];
  configChanges.value = [];
  logs.value = [];
  cmdStepLogs.value = {};
  appTimelineRows.value = [];
  channels.value = [];
  deviceSkills.value = [];
  selectedSkillId.value = '';
  skillOptions.value = [];
  skillVersionOptions.value = [];
  selectedVersionId.value = '';
  selectedNsId.value = '';
  installedSkillQuery.value = '';
  appPermSelected.value = [];
  appPermKeyword.value = '';
  stopQrPoll();
  stopCmdPoll();
  // 先加载状态 + agent 版本目录，再初始化各 agent 默认版本
  await Promise.all([
    loadDetail(row.deviceId),
    loadAgents(),
    loadChannels(),
    loadAppPerms(row.deviceId),
    loadSkillNamespaces(),
    loadSkills(),
  ]);
  // 默认选中第一个「已勾选」的 agent；一个都没勾则置空——下发指令整块门禁
  // （v-if activeAgentSelected），强制先在「本设备使用 Agent」勾选才能切/操作，
  // 否则对未勾选 agent 下发的命令，其进度/日志会被后端按勾选集过滤丢弃。
  activeAgent.value = commandedTabs.value[0]?.key ?? '';
  for (const c of CLIENT_TABS) {
    cmdState[c.key].version = defaultVersionFor(c.key);
    cmdState[c.key].configContent = '{}';
  }
  await Promise.all([loadCommands(), loadLogs(), loadConfigChanges(), loadStepLogs()]);
  // 历史配置加载后回填手动下发框：openclaw 走专用预览端点（库中配置主体 + aa_llm_model
  // 最新可用模型，已 2 空格美化）；hermes/obsidian 取 configChanges 最新一条并美化。
  // 美化规则：合法 JSON 一律 JSON.stringify(parse, null, 2)——每个 key/数组项一行，全展示。
  for (const c of CLIENT_TABS) {
    const raw =
      c.key === 'openclaw'
        ? await safeOpenclawPreview(row.deviceId)
        : (configChanges.value.find((r) => r.agent === c.key && r.content)?.content ?? '');
    const pretty = prettyJson(raw);
    if (pretty) cmdState[c.key].configContent = pretty;
  }
}

// openclaw 配置预览端点失败时静默回退到 configChanges 最新一条（不阻断 UI）。
async function safeOpenclawPreview(deviceId: string): Promise<string> {
  try {
    const res = await getClientOpenclawConfigApi(deviceId);
    if (res?.config) return res.config;
  } catch {
    // ignore
  }
  return configChanges.value.find((r) => r.agent === 'openclaw' && r.content)?.content ?? '';
}

// prettyJson 把任意 JSON 文本美化成 2 空格缩进、每个 key/数组项一行的形式；非法/空返回原串。
function prettyJson(s: string): string {
  if (!s || s === '{}') return s;
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

const drawerTitle = computed(() =>
  $t('client.device_controlTitle', {
    name: device.value?.name || device.value?.deviceId || '',
  }),
);

const configViewTitle = computed(() =>
  $t('client.device_configViewTitle', { agent: configViewRow.value?.agent || '' }),
);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <!-- 空 toolbar-actions：vben 的 showToolbar 要求 slot 有内容才渲染 toolbar 区域。
           device 无自定义操作，留空 span 仅触发 toolbar 渲染，让 toolbarConfig.refresh 的
           原生刷新按钮（带箭头圆圈，与 /agent/instance/list 一致）显示出来。 -->
      <template #toolbar-actions><span /></template>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${statusTagOf(row).color}`]">
          {{ statusTagOf(row).text }}
        </span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openControl(row)">
          {{ $t('client.device_detailControl') }}
        </Button>
        <Button
          type="primary" ghost
          size="small"
          :disabled="row.status !== 'active'"
          @click="onRevoke(row)"
        >
          {{ $t('client.device_revokeLogin') }}
        </Button>
        <Button
          v-if="Number(row.blocked) !== 1"
          type="primary" ghost
          size="small"
          danger
          @click="onBlock(row, 1)"
        >
          {{ $t('client.device_block') }}
        </Button>
        <Button
          v-else
          type="primary" ghost
          size="small"
          @click="onBlock(row, 0)"
        >
          {{ $t('client.device_unblock') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDelete(row)">
          {{ $t('client.device_delete') }}
        </Button>
      </template>
    </Grid>

    <Drawer
      v-model:open="drawerVisible"
      :title="drawerTitle"
      :width="960"
      placement="right"
      class="device-drawer"
      @close="() => { stopQrPoll(); stopCmdPoll(); }"
    >
      <div v-if="device" class="drawer-body">
        <Card size="small" :class="['module-card', 'sec-info', { collapsed: collapsedSecs.info }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('info')"><span class="sec-toggle">{{ collapsedSecs.info ? '▸' : '▾' }}</span> {{ $t('client.device_sectionInfo') }}</span></template>
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('client.device_deviceId')">
              {{ device.deviceId }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('client.device_platform')">
              {{ device.platform }} / {{ device.os }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('client.device_mac')">
              {{ device.macAddress }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('client.device_lastLogin')">
              {{ device.lastLoginAt }}（{{ device.lastLoginIp }}）
            </DescriptionsItem>
            <DescriptionsItem :label="$t('client.device_status')">
              <span :class="['aa-enum', `aa-enum--${statusTagOf(device).color}`]">
                {{ statusTagOf(device).text }}
              </span>
            </DescriptionsItem>
          </Descriptions>
        </Card>

        <!-- 本设备使用 agent：多选，选项来自 /client/config 的 agent 列表。
             选中的 agent 才会被 App 心跳上报，下发指令 tab 也只显示这些。 -->
        <Card size="small" :class="['module-card', 'sec-agents', { collapsed: collapsedSecs.agents }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('agents')"><span class="sec-toggle">{{ collapsedSecs.agents ? '▸' : '▾' }}</span> {{ $t('client.device_sectionSelectedAgents') }}</span></template>
          <div class="selected-agents-bar">
            <CheckboxGroup
              :value="commandedAgents"
              :options="agentCheckOptions"
              :disabled="selectedAgentsSaving"
              @change="(v: string[]) => onSaveSelectedAgents(v)"
            />
            <span class="selected-agents-hint">{{ $t('client.device_selectedAgentsHint') }}</span>
          </div>
        </Card>

        <!-- App 在线状态（设备级：本机 App 自身心跳，区别于下方 agent 状态） -->
        <Card size="small" :class="['module-card', 'sec-app', { collapsed: collapsedSecs.app }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('app')"><span class="sec-toggle">{{ collapsedSecs.app ? '▸' : '▾' }}</span> {{ $t('client.device_sectionAppStatus') }}</span></template>
          <div class="app-status-bar">
            <template v-if="appStatusRow">
              <span :class="['aa-enum', `aa-enum--${appStatusRow.status === 'running' ? 'magenta' : 'default'}`]">
                {{ appStatusRow.status === 'running' ? $t('client.status_appRunning') : $t('client.status_appStopped') }}
              </span>
              <span class="app-status-meta">v{{ appStatusRow.version || '?' }} · {{ appStatusRow.reportedAt }}</span>
              <Button type="primary" ghost size="small" class="app-status-history" @click="openAppTimeline">
                {{ $t('client.device_appHistory') }}
              </Button>
            </template>
            <span v-else style="color: #aab2c0">{{ $t('client.device_appNeverOnline') }}</span>
          </div>
        </Card>

        <Card size="small" :class="['module-card', 'sec-appperm', { collapsed: collapsedSecs.appperm }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('appperm')"><span class="sec-toggle">{{ collapsedSecs.appperm ? '▸' : '▾' }}</span> {{ $t('client.device_sectionAppPerm') }}</span></template>
          <Alert
            v-if="appPermDisabled"
            class="appperm-disabled-alert"
            type="warning"
            show-icon
            :message="$t('client.appperm_disabledTitle')"
            :description="$t('client.appperm_disabledHint')"
          />
          <div :class="{ 'appperm-locked': appPermDisabled }">
          <div class="appperm-toolbar">
            <Input
              v-model:value="appPermKeyword"
              :placeholder="$t('client.appperm_keywordPlaceholder')"
              allow-clear
              size="small"
              style="width: 200px"
            />
            <span class="appperm-count">
              {{ $t('client.appperm_selected') }}: {{ appPermSelected.length }}
              <span class="appperm-granted">
                {{
                  $t('client.appperm_grantedCount', {
                    granted: appPermGrantedCount,
                    total: appPermSelected.length,
                  })
                }}
              </span>
            </span>
            <Button
              size="small"
              :loading="appPermLoading"
              style="margin-left: auto"
              @click="refreshAppPerms"
            >
              {{ $t('client.appperm_refresh') }}
            </Button>
            <Button
              type="primary"
              size="small"
              :loading="appPermSaving"
              @click="saveAppPerms"
            >
              {{ $t('client.appperm_saveBtn') }}
            </Button>
          </div>
          <div class="appperm-hint">
            {{ $t('client.appperm_probeHint') }}
          </div>
          <div class="appperm-hint">
            {{ $t('client.appperm_regrantHint') }}
          </div>
          <div class="appperm-hint">
            {{ $t('client.appperm_removeHint') }}
          </div>
          <div class="appperm-hint">
            {{ $t('client.appperm_manualHint') }}
          </div>
          <div v-if="hasHermesAgent && !appPermDisabled" class="appperm-hint appperm-hint-hermes">
            {{ $t('client.appperm_hermesBypassHint') }}
          </div>
          <div
            v-for="g in appPermGroups"
            :key="g.key || 'other'"
            class="appperm-group"
          >
            <div class="appperm-group-title" :class="`appperm-cat-${g.key || 'other'}`">
              {{ g.label }}
              <span class="appperm-group-count">{{ g.apps.length }}</span>
            </div>
            <div class="app-grid">
              <div
                v-for="a in g.apps"
                :key="a.bundleId"
                class="app-card"
                :class="{ checked: appPermSelected.includes(a.bundleId) }"
                @click="toggleAppPerm(a.bundleId)"
              >
                <img
                  v-if="!appPermLogoErr[a.bundleId]"
                  class="app-logo"
                  :src="`/app-logos/${a.bundleId}.png`"
                  :alt="a.appName"
                  @error="appPermLogoErr[a.bundleId] = true"
                />
                <span v-else class="app-logo">{{ a.logo || '📦' }}</span>
                <span class="app-name">{{ a.appName }}</span>
                <span class="app-bundle">{{ a.bundleId }}</span>
                <span
                  v-if="appPermSelected.includes(a.bundleId)"
                  class="app-perm-badge"
                  :class="
                    appPermStates[a.bundleId]?.granted
                      ? 'granted'
                      : appPermStates[a.bundleId]?.denied
                        ? 'denied'
                        : 'pending'
                  "
                >
                  {{
                    appPermStates[a.bundleId]?.granted
                      ? $t('client.appperm_status_granted')
                      : appPermStates[a.bundleId]?.denied
                        ? $t('client.appperm_status_denied')
                        : $t('client.appperm_status_pending')
                  }}
                </span>
                <Button
                  v-if="appPermSelected.includes(a.bundleId) && appPermStates[a.bundleId]?.denied"
                  class="app-regrant"
                  size="small"
                  type="link"
                  danger
                  @click.stop="regrantAppPerm"
                >
                  {{ $t('client.appperm_regrant') }}
                </Button>
                <span class="app-check">
                  {{ appPermSelected.includes(a.bundleId) ? '✓' : '' }}
                </span>
              </div>
            </div>
          </div>
          <div v-if="appPermGroups.length === 0" class="appperm-empty">
            {{ appPermLoading ? '...' : $t('client.appperm_empty') }}
          </div>
          </div>
        </Card>

        <Card size="small" :class="['module-card', 'sec-status', { collapsed: collapsedSecs.status }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('status')"><span class="sec-toggle">{{ collapsedSecs.status ? '▸' : '▾' }}</span> {{ $t('client.device_sectionStatus') }}</span></template>
          <Table
            :columns="statusColumns"
            :data-source="agentStatuses"
            size="small"
            :pagination="false"
            :loading="drawerLoading"
            row-key="statusId"
          >
            <template #emptyText>
              <span style="color: #aab2c0">{{ $t('client.device_statusEmpty') }}</span>
            </template>
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'agent'">
                <span class="agent-cell">
                  <img
                    v-if="agentMeta[record.agent]?.logo"
                    class="agent-logo-cell"
                    :src="agentMeta[record.agent]?.logo"
                    :alt="record.agent"
                  />
                  <span>{{ agentMeta[record.agent]?.name || record.agent }}</span>
                </span>
              </template>
              <template v-else-if="column.key === 'status'">
                <span :class="['aa-enum', `aa-enum--${agentStatusColor(record.status)}`]">
                  {{ agentStatusLabel(record.status) }}
                </span>
              </template>
              <template v-else-if="column.key === 'version'">
                <span v-if="record.status === 'installing'" class="version-installing">
                  {{ installingVersionLabel(record.agent) }}
                </span>
                <span v-else>{{ record.version || '—' }}</span>
                <!-- openclaw 环境探测：本机已有安装且版本待对齐（升/降级需设备本机用户确认） -->
                <Tooltip v-if="record.extra?.env?.aligned === false">
                  <span class="aa-enum aa-enum--orange">{{ $t('client.device_alignPending') }}</span>
                  <template #title>
                    <div v-for="f in record.extra?.env?.found ?? []" :key="f.path">
                      {{ envOriginLabel(f.origin) }}：{{ f.version || '?' }}（{{ f.path }}）
                    </div>
                    <div>
                      {{ $t('client.device_alignTarget') }}：{{ record.extra?.env?.targetVersion }}
                    </div>
                  </template>
                </Tooltip>
              </template>
            </template>
          </Table>
        </Card>

        <Card v-if="agentTabOptions.length" size="small" :class="['module-card', 'agent-switch-card', 'sec-switch', { collapsed: collapsedSecs.switch }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('switch')"><span class="sec-toggle">{{ collapsedSecs.switch ? '▸' : '▾' }}</span> {{ $t('client.device_currentAgent') }}</span></template>
          <Segmented v-model:value="activeAgent" :options="agentTabOptions" />
          <span class="section-hint">{{ $t('client.device_agentSwitchHint') }}</span>
        </Card>

        <Card size="small" :class="['module-card', 'sec-cmd', { collapsed: collapsedSecs.cmd }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('cmd')"><span class="sec-toggle">{{ collapsedSecs.cmd ? '▸' : '▾' }}</span> {{ $t('client.device_sectionCommand') }}</span></template>
          <div v-if="activeAgentSelected" class="agent-ctrl">
            <div class="agent-line">
              <span class="agent-label">{{ $t('client.device_status') }}：</span>
              <span :class="['aa-enum', `aa-enum--${agentStatusColor(agentStatusOf(activeAgent))}`]">
                {{ agentStatusLabel(agentStatusOf(activeAgent)) || $t('client.status_notinstalled') }}
              </span>
            </div>

            <!-- 设备有执行中命令：展示在跑什么，整组动作按钮禁用（服务端设备级串行，防重复/冲突下发） -->
            <div v-if="runningCommand" class="agent-line">
              <span class="agent-label">{{ $t('client.device_runningCommand') }}：</span>
              <span class="aa-enum aa-enum--blue">
                {{ agentMeta[runningCommand.agent]?.name || runningCommand.agent }} ·
                {{ actionLabel(runningCommand.action) }}（{{ commandStatusTag(runningCommand.status).text }}）
              </span>
            </div>

            <!-- 版本选择：仅未安装时（安装用；无状态记录视为未安装） -->
            <div v-if="(agentStatusOf(activeAgent) || 'notinstalled') === 'notinstalled'" class="agent-line">
              <span class="agent-label">{{ $t('client.device_version') }}：</span>
              <Select
                v-model:value="cmdState[activeAgent].version"
                style="width: 240px"
                :options="versionOptionsFor(activeAgent)"
                :placeholder="$t('client.device_version')"
              />
            </div>

            <!-- 操作按钮（按 agent 当前状态启用/禁用）。「配置」点击弹出配置信息弹窗（查看/编辑后下发）。 -->
            <div class="agent-btn-row">
              <Button
                v-for="b in ACTION_BUTTONS"
                :key="b.value"
                :type="b.value === 'install' ? 'primary' : 'default'"
                :danger="b.danger"
                :disabled="!isActionAllowed(activeAgent, b.value) || !!cmdSendingAgent"
                :loading="cmdSendingAgent === activeAgent && cmdSendingAction === b.value"
                @click="onClickAction(activeAgent, b.value)"
              >
                {{ b.label }}
              </Button>
              <Button
                v-if="isActionAllowed(activeAgent, 'config_push')"
                :loading="cmdSendingAgent === activeAgent && cmdSendingAction === 'config_push'"
                @click="openConfigModal(activeAgent)"
              >
                {{ $t('client.device_actionConfig') }}
              </Button>
            </div>
          </div>
          <div v-else class="agent-ctrl-empty">
            <span class="section-hint">{{ $t('client.device_noAgentSelectedHint') }}</span>
          </div>
        </Card>

        <Card size="small" :class="['module-card', 'sec-channel', { collapsed: collapsedSecs.channel }]" :bordered="true">
          <template #title>
            <span class="sec-title" @click="toggleSec('channel')">
              <span class="sec-toggle">{{ collapsedSecs.channel ? '▸' : '▾' }}</span> 渠道绑定
              <span class="section-hint">微信等 chat 渠道：勾选启用后点「扫码登录」绑定（随上方当前 Agent 切换）</span>
            </span>
          </template>
          <div class="channel-list">
            <div
              v-for="entry in visibleChannelCatalog"
              :key="entry.channel"
              class="channel-row"
            >
              <div class="channel-head">
                <Checkbox
                  :checked="!!channelOf(entry.channel)?.enabled"
                  :disabled="channelBusy === entry.channel"
                  @change="(e: any) => onToggleChannel(entry.channel, e.target.checked)"
                >
                  <span v-if="entry.icon" class="channel-icon">{{ entry.icon }}</span>
                  <span class="channel-name">{{ entry.name }}</span>
                  <span class="channel-id">{{ entry.channel }}</span>
                </Checkbox>
                <span
                  v-if="channelOf(entry.channel)"
                  :class="['aa-enum', `aa-enum--${loginStatusTag(channelOf(entry.channel)!.loginStatus).color}`]"
                >
                  {{ loginStatusTag(channelOf(entry.channel)!.loginStatus).text }}
                </span>
                <span v-if="channelOf(entry.channel)?.accountAlias" class="channel-alias">
                  {{ channelOf(entry.channel)?.accountAlias }}
                </span>
              </div>
              <div v-if="channelOf(entry.channel)?.enabled" class="channel-actions">
                <Button
                  size="small"
                  type="primary"
                  :loading="channelBusy === entry.channel"
                  :disabled="channelOf(entry.channel)?.loginStatus === 'connected'"
                  @click="onChannelLogin(entry.channel)"
                >
                  扫码登录
                </Button>
                <Button
                  size="small"
                  :disabled="channelOf(entry.channel)?.loginStatus !== 'connected'"
                  @click="onChannelLogout(entry.channel)"
                >
                  登出
                </Button>
                <Button
                  size="small"
                  danger
                  :loading="channelBusy === entry.channel"
                  @click="onChannelUnbind(entry.channel)"
                >
                  解绑
                </Button>
                <Button size="small" @click="loadChannels">刷新</Button>
              </div>
              <!-- 管理后台直接扫码：设备回传的 QR -->
              <div
                v-if="qrChannel === entry.channel && channelOf(entry.channel)?.qrDataUrl"
                class="channel-qr"
              >
                <img
                  v-if="qrImgSrc(entry.channel)"
                  :src="qrImgSrc(entry.channel)"
                  alt="二维码"
                  class="qr-img"
                />
                <div v-else class="qr-loading">二维码生成中…</div>
                <div class="qr-tip">用手机{{ entry.name }}扫码登录（或设备端 App 本地扫码）</div>
                <details class="qr-url-details">
                  <summary>二维码内容 URL（图未显示时可用其它工具扫码）</summary>
                  <code>{{ channelOf(entry.channel)?.qrDataUrl }}</code>
                </details>
              </div>
              <div
                v-if="channelOf(entry.channel)?.lastMessage"
                class="channel-msg"
              >
                {{ channelOf(entry.channel)?.lastMessage }}
              </div>
            </div>
          </div>
        </Card>

        <!-- 技能管理：装/卸设备上的 skill（openclaw 全局托管目录），列表=心跳上报真实快照 -->
        <Card size="small" :class="['module-card', 'sec-skills', { collapsed: collapsedSecs.skills }]" :bordered="true">
          <template #title>
            <span class="sec-title" @click="toggleSec('skills')">
              <span class="sec-toggle">{{ collapsedSecs.skills ? '▸' : '▾' }}</span> {{ $t('client.device_sectionSkills') }}
              <span class="section-hint">{{ $t('client.device_skillHint') }}</span>
            </span>
          </template>
          <!-- 安装行：命名空间收窄 + 远程搜索技能库 -->
          <div class="skill-install-row">
            <Select
              v-model:value="selectedNsId"
              :placeholder="$t('client.device_skillNsPh')"
              class="skill-ns-select"
              @change="onSkillNsChange"
            >
              <Select.Option value="">{{ $t('client.device_skillNsAll') }}</Select.Option>
              <Select.Option v-for="n in skillNsOptions" :key="n.namespaceId" :value="n.namespaceId">
                {{ n.displayName }}
              </Select.Option>
            </Select>
            <Select
              v-model:value="selectedSkillId"
              show-search
              allow-clear
              :filter-option="false"
              :placeholder="$t('client.device_skillPickPh')"
              :loading="skillSearching"
              class="skill-select"
              @search="searchSkills"
              @focus="searchSkills('')"
            >
              <Select.Option v-for="s in skillOptions" :key="s.skillId" :value="s.skillId">
                {{ s.displayName }}（{{ s.slug }}）
                <span v-if="installedSlugs.has(s.slug)" class="skill-opt-installed">
                  {{ $t('client.device_skillInstalledTag') }}
                </span>
              </Select.Option>
            </Select>
            <!-- 版本选择：默认最新（服务端解析），可指定旧版（=回滚） -->
            <Select
              v-if="selectedSkillId"
              v-model:value="selectedVersionId"
              class="skill-ver-select"
            >
              <Select.Option value="">{{ $t('client.device_skillVerLatest') }}</Select.Option>
              <Select.Option
                v-for="v in skillVersionOptions"
                :key="v.versionId"
                :value="v.versionId"
                :disabled="(v.auditStatus ?? 0) !== 1"
              >
                {{ v.version }}{{ v.status === 0 ? $t('client.device_skillVerDraft') : '' }}
                <!-- 安全门禁：审计非「安全」禁选（后端下发/取包同拦） -->
                <span v-if="(v.auditStatus ?? 0) !== 1" class="skill-ver-unsafe">
                  {{ $t('skill.sec_statusFail') }}
                </span>
              </Select.Option>
            </Select>
            <Button
              size="small"
              type="primary"
              ghost
              :disabled="installDisabled"
              :loading="skillBusy"
              @click="onInstallSkill"
            >
              {{ $t('client.device_skillInstall') }}
            </Button>
            <Button size="small" @click="loadSkills">
              {{ $t('client.device_refresh') }}
            </Button>
          </div>
          <!-- 选中待装技能的介绍详情 -->
          <div v-if="selectedSkillDetail" class="skill-detail-box">
            <div class="skill-detail-name">
              {{ selectedSkillDetail.displayName }}
              <span class="skill-detail-slug">{{ selectedSkillDetail.slug }}</span>
            </div>
            <div class="skill-detail-desc">
              {{ selectedSkillDetail.summary || $t('client.device_skillNoDesc') }}
            </div>
            <div v-if="selectedSkillInstalled && installDisabled" class="skill-already-hint">
              {{ $t('client.device_skillAlreadyInstalled') }}
            </div>
          </div>
          <!-- 已装列表（设备真实快照，本地搜索过滤） -->
          <div class="skill-search-row">
            <Input
              v-model:value="installedSkillQuery"
              allow-clear
              :placeholder="$t('client.device_skillSearchPh')"
              class="skill-search-input"
            />
          </div>
          <Table
            size="small"
            :columns="skillColumns"
            :data-source="filteredDeviceSkills"
            row-key="slug"
            :pagination="{ pageSize: 10, showSizeChanger: false, size: 'small' }"
            :locale="{ emptyText: $t('client.device_skillEmpty') }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="skill-name">{{ record.name }}</div>
                <div v-if="record.description" class="skill-desc" :title="record.description">
                  {{ record.description }}
                </div>
              </template>
              <template v-else-if="column.key === 'version'">{{ record.version || '—' }}</template>
              <template v-else-if="column.key === 'latest'">
                <span
                  v-if="skillUpgradable(record as ClientDeviceApi.Skill)"
                  class="skill-latest-new"
                >
                  {{ record.latestVersion }}
                </span>
                <template v-else>{{ record.latestVersion || '—' }}</template>
              </template>
              <template v-else-if="column.key === 'actions'">
                <Button
                  size="small"
                  type="link"
                  class="skill-detail-btn"
                  @click="onSkillDetail(record as ClientDeviceApi.Skill)"
                >
                  {{ $t('client.device_skillDetail') }}
                </Button>
                <Button
                  v-if="skillUpgradable(record as ClientDeviceApi.Skill)"
                  size="small"
                  type="link"
                  class="skill-upgrade-btn"
                  @click="onUpgradeSkill(record as ClientDeviceApi.Skill)"
                >
                  {{ $t('client.device_skillUpgrade') }}
                </Button>
                <Button
                  size="small"
                  danger
                  @click="onUninstallSkill(record as ClientDeviceApi.Skill)"
                >
                  {{ $t('client.device_skillUninstall') }}
                </Button>
              </template>
            </template>
          </Table>
        </Card>

        <!-- MCP 分组：绑定后自动签专属密钥并注入 openclaw 配置（restart 生效），入口弹共享绑定抽屉 -->
        <Card size="small" :class="['module-card', 'sec-mcp', { collapsed: collapsedSecs.mcp }]" :bordered="true">
          <template #title>
            <span class="sec-title" @click="toggleSec('mcp')">
              <span class="sec-toggle">{{ collapsedSecs.mcp ? '▸' : '▾' }}</span> {{ $t('client.device_sectionMcp') }}
              <span class="section-hint">{{ $t('client.device_mcpHint') }}</span>
            </span>
          </template>
          <div class="mcp-manage-row">
            <Button size="small" type="primary" ghost @click="openMcpManage">
              {{ $t('client.device_mcpManage') }}
            </Button>
            <span class="mcp-manage-tip">{{ $t('client.device_mcpManageTip') }}</span>
          </div>
        </Card>

        <Card size="small" :class="['module-card', 'sec-history', { collapsed: collapsedSecs.history }]" :bordered="true">
          <template #title>
            <span class="sec-title" @click="toggleSec('history')">
              <span class="sec-toggle">{{ collapsedSecs.history ? '▸' : '▾' }}</span> {{ $t('client.device_sectionCommandHistory') }}
              <span class="section-hint">{{ $t('client.device_commandHistoryHint') }}</span>
            </span>
          </template>
          <div class="history-toolbar">
            <RangePicker
              v-model:value="cmdTimeRange"
              show-time
              format="YYYY-MM-DD HH:mm:ss"
              :placeholder="[$t('client.device_timeStart'), $t('client.device_timeEnd')]"
            />
            <Button size="small" @click="cmdTimeRange = undefined">
              {{ $t('client.device_reset') }}
            </Button>
            <Button size="small" @click="async () => { await loadCommands(); await loadStepLogs(); }">
              {{ $t('client.device_refreshHistory') }}
            </Button>
          </div>
          <Table
          :columns="commandColumns"
          :data-source="filteredCommands"
          size="small"
          :pagination="{ pageSize: 10, showSizeChanger: true, size: 'small' }"
          row-key="commandId"
          :expandable="{ onExpand: onExpandCommand }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <span :class="['aa-enum', `aa-enum--${commandStatusTag(record.status).color}`]">
                {{ commandStatusTag(record.status).text }}
              </span>
            </template>
            <template v-else-if="column.key === 'version'">{{ record.version || '—' }}</template>
            <template v-else-if="column.key === 'error'">
              <span v-if="record.error" class="cmd-error" :title="record.error">{{ record.error }}</span>
              <span v-else>—</span>
            </template>
            <template v-else-if="column.key === 'progress'">
              <div
                class="cmd-progress"
                :class="`cmd-prog--${progressColor(record.status)}`"
                :title="progressStep(record) || undefined"
              >
                <div class="cmd-prog-bar">
                  <div class="cmd-prog-fill" :style="{ width: `${progressPercent(record)}%` }"></div>
                </div>
                <span class="cmd-prog-text">{{ progressPercent(record) }}%</span>
              </div>
            </template>
          </template>
          <template #expandedRowRender="{ record }">
            <pre class="step-box">{{ stepLogsText(record.commandId) }}</pre>
          </template>
        </Table>
        </Card>

        <Card size="small" :class="['module-card', 'sec-logs', { collapsed: collapsedSecs.logs }]" :bordered="true">
          <template #title>
            <span class="sec-title" @click="toggleSec('logs')">
              <span class="sec-toggle">{{ collapsedSecs.logs ? '▸' : '▾' }}</span> {{ $t('client.device_sectionLogs') }}
              <span class="section-hint">{{ $t('client.device_logsHint') }}</span>
            </span>
          </template>
          <div class="logs-toolbar">
            <Button class="ml-2" size="small" @click="loadLogs">
              {{ $t('client.device_refresh') }}
            </Button>
          </div>
          <pre class="log-box">{{ logText }}</pre>
        </Card>

        <Card size="small" :class="['module-card', 'sec-config', { collapsed: collapsedSecs.config }]" :bordered="true">
          <template #title><span class="sec-title" @click="toggleSec('config')"><span class="sec-toggle">{{ collapsedSecs.config ? '▸' : '▾' }}</span> {{ $t('client.device_sectionConfigChanges') }}</span></template>
          <div class="history-toolbar">
            <RangePicker
              v-model:value="configTimeRange"
              show-time
              format="YYYY-MM-DD HH:mm:ss"
              :placeholder="[$t('client.device_timeStart'), $t('client.device_timeEnd')]"
            />
            <Button size="small" @click="configTimeRange = undefined">
              {{ $t('client.device_reset') }}
            </Button>
            <Button size="small" @click="loadConfigChanges">
              {{ $t('client.device_refreshHistory') }}
            </Button>
            <Button
              size="small"
              type="primary"
              ghost
              :disabled="selectedChangeIds.length !== 2"
              :title="$t('client.device_configCompareHint')"
              @click="onCompareConfigs"
            >
              {{ $t('client.device_configCompare') }}（{{ selectedChangeIds.length }}/2）
            </Button>
          </div>
          <Table
            :columns="configColumns"
            :data-source="filteredConfigChanges"
            size="small"
            :pagination="{ pageSize: 10, showSizeChanger: true, size: 'small' }"
            row-key="changeId"
            :row-selection="configRowSelection"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'source'">
                <Tooltip
                  v-if="record.source"
                  :title="record.sourceDetail || record.source"
                  placement="topLeft"
                >
                  <span
                    :class="['aa-enum', `aa-enum--${configSourceMeta(record.source).color}`, 'cfg-src-tag']"
                  >
                    {{ $t(`client.${configSourceMeta(record.source).key}`) }}
                  </span>
                </Tooltip>
                <span v-else>—</span>
              </template>
              <template v-else-if="column.key === 'content'">
                <Button
                  type="primary" ghost
                  size="small"
                  @click="onViewConfig(record)"
                >
                  {{ $t('client.device_view') }}
                </Button>
              </template>
            </template>
          </Table>
        </Card>
      </div>
    </Drawer>

    <!-- MCP 分组绑定（共享抽屉：上半区已绑分组+组内 server，下半区选择新分组） -->
    <McpBindDrawer
      v-model:open="mcpVisible"
      target-type="device"
      :target-id="device?.deviceId ?? ''"
      :target-name="mcpDeviceName"
    />

    <!-- 配置快照查看（放大版：元信息头 + 全文滚动） -->
    <Modal
      v-model:open="configViewVisible"
      :title="configViewTitle"
      :width="1160"
      :footer="null"
      wrap-class-name="cfg-modal-wrap"
      class="cfg-view-modal"
    >
      <div v-if="configViewRow" class="cfg-meta">
        <div class="cfg-meta-row">
          <span class="cfg-meta-label">{{ $t('client.device_agent') }}</span>
          <span>{{ configViewRow.agent }}</span>
          <span class="cfg-meta-label">{{ $t('client.device_changedAt') }}</span>
          <span>{{ configViewRow.changedAt }}</span>
          <span class="cfg-meta-label">{{ $t('client.device_configSource') }}</span>
          <Tooltip
            v-if="configViewRow.source"
            :title="configViewRow.sourceDetail || configViewRow.source"
          >
            <span :class="['aa-enum', `aa-enum--${configSourceMeta(configViewRow.source).color}`]">
              {{ $t(`client.${configSourceMeta(configViewRow.source).key}`) }}
            </span>
          </Tooltip>
          <span v-else>—</span>
        </div>
        <div class="cfg-meta-row cfg-meta-mono">
          <span class="cfg-meta-label">Hash</span>
          <span>{{ configViewRow.configHash }}</span>
          <span class="cfg-meta-label">{{ $t('client.device_size') }}</span>
          <span>{{ configViewRow.configSize }} B</span>
          <span class="cfg-meta-label">Mtime</span>
          <span>{{ configViewRow.configMtime || '—' }}</span>
          <span class="cfg-meta-label">ChangeId</span>
          <span>{{ configViewRow.changeId }}</span>
        </div>
        <div v-if="configViewRow.sourceDetail" class="cfg-meta-detail">
          {{ configViewRow.sourceDetail }}
        </div>
      </div>
      <pre class="cfg-content-pre">{{ configViewText }}</pre>
    </Modal>

    <!-- 配置快照行级对比（LCS）：A=较早 B=较晚 -->
    <Modal
      v-model:open="diffVisible"
      :title="$t('client.device_configDiffTitle')"
      :width="1160"
      :footer="null"
      wrap-class-name="cfg-modal-wrap"
    >
      <div class="diff-head">
        <div class="diff-side">
          <div class="diff-side-title">A · {{ diffMetaA?.changedAt || '' }}</div>
          <div class="diff-side-sub">
            <span :class="['aa-enum', `aa-enum--${configSourceMeta(diffMetaA?.source).color}`]">
              {{ $t(`client.${configSourceMeta(diffMetaA?.source).key}`) }}
            </span>
            <span class="diff-hash">{{ (diffMetaA?.configHash || '').slice(0, 12) }}</span>
          </div>
        </div>
        <div class="diff-stats">
          <span class="diff-stat diff-stat-add">+{{ diffAddCount }}</span>
          <span class="diff-stat diff-stat-del">−{{ diffDelCount }}</span>
          <Checkbox v-model:checked="diffOnlyChanged" class="diff-only">
            {{ $t('client.device_configDiffOnly') }}
          </Checkbox>
        </div>
        <div class="diff-side">
          <div class="diff-side-title">B · {{ diffMetaB?.changedAt || '' }}</div>
          <div class="diff-side-sub">
            <span :class="['aa-enum', `aa-enum--${configSourceMeta(diffMetaB?.source).color}`]">
              {{ $t(`client.${configSourceMeta(diffMetaB?.source).key}`) }}
            </span>
            <span class="diff-hash">{{ (diffMetaB?.configHash || '').slice(0, 12) }}</span>
          </div>
        </div>
      </div>
      <div v-if="diffIsIdentical" class="diff-empty">
        {{ $t('client.device_configDiffEmpty') }}
      </div>
      <div v-else class="diff-body">
        <table class="diff-table">
          <tbody>
            <tr
              v-for="(r, i) in diffVisibleRows"
              :key="i"
              :class="`diff-row--${r.type}`"
            >
              <td class="diff-ln">{{ r.aIdx >= 0 ? r.aIdx + 1 : '' }}</td>
              <td class="diff-ln">{{ r.bIdx >= 0 ? r.bIdx + 1 : '' }}</td>
              <td class="diff-sign">{{ r.type === 'add' ? '+' : r.type === 'del' ? '−' : ' ' }}</td>
              <td class="diff-text">{{ r.text }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </Modal>

    <!-- 下发指令「配置」按钮弹窗：查看/编辑当前 agent 配置并下发（放大便于编辑/查看） -->
    <Modal
      v-model:open="configModalVisible"
      :title="configModalTitle"
      :width="1160"
      :footer="null"
      wrap-class-name="cfg-modal-wrap"
    >
      <div class="config-modal-tip">
        {{ $t('client.device_configPushTip') }}
        <Button size="small" class="config-format-btn" @click="formatConfigModal">
          {{ $t('client.device_configFormat') }}
        </Button>
      </div>
      <Textarea
        v-model:value="configModalContent"
        :rows="28"
        class="agent-config-textarea cfg-edit-textarea"
        placeholder="{}"
      />
      <div class="config-modal-footer">
        <Button
          type="primary"
          :loading="cmdSendingAgent === configModalAgent && cmdSendingAction === 'config_push'"
          @click="onConfigPush"
        >
          {{ $t('client.device_actionConfigPush') }}
        </Button>
      </div>
    </Modal>

    <Modal
      v-model:open="appTimelineVisible"
      :title="$t('client.device_appHistoryTitle', { name: device?.name || device?.deviceId || '' })"
      :width="520"
      :footer="null"
    >
      <div class="history-toolbar" style="margin-bottom: 12px">
        <RangePicker
          v-model:value="appTimeRange"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          :placeholder="[$t('client.device_timeStart'), $t('client.device_timeEnd')]"
          @change="onAppTimeRangeChange"
        />
        <Button size="small" @click="() => { appTimeRange = undefined; appTimelinePage = 1; }">
          {{ $t('client.device_reset') }}
        </Button>
      </div>
      <Timeline v-if="pagedAppTimelineRows.length">
        <TimelineItem
          v-for="(r, i) in pagedAppTimelineRows"
          :key="(r.statusId || '') + i"
          :color="r.status === 'running' ? 'magenta' : 'gray'"
        >
          <div class="timeline-line">
            <span :class="['aa-enum', `aa-enum--${r.status === 'running' ? 'magenta' : 'default'}`, 'timeline-tag']">
              {{ r.status === 'running' ? $t('client.status_appRunning') : $t('client.status_appStopped') }}
            </span>
            <span class="timeline-ver">v{{ r.version || '?' }}</span>
            <span class="timeline-ts">{{ r.reportedAt }}</span>
          </div>
        </TimelineItem>
      </Timeline>
      <div v-else class="timeline-empty">{{ $t('client.status_appHistoryEmpty') }}</div>
      <Pagination
        v-if="filteredAppTimelineRows.length"
        size="small"
        style="margin-top: 12px; text-align: right"
        :current="appTimelinePage"
        :page-size="appTimelinePageSize"
        :total="filteredAppTimelineRows.length"
        :show-size-changer="true"
        @change="(p: number, ps: number) => { appTimelinePage = p; appTimelinePageSize = ps; }"
      />
    </Modal>
  </Page>
</template>

<style scoped>
.drawer-body {
  padding: 0 8px 24px;
}
/* 模块卡片：每个功能区块一张 Card，模块间明暗交替 + 标题框与内容色区分。 */
.module-card {
  margin-bottom: 14px;
  border: 1px solid #e4e7ed;
}
/* 模块折叠：折叠时只留标题行，隐藏内容区 */
.module-card.collapsed :deep(.ant-card-body) {
  display: none;
}
/* 整个标题行都是折叠/展开开关 */
.sec-title {
  display: flex;
  flex: 1;
  align-items: center;
  min-height: 20px;
  cursor: pointer;
  user-select: none;
}
.sec-toggle {
  display: inline-block;
  width: 22px;
  margin-right: 4px;
  font-size: 16px;
  line-height: 1;
  text-align: center;
}
.module-card :deep(.ant-card-head) {
  min-height: 38px;
  padding: 0 14px;
  background: #eef0f3;
  border-bottom: 1px solid #e4e7ed;
}
.module-card :deep(.ant-card-head-title) {
  padding: 9px 0;
  font-size: 13px;
  font-weight: 600;
  /* 浅色主题：模块色加深版（62% 主色 + 黑）——原 #fff 白字配 40% 淡填充看不清；暗色由下方 html.dark 覆盖 */
  color: color-mix(in srgb, var(--c, #6366f1) 62%, #000) !important;
}
.module-card :deep(.ant-card-body) {
  padding: 14px;
}
/* 内容区交替：白 + 浅绿（品牌色 #42b983 浅化），亮色下对比明显 */
.drawer-body > .module-card:nth-child(odd) :deep(.ant-card-body) {
  background: #ffffff;
}
.drawer-body > .module-card:nth-child(even) :deep(.ant-card-body) {
  background: #eaf8f1;
}
.section-title {
  margin: 18px 0 8px;
  font-size: 14px;
  font-weight: 600;
}
.section-hint {
  font-size: 12px;
  font-weight: 400;
  color: #8a94a6;
  margin-left: 8px;
}
.cmd-actions {
  margin-top: 12px;
}
/* 历史区工具条：时间区间 + 重置 + 刷新 */
.history-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}
.config-modal-tip {
  flex: none;
  margin-bottom: 8px;
  font-size: 12px;
  color: #8a94a6;
}
.config-modal-footer {
  flex: none;
  margin-top: 10px;
  text-align: right;
}
.agent-tabs {
  margin-top: 4px;
}
.agent-ctrl {
  padding: 4px 4px 8px;
}
.agent-ctrl-empty {
  padding: 6px 4px 10px;
  color: #ff4500;
}
.agent-line {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}
.agent-label {
  font-size: 13px;
  color: #5a6474;
  margin-right: 8px;
}
.agent-btn-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.agent-config {
  margin-top: 4px;
}
.agent-config-body {
  margin-top: 6px;
}
.agent-config-textarea {
  width: 100%;
  margin: 6px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}
.cfg-edit-textarea {
  flex: 1 1 auto;
  min-height: 320px;
}
.config-format-btn {
  margin-left: 10px;
}
/* ===== 配置快照查看（放大版） ===== */
.cfg-meta {
  flex: none; /* 元信息头固定高，不参与压缩 */
  margin-bottom: 8px;
}
.cfg-meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 12px;
  font-size: 12px;
  margin-bottom: 4px;
}
.cfg-meta-label {
  color: #8c8c8c;
}
.cfg-meta-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: #5a6474;
  word-break: break-all;
}
html.dark .cfg-meta-mono {
  color: #a8b0bd;
}
.cfg-meta-detail {
  margin: 6px 0 2px;
  padding: 6px 10px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 4px;
  font-size: 12px;
  color: #5a6474;
}
.cfg-content-pre {
  flex: 1 1 auto; /* 占满弹窗剩余高度（body 为 flex 列） */
  min-height: 240px;
  overflow: auto;
  margin: 8px 0 0;
  padding: 12px 14px;
  background: #1f2933;
  color: #c8d1dd;
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre;
}
/* ===== 快照行级对比 ===== */
.diff-head {
  flex: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
.diff-side-title {
  font-size: 13px;
  font-weight: 600;
}
.diff-side-sub {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
}
.diff-hash {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: #5a6474;
}
html.dark .diff-hash {
  color: #a8b0bd;
}
.diff-stats {
  display: flex;
  align-items: center;
  gap: 10px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 600;
}
.diff-stat-add {
  color: #349e48;
}
.diff-stat-del {
  color: #b8001f;
}
/* 对比头统计在弹窗主题底上：暗色下提亮（色值与下方深底 diff 面板同系） */
html.dark .diff-stat-add {
  color: #8fe3a3;
}
html.dark .diff-stat-del {
  color: #f2a4b1;
}
.diff-only {
  font-weight: 400;
}
.diff-empty {
  padding: 24px 0;
  text-align: center;
  color: #8c8c8c;
}
.diff-body {
  flex: 1 1 auto; /* 占满弹窗剩余高度（body 为 flex 列） */
  min-height: 200px;
  overflow: auto;
  border-radius: 6px;
  background: #1f2933;
}
.diff-table {
  width: 100%;
  border-collapse: collapse;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
}
.diff-table td {
  padding: 0 6px;
  vertical-align: top;
  white-space: pre;
}
.diff-ln {
  width: 44px;
  min-width: 44px;
  text-align: right;
  color: #5f6b7d;
  user-select: none;
  border-right: 1px solid #313d4d;
}
.diff-sign {
  width: 18px;
  min-width: 18px;
  text-align: center;
  user-select: none;
}
.diff-text {
  color: #c8d1dd;
  word-break: break-all;
  white-space: pre-wrap;
}
.diff-row--add .diff-sign,
.diff-row--add .diff-text {
  background: rgba(52, 158, 72, 0.18);
  color: #8fe3a3;
}
.diff-row--del .diff-sign,
.diff-row--del .diff-text {
  background: rgba(184, 0, 31, 0.2);
  color: #f2a4b1;
}
.diff-row--same .diff-sign,
.diff-row--same .diff-text {
  color: #7d8899;
}
.cfg-src-tag {
  cursor: default;
}
.logs-toolbar {
  margin-bottom: 8px;
}
.log-box {
  max-height: 240px;
  overflow: auto;
  margin: 0;
  padding: 8px 10px;
  background: #1f2933;
  color: #c8d1dd;
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
.step-box {
  max-height: 200px;
  overflow: auto;
  margin: 4px 0 0;
  padding: 8px 12px;
  background: #1f2933;
  color: #c8d1dd;
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.cmd-progress {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 96px;
}
.cmd-prog-bar {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: rgba(0, 0, 0, 0.06);
  overflow: hidden;
}
.cmd-prog-fill {
  height: 100%;
  border-radius: 3px;
  background: #6366f1;
  transition: width 0.3s;
}
.cmd-prog-text {
  font-size: 12px;
  min-width: 34px;
  text-align: right;
  color: #5a6474;
}
html.dark .cmd-prog-text {
  color: rgba(255, 255, 255, 0.75);
}
.cmd-prog--ok .cmd-prog-fill {
  background: #52c41a;
}
.cmd-prog--skip .cmd-prog-fill {
  background: #8c8c8c;
}
.cmd-prog--err .cmd-prog-fill {
  background: #cf1322;
}
.cmd-prog--run .cmd-prog-fill {
  background: #1890ff;
}
.cmd-prog--wait .cmd-prog-fill {
  background: #d9d9d9;
}
.cmd-error {
  display: inline-block;
  max-width: 100%;
  font-size: 12px;
  color: #cf1322;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}
html.dark .cmd-error {
  color: #ff7875; /* 暗色下暗红 #cf1322 与深底同暗，提亮到 antd 暗色 error 字色 */
}
.app-status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 6px 0;
}
.selected-agents-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 6px 0;
}
.selected-agents-hint {
  font-size: 12px;
  color: #8a94a6;
}
.app-status-meta {
  font-size: 12px;
  color: #8a94a6;
}
.app-status-history {
  margin-left: 4px;
  padding: 0;
}
.timeline-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.timeline-tag {
  margin: 0;
}
.timeline-ver {
  font-size: 12px;
  color: #5a6474;
}
html.dark .timeline-ver {
  color: rgba(255, 255, 255, 0.78);
}
.timeline-ts {
  font-size: 11px;
  color: #8a94a6;
}
.timeline-empty {
  padding: 24px 0;
  text-align: center;
  color: #aab2c0;
}
.ml-2 {
  margin-left: 8px;
}
/* 渠道绑定模块 */
.channel-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.channel-row {
  padding: 10px 12px;
  border: 1px dashed #dcdfe6;
  border-radius: 6px;
}
.channel-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.channel-icon {
  font-size: 14px;
  margin-right: 4px;
}
.channel-name {
  font-weight: 600;
  margin-right: 6px;
}
.channel-id {
  color: #909399;
  font-size: 12px;
}
.channel-alias {
  color: #42b983;
  font-size: 12px;
}
.channel-actions {
  margin-top: 8px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.channel-qr {
  margin-top: 10px;
  text-align: center;
}
.qr-img {
  width: 200px;
  height: 200px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
}
.qr-tip {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
}
.channel-msg {
  margin-top: 6px;
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}
html.dark .channel-row {
  border-color: #3a3a3a;
}
html.dark .channel-id,
html.dark .qr-tip {
  color: #8a8a8a;
}
html.dark .channel-msg {
  color: rgba(255, 255, 255, 0.78);
}

/* ===== 10 个 section 卡片配色：完全参考 landing【技术栈】卡片——透明基 + 浅彩渐变填充(主色 8%)
   + 半透主色边框(30%)。color-mix 基于 transparent，亮/暗主题自适应，无需写两套。
   主色按「运行详情」模块顺序循环 landing 技术栈 8 色（末 2 复用）。
   antd Card 经 cssinjs 注入白底，故 background-color 透明 !important，让根的渐变透出。 */
.module-card.sec-info    { --c: #6366f1; }
.module-card.sec-agents  { --c: #06b6d4; }
.module-card.sec-app     { --c: #10b981; }
.module-card.sec-status  { --c: #3b82f6; }
.module-card.sec-switch  { --c: #ef4444; }
.module-card.sec-cmd     { --c: #42b983; }
.module-card.sec-channel { --c: #8b5cf6; }
.module-card.sec-skills { --c: #10b981; }
.module-card.sec-mcp     { --c: #14b8a6; }
.module-card.sec-history { --c: #f59e0b; }

/* MCP 分组：入口行（管理按钮 + 说明） */
.mcp-manage-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.mcp-manage-tip {
  font-size: 12px;
  color: #6b7280;
}
html.dark .mcp-manage-tip {
  color: rgba(255, 255, 255, 0.65);
}

/* 技能管理：安装行（命名空间收窄 + 搜索选择 + 按钮） */
.skill-install-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 10px;
}
.skill-install-row .skill-ns-select {
  width: 150px;
  flex: none;
}
.skill-install-row .skill-select {
  flex: 1;
  min-width: 220px;
}
/* 版本选择（选中技能后出现）：宽度收窄，草稿/旧版本号短 */
.skill-install-row .skill-ver-select {
  width: 130px;
  flex: none;
}
/* 版本下拉里「不安全」标：审计门禁禁选项的说明（红色系，亮暗双主题可读的暗红/亮红对） */
.skill-ver-unsafe {
  margin-left: 6px;
  color: #b8001f;
  font-size: 12px;
}
html.dark .skill-ver-unsafe {
  color: #f2a4b1;
}
.skill-search-row {
  margin-bottom: 8px;
}
.skill-search-input {
  width: 240px;
}
/* 下拉选项里的「已安装」标 */
.skill-opt-installed {
  margin-left: 6px;
  padding: 0 6px;
  border: 1px solid #10b981;
  border-radius: 3px;
  color: #10b981;
  font-size: 12px;
}
/* 选中待装技能的详情块 */
.skill-detail-box {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(148, 163, 184, 0.09);
}
.skill-detail-name {
  font-weight: 500;
  margin-bottom: 4px;
}
.skill-detail-slug {
  margin-left: 8px;
  color: #94a3b8;
  font-size: 12px;
}
.skill-detail-desc {
  color: #94a3b8;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}
.skill-already-hint {
  margin-top: 6px;
  color: #f59e0b;
  font-size: 12px;
}
/* 库最新列：与已装版本不一致时高亮（可升级） */
.skill-latest-new {
  color: #f59e0b;
  font-weight: 500;
}
.skill-upgrade-btn {
  padding: 0 !important;
}
/* 操作列两个按钮压缩间距 */
.skill-detail-btn {
  padding-left: 0;
  padding-right: 4px;
}
.skill-name {
  font-weight: 500;
}
.skill-desc {
  color: #94a3b8;
  font-size: 12px;
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.module-card.sec-logs    { --c: #6366f1; }
.module-card.sec-config  { --c: #06b6d4; }
.module-card.sec-appperm { --c: #f97316; }

.module-card {
  border-color: color-mix(in srgb, var(--c, #6366f1) 30%, transparent) !important;
  background-color: transparent !important;
  background-image: linear-gradient(160deg, color-mix(in srgb, var(--c, #6366f1) 8%, transparent), transparent 60%) !important;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
  animation: module-card-in 0.4s ease both;
}
/* 与 landing【技术栈】同款交互：hover 边框加深(55%) + 主色发光投影 + 上浮 3px */
.module-card:hover {
  border-color: color-mix(in srgb, var(--c, #6366f1) 55%, transparent) !important;
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c, #6366f1) 45%, transparent);
  transform: translateY(-3px);
}
/* 抽屉打开时模块错峰淡入（动效）；仅动 opacity，不碰 transform，避免与 hover 冲突 */
@keyframes module-card-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
.drawer-body > .module-card:nth-child(1) { animation-delay: 0.04s; }
.drawer-body > .module-card:nth-child(2) { animation-delay: 0.10s; }
.drawer-body > .module-card:nth-child(3) { animation-delay: 0.16s; }
.drawer-body > .module-card:nth-child(4) { animation-delay: 0.22s; }
.drawer-body > .module-card:nth-child(5) { animation-delay: 0.28s; }
.drawer-body > .module-card:nth-child(6) { animation-delay: 0.34s; }
.drawer-body > .module-card:nth-child(7) { animation-delay: 0.40s; }
.drawer-body > .module-card:nth-child(8) { animation-delay: 0.46s; }
.module-card :deep(.ant-card-body) {
  background-color: transparent !important;
  background-image: none !important;
}
/* 标题框填充 = 模块主色与底色混色（仅标题着色；内容区仍透出卡片根的浅渐变）。
   浅色下 40%：比上一版 25% 更深，配「主色加深」标题字保证可读。 */
.module-card :deep(.ant-card-head) {
  background-color: color-mix(in srgb, var(--c, #6366f1) 40%, var(--aa-bg, #fff)) !important;
  background-image: none !important;
}
.agent-switch-card :deep(.ant-card-body) {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* ===== App 权限管理：App 勾选卡片网格（自 App 权限管理 页迁入；勾选态随模块橙色 color-mix，亮暗自适应） ===== */
.appperm-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 10px;
}
.appperm-group {
  margin-bottom: 12px;
}
.appperm-group-title {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 600;
  color: color-mix(in srgb, var(--c, #f97316) 55%, var(--aa-bg, #fff));
}
/* 分组标题按类别着色（色值同 styles/enum-badge.css 调色板，与 /client/appperm 类别章一致） */
.appperm-group-title.appperm-cat-comm {
  color: #349e48;
}
.appperm-group-title.appperm-cat-docs {
  color: #0266b3;
}
.appperm-group-title.appperm-cat-tools {
  color: #ff4500;
}
.appperm-group-title.appperm-cat-browser {
  color: #0dd3bb;
}
.appperm-group-title.appperm-cat-ai {
  color: #cc52b9;
}
.appperm-group-title.appperm-cat-system,
.appperm-group-title.appperm-cat-other {
  color: #8c8c8c;
}
.appperm-group-count {
  padding: 0 6px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 400;
  color: #888;
}
html.dark .appperm-group-count {
  border-color: #3a3f4a;
  color: #999;
}
.appperm-count {
  font-size: 13px;
  color: #888;
}
.appperm-empty {
  grid-column: 1 / -1;
  padding: 18px;
  color: #aab2c0;
}
.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 8px;
}
.app-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s;
}
.app-card:hover {
  border-color: #f97316;
}
.app-card.checked {
  border-color: #f97316;
  background: color-mix(in srgb, #f97316 10%, transparent);
}
.app-logo {
  font-size: 20px;
  line-height: 1.2;
}
img.app-logo {
  width: 20px;
  height: 20px;
  border-radius: 4px;
}
.app-name {
  font-weight: 500;
}
.app-bundle {
  font-size: 11px;
  color: #888;
  font-family: monospace;
  word-break: break-all;
}
/* 授权状态徽标（客户端上报）：绿=已授权 / 红=已拒绝 / 灰=待授权 */
.app-perm-badge {
  font-size: 11px;
  line-height: 1.4;
  padding: 0 6px;
  border-radius: 8px;
  width: fit-content;
}
.app-perm-badge.granted {
  color: #16a34a;
  background: color-mix(in srgb, #16a34a 12%, transparent);
}
.app-perm-badge.denied {
  color: #dc2626;
  background: color-mix(in srgb, #dc2626 12%, transparent);
}
.app-perm-badge.pending {
  color: #888;
  background: color-mix(in srgb, #888 12%, transparent);
}
.app-regrant {
  font-size: 11px;
  padding: 0;
  height: auto;
  width: fit-content;
}
.app-check {
  position: absolute;
  top: 6px;
  right: 10px;
  color: #f97316;
  font-weight: 700;
}
.appperm-hint {
  font-size: 11px;
  color: #999;
  margin-bottom: 6px;
}
/* 未启用 openclaw：整块锁定（hermes 自带工具不经客户端闸门，清单对其无约束意义） */
.appperm-locked {
  pointer-events: none;
  opacity: 0.45;
  filter: grayscale(0.6);
}
.appperm-disabled-alert {
  margin-bottom: 10px;
}
.appperm-hint-hermes {
  color: #d46b08;
}
</style>

<!-- 暗色主题覆盖：非 scoped，用 .device-drawer 限定避免污染全局。
     html.dark 下切换卡片标题框/内容区/交替底色为深色，保证字看清。 -->
<style>
html.dark .device-drawer .module-card {
  border-color: #3a3a3a;
}
/* App 权限管理卡片网格暗色适配 */
html.dark .device-drawer .app-card {
  border-color: #3a3a3a;
}
html.dark .device-drawer .app-name {
  color: #e8e8e8;
}
html.dark .device-drawer .app-bundle,
html.dark .device-drawer .appperm-count {
  color: #8a8a8a;
}
html.dark .device-drawer .appperm-hint-hermes {
  color: #d48806;
}
html.dark .device-drawer .appperm-hint {
  color: #8a8a8a;
}
html.dark .device-drawer .app-perm-badge.granted {
  color: #4ade80;
  background: color-mix(in srgb, #4ade80 14%, transparent);
}
html.dark .device-drawer .app-perm-badge.denied {
  color: #f87171;
  background: color-mix(in srgb, #f87171 14%, transparent);
}
html.dark .device-drawer .app-perm-badge.pending {
  color: #9a9a9a;
  background: color-mix(in srgb, #9a9a9a 14%, transparent);
}
/* 暗色：标题填充沿用 scoped 块的「主色 25% + 底色」混色（--aa-bg=#111827，本就偏深）；
   这里只补标题文字色——须带 !important 才能压过 scoped 块的 !important（原规则没带，从未生效）。 */
html.dark .device-drawer .module-card .ant-card-head {
  border-bottom-color: #3a3a3a;
}
html.dark .device-drawer .module-card .ant-card-head-title {
  color: #e8e8e8 !important;
}
html.dark .device-drawer .drawer-body > .module-card:nth-child(odd) .ant-card-body {
  background: #1f1f1f;
}
html.dark .device-drawer .drawer-body > .module-card:nth-child(even) .ant-card-body {
  background: #1d2e26;
}

/* section 标题框：随 tech 风格透明化——scoped 块已把 head 设为透明并露出卡片根的渐变，
   故此处不再单独着色（保留下方 head 文字暗色可读性规则）。 */

/* 模块内表格填满模块色：表格各层背景一律透明，卡片 body 的模块色透出（表格=模块色，不再露白底）。
   覆盖 colored 卡（history/config 表格）与 agent-status 表格，整张表随模块着色。 */
.device-drawer .ant-table,
.device-drawer .ant-table-container,
.device-drawer .ant-table-header,
.device-drawer .ant-table-thead,
.device-drawer .ant-table-tbody,
.device-drawer .ant-table-row,
.device-drawer .ant-table-content,
.device-drawer .ant-table-wrapper,
.device-drawer .ant-table-body,
.device-drawer .ant-table-cell {
  background: transparent !important;
}

/* 模块内时间选择器随模块色：半透明白蒙层（frosted）让模块色透出（指令历史=橙 / 配置变更=粉），
   同时把选择器内输入/占位/分隔符/图标加深，避免灰字在浅彩底上看不清。 */
.device-drawer .module-card .ant-picker {
  background: rgba(255, 255, 255, 0.55) !important;
}
.device-drawer .module-card .ant-picker:hover,
.device-drawer .module-card .ant-picker.ant-picker-focused {
  background: rgba(255, 255, 255, 0.72) !important;
}
.device-drawer .module-card .ant-picker-input > input,
.device-drawer .module-card .ant-picker .ant-picker-suffix,
.device-drawer .module-card .ant-picker-separator {
  color: rgba(0, 0, 0, 0.8) !important;
}
.device-drawer .module-card .ant-picker .ant-picker-placeholder {
  color: rgba(0, 0, 0, 0.5) !important;
}
html.dark .device-drawer .module-card .ant-picker {
  background: rgba(0, 0, 0, 0.22) !important;
}
html.dark .device-drawer .module-card .ant-picker:hover,
html.dark .device-drawer .module-card .ant-picker.ant-picker-focused {
  background: rgba(0, 0, 0, 0.32) !important;
}

/* 标题框辅助文字：浅色标题框是「主色 40% 混白」淡彩底，须深字才看清；
   暗色标题框偏深，用白半透。（旧版两头都写白半透，浅色下小字发浅看不清） */
.device-drawer .module-card .ant-card-head .section-hint {
  color: rgba(0, 0, 0, 0.65) !important;
}
html.dark .device-drawer .module-card .ant-card-head .section-hint {
  color: rgba(255, 255, 255, 0.85) !important;
}
/* 内容区辅助文字（selected-agents-hint / agent-label 在内容区浅渐变底上）：亮→深字、暗→亮字 */
.device-drawer .module-card .selected-agents-hint,
.device-drawer .module-card .agent-label {
  color: rgba(0, 0, 0, 0.72) !important;
}
html.dark .device-drawer .module-card .selected-agents-hint,
html.dark .device-drawer .module-card .agent-label {
  color: rgba(255, 255, 255, 0.85) !important;
}

/* Agent Logo 小图：运行状态表单元格 + Agent 切换器(VNode，放全局才能命中) */
.agent-logo-cell {
  width: 18px;
  height: 18px;
  margin-right: 6px;
  object-fit: contain;
  vertical-align: middle;
}
/* 运行状态表 Agent 列：logo + name 同行（inline-flex 确保不换行、垂直居中） */
.agent-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.agent-cell .agent-logo-cell {
  margin-right: 0; /* gap 已分隔，去额外 margin */
}
/* 运行状态表版本列：安装中时高亮「安装中 vX」（与 installing 状态标签同色系 gold） */
.version-installing {
  color: #d48806;
  font-weight: 500;
}
html.dark .version-installing {
  color: #ffc53d;
}
/* 当前 Agent 切换器（Segmented）：选中项直接品牌色背景 + 白字（不依赖 thumb 滑动块——thumb 绝对定位
   渲染不可靠，之前选中白字落在白容器上看不见；改 item-selected 自身背景，稳定可见）*/
.device-drawer .sec-switch .ant-segmented {
  background: #ffffff;
  padding: 2px;
}
html.dark .device-drawer .sec-switch .ant-segmented {
  background: rgba(255, 255, 255, 0.14);
}
.device-drawer .sec-switch .ant-segmented-thumb {
  background: transparent !important;
  box-shadow: none !important;
}
.device-drawer .sec-switch .ant-segmented-item-selected {
  background: #42b983 !important;
  color: #fff !important;
  font-weight: 700;
}
.device-drawer .sec-switch .ant-segmented-item:not(.ant-segmented-item-selected) {
  color: #5a6474;
}
html.dark .device-drawer .sec-switch .ant-segmented-item:not(.ant-segmented-item-selected) {
  color: rgba(255, 255, 255, 0.65);
}
.device-drawer .seg-agent {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.device-drawer .seg-agent-logo {
  width: 16px;
  height: 16px;
  object-fit: contain;
}
/* 本设备使用 Agent 复选框：Logo + 名字（VNode label） */
.device-drawer .chk-agent {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.device-drawer .chk-agent-logo {
  width: 16px;
  height: 16px;
  object-fit: contain;
}
/* 配置查看/编辑/对比 大弹窗：贴近顶部、占满纵向空间（wrap 是 teleport 容器，须全局样式）。
   body 做成 flex 列：元信息头固定，内容区（pre/textarea/diff）flex 填满到弹窗底。 */
.cfg-modal-wrap .ant-modal {
  top: 24px;
  padding-bottom: 12px;
}
.cfg-modal-wrap .ant-modal-body {
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 110px); /* 24 顶部偏移 + 标题栏 + 边距 */
  overflow-y: auto; /* 兜底：内容极端超高时整体可滚 */
}
.cfg-modal-wrap .cfg-meta-detail {
  background: rgba(0, 0, 0, 0.04);
}
html.dark .cfg-modal-wrap .cfg-meta-detail {
  background: rgba(255, 255, 255, 0.06);
  color: #a8b0bd;
}
</style>
