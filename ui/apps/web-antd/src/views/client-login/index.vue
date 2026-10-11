<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';
import type { Recordable } from '@vben/types';

import { computed, markRaw, onMounted, reactive } from 'vue';
import { useRoute } from 'vue-router';

import { AuthenticationLogin, z } from '@vben/common-ui';
import { $t } from '@vben/locales';
import { useAccessStore } from '@vben/stores';
import { preferences } from '@vben/preferences';

import { notification } from 'ant-design-vue';

import { useAuthStore } from '#/store';
import { clientBindApi, getCaptchaApi } from '#/api';

import CaptchaInput from '../_core/authentication/captcha-input.vue';

defineOptions({ name: 'ClientLogin' });

const route = useRoute();
const authStore = useAuthStore();
const accessStore = useAccessStore();

// deviceCode 来自 App 打开浏览器时的 ?dc=...
const deviceCode = computed(() => {
  const dc = route.query.dc;
  return Array.isArray(dc) ? (dc[0] ?? '') : (dc ?? '');
});

// phase: form（填表登录）| done（已绑定，跳回 App）| expired（dc 已被消费，失效）| error（无 dc / 绑定失败）
const phase = reactive({
  state: 'form' as 'form' | 'done' | 'expired' | 'error',
});

// 防同一页面会话内重复跳转（F5 刷新会重置，从而重新走 Bind 命中「已用」检测）
let redirected = false;

// gin 图形验证码
const captcha = reactive({ key: '', img: '' });

async function refreshCaptcha() {
  try {
    const res = await getCaptchaApi();
    captcha.key = res?.key ?? '';
    captcha.img = res?.img ?? '';
  } catch {
    /* 忽略 */
  }
}

// 绑定 deviceCode 并跳回桌面客户端（自定义 scheme）
async function bindAndRedirect() {
  if (!deviceCode.value) {
    phase.state = 'error';
    return;
  }
  let res: any;
  try {
    res = await clientBindApi(deviceCode.value);
  } catch {
    notification.warning({ message: $t('core.clientlogin.bindFailed') });
    phase.state = 'error';
    return;
  }
  // dc 一次性：已被 App 端 exchange 消费 → 判失效，不再绑定/跳转
  if (res?.expired) {
    phase.state = 'expired';
    return;
  }
  if (redirected) return;
  redirected = true;
  phase.state = 'done';
  // 跳回桌面客户端：aiarsenal://login?dc=<deviceCode>
  window.location.href = `aiarsenal://login?dc=${encodeURIComponent(deviceCode.value)}`;
}

const formSchema = computed((): VbenFormSchema[] => [
  {
    component: 'VbenInput',
    componentProps: { placeholder: $t('authentication.usernameTip') },
    fieldName: 'username',
    label: $t('authentication.username'),
    rules: z.string().min(1, { message: $t('authentication.usernameTip') }),
  },
  {
    component: 'VbenInputPassword',
    componentProps: { placeholder: $t('authentication.password') },
    fieldName: 'password',
    label: $t('authentication.password'),
    rules: z.string().min(1, { message: $t('authentication.passwordTip') }),
  },
  {
    component: markRaw(CaptchaInput),
    componentProps: { captcha, onRefresh: refreshCaptcha },
    fieldName: 'verifyCode',
    label: $t('page.auth.captcha'),
    rules: z.string().min(1, { message: $t('page.auth.captchaRequired') }),
  },
]);

// 提交：登录成功 → 绑定 + 跳回（onSuccess 取代默认的「跳首页」）
function onSubmit(values: Recordable<any>) {
  authStore.authLogin({ ...values, verifyKey: captcha.key }, bindAndRedirect);
}

onMounted(async () => {
  if (!deviceCode.value) {
    phase.state = 'error';
    return;
  }
  // 浏览器已登录（本会话已有 admin token）：直接绑定跳回，无需再填表单
  if (accessStore.accessToken) {
    await bindAndRedirect();
    return;
  }
  await refreshCaptcha();
});
</script>

<template>
  <div class="client-login">
    <img
      :src="preferences.logo.source"
      alt="AI Arsenal"
      class="brand-logo"
      draggable="false"
    />
    <div class="box">
      <div class="header">
        <div class="subtitle">{{ $t('core.clientlogin.subtitle') }}</div>
      </div>

      <div v-if="phase.state === 'done'" class="panel">
        <svg class="icon ok" viewBox="0 0 1024 1024" aria-hidden="true">
          <path
            fill="currentColor"
            d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm193.5 305.7L448 616.4 338.2 506.6a32 32 0 1 0-45.3 45.3l132.8 132.8a32 32 0 0 0 45.3 0l289.9-289.9a32 32 0 1 0-45.4-45.1z"
          />
        </svg>
        <p class="title">{{ $t('core.clientlogin.successTitle') }}</p>
        <p class="desc">{{ $t('core.clientlogin.successDesc') }}</p>
        <p class="desc small">
          {{ $t('core.clientlogin.manualReturn') }}<br />
          <a :href="`aiarsenal://login?dc=${encodeURIComponent(deviceCode)}`">
            aiarsenal://login?dc={{ deviceCode }}
          </a>
        </p>
      </div>

      <div v-else-if="phase.state === 'expired'" class="panel">
        <svg class="icon warn" viewBox="0 0 1024 1024" aria-hidden="true">
          <path
            fill="currentColor"
            d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm0 384a38.4 38.4 0 0 0-38.4 38.4v166.4a38.4 38.4 0 0 0 76.8 0V486.4A38.4 38.4 0 0 0 512 448zm0-192a44.8 44.8 0 1 0 0 89.6 44.8 44.8 0 0 0 0-89.6z"
          />
        </svg>
        <p class="title">{{ $t('core.clientlogin.expiredTitle') }}</p>
        <p class="desc">{{ $t('core.clientlogin.expiredDesc') }}</p>
      </div>

      <div v-else-if="phase.state === 'error'" class="panel">
        <svg class="icon warn" viewBox="0 0 1024 1024" aria-hidden="true">
          <path
            fill="currentColor"
            d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm0 384a38.4 38.4 0 0 0-38.4 38.4v166.4a38.4 38.4 0 0 0 76.8 0V486.4A38.4 38.4 0 0 0 512 448zm0-192a44.8 44.8 0 1 0 0 89.6 44.8 44.8 0 0 0 0-89.6z"
          />
        </svg>
        <p class="title">{{ $t('core.clientlogin.errorTitle') }}</p>
        <p class="desc">{{ $t('core.clientlogin.errorDesc') }}</p>
      </div>

      <AuthenticationLogin
        v-else
        :form-schema="formSchema"
        :loading="authStore.loginLoading"
        :show-code-login="false"
        :show-qrcode-login="false"
        :show-register="false"
        :show-forget-password="false"
        :show-third-party-login="false"
        :submit-button-text="$t('core.clientlogin.submitText')"
        @submit="onSubmit"
      />

      <p v-if="phase.state === 'form'" class="tip">
        {{ $t('core.clientlogin.tip') }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.client-login {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 24px;
  background: linear-gradient(135deg, #1f2933 0%, #314355 100%);
}
.brand-logo {
  width: 240px;
  max-width: 60vw;
  height: auto;
}
.box {
  width: 380px;
  max-width: calc(100vw - 32px);
  padding: 36px 32px 28px;
  background: #fff;
  border-radius: 14px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.28);
}
.header {
  text-align: center;
  margin-bottom: 22px;
}
.logo {
  width: 160px;
  max-width: 100%;
  height: auto;
  margin: 0 auto;
  display: block;
}
.subtitle {
  margin-top: 6px;
  font-size: 13px;
  color: #8a94a6;
}
.panel {
  text-align: center;
  padding: 20px 0;
}
.icon {
  width: 48px;
  height: 48px;
}
.icon.ok {
  color: #42b983;
}
.icon.warn {
  color: #e6a23c;
}
.title {
  margin: 14px 0 6px;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}
.desc {
  font-size: 13px;
  color: #8a94a6;
  line-height: 1.6;
}
.desc.small {
  margin-top: 8px;
  font-size: 12px;
  word-break: break-all;
}
.tip {
  margin-top: 14px;
  font-size: 12px;
  color: #aab2c0;
  text-align: center;
  line-height: 1.6;
}
</style>
