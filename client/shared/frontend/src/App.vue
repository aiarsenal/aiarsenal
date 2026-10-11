<template>
  <div class="app-root">
    <header class="app-header">
      <div class="brand">
        <span class="logo">AI Arsenal</span>
      </div>
      <div class="header-right">
        <template v-if="loggedIn">
          <span class="user">{{ displayName }}<template v-if="employeeNo">（{{ employeeNo }}）</template></span>
          <el-button size="small" link @click="onLogout">退出</el-button>
        </template>
      </div>
    </header>

    <main class="app-main">
      <div v-if="booting" class="booting">加载中…</div>
      <LoginWait v-else-if="!loggedIn" :auto-start="autoStart" @logged-in="onLoggedIn" />
      <Dashboard v-else @logout="onLogout" />
    </main>

    <!-- openclaw 版本对齐确认：install 检测到本机已有 openclaw 且版本 ≠ 目标时弹出。
         Rust 侧把主窗口从托盘拉起来 emit 事件；确认/拒绝回填后命令继续或就地终止。 -->
    <el-dialog
      v-model="alignVisible"
      title="openclaw 版本对齐确认"
      width="520px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
    >
      <div class="align-body">
        <p>检测到本机已有 openclaw：</p>
        <ul class="align-found">
          <li v-for="f in alignReq?.found ?? []" :key="f.path">
            <el-tag size="small" effect="plain">{{ originLabel(f.origin) }}</el-tag>
            <span class="align-ver">{{ f.version || '未知版本' }}</span>
            <span class="align-path">{{ f.path }}</span>
          </li>
        </ul>
        <p>
          AI Arsenal 客户端需要把 openclaw 对齐到
          <b>{{ alignReq?.target }}</b>（{{ actionLabel }}，当前 {{ alignReq?.current || '未知' }}）。
        </p>
        <p class="align-note">{{ alignReq?.note }}</p>
      </div>
      <template #footer>
        <el-button @click="onAlignRespond(false)">拒绝（不做任何改动）</el-button>
        <el-button type="primary" @click="onAlignRespond(true)">确认{{ actionLabel }}</el-button>
      </template>
    </el-dialog>

    <!-- exec 审批：agent 要经 system.run 执行命令且非白名单命中（ask 档）时弹出。
         Rust 侧 2s 重发同 id 事件（前端幂等刷新倒计时）；60s 无回应后端自动拒绝。 -->
    <el-dialog
      v-model="execVisible"
      title="执行审批请求"
      width="560px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
    >
      <div class="exec-body">
        <p class="exec-warn">Agent 请求在本机执行以下命令：</p>
        <pre class="exec-cmd">{{ execReq?.commandText }}</pre>
        <p v-if="execReq?.resolvedPath" class="exec-meta">解析路径：{{ execReq.resolvedPath }}</p>
        <p v-if="execReq?.cwd" class="exec-meta">工作目录：{{ execReq.cwd }}</p>
        <el-progress
          :percentage="execCountdownPct"
          :stroke-width="6"
          :show-text="false"
          :color="execCountdownPct < 25 ? '#f56c6c' : '#42b983'"
        />
        <p class="exec-meta">{{ execCountdown }} 秒后自动拒绝</p>
      </div>
      <template #footer>
        <el-button @click="onExecRespond('deny')">拒绝</el-button>
        <el-button type="warning" @click="onExecRespond('always')">总是允许</el-button>
        <el-button type="primary" @click="onExecRespond('once')">允许一次</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import LoginWait from './views/LoginWait.vue';
import Dashboard from './views/Dashboard.vue';
import {
  clientGetEnv,
  clientLogout,
  clientConfirmOpenclawAlign,
  onForceLogout,
  onOpenclawAlignConfirm,
  onExecApprovalRequest,
  respondExecApproval,
  type OpenclawAlignReq,
  type ExecApprovalReq,
} from './api';

// booting：启动时先读持久态，避免已有登录态还闪一下 LoginWait（会自动开浏览器）
const booting = ref(true);
const loggedIn = ref(false);
// 被动撤销登录（force-logout）回到登录页时不主动弹浏览器；主动退出/首次启动默认主动弹。
const autoStart = ref(true);
const userName = ref('');
const name = ref('');
const employeeNo = ref('');
const userId = ref('');

// 右上角显示真实姓名（工号），非昵称；姓名缺失回退到登录账号
const displayName = computed(() => name.value || userName.value);

const refreshEnv = async () => {
  const info = await clientGetEnv();
  userName.value = info.userName;
  name.value = info.name || '';
  employeeNo.value = info.employeeNo || '';
  userId.value = info.userId;
  loggedIn.value = info.loggedIn;
  booting.value = false;
};

const onLoggedIn = async () => {
  autoStart.value = true; // 登录成功后重置：下次主动退出/撤销按各自场景设置
  await refreshEnv();
};
const onLogout = async () => {
  await clientLogout();
  loggedIn.value = false;
  userName.value = '';
  name.value = '';
  employeeNo.value = '';
};

onMounted(async () => {
  await refreshEnv();
  // 设备登录态被服务端撤销（管理员在 /client/device 点【撤销登录】）：Rust 心跳收到 401
  // → state.logout() 清 token + emit("force-logout")，前端回到登录页并提示重新授权。
  onForceLogout(() => {
    loggedIn.value = false;
    autoStart.value = false; // 被动撤销：回登录页不主动弹浏览器，等用户手动点「开始登录」
    userName.value = '';
    name.value = '';
    employeeNo.value = '';
    ElMessage.error('该设备登录已被管理员撤销，请重新授权登录');
  });
  // openclaw 版本对齐确认（install 命令等待本机用户回应；10 分钟超时自动拒绝）
  onOpenclawAlignConfirm((p) => {
    alignReq.value = p;
    alignVisible.value = true;
  });
  // exec 审批（全局：Rust 每 2s 重发同 id，这里幂等刷新数据 + 倒计时）
  onExecApprovalRequest((p) => {
    execReq.value = p;
    execVisible.value = true;
    startExecTimer();
  });
});

onUnmounted(() => {
  stopExecTimer();
});

// ==================== openclaw 版本对齐确认弹窗 ====================
const alignVisible = ref(false);
const alignReq = ref<OpenclawAlignReq | null>(null);

const actionLabel = computed(() => {
  const a = alignReq.value?.action;
  if (a === 'upgrade') return '升级';
  if (a === 'downgrade') return '降级';
  return '重装';
});

const originLabel = (origin: string) => {
  if (origin === 'runtime') return '本客户端管理的 runtime';
  if (origin === 'npm-global') return 'npm 全局安装';
  if (origin === 'app') return 'OpenClaw.app';
  return origin;
};

const onAlignRespond = async (accept: boolean) => {
  try {
    await clientConfirmOpenclawAlign(accept);
  } catch {
    // 回填失败（如已超时自动终止）：关闭弹窗即可，命令侧按超时处理
  }
  alignVisible.value = false;
  alignReq.value = null;
};

// ==================== exec 审批弹窗（全局：登录页/任意页面都可能来） ====================
const execVisible = ref(false);
const execReq = ref<ExecApprovalReq | null>(null);
// 倒计时（expiresAt 是 Rust 侧发事件时算的 60s 死线；重发事件刷新）
const execCountdown = ref(0);
const execCountdownPct = computed(() => (execCountdown.value / 60) * 100);
let execTimer: number | null = null;

const stopExecTimer = () => {
  if (execTimer !== null) {
    clearInterval(execTimer);
    execTimer = null;
  }
};
const startExecTimer = () => {
  stopExecTimer();
  execTimer = window.setInterval(() => {
    const left = Math.max(0, Math.ceil(((execReq.value?.expiresAt ?? 0) - Date.now()) / 1000));
    execCountdown.value = left;
    if (left === 0) {
      // 后端同刻会超时拒绝；这里只收弹窗（回应接口此刻已无待审批，返回 false 无害）
      stopExecTimer();
      execVisible.value = false;
    }
  }, 500);
};

const onExecRespond = async (decision: 'once' | 'always' | 'deny') => {
  const id = execReq.value?.id;
  stopExecTimer();
  execVisible.value = false;
  execReq.value = null;
  if (!id) return;
  try {
    await respondExecApproval(id, decision);
  } catch {
    // 回填失败（已超时自动拒绝）：无需提示，后端已按拒绝处理
  }
};
</script>

<style scoped>
.app-root {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #f5f7fa;
}
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  height: 52px;
  background: #1f2933;
  color: #fff;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.logo {
  font-weight: 700;
  font-size: 16px;
  color: #42b983;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.header-right .user {
  font-size: 13px;
  color: #c0c8d4;
}
.app-main {
  flex: 1;
  overflow: auto;
}
.booting {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8a94a6;
  font-size: 13px;
}
.align-body {
  font-size: 13px;
  color: #4a5260;
  line-height: 1.7;
}
.align-found {
  margin: 0 0 10px;
  padding-left: 18px;
}
.align-found li {
  list-style: disc;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.align-ver {
  font-weight: 600;
}
.align-path {
  color: #9aa3af;
  font-size: 12px;
  word-break: break-all;
}
.align-note {
  color: #9aa3af;
  font-size: 12px;
}
.exec-body {
  font-size: 13px;
  color: #4a5260;
}
.exec-warn {
  margin: 0 0 8px;
  font-weight: 600;
}
.exec-cmd {
  margin: 0 0 10px;
  padding: 10px 12px;
  background: #1f2933;
  color: #e6edf3;
  border-radius: 6px;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}
.exec-meta {
  margin: 4px 0;
  color: #9aa3af;
  font-size: 12px;
  word-break: break-all;
}
</style>
