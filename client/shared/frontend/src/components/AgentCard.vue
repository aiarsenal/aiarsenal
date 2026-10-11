<template>
  <div class="agent-card">
    <div class="card-head">
      <div class="title">
        <img v-if="logoSrc" class="logo" :src="logoSrc" :alt="spec.name" />
        <span v-else-if="spec.icon" class="icon">{{ spec.icon }}</span>
        <span class="name">{{ spec.name || spec.key }}</span>
      </div>
      <div class="badges">
        <el-tag v-if="installed" size="small" type="success">已安装</el-tag>
        <el-tag v-else size="small" type="info">未安装</el-tag>
        <el-tag v-if="running" size="small" type="success" effect="dark">运行中</el-tag>
        <el-tag v-else size="small" type="warning">已停止</el-tag>
      </div>
    </div>

    <div v-if="spec.intro" class="card-intro">{{ spec.intro }}</div>

    <div class="card-version">
      <span class="version-label">版本：</span>
      <el-select v-model="versionIdx" size="small" :disabled="versions.length === 0" style="flex: 1">
        <el-option
          v-for="(v, i) in versions"
          :key="v.versionId || v.version"
          :label="v.version + (v.isDefault ? '（默认）' : '')"
          :value="i"
        />
        <template #empty>
          <span class="empty">无可安装版本</span>
        </template>
      </el-select>
    </div>

    <div class="card-meta">
      <div v-if="currentVersion?.containerImage" class="meta-row">
        <span class="meta-label">镜像：</span>
        <span class="meta-value mono">{{ currentVersion.containerImage }}</span>
      </div>
      <div v-if="currentVersion?.downloadUrl" class="meta-row">
        <span class="meta-label">下载：</span>
        <span class="meta-value mono">{{ currentVersion.downloadUrl }}</span>
      </div>
      <div v-if="runtime?.pidOrContainerId" class="meta-row">
        <span class="meta-label">PID/容器：</span>
        <span class="meta-value mono">{{ runtime.pidOrContainerId }}</span>
      </div>
    </div>

    <div class="card-actions">
      <el-button size="small" type="primary" :disabled="busy || running || !currentVersion" :loading="busy" @click="$emit('start')">启动</el-button>
      <el-button size="small" :disabled="busy || !running" @click="$emit('stop')">停止</el-button>
      <el-button size="small" :disabled="busy || !installed" @click="$emit('edit-config')">配置</el-button>
      <el-button size="small" type="success" :disabled="busy || installed || !currentVersion" :loading="busy" @click="$emit('install')">安装</el-button>
      <el-button size="small" type="danger" plain :disabled="busy || !installed" @click="$emit('uninstall')">卸载</el-button>
    </div>

    <pre v-if="log" ref="logEl" class="card-log">{{ log }}</pre>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import type { AgentVersion, AgentRuntime } from '../types';
// 三客户端官方 logo（按 agent key 映射；未知 agent 回退到 admin 配置的 emoji 图标）
import openclawLogo from '../assets/logos/openclaw.svg';
import hermesLogo from '../assets/logos/hermes.png';
import obsidianLogo from '../assets/logos/obsidian.svg';

const LOGOS: Record<string, string> = {
  openclaw: openclawLogo,
  hermes: hermesLogo,
  obsidian: obsidianLogo,
};

const props = defineProps<{
  spec: {
    key: string;
    name: string;
    intro?: string;
    icon?: string;
  };
  versions: AgentVersion[];
  modelVersionIdx?: number;
  runtime?: AgentRuntime;
  busy?: boolean;
  log?: string;
}>();

const emit = defineEmits<{
  (e: 'install'): void;
  (e: 'uninstall'): void;
  (e: 'start'): void;
  (e: 'stop'): void;
  (e: 'edit-config'): void;
  (e: 'update:modelVersionIdx', v: number): void;
}>();

const installed = computed(() => !!props.runtime?.installed);
const running = computed(() => !!props.runtime?.running);

// 本地选中版本下标（默认取 isDefault 的版本，否则 0）
const versionIdx = ref<number>(0);
const selectDefault = () => {
  const idx = props.versions.findIndex((v) => v.isDefault);
  versionIdx.value = idx >= 0 ? idx : 0;
};
watch(() => props.versions, selectDefault, { immediate: true });
watch(versionIdx, (v) => emit('update:modelVersionIdx', v));

const currentVersion = computed<AgentVersion | undefined>(() => props.versions[versionIdx.value]);
const logoSrc = computed<string>(() => LOGOS[props.spec.key] || '');

const logEl = ref<HTMLPreElement | null>(null);
watch(
  () => props.log,
  () => nextTick(() => {
    if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight;
  })
);

defineExpose({ currentVersion });
</script>

<style scoped>
.agent-card {
  background: #fff;
  border-radius: 10px;
  padding: 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  border: 1px solid #eef0f4;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.icon {
  font-size: 18px;
}
.logo {
  width: 22px;
  height: 22px;
  object-fit: contain;
}
.name {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  text-transform: capitalize;
}
.badges {
  display: flex;
  gap: 6px;
}
.card-intro {
  font-size: 12px;
  color: #8a94a6;
  line-height: 1.5;
}
.card-version {
  display: flex;
  align-items: center;
  gap: 8px;
}
.version-label {
  flex-shrink: 0;
  font-size: 13px;
  color: #606266;
}
.empty {
  color: #aab2c0;
  font-size: 12px;
}
.card-meta {
  font-size: 12px;
  color: #8a94a6;
  min-height: 18px;
}
.meta-row {
  display: flex;
  margin-bottom: 4px;
}
.meta-label {
  flex-shrink: 0;
  width: 64px;
}
.meta-value {
  word-break: break-all;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.card-log {
  margin: 0;
  max-height: 180px;
  overflow: auto;
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
</style>
