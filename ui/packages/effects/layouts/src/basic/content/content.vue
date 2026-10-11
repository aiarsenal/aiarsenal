<script lang="ts" setup>
import type { RouteLocationNormalizedLoadedGeneric } from 'vue-router';

import { unref } from 'vue';
import { RouterView } from 'vue-router';

import { usePreferences } from '@vben/preferences';
import { getTabKey, storeToRefs, useTabbarStore } from '@vben/stores';

import { transformComponent } from '../../hooks';
import { IFrameRouterView } from '../../iframe';
import { RouteCachedPage, RouteCachedView } from '../../route-cached';

defineOptions({ name: 'LayoutContent' });

const tabbarStore = useTabbarStore();
const { keepAlive } = usePreferences();

const { getCachedTabs, getExcludeCachedTabs, renderRouteView } =
  storeToRefs(tabbarStore);

/**
 * 是否显示component
 * @param route
 */
const showComponent = (route: RouteLocationNormalizedLoadedGeneric) => {
  return !route.meta.domCached && unref(renderRouteView);
};
</script>

<template>
  <div class="relative h-full">
    <IFrameRouterView />
    <RouteCachedView />
    <RouterView v-slot="{ Component, route }">
      <RouteCachedPage
        :component="Component"
        :route="route"
        v-if="route.meta.domCached"
      />
      <!-- 2026-08-25 移除页面切换 Transition（fade-slide + out-in）：
           out-in 的延迟插入与 KeepAlive 缓存、刷新流程（renderRouteView 的 v-if 抖动）叠加时
           会出现 activate/afterLeave/insertBefore 系列 DOM 错位，此后所有页面打不开（仅重载可恢复）。
           页面内容实际渲染在目录壳（parent.vue 裸 router-view）内的深层，动画只作用于目录壳，
           删除无功能损失。 -->
      <KeepAlive
        v-if="keepAlive"
        :exclude="getExcludeCachedTabs"
        :include="getCachedTabs"
      >
        <component
          :is="transformComponent(Component, route)"
          v-if="showComponent(route)"
          v-show="!route.meta.iframeSrc"
          :key="getTabKey(route)"
        />
      </KeepAlive>
      <component
        :is="Component"
        v-else-if="showComponent(route)"
        :key="getTabKey(route)"
      />
    </RouterView>
  </div>
</template>
