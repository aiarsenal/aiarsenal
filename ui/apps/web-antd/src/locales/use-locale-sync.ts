import type { SupportedLanguagesType } from '@vben/locales';

import { watch } from 'vue';

import { preferences } from '@vben/preferences';
import { useAccessStore, useTabbarStore } from '@vben/stores';

/**
 * 遍历菜单/路由树，按当前 locale 回写显示标题。
 *
 * 后端菜单在 mapMenu 时已把中英文标题存入 meta.i18nTitles。
 * 注意：侧边栏实际显示的是 menu.name（由 generateMenus 从 meta.title 拷贝而来），
 * 而非 meta.title 本身；wrapperMenus 再用 $t(item.name) 渲染。
 * 因此必须同时改写 name 与 meta.title，才能让侧边栏/面包屑/页签/标签页标题
 * 随语言切换实时变更（无需重新拉菜单、无需刷新）。
 */
function rewriteTitle(node: any, locale: SupportedLanguagesType) {
  // 菜单节点（generateMenus 产物）把 i18nTitles 挂在顶层；
  // 路由节点则挂在 meta 上。
  const titles = node?.i18nTitles ?? node?.meta?.i18nTitles;
  if (titles) {
    const next = titles[locale] ?? titles['zh-CN'] ?? node?.meta?.title ?? node?.name;
    // 侧边栏/顶部菜单显示用 name
    node.name = next;
    // 面包屑/页签/浏览器标题用 meta.title（菜单节点无 meta 则跳过）
    if (node?.meta) node.meta.title = next;
  }
  const children = node?.children;
  if (Array.isArray(children)) {
    children.forEach((child: any) => rewriteTitle(child, locale));
  }
}

/**
 * 收集路由树中所有节点的 i18nTitles，建立 name / path → i18nTitles 查找表。
 * 页签（tabbarStore.tabs）的 meta 在 addTab 时被 cloneTab 浅拷贝，与路由 meta
 * 不再共享引用，因此改写路由 meta.title 传不到已存在的页签；需要凭 name/path
 * 反查 i18nTitles 后直接改写页签自身的 meta.title。
 */
function buildI18nTitleMap(routes: any[]): Map<string, any> {
  const map = new Map<string, any>();
  const visit = (node: any) => {
    const titles = node?.meta?.i18nTitles ?? node?.i18nTitles;
    if (titles) {
      if (node?.name) map.set(String(node.name), titles);
      if (node?.path) map.set(String(node.path), titles);
    }
    if (Array.isArray(node?.children)) node.children.forEach(visit);
  };
  routes.forEach(visit);
  return map;
}

/**
 * 监听偏好设置中的 locale 变化，实时切换菜单与路由标题语言。
 * 需在根组件（app.vue）中调用一次，随应用生命周期常驻。
 */
export function useLocaleSync() {
  const accessStore = useAccessStore();
  const tabbarStore = useTabbarStore();

  watch(
    () => preferences.app.locale,
    (locale) => {
      const lang = locale as SupportedLanguagesType;
      // 改写菜单树（侧边栏 / 顶部菜单显示文本）
      accessStore.accessMenus.forEach((m) => rewriteTitle(m, lang));
      // 改写路由树（面包屑 / 浏览器标题）
      accessStore.accessRoutes.forEach((r) => rewriteTitle(r, lang));
      // 关键：侧边栏菜单来自 computed(() => accessStore.accessMenus)，
      // 它只追踪 accessMenus 这个引用，原地改 node.name 不会触发重算。
      // 这里用新数组引用「撞」一下，让整条 computed 链（menus→sidebarMenus→
      // headerMenus→wrapperMenus）重新执行，从而读取已改写的 name。
      accessStore.setAccessMenus([...accessStore.accessMenus] as any);
      accessStore.setAccessRoutes([...accessStore.accessRoutes] as any);

      // 改写已存在的页签标题（页签 meta 是独立拷贝，不随路由变化）
      const titleMap = buildI18nTitleMap(accessStore.accessRoutes as any[]);
      tabbarStore.getTabs.forEach((tab: any) => {
        const titles =
          tab?.meta?.i18nTitles ??
          titleMap.get(String(tab?.name ?? '')) ??
          titleMap.get(String(tab?.path ?? ''));
        if (!titles) return;
        const next = titles[lang] ?? titles['zh-CN'] ?? tab?.meta?.title;
        if (next && tab?.meta) {
          tab.meta.title = next;
          tab.meta.i18nTitles = titles;
        }
      });
    },
  );
}
