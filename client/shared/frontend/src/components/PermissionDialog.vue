<template>
  <el-dialog
    v-model="visible"
    title="系统权限"
    width="560px"
    :close-on-click-modal="false"
  >
    <div class="perm-body">
      <p class="perm-desc">
        agent 通过 node 命令使用截屏 / 控鼠键 / 定位 / 通讯录等能力，均需 macOS 授权本客户端。
        首次使用会逐项弹系统授权框；这里可一次性集中申请，也可随时查看状态。
      </p>
      <el-alert
        v-if="isDev"
        type="warning"
        :closable="false"
        show-icon
        class="perm-dev-banner"
        title="开发模式（未签名 / debug 构建）：部分 TCC 授权框可能不弹或归因异常，正式签名包才完全可靠"
      />

      <div class="perm-row perm-row-head">
        <span class="perm-name">权限</span>
        <span class="perm-state">状态</span>
        <span class="perm-ops">操作</span>
      </div>
      <div v-if="loading" class="perm-loading">查询中…</div>
      <div v-for="p in rows" :key="p.key" class="perm-row">
        <span class="perm-name">
          {{ labelOf(p.key) }}
          <span v-if="p.key === 'location'" class="perm-sub">（首次定位时弹）</span>
        </span>
        <span class="perm-state">
          <el-tag size="small" :type="tagOf(p.state)">{{ stateLabel(p.state) }}</el-tag>
        </span>
        <span class="perm-ops">
          <template v-if="p.state !== 'granted'">
            <el-button
              v-if="p.state === 'notDetermined' && p.interactiveSupported"
              size="small"
              type="primary"
              :loading="requesting === p.key"
              @click="onRequest(p.key)"
            >
              申请
            </el-button>
            <el-button size="small" @click="onOpenSettings(p.key)">系统设置</el-button>
          </template>
          <span v-else class="perm-done">✓</span>
        </span>
      </div>

      <div class="perm-footer-ops">
        <el-button :loading="requestingAll" @click="onRequestAll">一键申请（未授权项）</el-button>
        <span class="perm-hint">申请后系统弹框挂起，授权后本页 2s 内自动变绿</span>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import {
  clientIsDevBuild,
  getPermissionStatuses,
  openPermissionSettings,
  requestPermission,
  type PermStatus,
} from '../api';

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>();

const visible = ref(props.modelValue);
watch(
  () => props.modelValue,
  (v) => {
    visible.value = v;
    if (v) startPolling();
    else stopPolling();
  }
);
watch(visible, (v) => emit('update:modelValue', v));

const loading = ref(false);
const rows = ref<PermStatus[]>([]);
const requesting = ref(''); // 正在申请的 key（单行按钮 loading）
const requestingAll = ref(false); // 一键申请进行中
const isDev = ref(false);

const LABELS: Record<string, string> = {
  screen: '屏幕录制',
  accessibility: '辅助功能',
  location: '定位',
  contacts: '通讯录',
  calendar: '日历',
  reminders: '提醒事项',
  photos: '照片',
  camera: '摄像头',
  notifications: '通知',
};
const labelOf = (key: string) => LABELS[key] ?? key;

const stateLabel = (s: string) => {
  if (s === 'granted') return '已授权';
  if (s === 'notDetermined') return '未申请';
  if (s === 'denied') return '已拒绝';
  if (s === 'restricted') return '受限';
  return '未知';
};
const tagOf = (s: string) => {
  if (s === 'granted') return 'success';
  if (s === 'notDetermined') return 'warning';
  if (s === 'denied' || s === 'restricted') return 'danger';
  return 'info';
};

const refresh = async () => {
  try {
    rows.value = await getPermissionStatuses();
    loading.value = false;
  } catch {
    // 轮询失败静默（win / 未启用 feature 的构建误开时行数据为空）
  }
};

// 打开即拉 + 2s 轮询：用户在系统弹框/系统设置里授权后自动变绿；关闭即停
let timer: ReturnType<typeof setInterval> | null = null;
const startPolling = () => {
  loading.value = rows.value.length === 0;
  void refresh();
  if (timer) clearInterval(timer);
  timer = setInterval(refresh, 2000);
};
const stopPolling = () => {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
};

const onRequest = async (key: string) => {
  requesting.value = key;
  try {
    await requestPermission(key);
    await refresh();
  } catch {
    ElMessage.error('权限申请失败');
  } finally {
    requesting.value = '';
  }
};

const onOpenSettings = async (key: string) => {
  try {
    await openPermissionSettings(key);
  } catch {
    ElMessage.error('打开系统设置失败');
  }
};

// 串行逐项申请（可申请且未授权的），避免系统弹框叠一堆；拒绝过的直接跳去系统设置
const onRequestAll = async () => {
  requestingAll.value = true;
  try {
    const targets = rows.value.filter(
      (p) => p.state === 'notDetermined' && p.interactiveSupported
    );
    for (const p of targets) {
      try {
        await requestPermission(p.key);
      } catch {
        // 单项失败继续下一项
      }
      await new Promise((r) => setTimeout(r, 300));
    }
    await refresh();
    if (targets.length === 0) ElMessage.info('没有可申请项（已全部授权 / 剩余项需去系统设置）');
  } finally {
    requestingAll.value = false;
  }
};

onMounted(async () => {
  isDev.value = await clientIsDevBuild().catch(() => false);
});

onBeforeUnmount(stopPolling);
</script>

<style scoped>
.perm-body {
  font-size: 13px;
  color: #4a5260;
}
.perm-desc {
  margin: 0 0 12px;
  line-height: 1.7;
  color: #6b7480;
}
.perm-dev-banner {
  margin-bottom: 12px;
}
.perm-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 4px;
  border-bottom: 1px solid #f0f2f5;
}
.perm-row-head {
  color: #9aa3af;
  font-size: 12px;
  border-bottom: 1px solid #e4e8ee;
}
.perm-name {
  flex: 1.2;
}
.perm-sub {
  color: #9aa3af;
  font-size: 11px;
}
.perm-state {
  flex: 0.8;
}
.perm-ops {
  flex: 1;
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  align-items: center;
}
.perm-done {
  color: #42b983;
  font-weight: 600;
}
.perm-loading {
  padding: 12px 4px;
  color: #9aa3af;
}
.perm-footer-ops {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
}
.perm-hint {
  font-size: 12px;
  color: #9aa3af;
}
</style>
