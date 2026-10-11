<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, h, nextTick, reactive, ref } from 'vue';
// 注：成员下拉曾经走 ApiSelect（api 自取数），真机上控件不渲染（数据已到位仍空白），
// 换普通 Select + 弹窗打开时 loadServerOptions 手动拉数，行为一致且路径已被多页面验证。

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Select, Tag, Tooltip } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import GrantEditor, { type GrantRowValue } from '#/components/GrantEditor.vue';
import {
  addMcpGroupApi,
  deleteMcpGroupApi,
  editMcpGroupApi,
  getMcpGroupGrantApi,
  getMcpGroupListApi,
  saveMcpGroupGrantApi,
} from '#/api/system/mcp/group';
import { getAllMcpServersApi } from '#/api/system/mcp/server';

defineOptions({ name: 'SystemMcpGroup' });

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('mcp.group_name'),
    componentProps: { placeholder: $t('mcp.group_namePlaceholder') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.group_seq'), width: 60 },
    { field: 'name', title: $t('mcp.group_groupName'), minWidth: 140 },
    { field: 'description', title: $t('mcp.group_description'), minWidth: 180 },
    {
      field: 'servers',
      title: $t('mcp.group_servers'),
      minWidth: 220,
      slots: { default: 'servers' },
    },
    {
      field: 'owner',
      title: $t('mcp.group_owner'),
      minWidth: 100,
      // 后端回填 ownerName（归属人姓名）；存量行查不到姓名时回落显示 id
      formatter: ({ row }: any) => row?.ownerName || row?.owner || '',
    },
    {
      field: 'visibility',
      title: $t('mcp.group_visibility'),
      width: 100,
      slots: { default: 'visibility' },
    },
    { field: 'createdAt', title: $t('mcp.group_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('mcp.group_action'),
      width: 220,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getMcpGroupListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        return { items: res?.list ?? [], total: res?.total ?? 0 };
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

// ===== 成员 server 格式化：jsonb 字符串 → "name1, name2" =====
function formatServers(s: any): string {
  try {
    const arr = typeof s === 'string' ? JSON.parse(s) : s;
    return (arr ?? [])
      .map((x: any) => (typeof x === 'string' ? x : x?.name))
      .filter(Boolean)
      .join(', ');
  } catch {
    return '';
  }
}

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value
    ? $t('mcp.group_modalTitleEdit')
    : $t('mcp.group_modalTitleAdd'),
);

// 成员下拉：打开弹窗时手动拉全量 server（name 唯一，即分组引用键），
// 存 serverOptions 供普通 Select 使用——不走 ApiSelect 封装（真机上控件不渲染，排查无果换可靠路径）。
// 下拉选项 label 带「｜备注」（optionFilterProp=label → 备注也可搜）；选中 tag 走 memberTagRender
// 短名展示 + 悬停 Tooltip 展示详细信息，详情按 name 从 serverDetailMap 回查。
// 编辑前的原始成员（对象形态可能带 alias/tools 等裁剪配置）记在 editingServerRaw，
// 提交时原样保留，避免多选只回填 name 把高级配置抹掉；新勾选的成员提交纯 name。
let editingServerRaw = new Map<string, any>();
const serverOptions = ref<{ label: string; value: string }[]>([]);
const serversLoading = ref(false);
const serverDetailMap = new Map<string, any>();

async function loadServerOptions() {
  serversLoading.value = true;
  try {
    const list = await getAllMcpServersApi();
    serverDetailMap.clear();
    for (const s of list ?? []) {
      serverDetailMap.set(s.name, s);
    }
    serverOptions.value = (list ?? []).map((s: any) => ({
      label:
        `${s.name}${s.description ? `｜${s.description}` : ''}` +
        (s.enabled === 0 ? '（已停用）' : ''),
      value: s.name,
    }));
  } catch {
    /* 拦截器已提示 */
  } finally {
    serversLoading.value = false;
  }
}

// 选中成员 tag 自定义渲染（Select tagRender 函数 prop）：短名 + Tooltip 详细信息。
// 值不在 serverDetailMap（server 已删但分组仍引用）时只展示名称。
function memberTagRender({ value, closable, onClose }: any) {
  const s = serverDetailMap.get(value);
  const name = s?.enabled === 0 ? `${value}（已停用）` : String(value);
  const rows = [`${$t('mcp.server_name')}：${value}`];
  if (s) {
    if (s.description) rows.push(`${$t('mcp.group_noteLabel')}：${s.description}`);
    if (s.type) rows.push(`${$t('mcp.server_type')}：${s.type}`);
    const endpoint = s.url || s.command;
    if (endpoint) rows.push(`${$t('mcp.group_endpointLabel')}：${endpoint}`);
    rows.push(
      `${$t('mcp.server_status')}：${s.enabled === 0 ? $t('mcp.server_disabled') : $t('mcp.server_enabled')}`,
    );
  }
  const title = h(
    'div',
    { style: 'max-width:360px;white-space:normal' },
    rows.map((r) => h('div', { style: 'line-height:1.8' }, r)),
  );
  return h(
    Tooltip,
    { title },
    () =>
      h(
        Tag,
        { closable, onClose, style: 'margin-inline-end:4px' },
        () => name,
      ),
  );
}

function parseServerNames(s: any): string[] {
  try {
    const arr = typeof s === 'string' ? JSON.parse(s) : s;
    return (arr ?? [])
      .map((x: any) => (typeof x === 'string' ? x : x?.name))
      .filter(Boolean);
  } catch {
    return [];
  }
}

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('mcp.group_groupName'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.group_groupNamePlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'description',
    label: $t('mcp.group_description'),
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.group_descriptionPlaceholder'),
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'servers',
    label: $t('mcp.group_servers'),
    componentProps: {
      options: serverOptions.value,
      loading: serversLoading.value,
      // antd Select 是 inline-flex，不指定宽度会缩成内容宽（一行小窄条）
      style: { width: '100%' },
      mode: 'multiple',
      showSearch: true,
      optionFilterProp: 'label', // label 含备注 → 备注也可搜
      tagRender: memberTagRender, // 选中 tag 短名 + 悬停详情
      placeholder: $t('mcp.group_serversPlaceholder'),
    },
    help: $t('mcp.group_serversHelp'),
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  editingServerRaw = new Map();
  modalVisible.value = true;
  loadServerOptions(); // 不 await：先出弹窗，选项到了自动出现
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({ servers: [] });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.groupId;
  modalVisible.value = true;
  loadServerOptions();
  await nextTick();
  await formApi.resetForm();
  // 原始对象成员（带 alias/tools 裁剪配置）记下供提交时保留；表单只回填 name 数组
  try {
    const arr =
      typeof row.servers === 'string' ? JSON.parse(row.servers) : row.servers;
    editingServerRaw = new Map(
      (arr ?? [])
        .filter((x: any) => x && typeof x === 'object' && x.name)
        .map((x: any) => [x.name, x]),
    );
  } catch {
    editingServerRaw = new Map();
  }
  formApi.setValues({
    name: row.name,
    description: row.description,
    servers: parseServerNames(row.servers),
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // 多选值（name 数组）→ 提交 JSON：原有对象配置原样保留，新选的用纯 name
  const servers = (values.servers ?? []).map(
    (n: string) => editingServerRaw.get(n) ?? n,
  );
  const payload = { ...values, servers: JSON.stringify(servers) };
  try {
    if (isEdit.value) {
      // gin edit 需要 groupId（= 行 biz id）
      await editMcpGroupApi({ ...payload, groupId: editingId.value });
      message.success($t('mcp.group_editSuccess'));
    } else {
      await addMcpGroupApi(payload);
      message.success($t('mcp.group_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('mcp.group_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.groupId),
    rows.length,
  );
}

async function onDeleteRow(row: any) {
  await doDelete([row.groupId], 1);
}

function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.group_confirmDeleteTitle'),
    content: $t('mcp.group_confirmDeleteContent', { count }),
    okText: $t('mcp.group_confirm'),
    cancelText: $t('mcp.group_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpGroupApi(ids);
      message.success($t('mcp.group_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 组合授权（可见性档位 + 「可使用」条件行；单动作单编辑器，与 KB 读写双编辑器不同） =====
const VIS_OPTIONS = computed(() => [
  { label: $t('mcp.group_visAll'), value: 2 },
  { label: $t('mcp.group_visCond'), value: 1 },
  { label: $t('mcp.group_visPrivate'), value: 0 },
]);

const grantModal = reactive({
  open: false,
  loading: false,
  groupId: '',
  name: '',
  visibility: 0,
});
const grantRules = ref<GrantRowValue[]>([]);

async function openGrant(row: any) {
  Object.assign(grantModal, {
    open: true,
    loading: true,
    groupId: row.groupId,
    name: row.name,
    visibility: 0,
  });
  grantRules.value = [];
  try {
    const info = await getMcpGroupGrantApi(row.groupId);
    grantModal.visibility = info?.visibility ?? 0;
    grantRules.value = (info?.rules ?? []) as GrantRowValue[];
  } catch {
    /* 拦截器已提示 */
  } finally {
    grantModal.loading = false;
  }
}

async function onSubmitGrant() {
  grantModal.loading = true;
  try {
    await saveMcpGroupGrantApi({
      groupId: grantModal.groupId,
      visibility: grantModal.visibility,
      rules: JSON.stringify(grantRules.value ?? []),
    });
    message.success($t('mcp.group_grantSaved'));
    grantModal.open = false;
    gridApi.query(); // 可见性徽标随授权变化
  } finally {
    grantModal.loading = false;
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('mcp.group_add') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.group_addBatchDelete') }}
        </Button>
      </template>
      <template #servers="{ row }">
        {{ formatServers(row.servers) || $t('mcp.group_noMembers') }}
      </template>
      <template #visibility="{ row }">
        <Tag v-if="row.visibility === 2" color="green">
          {{ $t('mcp.group_visAll') }}
        </Tag>
        <Tag v-else-if="row.visibility === 1" color="blue">
          {{ $t('mcp.group_visCond') }}
        </Tag>
        <Tag v-else color="red">{{ $t('mcp.group_visPrivate') }}</Tag>
      </template>
      <template #action="{ row }">
        <!-- 编辑/删除/配授权限 owner+超管（后端 canManage 回填；越权直调会被 service 层拒绝） -->
        <template v-if="row.canManage">
          <Button type="primary" ghost size="small" @click="openEdit(row)">
            {{ $t('mcp.group_edit') }}
          </Button>
          <Button
            type="primary"
            ghost
            size="small"
            class="ml-2"
            @click="openGrant(row)"
          >
            {{ $t('mcp.group_grant') }}
          </Button>
          <Button
            type="primary"
            ghost
            size="small"
            danger
            class="ml-2"
            @click="onDeleteRow(row)"
          >
            {{ $t('mcp.group_delete') }}
          </Button>
        </template>
        <span v-else class="text-xs opacity-50">
          {{ $t('mcp.group_noManage') }}
        </span>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="720"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <!-- 组合授权（可见性档位 + 「可使用」条件行） -->
    <Modal
      v-model:open="grantModal.open"
      :title="$t('mcp.group_grantTitle')"
      :width="680"
      :confirm-loading="grantModal.loading"
      :destroy-on-close="true"
      @ok="onSubmitGrant"
    >
      <div class="flex flex-col gap-2">
        <div class="text-sm opacity-70">{{ grantModal.name }}</div>
        <div>{{ $t('mcp.group_visibility') }}</div>
        <Select
          v-model:value="grantModal.visibility"
          :options="VIS_OPTIONS"
          style="width: 200px"
        />
        <div class="text-xs opacity-60">{{ $t('mcp.group_visTip') }}</div>
        <template v-if="grantModal.visibility === 1">
          <div class="mt-1">{{ $t('mcp.group_grantRules') }}</div>
          <GrantEditor
            v-model="grantRules"
            :default-mods="['users', 'dept']"
            :mod-order="['users', 'dept', 'roles', 'posts', 'group', 'set']"
          />
          <div class="text-xs opacity-60">
            {{ $t('mcp.group_grantRulesTip') }}
          </div>
        </template>
      </div>
    </Modal>
  </Page>
</template>
