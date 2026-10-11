<template>
  <div class="login-wait">
    <div class="card">
      <div v-if="started" class="spinner" />
      <h2>请在浏览器中完成登录</h2>
      <p class="desc">
        <template v-if="started">已为你打开系统浏览器登录页。完成授权后将自动返回本应用。</template>
        <template v-else>点击下方按钮开始登录，完成浏览器授权后将自动返回本应用。</template>
      </p>
      <p v-if="state.deviceCode" class="dc">deviceCode: {{ state.deviceCode }}</p>
      <p v-if="state.error" class="error">{{ state.error }}</p>
      <el-button v-if="!started" type="primary" size="small" @click="start">
        {{ state.error ? '重试' : '开始登录' }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue';
import {
  clientExchange,
  clientStartLogin,
  onDeepLink,
  openLoginPage,
} from '../api';
import type { UnlistenFn } from '@tauri-apps/api/event';

const props = defineProps<{ autoStart?: boolean }>();
const emit = defineEmits<{ (e: 'logged-in'): void }>();

const state = reactive({
  deviceCode: '',
  error: '',
  polling: false,
});

let pollTimer: ReturnType<typeof setInterval> | null = null;
let unlisten: UnlistenFn | null = null;
// 防止 start() 重复执行（组件重挂载/重复触发时只开一次浏览器）——修复"跳转2次登录"。
// 响应式：template 据 started 决定显示「开始登录」按钮还是 spinner。
const started = ref(false);
// 已通过某 dc 换 token 成功，忽略后续 deep-link/轮询的重复 exchange
let done = false;

const finishLogin = () => {
  if (done) return;
  done = true;
  stopPolling();
  emit('logged-in');
};

const start = async () => {
  if (started.value) return;
  started.value = true;
  state.error = '';
  try {
    const dc = await clientStartLogin();
    state.deviceCode = dc;
    await openLoginPage(dc);
    startPolling(dc);
  } catch (e) {
    state.error = `发起登录失败: ${e}`;
    started.value = false; // 失败允许重试
  }
};

const startPolling = (dc: string) => {
  if (pollTimer) clearInterval(pollTimer);
  state.polling = true;
  const tick = async () => {
    if (done) return;
    try {
      const res = await clientExchange(dc);
      if (res.status === 'ok') {
        finishLogin();
      }
    } catch (e) {
      // 轮询期间网络错误不中断，继续重试
      state.error = `轮询失败: ${e}`;
    }
  };
  tick();
  pollTimer = setInterval(tick, 2000);
};

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  state.polling = false;
};

onMounted(async () => {
  // deep-link 回调：浏览器重定向 aiarsenal://login?dc=... 触发，立即换 token
  unlisten = await onDeepLink(async (dc) => {
    if (done) return;
    try {
      const res = await clientExchange(dc);
      if (res.status === 'ok') {
        finishLogin();
      }
    } catch (e) {
      state.error = `回调换 token 失败: ${e}`;
    }
  });
  // autoStart=false（如设备被管理员撤销登录，被动回到本页）时不主动打开浏览器，
  // 等用户点「开始登录」；主动退出/首次启动默认 autoStart=true 主动发起。
  if (props.autoStart !== false) {
    await start();
  }
});

onUnmounted(() => {
  stopPolling();
  if (unlisten) unlisten();
});
</script>

<style scoped>
.login-wait {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 40px;
}
.card {
  text-align: center;
  max-width: 420px;
}
.spinner {
  width: 44px;
  height: 44px;
  margin: 0 auto 18px;
  border: 4px solid #e3e7ee;
  border-top-color: #42b983;
  border-radius: 50%;
  animation: rot 1s linear infinite;
}
@keyframes rot {
  to {
    transform: rotate(360deg);
  }
}
h2 {
  font-size: 18px;
  color: #303133;
  margin: 0 0 8px;
}
.desc {
  color: #8a94a6;
  font-size: 13px;
  line-height: 1.6;
}
.dc {
  margin-top: 10px;
  font-size: 12px;
  color: #aab2c0;
  word-break: break-all;
}
.error {
  margin-top: 10px;
  font-size: 12px;
  color: #f56c6c;
}
</style>
