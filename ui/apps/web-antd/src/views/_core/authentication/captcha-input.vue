<script setup lang="ts">
import { computed } from 'vue';

import { $t } from '@vben/locales';

/**
 * 图形验证码输入组件（供 VbenForm schema 字段使用）
 * - modelValue: 验证码输入值（verifyCode），由 VbenForm 双向绑定
 * - captcha: 父级持有的 reactive { key, img }，图片地址响应式
 * - refresh: 点击图片触发父级重新拉取验证码
 */
interface Props {
  modelValue?: string;
  captcha?: { key: string; img: string };
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  captcha: () => ({ key: '', img: '' }),
});

const emit = defineEmits<{
  'update:modelValue': [string];
  refresh: [];
}>();

const value = computed<string>({
  get: () => props.modelValue ?? '',
  set: (v) => emit('update:modelValue', v),
});
</script>

<template>
  <div class="flex w-full items-center gap-2">
    <input
      v-model="value"
      type="text"
      autocomplete="off"
      :placeholder="$t('page.auth.captchaPlaceholder')"
      class="h-10 flex-1 rounded-md border border-solid border-input bg-background px-3 text-sm outline-none focus:border-primary"
    />
    <img
      v-if="captcha?.img"
      :src="captcha.img"
      alt="captcha"
      :title="$t('page.auth.captchaRefresh')"
      class="h-10 w-28 cursor-pointer rounded-md border border-solid border-input object-contain"
      @click="emit('refresh')"
    />
    <div
      v-else
      :title="$t('page.auth.captchaGet')"
      class="flex h-10 w-28 cursor-pointer items-center justify-center rounded-md border border-dashed border-input text-xs text-muted-foreground"
      @click="emit('refresh')"
    >
      {{ $t('page.auth.captchaGet') }}
    </div>
  </div>
</template>
