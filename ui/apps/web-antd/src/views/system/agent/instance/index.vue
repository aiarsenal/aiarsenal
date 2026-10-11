<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Card, Descriptions, DescriptionsItem, Drawer, Input, message, Modal, Select, Spin, Table, Tabs, Tooltip, Upload } from 'ant-design-vue';

import dayjs, { type Dayjs } from 'dayjs';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addAgentInstanceApi,
  deleteAgentInstanceApi,
  enqueueAgentInstanceCommandApi,
  getAgentInstanceConfigChangesApi,
  getAgentInstanceCommandsApi,
  getAgentInstanceListApi,
  getAgentInstanceSkillsApi,
  getAgentRuntimeStatusApi,
  getAgentRuntimeStatusHistoryApi,
  installAgentInstanceSkillApi,
  refreshAgentInstanceLlmKeyApi,
  restartAgentInstanceApi,
  requestDesktopAccessApi,
  startAgentInstanceApi,
  stopAgentInstanceApi,
  uninstallAgentInstanceSkillApi,
} from '#/api/system/agent/instance';
import { getAgentImageListApi } from '#/api/system/agent/settings';
import {
  deleteAgentWorkspaceApi,
  downloadAgentWorkspaceApi,
  getAgentWorkspaceListApi,
  mkdirAgentWorkspaceApi,
  renameAgentWorkspaceApi,
  uploadAgentWorkspaceApi,
} from '#/api/system/agent/workspace';
import type { AgentWorkspaceApi } from '#/api/system/agent/workspace';
import {
  getSkillApi,
  getSkillListApi,
  getSkillVersionsApi,
  type SkillApi,
} from '#/api/system/skill/skill';
import type { InstanceSkillItem } from '#/api/system/agent/instance';
import ActionOverflow from '#/components/ActionOverflow.vue';
import ReportTimeFilter from '#/components/ReportTimeFilter.vue';
import ConfigDiffDialog from '#/components/ConfigDiffDialog.vue';
import McpBindDrawer from '#/components/McpBindDrawer.vue';
import { AGENT_OPTIONS, SPEC_PRESETS } from '#/constants/agent-options';
import QRCode from 'qrcode';

defineOptions({ name: 'AgentInstance' });

const statusTag = (s: number) => {
  const map: Record<number, { color: string; key: string }> = {
    0: { color: 'blue', key: 'instance_status_creating' },
    1: { color: 'magenta', key: 'instance_status_running' },
    2: { color: 'default', key: 'instance_status_stopped' },
    3: { color: 'red', key: 'instance_status_error' },
  };
  const m = map[s] || map[3];
  return { color: m.color, text: $t(`agent.${m.key}`) };
};

// 配置变更来源 tag：dispatch:* = api 下发（蓝族，按场景细分），runtime:* = 运行时上报（橙=应用/红=被守卫改回）。
// 颜色一律避开绿色（站点主题已是绿，避免与状态列 running 混淆）。
const sourceTag = (src: string) => {
  const map: Record<string, { color: string; key: string }> = {
    'dispatch:create': { color: 'blue', key: 'instance_cfgSrc_dispatch_create' },
    'dispatch:key-rotate': { color: 'blue', key: 'instance_cfgSrc_dispatch_keyRotate' },
    'dispatch:model-edit': { color: 'geekblue', key: 'instance_cfgSrc_dispatch_modelEdit' },
    'dispatch:model-offline': { color: 'purple', key: 'instance_cfgSrc_dispatch_modelOffline' },
    'runtime:applied': { color: 'orange', key: 'instance_cfgSrc_runtime_applied' },
    'runtime:reverted': { color: 'red', key: 'instance_cfgSrc_runtime_reverted' },
    // 兼容旧值
    'dispatch': { color: 'blue', key: 'instance_cfgSrc_dispatch' },
    'runtime': { color: 'orange', key: 'instance_cfgSrc_runtime' },
  };
  const m = map[src] || { color: 'default', key: 'instance_cfgSrc_unknown' };
  return { color: m.color, text: $t(`agent.${m.key}`) };
};

const searchSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input', fieldName: 'name', label: $t('agent.instance_name') },
]);

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('agent.instance_seq'), width: 60 },
    { field: 'name', title: $t('agent.instance_name'), minWidth: 140 },
    { field: 'agent', title: $t('agent.instance_agent'), width: 100 },
    { field: 'version', title: $t('agent.instance_version'), width: 90 },
    { field: 'llmKeyHash', title: $t('agent.instance_llmKey'), width: 130, slots: { default: 'llmKeyHash' } },
    {
      field: 'status',
      title: $t('agent.instance_status'),
      width: 100,
      slots: { default: 'status' },
    },
    { field: 'imageTag', title: $t('agent.instance_image'), minWidth: 200 },
    { field: 'containerId', title: $t('agent.instance_container'), width: 130, slots: { default: 'containerId' } },
    { field: 'hostPort', title: $t('agent.instance_port'), width: 80 },
    { field: 'accessUrl', title: $t('agent.instance_accessUrl'), minWidth: 180, slots: { default: 'accessUrl' } },
    { field: 'createdAt', title: $t('agent.instance_createdAt'), minWidth: 150 },
    { field: 'action', title: $t('agent.instance_action'), width: 400, fixed: 'right', slots: { default: 'action' }, showOverflow: false },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getAgentInstanceListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          status: -1, // -1=全部（后端 status>=0 才过滤；0 是「创建中」，不能当默认值）
          ...(form as any),
        });
        const items = res?.list ?? [];
        // 创建是异步的（后台协程起容器）：列表里有 status=0(创建中) 实例就起轮询，
        // 协程把状态刷成 运行中/错误 后，下一轮判定无创建中即自动停。
        if (items.some((r: any) => r?.status === 0)) startPolling();
        return { items, total: res?.total ?? 0 };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: { code: 'query' } },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

// 异步创建状态轮询：直接拉状态、用 setRow 就地更新变化的行
// （不整表重载、不闪 loading 蒙层、不重置滚动），无创建中实例后自动停。startPolling 幂等。
let pollTimer: ReturnType<typeof setInterval> | null = null;
function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}
async function pollStatus() {
  const grid = gridApi.grid as any;
  const rows: any[] = grid?.getData?.() ?? [];
  if (!rows.length) {
    stopPolling();
    return;
  }
  let res: any;
  try {
    res = await getAgentInstanceListApi({ pageNum: 1, pageSize: 200, status: -1 });
  } catch {
    return;
  }
  const fresh: any[] = res?.list ?? [];
  const byId = new Map(fresh.map((f: any) => [f.instanceId, f]));
  let stillCreating = false;
  let changed = false;
  for (const row of rows) {
    const f = byId.get(row.instanceId);
    if (!f) continue;
    if (f.status === 0) stillCreating = true;
    // 就地改变化字段；setRow 不重渲染 slot 单元格，故改完统一 refreshColumn 触发状态标签重绘
    for (const k of ['status', 'containerId', 'hostPort', 'imageTag', 'accessUrl', 'updatedAt']) {
      if (f[k] !== undefined && f[k] !== row[k]) {
        row[k] = f[k];
        changed = true;
      }
    }
  }
  if (changed) grid?.refreshColumn?.();
  if (!stillCreating) stopPolling();
}
function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(pollStatus, 5000);
}
onUnmounted(stopPolling);

const modalVisible = ref(false);

// 镜像设置缓存（来自 /agent/settings/list）：用于级联版本号 + 按 agent/version 自动回填镜像。
// version 下拉选项实时读表单 agent 过滤；image 字段按二元组查出来只读回填。
const imageSettings = ref<any[]>([]);
function resolveImage(agent: string, version: string) {
  return (
    imageSettings.value.find(
      (s) => s.agent === agent && s.version === version,
    )?.image ?? ''
  );
}
async function loadImageSettings() {
  try {
    const res = await getAgentImageListApi({ agent: '', version: '' });
    imageSettings.value = res?.list ?? [];
  } catch {
    imageSettings.value = [];
  }
}

// 版本号数值化比较：逐段按数字比（"0.10.0" > "0.9.0"），非数字段回退字符串比。
function cmpVersion(a: string, b: string): number {
  const pa = a.split('.');
  const pb = b.split('.');
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i] ?? '';
    const y = pb[i] ?? '';
    const nx = Number(x);
    const ny = Number(y);
    const c =
      x !== '' && y !== '' && Number.isFinite(nx) && Number.isFinite(ny)
        ? nx - ny
        : x.localeCompare(y);
    if (c !== 0) return c;
  }
  return 0;
}

// 该 agent 的最新版本：镜像设置里版本号最大者。后端列表按字典序排（0.10/0.9 会错序），
// 不能取首项，须前端比较取最大；无该 agent 记录返回 ''（表单必填校验会拦）。
function latestVersionFor(agent: string): string {
  return imageSettings.value
    .filter((s) => s.agent === agent)
    .reduce((best, s) => (cmpVersion(s.version, best) > 0 ? s.version : best), '');
}

const formSchema = computed<VbenFormSchema[]>(() => [
  { component: 'Input' as const, fieldName: 'name', label: $t('agent.instance_name'), rules: 'required', componentProps: { placeholder: $t('agent.instance_namePlaceholder') } },
  {
    component: 'Select' as const,
    fieldName: 'agent',
    label: $t('agent.instance_agent'),
    rules: 'required',
    // width 必须显式给：antd-vue 的 placeholder 是 flex:1（flex-basis:0），
    // 空值时 Select 固有宽度塌缩到只剩内边距+箭头，placeholder 被裁到只剩一个字母
    componentProps: {
      options: AGENT_OPTIONS,
      // 切换 agent 后版本随动：重选新 agent 的最新版本并回填镜像（原版本多半不再适用）
      onChange: (v: any) => {
        const ver = latestVersionFor(v);
        formApi.setValues({ version: ver, imageTag: resolveImage(v, ver) });
      },
      style: { width: '100%' },
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'version',
    label: $t('agent.instance_version'),
    rules: 'required',
    // 版本号选项实时来自镜像设置（按当前 agent 过滤）；选中后按二元组回填只读 image
    componentProps: (values: any) => ({
      options: imageSettings.value
        .filter((s) => s.agent === values.agent)
        .map((s) => ({ label: s.version, value: s.version })),
      placeholder: $t('agent.instance_versionPlaceholder'),
      style: { width: '100%' },
      onChange: (v: any) =>
        formApi.setValues({ imageTag: resolveImage(values.agent, v) }),
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'imageTag',
    label: $t('agent.instance_image'),
    // 只读：由 agent/version 从镜像设置自动确定，不可编辑
    componentProps: { placeholder: $t('agent.instance_imageAutoPlaceholder'), disabled: true },
  },
  {
    component: 'Select' as const,
    fieldName: 'spec',
    label: $t('agent.instance_spec'),
    rules: 'required',
    componentProps: { options: SPEC_PRESETS, placeholder: $t('agent.instance_specPlaceholder'), style: { width: '100%' } },
  },
  { component: 'InputNumber' as const, fieldName: 'containerPort', label: $t('agent.instance_containerPort'), componentProps: { min: 1, style: { width: '100%' } } },
  { component: 'InputNumber' as const, fieldName: 'hostPort', label: $t('agent.instance_hostPort'), componentProps: { min: 0, style: { width: '100%' } } },
  { component: 'Textarea' as const, fieldName: 'description', label: $t('agent.instance_description'), componentProps: { rows: 2 } },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  modalVisible.value = true;
  await nextTick();
  await loadImageSettings();
  await formApi.resetForm();
  // 版本默认填该 agent 的最新版本（镜像设置里最大版本号），镜像随之回填
  const version = latestVersionFor('openclaw');
  await formApi.setValues({
    agent: 'openclaw',
    version,
    imageTag: resolveImage('openclaw', version),
    spec: 'lite',
    containerPort: 3000,
    hostPort: 0,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const v: any = await formApi.getValues();
  // 规格：按所选预设档位解析出 cpu/mem/disk（表单不再单独采集这三项）
  const preset = SPEC_PRESETS.find((p) => p.value === v.spec) ?? SPEC_PRESETS[0];
  // 镜像：以表单回填值为准，兜底再按二元组查一次
  const imageTag = v.imageTag || resolveImage(v.agent, v.version);
  const ports = v.containerPort > 0 ? [{ hostPort: v.hostPort || 0, containerPort: v.containerPort }] : [];
  try {
    await addAgentInstanceApi({
      name: v.name,
      agent: v.agent,
      version: v.version,
      imageTag,
      cpuCores: preset.cpuCores,
      memoryGb: preset.memoryGb,
      diskGb: preset.diskGb,
      description: v.description,
      ports,
    });
    message.success($t('agent.instance_addSuccess'));
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 操作列按钮：按行状态生成；>4 个时由 ActionOverflow 折叠进「⋯」下拉（运行详情固定第一位）
function actionsOf(row: any): any[] {
  const items: any[] = [];
  items.push({ key: 'report', label: $t('agent.instance_runtimeBtn'), onClick: () => openReport(row) });
  if (row.status === 1) {
    items.push({ key: 'desktop', label: $t('agent.instance_openDesktop'), onClick: () => openDesktop(row) });
    items.push({ key: 'bindSocial', label: $t('agent.instance_bindSocial'), onClick: () => openBindSocial(row) });
    items.push({ key: 'stop', label: $t('agent.instance_stop'), onClick: () => onAction(row, stopAgentInstanceApi, 'instance_stopSuccess') });
    items.push({ key: 'restart', label: $t('agent.instance_restart'), onClick: () => onAction(row, restartAgentInstanceApi, 'instance_restartSuccess') });
  } else if (row.status === 2 || row.status === 3) {
    // 非运行（停止/错误）才出现「启动」，运行中由「停止/重启」接管
    items.push({ key: 'start', label: $t('agent.instance_start'), onClick: () => onAction(row, startAgentInstanceApi, 'instance_startSuccess') });
  }
  items.push({ key: 'skill', label: $t('agent.instance_skillManage'), onClick: () => openSkillManage(row) });
  // MCP 工具绑定：openclaw（mcp.servers）与 hermes（config.yaml mcp_servers）均支持
  if (row.agent === 'openclaw' || row.agent === 'hermes') {
    items.push({ key: 'mcp', label: $t('agent.instance_mcpManage'), onClick: () => openMcpManage(row) });
  }
  if (row.llmKeyHash) {
    items.push({ key: 'refreshKey', label: $t('agent.instance_refreshKey'), onClick: () => onRefreshKey(row) });
  }
  items.push({ key: 'delete', label: $t('agent.instance_delete'), danger: true, onClick: () => onDelete(row) });
  return items;
}

async function onAction(row: any, fn: any, okKey: string) {
  try {
    await fn(row.instanceId);
    message.success($t(`agent.${okKey}`));
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 刷新密钥：轮换实例专属 LLM key 并重发 agent 运行配置（5 分钟宽限内零停机切换）。
async function onRefreshKey(row: any) {
  try {
    await refreshAgentInstanceLlmKeyApi(row.instanceId);
    message.success($t('agent.instance_refreshKeySuccess'));
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('agent.instance_selectToDelete'));
    return;
  }
  Modal.confirm({
    title: $t('agent.team_confirmDeleteTitle'),
    content: $t('agent.instance_confirmDeleteContent', { count: rows.length }),
    okText: $t('agent.team_confirm'),
    cancelText: $t('agent.team_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentInstanceApi(rows.map((r: any) => r.instanceId));
      message.success($t('agent.instance_deleteSuccess'));
      gridApi.query();
    },
  });
}

// 单行删除：删当前行（原来行内按钮误绑 onDeleteBatch，会删勾选行而非本行）。
async function onDelete(row: any) {
  Modal.confirm({
    title: $t('agent.team_confirmDeleteTitle'),
    content: $t('agent.instance_confirmDeleteContent', { count: 1 }),
    okText: $t('agent.team_confirm'),
    cancelText: $t('agent.team_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentInstanceApi([row.instanceId]);
      message.success($t('agent.instance_deleteSuccess'));
      gridApi.query();
    },
  });
}

// 上报数据弹窗：展示容器内 manager（image/manager）心跳上报的各模块状态。
// 数据来自 /agent/instance/:id/runtime-status → summaryJson（manager 上报的原始 modules）。
const reportVisible = ref(false);
const reportLoading = ref(false);
const reportTitle = ref('');
const reportModules = ref<any[]>([]);
const reportLastReported = ref('');
const reportFresh = ref(false);
const reportTab = ref('current');
const reportHistory = ref<any[]>([]);
const reportConfigChanges = ref<any[]>([]);
const reportCommands = ref<any[]>([]);

// 配置变更对比：跨页多选（最多 2 条），按 changedAt 排序后逐行 diff。
const selectedChangeIds = ref<string[]>([]);
const diffVisible = ref(false);
const diffLeft = ref<any>(null);
const diffRight = ref<any>(null);

// 弹窗内翻页 + 时间筛选：数据已一次性全量拉到前端（各 tab 上限 50 行），故纯前端筛选/分页。
const reportPage = ref(1);
const reportPageSize = ref(10);
const reportTimeRange = ref<[Dayjs, Dayjs] | undefined>();

// 各 tab 对应的时间列：history→reportedAt / config→changedAt / commands→issuedAt；current 为快照无时间列。
function inTimeRange(ts?: string): boolean {
  const range = reportTimeRange.value;
  if (!range || range.length !== 2) return true;
  if (!ts) return false;
  const t = dayjs(ts);
  if (!t.isValid()) return false;
  const start = range[0]?.startOf('second').valueOf() ?? -Infinity;
  const end = range[1]?.endOf('second').valueOf() ?? Infinity;
  const v = t.valueOf();
  return v >= start && v <= end;
}
function filterByTime(rows: any[], field: string): any[] {
  return reportTimeRange.value ? rows.filter((r) => inTimeRange(r?.[field])) : rows;
}

// 按时间筛选后的各 tab 数据（current 为快照，不过滤）。
const historyFiltered = computed(() => filterByTime(reportHistory.value, 'reportedAt'));
const configFiltered = computed(() => filterByTime(reportConfigChanges.value, 'changedAt'));
const commandsFiltered = computed(() => filterByTime(reportCommands.value, 'issuedAt'));

// 配置变更多选：跨页保留（preserveSelectedRowKeys），最多 2 条——选满后未选行禁用。
const configRowSelection = computed(() => ({
  type: 'checkbox' as const,
  selectedRowKeys: selectedChangeIds.value,
  preserveSelectedRowKeys: true,
  onChange: (keys: any[]) => {
    selectedChangeIds.value = keys as string[];
  },
  getCheckboxProps: (record: any) => ({
    disabled:
      selectedChangeIds.value.length >= 2 && !selectedChangeIds.value.includes(record.changeId),
  }),
}));

// 客户端分页配置（三表共用：同时只显示一个 tab，切 tab/改筛选时重置到第 1 页）。
const reportPagination = computed(() => ({
  current: reportPage.value,
  pageSize: reportPageSize.value,
  showSizeChanger: true,
  pageSizeOptions: ['10', '20', '50'],
  size: 'small',
  showTotal: (total: number) => $t('agent.instance_reportTotal', { total }),
  onChange: (p: number, ps: number) => {
    reportPage.value = p;
    reportPageSize.value = ps;
  },
  onShowSizeChange: (p: number, ps: number) => {
    reportPage.value = p;
    reportPageSize.value = ps;
  },
}));

// 改时间筛选回到第 1 页（切 tab 的翻页重置在 onReportTabChange 里处理）。
watch(reportTimeRange, () => {
  reportPage.value = 1;
});

const reportColumns = computed(() => [
  { title: $t('agent.instance_reportColModule'), dataIndex: 'name', width: 100, fixed: 'left' },
  { title: $t('agent.instance_reportColStatus'), dataIndex: 'status', width: 90 },
  { title: $t('agent.instance_reportColVersion'), dataIndex: 'version', width: 160, ellipsis: true },
  { title: $t('agent.instance_reportColPid'), dataIndex: 'pidOrContainerId', width: 120 },
  { title: $t('agent.instance_reportColUptime'), dataIndex: 'uptimeSeconds', width: 90 },
  { title: $t('agent.instance_reportColConfig'), dataIndex: 'configHash', minWidth: 210 },
  { title: $t('agent.instance_reportColError'), dataIndex: 'lastError', ellipsis: true },
]);

// 历史时间线列（状态随时间变化，心跳合并去重后的历史行）。
const historyColumns = computed(() => [
  { title: $t('agent.instance_reportColReportedAt'), dataIndex: 'reportedAt', width: 180 },
  { title: 'infra', dataIndex: 'infraStatus', width: 70 },
  { title: 'agent', dataIndex: 'agentStatus', width: 70 },
  { title: reportInstanceAgent.value || 'agent', dataIndex: 'agentProcStatus', width: 80 },
  { title: 'version', dataIndex: 'agentProcVersion', width: 150, ellipsis: true },
  { title: 'configHash', dataIndex: 'configHash', width: 110, ellipsis: true },
  { title: $t('agent.instance_reportColCommand'), dataIndex: 'runningCommandSig', width: 120, ellipsis: true },
]);

// 配置变更列（下发 dispatch + 运行时 runtime，按 agent 区分，含配置全文）。
const configChangeColumns = computed(() => [
  { title: $t('agent.instance_reportColChangedAt'), dataIndex: 'changedAt', width: 180 },
  { title: 'agent', dataIndex: 'agentId', width: 90 },
  { title: $t('agent.instance_reportColSource'), dataIndex: 'source', width: 120 },
  { title: $t('agent.instance_reportColDispatcher'), dataIndex: 'dispatcher', width: 130, ellipsis: true },
  { title: 'configHash', dataIndex: 'configHash', width: 130, ellipsis: true },
  { title: 'size', dataIndex: 'configSize', width: 70 },
  { title: $t('agent.instance_reportColContent'), dataIndex: 'content', ellipsis: true },
]);

// 命令历史列（控制面下发 + 执行结果 + 进度）。
const commandColumns = computed(() => [
  { title: $t('agent.instance_reportColIssuedAt'), dataIndex: 'issuedAt', width: 180 },
  { title: $t('agent.instance_reportColCommandId'), dataIndex: 'instanceCommandId', width: 120, ellipsis: true },
  { title: $t('agent.instance_reportColType'), dataIndex: 'commandType', width: 100 },
  { title: 'agent', dataIndex: 'agentId', width: 90 },
  { title: $t('agent.instance_reportColStatus'), dataIndex: 'status', width: 80 },
  { title: $t('agent.instance_reportColPayload'), dataIndex: 'payloadJson', width: 160, ellipsis: true },
  { title: $t('agent.instance_reportColProgress'), dataIndex: 'progressJson', width: 140, ellipsis: true },
  { title: $t('agent.instance_reportColDispatcher'), dataIndex: 'issuedBy', width: 130, ellipsis: true },
  { title: $t('agent.instance_reportColResult'), dataIndex: 'resultJson', ellipsis: true },
]);

// manager 上报的状态是字符串（running/stopped/error/creating），与列表里 uint status 不同映射。
// 文案走 i18n（与实例 status 同源 key），不再硬编码英文。
// 色词语义对齐 client/device 运行详情：绿=运行、橙红=进行中、灰=停、暗红=错。
const moduleStatusTag = (s?: string) => {
  switch (s) {
    case 'running': return { color: 'magenta', text: $t('agent.instance_status_running') };
    case 'stopped': return { color: 'default', text: $t('agent.instance_status_stopped') };
    case 'error': return { color: 'red', text: $t('agent.instance_status_error') };
    case 'creating': return { color: 'warning', text: $t('agent.instance_status_creating') };
    default: return { color: 'default', text: s || '-' };
  }
};

// 命令状态码 → 文案/颜色（0=待执行 2=执行中 3=已完成 4=失败；注意无 1）。
// pending=灰（对齐 device：仅 running 用蓝），避免与执行中同蓝分不清。
const commandStatusTag = (s?: number) => {
  switch (s) {
    case 0: return { color: 'default', text: $t('agent.instance_cmd_status_pending') };
    case 2: return { color: 'blue', text: $t('agent.instance_cmd_status_running') };
    case 3: return { color: 'magenta', text: $t('agent.instance_cmd_status_done') };
    case 4: return { color: 'red', text: $t('agent.instance_cmd_status_failed') };
    default: return { color: 'default', text: s === undefined || s === null ? '-' : String(s) };
  }
};

// 运行时模块状态码 → 文案/颜色（与实例 status 同码：0=创建中 1=运行中 2=已停止 3=错误）。
const runtimeStatusTag = (s?: number) => {
  switch (s) {
    case 0: return { color: 'warning', text: $t('agent.instance_status_creating') };
    case 1: return { color: 'magenta', text: $t('agent.instance_status_running') };
    case 2: return { color: 'default', text: $t('agent.instance_status_stopped') };
    case 3: return { color: 'red', text: $t('agent.instance_status_error') };
    default: return { color: 'default', text: s === undefined || s === null ? '-' : String(s) };
  }
};

function fmtUptime(sec?: number) {
  if (!sec || sec <= 0) return '-';
  const s = Math.floor(sec % 60);
  const m = Math.floor((sec / 60) % 60);
  const h = Math.floor((sec / 3600) % 24);
  const d = Math.floor(sec / 86_400);
  const parts: string[] = [];
  if (d) parts.push(`${d}d`);
  if (h) parts.push(`${h}h`);
  if (m) parts.push(`${m}m`);
  parts.push(`${s}s`);
  return parts.slice(0, 3).join(' ');
}

// summaryJson 是后端 jsonb 直出的 JSON 字符串（modules 原样），需前端解析成模块列表。
function parseModules(summaryJson?: string): any[] {
  if (!summaryJson) return [];
  try {
    const mods = JSON.parse(summaryJson) as Record<string, any>;
    return Object.entries(mods).map(([name, m]) => ({ name, ...(m || {}) }));
  } catch {
    return [];
  }
}

// 活跃判定：最近上报在 60s 内（manager 心跳默认 15s）视为在线。
function isFresh(ts?: string): boolean {
  if (!ts) return false;
  const t = new Date(ts).getTime();
  if (!Number.isFinite(t)) return false;
  return Date.now() - t < 60_000;
}

// reportInstanceId 记录当前打开上报弹窗的实例，供「刷新」按需重拉（不切 tab、不全量重置）。
// reportInstanceAgent 同行记下实例的 agent 名——历史时间线「智能体进程」列标题随实例 agent 显示。
const reportInstanceId = ref('');
const reportInstanceAgent = ref('');

// 各 tab 数据懒加载：只拉当前展示的 tab，未访问的 tab 不预拉（避免开窗即 Promise.all 拉 4 份）。
// loadedTabs 记录本会话已加载的 tab，切回不重复拉（要取最新用「刷新」按钮，只刷当前 tab）。
const loadedTabs = new Set<string>();

// 运行详情各 tab 模块卡片折叠状态（对齐 client/device 的 sec 折叠：标题行点击收起，localStorage 记忆）。
const REPORT_SEC_COLLAPSE_KEY = 'aa.agent.instance.reportSecs';
function loadReportSecs(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(REPORT_SEC_COLLAPSE_KEY) ?? '{}');
  } catch {
    return {};
  }
}
const reportSecs = reactive<Record<string, boolean>>(loadReportSecs());
function toggleReportSec(key: string) {
  reportSecs[key] = !reportSecs[key];
  localStorage.setItem(REPORT_SEC_COLLAPSE_KEY, JSON.stringify(reportSecs));
}

// loadReportTab 拉取并填充【单个 tab】的数据。tab: current/history/config/commands。
async function loadReportTab(tab: string) {
  const id = reportInstanceId.value;
  if (!id) return;
  reportLoading.value = true;
  try {
    switch (tab) {
      case 'current': {
        const status = await getAgentRuntimeStatusApi(id);
        reportLastReported.value = status?.lastReportedAt ?? '';
        reportFresh.value = isFresh(status?.lastReportedAt);
        reportModules.value = parseModules(status?.summaryJson);
        break;
      }
      case 'history':
        reportHistory.value = (await getAgentRuntimeStatusHistoryApi(id)) ?? [];
        break;
      case 'config':
        reportConfigChanges.value = (await getAgentInstanceConfigChangesApi(id)) ?? [];
        break;
      case 'commands':
        reportCommands.value = (await getAgentInstanceCommandsApi(id))?.list ?? [];
        break;
      case 'workspace':
        await loadWorkspace();
        break;
    }
    loadedTabs.add(tab);
  } catch {
    /* 拦截器已提示；保留空表 */
  } finally {
    reportLoading.value = false;
  }
}

// 切 tab（仅用户点击触发，程序化置值不发 change）：回到第 1 页；该 tab 本会话未加载过才懒拉。
function onReportTabChange(tab: string) {
  reportPage.value = 1;
  if (!loadedTabs.has(tab)) {
    loadReportTab(tab);
  }
}

// ===== 工作区文件面板（实例容器内挂载目录；path 相对工作区根，''=根） =====
const wsPath = ref('');
const wsEntries = ref<AgentWorkspaceApi.EntryItem[]>([]);
const wsLoading = ref(false);

const wsColumns = computed(() => [
  { dataIndex: 'name', title: $t('agent.ws_colName'), minWidth: 220 },
  { dataIndex: 'type', title: $t('agent.ws_colType'), width: 90 },
  { dataIndex: 'size', title: $t('agent.ws_colSize'), width: 100 },
  { dataIndex: 'modifiedAt', title: $t('agent.ws_colModified'), width: 170 },
  { key: 'wsAction', title: $t('agent.ws_colAction'), width: 200 },
]);

async function loadWorkspace(path = wsPath.value) {
  const id = reportInstanceId.value;
  if (!id) return;
  wsLoading.value = true;
  try {
    const res = await getAgentWorkspaceListApi(id, path);
    wsPath.value = res?.path ?? path;
    wsEntries.value = res?.list ?? [];
  } catch {
    wsEntries.value = [];
  } finally {
    wsLoading.value = false;
  }
}

/** 面包屑段（含根），点击回跳 */
const wsCrumbs = computed(() => {
  const crumbs = [{ name: '/', path: '' }];
  let acc = '';
  for (const seg of wsPath.value.split('/').filter(Boolean)) {
    acc = acc ? `${acc}/${seg}` : seg;
    crumbs.push({ name: seg, path: acc });
  }
  return crumbs;
});

function wsEnter(entry: AgentWorkspaceApi.EntryItem) {
  if (entry.type === 'dir') {
    loadWorkspace(entry.path);
  }
}

function fmtSize(bytes?: number) {
  if (!Number.isFinite(bytes) || !bytes) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  let v = bytes as number;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)}${units[i]}`;
}

function fmtWsTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

// 新建目录 / 重命名（共用一个输入弹窗）
const wsMkdirOpen = ref(false);
const wsMkdirName = ref('');
async function onWsMkdir() {
  const name = wsMkdirName.value.trim();
  if (!name) return;
  const target = wsPath.value ? `${wsPath.value}/${name}` : name;
  try {
    await mkdirAgentWorkspaceApi(reportInstanceId.value, target);
    message.success($t('agent.ws_mkdirSuccess'));
    wsMkdirOpen.value = false;
    wsMkdirName.value = '';
    await loadWorkspace();
  } catch {
    /* 拦截器已提示 */
  }
}

const wsRenameOpen = ref(false);
const wsRenameFrom = ref('');
const wsRenameName = ref('');
function openWsRename(entry: AgentWorkspaceApi.EntryItem) {
  wsRenameFrom.value = entry.path;
  wsRenameName.value = entry.name;
  wsRenameOpen.value = true;
}
async function onWsRename() {
  const name = wsRenameName.value.trim();
  if (!name || !wsRenameFrom.value) return;
  const to = wsRenameFrom.value.split('/').slice(0, -1).concat(name).join('/');
  try {
    await renameAgentWorkspaceApi(reportInstanceId.value, wsRenameFrom.value, to);
    message.success($t('agent.ws_renameSuccess'));
    wsRenameOpen.value = false;
    await loadWorkspace();
  } catch {
    /* 拦截器已提示 */
  }
}

// 删除（目录递归）
function onWsDelete(entry: AgentWorkspaceApi.EntryItem) {
  Modal.confirm({
    title: $t('agent.ws_deleteTitle'),
    content: $t('agent.ws_deleteContent', { name: entry.name }),
    okText: $t('agent.team_confirm'),
    cancelText: $t('agent.team_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteAgentWorkspaceApi(reportInstanceId.value, [entry.path]);
      message.success($t('agent.ws_deleteSuccess'));
      await loadWorkspace();
    },
  });
}

// 上传（同名覆盖，成功后刷新列表）
async function onWsUpload(file: File) {
  try {
    await uploadAgentWorkspaceApi(reportInstanceId.value, file, wsPath.value);
    message.success($t('agent.ws_uploadSuccess'));
    await loadWorkspace();
  } catch {
    /* 拦截器已提示 */
  }
  return false; // 阻断 antd Upload 自动上传，全部走自定义接口
}

async function onWsDownload(entry: AgentWorkspaceApi.EntryItem) {
  try {
    await downloadAgentWorkspaceApi(
      reportInstanceId.value,
      entry.path,
      entry.name,
    );
  } catch {
    /* 拦截器已提示 */
  }
}

async function openReport(row: any) {
  reportTitle.value = `${$t('agent.instance_runtimeBtn')} · ${row.name}`;
  reportInstanceId.value = row.instanceId;
  reportInstanceAgent.value = row.agent || '';
  reportVisible.value = true;
  reportTab.value = 'current';
  reportModules.value = [];
  reportHistory.value = [];
  reportConfigChanges.value = [];
  reportCommands.value = [];
  reportLastReported.value = '';
  reportFresh.value = false;
  reportTimeRange.value = undefined;
  reportPage.value = 1;
  selectedChangeIds.value = [];
  wsPath.value = '';
  wsEntries.value = [];
  loadedTabs.clear();
  await loadReportTab('current');
}

// ===== 技能管理抽屉：列出实例已安装 skill（可卸载）+ 搜索 aa_skill 库安装 =====
// 安装/卸载走命令队列（install_skill/uninstall_skill）：manager 收到 install_skill 后用 https 取包
// 链接自行下载解压、openclaw skills install；卸载则定位 openclaw 托管目录删除。
const skillVisible = ref(false);
const skillTitle = ref('');
const skillInstanceId = ref('');
const installedSkills = ref<InstanceSkillItem[]>([]);
const instanceCommands = ref<any[]>([]); // 近期命令（用于判定「安装中」）
const skillLoading = ref(false);
const skillSearchKey = ref('');
const skillResults = ref<SkillApi.ListItem[]>([]);
const skillSearching = ref(false);
const skillOpId = ref(''); // 正在安装/卸载的 skillId（按钮 loading）

// MCP 工具绑定抽屉（openclaw/hermes 实例）：绑分组 → 后台自动签 key + 重推配置注入 mcp 段
// （openclaw mcp.servers / hermes config.yaml mcp_servers）。
const mcpVisible = ref(false);
const mcpInstanceId = ref('');
const mcpInstanceName = ref('');

function openMcpManage(row: any) {
  mcpInstanceId.value = row.instanceId;
  mcpInstanceName.value = row.name || '';
  mcpVisible.value = true;
}

// 已装 skillId 集合：搜索结果里命中则禁用「安装」。
const installedSkillIds = computed(() => new Set(installedSkills.value.map((s) => s.skillId)));

// 安装中 skillId 集合：存在非终态（0 pending / 2 running）的 install_skill 命令。
// 关联表在命令 done 才落（方案 B），故安装进行中时只能从命令历史判定，避免重复点安装。
const installingSkillIds = computed(() => {
  const set = new Set<string>();
  for (const c of instanceCommands.value) {
    if (c?.commandType !== 'install_skill') continue;
    if (c.status === 0 || c.status === 2) {
      const id = cmdSkillId(c?.payloadJson);
      if (id) set.add(id);
    }
  }
  return set;
});

// 卸载中 skillId 集合：存在非终态 uninstall_skill 命令。关联在 done 才删（B），故卸载进行中
// 时 skill 仍在已装列表，需据此标「卸载中」并禁用按钮，避免重复点卸载。
const uninstallingSkillIds = computed(() => {
  const set = new Set<string>();
  for (const c of instanceCommands.value) {
    if (c?.commandType !== 'uninstall_skill') continue;
    if (c.status === 0 || c.status === 2) {
      const id = cmdSkillId(c?.payloadJson);
      if (id) set.add(id);
    }
  }
  return set;
});

// cmdSkillId 从命令 payloadJson（字符串）解析 skillId。
function cmdSkillId(payloadJson: any): string {
  if (!payloadJson) return '';
  try {
    const p = typeof payloadJson === 'string' ? JSON.parse(payloadJson) : payloadJson;
    return p?.skillId ?? '';
  } catch {
    return '';
  }
}

// 待安装列表：已装的从搜索结果里剔除（已装的不应再出现在「待安装」）。安装中的保留并标「安装中」。
const installableResults = computed(() => skillResults.value.filter((s) => !installedSkillIds.value.has(s.skillId)));

// 技能详情弹窗：点击技能名查看（已装/待装均可）。getSkillApi 返回完整 ListItem。
const skillDetailVisible = ref(false);
const skillDetailLoading = ref(false);
const skillDetail = ref<SkillApi.ListItem | null>(null);
const skillDetailInstalledAt = ref('');

async function openSkillDetail(skillId: string, installedAt?: string) {
  skillDetailInstalledAt.value = installedAt ?? '';
  skillDetailVisible.value = true;
  skillDetailLoading.value = true;
  skillDetail.value = null;
  try {
    skillDetail.value = await getSkillApi(skillId);
  } catch {
    /* 拦截器已提示 */
  } finally {
    skillDetailLoading.value = false;
  }
}

async function loadInstalledSkills() {
  if (!skillInstanceId.value) return;
  skillLoading.value = true;
  try {
    installedSkills.value = await getAgentInstanceSkillsApi(skillInstanceId.value);
  } catch {
    /* 拦截器已提示 */
  } finally {
    skillLoading.value = false;
  }
}

// 拉近期命令，用于判定「安装中」（非终态 install_skill）。轻量，不做 loading 遮罩。
async function loadInstanceCommands() {
  if (!skillInstanceId.value) return;
  try {
    const res = await getAgentInstanceCommandsApi(skillInstanceId.value);
    instanceCommands.value = res.list ?? [];
  } catch {
    /* 拦截器已提示 */
  }
}

async function searchSkills() {
  skillSearching.value = true;
  try {
    const res = await getSkillListApi({ pageNum: 1, pageSize: 50, keyWords: skillSearchKey.value, status: 1 });
    skillResults.value = res.list ?? [];
  } catch {
    /* 拦截器已提示 */
  } finally {
    skillSearching.value = false;
  }
}

// 抽屉打开时轮询：有「安装中」skill 时定期刷新（已装列表 + 命令），让「安装中」→「已安装」自动转场。
let skillPollTimer: ReturnType<typeof setInterval> | undefined;
function stopSkillPoll() {
  if (skillPollTimer) {
    clearInterval(skillPollTimer);
    skillPollTimer = undefined;
  }
}
function startSkillPoll() {
  stopSkillPoll();
  skillPollTimer = setInterval(async () => {
    if (!skillVisible.value) {
      stopSkillPoll();
      return;
    }
    await Promise.all([loadInstalledSkills(), loadInstanceCommands()]);
  }, 4000);
}

// 抽屉关闭 / 组件卸载即停轮询，避免泄漏。
watch(skillVisible, (v) => { if (!v) stopSkillPoll(); });
onUnmounted(stopSkillPoll);

async function openSkillManage(row: any) {
  skillTitle.value = `${$t('agent.instance_skillManage')} · ${row.name}`;
  skillInstanceId.value = row.instanceId;
  skillVisible.value = true;
  installedSkills.value = [];
  instanceCommands.value = [];
  skillSearchKey.value = '';
  skillResults.value = [];
  skillOpId.value = '';
  pickVisible.value = false;
  pickVersions.value = [];
  pickVersionId.value = '';
  await Promise.all([loadInstalledSkills(), loadInstanceCommands()]);
  await searchSkills(); // 默认拉一批可装 skill
  startSkillPoll();
}

async function onInstallSkill(skill: SkillApi.ListItem) {
  // 有多个可选版本时弹选择框（默认最新，可指定旧版=回滚）；仅一个版本直接装。
  const versions = await loadSkillVersions(skill.skillId);
  if (versions.length > 1) {
    openVersionPick(skill.skillId, skill.displayName || skill.slug);
    return;
  }
  await doInstallSkill(skill.skillId, '');
}

async function doInstallSkill(skillId: string, versionId: string) {
  skillOpId.value = skillId;
  try {
    await installAgentInstanceSkillApi(skillInstanceId.value, skillId, versionId);
    message.success($t('agent.instance_skillInstallSent'));
    // 立即刷新命令：新落的 install_skill(pending) 让该 skill 转为「安装中」、按钮禁用。
    await loadInstanceCommands();
  } catch {
    /* 拦截器已提示 */
  } finally {
    skillOpId.value = '';
  }
}

// ===== 版本选择弹窗（安装指定旧版=回滚 / 升级到库最新或任意版本） =====
// 升级/回滚同一条 install_skill 管线：带显式 versionId，manager 挪走旧目录重装、失败挪回。
const pickVisible = ref(false);
const pickSkillId = ref('');
const pickName = ref('');
const pickVersions = ref<SkillApi.VersionItem[]>([]);
const pickLoading = ref(false);
const pickVersionId = ref(''); // ''=最新（服务端解析，同「库最新」口径）
const pickInstalling = ref(false);
const pickInstalledVersion = ref(''); // 升级场景展示「已装 x」（安装场景为空）

async function loadSkillVersions(skillId: string): Promise<SkillApi.VersionItem[]> {
  try {
    const res = await getSkillVersionsApi(skillId);
    return (res?.list ?? []).filter((v) => v.status !== 2); // 跳过已 yanked；草稿可装
  } catch {
    return [];
  }
}

function openVersionPick(skillId: string, name: string, installedVersion = '') {
  pickSkillId.value = skillId;
  pickName.value = name;
  pickInstalledVersion.value = installedVersion;
  pickVersionId.value = '';
  pickVisible.value = true;
  pickLoading.value = true;
  void loadSkillVersions(skillId).then((list) => {
    pickVersions.value = list;
    pickLoading.value = false;
  });
}

async function confirmPickInstall() {
  if (!pickSkillId.value) return;
  pickInstalling.value = true;
  try {
    await installAgentInstanceSkillApi(
      skillInstanceId.value,
      pickSkillId.value,
      pickVersionId.value,
    );
    message.success($t('agent.instance_skillInstallSent'));
    pickVisible.value = false;
    await loadInstanceCommands();
  } catch {
    /* 拦截器已提示 */
  } finally {
    pickInstalling.value = false;
  }
}

// 行内升级：已装版本 id ≠ 库最新（或历史行版本未知）→ 弹版本选择（默认最新；选旧版=回滚）。
function skillUpgradable(item: InstanceSkillItem) {
  return !!item.latestVersionId && item.latestVersionId !== item.versionId;
}

function onUpgradeSkill(item: InstanceSkillItem) {
  openVersionPick(item.skillId, item.displayName || item.slug, item.version || '');
}

function onUninstallSkill(item: InstanceSkillItem) {
  Modal.confirm({
    title: $t('agent.instance_skillUninstallTitle'),
    content: $t('agent.instance_skillUninstallConfirm', { name: item.displayName || item.slug }),
    okType: 'danger',
    okText: $t('agent.instance_skillUninstall'),
    cancelText: $t('agent.instance_cancel'),
    onOk: async () => {
      skillOpId.value = item.skillId;
      try {
        await uninstallAgentInstanceSkillApi(skillInstanceId.value, item.skillId);
        message.success($t('agent.instance_skillUninstallSent'));
        // 立即刷新命令：新落的 uninstall_skill(pending) 让该 skill 转为「卸载中」、按钮禁用。
        await loadInstanceCommands();
      } catch {
        /* 拦截器已提示 */
      } finally {
        skillOpId.value = '';
      }
    },
  });
}

// 一键刷新：只重拉【当前 tab】的数据（不全量重置、不拉其它 tab）。
async function refreshReport() {
  await loadReportTab(reportTab.value);
}

// 对比：取选中的 2 条快照，按 changedAt 升序（早=左/旧，晚=右/新）后打开 diff 弹窗。
// reportConfigChanges 全量在前端，跨页选中的 id 都能命中。
function openDiff() {
  if (selectedChangeIds.value.length !== 2) return;
  const byId = new Map(reportConfigChanges.value.map((c: any) => [c.changeId, c]));
  const picks = selectedChangeIds.value.map((id) => byId.get(id)).filter(Boolean) as any[];
  if (picks.length !== 2) return;
  picks.sort((a, b) => String(a.changedAt || '').localeCompare(String(b.changedAt || '')));
  diffLeft.value = picks[0];
  diffRight.value = picks[1];
  diffVisible.value = true;
}

// 桌面预览：页内弹窗内嵌 iframe。走后端同源反代（直连容器端口会触发混合内容/自签证书）：
// 先签短期路径 token，拿到同源代理 URL 再加载 iframe。
const desktopVisible = ref(false);
const desktopUrl = ref('');
const desktopTitle = ref('');
async function openDesktop(row: any) {
  try {
    const res = await requestDesktopAccessApi(row.instanceId);
    if (!res?.url) {
      message.warning($t('agent.instance_loadFailed'));
      return;
    }
    desktopUrl.value = res.url;
    desktopTitle.value = `${$t('agent.instance_openDesktop')} · ${row.name}`;
    desktopVisible.value = true;
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 绑定社交软件弹窗（微信走 channel_login 命令 + 进度轮询渲染二维码） =====
// QR 流转：enqueue channel_login → manager 阻塞跑渠道登录抓 stdout QR 写命令进度
// → progress_json.step → 这里轮询 getAgentInstanceCommandsApi 读 step 渲染二维码 → 扫码完成 status=3。
// openclaw 走 `channels login`（openclaw-weixin）；hermes 走 Python qr_login shim（weixin），
// 渠道名/agentId 按 bindAgent 区分（见 bindChannel）。
const bindVisible = ref(false);
const bindInstanceId = ref('');
const bindTab = ref('wechat');
// 绑定目标实例的 agent（决定微信渠道名：openclaw→openclaw-weixin，hermes→weixin）。
const bindAgent = ref('openclaw');
// bindChannel 按 agent 给渠道标识：openclaw 用 openclaw-weixin，hermes 用 weixin。
function bindChannel() {
  return bindAgent.value === 'hermes' ? 'weixin' : 'openclaw-weixin';
}
// 微信绑定态：querying(查登录态) / bound(已绑定) / idle(未绑定,可绑) / loading(发起扫码) / qr(扫码中) / success / unbinding(解绑中) / failed
const wechatStatus = ref<'bound' | 'failed' | 'idle' | 'loading' | 'qr' | 'querying' | 'success' | 'unbinding'>('idle');
const wechatQrSrc = ref('');
const wechatMessage = ref('');
const wechatErrText = ref('');
const bindAlias = ref('');
const bindAccountId = ref('');
let bindPollTimer: ReturnType<typeof setInterval> | null = null;
let bindPollCmdId = '';
let bindEnqueuedAt = 0;

function stopBindPoll() {
  if (bindPollTimer) {
    clearInterval(bindPollTimer);
    bindPollTimer = null;
  }
  bindPollCmdId = '';
}

// 通用命令轮询：下发后轮询 getAgentInstanceCommandsApi，done(3)→onDone(resultJson)、failed(4)→onFailed(errText)；
// 每轮调 onProgress(cmd)（login 用它抓 QR/超时提示）。status/logout 不传 onProgress。
function startBindPoll(
  cmdId: string,
  opts: {
    onDone: (resultJson: string) => void;
    onFailed: (errText: string) => void;
    onProgress?: (cmd: any) => void;
  },
) {
  stopBindPoll();
  bindPollCmdId = cmdId;
  bindEnqueuedAt = Date.now();
  bindPollTimer = setInterval(async () => {
    if (!bindInstanceId.value || !bindPollCmdId) return;
    let res: any;
    try {
      res = await getAgentInstanceCommandsApi(bindInstanceId.value);
    } catch {
      return;
    }
    const cmd = (res?.list ?? []).find((c: any) => c.instanceCommandId === bindPollCmdId);
    if (!cmd) return;
    if (opts.onProgress) opts.onProgress(cmd);
    const status: number = cmd.status;
    if (status === 3) {
      stopBindPoll();
      opts.onDone(cmd.resultJson || '');
    } else if (status === 4) {
      stopBindPoll();
      opts.onFailed(parseFailText(cmd.resultJson) || $t('agent.instance_bindFailed'));
    }
  }, 1500);
}

// progress_json 形如 {"step":"<qrUrl>"}；取 step。
function parseProgressStep(progressJson?: string): string {
  if (!progressJson) return '';
  try {
    return (JSON.parse(progressJson) as any)?.step || '';
  } catch {
    return '';
  }
}

// 从 result_json（failed 时并入的 errMsg / 执行过程）取失败提示。
function parseFailText(resultJson?: string): string {
  if (!resultJson) return '';
  try {
    const r = JSON.parse(resultJson) as any;
    return r?.error || r?.errMsg || r?.message || '';
  } catch {
    return '';
  }
}

async function renderQr(raw: string) {
  if (!raw) {
    wechatQrSrc.value = '';
    return;
  }
  if (/^data:image\//i.test(raw)) {
    wechatQrSrc.value = raw; // 已是 data URL，直通
    return;
  }
  try {
    wechatQrSrc.value = await QRCode.toDataURL(raw, { width: 240, margin: 1 });
  } catch {
    wechatQrSrc.value = '';
  }
}

async function startWechatLogin() {
  if (!bindInstanceId.value) return;
  stopBindPoll();
  wechatStatus.value = 'loading';
  wechatQrSrc.value = '';
  wechatMessage.value = $t('agent.instance_bindLoading');
  wechatErrText.value = '';
  let cmdId = '';
  try {
    const res = await enqueueAgentInstanceCommandApi(bindInstanceId.value, {
      commandType: 'channel_login',
      agentId: bindAgent.value,
      payload: { channel: bindChannel() },
      timeoutSeconds: 300,
    });
    cmdId = res?.commandId || '';
  } catch {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindEnqueueFailed');
    return;
  }
  if (!cmdId) {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindEnqueueFailed');
    return;
  }
  startBindPoll(cmdId, {
    onDone: () => {
      wechatStatus.value = 'success';
      wechatQrSrc.value = '';
      wechatMessage.value = $t('agent.instance_bindSuccess');
    },
    onFailed: (e) => {
      wechatStatus.value = 'failed';
      wechatErrText.value = e || $t('agent.instance_bindFailed');
    },
    onProgress: (cmd) => {
      const step = parseProgressStep(cmd.progressJson);
      if (step) {
        if (/^(https?:\/\/|data:image\/|weixin:\/\/)/i.test(step)) {
          renderQr(step);
          if (wechatStatus.value !== 'success' && wechatStatus.value !== 'failed') {
            wechatStatus.value = 'qr';
          }
        } else if (wechatStatus.value === 'loading') {
          wechatMessage.value = step; // 非二维码的状态文案（如「发起微信扫码登录…」）
        }
      } else if (
        cmd.status === 0 &&
        Date.now() - bindEnqueuedAt > 20_000 &&
        wechatStatus.value === 'loading'
      ) {
        // pending 超 20s：实例迟迟未认领（多半没在跑/未上报）。
        wechatMessage.value = $t('agent.instance_bindPending');
      }
    },
  });
}

// 查询渠道登录态：下发 channel_status → 轮询 → manager 结构化结果 {bound,alias,accountId,...}
// bound=true 进 bound 态（展示账号+解绑），否则进 idle 态（可绑定）。
async function queryChannelStatus() {
  if (!bindInstanceId.value) return;
  stopBindPoll();
  wechatStatus.value = 'querying';
  wechatErrText.value = '';
  let cmdId = '';
  try {
    const res = await enqueueAgentInstanceCommandApi(bindInstanceId.value, {
      commandType: 'channel_status',
      agentId: bindAgent.value,
      payload: { channel: bindChannel() },
      timeoutSeconds: 30,
    });
    cmdId = res?.commandId || '';
  } catch {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindQueryFailed');
    return;
  }
  if (!cmdId) {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindQueryFailed');
    return;
  }
  startBindPoll(cmdId, {
    onDone: (r) => {
      let info: any = {};
      try {
        info = JSON.parse(r);
      } catch {
        info = {};
      }
      if (info.bound) {
        bindAlias.value = info.alias || '';
        bindAccountId.value = info.accountId || '';
        wechatStatus.value = 'bound';
      } else {
        wechatStatus.value = 'idle';
      }
    },
    onFailed: (e) => {
      wechatStatus.value = 'failed';
      wechatErrText.value = e || $t('agent.instance_bindQueryFailed');
    },
  });
}

// 解绑：下发 channel_logout（清账号 token）→ 轮询完成 → 复查 channel_status 回到 idle（可重绑）。
async function startUnbind() {
  if (!bindInstanceId.value) return;
  stopBindPoll();
  wechatStatus.value = 'unbinding';
  wechatErrText.value = '';
  let cmdId = '';
  try {
    const res = await enqueueAgentInstanceCommandApi(bindInstanceId.value, {
      commandType: 'channel_logout',
      agentId: bindAgent.value,
      payload: { channel: bindChannel() },
      timeoutSeconds: 30,
    });
    cmdId = res?.commandId || '';
  } catch {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindUnbindFailed');
    return;
  }
  if (!cmdId) {
    wechatStatus.value = 'failed';
    wechatErrText.value = $t('agent.instance_bindUnbindFailed');
    return;
  }
  startBindPoll(cmdId, {
    onDone: () => {
      // channel_logout 自包含：清 .env 凭据 + 重生成 config + 内部已 restart 丢弃微信连接，
      // 无需再补 restart（曾与 channel_status 并发挤进队列，忙窗口吃掉 status 命令 → 30s 误判超时）。
      message.success($t('agent.instance_bindUnbindSuccess'));
      queryChannelStatus();
    },
    onFailed: (e) => {
      wechatStatus.value = 'failed';
      wechatErrText.value = e || $t('agent.instance_bindUnbindFailed');
    },
  });
}

function openBindSocial(row: any) {
  bindInstanceId.value = row.instanceId;
  bindAgent.value = row.agent || 'openclaw'; // 决定微信渠道名（openclaw-weixin / weixin）
  bindTab.value = 'wechat';
  wechatStatus.value = 'querying';
  wechatQrSrc.value = '';
  wechatMessage.value = '';
  wechatErrText.value = '';
  bindAlias.value = '';
  bindAccountId.value = '';
  bindVisible.value = true;
  queryChannelStatus();
}

// 切离微信 tab 停轮询（切回不自动重启，避免重复下发；用户可在微信 tab 点重试）。
function onBindTabChange(key: string) {
  if (key !== 'wechat') stopBindPoll();
}

function closeBind() {
  stopBindPoll();
}

onUnmounted(stopBindPoll);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('agent.instance_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('agent.instance_batchDelete') }}
        </Button>
      </template>
      <template #status="{ row }">
        <span :class="['aa-enum', `aa-enum--${statusTag(row.status).color}`]">{{ statusTag(row.status).text }}</span>
      </template>
      <template #containerId="{ row }">
        <span class="font-mono text-xs">{{ row.containerId ? row.containerId.slice(0, 12) : '-' }}</span>
      </template>
      <template #llmKeyHash="{ row }">
        <span v-if="row.llmKeyHash" class="font-mono text-xs" :title="row.llmKeyHash">{{ row.llmKeyHash.slice(0, 12) }}…</span>
        <span v-else class="text-xs text-gray-400">-</span>
      </template>
      <template #accessUrl="{ row }">
        <a v-if="row.accessUrl" :href="row.accessUrl" target="_blank" class="font-mono text-xs text-primary">
          {{ row.accessUrl.replace(/^https?:\/\//, '') }}
        </a>
        <span v-else class="text-xs text-gray-400">-</span>
      </template>
      <template #action="{ row }">
        <ActionOverflow :items="actionsOf(row)" :max="4" />
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="$t('agent.instance_modalTitleAdd')"
      :width="560"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <Drawer
      v-model:open="reportVisible"
      :title="reportTitle"
      :width="1000"
      :footer="null"
      :destroy-on-close="true"
      class="report-drawer"
    >
      <div class="drawer-body">
      <div class="mb-3 flex items-center gap-3 text-xs text-gray-500">
        <span>{{ $t('agent.instance_reportLast') }}: {{ reportLastReported || '-' }}</span>
        <span :class="['aa-enum', `aa-enum--${reportFresh ? 'magenta' : 'default'}`]">
          {{ reportFresh ? $t('agent.instance_reportFresh') : $t('agent.instance_reportStale') }}
        </span>
        <Button
          size="small"
          type="primary"
          ghost
          :loading="reportLoading"
          class="ml-auto"
          @click="refreshReport"
        >
          {{ $t('agent.instance_reportRefresh') }}
        </Button>
      </div>
      <Tabs v-model:active-key="reportTab" class="report-tabs" @change="onReportTabChange">
        <Tabs.TabPane key="current" :tab="$t('agent.instance_reportTabCurrent')">
          <Card
            size="small"
            :bordered="true"
            :class="['module-card', 'sec-current', { collapsed: reportSecs.current }]"
          >
            <template #title>
              <span class="sec-title" @click="toggleReportSec('current')">
                <span class="sec-toggle">{{ reportSecs.current ? '▸' : '▾' }}</span>
                {{ $t('agent.instance_reportTabCurrent') }}
              </span>
            </template>
          <Table
            :columns="reportColumns"
            :data-source="reportModules"
            :pagination="false"
            :loading="reportLoading"
            :scroll="{ x: 840 }"
            row-key="name"
            size="small"
            bordered
            :locale="{ emptyText: $t('agent.instance_reportEmpty') }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'status'">
                <span :class="['aa-enum', `aa-enum--${moduleStatusTag(record.status).color}`]">
                  {{ moduleStatusTag(record.status).text }}
                </span>
              </template>
              <template v-else-if="column.dataIndex === 'uptimeSeconds'">
                {{ fmtUptime(record.uptimeSeconds) }}
              </template>
              <template v-else-if="column.dataIndex === 'configHash'">
                <div class="font-mono text-xs leading-5">
                  <div v-if="record.configHash">hash: {{ record.configHash.slice(0, 12) }}</div>
                  <div v-if="record.configSize">size: {{ record.configSize }} B</div>
                  <div v-if="record.configMtime">mtime: {{ record.configMtime }}</div>
                  <span
                    v-if="!record.configHash && !record.configSize && !record.configMtime"
                    class="text-gray-400"
                  >-</span>
                </div>
              </template>
              <template v-else-if="column.dataIndex === 'lastError'">
                <span v-if="record.lastError" class="text-xs text-red-500">{{ record.lastError }}</span>
                <span v-else class="text-gray-400">-</span>
              </template>
              <template v-else-if="column.dataIndex === 'pidOrContainerId'">
                <span v-if="record.pidOrContainerId" class="font-mono text-xs">{{ record.pidOrContainerId }}</span>
                <span v-else class="text-gray-400">-</span>
              </template>
            </template>
          </Table>
          </Card>
        </Tabs.TabPane>
        <Tabs.TabPane key="history" :tab="$t('agent.instance_reportTabHistory')">
          <Card
            size="small"
            :bordered="true"
            :class="['module-card', 'sec-history', { collapsed: reportSecs.history }]"
          >
            <template #title>
              <span class="sec-title" @click="toggleReportSec('history')">
                <span class="sec-toggle">{{ reportSecs.history ? '▸' : '▾' }}</span>
                {{ $t('agent.instance_reportTabHistory') }}
              </span>
            </template>
          <ReportTimeFilter v-model="reportTimeRange" />
          <Table
            :columns="historyColumns"
            :data-source="historyFiltered"
            :pagination="reportPagination"
            :loading="reportLoading"
            row-key="historyId"
            size="small"
            bordered
          >
            <template #bodyCell="{ column, record }">
              <template v-if="['infraStatus', 'agentStatus', 'agentProcStatus'].includes(column.dataIndex)">
                <span :class="['aa-enum', `aa-enum--${runtimeStatusTag(record[column.dataIndex]).color}`]">
                  {{ runtimeStatusTag(record[column.dataIndex]).text }}
                </span>
              </template>
            </template>
          </Table>
          </Card>
        </Tabs.TabPane>
        <Tabs.TabPane key="config" :tab="$t('agent.instance_reportTabConfig')">
          <Card
            size="small"
            :bordered="true"
            :class="['module-card', 'sec-config', { collapsed: reportSecs.config }]"
          >
            <template #title>
              <span class="sec-title" @click="toggleReportSec('config')">
                <span class="sec-toggle">{{ reportSecs.config ? '▸' : '▾' }}</span>
                {{ $t('agent.instance_reportTabConfig') }}
              </span>
            </template>
          <ReportTimeFilter v-model="reportTimeRange" />
          <div class="mb-3 flex items-center gap-2">
            <Button
              size="small"
              type="primary"
              ghost
              :disabled="selectedChangeIds.length !== 2"
              @click="openDiff"
            >
              {{ $t('agent.instance_reportCompare') }}
            </Button>
            <span class="text-xs text-gray-400">
              {{ $t('agent.instance_diffSelectTip') }}（{{ selectedChangeIds.length }}/2）
            </span>
          </div>
          <Table
            :columns="configChangeColumns"
            :data-source="configFiltered"
            :pagination="reportPagination"
            :loading="reportLoading"
            :row-selection="configRowSelection"
            row-key="changeId"
            size="small"
            bordered
          >
            <template #expandedRowRender="{ record }">
              <pre class="max-h-96 overflow-auto rounded bg-black/5 p-2 text-xs whitespace-pre-wrap break-all">{{ record.content || $t('agent.instance_reportNoContent') }}</pre>
            </template>
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'source'">
                <span :class="['aa-enum', `aa-enum--${sourceTag(record.source).color}`]">{{ sourceTag(record.source).text }}</span>
              </template>
              <template v-else-if="column.dataIndex === 'dispatcher'">
                <span v-if="record.dispatcherName" class="text-xs">
                  {{ record.dispatcherName }}<span v-if="record.dispatcherEmpNo" class="text-gray-400">（{{ record.dispatcherEmpNo }}）</span>
                </span>
                <span v-else class="text-xs text-gray-400">{{ record.dispatcher || '-' }}</span>
              </template>
            </template>
          </Table>
          </Card>
        </Tabs.TabPane>
        <Tabs.TabPane key="commands" :tab="$t('agent.instance_reportTabCommands')">
          <Card
            size="small"
            :bordered="true"
            :class="['module-card', 'sec-commands', { collapsed: reportSecs.commands }]"
          >
            <template #title>
              <span class="sec-title" @click="toggleReportSec('commands')">
                <span class="sec-toggle">{{ reportSecs.commands ? '▸' : '▾' }}</span>
                {{ $t('agent.instance_reportTabCommands') }}
              </span>
            </template>
          <ReportTimeFilter v-model="reportTimeRange" />
          <Table
            :columns="commandColumns"
            :data-source="commandsFiltered"
            :pagination="reportPagination"
            :loading="reportLoading"
            row-key="instanceCommandId"
            size="small"
            bordered
            :scroll="{ x: 1250 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'status'">
                <span :class="['aa-enum', `aa-enum--${commandStatusTag(record.status).color}`]">
                  {{ commandStatusTag(record.status).text }}
                </span>
              </template>
              <template v-else-if="column.dataIndex === 'agentId'">
                <span v-if="record.agentId" class="font-mono text-xs">{{ record.agentId }}</span>
                <span v-else class="text-gray-400">-</span>
              </template>
              <template v-else-if="column.dataIndex === 'issuedBy'">
                <span v-if="record.issuedByName" class="text-xs">
                  {{ record.issuedByName }}<span v-if="record.issuedByEmpNo" class="text-gray-400">（{{ record.issuedByEmpNo }}）</span>
                </span>
                <span v-else class="text-xs text-gray-400">{{ record.issuedBy || '-' }}</span>
              </template>
            </template>
            <template #expandedRowRender="{ record }">
              <div class="space-y-2 text-xs">
                <div v-if="record.payloadJson">
                  <div class="mb-1 text-gray-500">{{ $t('agent.instance_reportPayloadLabel') }}</div>
                  <pre class="max-h-64 overflow-auto rounded bg-black/5 p-2 whitespace-pre-wrap break-all">{{ record.payloadJson }}</pre>
                </div>
                <div v-if="record.resultJson">
                  <div class="mb-1 text-gray-500">{{ $t('agent.instance_reportResultLabel') }}</div>
                  <pre class="max-h-64 overflow-auto rounded bg-black/5 p-2 whitespace-pre-wrap break-all">{{ record.resultJson }}</pre>
                </div>
                <div v-if="record.progressJson">
                  <div class="mb-1 text-gray-500">{{ $t('agent.instance_reportColProgress') }}</div>
                  <pre class="max-h-40 overflow-auto rounded bg-black/5 p-2 whitespace-pre-wrap break-all">{{ record.progressJson }}</pre>
                </div>
                <div v-if="record.errorMessage" class="text-red-500">
                  {{ $t('agent.instance_reportErrorLabel') }}：{{ record.errorMessage }}
                </div>
              </div>
            </template>
          </Table>
          </Card>
        </Tabs.TabPane>
        <Tabs.TabPane key="workspace" :tab="$t('agent.ws_tab')">
          <Card
            size="small"
            :bordered="true"
            :class="['module-card', 'sec-workspace']"
          >
            <template #title>
              <span class="sec-title">
                {{ $t('agent.ws_sectionTitle') }}
                <span class="section-hint">{{ $t('agent.ws_sectionHint') }}</span>
              </span>
            </template>
            <div class="mb-3 flex flex-wrap items-center gap-2">
              <!-- 面包屑（根 / 各级目录，点击回跳） -->
              <span class="flex min-w-0 flex-1 flex-wrap items-center gap-1 text-sm">
                <template v-for="(c, i) in wsCrumbs" :key="c.path || '/'">
                  <span v-if="i > 0" class="opacity-40">/</span>
                  <a
                    class="cursor-pointer break-all hover:underline"
                    :class="{ 'opacity-60': i < wsCrumbs.length - 1 }"
                    @click="loadWorkspace(c.path)"
                  >{{ c.name }}</a>
                </template>
              </span>
              <Button size="small" @click="wsMkdirOpen = true">
                {{ $t('agent.ws_mkdir') }}
              </Button>
              <Upload
                :show-upload-list="false"
                :before-upload="onWsUpload"
                :disabled="wsLoading"
              >
                <Button size="small">{{ $t('agent.ws_upload') }}</Button>
              </Upload>
              <Button size="small" :loading="wsLoading" @click="loadWorkspace()">
                {{ $t('agent.ws_refresh') }}
              </Button>
            </div>
            <Table
              :columns="wsColumns"
              :data-source="wsEntries"
              :loading="wsLoading"
              :pagination="false"
              row-key="path"
              size="small"
              bordered
              :locale="{ emptyText: $t('agent.ws_empty') }"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.dataIndex === 'name'">
                  <span v-if="record.type === 'dir'" class="mr-1">📁</span>
                  <span v-else class="mr-1">📄</span>
                  <a
                    v-if="record.type === 'dir'"
                    class="cursor-pointer hover:underline"
                    @click="wsEnter(record as AgentWorkspaceApi.EntryItem)"
                  >{{ record.name }}</a>
                  <span v-else>{{ record.name }}</span>
                </template>
                <template v-else-if="column.dataIndex === 'size'">
                  {{ record.type === 'dir' ? '-' : fmtSize(record.size) }}
                </template>
                <template v-else-if="column.dataIndex === 'type'">
                  {{ record.type === 'dir' ? $t('agent.ws_typeDir') : $t('agent.ws_typeFile') }}
                </template>
                <template v-else-if="column.dataIndex === 'modifiedAt'">
                  {{ fmtWsTime(record.modifiedAt) }}
                </template>
                <template v-else-if="column.key === 'wsAction'">
                  <Button
                    v-if="record.type === 'file'"
                    type="link"
                    size="small"
                    @click="onWsDownload(record as AgentWorkspaceApi.EntryItem)"
                  >
                    {{ $t('agent.ws_download') }}
                  </Button>
                  <Button type="link" size="small" @click="openWsRename(record as AgentWorkspaceApi.EntryItem)">
                    {{ $t('agent.ws_rename') }}
                  </Button>
                  <Button type="link" size="small" danger @click="onWsDelete(record as AgentWorkspaceApi.EntryItem)">
                    {{ $t('agent.ws_delete') }}
                  </Button>
                </template>
              </template>
            </Table>
          </Card>
        </Tabs.TabPane>
      </Tabs>
      </div>
    </Drawer>

    <!-- 工作区：新建目录 -->
    <Modal
      v-model:open="wsMkdirOpen"
      :title="$t('agent.ws_mkdirTitle')"
      :width="440"
      @ok="onWsMkdir"
    >
      <div class="py-2">
        <div class="mb-1 text-xs opacity-60">{{ $t('agent.ws_mkdirParent') }}: /{{ wsPath }}</div>
        <Input v-model:value="wsMkdirName" :placeholder="$t('agent.ws_mkdirPlaceholder')" @press-enter="onWsMkdir" />
      </div>
    </Modal>

    <!-- 工作区：重命名 -->
    <Modal
      v-model:open="wsRenameOpen"
      :title="$t('agent.ws_renameTitle')"
      :width="440"
      @ok="onWsRename"
    >
      <div class="py-2">
        <Input v-model:value="wsRenameName" :placeholder="$t('agent.ws_renamePlaceholder')" @press-enter="onWsRename" />
      </div>
    </Modal>

    <!-- 技能管理：已安装（可卸载）+ 搜索 aa_skill 库安装（下发 install_skill/uninstall_skill 命令） -->
    <Drawer
      v-model:open="skillVisible"
      :title="skillTitle"
      :width="900"
      :footer="null"
      :destroy-on-close="true"
    >
      <!-- 已安装 -->
      <div class="mb-2 flex items-center">
        <span class="text-base font-medium">{{ $t('agent.instance_skillInstalled') }}</span>
        <Button
          size="small"
          type="primary"
          ghost
          :loading="skillLoading"
          class="ml-auto"
          @click="loadInstalledSkills"
        >
          {{ $t('agent.instance_reportRefresh') }}
        </Button>
      </div>
      <Spin :spinning="skillLoading">
        <div
          v-if="installedSkills.length === 0"
          class="py-4 text-center text-sm text-gray-400"
        >
          {{ $t('agent.instance_skillInstalledEmpty') }}
        </div>
        <div
          v-for="item in installedSkills"
          :key="item.instanceSkillId"
          class="mb-2 flex cursor-pointer items-start gap-3 rounded border border-solid border-gray-200 p-3 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
          @click="openSkillDetail(item.skillId, item.installedAt)"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-medium text-[var(--primary-color,#42b983)] hover:underline">
                {{ item.displayName || item.slug }}
              </span>
            </div>
            <div v-if="item.summary" class="mt-1 truncate text-xs text-gray-500">
              {{ item.summary }}
            </div>
            <!-- 版本对照：已装 x · 库最新 y（不一致橙色=可升级；已装未知也放升级） -->
            <div
              v-if="item.version || item.latestVersion"
              class="mt-1 flex items-center gap-1 text-xs"
            >
              <span class="text-gray-400">
                {{ $t('agent.instance_skillInstalledVer') }}: {{ item.version || '—' }}
              </span>
              <span
                v-if="item.latestVersion"
                :class="skillUpgradable(item) ? 'font-medium text-amber-500' : 'text-gray-400'"
              >
                · {{ $t('agent.instance_skillLatestVer') }}: {{ item.latestVersion }}
              </span>
            </div>
          </div>
          <Button
            v-if="skillUpgradable(item) && !installingSkillIds.has(item.skillId) && !uninstallingSkillIds.has(item.skillId)"
            type="primary"
            ghost
            size="small"
            class="mr-1"
            :loading="skillOpId === item.skillId"
            @click.stop="onUpgradeSkill(item)"
          >
            {{ $t('agent.instance_skillUpgrade') }}
          </Button>
          <Button
            v-if="uninstallingSkillIds.has(item.skillId)"
            size="small"
            loading
            disabled
          >
            {{ $t('agent.instance_skillUninstalling') }}
          </Button>
          <Button
            v-else-if="installingSkillIds.has(item.skillId)"
            size="small"
            loading
            disabled
          >
            {{ $t('agent.instance_skillInstalling') }}
          </Button>
          <Button
            v-else
            type="primary"
            ghost
            danger
            size="small"
            :loading="skillOpId === item.skillId"
            @click.stop="onUninstallSkill(item)"
          >
            {{ $t('agent.instance_skillUninstall') }}
          </Button>
        </div>
      </Spin>

      <div class="my-4 border-t border-dashed border-gray-300 dark:border-gray-700"></div>

      <!-- 搜索安装 -->
      <div class="mb-2 text-base font-medium">{{ $t('agent.instance_skillSearchTitle') }}</div>
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="skillSearchKey"
          allow-clear
          :placeholder="$t('agent.instance_skillSearchPlaceholder')"
          @press-enter="searchSkills"
        />
        <Button type="primary" :loading="skillSearching" @click="searchSkills">
          {{ $t('agent.instance_skillSearch') }}
        </Button>
      </div>
      <Spin :spinning="skillSearching">
        <div
          v-if="installableResults.length === 0"
          class="py-4 text-center text-sm text-gray-400"
        >
          {{ $t('agent.instance_skillSearchEmpty') }}
        </div>
        <div
          v-for="skill in installableResults"
          :key="skill.skillId"
          class="mb-2 flex cursor-pointer items-start gap-3 rounded border border-solid border-gray-200 p-3 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
          @click="openSkillDetail(skill.skillId)"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-medium text-[var(--primary-color,#42b983)] hover:underline">
                {{ skill.displayName || skill.slug }}
              </span>
            </div>
            <div v-if="skill.summary" class="mt-1 truncate text-xs text-gray-500">
              {{ skill.summary }}
            </div>
          </div>
          <Button
            v-if="installingSkillIds.has(skill.skillId)"
            size="small"
            loading
            disabled
          >
            {{ $t('agent.instance_skillInstalling') }}
          </Button>
          <Button
            v-else
            type="primary"
            ghost
            size="small"
            :loading="skillOpId === skill.skillId"
            @click.stop="onInstallSkill(skill)"
          >
            {{ $t('agent.instance_skillInstall') }}
          </Button>
        </div>
      </Spin>
    </Drawer>

    <!-- MCP 工具绑定：绑分组自动签专属 key + 重推配置注入 mcp 段（共享组件，设备页同款） -->
    <McpBindDrawer
      v-model:open="mcpVisible"
      target-type="instance"
      :target-id="mcpInstanceId"
      :target-name="mcpInstanceName"
    />

    <!-- 版本选择：安装指定版本（旧版=回滚）/ 升级到任意版本，默认最新 -->
    <Modal
      v-model:open="pickVisible"
      :title="$t('agent.instance_skillPickTitle')"
      :width="480"
      :ok-text="$t('agent.instance_skillInstall')"
      :cancel-text="$t('agent.instance_cancel')"
      :confirm-loading="pickInstalling"
      :destroy-on-close="true"
      @ok="confirmPickInstall"
    >
      <div class="mb-3 text-sm">
        <span class="font-medium">{{ pickName }}</span>
        <span v-if="pickInstalledVersion" class="ml-2 text-gray-400">
          {{ $t('agent.instance_skillInstalledVer') }}: {{ pickInstalledVersion || '—' }}
        </span>
      </div>
      <Spin :spinning="pickLoading">
        <Select v-model:value="pickVersionId" class="w-full">
          <Select.Option value="">{{ $t('agent.instance_skillVerLatest') }}</Select.Option>
          <Select.Option
            v-for="v in pickVersions"
            :key="v.versionId"
            :value="v.versionId"
            :disabled="(v.auditStatus ?? 0) !== 1"
          >
            {{ v.version }}{{ v.status === 0 ? $t('agent.instance_skillVerDraft') : '' }}
            <!-- 安全门禁：审计非「安全」禁选（后端下发/取包同拦） -->
            <span v-if="(v.auditStatus ?? 0) !== 1" class="skill-ver-unsafe">
              {{ $t('skill.sec_statusFail') }}
            </span>
          </Select.Option>
        </Select>
        <div class="mt-2 text-xs text-gray-400">
          {{ $t('agent.instance_skillPickHint') }}
        </div>
      </Spin>
    </Modal>

    <!-- 技能详情：点击技能名查看（已装/待装均可） -->
    <Modal
      v-model:open="skillDetailVisible"
      :title="$t('agent.instance_skillDetail')"
      :width="640"
      :footer="null"
      :destroy-on-close="true"
    >
      <Spin :spinning="skillDetailLoading">
        <Descriptions v-if="skillDetail" :column="1" size="small" bordered>
          <DescriptionsItem :label="$t('skill.skill_name')">
            {{ skillDetail.displayName || skillDetail.slug }}
          </DescriptionsItem>
          <DescriptionsItem :label="$t('skill.skill_slug')">{{ skillDetail.slug }}</DescriptionsItem>
          <DescriptionsItem v-if="skillDetail.summary" :label="$t('skill.skill_summary')">
            <span class="whitespace-pre-wrap">{{ skillDetail.summary }}</span>
          </DescriptionsItem>
          <DescriptionsItem :label="$t('skill.skill_owner')">
            {{ skillDetail.ownerName || '-' }}
            <span v-if="skillDetail.ownerEmpNo" class="ml-1 text-gray-400">({{ skillDetail.ownerEmpNo }})</span>
          </DescriptionsItem>
          <DescriptionsItem :label="$t('skill.skill_visibility')">
            {{ skillDetail.visibility === 0 ? $t('skill.skill_visibilityPublic') : $t('skill.skill_visibilityPrivate') }}
          </DescriptionsItem>
          <DescriptionsItem :label="$t('skill.skill_download')">{{ skillDetail.downloadCount ?? 0 }}</DescriptionsItem>
          <DescriptionsItem :label="$t('skill.skill_createdAt')">{{ skillDetail.createdAt || '-' }}</DescriptionsItem>
          <DescriptionsItem v-if="skillDetailInstalledAt" :label="$t('agent.instance_skillInstalledAt')">
            {{ skillDetailInstalledAt }}
          </DescriptionsItem>
        </Descriptions>
      </Spin>
    </Modal>

    <Modal
      v-model:open="desktopVisible"
      :title="desktopTitle"
      :width="1200"
      :footer="null"
      :destroy-on-close="true"
      wrap-class-name="desktop-modal"
    >
      <div class="mb-2 flex items-center justify-between rounded bg-gray-500/5 px-3 py-2">
        <span class="text-xs text-gray-500">
          <span class="mr-1 inline-block size-1.5 animate-pulse rounded-full bg-green-500 align-middle"></span>
          {{ $t('agent.instance_desktopHint') }}
        </span>
        <a
          :href="desktopUrl"
          target="_blank"
          class="inline-flex items-center gap-1 text-xs text-primary hover:opacity-80"
          :title="desktopUrl"
        >
          {{ $t('agent.instance_openInNewTab') }}
          <span aria-hidden="true">↗</span>
        </a>
      </div>
      <iframe
        :src="desktopUrl"
        class="w-full rounded border border-solid border-gray-200"
        style="height: 72vh; border: 0"
        allow="clipboard-read; clipboard-write; fullscreen"
        referrerpolicy="no-referrer"
      />
    </Modal>

    <Modal
      v-model:open="bindVisible"
      :title="$t('agent.instance_bindSocial')"
      :width="480"
      :footer="null"
      :destroy-on-close="true"
      @cancel="closeBind"
    >
      <Tabs v-model:active-key="bindTab" @change="onBindTabChange">
        <Tabs.TabPane key="wechat" :tab="$t('agent.instance_bindWechat')">
          <div class="flex min-h-[300px] flex-col items-center justify-center py-4">
            <template v-if="wechatStatus === 'querying'">
              <Spin />
              <div class="mt-3 px-6 text-center text-sm text-gray-500">
                {{ $t('agent.instance_bindQuerying') }}
              </div>
            </template>
            <template v-else-if="wechatStatus === 'loading'">
              <Spin />
              <div class="mt-3 px-6 text-center text-sm text-gray-500">
                {{ wechatMessage || $t('agent.instance_bindLoading') }}
              </div>
            </template>
            <template v-else-if="wechatStatus === 'qr'">
              <img
                v-if="wechatQrSrc"
                :src="wechatQrSrc"
                alt="QR"
                class="rounded border border-solid border-gray-200"
                style="width: 240px; height: 240px"
              />
              <div class="mt-3 text-sm text-gray-600">
                {{ $t('agent.instance_bindScanTip') }}
              </div>
            </template>
            <template v-else-if="wechatStatus === 'success'">
              <div class="text-5xl text-green-500">✓</div>
              <div class="mt-3 text-base font-medium text-green-600">
                {{ $t('agent.instance_bindSuccess') }}
              </div>
            </template>
            <template v-else-if="wechatStatus === 'bound'">
              <div class="text-5xl text-green-500">✓</div>
              <div class="mt-2 text-base font-medium text-green-600">
                {{ $t('agent.instance_bindBound') }}
              </div>
              <div class="mt-1 text-sm text-gray-500">
                {{ $t('agent.instance_bindAccount') }}：{{ bindAlias || bindAccountId || '-' }}
              </div>
              <div class="mt-4 flex items-center gap-1">
                <!-- hermes 支持解绑（清凭据 + restart 丢弃连接）；openclaw 微信(im-bot)不支持解绑，禁用 -->
                <Button v-if="bindAgent === 'hermes'" type="primary" ghost @click="startUnbind">
                  {{ $t('agent.instance_bindUnbind') }}
                </Button>
                <template v-else>
                  <Button type="primary" ghost disabled>
                    {{ $t('agent.instance_bindUnbind') }}
                  </Button>
                  <Tooltip :title="$t('agent.instance_bindUnbindUnsupported')">
                    <span class="cursor-help border border-solid border-gray-300 text-xs leading-[14px] text-gray-400 rounded-full inline-block size-[14px] text-center align-middle">?</span>
                  </Tooltip>
                </template>
              </div>
            </template>
            <template v-else-if="wechatStatus === 'unbinding'">
              <Spin />
              <div class="mt-3 px-6 text-center text-sm text-gray-500">
                {{ $t('agent.instance_bindUnbinding') }}
              </div>
            </template>
            <template v-else-if="wechatStatus === 'failed'">
              <div class="text-5xl text-red-500">✕</div>
              <div class="mt-3 max-w-[320px] text-center text-sm text-red-500">
                {{ wechatErrText || $t('agent.instance_bindFailed') }}
              </div>
              <Button class="mt-4" type="primary" ghost @click="queryChannelStatus">
                {{ $t('agent.instance_bindRetry') }}
              </Button>
            </template>
            <template v-else>
              <Button type="primary" @click="startWechatLogin">
                {{ $t('agent.instance_bindStart') }}
              </Button>
            </template>
          </div>
        </Tabs.TabPane>
        <Tabs.TabPane key="feishu" :tab="$t('agent.instance_bindFeishu')" disabled>
          <div class="py-10 text-center text-sm text-gray-400">
            {{ $t('agent.instance_bindComingSoon') }}
          </div>
        </Tabs.TabPane>
        <Tabs.TabPane key="dingtalk" :tab="$t('agent.instance_bindDingtalk')" disabled>
          <div class="py-10 text-center text-sm text-gray-400">
            {{ $t('agent.instance_bindComingSoon') }}
          </div>
        </Tabs.TabPane>
      </Tabs>
    </Modal>

    <ConfigDiffDialog v-model:open="diffVisible" :left="diffLeft" :right="diffRight" />
  </Page>
</template>

<style scoped>
/* 版本下拉「不安全」标：审计门禁禁选项的说明（暗红/亮红对，亮暗双主题可读） */
.skill-ver-unsafe {
  margin-left: 6px;
  color: #b8001f;
  font-size: 12px;
}
html.dark .skill-ver-unsafe {
  color: #f2a4b1;
}
/* ===== 运行详情模块卡片：同款复刻 client/device 的 module-card（tech 风格）——
   透明基 + 浅彩渐变填充(主色 8%) + 半透主色边框(30%) + hover 发光上浮 + 标题可点击折叠。
   color-mix 基于 transparent / --aa-bg（亮#fff 暗#111827，global-theme.css 定义），亮暗自适应。 */
.drawer-body {
  padding: 0 8px 24px;
}
.module-card {
  margin-bottom: 14px;
}
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
.module-card {
  border-color: color-mix(in srgb, var(--c, #6366f1) 30%, transparent) !important;
  background-color: transparent !important;
  background-image: linear-gradient(160deg, color-mix(in srgb, var(--c, #6366f1) 8%, transparent), transparent 60%) !important;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
  animation: module-card-in 0.4s ease both;
}
/* 同 device 交互：hover 边框加深(55%) + 主色发光投影 + 上浮 3px */
.module-card:hover {
  border-color: color-mix(in srgb, var(--c, #6366f1) 55%, transparent) !important;
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c, #6366f1) 45%, transparent);
  transform: translateY(-3px);
}
@keyframes module-card-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
/* 4 个 tab 卡片主色（对齐 device 同域模块色）：当前=蓝(状态)、历史=橙、配置=青、命令=品牌绿 */
.module-card.sec-current {
  --c: #3b82f6;
  animation-delay: 0.04s;
}
.module-card.sec-history {
  --c: #f59e0b;
  animation-delay: 0.1s;
}
.module-card.sec-config {
  --c: #06b6d4;
  animation-delay: 0.16s;
}
.module-card.sec-commands {
  --c: #42b983;
  animation-delay: 0.22s;
}
/* 标题框填充 = 主色 40% 与底色混色；标题字 = 主色加深(62%+黑) */
.module-card :deep(.ant-card-head) {
  min-height: 38px;
  padding: 0 14px;
  background-color: color-mix(in srgb, var(--c, #6366f1) 40%, var(--aa-bg, #fff)) !important;
  background-image: none !important;
  border-bottom: 1px solid #e4e7ed;
}
.module-card :deep(.ant-card-head-title) {
  padding: 9px 0;
  font-size: 13px;
  font-weight: 600;
  color: color-mix(in srgb, var(--c, #6366f1) 62%, #000) !important;
}
/* 内容区透明，透出卡片根的浅渐变 */
.module-card :deep(.ant-card-body) {
  padding: 14px;
  background-color: transparent !important;
  background-image: none !important;
}

/* ===== tab 标签染对应模块主色（与卡片标题栏同款填充）：激活=主色 40% 混底，
   未激活=主色 8% 淡底。nth-child 对应 pane 顺序：当前/历史/配置/命令。 */
.report-tabs :deep(.ant-tabs-tab) {
  margin: 0 10px 8px 0 !important;
  padding: 6px 16px !important;
  border-radius: 999px;
  background: color-mix(in srgb, var(--tc, #8c8c8c) 8%, var(--aa-bg, #fff));
  transition: background-color 0.2s ease;
}
.report-tabs :deep(.ant-tabs-tab:nth-child(1)) {
  --tc: #3b82f6;
}
.report-tabs :deep(.ant-tabs-tab:nth-child(2)) {
  --tc: #f59e0b;
}
.report-tabs :deep(.ant-tabs-tab:nth-child(3)) {
  --tc: #06b6d4;
}
.report-tabs :deep(.ant-tabs-tab:nth-child(4)) {
  --tc: #42b983;
}
.report-tabs :deep(.ant-tabs-tab-active) {
  background: color-mix(in srgb, var(--tc, #3b82f6) 40%, var(--aa-bg, #fff)) !important;
}
.report-tabs :deep(.ant-tabs-tab-btn) {
  font-weight: 600;
}
.report-tabs :deep(.ant-tabs-tab-active .ant-tabs-tab-btn) {
  color: color-mix(in srgb, var(--tc, #3b82f6) 62%, #000) !important;
}
/* tab 已用整块底色区分激活态，隐藏默认 ink 条避免杂色 */
.report-tabs :deep(.ant-tabs-ink-bar) {
  display: none;
}
</style>
<style>
/* 暗色：标题字换浅色（须 !important 压过 scoped 的 !important），标题底由 color-mix(--aa-bg) 自适应 */
html.dark .report-drawer .module-card .ant-card-head {
  border-bottom-color: #3a3a3a;
}
html.dark .report-drawer .module-card .ant-card-head-title {
  color: #e8e8e8 !important;
}
html.dark .report-drawer .report-tabs .ant-tabs-tab-active .ant-tabs-tab-btn {
  color: #e8e8e8 !important;
}
</style>
