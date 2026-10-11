<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
} from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Input,
  message,
  Modal,
  Textarea,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addLLMKeyApi,
  blockLLMKeyApi,
  checkLLMKeyUseApi,
  deleteLLMKeyApi,
  editLLMKeyApi,
  getLLMKeyListApi,
  getLLMModelAllApi,
  rotateLLMKeyApi,
} from '#/api/system/llm/key';
import { getLLMBudgetListApi } from '#/api/system/llm/budget';

defineOptions({ name: 'SystemLlmKey' });

// allowedModels 落库为 JSON 数组串（text 列），前端用 string[] 编辑，提交时序列化
function parseArr(v: any): string[] {
  try {
    const a = typeof v === 'string' ? JSON.parse(v) : v;
    return Array.isArray(a) ? a : [];
  } catch {
    return [];
  }
}

// llm 网关域名：由 api 域名派生（api.<domain> → llm.<domain>），随环境自动切换
const llmBase = (import.meta.env.VITE_GLOB_API_URL || '')
  .replace(/\/$/, '')
  .replace('//api.', '//llm.');

// ===== 已部署模型下拉（allowedModels 多选）：分组池 + 具体 group/name =====
const modelOptions = ref<{ label: string; value: string }[]>([]);
async function loadModels() {
  if (modelOptions.value.length) return;
  try {
    const res: any = await getLLMModelAllApi();
    const list = Array.isArray(res) ? res : res?.list ?? [];
    const seen = new Set<string>();
    const opts: { label: string; value: string }[] = [];
    for (const m of list) {
      const g = m?.modelGroup ?? '';
      if (!g) continue;
      // 分组池入口（裸 group）：选中后该组所有模型可调（组内按 weight 分发）
      if (!seen.has(g)) {
        seen.add(g);
        opts.push({ label: g, value: g });
      }
      // 具体 group/name
      if (m.modelName) {
        const id = `${g}/${m.modelName}`;
        if (!seen.has(id)) {
          seen.add(id);
          opts.push({ label: id, value: id });
        }
      }
    }
    modelOptions.value = opts;
  } catch {
    /* 忽略 */
  }
}
onMounted(loadModels);

// ===== 共享预算下拉（budgetId）：多 key 挂同一预算共用额度 =====
const budgetOptions = ref<{ label: string; value: string }[]>([]);
async function loadBudgets() {
  if (budgetOptions.value.length) return;
  try {
    const res: any = await getLLMBudgetListApi({ pageNum: 1, pageSize: 200 });
    const list = Array.isArray(res) ? res : res?.list ?? [];
    budgetOptions.value = list
      .filter((b: any) => b?.budgetId)
      .map((b: any) => ({ label: b.budgetName || b.budgetId, value: b.budgetId }));
  } catch {
    /* 忽略：预算拉不到只是少了下拉选项 */
  }
}
onMounted(loadBudgets);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'alias',
    label: $t('llm.key_alias'),
    componentProps: { placeholder: $t('llm.key_aliasPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'userId',
    label: $t('llm.key_userId'),
    componentProps: { placeholder: $t('llm.key_userId') },
  },
  {
    component: 'Input',
    fieldName: 'teamId',
    label: $t('llm.key_teamId'),
    componentProps: { placeholder: $t('llm.key_teamId') },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('llm.key_seq'), width: 60 },
    { field: 'keyAlias', title: $t('llm.key_alias'), minWidth: 140 },
    { field: 'teamId', title: $t('llm.key_team'), minWidth: 120 },
    { field: 'spend', title: $t('llm.key_spend'), width: 120, slots: { default: 'spend' } },
    {
      field: 'maxBudget',
      title: $t('llm.key_budget'),
      width: 110,
      slots: { default: 'budget' },
    },
    { field: 'createdAt', title: $t('llm.key_createdAt'), minWidth: 170 },
    { field: 'expires', title: $t('llm.key_expires'), minWidth: 170 },
    {
      field: 'blocked',
      title: $t('llm.key_status'),
      width: 90,
      slots: { default: 'status' },
    },
    { field: 'lastActive', title: $t('llm.key_lastActive'), minWidth: 170 },
    {
      field: 'action',
      title: $t('llm.key_action'),
      width: 260,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getLLMKeyListApi({
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

// ===== 触摸板/滚轮方向锁：斜向滚动时只走主导轴，避免同时横竖滚动 =====
// vxe-table 的 triggerBodyWheelEvent 在 deltaX/deltaY 同时存在时会同时横竖滚，
// 这里在捕获阶段拦截对角滚动：按主导轴重发一个仅含单轴 delta 的 wheel 事件，
// 实现“上下滑就上下滚、左右滑就左右滚”。
let wheelEl: HTMLElement | null = null;
let wheelLockAxis: null | 'x' | 'y' = null;
let wheelLastAt = 0;

function onGridWheel(ev: WheelEvent) {
  // 浏览器缩放(Ctrl)/显式横向滚(Shift)交给原生处理
  if (ev.ctrlKey || ev.shiftKey) return;
  const { deltaX, deltaY } = ev;
  if (!deltaX || !deltaY) {
    // 纯单轴：直接放行，并据此设定当前手势方向（普通鼠标滚轮走这里）
    wheelLockAxis = deltaY ? 'y' : 'x';
    wheelLastAt = Date.now();
    return;
  }
  const target = ev.target as HTMLElement | null;
  const viewport = target?.closest?.('.vxe-table--viewport-wrapper');
  if (!viewport) return; // 不在表格视口上：放行
  // 200ms 无事件视为新手势，重置方向锁
  if (Date.now() - wheelLastAt > 200) wheelLockAxis = null;
  wheelLastAt = Date.now();
  if (!wheelLockAxis) {
    wheelLockAxis = Math.abs(deltaY) >= Math.abs(deltaX) ? 'y' : 'x';
  }
  const clamped =
    wheelLockAxis === 'y' ? { deltaX: 0, deltaY } : { deltaX, deltaY: 0 };
  ev.preventDefault();
  ev.stopImmediatePropagation();
  // 重发单轴 wheel，让 vxe-table 仅按主导轴滚动
  target!.dispatchEvent(
    new WheelEvent('wheel', {
      deltaX: clamped.deltaX,
      deltaY: clamped.deltaY,
      deltaMode: ev.deltaMode,
      bubbles: true,
      cancelable: true,
    }),
  );
}

onMounted(async () => {
  await nextTick();
  wheelEl =
    (gridApi.grid as any)?.getRefMaps?.()?.refElem?.value ?? null;
  wheelEl?.addEventListener('wheel', onGridWheel, {
    capture: true,
    passive: false,
  });
});
onBeforeUnmount(() => {
  wheelEl?.removeEventListener('wheel', onGridWheel, { capture: true });
  wheelEl = null;
});

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingTokenHash = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('llm.key_modalTitleEdit') : $t('llm.key_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'keyAlias',
    label: $t('llm.key_alias'),
    help: $t('llm.key_aliasHelp'),
    rules: 'required',
    componentProps: { placeholder: $t('llm.key_aliasPlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'maxBudget',
    label: $t('llm.key_maxBudget'),
    help: $t('llm.key_maxBudgetHelp'),
    componentProps: { min: 0, precision: 2, placeholder: $t('llm.key_unlimitedHint'), style: { width: '100%' } },
  },
  {
    component: 'Select' as const,
    fieldName: 'budgetId',
    label: $t('llm.key_sharedBudget'),
    help: $t('llm.key_sharedBudgetHelp'),
    componentProps: () => ({
      allowClear: true,
      showSearch: true,
      options: budgetOptions.value,
      placeholder: $t('llm.key_unlimitedHint'),
      style: { width: '100%' } as any,
    }),
  },
  {
    component: 'Select' as const,
    fieldName: 'budgetDuration',
    label: $t('llm.key_budgetDuration'),
    help: $t('llm.key_budgetDurationHelp'),
    componentProps: {
      options: [
        { label: $t('llm.key_durationNone'), value: '' },
        { label: $t('llm.key_durationDay'), value: '1d' },
        { label: $t('llm.key_durationWeek'), value: '7d' },
        { label: $t('llm.key_durationMonth'), value: '30d' },
      ],
      style: { width: '100%' },
    },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'tpmLimit',
    label: $t('llm.key_tpmLimit'),
    help: $t('llm.key_tpmLimitHelp'),
    componentProps: { min: 0, placeholder: $t('llm.key_unlimitedHint'), style: { width: '100%' } },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'rpmLimit',
    label: $t('llm.key_rpmLimit'),
    help: $t('llm.key_rpmLimitHelp'),
    componentProps: { min: 0, placeholder: $t('llm.key_unlimitedHint'), style: { width: '100%' } },
  },
  {
    component: 'DatePicker' as const,
    fieldName: 'expires',
    label: $t('llm.key_expires'),
    help: $t('llm.key_expiresHelp'),
    componentProps: {
      showTime: true,
      style: { width: '100%' },
      placeholder: $t('llm.key_expiresPlaceholder'),
    },
  },
  {
    component: 'Select' as const,
    fieldName: 'allowedModels',
    label: $t('llm.key_allowedModels'),
    help: $t('llm.key_allowedModelsHelp'),
    componentProps: () => ({
      mode: 'multiple',
      allowClear: true,
      showSearch: true,
      options: modelOptions.value,
      placeholder: $t('llm.key_allowedModelsPlaceholder'),
      style: { width: '100%' },
    }),
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
  editingTokenHash.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingTokenHash.value = row.tokenHash;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    keyAlias: row.keyAlias,
    maxBudget: row.maxBudget ?? 0,
    budgetId: row.budgetId || undefined,
    budgetDuration: row.budgetDuration ?? '',
    tpmLimit: row.tpmLimit ?? 0,
    rpmLimit: row.rpmLimit ?? 0,
    expires: row.expires ? (dayjs(row.expires) as any) : undefined,
    allowedModels: parseArr(row.allowedModels),
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // expires: dayjs 对象 → RFC3339 串
  if (values.expires && typeof (values.expires as any)?.toISOString === 'function') {
    values.expires = (values.expires as dayjs.Dayjs).toISOString();
  } else if (values.expires) {
    values.expires = String(values.expires);
  } else {
    values.expires = '';
  }
  // allowedModels: string[] → JSON 串
  values.allowedModels = JSON.stringify(values.allowedModels ?? []);
  try {
    if (isEdit.value) {
      await editLLMKeyApi({ ...values, tokenHash: editingTokenHash.value });
      message.success($t('llm.key_editSuccess'));
      modalVisible.value = false;
      gridApi.query();
    } else {
      const res = await addLLMKeyApi(values);
      message.success($t('llm.key_addSuccess'));
      modalVisible.value = false;
      plainKey.value = res?.key || '';
      graceMinutes.value = 0;
      plainVisible.value = true;
      gridApi.query();
    }
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('llm.key_selectToDelete'));
    return;
  }
  await doDelete(
    rows.map((r: any) => r.tokenHash),
    rows.length,
  );
}
async function onDeleteRow(row: any) {
  await doDelete([row.tokenHash], 1);
}
async function doDelete(ids: string[], count: number) {
  // 先查哪些 key 被实例使用（best-effort：查失败按"未使用"走单次确认）。
  let boundNames: string[] = [];
  try {
    const results = await Promise.all(
      ids.map((h) => checkLLMKeyUseApi(h).catch(() => null)),
    );
    boundNames = results
      .filter((r) => r?.usedByInstance)
      .map((r) => r!.instanceName);
  } catch {
    /* ignore */
  }
  const names = boundNames.length ? boundNames.join('、') : '';

  const doRemove = async () => {
    await deleteLLMKeyApi(ids);
    message.success($t('llm.key_deleteSuccess'));
    gridApi.query();
  };

  if (!names) {
    // 未被实例使用：常规单次确认。
    Modal.confirm({
      title: $t('llm.key_confirmTitle'),
      content: $t('llm.key_confirmDeleteContent', { count }),
      okText: $t('llm.key_confirm'),
      cancelText: $t('llm.key_cancel'),
      okType: 'danger',
      onOk: doRemove,
    });
    return;
  }

  // 被实例使用：两次提醒 + 两次确认。
  Modal.confirm({
    title: $t('llm.key_deleteWarnBoundTitle'),
    content: $t('llm.key_deleteWarnBound1', { names }),
    okText: $t('llm.key_confirm'),
    cancelText: $t('llm.key_cancel'),
    okType: 'danger',
    onOk: () => {
      // 第二次确认
      Modal.confirm({
        title: $t('llm.key_deleteWarnBoundTitle'),
        content: $t('llm.key_deleteWarnBound2', { names }),
        okText: $t('llm.key_confirmDelete'),
        cancelText: $t('llm.key_cancel'),
        okType: 'danger',
        onOk: doRemove,
      });
    },
  });
}

// ===== 封禁/解封 =====
async function onToggleBlock(row: any) {
  const willBlock = !row.blocked;
  await blockLLMKeyApi(row.tokenHash, willBlock);
  message.success(willBlock ? $t('llm.key_blockSuccess') : $t('llm.key_unblockSuccess'));
  gridApi.query();
}

// ===== 轮换 =====
async function onRotate(row: any) {
  Modal.confirm({
    title: $t('llm.key_confirmTitle'),
    content: $t('llm.key_rotateConfirmContent', { name: row.keyAlias }),
    okText: $t('llm.key_confirm'),
    cancelText: $t('llm.key_cancel'),
    okType: 'danger',
    onOk: async () => {
      const res = await rotateLLMKeyApi(row.tokenHash);
      plainKey.value = res?.key || '';
      graceMinutes.value = res?.graceMinutes || 0;
      plainVisible.value = true;
      gridApi.query();
    },
  });
}

// ===== 明文密钥展示弹窗 =====
const plainVisible = ref(false);
const plainKey = ref('');
const graceMinutes = ref(0);

const exampleModel = computed(() => {
  // 取第一个具体 group/name 作为 curl 示例（池入口也可调，具体模型更直观）
  return (
    modelOptions.value.find((o) => o.value.includes('/'))?.value ||
    modelOptions.value[0]?.value ||
    'auto/gpt-4o-mini'
  );
});

const curlOpenAI = computed(
  () =>
    `curl -X POST ${llmBase}/v1/chat/completions \\
  -H "Authorization: Bearer ${plainKey.value}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${exampleModel.value}","messages":[{"role":"user","content":"hi"}]}'`,
);

const curlAnthropic = computed(
  () =>
    `curl -X POST ${llmBase}/v1/messages \\
  -H "x-api-key: ${plainKey.value}" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "content-type: application/json" \\
  -d '{"model":"${exampleModel.value}","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}'`,
);

function copyText(text: string) {
  navigator.clipboard?.writeText(text);
  message.success($t('llm.key_copied'));
}

function fmtSpend(v: any) {
  return `$${Number(v || 0).toFixed(6)}`;
}
function fmtBudget(v: any) {
  return v ? `$${Number(v).toFixed(2)}` : '∞';
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('llm.key_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('llm.key_batchDelete') }}</Button>
      </template>
      <template #spend="{ row }">{{ fmtSpend(row.spend) }}</template>
      <template #budget="{ row }">{{ fmtBudget(row.maxBudget) }}</template>
      <template #status="{ row }">
        <span v-if="row.blocked" class="aa-enum aa-enum--red">{{ $t('llm.key_blocked') }}</span>
        <span v-else class="aa-enum aa-enum--magenta">{{ $t('llm.key_normal') }}</span>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('llm.key_edit') }}</Button>
        <Button type="primary" ghost size="small" @click="onToggleBlock(row)">
          {{ row.blocked ? $t('llm.key_unblock') : $t('llm.key_block') }}
        </Button>
        <Button type="primary" ghost size="small" @click="onRotate(row)">{{ $t('llm.key_rotate') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('llm.key_delete') }}
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

    <Modal
      v-model:open="plainVisible"
      :title="$t('llm.key_plainTitle')"
      :width="760"
      :ok-text="$t('llm.key_plainOk')"
      @ok="plainVisible = false"
    >
      <Alert
        type="warning"
        show-icon
        :closable="false"
        :message="$t('llm.key_plainWarn')"
      />
      <Alert
        v-if="graceMinutes > 0"
        type="info"
        show-icon
        :closable="false"
        class="mt-2"
        :message="$t('llm.key_graceInfo', { minutes: graceMinutes })"
      />
      <div class="mt-3">
        <Input :value="plainKey" readonly>
          <template #addonAfter>
            <Button size="small" @click="copyText(plainKey)">{{ $t('llm.key_copy') }}</Button>
          </template>
        </Input>
      </div>
      <div class="mt-4">
        <div class="mb-2">
          <b>{{ $t('llm.key_openAIProtocol') }}</b>
          <Button size="small" type="primary" ghost class="ml-2" @click="copyText(curlOpenAI)">
            {{ $t('llm.key_copy') }}
          </Button>
        </div>
        <Textarea :value="curlOpenAI" :rows="4" readonly class="mb-3" />
        <div class="mb-2">
          <b>{{ $t('llm.key_anthropicProtocol') }}</b>
          <Button size="small" type="primary" ghost class="ml-2" @click="copyText(curlAnthropic)">
            {{ $t('llm.key_copy') }}
          </Button>
        </div>
        <Textarea :value="curlAnthropic" :rows="5" readonly />
      </div>
    </Modal>
  </Page>
</template>
