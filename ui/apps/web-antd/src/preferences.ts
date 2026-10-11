import {
  appCopyrightPreferences,
  defineOverridesPreferences,
  definePreferencesExtension,
} from '@vben/preferences';

interface WebAntdPreferencesExtension {
  defaultTableSize: number;
  enableFormFullscreen: boolean;
  reportTitle: string;
  tenantMode: 'multi' | 'single';
}

/**
 * @description 项目配置文件
 * 只需要覆盖项目中的一部分配置，不需要的配置不用覆盖，会自动使用默认配置
 * !!! 更改配置后请清空缓存，否则可能不生效
 */
export const overridesPreferences = defineOverridesPreferences({
  // overrides
  app: {
    name: import.meta.env.VITE_APP_TITLE,
    // 隐藏偏好设置（主题/颜色/布局）齿轮按钮，默认品牌绿 #42b983 与 logo 一致
    enablePreferences: false,
    // mixed 模式：合并前端静态路由（modules/home.ts 的 /home）+ gin 后端下发的菜单路由
    // （纯 backend 模式不注册前端 accessRoutes，致 /home 404）
    accessMode: 'mixed',
    // gin 菜单无 home 项，首页走前端静态 /home 路由（modules/home.ts）
    defaultHomePath: '/home',
    // gin 登录一次性返回 userInfo，无独立 refresh 端点
    enableRefreshToken: false,
    // 登录过期走整页跳转登录（非弹窗）
    loginExpiredMode: 'page',
  },
  widget: {
    // 明暗切换开启（独立按钮，见 basic.vue）
    themeToggle: true,
    // 保留通知、语言切换、全屏等
    notification: true,
    languageToggle: true,
    fullscreen: true,
    globalSearch: true,
    refresh: true,
    sidebarToggle: true,
    lockScreen: false,
    timezone: false,
  },
  logo: {
    // 侧边栏顶部显示 logo + 站点标题（basic/layout.vue 的 sidebar-logo-visible + VbenLogo 都看 enable）
    enable: true,
    // 方形徽标：透明圆角图标（logo.png 由 cover.png 裁成正方形 RGBA，四角透明，亮/暗底均浮起干净）
    // 全站单一来源——landing/桌面登录/登录页/后台都走此 source，杜绝多文件不一致
    // icon 模式：徽标图 + 单独的「AI Arsenal」文字 span（文字走主题色，亮暗都可见）
    source: '/logo.png?v=9',
    sourceDark: '/logo.png?v=9',
    logoMode: 'icon',
    showText: true,
  },
  sidebar: {
    // 悬停展开模式：空闲态宽 0 彻底消失（无图标栏），鼠标滑过屏幕最左 ~10px 隐形触发条即展开
    // （以 fixed 覆盖在内容上方，不挤压内容），移开即收起。展开后点侧栏右下角图钉按钮可锁定常驻。
    // 注：vben 的 expandOnHover 命名反直觉——false 才是「瞬态悬停模式」；
    // true 表示已固定（handleMouseleave 主动 early-return，悬停不收起）。别改回 true。
    collapsed: true,
    expandOnHover: false,
    collapseWidth: 0,
  },
  tabbar: {
    // 标签条变高，与放大后的页面风格配套
    height: 44,
  },
  theme: {
    // AI Arsenal 品牌色 #42b983
    colorPrimary: '#42b983',
    // 圆角 0.5rem=8px：容器圆角(rounded-lg=8)，驱动 vben 自研组件 --radius。
    // fontSize 保持默认 16 不动（仅改形状/圆角/动效，不收紧字号）。
    radius: '0.5',
  },
  copyright: {
    ...appCopyrightPreferences,
    // 覆盖 vben 默认版权为公司信息
    companyName: 'AI Arsenal',
    companySiteLink: 'https://aiarsenal.cc',
  },
});

export const preferencesExtension =
  definePreferencesExtension<WebAntdPreferencesExtension>({
    tabLabel: 'preferences.antd.tabLabel',
    title: 'preferences.antd.title',
    fields: [
      {
        component: 'switch',
        defaultValue: true,
        key: 'enableFormFullscreen',
        label: 'preferences.antd.fields.enableFormFullscreen.label',
        tip: 'preferences.antd.fields.enableFormFullscreen.tip',
      },
      {
        component: 'select',
        defaultValue: 'single',
        key: 'tenantMode',
        label: 'preferences.antd.fields.tenantMode.label',
        options: [
          {
            label: 'preferences.antd.fields.tenantMode.options.single.label',
            value: 'single',
          },
          {
            label: 'preferences.antd.fields.tenantMode.options.multi.label',
            value: 'multi',
          },
        ],
      },
      {
        component: 'number',
        componentProps: {
          max: 200,
          min: 10,
          step: 10,
        },
        defaultValue: 20,
        key: 'defaultTableSize',
        label: 'preferences.antd.fields.defaultTableSize.label',
      },
      {
        component: 'input',
        defaultValue: '',
        key: 'reportTitle',
        label: 'preferences.antd.fields.reportTitle.label',
        placeholder: 'preferences.antd.fields.reportTitle.placeholder',
      },
    ],
  });
