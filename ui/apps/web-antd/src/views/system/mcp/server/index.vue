<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Spin, Switch, Tabs, Textarea } from "ant-design-vue";

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addMcpServerApi,
  callMcpServerToolApi,
  deleteMcpServerApi,
  editMcpServerApi,
  getMcpServerListApi,
  testMcpServerApi,
} from '#/api/system/mcp/server';
import ConnectGuide from '../components/connect-guide.vue';

defineOptions({ name: 'SystemMcpServer' });

const TYPE_OPTIONS = [
  { label: 'stdio', value: 'stdio' },
  { label: 'sse', value: 'sse' },
  { label: 'streamable-http', value: 'streamable-http' },
  { label: 'openapi', value: 'openapi' },
];
const VISIBILITY_OPTIONS = computed(() => [
  { label: $t('mcp.server_visibilityPrivate'), value: 'private' },
  { label: $t('mcp.server_visibilityGroup'), value: 'group' },
  { label: $t('mcp.server_visibilityPublic'), value: 'public' },
]);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('mcp.server_name'),
    componentProps: { placeholder: $t('mcp.server_namePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'type',
    label: $t('mcp.server_type'),
    componentProps: {
      allowClear: true,
      options: TYPE_OPTIONS,
      placeholder: $t('mcp.server_typeAll'),
      style: { width: '100%' },
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('mcp.server_seq'), width: 60 },
    { field: 'name', title: $t('mcp.server_name'), minWidth: 140 },
    { field: 'type', title: $t('mcp.server_type'), width: 130 },
    {
      field: 'endpoint',
      title: $t('mcp.server_endpoint'),
      minWidth: 220,
      slots: { default: 'endpoint' },
    },
    { field: 'visibility', title: $t('mcp.server_visibility'), width: 100 },
    {
      field: 'enabled',
      title: $t('mcp.server_status'),
      width: 90,
      slots: { default: 'enabled' },
    },
    { field: 'createdAt', title: $t('mcp.server_createdAt'), minWidth: 160 },
    {
      field: 'action',
      title: $t('mcp.server_action'),
      width: 280,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        // gin: dto.McpServerSearchRes → { list, total, currentPage }
        const res = await getMcpServerListApi({
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

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('mcp.server_modalTitleEdit') : $t('mcp.server_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('mcp.server_name'),
    rules: 'required',
    componentProps: { placeholder: $t('mcp.server_namePlaceholderForm') },
  },
  {
    component: 'Select' as const,
    fieldName: 'type',
    label: $t('mcp.server_type'),
    rules: 'required',
    componentProps: {
      options: TYPE_OPTIONS,
      placeholder: $t('mcp.server_typePlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'description',
    label: $t('mcp.server_description'),
    componentProps: { placeholder: $t('mcp.server_descriptionPlaceholder') },
  },
  // stdio 专属
  {
    component: 'Input' as const,
    fieldName: 'command',
    label: $t('mcp.server_command'),
    dependencies: {
      triggerFields: ['type'],
      show: (values: any) => values.type === 'stdio',
    },
    componentProps: { placeholder: $t('mcp.server_commandPlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'args',
    label: $t('mcp.server_args'),
    dependencies: {
      triggerFields: ['type'],
      show: (values: any) => values.type === 'stdio',
    },
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.server_argsPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'env',
    label: $t('mcp.server_env'),
    dependencies: {
      triggerFields: ['type'],
      show: (values: any) => values.type === 'stdio',
    },
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.server_envPlaceholder'),
      style: { width: '100%' },
    },
  },
  // 远程端点专属
  {
    component: 'Input' as const,
    fieldName: 'url',
    label: $t('mcp.server_url'),
    dependencies: {
      triggerFields: ['type'],
      show: (values: any) => values.type !== 'stdio',
    },
    componentProps: { placeholder: $t('mcp.server_urlPlaceholder') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'headers',
    label: $t('mcp.server_headers'),
    dependencies: {
      triggerFields: ['type'],
      show: (values: any) => values.type !== 'stdio',
    },
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.server_headersPlaceholder'),
      style: { width: '100%' },
    },
  },
  // 高级（可选，与 DTO 对齐）
  {
    component: 'Textarea' as const,
    fieldName: 'options',
    label: $t('mcp.server_options'),
    componentProps: {
      rows: 2,
      placeholder: $t('mcp.server_optionsPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'Input' as const,
    fieldName: 'proxy',
    label: $t('mcp.server_proxy'),
    componentProps: { placeholder: $t('mcp.server_proxyPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'visibility',
    label: $t('mcp.server_visibility'),
    componentProps: {
      options: VISIBILITY_OPTIONS.value,
      placeholder: $t('mcp.server_visibilityPlaceholder'),
      style: { width: '100%' },
    },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'enabled',
    label: $t('mcp.server_status'),
    componentProps: {
      options: [
        { label: $t('mcp.server_enabled'), value: 1 },
        { label: $t('mcp.server_disabled'), value: 0 },
      ],
    },
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
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    type: 'stdio',
    visibility: 'private',
    enabled: 1,
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.serverId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    name: row.name,
    type: row.type,
    description: row.description ?? '',
    command: row.command ?? '',
    args: row.args ?? '',
    env: row.env ?? '',
    url: row.url ?? '',
    headers: row.headers ?? '',
    options: row.options ?? '',
    proxy: row.proxy ?? '',
    visibility: row.visibility ?? 'private',
    enabled: row.enabled ?? 1,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin edit: dto.McpServerEditReq = { serverId, ...McpServerReq }
      await editMcpServerApi({ ...values, serverId: editingId.value });
      message.success($t('mcp.server_editSuccess'));
    } else {
      await addMcpServerApi(values);
      message.success($t('mcp.server_addSuccess'));
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
    message.warning($t('mcp.server_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.serverId),
    rows.length,
  );
}
async function onDeleteRow(row: any) {
  await doDelete([row.serverId], 1);
}
function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('mcp.server_confirmDeleteTitle'),
    content: $t('mcp.server_confirmDeleteContent', { count }),
    okText: $t('mcp.server_confirm'),
    cancelText: $t('mcp.server_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMcpServerApi(ids);
      message.success($t('mcp.server_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 状态切换（行内 Switch，乐观更新）=====
async function onEnabledChange(row: any, checked: boolean) {
  const next = checked ? 1 : 0;
  try {
    // 复用 edit 端点：dto.McpServerEditReq 需要 serverId + 必填 name/type
    await editMcpServerApi({
      serverId: row.serverId,
      name: row.name,
      type: row.type,
      description: row.description ?? '',
      url: row.url ?? '',
      command: row.command ?? '',
      args: row.args ?? '',
      env: row.env ?? '',
      headers: row.headers ?? '',
      options: row.options ?? '',
      proxy: row.proxy ?? '',
      visibility: row.visibility ?? 'private',
      enabled: next,
    });
    row.enabled = next;
    message.success(
      next === 1
        ? $t('mcp.server_toggleEnableSuccess')
        : $t('mcp.server_toggleDisableSuccess'),
    );
  } catch {
    row.enabled = next === 1 ? 0 : 1;
  }
}

// ===== 连接测试 / 工具预览 / 在线试调用 =====
const testVisible = ref(false);
const testLoading = ref(false);
const testResult = ref<any>(null);
const testServerName = ref('');
const testServerId = ref('');
// 每工具的试调用状态：展开与否 / 参数 JSON / 执行中 / 结果
const callStates = reactive<Record<string, {
  open: boolean;
  args: string;
  running: boolean;
  result: string;
  error: boolean;
  durationMs: number;
}>>({});

async function onTest(row: any) {
  testServerName.value = row.name;
  testServerId.value = row.serverId;
  testVisible.value = true;
  testLoading.value = true;
  testResult.value = null;
  Object.keys(callStates).forEach((k) => delete callStates[k]);
  try {
    testResult.value = await testMcpServerApi({ serverId: row.serverId });
  } catch {
    /* 拦截器已提示 */
    testVisible.value = false;
  } finally {
    testLoading.value = false;
  }
}

// 依 inputSchema 生成参数骨架（string→""、number/integer→0、enum→首项）
function skeletonFromSchema(schema: any): string {
  const props = schema?.properties ?? {};
  const out: Record<string, any> = {};
  for (const [k, v] of Object.entries(props)) {
    const spec: any = v;
    if (Array.isArray(spec?.enum) && spec.enum.length > 0) {
      out[k] = spec.enum[0];
    } else if (spec?.type === 'number' || spec?.type === 'integer') {
      out[k] = 0;
    } else if (spec?.type === 'boolean') {
      out[k] = false;
    } else if (spec?.type === 'array') {
      out[k] = [];
    } else {
      out[k] = '';
    }
  }
  return JSON.stringify(out, null, 2);
}

function onToggleCall(tool: any) {
  const st = callStates[tool.name];
  if (st) {
    st.open = !st.open;
    return;
  }
  callStates[tool.name] = {
    open: true,
    args: skeletonFromSchema(tool.inputSchema),
    running: false,
    result: '',
    error: false,
    durationMs: 0,
  };
}

async function onRunCall(toolName: string) {
  const st = callStates[toolName];
  if (!st) return;
  let args: Record<string, any>;
  try {
    args = st.args.trim() === '' ? {} : JSON.parse(st.args);
  } catch {
    st.error = true;
    st.result = $t('mcp.server_callArgsInvalid');
    return;
  }
  // serverName 即列表行 name；serverId 从打开测试的那行取
  st.running = true;
  st.result = '';
  st.error = false;
  try {
    const res = await callMcpServerToolApi({
      serverId: testServerId.value,
      tool: toolName,
      arguments: args,
    });
    st.result = res?.text ?? '';
    st.durationMs = res?.durationMs ?? 0;
  } catch (e: any) {
    st.error = true;
    st.result = e?.message ?? String(e);
  } finally {
    st.running = false;
  }
}

// ===== 接入引导 =====
const guideVisible = ref(false);
const guideScope = ref('');

function onGuide(row: any) {
  guideScope.value = row?.name ?? '';
  guideVisible.value = true;
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('mcp.server_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">
          {{ $t('mcp.server_addBatchDelete') }}
        </Button>
      </template>
      <template #endpoint="{ row }">
        {{ row.type === 'stdio' ? row.command : row.url }}
      </template>
      <template #enabled="{ row }">
        <Switch
          :checked="row.enabled === 1"
          :checked-children="$t('mcp.server_switchOn')"
          :un-checked-children="$t('mcp.server_switchOff')"
          @change="(v: any) => onEnabledChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">
          {{ $t('mcp.server_edit') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onTest(row)">
          {{ $t('mcp.server_test') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onGuide(row)">
          {{ $t('mcp.connect_guide') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('mcp.server_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
    </Modal>

    <!-- 连接测试 / 工具预览 / 在线试调用 -->
    <Modal
      v-model:open="testVisible"
      :title="$t('mcp.server_testTitle', { name: testServerName })"
      :width="760"
      :footer="null"
      :destroy-on-close="true"
    >
      <Spin :spinning="testLoading">
        <div v-if="testResult">
          <div
            v-if="testResult.ok"
            class="mb-3 rounded border border-solid border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700 dark:border-green-800 dark:bg-green-950 dark:text-green-400"
          >
            {{ $t('mcp.server_testSuccess') }}（{{ testResult.durationMs }}ms）
          </div>
          <div
            v-else
            class="mb-3 rounded border border-solid border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-950 dark:text-red-400"
          >
            {{ $t('mcp.server_testFail') }}：{{ testResult.message }}
          </div>

          <Tabs v-if="testResult.ok">
            <Tabs.TabPane
              key="tools"
              :tab="$t('mcp.server_tabTools', { count: testResult.tools.length })"
            >
              <div v-if="!testResult.tools.length" class="py-4 text-center text-xs opacity-60">
                {{ $t('mcp.server_testNoTools') }}
              </div>
              <div
                v-for="tool in testResult.tools"
                :key="tool.name"
                class="mb-3 rounded border border-solid border-gray-200 p-3 dark:border-gray-700"
              >
                <div class="flex items-center justify-between">
                  <div>
                    <span class="font-medium">{{ tool.name }}</span>
                    <span class="ml-2 text-xs opacity-70">{{ tool.description }}</span>
                  </div>
                  <Button size="small" @click="onToggleCall(tool)">
                    {{ $t('mcp.server_call') }}
                  </Button>
                </div>
                <details class="mt-2">
                  <summary class="cursor-pointer text-xs opacity-70">
                    {{ $t('mcp.server_inputSchema') }}
                  </summary>
                  <pre class="mt-1 overflow-x-auto rounded bg-zinc-900 p-2 text-xs leading-5 text-zinc-100">{{ JSON.stringify(tool.inputSchema, null, 2) }}</pre>
                </details>

                <!-- 试调用展开区 -->
                <div v-if="callStates[tool.name]?.open" class="mt-3 border-t border-dashed border-gray-200 pt-3 dark:border-gray-700">
                  <div class="mb-1 text-xs opacity-70">
                    {{ $t('mcp.server_callArgs') }}
                  </div>
                  <Textarea
                    v-model:value="callStates[tool.name].args"
                    :rows="4"
                    :placeholder="$t('mcp.server_callArgsPlaceholder')"
                  />
                  <Button
                    size="small"
                    type="primary"
                    class="mt-2"
                    :loading="callStates[tool.name].running"
                    @click="onRunCall(tool.name)"
                  >
                    {{ $t('mcp.server_callRun') }}
                  </Button>
                  <template v-if="callStates[tool.name].result">
                    <div class="mt-2 mb-1 text-xs opacity-70">
                      {{ $t('mcp.server_callResult') }}（{{ callStates[tool.name].durationMs }}ms）
                    </div>
                    <pre
                      class="max-h-60 overflow-auto rounded p-2 text-xs leading-5"
                      :class="callStates[tool.name].error
                        ? 'bg-red-50 text-red-600 dark:bg-red-950 dark:text-red-400'
                        : 'bg-zinc-900 text-zinc-100'"
                      >{{ callStates[tool.name].result }}</pre>
                  </template>
                </div>
              </div>
            </Tabs.TabPane>
            <Tabs.TabPane
              key="prompts"
              :tab="$t('mcp.server_tabPrompts', { count: testResult.prompts.length })"
            >
              <div
                v-for="p in testResult.prompts"
                :key="p.name"
                class="py-1.5"
              >
                <span class="font-medium">{{ p.name }}</span>
                <span class="ml-2 text-xs opacity-70">{{ p.description }}</span>
              </div>
              <div v-if="!testResult.prompts.length" class="py-4 text-center text-xs opacity-60">
                —
              </div>
            </Tabs.TabPane>
            <Tabs.TabPane
              key="resources"
              :tab="$t('mcp.server_tabResources', { count: testResult.resources.length })"
            >
              <div
                v-for="r in testResult.resources"
                :key="r.name"
                class="py-1.5"
              >
                <span class="font-medium">{{ r.name }}</span>
                <span class="ml-2 text-xs opacity-70">{{ r.description }}</span>
              </div>
              <div v-if="!testResult.resources.length" class="py-4 text-center text-xs opacity-60">
                —
              </div>
            </Tabs.TabPane>
          </Tabs>
        </div>
      </Spin>
    </Modal>

    <!-- 客户端接入引导 -->
    <ConnectGuide v-model:open="guideVisible" :scope="guideScope" />
  </Page>
</template>
