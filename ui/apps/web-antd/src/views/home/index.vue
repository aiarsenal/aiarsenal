<script lang="ts" setup>
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { useUserStore } from '@vben/stores';

import { Card } from 'ant-design-vue';

import { $t } from '#/locales';

defineOptions({ name: 'Home' });

const router = useRouter();
const userStore = useUserStore();
const userName = computed(() => userStore.userInfo?.username || 'AI Arsenal');

// 模块状态（与 README 核心能力一致：以代码实现度为准）
// path：点击卡片跳转的模块落地页；可达性按当前用户动态路由实时判断
const modules = computed(() => [
  { name: $t('page.home.moduleSystemName'), status: 'online', desc: $t('page.home.moduleSystemDesc'), icon: '⚙️', color: '#3b82f6', path: '/system/user/list' },
  { name: $t('page.home.moduleSkillName'), status: 'online', desc: $t('page.home.moduleSkillDesc'), icon: '🧩', color: '#8b5cf6', path: '/skill/skill/list' },
  { name: $t('page.home.moduleLlmName'), status: 'online', desc: $t('page.home.moduleLlmDesc'), icon: '🧠', color: '#ec4899', path: '/llm/credential/list' },
  { name: $t('page.home.moduleMcpName'), status: 'online', desc: $t('page.home.moduleMcpDesc'), icon: '🔌', color: '#06b6d4', path: '/mcp/server/list' },
  { name: $t('page.home.moduleDocName'), status: 'online', desc: $t('page.home.moduleDocDesc'), icon: '📑', color: '#64748b', path: '/doc/document/list' },
  { name: $t('page.home.moduleKnowledgeName'), status: 'online', desc: $t('page.home.moduleKnowledgeDesc'), icon: '📚', color: '#f59e0b', path: '/kb/document/list' },
  { name: $t('page.home.moduleProjectName'), status: 'online', desc: $t('page.home.moduleProjectDesc'), icon: '📋', color: '#14b8a6', path: '/project/list' },
  { name: $t('page.home.moduleAgentName'), status: 'developing', desc: $t('page.home.moduleAgentDesc'), icon: '🤖', color: '#10b981', path: '/agent/instance/list' },
  { name: $t('page.home.moduleClientName'), status: 'developing', desc: $t('page.home.moduleClientDesc'), icon: '💻', color: '#6366f1', path: '/client/device' },
]);

const statusMap = computed<Record<string, { color: string; text: string }>>(() => ({
  online: { color: 'magenta', text: $t('page.home.statusOnline') },
  developing: { color: 'orange', text: $t('page.home.statusDeveloping') },
  planned: { color: 'blue', text: $t('page.home.statusPlanned') },
}));

const onlineCount = computed(() => modules.value.filter((m) => m.status === 'online').length);

// 按时段问候
const greeting = computed(() => {
  const h = new Date().getHours();
  if (h >= 5 && h < 12) return $t('page.home.greetingMorning', { name: userName.value });
  if (h < 18) return $t('page.home.greetingAfternoon', { name: userName.value });
  if (h < 23) return $t('page.home.greetingEvening', { name: userName.value });
  return $t('page.home.greetingLateNight', { name: userName.value });
});

const today = computed(() =>
  new Intl.DateTimeFormat(preferences.app.locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  }).format(new Date()),
);

// 后端菜单按权限下发动态路由：无权限的模块解析到 404 兜底路由（FallbackNotFound），降淡不可点
function isReachable(path: string) {
  const { matched } = router.resolve(path);
  return matched.length > 0 && matched[0]?.name !== 'FallbackNotFound';
}

function go(path: string) {
  if (isReachable(path)) router.push(path);
}

const techStack = computed(() => [
  { name: 'GLM-5.2', desc: $t('page.home.techGlmDesc'), logo: '🧠', color: '#6366f1' },
  { name: 'Gin', desc: $t('page.home.techGinDesc'), logo: '🐹', color: '#06b6d4' },
  { name: 'GORM', desc: $t('page.home.techGormDesc'), logo: '🗂️', color: '#10b981' },
  { name: 'PostgreSQL', desc: $t('page.home.techPgDesc'), logo: '🐘', color: '#3b82f6' },
  { name: 'Redis', desc: $t('page.home.techRedisDesc'), logo: '🟥', color: '#ef4444' },
  { name: 'Vue 3', desc: $t('page.home.techVueDesc'), logo: '⚡', color: '#42b983' },
  { name: 'Vben Admin', desc: $t('page.home.techVbenDesc'), logo: '🎨', color: '#8b5cf6' },
  { name: 'Tauri', desc: $t('page.home.techTauriDesc'), logo: '🖥️', color: '#f59e0b' },
]);
</script>

<template>
  <Page :auto-content-height="true">
    <div class="space-y-6">
      <!-- 欢迎卡片 -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <h1 class="m-0 text-3xl font-semibold">{{ greeting }}</h1>
            <p class="mt-3 mb-0 text-lg text-gray-500 dark:text-gray-400">
              {{ $t('page.home.subtitle') }}
            </p>
          </div>
          <div class="text-right text-sm text-gray-400">
            <div>{{ today }}</div>
            <div class="mt-1 text-base font-medium text-gray-600 dark:text-gray-300">
              {{ onlineCount }}/{{ modules.length }} {{ $t('page.home.modulesOnline') }}
            </div>
            <div class="mt-1">{{ $t('page.home.techBase') }}：Gin + Vben Admin</div>
          </div>
        </div>
      </Card>

      <!-- 模块矩阵（点击直达，无权限模块降淡） -->
      <Card :title="$t('page.home.modulesTitle')">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="m in modules"
            :key="m.name"
            :style="{ '--c': m.color }"
            :class="[
              'module-card rounded-xl border border-solid p-5 transition',
              isReachable(m.path) ? 'module-card--link cursor-pointer' : 'opacity-60',
            ]"
            @click="go(m.path)"
          >
            <div class="mb-3 flex items-center justify-between">
              <span class="module-icon inline-flex h-12 w-12 items-center justify-center rounded-xl text-2xl">
                {{ m.icon }}
              </span>
              <span :class="['aa-enum', `aa-enum--${statusMap[m.status]?.color}`]">
                {{ statusMap[m.status]?.text }}
              </span>
            </div>
            <div class="flex items-center gap-1 text-lg font-semibold">
              {{ m.name }}
              <span class="module-arrow text-sm opacity-0 transition">→</span>
            </div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ m.desc }}</div>
          </div>
        </div>
      </Card>

      <!-- 技术栈 -->
      <Card :title="$t('page.home.techStackTitle')">
        <div class="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-8">
          <div
            v-for="t in techStack"
            :key="t.name"
            :style="{ '--c': t.color }"
            class="tech-card rounded-xl border border-solid p-4 text-center transition"
          >
            <span class="tech-icon mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full text-2xl">
              {{ t.logo }}
            </span>
            <div class="text-base font-semibold">{{ t.name }}</div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t.desc }}</div>
          </div>
        </div>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
/* 通过 --c 注入各模块主题色，用 color-mix 自动算出边框/底纹/阴影，
   亮暗主题自适应（透明度叠加），无需写两套。 */
.module-card {
  --c: #3b82f6;
  border-color: color-mix(in srgb, var(--c) 30%, transparent);
  background:
    linear-gradient(
      135deg,
      color-mix(in srgb, var(--c) 9%, transparent),
      transparent 60%
    );
}

.module-card--link:hover {
  /* box-sizing: border-box（Tailwind 预设），加粗边框不撑大外框、无位移 */
  border-width: 2px;
  border-color: color-mix(in srgb, var(--c) 55%, transparent);
  box-shadow: 0 10px 30px -12px color-mix(in srgb, var(--c) 50%, transparent);
  transform: translateY(-3px);
}

.module-card--link:hover .module-arrow {
  opacity: 1;
}

.module-icon {
  background: color-mix(in srgb, var(--c) 16%, transparent);
}

.tech-card {
  --c: #6366f1;
  border-color: color-mix(in srgb, var(--c) 30%, transparent);
  background:
    linear-gradient(
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
