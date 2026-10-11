import { initPreferences } from '@vben/preferences';
import { unmountGlobalLoading } from '@vben/utils';

import { overridesPreferences, preferencesExtension } from './preferences';

/**
 * 应用初始化完成之后再进行页面加载渲染
 */
async function initApplication() {
  // name用于指定项目唯一标识
  // 用于区分不同项目的偏好设置以及存储数据的key前缀以及其他一些需要隔离的数据
  const env = import.meta.env.PROD ? 'prod' : 'dev';
  // namespace 作为所有持久化 key 的前缀（preferences + pinia SecureLS）。
  // 不拼版本号：VITE_APP_VERSION 从未在任何 .env 定义 → 前缀会出现字面量 "undefined"（脏），
  // 且若版本号变化会让整套 key 换名、旧 key 永不清理 → localStorage 堆积。
  // 固定 NS-env，永久稳定，不再产生孤儿 key。
  const namespace = `${import.meta.env.VITE_APP_NAMESPACE}-${env}`;

  // app偏好设置初始化
  await initPreferences({
    extension: preferencesExtension,
    namespace,
    overrides: overridesPreferences,
  });

  // 启动应用并挂载
  // vue应用主要逻辑及视图
  const { bootstrap } = await import('./bootstrap');
  await bootstrap(namespace);

  // 移除并销毁loading
  unmountGlobalLoading();

  // 应用就绪：通知 index.html 的启动动画可以淡出（满足握手后由其自行 hide）
  // 动画未加载时 window.__aiHideSplash 不存在，可选链安全跳过
  (window as any).__appReady = true;
  (window as any).__aiHideSplash?.();
}

initApplication();
