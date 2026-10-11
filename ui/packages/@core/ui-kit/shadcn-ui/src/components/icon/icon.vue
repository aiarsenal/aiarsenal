<script setup lang="ts">
import type { Component } from 'vue';

import { computed } from 'vue';

import { IconDefault, IconifyIcon } from '@vben-core/icons';
import {
  isFunction,
  isHttpUrl,
  isObject,
  isString,
} from '@vben-core/shared/utils';

const props = defineProps<{
  // 没有是否显示默认图标
  fallback?: boolean;
  icon?: Component | Function | string;
}>();

const isRemoteIcon = computed(() => {
  return isString(props.icon) && isHttpUrl(props.icon);
});

const isComponent = computed(() => {
  const { icon } = props;
  return !isString(icon) && (isObject(icon) || isFunction(icon));
});

// 文本/emoji 图标：iconify 名都含 ':'(如 lucide:x)；不含 ':' 且非 URL 的字符串当文本渲染
// （后端菜单 icon 存的是 emoji 如 ⚙️📋，否则会被丢给 IconifyIcon 查无 → 不显示）
const isTextIcon = computed(() => {
  return (
    isString(props.icon) &&
    !isRemoteIcon.value &&
    !(props.icon as string).includes(':')
  );
});
</script>

<template>
  <component :is="icon as Component" v-if="isComponent" v-bind="$attrs" />
  <img v-else-if="isRemoteIcon" :src="icon as string" v-bind="$attrs" />
  <span
    v-else-if="isTextIcon"
    v-bind="$attrs"
    style="display: inline-flex; align-items: center; justify-content: center; line-height: 1"
    >{{ icon }}</span
  >
  <IconifyIcon v-else-if="icon" v-bind="$attrs" :icon="icon as string" />
  <IconDefault v-else-if="fallback" v-bind="$attrs" />
</template>
