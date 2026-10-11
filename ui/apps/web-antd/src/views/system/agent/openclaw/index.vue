<script lang="ts" setup>
import type { AgentOpenclawApi } from '#/api/system/agent/openclaw';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Card,
  Empty,
  Input,
  message,
  Modal,
  Spin,
  Tag,
} from 'ant-design-vue';

import {
  getAgentOpenclawConfigApi,
  getAgentOpenclawConfigsApi,
  saveAgentOpenclawConfigApi,
  validateAgentOpenclawConfigApi,
} from '#/api/system/agent/openclaw';
import { $t } from '#/locales';

defineOptions({ name: 'SystemAgentOpenclaw' });

// ===== 实例配置列表（左栏） =====
const loading = ref(false);
const list = ref<AgentOpenclawApi.ConfigItem[]>([]);
const selectedId = ref('');

const statusTag = (s?: number) => {
  const map: Record<number, { color: string; key: string }> = {
    0: { color: 'blue', key: 'agent.instance_status_creating' },
    1: { color: 'magenta', key: 'agent.instance_status_running' },
    2: { color: 'default', key: 'agent.instance_status_stopped' },
    3: { color: 'red', key: 'agent.instance_status_error' },
  };
  const m = (s != null && map[s]) || map[3]!;
  return { color: m.color, text: $t(m.key) };
};

async function loadList() {
  loading.value = true;
  try {
    const res = await getAgentOpenclawConfigsApi();
    list.value = res?.list ?? [];
    // 默认选中第一台；已选实例还在列表里则保持
    if (!list.value.some((c) => c.instanceId === selectedId.value)) {
      selectedId.value = list.value[0]?.instanceId ?? '';
      if (selectedId.value) await loadDetail();
      else detail.value = undefined;
    }
  } catch {
    list.value = [];
  } finally {
    loading.value = false;
  }
}

// ===== 详情与编辑器（右栏） =====
const detail = ref<AgentOpenclawApi.ConfigDetail | undefined>();
const detailLoading = ref(false);
/** 编辑缓冲：进入编辑态后与后端内容脱钩，保存/撤销才回写 */
const editorContent = ref('');
const dirty = ref(false);
const saving = ref(false);
const validating = ref(false);

const selectedItem = computed(() =>
  list.value.find((c) => c.instanceId === selectedId.value),
);

async function loadDetail() {
  if (!selectedId.value) return;
  detailLoading.value = true;
  try {
    detail.value = await getAgentOpenclawConfigApi(selectedId.value);
    editorContent.value = detail.value?.content ?? '';
    dirty.value = false;
  } catch {
    detail.value = undefined;
    editorContent.value = '';
    dirty.value = false;
  } finally {
    detailLoading.value = false;
  }
}

async function onSelect(id: string) {
  if (id === selectedId.value) return;
  if (dirty.value && !(await confirmDiscard())) return;
  selectedId.value = id;
  await loadDetail();
}

/** 未保存改动离开确认（切实例 / 刷新列表共用） */
function confirmDiscard(): Promise<boolean> {
  return new Promise((resolve) => {
    Modal.confirm({
      title: $t('agent.oc_discardTitle'),
      content: $t('agent.oc_discardContent'),
      okText: $t('agent.oc_discardOk'),
      cancelText: $t('agent.oc_cancel'),
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
}

function onEditorInput() {
  dirty.value = true;
}

// ===== 校验 / 保存下发 =====
const validateResult = ref<AgentOpenclawApi.ValidateResult | undefined>();

async function onValidate() {
  validating.value = true;
  validateResult.value = undefined;
  try {
    validateResult.value = await validateAgentOpenclawConfigApi(editorContent.value);
  } catch {
    /* 拦截器已提示 */
  } finally {
    validating.value = false;
  }
}

async function onSave() {
  if (!selectedId.value) return;
  saving.value = true;
  try {
    const res = await saveAgentOpenclawConfigApi(
      selectedId.value,
      editorContent.value,
    );
    message.success($t('agent.oc_saveSuccess'));
    dirty.value = false;
    validateResult.value = undefined;
    await loadDetail();
    await loadList();
    if (res?.commandId) {
      message.info($t('agent.oc_dispatched'));
    }
  } catch {
    /* 拦截器已提示 */
  } finally {
    saving.value = false;
  }
}

async function onRefresh() {
  if (dirty.value && !(await confirmDiscard())) return;
  await loadDetail();
}

// ===== 展示 =====
function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString();
}

function fmtSize(bytes?: number) {
  if (!Number.isFinite(bytes) || !bytes) return '-';
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

onMounted(loadList);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="grid grid-cols-1 gap-4 xl:grid-cols-4">
      <!-- 左栏：openclaw 实例列表 -->
      <Card class="xl:col-span-1">
        <template #title>
          <span>{{ $t('agent.oc_listTitle') }}</span>
        </template>
        <template #extra>
          <Button size="small" :loading="loading" @click="loadList">
            {{ $t('agent.ws_refresh') }}
          </Button>
        </template>
        <Spin :spinning="loading">
          <Empty
            v-if="!loading && list.length === 0"
            :description="$t('agent.oc_empty')"
            class="py-12"
          />
          <div v-else class="flex flex-col gap-2">
            <div
              v-for="c in list"
              :key="c.instanceId"
              class="cursor-pointer rounded-lg border border-solid p-3 transition-colors"
              :class="
                c.instanceId === selectedId
                  ? 'border-primary bg-primary/5'
                  : 'border-border hover:border-primary/50'
              "
              @click="onSelect(c.instanceId)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="min-w-0 flex-1 truncate font-medium">{{ c.name }}</span>
                <span :class="['aa-enum', `aa-enum--${statusTag(c.status).color}`]">
                  {{ statusTag(c.status).text }}
                </span>
              </div>
              <div class="mt-1 flex items-center gap-2 text-xs opacity-50">
                <span>v{{ c.version || '-' }}</span>
                <span v-if="c.configSize">· {{ fmtSize(c.configSize) }}</span>
              </div>
              <div class="mt-1 text-xs opacity-40">
                {{ $t('agent.oc_syncedAt') }}: {{ fmtTime(c.syncedAt) }}
              </div>
            </div>
          </div>
        </Spin>
      </Card>

      <!-- 右栏：openclaw.json 查看/编辑 -->
      <Card class="xl:col-span-3">
        <template #title>
          <span>
            {{ $t('agent.oc_editorTitle') }}
            <span v-if="selectedItem" class="ml-2 text-xs opacity-50">
              {{ selectedItem.name }} · {{ selectedItem.configPath || 'openclaw.json' }}
            </span>
          </span>
        </template>
        <template #extra>
          <div class="flex items-center gap-2">
            <Tag v-if="dirty" color="warning">{{ $t('agent.oc_dirty') }}</Tag>
            <Button size="small" :loading="validating" :disabled="!selectedId" @click="onValidate">
              {{ $t('agent.oc_validate') }}
            </Button>
            <Button size="small" :disabled="!selectedId" @click="onRefresh">
              {{ $t('agent.oc_reload') }}
            </Button>
            <Button
              type="primary"
              size="small"
              :loading="saving"
              :disabled="!selectedId || !dirty"
              @click="onSave"
            >
              {{ $t('agent.oc_save') }}
            </Button>
          </div>
        </template>
        <Spin :spinning="detailLoading">
          <Empty
            v-if="!detailLoading && !selectedId"
            :description="$t('agent.oc_pickHint')"
            class="py-12"
          />
          <template v-else>
            <!-- 元信息条 -->
            <div class="mb-2 flex flex-wrap items-center gap-x-6 gap-y-1 text-xs opacity-60">
              <span>hash: {{ detail?.configHash?.slice(0, 12) || '-' }}</span>
              <span>{{ $t('agent.oc_configSize') }}: {{ fmtSize(detail?.configSize) }}</span>
              <span>{{ $t('agent.oc_updatedAt') }}: {{ fmtTime(detail?.updatedAt) }}</span>
              <span>{{ $t('agent.oc_syncedAt') }}: {{ fmtTime(detail?.syncedAt) }}</span>
            </div>
            <Input.TextArea
              :value="editorContent"
              class="font-mono"
              :rows="22"
              spellcheck="false"
              :placeholder="$t('agent.oc_editorPlaceholder')"
              @update:value="(v: string) => { editorContent = v; onEditorInput(); }"
            />
            <!-- 校验结果 -->
            <div v-if="validateResult" class="mt-2">
              <div v-if="validateResult.valid" class="text-sm text-emerald-500">
                ✓ {{ $t('agent.oc_validOk') }}
              </div>
              <div v-else class="rounded border border-solid border-red-300 bg-red-50 p-2 text-sm dark:border-red-800 dark:bg-red-950/40">
                <div class="mb-1 font-medium text-red-500">
                  {{ $t('agent.oc_validFail') }}
                </div>
                <div
                  v-for="(e, i) in validateResult.errors"
                  :key="i"
                  class="font-mono text-xs text-red-500"
                >
                  · {{ e }}
                </div>
              </div>
            </div>
            <div class="mt-2 text-xs opacity-40">{{ $t('agent.oc_saveHint') }}</div>
          </template>
        </Spin>
      </Card>
    </div>
  </Page>
</template>
