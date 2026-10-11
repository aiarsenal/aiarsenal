<script lang="ts" setup>
import type {
  WorkbenchProjectItem,
  WorkbenchQuickNavItem,
  WorkbenchTodoItem,
  WorkbenchTrendItem,
} from '@vben/common-ui';

import { computed } from 'vue';
import { useRouter } from 'vue-router';

import {
  AnalysisChartCard,
  WorkbenchHeader,
  WorkbenchProject,
  WorkbenchQuickNav,
  WorkbenchTodo,
  WorkbenchTrends,
} from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { useUserStore } from '@vben/stores';
import { openWindow } from '@vben/utils';

import { $t } from '#/locales';

import AnalyticsVisitsSource from '../analytics/analytics-visits-source.vue';

const userStore = useUserStore();

// 这是一个示例数据，实际项目中需要根据实际情况进行调整
// url 也可以是内部路由，在 navTo 方法中识别处理，进行内部跳转
// 例如：url: /dashboard/workspace
const projectItems = computed<WorkbenchProjectItem[]>(() => [
  {
    color: '',
    content: $t('dashboard.workspace_project1Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project1Group'),
    icon: 'carbon:logo-github',
    title: 'Github',
    url: 'https://github.com',
  },
  {
    color: '#3fb27f',
    content: $t('dashboard.workspace_project2Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project2Group'),
    icon: 'ion:logo-vue',
    title: 'Vue',
    url: 'https://vuejs.org',
  },
  {
    color: '#e18525',
    content: $t('dashboard.workspace_project3Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project3Group'),
    icon: 'ion:logo-html5',
    title: 'Html5',
    url: 'https://developer.mozilla.org/zh-CN/docs/Web/HTML',
  },
  {
    color: '#bf0c2c',
    content: $t('dashboard.workspace_project4Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project4Group'),
    icon: 'ion:logo-angular',
    title: 'Angular',
    url: 'https://angular.io',
  },
  {
    color: '#00d8ff',
    content: $t('dashboard.workspace_project5Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project5Group'),
    icon: 'bx:bxl-react',
    title: 'React',
    url: 'https://reactjs.org',
  },
  {
    color: '#EBD94E',
    content: $t('dashboard.workspace_project6Content'),
    date: '2021-04-01',
    group: $t('dashboard.workspace_project6Group'),
    icon: 'ion:logo-javascript',
    title: 'Js',
    url: 'https://developer.mozilla.org/zh-CN/docs/Web/JavaScript',
  },
]);

// 同样，这里的 url 也可以使用以 http 开头的外部链接
const quickNavItems = computed<WorkbenchQuickNavItem[]>(() => [
  {
    color: '#1fdaca',
    icon: 'ion:home-outline',
    title: $t('dashboard.workspace_navHome'),
    url: '/',
  },
  {
    color: '#bf0c2c',
    icon: 'ion:grid-outline',
    title: $t('dashboard.workspace_navDashboard'),
    url: '/dashboard',
  },
  {
    color: '#e18525',
    icon: 'ion:layers-outline',
    title: $t('dashboard.workspace_navComponents'),
    url: '/demos/features/icons',
  },
  {
    color: '#3fb27f',
    icon: 'ion:settings-outline',
    title: $t('dashboard.workspace_navSystem'),
    url: '/demos/features/login-expired', // 这里的 URL 是示例，实际项目中需要根据实际情况进行调整
  },
  {
    color: '#4daf1bc9',
    icon: 'ion:key-outline',
    title: $t('dashboard.workspace_navAccess'),
    url: '/demos/access/page-control',
  },
  {
    color: '#00d8ff',
    icon: 'ion:bar-chart-outline',
    title: $t('dashboard.workspace_navCharts'),
    url: '/analytics',
  },
]);

const todoItems = computed<WorkbenchTodoItem[]>(() => [
  {
    completed: false,
    content: $t('dashboard.workspace_todo1Content'),
    date: '2024-07-30 11:00:00',
    title: $t('dashboard.workspace_todo1Title'),
  },
  {
    completed: true,
    content: $t('dashboard.workspace_todo2Content'),
    date: '2024-07-30 11:00:00',
    title: $t('dashboard.workspace_todo2Title'),
  },
  {
    completed: false,
    content: $t('dashboard.workspace_todo3Content'),
    date: '2024-07-30 11:00:00',
    title: $t('dashboard.workspace_todo3Title'),
  },
  {
    completed: false,
    content: $t('dashboard.workspace_todo4Content'),
    date: '2024-07-30 11:00:00',
    title: $t('dashboard.workspace_todo4Title'),
  },
  {
    completed: false,
    content: $t('dashboard.workspace_todo5Content'),
    date: '2024-07-30 11:00:00',
    title: $t('dashboard.workspace_todo5Title'),
  },
]);

const trendItems = computed<WorkbenchTrendItem[]>(() => [
  {
    avatar: 'svg:avatar-1',
    content: $t('dashboard.workspace_trend1Content'),
    date: $t('dashboard.workspace_trend1Date'),
    title: $t('dashboard.workspace_trend1Title'),
  },
  {
    avatar: 'svg:avatar-2',
    content: $t('dashboard.workspace_trend2Content'),
    date: $t('dashboard.workspace_trend2Date'),
    title: $t('dashboard.workspace_trend2Title'),
  },
  {
    avatar: 'svg:avatar-3',
    content: $t('dashboard.workspace_trend3Content'),
    date: $t('dashboard.workspace_trend3Date'),
    title: $t('dashboard.workspace_trend3Title'),
  },
  {
    avatar: 'svg:avatar-4',
    content: $t('dashboard.workspace_trend4Content'),
    date: $t('dashboard.workspace_trend4Date'),
    title: $t('dashboard.workspace_trend4Title'),
  },
  {
    avatar: 'svg:avatar-1',
    content: $t('dashboard.workspace_trend5Content'),
    date: $t('dashboard.workspace_trend5Date'),
    title: $t('dashboard.workspace_trend5Title'),
  },
  {
    avatar: 'svg:avatar-2',
    content: $t('dashboard.workspace_trend6Content'),
    date: $t('dashboard.workspace_trend6Date'),
    title: $t('dashboard.workspace_trend6Title'),
  },
  {
    avatar: 'svg:avatar-3',
    content: $t('dashboard.workspace_trend7Content'),
    date: $t('dashboard.workspace_trend7Date'),
    title: $t('dashboard.workspace_trend7Title'),
  },
  {
    avatar: 'svg:avatar-4',
    content: $t('dashboard.workspace_trend8Content'),
    date: $t('dashboard.workspace_trend8Date'),
    title: $t('dashboard.workspace_trend8Title'),
  },
  {
    avatar: 'svg:avatar-4',
    content: $t('dashboard.workspace_trend9Content'),
    date: $t('dashboard.workspace_trend9Date'),
    title: $t('dashboard.workspace_trend9Title'),
  },
]);

const router = useRouter();

// 这是一个示例方法，实际项目中需要根据实际情况进行调整
// This is a sample method, adjust according to the actual project requirements
function navTo(nav: WorkbenchProjectItem | WorkbenchQuickNavItem) {
  if (nav.url?.startsWith('http')) {
    openWindow(nav.url);
    return;
  }
  if (nav.url?.startsWith('/')) {
    router.push(nav.url).catch((error) => {
      console.error('Navigation failed:', error);
    });
  } else {
    console.warn(`Unknown URL for navigation item: ${nav.title} -> ${nav.url}`);
  }
}
</script>

<template>
  <div class="p-5">
    <WorkbenchHeader
      :avatar="userStore.userInfo?.avatar || preferences.app.defaultAvatar"
    >
      <template #title>
        {{ $t('dashboard.workspace_greeting', { name: userStore.userInfo?.realName }) }}
      </template>
      <template #description> {{ $t('dashboard.workspace_weather') }} </template>
    </WorkbenchHeader>

    <div class="flex flex-col lg:flex-row">
      <div class="mr-4 w-full lg:w-3/5">
        <WorkbenchProject :items="projectItems" :title="$t('dashboard.workspace_projectTitle')" @click="navTo" />
        <WorkbenchTrends :items="trendItems" class="mt-5" :title="$t('dashboard.workspace_trendsTitle')" />
      </div>
      <div class="w-full lg:w-2/5">
        <WorkbenchQuickNav
          :items="quickNavItems"
          class="lg:mt-0"
          :title="$t('dashboard.workspace_quickNavTitle')"
          @click="navTo"
        />
        <WorkbenchTodo :items="todoItems" class="mt-5" :title="$t('dashboard.workspace_todoTitle')" />
        <AnalysisChartCard class="mt-5" :title="$t('dashboard.workspace_visitSourceTitle')">
          <AnalyticsVisitsSource />
        </AnalysisChartCard>
      </div>
    </div>
  </div>
</template>
