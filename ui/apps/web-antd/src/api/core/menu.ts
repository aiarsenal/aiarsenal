import type { RouteRecordStringComponent } from '@vben/types';

import { preferences } from '@vben/preferences';

import { requestClient } from '#/api/request';

/**
 * gin 后端菜单契约：
 * - GET /api/v1/system/user/getUserMenus → {code:0, data:{menuList, permissions}}
 * - UserMenus = {id, pid, name, component, path, meta:{icon,title,enName,isLink,
 *   isHide,isKeepAlive,isAffix,isIframe}, children}
 * - 映射到 vben RouteRecordStringComponent（component 为字符串路径，
 *   由 access.ts 的 import.meta.glob 解析 views 下的 .vue 文件为组件）
 */

function mapMenu(m: any): RouteRecordStringComponent {
  const meta = m?.meta ?? {};
  const link: string = meta.isLink ?? '';
  // 同时保留中英文标题；切换语言时由 useLocaleSync 实时改写 meta.title（无需刷新/重拉菜单）
  const zhTitle: string = meta.title ?? '';
  const enTitle: string = meta.enName ?? zhTitle;
  const isEn = preferences.app.locale === 'en-US';
  const node: RouteRecordStringComponent = {
    path: m?.path ?? '',
    name: m?.name,
    component: m?.component ?? '',
    meta: {
      title: isEn ? enTitle : zhTitle,
      // 原始中英文标题，供语言切换时动态回写
      i18nTitles: { 'en-US': enTitle, 'zh-CN': zhTitle },
      icon: meta.icon,
      hideInMenu: !!meta.isHide,
      keepAlive: !!meta.isKeepAlive,
      affixTab: !!meta.isAffix,
      ...(link
        ? meta.isIframe
          ? { iframeSrc: link }
          : { link }
        : {}),
    },
    children: Array.isArray(m?.children) ? m.children.map(mapMenu) : undefined,
  };
  return node;
}

/**
 * 获取用户所有菜单
 */
export async function getAllMenusApi() {
  const res = await requestClient.get<{
    menuList: any[];
    permissions: string[];
  }>('/api/v1/system/user/getUserMenus');
  return (res?.menuList ?? []).map(mapMenu);
}
