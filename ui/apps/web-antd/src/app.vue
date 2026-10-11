<script lang="ts" setup>
import { computed } from 'vue';

import { useAntdDesignTokens } from '@vben/hooks';
import { preferences, usePreferences } from '@vben/preferences';

import { App, ConfigProvider, theme } from 'ant-design-vue';

import { antdLocale, useLocaleSync } from '#/locales';

defineOptions({ name: 'App' });

// 监听语言切换，实时改写菜单/路由标题（无需刷新页面）
useLocaleSync();

const { isDark } = usePreferences();
const { tokens } = useAntdDesignTokens();

/**
 * 形态类令牌覆盖（形状 / 尺寸 / 字体 / 动效，颜色无关）。
 * 颜色仍由 tokens（来自 --primary/--foreground/... 等 CSS 变量）提供，此处只覆盖形态类令牌，
 * 故不会改变品牌色与暗亮主题。
 * - 圆角：控件 6 / 小 4 / 容器 8（rounded-md=6、rounded-lg=8）
 * - 控件高度：36 / 32 / 40（按钮/输入 ~36、图标按钮 36×36）
 * - 字体族：Segoe UI Variable 栈
 * - 动效：150ms 变色过渡（transition-colors）
 */
const designStyleTokens = {
  borderRadius: 6,
  borderRadiusSM: 4,
  borderRadiusLG: 8,
  controlHeight: 36,
  controlHeightSM: 32,
  controlHeightLG: 40,
  // 小号组件（Tag、size="small" 按钮/输入等）字号提到 14，与 36px 控件文字同号，真正「配套」
  fontSizeSM: 14,
  // 弹窗/表单里偏小的元素配套放大（审计发现的漏网项，尺寸独立、不走 controlHeight）：
  fontSizeLG: 18, // 大号文字 + Radio 圆点 + 抽屉/卡片标题 + 大号控件文字
  fontSizeHeading5: 18, // Modal 标题
  controlInteractiveSize: 18, // Checkbox 方框 16→18
  fontFamily:
    "'Segoe UI Variable', 'Segoe UI', 'SF Pro Text', system-ui, sans-serif",
  motionDurationMid: '0.15s',
  motionDurationSlow: '0.2s',
} as const;

/**
 * 组件级令牌：Switch 高度在源码里 = fontSize*lineHeight、独立于 controlHeight 且
 * switchHeight 会被重算（无法直接覆盖），只能用组件级 fontSize 放大（高度+手柄按比例）。
 */
const componentTokens = {
  Switch: { fontSize: 18 },
} as const;

const tokenTheme = computed(() => {
  const algorithm = isDark.value
    ? [theme.darkAlgorithm]
    : [theme.defaultAlgorithm];

  // antd 紧凑模式算法
  if (preferences.app.compact) {
    algorithm.push(theme.compactAlgorithm);
  }

  // 全站背景对齐 landing：弹窗/抽屉/菜单/卡片/表格/列表页等 antd 容器底色统一为 landing 配色。
  // 亮：容器/弹层白 #ffffff、布局 #f9fafb（gray-50）；暗：容器/弹层 gray-900 #111827、布局 gray-950 #030712。
  // antd 用 CSS-in-JS 按 token 运行时注入，必须经 ConfigProvider token 覆盖，CSS 类改不到其内部。
  const bgTokens = isDark.value
    ? {
        colorBgContainer: '#111827',
        colorBgElevated: '#111827',
        colorBgLayout: '#030712',
        colorBgSpotlight: '#030712',
      }
    : {
        colorBgContainer: '#ffffff',
        colorBgElevated: '#ffffff',
        colorBgLayout: '#f9fafb',
        colorBgSpotlight: '#ffffff',
      };

  return {
    algorithm,
    token: { ...tokens, ...designStyleTokens, ...bgTokens },
    components: componentTokens,
  };
});
</script>

<template>
  <ConfigProvider :locale="antdLocale" :theme="tokenTheme">
    <App>
      <RouterView />
    </App>
  </ConfigProvider>
</template>
