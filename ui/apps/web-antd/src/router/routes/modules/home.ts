import type { RouteRecordRaw } from 'vue-router';

import { $t } from '#/locales';

/**
 * AI Arsenal 首页（gin 菜单无 home 项，故前端静态定义）
 * 登录后 defaultHomePath='/home' 跳转至此。Phase 4 丰富内容。
 */
const routes: RouteRecordRaw[] = [
  {
    meta: {
      icon: 'lucide:layout-dashboard',
      order: -1,
      title: $t('page.dashboard.title'),
    },
    name: 'Home',
    path: '/home',
    component: () => import('#/views/home/index.vue'),
  },
];

export default routes;
