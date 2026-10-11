<script lang="ts" setup>
import { computed, onMounted } from 'vue';

import { $t, loadLocaleMessages } from '@vben/locales';
import { preferences, updatePreferences, usePreferences } from '@vben/preferences';

import { Button, Dropdown, Menu, MenuItem, Tag } from 'ant-design-vue';

defineOptions({ name: 'Landing' });

const { isDark } = usePreferences();

// 落地页为纯项目介绍（开源定位：不提供托管服务/控制台入口），CTA 只指向 GitHub 仓库；
// 登录地址 /auth/login 由部署者自持（README「默认账号」节有说明）。
const REPO_URL = 'https://github.com/aiarsenal/aiarsenal';

// 核心能力：已上线模块（与 home 模块矩阵同色，--c 注入主题色）
const features = computed(() => [
  { icon: '⚙️', name: $t('page.landing.featureSystemName'), desc: $t('page.landing.featureSystemDesc'), color: '#3b82f6', status: 'online' },
  { icon: '🧩', name: $t('page.landing.featureSkillName'), desc: $t('page.landing.featureSkillDesc'), color: '#8b5cf6', status: 'online' },
  { icon: '🧠', name: $t('page.landing.featureLlmName'), desc: $t('page.landing.featureLlmDesc'), color: '#ec4899', status: 'online' },
  { icon: '🔌', name: $t('page.landing.featureMcpName'), desc: $t('page.landing.featureMcpDesc'), color: '#06b6d4', status: 'online' },
  { icon: '📑', name: $t('page.landing.featureDocsName'), desc: $t('page.landing.featureDocsDesc'), color: '#64748b', status: 'online' },
  { icon: '📋', name: $t('page.landing.roadmapProjectName'), desc: $t('page.landing.roadmapProjectDesc'), color: '#14b8a6', status: 'online' },
]);

// 路线图：在建/规划能力（developing/planned），带集成标签
const roadmap = computed(() => [
  { icon: '🤖', name: $t('page.landing.featureAgentName'), desc: $t('page.landing.featureAgentDesc'), color: '#10b981', status: 'developing', tags: ['Docker', 'CVM', $t('page.landing.tagRollingDeploy')] },
  { icon: '💻', name: $t('page.landing.roadmapClientName'), desc: $t('page.landing.roadmapClientDesc'), color: '#6366f1', status: 'developing', tags: ['Mac', 'Windows', $t('page.landing.tagChannelBind')] },
  { icon: '🌐', name: $t('page.landing.roadmapEcosystemName'), desc: $t('page.landing.roadmapEcosystemDesc'), color: '#f43f5e', status: 'planned', tags: [$t('page.landing.tagFeishu'), $t('page.landing.tagDingTalk'), $t('page.landing.tagTencentDocs')] },
  { icon: '⚡', name: $t('page.landing.roadmapGatewayName'), desc: $t('page.landing.roadmapGatewayDesc'), color: '#0ea5e9', status: 'planned', tags: ['Anthropic', 'Gemini', 'OAuth'] },
]);

// 状态徽标：与 home 一致用 antd Tag 配色（绿=上线/橙=开发中/蓝=规划中）
const statusMap = computed<Record<string, { color: string; text: string }>>(() => ({
  online: { color: 'magenta', text: $t('page.landing.statusOnline') },
  developing: { color: 'orange', text: $t('page.landing.statusDeveloping') },
  planned: { color: 'blue', text: $t('page.landing.statusPlanned') },
}));

// 技术栈：与 home 同色，--c 注入
const techStack = computed(() => [
  { name: 'GLM-5.2', logo: '🧠', desc: $t('page.landing.techGlmDesc'), color: '#6366f1' },
  { name: 'Gin', logo: '🐹', desc: $t('page.landing.techGinDesc'), color: '#06b6d4' },
  { name: 'GORM', logo: '🗂️', desc: $t('page.landing.techGormDesc'), color: '#10b981' },
  { name: 'PostgreSQL', logo: '🐘', desc: $t('page.landing.techPgDesc'), color: '#3b82f6' },
  { name: 'Redis', logo: '🟥', desc: $t('page.landing.techRedisDesc'), color: '#ef4444' },
  { name: 'Vue 3', logo: '⚡', desc: $t('page.landing.techVueDesc'), color: '#42b983' },
  { name: 'Vben Admin', logo: '🎨', desc: $t('page.landing.techVbenDesc'), color: '#8b5cf6' },
  { name: 'Tauri', logo: '🖥️', desc: $t('page.landing.techTauriDesc'), color: '#f59e0b' },
]);

const stats = computed(() => [
  { num: '9', label: $t('page.landing.statModules') },
  { num: '60+', label: $t('page.landing.statPages') },
  { num: '7', label: $t('page.landing.statOnline') },
]);

// 平滑滚动到页内区块。用 JS 而非 href="#xxx"：测试/生产是 hash 路由模式，
// href 锚点会被 vue-router 当成路由 /xxx 解析 → 跳 404。
function scrollToSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
}
async function onLang(lang: 'en-US' | 'zh-CN') {
  updatePreferences({ app: { locale: lang } });
  // 必须加载对应语言的消息表，否则 $t 仍用旧语言（这是 landing 切语言不生效的根因）
  await loadLocaleMessages(lang);
}
function onToggleDark() {
  updatePreferences({ theme: { mode: isDark.value ? 'light' : 'dark' } });
}
const langLabel = computed(() =>
  preferences.app.locale === 'zh-CN' ? $t('page.landing.langChinese') : $t('page.landing.langEnglish'),
);

// 版权文案（统一，不再区分环境/域名）
const copyrightText = '© 2021-2026 AI Arsenal. All Rights Reserved.';

onMounted(() => {
  document.title = 'AI Arsenal';
});
</script>

<template>
  <div class="min-h-screen bg-gradient-to-b from-white to-gray-50 dark:from-gray-900 dark:to-gray-950">
    <!-- Navbar -->
    <nav class="sticky top-0 z-10 border-b border-gray-100 bg-white/80 backdrop-blur dark:border-gray-800 dark:bg-gray-900/80">
      <div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        <div class="flex items-center gap-2">
          <img :src="preferences.logo.source" alt="logo" class="h-9 w-auto max-w-[150px] object-contain" />
          <span class="text-lg font-semibold">AI Arsenal</span>
        </div>
        <div class="hidden items-center gap-6 md:flex">
          <a href="#features" class="cursor-pointer text-sm text-gray-600 hover:text-primary dark:text-gray-300" @click.prevent="scrollToSection('features')">{{ $t('page.landing.navFeatures') }}</a>
          <a href="#roadmap" class="cursor-pointer text-sm text-gray-600 hover:text-primary dark:text-gray-300" @click.prevent="scrollToSection('roadmap')">{{ $t('page.landing.navRoadmap') }}</a>
          <a href="#tech" class="cursor-pointer text-sm text-gray-600 hover:text-primary dark:text-gray-300" @click.prevent="scrollToSection('tech')">{{ $t('page.landing.navTech') }}</a>
          <a href="#about" class="cursor-pointer text-sm text-gray-600 hover:text-primary dark:text-gray-300" @click.prevent="scrollToSection('about')">{{ $t('page.landing.navAbout') }}</a>
        </div>
        <div class="flex items-center gap-2">
          <Dropdown :trigger="['click']">
            <Button size="small">{{ langLabel }}</Button>
            <template #overlay>
              <Menu :selectable="false" @click="(e: any) => onLang(e.key)">
                <MenuItem key="zh-CN">{{ $t('page.landing.langChinese') }}</MenuItem>
                <MenuItem key="en-US">{{ $t('page.landing.langEnglish') }}</MenuItem>
              </Menu>
            </template>
          </Dropdown>
          <Button size="small" :title="isDark ? $t('page.landing.toggleLight') : $t('page.landing.toggleDark')" @click="onToggleDark">
            {{ isDark ? '☀️' : '🌙' }}
          </Button>
          <a :href="REPO_URL" target="_blank" rel="noopener"><Button type="primary" size="small">GitHub</Button></a>
        </div>
      </div>
    </nav>

    <!-- Hero -->
    <section class="mx-auto max-w-6xl px-4 py-20 text-center">
      <div class="mb-4 inline-block rounded-full bg-primary/10 px-4 py-1 text-sm text-primary">{{ $t('page.landing.heroBadge') }}</div>
      <h1 class="text-4xl font-bold leading-tight md:text-6xl">
        {{ $t('page.landing.heroTitle1') }}<br />
        <span class="text-primary">{{ $t('page.landing.heroTitle2') }}</span>
      </h1>
      <p class="mx-auto mt-6 max-w-3xl text-base text-gray-600 dark:text-gray-300 md:text-lg">
        {{ $t('page.landing.heroSubtitle') }}
      </p>
      <div class="mt-8 flex justify-center gap-4">
        <a :href="REPO_URL" target="_blank" rel="noopener"><Button type="primary" size="large">⭐ {{ $t('page.landing.heroRepo') }}</Button></a>
        <a href="#features" class="cursor-pointer" @click.prevent="scrollToSection('features')"><Button size="large">{{ $t('page.landing.learnMore') }}</Button></a>
      </div>
      <div class="mt-12 flex justify-center gap-12">
        <div v-for="s in stats" :key="s.label">
          <div class="text-2xl font-bold text-primary md:text-3xl">{{ s.num }}</div>
          <div class="text-sm text-gray-500">{{ s.label }}</div>
        </div>
      </div>
      <div class="mt-14 mx-auto max-w-3xl overflow-hidden rounded-2xl border border-gray-100 shadow-xl ring-1 ring-black/5 dark:border-gray-800">
        <video
          src="/brand.mp4"
          alt="AI Arsenal cover"
          class="w-full"
          autoplay
          loop
          muted
          playsinline
        />
      </div>
    </section>

    <!-- Features：已上线核心模块（彩色卡片，与 home 模块矩阵同款配色） -->
    <section id="features" class="mx-auto max-w-6xl px-4 py-16">
      <h2 class="text-center text-3xl font-bold">{{ $t('page.landing.featuresTitle') }}</h2>
      <p class="mt-2 text-center text-gray-500 dark:text-gray-400">{{ $t('page.landing.featuresSubtitle') }}</p>
      <div class="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="f in features"
          :key="f.name"
          :style="{ '--c': f.color }"
          class="module-card rounded-xl border border-solid bg-white p-6 transition dark:bg-gray-900"
        >
          <div class="mb-3 flex items-center justify-between">
            <span class="module-icon inline-flex h-12 w-12 items-center justify-center rounded-xl text-2xl">{{ f.icon }}</span>
            <Tag :color="statusMap[f.status]?.color">{{ statusMap[f.status]?.text }}</Tag>
          </div>
          <h3 class="text-lg font-semibold">{{ f.name }}</h3>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ f.desc }}</p>
        </div>
      </div>
    </section>

    <!-- Roadmap：在建/规划模块（彩色卡片 + 状态徽标 + 集成标签） -->
    <section id="roadmap" class="mx-auto max-w-6xl px-4 py-16">
      <h2 class="text-center text-3xl font-bold">{{ $t('page.landing.roadmapTitle') }}</h2>
      <p class="mt-2 text-center text-gray-500 dark:text-gray-400">{{ $t('page.landing.roadmapSubtitle') }}</p>
      <div class="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <div
          v-for="r in roadmap"
          :key="r.name"
          :style="{ '--c': r.color }"
          class="module-card rounded-xl border border-solid bg-white p-6 transition dark:bg-gray-900"
        >
          <div class="mb-3 flex items-center justify-between">
            <span class="module-icon inline-flex h-12 w-12 items-center justify-center rounded-xl text-2xl">{{ r.icon }}</span>
            <Tag :color="statusMap[r.status]?.color">{{ statusMap[r.status]?.text }}</Tag>
          </div>
          <h3 class="text-lg font-semibold">{{ r.name }}</h3>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ r.desc }}</p>
          <div class="mt-3 flex flex-wrap gap-1">
            <span v-for="tag in r.tags" :key="tag" class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-800 dark:text-gray-300">{{ tag }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Tech Stack：技术栈（彩色卡片，与 home 同款配色） -->
    <section id="tech" class="mx-auto max-w-6xl px-4 py-16">
      <h2 class="text-center text-3xl font-bold">{{ $t('page.landing.techTitle') }}</h2>
      <p class="mt-2 text-center text-gray-500 dark:text-gray-400">{{ $t('page.landing.techSubtitle') }}</p>
      <div class="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-8">
        <div
          v-for="t in techStack"
          :key="t.name"
          :style="{ '--c': t.color }"
          class="tech-card rounded-xl border border-solid bg-white p-4 text-center transition dark:bg-gray-900"
        >
          <span class="tech-icon mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full text-2xl">{{ t.logo }}</span>
          <div class="text-base font-semibold">{{ t.name }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t.desc }}</div>
        </div>
      </div>
    </section>

    <!-- About -->
    <section id="about" class="mx-auto max-w-4xl px-4 py-16 text-center">
      <h2 class="text-3xl font-bold">{{ $t('page.landing.aboutTitle') }}</h2>
      <p class="mt-4 text-gray-600 dark:text-gray-300">{{ $t('page.landing.aboutDesc') }}</p>
    </section>

    <!-- Footer -->
    <footer class="border-t border-gray-100 py-8 dark:border-gray-800">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-2 px-4 text-sm text-gray-500 md:flex-row">
        <div class="flex items-center gap-2">
          <img :src="preferences.logo.source" alt="logo" class="h-5 w-5 rounded" />
          <span>AI Arsenal</span>
        </div>
        <p>{{ copyrightText }}</p>
        <p>{{ $t('page.landing.poweredBy') }}</p>
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* 通过 --c 注入各模块主题色，用 color-mix 自动算出边框/底纹/阴影，
   亮暗主题自适应（透明度叠加），无需写两套。与 home 页模块卡同款配方。
   用 background-image（非 background 简写）以便与 bg-white/dark:bg-gray-900 叠加。 */
.module-card {
  --c: #3b82f6;
  border-color: color-mix(in srgb, var(--c) 30%, transparent);
  background-image: linear-gradient(
    135deg,
    color-mix(in srgb, var(--c) 9%, transparent),
    transparent 60%
  );
}

.module-card:hover {
  /* box-sizing: border-box（Tailwind 预设），加粗边框不撑大外框、无位移 */
  border-width: 2px;
  border-color: color-mix(in srgb, var(--c) 55%, transparent);
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c) 50%, transparent);
  transform: translateY(-3px);
}

.module-icon {
  background: color-mix(in srgb, var(--c) 16%, transparent);
}

.tech-card {
  --c: #6366f1;
  border-color: color-mix(in srgb, var(--c) 30%, transparent);
  background-image: linear-gradient(
    160deg,
    color-mix(in srgb, var(--c) 8%, transparent),
    transparent 60%
  );
}

.tech-card:hover {
  border-color: color-mix(in srgb, var(--c) 55%, transparent);
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c) 45%, transparent);
  transform: translateY(-3px);
}

.tech-icon {
  background: color-mix(in srgb, var(--c) 16%, transparent);
}
</style>
