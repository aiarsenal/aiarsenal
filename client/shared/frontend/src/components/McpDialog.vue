<template>
  <el-dialog
    v-model="visible"
    title="MCP 管理"
    width="560px"
    :close-on-click-modal="false"
  >
    <div class="mcp-body">
      <!-- 上半区：已绑分组 -->
      <div class="mcp-sec-head">
        <span>已绑定分组</span>
        <el-button size="small" :loading="loading" @click="refresh">刷新</el-button>
      </div>
      <div v-if="loading && bound.length === 0" class="mcp-loading">加载中…</div>
      <div v-if="!loading && bound.length === 0" class="mcp-empty">
        尚未绑定任何分组，从下方选择分组绑定
      </div>
      <div v-for="g in bound" :key="g.groupId" class="mcp-group">
        <div class="mcp-group-main">
          <div class="mcp-group-name">{{ g.groupName }}</div>
          <div v-if="g.description" class="mcp-group-desc">{{ g.description }}</div>
          <div v-if="serversText(g)" class="mcp-group-servers">
            包含服务：{{ serversText(g) }}
          </div>
        </div>
        <el-button
          v-if="g.canUnbind"
          size="small"
          type="danger"
          plain
          :loading="oping === g.groupId"
          @click="onUnbind(g)"
        >
          解绑
        </el-button>
        <el-tooltip v-else content="无该分组的解绑权限（由管理员或他人绑定）" placement="top">
          <span class="mcp-locked">仅查看</span>
        </el-tooltip>
      </div>

      <div class="mcp-divider"></div>

      <!-- 下半区：添加绑定（选项已按当前用户权限过滤） -->
      <div class="mcp-sec-head">
        <span>添加绑定</span>
      </div>
      <div class="mcp-add">
        <el-select
          v-model="selected"
          placeholder="选择要绑定的分组"
          filterable
          class="mcp-add-select"
        >
          <el-option
            v-for="o in selectable"
            :key="o.groupId"
            :label="o.name"
            :value="o.groupId"
          />
        </el-select>
        <el-button type="primary" :disabled="!selected" :loading="binding" @click="onBind">
          绑定
        </el-button>
      </div>
      <div v-if="!loading && selectable.length === 0" class="mcp-no-options">
        没有可绑定的分组（全部已绑，或暂无有权限的分组）
      </div>
      <p class="mcp-hint">
        绑定 / 解绑后服务端会重建本机 openclaw 的 MCP 配置并自动重启生效，无需手动管理密钥。
      </p>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { clientMcpBind, clientMcpList, clientMcpUnbind } from '../api';
import type { McpBoundGroup } from '../types';

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>();

const visible = ref(props.modelValue);
watch(
  () => props.modelValue,
  (v) => {
    visible.value = v;
    if (v) void refresh();
  }
);
watch(visible, (v) => emit('update:modelValue', v));

const loading = ref(false);
const binding = ref(false);
const oping = ref(''); // 正在解绑的 groupId（单行按钮 loading）
const bound = ref<McpBoundGroup[]>([]);
const options = ref<{ groupId: string; name: string }[]>([]);
const selected = ref<string>();

// 可绑选项 = 服务端下发的 options 里去掉已绑的（服务端已按权限过滤）
const selectable = computed(() => {
  const boundIds = new Set(bound.value.map((b) => b.groupId));
  return options.value.filter((o) => !boundIds.has(o.groupId));
});

const serversText = (g: McpBoundGroup) =>
  (g.servers ?? []).map((s) => s.alias || s.name).join('、');

const refresh = async () => {
  loading.value = true;
  try {
    const v = await clientMcpList();
    bound.value = v.bound ?? [];
    options.value = v.options ?? [];
  } catch (e) {
    ElMessage.error(`拉取 MCP 分组失败：${e}`);
  } finally {
    loading.value = false;
  }
};

const onBind = async () => {
  if (!selected.value) return;
  binding.value = true;
  try {
    await clientMcpBind(selected.value);
    ElMessage.success('绑定成功');
    selected.value = undefined;
    await refresh();
  } catch (e) {
    ElMessage.error(`绑定失败：${e}`);
  } finally {
    binding.value = false;
  }
};

const onUnbind = async (g: McpBoundGroup) => {
  oping.value = g.groupId;
  try {
    await clientMcpUnbind(g.groupId);
    ElMessage.success('已解绑');
    await refresh();
  } catch (e) {
    ElMessage.error(`解绑失败：${e}`);
  } finally {
    oping.value = '';
  }
};
</script>

<style scoped>
.mcp-body {
  font-size: 13px;
  color: #4a5260;
}
.mcp-sec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  color: #1f2933;
  margin-bottom: 8px;
}
.mcp-loading,
.mcp-empty {
  padding: 10px 4px;
  color: #9aa3af;
}
.mcp-group {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 4px;
  border-bottom: 1px solid #f0f2f5;
}
.mcp-group-main {
  flex: 1;
  min-width: 0;
}
.mcp-group-name {
  font-weight: 600;
  color: #42b983;
}
.mcp-group-desc {
  margin-top: 2px;
  font-size: 12px;
  color: #6b7480;
}
.mcp-group-servers {
  margin-top: 2px;
  font-size: 12px;
  color: #9aa3af;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.mcp-locked {
  font-size: 12px;
  color: #9aa3af;
  cursor: help;
}
.mcp-divider {
  margin: 16px 0 12px;
  border-top: 1px dashed #e4e8ee;
}
.mcp-add {
  display: flex;
  gap: 10px;
}
.mcp-add-select {
  flex: 1;
  min-width: 0;
}
.mcp-no-options {
  margin-top: 8px;
  font-size: 12px;
  color: #9aa3af;
}
.mcp-hint {
  margin: 12px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: #9aa3af;
}
</style>
