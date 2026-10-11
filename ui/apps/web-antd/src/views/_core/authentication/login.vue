<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';
import type { Recordable } from '@vben/types';

import { computed, markRaw, onMounted, reactive } from 'vue';

import { AuthenticationLogin, z } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { useAuthStore } from '#/store';
import { getCaptchaApi } from '#/api';

import CaptchaInput from './captcha-input.vue';

defineOptions({ name: 'Login' });

const authStore = useAuthStore();

// gin 图形验证码：{ key, img }，verifyKey 随登录提交
const captcha = reactive({ key: '', img: '' });

async function refreshCaptcha() {
  try {
    const res = await getCaptchaApi();
    captcha.key = res?.key ?? '';
    captcha.img = res?.img ?? '';
  } catch {
    /* 忽略：后端不可达时图片区显示「点击获取」可重试 */
  }
}

onMounted(refreshCaptcha);

const formSchema = computed((): VbenFormSchema[] => {
  return [
    {
      component: 'VbenInput',
      componentProps: {
        placeholder: $t('authentication.usernameTip'),
      },
      fieldName: 'username',
      label: $t('authentication.username'),
      rules: z.string().min(1, { message: $t('authentication.usernameTip') }),
    },
    {
      component: 'VbenInputPassword',
      componentProps: {
        placeholder: $t('authentication.password'),
      },
      fieldName: 'password',
      label: $t('authentication.password'),
      rules: z.string().min(1, { message: $t('authentication.passwordTip') }),
    },
    {
      component: markRaw(CaptchaInput),
      componentProps: {
        captcha,
        onRefresh: refreshCaptcha,
      },
      fieldName: 'verifyCode',
      label: $t('page.auth.captcha'),
      rules: z.string().min(1, { message: $t('page.auth.captchaRequired') }),
    },
  ];
});

// 提交：把 captcha key 作为 verifyKey 一并传入（非用户输入字段）
function onSubmit(values: Recordable<any>) {
  authStore.authLogin({ ...values, verifyKey: captcha.key });
}
</script>

<template>
  <AuthenticationLogin
    :form-schema="formSchema"
    :loading="authStore.loginLoading"
    :show-code-login="false"
    :show-qrcode-login="false"
    :show-register="false"
    :show-forget-password="false"
    :show-third-party-login="false"
    @submit="onSubmit"
  />
</template>
