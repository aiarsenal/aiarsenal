<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  AutoComplete,
  Button,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
} from 'ant-design-vue';

import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  deleteClientAppApi,
  getClientAppListApi,
  APP_PERM_CATS,
  saveClientAppApi,
  suggestClientAppsApi,
  type ClientAppPermApi,
} from '#/api/system/client/appperm';

defineOptions({ name: 'SystemClientAppPerm' });

// ===== App 目录（App 权限管理候选 App；设备级勾选在 设备管理 → 运行详情 里配） =====

/** 类别 key → 枚举章色词（接 styles/enum-badge.css 调色板，每类别一色） */
function catColor(key: string): string {
  switch (key) {
    case 'comm': { return 'success'; } // 沟通协作=绿
    case 'docs': { return 'processing'; } // 文档办公=蓝
    case 'tools': { return 'warning'; } // 办公工具=橙红
    case 'browser': { return 'geekblue'; } // 浏览器=青
    case 'ai': { return 'purple'; } // AI助手=洋红
    case 'system': { return 'grey'; } // 系统应用=灰
    default: { return 'default'; } // 未分类=灰
  }
}

/** 类别 key → i18n 文案；未知/空值兜底「其他」 */
function catLabel(key: string): string {
  return APP_PERM_CATS.includes(key as any)
    ? $t(`client.appperm_cat_${key}`)
    : $t('client.appperm_cat_other');
}

const catOptions = APP_PERM_CATS.map((c) => ({
  value: c,
  label: catLabel(c),
}));

// 真实图标 public/app-logos/<bundleId>.png；加载失败回退 emoji（logo 字段/📦）
const logoErr = reactive<Record<string, boolean>>({});

// ===== 搜索表单（关键词 + 类别筛选） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'keyword',
    label: $t('client.appperm_keywordPlaceholder'),
    componentProps: {
      allowClear: true,
      placeholder: $t('client.appperm_keywordPlaceholder'),
    },
  },
  {
    component: 'Select',
    fieldName: 'category',
    label: $t('client.appperm_category'),
    componentProps: {
      allowClear: true,
      options: catOptions,
      placeholder: $t('client.appperm_category'),
    },
  },
]);

// query 运行时读表单当前值（同 status 页写法：刷新/内置刷新/submitOnChange 共用同一份）
let gridApiRef: any;
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('client.appperm_seq'), width: 50, fixed: 'left' },
    {
      field: 'logo',
      title: $t('client.appperm_logo'),
      width: 70,
      align: 'center',
      slots: { default: 'logo' },
    },
    { field: 'appName', title: $t('client.appperm_appName'), minWidth: 140 },
    {
      field: 'category',
      title: $t('client.appperm_category'),
      width: 110,
      slots: { default: 'category' },
    },
    {
      field: 'bundleId',
      title: $t('client.appperm_bundleId'),
      minWidth: 240,
      slots: { default: 'bundleId' },
    },
    { field: 'weigh', title: $t('client.appperm_weigh'), width: 80 },
    {
      field: 'remarks',
      title: $t('client.appperm_remarks'),
      minWidth: 150,
      showOverflow: true,
    },
    {
      field: 'action',
      title: $t('client.appperm_action'),
      width: 150,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async (params: any) => {
        const form = (await gridApiRef?.formApi?.getValues?.()) ?? {};
        const f = form as { keyword?: string; category?: string };
        const res = await getClientAppListApi();
        // 后端已按 weigh desc, app_name asc 排好，前端只过滤 + 分页
        const k = (f.keyword ?? '').trim().toLowerCase();
        const all = ((res?.list ?? []) as any[]).filter((a) => {
          if (f.category && (a.category || '') !== f.category) return false;
          if (
            k &&
            !String(a.appName || '').toLowerCase().includes(k) &&
            !String(a.bundleId || '').toLowerCase().includes(k) &&
            !catLabel(String(a.category || '')).toLowerCase().includes(k)
          ) {
            return false;
          }
          return true;
        });
        const page = params?.page;
        const total = all.length;
        const start = page ? (page.currentPage - 1) * page.pageSize : 0;
        const pageSize = page ? page.pageSize : total;
        return { items: all.slice(start, start + pageSize), total };
      },
    },
  },
  toolbarConfig: {
    custom: true,
    refresh: { code: 'query' },
  },
}));

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions: { schema: searchSchema, submitOnChange: true },
});
gridApiRef = gridApi;

// ===== 新增/编辑弹窗 =====
const editVisible = ref(false);
const editSaving = ref(false);
const editForm = reactive({
  appId: '',
  bundleId: '',
  appName: '',
  category: '',
  logo: '',
  weigh: 0,
  remarks: '',
});

function openAdd() {
  editForm.appId = '';
  editForm.bundleId = '';
  editForm.appName = '';
  editForm.category = '';
  editForm.logo = '';
  editForm.weigh = 0;
  editForm.remarks = '';
  suggests.value = [];
  editVisible.value = true;
}

function openEdit(row: any) {
  editForm.appId = row.appId;
  editForm.bundleId = row.bundleId;
  editForm.appName = row.appName;
  editForm.category = row.category || '';
  editForm.logo = row.logo || '';
  editForm.weigh = row.weigh || 0;
  editForm.remarks = row.remarks || '';
  suggests.value = [];
  editVisible.value = true;
}

// ===== Bundle ID 联想（后端代理 Apple iTunes Search 查 Mac App Store；查不到降级手填） =====
const suggests = ref<ClientAppPermApi.AppSuggest[]>([]);
let suggestTimer: ReturnType<typeof setTimeout> | undefined;

async function fetchSuggests(kw: string) {
  const k = kw.trim();
  if (!k) {
    suggests.value = [];
    return;
  }
  try {
    const res = await suggestClientAppsApi(k);
    suggests.value = res?.list ?? [];
  } catch {
    suggests.value = []; // 静默降级：不打扰，用户手填
  }
}

/** trackName 与 App 名互含（大小写不敏感）算强匹配；精确相等更优 */
function strongMatch(s: ClientAppPermApi.AppSuggest, name: string): boolean {
  const t = (s.trackName || '').toLowerCase();
  const n = name.trim().toLowerCase();
  return !!t && !!n && (t === n || t.includes(n) || n.includes(t));
}

/** App 名输入停顿 500ms 后拉候选；Bundle ID 还空着时按「精确相等 → 本机互含」择一自动填充。
 *  精确相等唯一（如输 Cursor 命中本机 Cursor，排除 CursorEffect2 杂项）或
 *  本机互含唯一（如中文名命中本机 App）才填，多候选留给用户下拉选。 */
function onAppNameInput() {
  if (suggestTimer) clearTimeout(suggestTimer);
  suggestTimer = setTimeout(async () => {
    const name = editForm.appName;
    if (!name.trim() || editForm.appId) return; // 编辑模式 bundleId 不可改，不联想
    await fetchSuggests(name);
    if (editForm.bundleId.trim()) return;
    const n = name.trim().toLowerCase();
    const pick =
      suggests.value.find((s) => (s.trackName || '').toLowerCase() === n) ??
      (() => {
        const local = suggests.value.filter(
          (s) => s.source === 'local' && strongMatch(s, name),
        );
        return local.length === 1 ? local[0] : undefined;
      })();
    if (pick) {
      editForm.bundleId = pick.bundleId;
      message.success($t('client.appperm_autofilled'));
    }
  }, 500);
}

/** Bundle ID 框内直接输关键词（App 名/英文片段）也能搜；防抖后拉候选 */
function onBundleSearch(kw: string) {
  if (suggestTimer) clearTimeout(suggestTimer);
  suggestTimer = setTimeout(() => {
    if (!editForm.appId) fetchSuggests(kw || editForm.appName);
  }, 500);
}

const suggestOptions = computed(() =>
  suggests.value.map((s) => ({ value: s.bundleId, label: s.trackName, suggest: s })),
);

async function saveApp() {
  if (!editForm.bundleId.trim() || !editForm.appName.trim()) {
    message.warning($t('client.appperm_required'));
    return;
  }
  editSaving.value = true;
  try {
    await saveClientAppApi({
      appId: editForm.appId || undefined,
      bundleId: editForm.bundleId.trim(),
      appName: editForm.appName.trim(),
      category: editForm.category || '',
      logo: editForm.logo.trim(),
      weigh: editForm.weigh || 0,
      remarks: editForm.remarks.trim(),
    });
    message.success($t('client.appperm_appSaved'));
    editVisible.value = false;
    gridApi.query();
  } catch (e: any) {
    message.error(e?.message || $t('client.appperm_saveFailed'));
  } finally {
    editSaving.value = false;
  }
}

function removeApp(row: any) {
  Modal.confirm({
    title: $t('client.appperm_tip'),
    content: $t('client.appperm_deleteContent') + row.appName + '?',
    okText: $t('client.appperm_confirm'),
    cancelText: $t('client.appperm_cancel'),
    onOk: async () => {
      try {
        await deleteClientAppApi(row.appId);
        message.success($t('client.appperm_deleted'));
        gridApi.query();
      } catch (e: any) {
        message.error(e?.message || $t('client.appperm_saveFailed'));
      }
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">
          {{ $t('client.appperm_add') }}
        </Button>
      </template>
      <template #logo="{ row }">
        <img
          v-if="!logoErr[row.bundleId]"
          :src="`/app-logos/${row.bundleId}.png`"
          class="app-logo"
          :alt="row.appName"
          @error="logoErr[row.bundleId] = true"
        />
        <span v-else style="font-size: 20px">{{ row.logo || '📦' }}</span>
      </template>
      <template #category="{ row }">
        <span :class="['aa-enum', `aa-enum--${catColor(row.category)}`]">
          {{ catLabel(row.category || '') }}
        </span>
      </template>
      <template #bundleId="{ row }">
        <code style="font-size: 12px">{{ row.bundleId }}</code>
      </template>
      <template #action="{ row }">
        <!-- flex+gap 布局：showOverflow 单元格 nowrap+ellipsis，inline 按钮超宽会被裁 -->
        <div class="row-actions">
          <Button type="primary" ghost size="small" @click="openEdit(row)">
            {{ $t('client.appperm_edit') }}
          </Button>
          <Button type="primary" ghost size="small" danger @click="removeApp(row)">
            {{ $t('client.appperm_delete') }}
          </Button>
        </div>
      </template>
    </Grid>

    <Modal
      v-model:open="editVisible"
      :title="editForm.appId ? $t('client.appperm_edit') : $t('client.appperm_add')"
      :confirm-loading="editSaving"
      :ok-text="$t('client.appperm_confirm')"
      :cancel-text="$t('client.appperm_cancel')"
      @ok="saveApp"
    >
      <div class="edit-row">
        <label class="edit-label">{{ $t('client.appperm_appName') }}</label>
        <Input
          v-model:value="editForm.appName"
          style="width: 100%"
          @input="onAppNameInput"
        />
      </div>
      <div class="edit-row">
        <label class="edit-label">{{ $t('client.appperm_bundleId') }}</label>
        <AutoComplete
          v-model:value="editForm.bundleId"
          :options="suggestOptions"
          :disabled="!!editForm.appId"
          :placeholder="$t('client.appperm_bundleIdPlaceholder')"
          style="width: 100%"
          @search="onBundleSearch"
        >
          <template #option="{ suggest }">
            <div class="suggest-item">
              <img v-if="suggest.artworkUrl" :src="suggest.artworkUrl" class="suggest-icon" alt="" />
              <span v-else class="suggest-icon">{{ suggest.source === 'local' ? '💾' : '📦' }}</span>
              <div class="suggest-main">
                <div class="suggest-name">{{ suggest.trackName }}</div>
                <code class="suggest-bundle">{{ suggest.bundleId }}</code>
              </div>
              <span v-if="suggest.source === 'local'" class="suggest-source">
                {{ $t('client.appperm_suggestLocal') }}
              </span>
              <span v-else class="suggest-artist">{{ suggest.artistName }}</span>
            </div>
          </template>
        </AutoComplete>
        <div class="edit-tip">{{ $t('client.appperm_bundleIdTip') }}</div>
      </div>
      <div class="edit-row">
        <label class="edit-label">{{ $t('client.appperm_category') }}</label>
        <Select
          v-model:value="editForm.category"
          :options="catOptions"
          :placeholder="$t('client.appperm_cat_other')"
          allow-clear
          style="width: 100%"
        />
      </div>
      <div class="edit-row" style="display: flex; gap: 12px">
        <div style="flex: 1">
          <label class="edit-label">{{ $t('client.appperm_logo') }}</label>
          <Input v-model:value="editForm.logo" placeholder="🖥️" />
        </div>
        <div style="width: 120px">
          <label class="edit-label">{{ $t('client.appperm_weigh') }}</label>
          <InputNumber v-model:value="editForm.weigh" :min="0" :max="9999" style="width: 100%" />
        </div>
      </div>
      <div class="edit-row">
        <label class="edit-label">{{ $t('client.appperm_remarks') }}</label>
        <Input v-model:value="editForm.remarks" style="width: 100%" />
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
.row-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.app-logo {
  display: inline-block;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  vertical-align: middle;
}
.edit-row {
  margin-bottom: 12px;
}
.edit-label {
  display: block;
  margin-bottom: 4px;
  font-size: 13px;
  color: #555;
}
.edit-tip {
  margin-top: 4px;
  font-size: 12px;
  color: #999;
}
.suggest-item {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 2px 0;
}
.suggest-icon {
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 5px;
  font-size: 18px;
  line-height: 22px;
  text-align: center;
}
.suggest-main {
  flex: 1;
  min-width: 0;
}
.suggest-name {
  font-size: 13px;
  line-height: 1.3;
}
.suggest-bundle {
  font-size: 11px;
  color: #888;
}
.suggest-source {
  flex: none;
  padding: 0 6px;
  border: 1px solid #52c41a;
  border-radius: 3px;
  color: #52c41a;
  font-size: 11px;
  line-height: 18px;
}
.suggest-artist {
  flex: none;
  max-width: 130px;
  overflow: hidden;
  font-size: 11px;
  color: #aaa;
  text-overflow: ellipsis;
  white-space: nowrap;
}
html.dark .edit-label {
  color: #aaa;
}
html.dark .edit-tip {
  color: #777;
}
</style>
