<script lang="ts" setup>
/**
 * MCP 分组绑定抽屉（docker 实例 / 桌面设备共用，props.targetType 区分）：
 * - 上半区：当前有效分组卡片（实例含「直绑 / 来自项目」来源标注，项目继承只读）+ 组内 server 概览
 * - 下半区：从全部分组里选择绑定（已绑的不重复出现）
 * - 绑/解绑由后端自动签收/清理专属 Bearer key 并把 mcp 段注入实例配置
 *   （openclaw=mcp.servers 读改写重推 / hermes=config.yaml mcp_servers 重建；设备=尝试 restart 全量重建），
 *   前端无需关心密钥。
 */
import { computed, ref, watch } from 'vue';

import { Button, Drawer, message, Select, Spin, Tag } from 'ant-design-vue';

import { $t } from '#/locales';
import {
  bindAgentInstanceMcpApi,
  getAgentInstanceMcpApi,
  unbindAgentInstanceMcpApi,
} from '#/api/system/agent/instance';
import {
  bindClientDeviceMcpApi,
  getClientDeviceMcpApi,
  unbindClientDeviceMcpApi,
} from '#/api/system/client/device';
import { getAllMcpGroupsApi, type McpGroupApi } from '#/api/system/mcp/group';

// 两种接口响应的归一化视图：设备侧无 source/projectNames（只有直绑），字段设为可选
interface BoundItem {
  canUnbind?: boolean; // 后端按权限回填（可使用该组 ∨ 本人绑的；项目行恒 false）
  description?: string;
  groupId: string;
  groupName: string;
  projectNames?: string[];
  servers?: { alias?: string; name: string }[];
  source?: 'direct' | 'project';
}

const props = defineProps<{
  open: boolean;
  targetId: string;
  targetName?: string;
  targetType: 'device' | 'instance';
}>();
const emit = defineEmits<{ 'update:open': [boolean] }>();

const isInstance = computed(() => props.targetType === 'instance');
const title = computed(() =>
  props.targetName
    ? `${$t('mcp.bindTitle')} · ${props.targetName}`
    : $t('mcp.bindTitle'),
);

const loading = ref(false);
const binding = ref(false);
const opGroupId = ref('');
const bound = ref<BoundItem[]>([]);
const allGroups = ref<McpGroupApi.ListItem[]>([]);
const selected = ref<string | undefined>();

const boundIds = computed(() => new Set(bound.value.map((b) => b.groupId)));
const selectable = computed(() =>
  allGroups.value.filter((g) => !boundIds.value.has(g.groupId)),
);

async function load() {
  if (!props.targetId) return;
  loading.value = true;
  try {
    const [list, groups] = await Promise.all([
      isInstance.value
        ? getAgentInstanceMcpApi(props.targetId)
        : getClientDeviceMcpApi(props.targetId),
      getAllMcpGroupsApi(),
    ]);
    bound.value = (list ?? []) as BoundItem[];
    allGroups.value = (groups as any)?.list ?? groups ?? [];
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.open,
  (v) => {
    if (v && props.targetId) {
      selected.value = undefined;
      load();
    }
  },
);

function serversText(item: BoundItem): string {
  const names = (item.servers ?? []).map((s) => s.alias || s.name);
  return names.join('、');
}

async function onBind() {
  if (!selected.value) return;
  binding.value = true;
  try {
    if (isInstance.value) {
      await bindAgentInstanceMcpApi(props.targetId, selected.value);
    } else {
      await bindClientDeviceMcpApi(props.targetId, selected.value);
    }
    message.success($t('mcp.bindDone'));
    selected.value = undefined;
    await load();
  } catch {
    /* 拦截器已提示 */
  } finally {
    binding.value = false;
  }
}

async function onUnbind(item: BoundItem) {
  opGroupId.value = item.groupId;
  try {
    if (isInstance.value) {
      await unbindAgentInstanceMcpApi(props.targetId, item.groupId);
    } else {
      await unbindClientDeviceMcpApi(props.targetId, item.groupId);
    }
    message.success($t('mcp.unbindDone'));
    await load();
  } catch {
    /* 拦截器已提示（如项目继承分组须去项目里解绑） */
  } finally {
    opGroupId.value = '';
  }
}
</script>

<template>
  <Drawer
    :open="open"
    :title="title"
    :width="720"
    :footer="null"
    :destroy-on-close="true"
    @update:open="emit('update:open', $event)"
  >
    <!-- 已绑定 -->
    <div class="mb-2 flex items-center">
      <span class="text-base font-medium">{{ $t('mcp.bindCurrent') }}</span>
      <Button
        size="small"
        type="primary"
        ghost
        :loading="loading"
        class="ml-auto"
        @click="load"
      >
        {{ $t('mcp.bindRefresh') }}
      </Button>
    </div>
    <Spin :spinning="loading">
      <div
        v-if="bound.length === 0"
        class="py-4 text-center text-sm text-gray-400"
      >
        {{ $t('mcp.bindEmpty') }}
      </div>
      <div
        v-for="item in bound"
        :key="item.groupId"
        class="mb-2 flex items-start gap-3 rounded border border-solid border-gray-200 p-3 dark:border-gray-700"
      >
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium text-[var(--primary-color,#42b983)]">
              {{ item.groupName }}
            </span>
            <Tag v-if="isInstance && item.source === 'project'" color="blue">
              {{ $t('mcp.bindSourceProject')
              }}<template v-if="item.projectNames?.length">
                · {{ item.projectNames.join('、') }}</template
              >
            </Tag>
            <Tag v-else-if="isInstance" color="green">
              {{ $t('mcp.bindSourceDirect') }}
            </Tag>
          </div>
          <div v-if="item.description" class="mt-1 text-xs text-gray-500">
            {{ item.description }}
          </div>
          <div
            v-if="serversText(item)"
            class="mt-1 truncate text-xs text-gray-400"
          >
            {{ $t('mcp.bindServers') }}: {{ serversText(item) }}
          </div>
        </div>
        <Button
          v-if="
            (!isInstance || item.source === 'direct') && item.canUnbind !== false
          "
          type="primary"
          ghost
          danger
          size="small"
          :loading="opGroupId === item.groupId"
          @click="onUnbind(item)"
        >
          {{ $t('mcp.unbindAction') }}
        </Button>
      </div>
    </Spin>

    <div class="my-4 border-t border-dashed border-gray-300 dark:border-gray-700"></div>

    <!-- 添加绑定 -->
    <div class="mb-2 text-base font-medium">{{ $t('mcp.bindAdd') }}</div>
    <div class="flex items-center gap-2">
      <Select
        v-model:value="selected"
        :options="
          selectable.map((g) => ({ label: g.name, value: g.groupId }))
        "
        :placeholder="$t('mcp.bindSelectPlaceholder')"
        show-search
        option-filter-prop="label"
        class="min-w-0 flex-1"
      />
      <Button
        type="primary"
        :disabled="!selected"
        :loading="binding"
        @click="onBind"
      >
        {{ $t('mcp.bindAction') }}
      </Button>
    </div>
    <div v-if="selectable.length === 0" class="mt-2 text-xs text-gray-400">
      {{ $t('mcp.bindNoSelectable') }}
    </div>
    <div class="mt-3 text-xs text-gray-400">{{ $t('mcp.bindHint') }}</div>
  </Drawer>
</template>
