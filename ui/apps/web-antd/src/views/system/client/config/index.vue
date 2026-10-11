<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch, Table } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  deleteClientAgentApi,
  deleteClientAgentVersionApi,
  getClientAgentListApi,
  saveClientAgentApi,
  saveClientAgentVersionApi,
} from '#/api/system/client/config';

defineOptions({ name: 'SystemClientConfig' });

// ===== 列表 =====
const allAgents = ref<any[]>([]);

async function loadList() {
  try {
    const res = await getClientAgentListApi();
    allAgents.value = (res?.list ?? []) as any[];
  } catch {
    allAgents.value = [];
  }
}

function filterAgents(kw: string): any[] {
  const k = (kw || '').trim().toLowerCase();
  if (!k) return allAgents.value;
  return allAgents.value.filter(
    (a: any) =>
      String(a.key ?? '').toLowerCase().includes(k) ||
      String(a.name ?? '').toLowerCase().includes(k),
  );
}

const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'expand', width: 50, slots: { content: 'expand_content' } },
    { type: 'seq', title: $t('client.config_seq'), width: 60, fixed: 'left' },
    { field: 'icon', title: $t('client.config_icon'), width: 70, slots: { default: 'icon' } },
    { field: 'key', title: 'Agent Key', minWidth: 130 },
    { field: 'name', title: $t('client.config_name'), minWidth: 120 },
    { field: 'intro', title: $t('client.config_intro'), minWidth: 220, showOverflow: true },
    {
      field: 'enabled',
      title: $t('client.config_enabled'),
      width: 80,
      slots: { default: 'enabled' },
    },
    { field: 'weigh', title: $t('client.config_weigh'), width: 80 },
    {
      field: 'action',
      title: $t('client.config_action'),
      width: 160,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async ({ form }: any) => {
        await loadList();
        const kw = String(form?.keyword ?? '');
        const items = filterAgents(kw);
        return { items, total: items.length };
      },
    },
  },
  toolbarConfig: {
    custom: true,
    refresh: { code: 'query' },
  },
}));

const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyword',
    label: $t('client.config_keyword'),
    componentProps: { placeholder: $t('client.config_keywordPlaceholder') },
  },
]);

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema },
});

// ===== Agent 弹窗 =====
const agentModalVisible = ref(false);
const agentEditing = ref(false);
const agentModalTitle = computed(() =>
  agentEditing.value
    ? $t('client.config_editAgent')
    : $t('client.config_addAgent'),
);

const agentFormSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'agent',
    label: 'Agent Key',
    rules: 'required',
    componentProps: {
      placeholder: $t('client.config_agentKeyPlaceholder'),
      disabled: agentEditing.value,
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('client.config_name'),
    rules: 'required',
    componentProps: { placeholder: $t('client.config_namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'icon',
    label: $t('client.config_icon'),
    componentProps: { placeholder: $t('client.config_iconPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'logo',
    label: $t('client.config_logo'),
    componentProps: { placeholder: $t('client.config_logoPlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'intro',
    label: $t('client.config_intro'),
    componentProps: { rows: 3, placeholder: $t('client.config_introPlaceholder') },
  },
  {
    component: 'Switch' as const,
    fieldName: 'enabled',
    label: $t('client.config_enabled'),
    componentProps: { checkedValue: 1, unCheckedValue: 0 },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'weigh',
    label: $t('client.config_weigh'),
    componentProps: { min: 0, max: 999 },
  },
]);

const [AgentForm, agentFormApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: agentFormSchema,
    showDefaultActions: false,
  }) as any,
);

async function openAddAgent() {
  agentEditing.value = false;
  agentModalVisible.value = true;
  await nextTick();
  await agentFormApi.resetForm();
  agentFormApi.setValues({ enabled: 1, weigh: 0 });
}

async function openEditAgent(row: any) {
  agentEditing.value = true;
  agentModalVisible.value = true;
  await nextTick();
  await agentFormApi.resetForm();
  agentFormApi.setValues({
    agentId: row.agentId,
    agent: row.key,
    name: row.name,
    icon: row.icon,
    logo: row.logo,
    intro: row.intro,
    enabled: row.enabled ? 1 : 0,
    weigh: row.weigh,
  });
}

async function onSubmitAgent() {
  const { valid } = await agentFormApi.validate();
  if (!valid) return;
  const values: any = await agentFormApi.getValues();
  try {
    await saveClientAgentApi(values);
    message.success($t('client.config_saved'));
    agentModalVisible.value = false;
        gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内启用切换：save agent（整行回填），失败回滚。
async function onEnabledChange(row: any, checked: boolean) {
  const newEnabled = checked ? 1 : 0;
  const prev = row.enabled;
  row.enabled = !!newEnabled;
  try {
    await saveClientAgentApi({
      agentId: row.agentId,
      agent: row.key,
      name: row.name,
      intro: row.intro || '',
      icon: row.icon || '',
      logo: row.logo || '',
      enabled: newEnabled,
      weigh: row.weigh ?? 0,
    });
    message.success($t('client.config_saved'));
  } catch {
    row.enabled = prev;
  }
}

function onDeleteAgent(row: any) {
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.config_deleteAgentContent', { name: row.name }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteClientAgentApi(row.agentId);
      message.success($t('client.config_deleted'));
            gridApi.query();
    },
  });
}

// ===== Version 弹窗 =====
const versionModalVisible = ref(false);
const versionEditing = ref(false);
const versionAgentId = ref('');
const versionAgentName = ref('');
const versionModalTitle = computed(() =>
  versionEditing.value
    ? $t('client.config_editVersion')
    : $t('client.config_addVersion'),
);

function objToStr(v: unknown): string {
  if (typeof v === 'string') return v;
  if (v == null) return '{}';
  try {
    return JSON.stringify(v);
  } catch {
    return '{}';
  }
}

// extra(jsonb) 解析为对象；字符串/空/非法 → {}。
function parseExtraObj(v: unknown): Record<string, any> {
  if (v == null || v === '') return {};
  if (typeof v === 'object') return v as Record<string, any>;
  try {
    return JSON.parse(v as string);
  } catch {
    return {};
  }
}
// 从 extra.downloads 取 win/mac 下载地址（per-OS）。
function downloadsOf(extra: unknown): { win: string; mac: string } {
  const o = parseExtraObj(extra);
  const d = o.downloads && typeof o.downloads === 'object' ? o.downloads : {};
  return { win: d.win || '', mac: d.mac || '' };
}

const versionFormSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'version',
    label: $t('client.config_version'),
    rules: 'required',
    componentProps: { placeholder: $t('client.config_versionPlaceholder') },
  },
  {
    component: 'Switch' as const,
    fieldName: 'isDefault',
    label: $t('client.config_isDefault'),
    componentProps: { checkedValue: 1, unCheckedValue: 0 },
  },
  {
    component: 'Switch' as const,
    fieldName: 'enabled',
    label: $t('client.config_enabled'),
    componentProps: { checkedValue: 1, unCheckedValue: 0 },
  },
  {
    component: 'Input' as const,
    fieldName: 'downloadWin',
    label: $t('client.config_downloadWin'),
    componentProps: { placeholder: $t('client.config_downloadUrlPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'downloadMac',
    label: $t('client.config_downloadMac'),
    componentProps: { placeholder: $t('client.config_downloadUrlPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'containerImage',
    label: $t('client.config_containerImage'),
    componentProps: { placeholder: $t('client.config_containerImagePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'channel',
    label: $t('client.config_channel'),
    componentProps: { placeholder: $t('client.config_channelPlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'configTemplate',
    label: $t('client.config_configTemplate'),
    componentProps: { rows: 4, placeholder: '{}' },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'envVars',
    label: $t('client.config_envVars'),
    componentProps: { rows: 3, placeholder: '{}' },
  },
]);

const [VersionForm, versionFormApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: versionFormSchema,
    showDefaultActions: false,
  }) as any,
);

async function openAddVersion(agentRow: any) {
  versionEditing.value = false;
  versionAgentId.value = agentRow.agentId;
  versionAgentName.value = agentRow.name;
  versionModalVisible.value = true;
  await nextTick();
  await versionFormApi.resetForm();
  versionFormApi.setValues({
    isDefault: 0,
    enabled: 1,
    configTemplate: '{}',
    envVars: '{}',
    downloadWin: '',
    downloadMac: '',
  });
}

async function openEditVersion(agentRow: any, verRow: any) {
  versionEditing.value = true;
  versionAgentId.value = agentRow.agentId;
  versionAgentName.value = agentRow.name;
  versionModalVisible.value = true;
  await nextTick();
  await versionFormApi.resetForm();
  const dl = downloadsOf(verRow.extra);
  versionFormApi.setValues({
    versionId: verRow.versionId,
    agentId: agentRow.agentId,
    version: verRow.version,
    isDefault: verRow.isDefault ? 1 : 0,
    enabled: verRow.enabled ? 1 : 0,
    downloadWin: dl.win,
    downloadMac: dl.mac,
    containerImage: verRow.containerImage || '',
    channel: verRow.channel || '',
    configTemplate: objToStr(verRow.configTemplate),
    envVars: objToStr(verRow.envVars),
  });
}

async function onSubmitVersion() {
  const { valid } = await versionFormApi.validate();
  if (!valid) return;
  const values: any = await versionFormApi.getValues();
  // 确保 agentId 带上（编辑回显已 set，新增时补）
  if (!values.agentId) values.agentId = versionAgentId.value;
  // per-OS 下载地址打包进 extra.downloads（保留 extra 里其它字段）
  const extraObj = parseExtraObj(values.extra);
  extraObj.downloads = { win: values.downloadWin || '', mac: values.downloadMac || '' };
  values.extra = JSON.stringify(extraObj);
  delete values.downloadWin;
  delete values.downloadMac;
  values.downloadUrl = values.downloadUrl || '';
  try {
    await saveClientAgentVersionApi(values);
    message.success($t('client.config_saved'));
    versionModalVisible.value = false;
        gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

function onDeleteVersion(verRow: any) {
  Modal.confirm({
    title: $t('client.device_tip'),
    content: $t('client.config_deleteVersionContent', {
      version: verRow.version,
    }),
    okText: $t('client.device_confirm'),
    cancelText: $t('client.device_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteClientAgentVersionApi(verRow.versionId);
      message.success($t('client.config_deleted'));
            gridApi.query();
    },
  });
}

// 版本子表列定义（antd Table）
const versionColumns = computed(() => [
  { title: $t('client.config_version'), dataIndex: 'version', width: 120 },
  { title: $t('client.config_isDefaultShort'), dataIndex: 'isDefault', width: 70, key: 'isDefault' },
  { title: $t('client.config_enabledShort'), dataIndex: 'enabled', width: 70, key: 'enabled' },
  { title: $t('client.config_containerImageShort'), dataIndex: 'containerImage', ellipsis: true },
  { title: $t('client.config_downloadUrlShort'), key: 'downloads', width: 220 },
  { title: $t('client.config_action'), key: 'action', width: 180 },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAddAgent">
          {{ $t('client.config_addAgent') }}
        </Button>
      </template>
      <template #icon="{ row }">
        <img v-if="row.logo" class="agent-logo" :src="row.logo" :alt="row.name" />
        <span v-else>{{ row.icon }}</span>
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="!!row.enabled"
          :checked-children="$t('client.config_yes')"
          :un-checked-children="$t('client.config_no')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEditAgent(row)">
          {{ $t('client.config_edit') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteAgent(row)">
          {{ $t('client.config_delete') }}
        </Button>
      </template>
      <template #expand_content="{ row }">
        <div style="padding: 10px 16px 16px 24px; background: var(--aa-bg); border-radius: 6px">
          <div
            style="
              display: flex;
              align-items: center;
              justify-content: space-between;
              margin-bottom: 8px;
            "
          >
            <span class="ver-list-title">
              {{ $t('client.config_versionListTitle', { count: row.versions?.length || 0 }) }}
            </span>
            <Button type="primary" size="small" @click="openAddVersion(row)">
              {{ $t('client.config_addVersion') }}
            </Button>
          </div>
          <Table
            :data-source="row.versions || []"
            :columns="versionColumns"
            :pagination="false"
            :row-key="(r: any) => r.versionId"
            bordered
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'isDefault'">
                <span v-if="record.isDefault" class="aa-enum aa-enum--magenta">{{ $t('client.config_defaultTag') }}</span>
              </template>
              <template v-else-if="column.key === 'enabled'">
                <span v-if="record.enabled" class="aa-enum aa-enum--magenta">{{ $t('client.config_yes') }}</span>
                <span v-else class="aa-enum aa-enum--default">{{ $t('client.config_no') }}</span>
              </template>
              <template v-else-if="column.key === 'downloads'">
                <div class="dl-cell">
                  <div><span class="dl-tag">Win</span> {{ downloadsOf(record.extra).win || '-' }}</div>
                  <div><span class="dl-tag">Mac</span> {{ downloadsOf(record.extra).mac || '-' }}</div>
                </div>
              </template>
              <template v-else-if="column.key === 'action'">
                <div style="display: flex; gap: 6px; align-items: center">
                  <Button
                    type="primary" ghost
                    size="small"
                    @click="openEditVersion(row, record)"
                  >
                    {{ $t('client.config_edit') }}
                  </Button>
                  <Button
                    type="primary" ghost
                    size="small"
                    danger
                    @click="onDeleteVersion(record)"
                  >
                    {{ $t('client.config_delete') }}
                  </Button>
                </div>
              </template>
            </template>
          </Table>
        </div>
      </template>
    </Grid>

    <Modal
      v-model:open="agentModalVisible"
      :title="agentModalTitle"
      :width="560"
      :destroy-on-close="true"
      @ok="onSubmitAgent"
    >
      <AgentForm />
    </Modal>

    <Modal
      v-model:open="versionModalVisible"
      :title="versionModalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="onSubmitVersion"
    >
      <div style="margin-bottom: 8px; color: #8a94a6; font-size: 12px">
        {{ $t('client.config_belongAgent', { name: versionAgentName }) }}
      </div>
      <VersionForm />
    </Modal>
  </Page>
</template>

<style scoped>
.dl-cell {
  font-size: 12px;
  line-height: 1.6;
  word-break: break-all;
}
.dl-tag {
  display: inline-block;
  min-width: 32px;
  margin-right: 4px;
  color: #8a94a6;
  font-weight: 600;
}
.agent-logo {
  width: 24px;
  height: 24px;
  object-fit: contain;
  vertical-align: middle;
}
/* 版本列表小标题：展开区底是 var(--aa-bg) 随主题变，浅用深字/暗用浅字 */
.ver-list-title {
  font-size: 13px;
  font-weight: 600;
  color: #606266;
}
html.dark .ver-list-title {
  color: rgba(255, 255, 255, 0.85);
}
</style>
