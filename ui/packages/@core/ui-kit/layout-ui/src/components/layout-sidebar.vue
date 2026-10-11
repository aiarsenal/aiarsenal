<script setup lang="ts">
import type { CSSProperties } from 'vue';

import {
  computed,
  onUnmounted,
  ref,
  shallowRef,
  useSlots,
  watch,
  watchEffect,
} from 'vue';

import { VbenScrollbar } from '@vben-core/shadcn-ui';

import { useScrollLock } from '@vueuse/core';

import { useSidebarDrag } from '../hooks/use-sidebar-drag';
import { SidebarCollapseButton, SidebarFixedButton } from './widgets';

interface Props {
  /**
   * 折叠区域高度
   * @default 42
   */
  collapseHeight?: number;
  /**
   * 折叠宽度
   * @default 48
   */
  collapseWidth?: number;
  /**
   * 隐藏的dom是否可见
   * @default true
   */
  domVisible?: boolean;
  /**
   * 扩展区域extra-title的高度
   */
  extraTitleHeight?: number;
  /**
   * 扩展区域宽度
   */
  extraWidth: number;
  /**
   * 固定扩展区域
   * @default false
   */
  fixedExtra?: boolean;
  /**
   * 头部高度
   */
  headerHeight: number;
  /**
   * 是否侧边混合模式
   * @default false
   */
  isSidebarMixed?: boolean;
  /**
   * 是否移动端
   * 移动端为抽屉模式，可见性由 collapse 驱动，须跳过桌面「悬停浮层」逻辑
   * （该分支用 expandOnHovering 控制可见性，手机无 hover 永远 false → 侧栏被 translateX 藏在视口外）
   * @default false
   */
  isMobile?: boolean;
  /**
   * 顶部margin
   * @default 60
   */
  marginTop?: number;
  /**
   * 混合菜单宽度
   * @default 80
   */
  mixedWidth?: number;
  /**
   * 顶部padding
   * @default 60
   */
  paddingTop?: number;
  /**
   * 是否显示
   * @default true
   */
  show?: boolean;
  /**
   * 显示折叠按钮
   * @default true
   */
  showCollapseButton?: boolean;
  /**
   * 显示固定按钮
   * @default true
   */
  showFixedButton?: boolean;
  /**
   * 主题
   */
  theme: string;
  /**
   * 子主题
   */
  themeSub: string;
  /**
   * 宽度
   */
  width: number;
  /**
   * zIndex
   * @default 0
   */
  zIndex?: number;
}

const props = withDefaults(defineProps<Props>(), {
  collapseHeight: 42,
  collapseWidth: 48,
  domVisible: true,
  extraTitleHeight: undefined,
  fixedExtra: false,
  isSidebarMixed: false,
  isMobile: false,
  marginTop: 0,
  mixedWidth: 70,
  paddingTop: 0,
  show: true,
  showCollapseButton: true,
  showFixedButton: true,
  zIndex: 0,
});

const emit = defineEmits<{ leave: []; 'update:width': [value: number] }>();
const draggable = defineModel<boolean>('draggable');
const collapse = defineModel<boolean>('collapse');
const extraCollapse = defineModel<boolean>('extraCollapse');
const expandOnHovering = defineModel<boolean>('expandOnHovering');
const expandOnHover = defineModel<boolean>('expandOnHover');
const extraVisible = defineModel<boolean>('extraVisible');

const isLocked = useScrollLock(document.body);
const slots = useSlots();

const asideRef = shallowRef<HTMLElement | null>(null);
const dragBarRef = shallowRef<HTMLElement | null>(null);

const hiddenSideStyle = computed((): CSSProperties => calcMenuWidthStyle(true));

// 悬停滑出模式记忆：收起时侧栏不再收缩宽度，而是整体 translateX 平移出视口，
// 避免宽度从 0 过渡到目标值期间，菜单文字在窄列重排造成的「上下跳动」。
// expandedWidth 始终记住最近一次展开宽度，悬停模式下作为侧栏与内部内容的固定宽度。
const expandedWidth = ref(props.width);
watchEffect(() => {
  if (props.width > 0) {
    expandedWidth.value = props.width;
  }
});

const style = computed((): CSSProperties => {
  const { isMobile, isSidebarMixed, marginTop, paddingTop, zIndex } = props;

  const baseStyle: CSSProperties = {
    '--scroll-shadow': 'var(--sidebar)',
    height: `calc(100% - ${marginTop}px)`,
    marginTop: `${marginTop}px`,
    paddingTop: `${paddingTop}px`,
    zIndex,
    ...(isSidebarMixed && extraVisible.value ? { transition: 'none' } : {}),
  };

  // 瞬态悬停模式（移动端除外 + 未锁定常驻 + 非 mixed）：侧栏宽度恒为展开宽度，
  // 收起时整体 translateX(-100%) 平移到视口左侧外，展开时 translateX(0) 滑回。
  // 宽度恒定 → 内部菜单/Logo/按钮布局永不重排 → 纯水平滑出、无垂直跳动。
  // 最左隐形触发条在侧栏移走后露出承接 hover，侧栏滑回时将其覆盖。
  // 移动端不进入此分支：手机无 hover，expandOnHovering 恒为 false 会让侧栏永远 translateX(-100%) 藏在视口外，
  // 抽屉可见性改由 collapse→width 驱动（下方 calcMenuWidthStyle 标准路径）。
  if (!isMobile && !expandOnHover.value && !isSidebarMixed) {
    const w = expandedWidth.value;
    return {
      ...baseStyle,
      flex: `0 0 ${w}px`,
      maxWidth: `${w}px`,
      minWidth: `${w}px`,
      // 显式只过渡 transform，覆盖 class 上的 transition-all duration-300：
      // 显隐完全由 expandOnHovering 驱动（与 collapse 解耦）：滑入 0.3s、滑出 0.6s（刻意放慢一倍）。
      // 收缩时不折叠菜单，文字随侧栏整体平移滑出（见 handleMouseleave），不再只剩图标。
      // width 不参与过渡（恒定），消除首次展开宽度撑开的二次动画，保证纯水平滑出/滑入。
      transform: !expandOnHovering.value ? 'translateX(-100%)' : 'translateX(0)',
      transition: `transform ${!expandOnHovering.value ? 0.6 : 0.3}s ease-in-out`,
      width: `${w}px`,
    };
  }

  return { ...baseStyle, ...calcMenuWidthStyle(false) };
});

const extraStyle = computed((): CSSProperties => {
  const { extraWidth, show, width, zIndex } = props;

  return {
    left: `${width}px`,
    width: extraVisible.value && show ? `${extraWidth}px` : 0,
    zIndex,
  };
});

const extraTitleStyle = computed((): CSSProperties => {
  const { extraTitleHeight, headerHeight } = props;

  return {
    height: `${extraTitleHeight ?? headerHeight - 1}px`,
  };
});

const contentWidthStyle = computed((): CSSProperties => {
  const { fixedExtra, isSidebarMixed, mixedWidth } = props;
  if (isSidebarMixed && fixedExtra) {
    return { width: `${mixedWidth}px` };
  }
  return {};
});

const contentStyle = computed((): CSSProperties => {
  const { collapseHeight, headerHeight } = props;

  return {
    height: `calc(100% - ${headerHeight + collapseHeight}px)`,
    paddingTop: '8px',
    ...contentWidthStyle.value,
  };
});

const headerStyle = computed((): CSSProperties => {
  const { headerHeight, isSidebarMixed } = props;

  return {
    ...(isSidebarMixed ? { display: 'flex', justifyContent: 'center' } : {}),
    height: `${headerHeight - 1}px`,
    ...contentWidthStyle.value,
  };
});

const extraContentStyle = computed((): CSSProperties => {
  const { collapseHeight, extraTitleHeight, headerHeight } = props;
  const titleHeight = extraTitleHeight ?? headerHeight;
  return {
    height: `calc(100% - ${titleHeight + collapseHeight}px)`,
  };
});

const collapseStyle = computed((): CSSProperties => {
  return {
    height: `${props.collapseHeight}px`,
  };
});

// 悬停浮层模式下，收缩按钮（v-model 绑 collapse）点击置 collapse=true 时拦截：
// 不折叠菜单，改为触发侧栏整体滑出（expandOnHovering=false）并立即恢复 collapse=false，
// 让菜单文字随侧栏平移滑出、避免只剩图标。同一帧内 true→false 不产生闪烁。
// 锁定常驻模式（expandOnHover=true）不拦截，保留原折叠语义。
// 移动端不拦截：抽屉由 collapse 直接控制，否则点遮罩关闭(true)会被这里立即改回 false → 抽屉关不掉。
watch(collapse, (val) => {
  if (val && !props.isMobile && !expandOnHover.value && !props.isSidebarMixed) {
    expandOnHovering.value = false;
    collapse.value = false;
  }
});

watchEffect(() => {
  extraVisible.value = props.fixedExtra ? true : extraVisible.value;
});

function calcMenuWidthStyle(isHiddenDom: boolean): CSSProperties {
  const {
    collapseWidth,
    extraWidth,
    mixedWidth,
    fixedExtra,
    isSidebarMixed,
    show,
    width,
  } = props;

  let widthValue =
    width === 0
      ? '0px'
      : `${width + (isSidebarMixed && fixedExtra && extraVisible.value ? extraWidth : 0)}px`;

  // 占位 dom（hiddenSideStyle，isHiddenDom=true）：悬停浮层模式下侧栏是 fixed 覆盖层，
  // 占位恒为 collapseWidth(0)，不挤压内容——展开/收缩均如此。否则收缩时 collapse 不再置 true，
  // 会让 width 算成 sidebarWidth、占位撑开把正文挤窄，引发垂直重排（即「上下移动」）。
  if (isHiddenDom && !expandOnHover.value) {
    widthValue = isSidebarMixed ? `${mixedWidth}px` : `${collapseWidth}px`;
  }
  return {
    ...(widthValue === '0px' ? { overflow: 'hidden' } : {}),
    // 用 flex:none 让尺寸由 width 决定（而非 flex-basis），使占位 dom 与 aside 都走 width 过渡，
    // 避免收起时 flex-grow 重新分配与 width 线性过渡的逐帧偏差（菜单看起来慢半拍）。
    flex: 'none',
    marginLeft: show ? 0 : `-${widthValue}`,
    maxWidth: widthValue,
    minWidth: widthValue,
    width: widthValue,
  };
}

function handleMouseenter(e: MouseEvent) {
  if (e?.offsetX < 10) {
    return;
  }

  // 未开启和未折叠状态不生效
  if (expandOnHover.value) {
    return;
  }
  if (!expandOnHovering.value) {
    collapse.value = false;
  }
  if (props.isSidebarMixed) {
    isLocked.value = true;
  }
  expandOnHovering.value = true;
}

// 左边隐形触发条专用展开入口：跳过 offsetX<10 守卫（条本身只占最左 ~10px，
// 是折叠态宽 0 时唯一的悬停入口）。逻辑与 handleMouseenter 一致，仅去掉了那条守卫。
function handleHoverTrigger() {
  if (expandOnHover.value) {
    return;
  }
  if (!expandOnHovering.value) {
    collapse.value = false;
  }
  expandOnHovering.value = true;
}

function handleMouseleave() {
  emit('leave');
  if (props.isSidebarMixed) {
    isLocked.value = false;
  }
  if (expandOnHover.value) {
    return;
  }

  expandOnHovering.value = false;
  // 非 mixed 悬停模式不折叠菜单（不置 collapse=true）：让菜单文字随侧栏整体平移滑出，
  // 避免收缩动画期间文字提前消失、只剩图标；内容不被挤压由 vben-layout mainStyle 恒定保证。
  if (props.isSidebarMixed) {
    collapse.value = true;
  }
  extraVisible.value = false;
}

const { startDrag, endDrag } = useSidebarDrag();

const handleDragSidebar = (e: MouseEvent) => {
  const { isSidebarMixed, collapseWidth, width } = props;
  const minLimit = isSidebarMixed ? width + collapseWidth : collapseWidth;
  const maxLimit = isSidebarMixed ? width + 320 : 320;

  startDrag(
    e,
    {
      min: minLimit,
      max: maxLimit,
    },
    {
      target: asideRef.value,
      dragBar: dragBarRef.value,
    },
    (newWidth) => {
      if (isSidebarMixed) {
        emit('update:width', newWidth - width);
        extraCollapse.value = collapse.value =
          newWidth - width <= collapseWidth;
      } else {
        emit('update:width', newWidth);
        collapse.value = extraCollapse.value = newWidth <= collapseWidth;
      }
    },
  );
};

onUnmounted(() => {
  endDrag();
});
</script>

<template>
  <div
    v-if="domVisible"
    :class="theme"
    :style="hiddenSideStyle"
    class="h-full transition-all duration-300"
  ></div>
  <!-- 左边可见细条（悬停入口）：折叠态宽 0 时唯一的悬停入口，同时作为「这里有侧边栏」的视觉提示
       （仅桌面 + 悬停模式 + 非 mixed）。放在 aside 之前，同 zIndex 时 aside 后渲染盖住最左，
       展开后此条被覆盖、不抢事件。bg-sidebar 让它读起来像收起的侧栏边缘。 -->
  <div
    v-if="domVisible && !isSidebarMixed && !expandOnHover"
    class="fixed left-0 top-0 h-full w-2 cursor-pointer border-r border-border bg-sidebar transition-all duration-150 hover:w-3 hover:bg-primary/15"
    :style="{ zIndex }"
    @mouseenter="handleHoverTrigger"
  ></div>
  <aside
    ref="asideRef"
    :style="style"
    class="fixed left-0 top-0 h-full overflow-hidden transition-all duration-300"
    :class="theme"
    @mouseenter="handleMouseenter"
    @mouseleave="handleMouseleave"
  >
    <div
      class="h-full"
      :class="[
        {
          'bg-sidebar-deep': isSidebarMixed,
          'border-r border-border bg-sidebar': !isSidebarMixed,
        },
      ]"
      :style="{
        width: `${isSidebarMixed ? width : expandedWidth}px`,
      }"
    >
      <SidebarFixedButton
        v-if="!isSidebarMixed && showFixedButton"
        v-model:expand-on-hover="expandOnHover"
      />
      <div v-if="slots.logo" :style="headerStyle" class="sidebar-logo-area">
        <slot name="logo"></slot>
      </div>
      <VbenScrollbar :style="contentStyle" shadow shadow-border>
        <slot></slot>
      </VbenScrollbar>

      <div :style="collapseStyle"></div>
      <SidebarCollapseButton
        v-if="showCollapseButton && !isSidebarMixed"
        v-model:collapsed="collapse"
      />
    </div>
    <div
      v-if="isSidebarMixed"
      :class="[
        themeSub,
        {
          'border-l': extraVisible,
        },
      ]"
      :style="extraStyle"
      class="fixed top-0 h-full overflow-hidden border-r border-border bg-sidebar transition-all duration-200"
    >
      <SidebarCollapseButton
        v-if="isSidebarMixed && expandOnHover"
        v-model:collapsed="extraCollapse"
      />

      <SidebarFixedButton
        v-if="!extraCollapse"
        v-model:expand-on-hover="expandOnHover"
      />
      <div v-if="!extraCollapse" :style="extraTitleStyle" class="pl-2">
        <slot name="extra-title"></slot>
      </div>
      <VbenScrollbar
        :style="extraContentStyle"
        class="border-border py-2"
        shadow
        shadow-border
      >
        <slot name="extra"></slot>
      </VbenScrollbar>
    </div>
    <div
      v-if="draggable"
      ref="dragBarRef"
      class="absolute inset-y-0 -right-px z-1000 w-0.5 cursor-col-resize hover:bg-primary"
      @mousedown="handleDragSidebar"
    ></div>
  </aside>
</template>
