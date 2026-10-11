const BasicLayout = () => import('./basic.vue');
const AuthPageLayout = () => import('./auth.vue');
const ParentLayout = () => import('./parent.vue');

const IFrameView = () => import('@vben/layouts').then((m) => m.IFrameView);

export { AuthPageLayout, BasicLayout, IFrameView, ParentLayout };
