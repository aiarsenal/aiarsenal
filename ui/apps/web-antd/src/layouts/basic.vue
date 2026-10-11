<script lang="ts" setup>
import type { NotificationItem } from '@vben/layouts';

import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationLoginExpiredModal } from '@vben/common-ui';
import { VBEN_DOC_URL, VBEN_GITHUB_URL } from '@vben/constants';
import { useWatermark } from '@vben/hooks';
import { BookOpenText, CircleHelp, SvgGithubIcon } from '@vben/icons';
import {
  BasicLayout,
  LanguageToggle,
  LockScreen,
  Notification,
  ThemeToggle,
  UserDropdown,
} from '@vben/layouts';
import { preferences, usePreferences } from '@vben/preferences';
import { useAccessStore, useUserStore } from '@vben/stores';
import { openWindow } from '@vben/utils';

import { Button, Modal, Tag } from 'ant-design-vue';

import {
  deleteNoticeApi,
  getNoticeListApi,
  readAllNoticeApi,
  readNoticeApi,
} from '#/api/system/notice';

import { $t } from '#/locales';
import { useAuthStore } from '#/store';
import NoticeBrowser from '#/components/NoticeBrowser.vue';
import LoginForm from '#/views/_core/authentication/login.vue';

// 通知数据：从 gin /system/notice/list 拉取真实数据
const notifications = ref<NotificationItem[]>([]);
// 「查看所有消息」浏览弹窗
const noticeBrowserOpen = ref(false);

async function loadNotices() {
  try {
    const res = await getNoticeListApi({ pageNum: 1, pageSize: 20 });
    const list = res?.noticeList ?? [];
    notifications.value = list.map((n: any) => ({
      id: n.noticeId,
      avatar: '',
      title: n.title,
      message: n.content,
      date: n.createdAt || '',
      isRead: n.isRead === 1,
      link: n.link || undefined,
      type: n.type || 'info',
    }));
  } catch {
    /* 忽略：未登录或接口异常时保持空列表 */
  }
}

onMounted(() => {
  loadNotices();
});

const router = useRouter();
const userStore = useUserStore();
const authStore = useAuthStore();
const accessStore = useAccessStore();
const { destroyWatermark, updateWatermark } = useWatermark();
const { isDark } = usePreferences();
const showDot = computed(() =>
  notifications.value.some((item) => !item.isRead),
);

const menus = computed(() => [
  {
    handler: () => {
      router.push({ name: 'Profile' });
    },
    icon: 'lucide:user',
    text: $t('page.auth.profile'),
  },
  {
    handler: () => {
      openWindow(VBEN_DOC_URL, {
        target: '_blank',
      });
    },
    icon: BookOpenText,
    text: $t('ui.widgets.document'),
  },
  {
    handler: () => {
      openWindow(VBEN_GITHUB_URL, {
        target: '_blank',
      });
    },
    icon: SvgGithubIcon,
    text: 'GitHub',
  },
  {
    handler: () => {
      openWindow(`${VBEN_GITHUB_URL}/issues`, {
        target: '_blank',
      });
    },
    icon: CircleHelp,
    text: $t('ui.widgets.qa'),
  },
]);

const avatar = computed(() => {
  return userStore.userInfo?.avatar ?? preferences.app.defaultAvatar;
});

async function handleLogout() {
  await authStore.logout(false);
}

// 侧栏 logo + 文字点击：新标签页打开官网介绍页（/）
function onClickLogo() {
  window.open('/', '_blank');
}

function handleNoticeClear() {
  notifications.value = [];
}

async function markRead(id: number | string) {
  const item = notifications.value.find((item) => item.id === id);
  if (item && !item.isRead) {
    item.isRead = true;
    try {
      await readNoticeApi(String(id));
    } catch {
      /* 忽略 */
    }
  }
}

async function remove(id: number | string) {
  notifications.value = notifications.value.filter((item) => item.id !== id);
  try {
    await deleteNoticeApi(String(id));
  } catch {
    /* 忽略 */
  }
}

async function handleMakeAll() {
  notifications.value.forEach((item) => (item.isRead = true));
  try {
    await readAllNoticeApi();
  } catch {
    /* 忽略 */
  }
}

const viewAll = () => {
  noticeBrowserOpen.value = true;
};

// ===== 通知详情弹窗：点击任一条通知 → 看全文并标已读 =====
const detailOpen = ref(false);
const detail = ref<NotificationItem | null>(null);

function typeTag(t?: string) {
  switch (t) {
    case 'warning': {
      return { color: 'gold', text: '警告' };
    }
    case 'success': {
      return { color: 'magenta', text: '成功' };
    }
    case 'error': {
      return { color: 'red', text: '错误' };
    }
    default: {
      return { color: 'blue', text: '信息' };
    }
  }
}

const handleClick = (item: NotificationItem) => {
  detail.value = item;
  detailOpen.value = true;
  if (item.id) markRead(item.id);
};

function detailGo() {
  const it = detail.value;
  if (!it?.link) return;
  detailOpen.value = false;
  navigateTo(it.link, it.query, it.state);
}

function navigateTo(
  link: string,
  query?: Record<string, any>,
  state?: Record<string, any>,
) {
  if (link.startsWith('http://') || link.startsWith('https://')) {
    // 外部链接，在新标签页打开
    window.open(link, '_blank');
  } else {
    // 内部路由链接，支持 query 参数和 state
    router.push({
      path: link,
      query: query || {},
      state,
    });
  }
}

watch(
  () => ({
    enable: preferences.app.watermark,
    content: preferences.app.watermarkContent,
    isDark: isDark.value,
  }),
  async ({ enable, content, isDark: isDarkValue }) => {
    if (enable) {
      const watermarkColor = isDarkValue
        ? 'rgba(255, 255, 255, 0.12)'
        : 'rgba(0, 0, 0, 0.12)';

      await updateWatermark({
        advancedStyle: {
          colorStops: [
            {
              color: watermarkColor,
              offset: 0,
            },
            {
              color: watermarkColor,
              offset: 1,
            },
          ],
          type: 'linear',
        },
        content:
          content ||
          `${userStore.userInfo?.username} - ${userStore.userInfo?.realName}`,
      });
    } else {
      destroyWatermark();
    }
  },
  {
    immediate: true,
  },
);
</script>

<template>
  <BasicLayout
    @clear-preferences-and-logout="handleLogout"
    @click-logo="onClickLogo"
  >
    <template #user-dropdown>
      <UserDropdown
        :avatar
        :menus
        :text="userStore.userInfo?.realName"
        description="ann.vben@gmail.com"
        tag-text="Pro"
        @logout="handleLogout"
        @clear-preferences-and-logout="handleLogout"
      />
    </template>
    <template #notification>
      <Notification
        :dot="showDot"
        :notifications="notifications"
        @clear="handleNoticeClear"
        @read="(item) => item.id && markRead(item.id)"
        @refresh="loadNotices"
        @remove="(item) => item.id && remove(item.id)"
        @make-all="handleMakeAll"
        @on-click="handleClick"
        @view-all="viewAll"
      />
    </template>
    <!-- 偏好设置齿轮 / 明暗切换 / 内置语言切换 全部置空隐藏 -->
    <!-- （enablePreferences:false 时这些插槽不会被列出；此处兜底防止缓存残留时显示） -->
    <template #preferences></template>
    <template #theme-toggle></template>
    <template #language-toggle></template>
    <!-- 独立明暗切换按钮：不受 enablePreferences 开关影响，始终可见 -->
    <template #header-right-120>
      <ThemeToggle class="mt-0.5 mr-1" />
    </template>
    <!-- 独立语言切换按钮：不受 enablePreferences 开关影响，始终可见 -->
    <template #header-right-130>
      <LanguageToggle class="mr-1" />
    </template>
    <template #extra>
      <AuthenticationLoginExpiredModal
        v-model:open="accessStore.loginExpired"
        :avatar
      >
        <LoginForm />
      </AuthenticationLoginExpiredModal>
    </template>
    <template #lock-screen>
      <LockScreen :avatar @to-login="handleLogout" />
    </template>
  </BasicLayout>
  <!-- 「查看所有消息」浏览弹窗：状态/类型/时间筛选 + 翻页（放 BasicLayout 外：它无默认 slot，放内部不渲染） -->
  <NoticeBrowser v-model:open="noticeBrowserOpen" @changed="loadNotices" />
  <!-- 通知详情弹窗：点击铃铛列表里的任一条打开，全文展示 + 前往链接 -->
  <Modal
    v-model:open="detailOpen"
    :title="detail?.title || '通知详情'"
    :footer="null"
    :width="520"
  >
    <div v-if="detail" class="flex flex-col gap-3 py-2">
      <div class="flex items-center gap-2">
        <Tag :color="typeTag(detail.type).color">
          {{ typeTag(detail.type).text }}
        </Tag>
        <span class="text-xs opacity-60">{{ detail.date }}</span>
      </div>
      <div class="text-sm break-words whitespace-pre-wrap">
        {{ detail.message }}
      </div>
      <div v-if="detail.link" class="text-right">
        <Button type="primary" size="small" @click="detailGo">前往查看</Button>
      </div>
    </div>
  </Modal>
</template>
