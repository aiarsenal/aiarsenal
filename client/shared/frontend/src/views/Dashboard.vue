<template>
  <div class="dashboard">
    <div class="bg-card">
      <div class="bg-logo">AI Arsenal</div>
      <div class="bg-title">已在后台运行</div>
      <div class="bg-desc">
        客户端正在后台上报状态，无需保持窗口可见。<br />
        如需操作 agent（安装 / 启动 / 重启等），请在管理后台「设备管理」中下发指令。
      </div>
      <div class="bg-meta">
        <span>{{ displayName }}</span>
        <el-tag size="small" :type="env === 'prod' ? 'danger' : 'info'" effect="plain">
          {{ envLabel }}
        </el-tag>
      </div>
      <div class="bg-keepawake">
        <span>防止锁屏</span>
        <el-switch v-model="keepAwake" @change="onToggleKeepAwake" />
        <span class="bg-keepawake-hint">开启时本机不休眠 / 不锁屏（openclaw / hermes 可持续操作）</span>
      </div>
      <!-- exec 审批档位（mac）：agent 经 system.run 执行命令的闸门，策略文件与 openclaw CLI 互操作 -->
      <div v-if="isMac && execPolicy" class="bg-exec">
        <div class="bg-exec-head">
          <span>执行审批</span>
          <el-select :model-value="execPolicy.mode" size="small" style="width: 130px" @change="onExecModeChange">
            <el-option label="全部放行" value="full" />
            <el-option label="白名单外询问" value="ask" />
            <el-option label="白名单外拒绝" value="allowlist" />
            <el-option label="全部拒绝" value="deny" />
          </el-select>
        </div>
        <p class="bg-exec-desc">{{ execDesc }}</p>
        <p class="bg-exec-meta">白名单 {{ execPolicy.allowlistCount }} 条 · {{ execPolicy.path }}</p>
      </div>
      <div class="bg-actions">
        <el-button v-if="isMac" @click="permVisible = true">系统权限</el-button>
        <el-button @click="mcpVisible = true">MCP 管理</el-button>
        <el-button type="primary" @click="onHide">隐藏到托盘</el-button>
        <el-button @click="onLogout">退出登录</el-button>
      </div>
    </div>

    <!-- 权限清单页（mac）：9 项 TCC 权限状态 + 一键申请（win 无 desktop node，隐藏入口） -->
    <PermissionDialog v-if="isMac" v-model="permVisible" />
    <!-- MCP 分组管理：绑定/解绑本设备的 MCP 分组（win/mac 都注册命令） -->
    <McpDialog v-model="mcpVisible" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import {
  clientGetEnv,
  clientGetKeepAwake,
  clientGetExecPolicy,
  clientHideWindow,
  clientLogout,
  clientSetKeepAwake,
  clientSetExecPolicy,
  type ExecPolicy,
} from '../api';
import PermissionDialog from '../components/PermissionDialog.vue';
import McpDialog from '../components/McpDialog.vue';

// win/mac 共用前端：权限清单页只在 mac 注册命令，win 上隐藏入口
const isMac = /Mac/i.test(navigator.userAgent);
const permVisible = ref(false);
const mcpVisible = ref(false);

// exec 审批档位（mac only；win 无 desktop node 不注册命令）
const execPolicy = ref<ExecPolicy | null>(null);
const execDesc = computed(() => {
  switch (execPolicy.value?.mode) {
    case 'full':
      return '全部放行（与现状一致，零拦截）';
    case 'ask':
      return '白名单命中静默放行，未命中弹窗询问（60s 无回应自动拒绝）';
    case 'allowlist':
      return '只跑白名单命令，未命中直接拒绝';
    case 'deny':
      return '拒绝一切命令执行';
    default:
      return '';
  }
});

const onExecModeChange = async (mode: string) => {
  const prev = execPolicy.value;
  try {
    execPolicy.value = await clientSetExecPolicy(mode);
    ElMessage.success(`执行审批已切至「${mode}」`);
  } catch (e) {
    execPolicy.value = prev; // 写失败回填旧档
    ElMessage.error(`切档失败：${e}`);
  }
};

const emit = defineEmits<{ (e: 'logout'): void }>();

const env = ref('test');
const displayName = ref('');
const envLabel = ref('测试');
const keepAwake = ref(true);

const refresh = async () => {
  const info = await clientGetEnv();
  env.value = info.env;
  // 环境编译期烘焙（test/prod/custom），运行时不可切；非 test/prod 直接显示构建名
  envLabel.value = info.env === 'prod' ? '生产' : info.env === 'test' ? '测试' : info.env;
  displayName.value = info.name || info.userName || '';
  keepAwake.value = await clientGetKeepAwake().catch(() => true);
};

const onToggleKeepAwake = async (v: boolean | string | number) => {
  const next = Boolean(v);
  try {
    keepAwake.value = await clientSetKeepAwake(next);
  } catch {
    // 设置失败（如 mac 起不来 caffeinate）：回填真实状态
    keepAwake.value = await clientGetKeepAwake().catch(() => !next);
  }
};

const onHide = async () => {
  await clientHideWindow();
};

const onLogout = async () => {
  await clientLogout();
  emit('logout');
};

onMounted(() => {
  refresh();
  // mac 才拉档位（win 未注册命令，invoke 会报错）
  if (isMac) clientGetExecPolicy().then((p) => (execPolicy.value = p)).catch(() => {});
});
</script>

<style scoped>
.dashboard {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
}
.bg-card {
  width: 460px;
  max-width: 100%;
  padding: 36px 32px;
  background: #fff;
  border-radius: 14px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.06);
  text-align: center;
}
.bg-logo {
  font-weight: 700;
  font-size: 22px;
  color: #42b983;
  margin-bottom: 14px;
}
.bg-title {
  font-size: 18px;
  font-weight: 600;
  color: #1f2933;
  margin-bottom: 10px;
}
.bg-desc {
  font-size: 13px;
  line-height: 1.7;
  color: #6b7480;
  margin-bottom: 18px;
}
.bg-meta {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  color: #4a5260;
  margin-bottom: 14px;
}
.bg-keepawake {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  color: #4a5260;
  margin-bottom: 22px;
}
.bg-keepawake-hint {
  font-size: 12px;
  color: #9aa3af;
}
.bg-exec {
  margin: 0 0 22px;
  padding: 12px 14px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  text-align: left;
}
.bg-exec-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: #4a5260;
}
.bg-exec-desc {
  margin: 8px 0 4px;
  font-size: 12px;
  color: #6b7480;
  line-height: 1.6;
}
.bg-exec-meta {
  margin: 0;
  font-size: 11px;
  color: #9aa3af;
  word-break: break-all;
}
.bg-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
}
</style>
