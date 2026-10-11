<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Switch } from 'ant-design-vue';

import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  addMenuApi,
  deleteMenuApi,
  getMenuInfoApi,
  getMenuListApi,
  getMenuParamsApi,
  updateMenuApi,
} from '#/api/system/menu';
import GrantEditor, { type GrantRowValue } from '#/components/GrantEditor.vue';
import { $t } from '#/locales';

defineOptions({ name: 'SystemMenu' });

// ===== 上级菜单（getMenuParamsApi；角色直授已收口到角色管理页，此处不再拉角色）=====
const params = reactive<{ menus: any[] }>({
  menus: [],
});

async function loadParams() {
  try {
    const res: any = await getMenuParamsApi();
    params.menus = res?.menus ?? [];
  } catch {
    /* 忽略 */
  }
}
onMounted(loadParams);

// 扁平 menus → 树（id/pid），供父级 TreeSelect 使用
const menuTreeData = computed(() => buildMenuTree(params.menus, '0'));
function buildMenuTree(list: any[], rootPid: string): any[] {
  const map: Record<string, any> = {};
  const roots: any[] = [];
  (list ?? []).forEach((m: any) => {
    map[m.id] = { value: m.id, label: m.title, children: [] as any[] };
  });
  (list ?? []).forEach((m: any) => {
    const node = map[m.id];
    if (!m.pid || m.pid === rootPid || m.pid === '0' || !map[m.pid]) {
      roots.push(node);
    } else {
      map[m.pid]?.children.push(node);
      if (map[m.pid] && map[m.pid].children.length === 0) {
        delete map[m.pid].children;
      }
    }
  });
  // 清理空 children
  const clean = (nodes: any[]) => {
    nodes.forEach((n: any) => {
      if (n.children?.length) clean(n.children);
      else delete n.children;
    });
  };
  clean(roots);
  return roots;
}

// 菜单数量不多，直接展示整棵树，不需要搜索表单。

// ===== 菜单图标 emoji 选项（约定 aa_menu.icon 存 emoji，见 menu-icon-emoji-rule）=====
// label 含中英关键词，Select optionFilterProp='label' 即可中英搜索。
const EMOJIS: { label: string; value: string; }[] = [
  { value: '⚙️', label: '⚙️ 系统设置 system' },
  { value: '🛠️', label: '🛠️ 工具 tools' },
  { value: '🔧', label: '🔧 配置 config' },
  { value: '🔩', label: '🔩 螺丝 screw' },
  { value: '🧰', label: '🧰 工具箱 toolbox' },
  { value: '🎛️', label: '🎛️ 控制台 console' },
  { value: '🖥️', label: '🖥️ 桌面 desktop' },
  { value: '💻', label: '💻 电脑 computer' },
  { value: '📱', label: '📱 手机 mobile' },
  { value: '🌐', label: '🌐 网络 web' },
  { value: '🧭', label: '🧭 导航 navigate' },
  { value: '🧩', label: '🧩 模块 module' },
  { value: '🗂️', label: '🗂️ 分类 category' },
  { value: '📂', label: '📂 文件夹 folder' },
  { value: '📁', label: '📁 目录 directory' },
  { value: '📋', label: '📋 列表 list' },
  { value: '📄', label: '📄 文档 file' },
  { value: '📝', label: '📝 备注 note' },
  { value: '🗒️', label: '🗒️ 记事 memo' },
  { value: '📕', label: '📕 手册 manual' },
  { value: '📗', label: '📗 指南 guide' },
  { value: '📚', label: '📚 知识库 library' },
  { value: '📖', label: '📖 文档 doc' },
  { value: '📌', label: '📌 标记 pin' },
  { value: '📎', label: '📎 附件 attach' },
  { value: '🔖', label: '🔖 书签 bookmark' },
  { value: '🏷️', label: '🏷️ 标签 tag' },
  { value: '📦', label: '📦 包 package' },
  { value: '👥', label: '👥 用户组 users' },
  { value: '👤', label: '👤 用户 user' },
  { value: '🧑', label: '🧑 成员 member' },
  { value: '💼', label: '💼 商务 business' },
  { value: '🛡️', label: '🛡️ 安全 security' },
  { value: '🔑', label: '🔑 密钥 key' },
  { value: '🔒', label: '🔒 锁定 lock' },
  { value: '🔓', label: '🔓 解锁 unlock' },
  { value: '🎫', label: '🎫 凭证 ticket' },
  { value: '🎖️', label: '🎖️ 角色 role' },
  { value: '🖼️', label: '🖼️ 图片 image' },
  { value: '🎨', label: '🎨 主题 theme' },
  { value: '📸', label: '📸 截图 photo' },
  { value: '🎥', label: '🎥 视频 video' },
  { value: '🎬', label: '🎬 录制 record' },
  { value: '🎵', label: '🎵 音频 audio' },
  { value: '🎮', label: '🎮 游戏 game' },
  { value: '🎲', label: '🎲 随机 random' },
  { value: '🎁', label: '🎁 礼物 gift' },
  { value: '💎', label: '💎 精选 diamond' },
  { value: '💬', label: '💬 消息 chat' },
  { value: '📧', label: '📧 邮件 mail' },
  { value: '✉️', label: '✉️ 信件 letter' },
  { value: '📨', label: '📨 收件 inbox' },
  { value: '📣', label: '📣 公告 announce' },
  { value: '🔔', label: '🔔 通知 notify' },
  { value: '📡', label: '📡 信号 signal' },
  { value: '📊', label: '📊 统计 chart' },
  { value: '📈', label: '📈 增长 trend-up' },
  { value: '📉', label: '📉 下降 trend-down' },
  { value: '💹', label: '💹 行情 market' },
  { value: '🧾', label: '🧾 账单 receipt' },
  { value: '💰', label: '💰 资金 money' },
  { value: '💵', label: '💵 货币 currency' },
  { value: '💳', label: '💳 支付 pay' },
  { value: '🏦', label: '🏦 银行 bank' },
  { value: '🛒', label: '🛒 购物 cart' },
  { value: '🛍️', label: '🛍️ 订单 order' },
  { value: '🚀', label: '🚀 部署 deploy' },
  { value: '🎯', label: '🎯 目标 target' },
  { value: '🏆', label: '🏆 成就 award' },
  { value: '🎉', label: '🎉 活动 event' },
  { value: '✅', label: '✅ 正常 ok' },
  { value: '❌', label: '❌ 失败 fail' },
  { value: '⚠️', label: '⚠️ 警告 warning' },
  { value: '❓', label: '❓ 帮助 help' },
  { value: '❗', label: '❗ 重要 important' },
  { value: 'ℹ️', label: 'ℹ️ 信息 info' },
  { value: '💡', label: '💡 提示 idea' },
  { value: '🔥', label: '🔥 热门 hot' },
  { value: '♻️', label: '♻️ 刷新 refresh' },
  { value: '🔄', label: '🔄 同步 sync' },
  { value: '⚡', label: '⚡ 闪电 fast' },
  { value: '🌈', label: '🌈 多彩 rainbow' },
  { value: '⭐', label: '⭐ 收藏 star' },
  { value: '✨', label: '✨ 新增 new' },
  { value: '🍀', label: '🍀 幸运 lucky' },
  { value: '🌙', label: '🌙 夜间 night' },
  { value: '☀️', label: '☀️ 日间 day' },
];
// 当前编辑值若不在列表内（历史数据），动态补一项，避免 Select 回显空。
const currentIcon = ref('');
const iconOptions = computed(() => {
  const cur = currentIcon.value;
  if (cur && !EMOJIS.some((e) => e.value === cur)) {
    return [{ value: cur, label: cur }, ...EMOJIS];
  }
  return EMOJIS;
});

// 树 → 扁平（list 接口无搜索时返回树，统一打平后交给 vxe-grid 重建树）
function flattenTree(list: any[]): any[] {
  const out: any[] = [];
  const walk = (nodes: any[]) => {
    (nodes ?? []).forEach((n: any) => {
      const { children, ...rest } = n;
      out.push(rest);
      if (children?.length) walk(children);
    });
  };
  walk(list);
  return out;
}

// ===== 表格 =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'seq', title: $t('sysmenu.seq'), width: 60, fixed: 'left' },
    { field: 'title', title: $t('sysmenu.title'), minWidth: 200, treeNode: true, slots: { default: 'title' } },
    { field: 'enName', title: $t('sysmenu.enName'), minWidth: 120 },
    { field: 'path', title: $t('sysmenu.path'), minWidth: 140 },
    { field: 'component', title: $t('sysmenu.componentLabel'), minWidth: 160 },
    { field: 'name', title: $t('sysmenu.name'), minWidth: 160 },
    { field: 'weigh', title: $t('sysmenu.weigh'), width: 80 },
    { field: 'menuType', title: $t('sysmenu.menuType'), width: 90, slots: { default: 'menuType' } },
    { field: 'isHide', title: $t('sysmenu.isHide'), width: 110, slots: { default: 'isHide' } },
    {
      field: 'action',
      title: $t('sysmenu.action'),
      width: 240,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: false },
  // rowConfig.keyField 必须设为稳定唯一键(id)，否则 vxe-table 的 reserve 会落到内部自增
  // _X_ROW_KEY，重载后对不上 → 展开/选中状态被清空（官方缺此配置会告警 vxe.error.reqProp）。
  rowConfig: { keyField: 'id' },
  // treeNode: true（在 title 列）让 vxe-grid 渲染展开/折叠 + 缩进，展示父子层级（同 dept 列表）；
  // 默认折叠，仅展示一级，下级点击展开；
  // reserve: true 配合上面的 keyField，刷新/重载后按 id 自动恢复树的展开状态。
  treeConfig: { transform: true, rowField: 'id', parentField: 'pid', expandRowKeys: [], reserve: true },
  proxyConfig: {
    ajax: {
      query: async () => {
        const res = await getMenuListApi({});
        const rules = res?.rules ?? [];
        // gin 无搜索时返回树，统一打平后让 vxe-grid 重建树
        return { items: flattenTree(rules), total: 0 };
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
});

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);

// 组合授权条件行（后端 model.GrantRow 的前端形态）；提交时序列化为 grantRules 字符串
const grantRows = ref<GrantRowValue[]>([]);
function parseGrantRules(s?: string): any[] {
  try {
    const v = JSON.parse(s ?? '[]');
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}
const modalTitle = computed(() =>
  isEdit.value ? $t('sysmenu.modalTitleEdit') : $t('sysmenu.modalTitleAdd'),
);

const menuTypeOptions = computed(() => [
  { label: $t('sysmenu.typeDir'), value: 0 },
  { label: $t('sysmenu.typeMenu'), value: 1 },
  { label: $t('sysmenu.typeButton'), value: 2 },
]);
const yesNoOptions = computed(() => [
  { label: $t('sysmenu.yes'), value: 1 },
  { label: $t('sysmenu.no'), value: 0 },
]);
const showHideOptions = computed(() => [
  { label: $t('sysmenu.show'), value: 0 },
  { label: $t('sysmenu.hide'), value: 1 },
]);

const formSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'TreeSelect' as const,
    fieldName: 'parentId',
    label: $t('sysmenu.parentId'),
    componentProps: () => ({
      treeData: menuTreeData.value,
      placeholder: $t('sysmenu.parentIdPlaceholder'),
      allowClear: true,
      treeDefaultExpandAll: true,
      style: { width: '100%' },
    }),
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'menuType',
    label: $t('sysmenu.menuTypeLabel'),
    rules: 'required',
    defaultValue: 0,
    componentProps: { options: menuTypeOptions.value },
  },
  {
    component: 'Input' as const,
    fieldName: 'menuName',
    label: $t('sysmenu.menuNameLabel'),
    rules: 'required',
    componentProps: { placeholder: $t('sysmenu.menuNameLabelPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'enName',
    label: $t('sysmenu.enName'),
    componentProps: { placeholder: $t('sysmenu.enNamePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('sysmenu.name'),
    rules: 'required',
    componentProps: { placeholder: $t('sysmenu.namePlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'path',
    label: $t('sysmenu.path'),
    componentProps: { placeholder: $t('sysmenu.pathPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'component',
    label: $t('sysmenu.componentLabel'),
    componentProps: { placeholder: $t('sysmenu.componentLabelPlaceholder') },
  },
  {
    component: 'Input' as const,
    fieldName: 'redirect',
    label: $t('sysmenu.redirect'),
    componentProps: { placeholder: $t('sysmenu.redirectPlaceholder') },
  },
  {
    component: 'Select' as const,
    fieldName: 'icon',
    label: $t('sysmenu.icon'),
    componentProps: () => ({
      options: iconOptions.value,
      showSearch: true,
      optionFilterProp: 'label',
      allowClear: true,
      placeholder: $t('sysmenu.iconPlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input' as const,
    fieldName: 'linkUrl',
    label: $t('sysmenu.linkUrl'),
    componentProps: { placeholder: $t('sysmenu.linkUrlPlaceholder') },
  },
  {
    component: 'InputNumber' as const,
    fieldName: 'menuSort',
    label: $t('sysmenu.menuSort'),
    defaultValue: 0,
    componentProps: { style: { width: '100%' } },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isHide',
    label: $t('sysmenu.isHideLabel'),
    defaultValue: 0,
    componentProps: { options: showHideOptions.value },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isKeepAlive',
    label: $t('sysmenu.isKeepAlive'),
    defaultValue: 1,
    componentProps: { options: yesNoOptions.value },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isAffix',
    label: $t('sysmenu.isAffix'),
    defaultValue: 0,
    componentProps: { options: yesNoOptions.value },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isLink',
    label: $t('sysmenu.isLink'),
    defaultValue: 0,
    componentProps: { options: yesNoOptions.value },
  },
  {
    component: 'RadioGroup' as const,
    fieldName: 'isIframe',
    label: $t('sysmenu.isIframe'),
    defaultValue: 0,
    componentProps: { options: yesNoOptions.value },
  },
]);

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: formSchema,
    showDefaultActions: false,
  }) as any,
);

async function openAdd(row?: any) {
  isEdit.value = false;
  editingId.value = '';
  currentIcon.value = '';
  grantRows.value = [];
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  formApi.setValues({
    menuType: 0,
    menuSort: 0,
    isHide: 0,
    isKeepAlive: 1,
    isAffix: 0,
    isLink: 0,
    isIframe: 0,
    parentId: row?.id ?? '',
  });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.id;
  try {
    const res: any = await getMenuInfoApi(row.id);
    const rule = res?.rule ?? {};
    modalVisible.value = true;
    await nextTick();
    await formApi.resetForm();
    currentIcon.value = rule.icon ?? '';
    grantRows.value = parseGrantRules(rule.grantRules);
    formApi.setValues({
      parentId: !rule.pid || rule.pid === '0' ? '' : rule.pid,
      menuType: Number(rule.menuType ?? 0),
      menuName: rule.title ?? '',
      enName: rule.enName ?? '',
      name: rule.name ?? '',
      path: rule.path ?? '',
      component: rule.component ?? '',
      redirect: !rule.redirect || rule.redirect === '0' ? '' : rule.redirect,
      icon: rule.icon ?? '',
      linkUrl: rule.linkUrl ?? '',
      menuSort: Number(rule.weigh ?? 0),
      isHide: Number(rule.isHide ?? 0),
      isKeepAlive: Number(rule.isCached ?? 1),
      isAffix: Number(rule.isAffix ?? 0),
      isLink: Number(rule.isLink ?? 0),
      isIframe: Number(rule.isIframe ?? 0),
    });
  } catch {
    message.error($t('sysmenu.getMenuInfoFailed'));
  }
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  // 后端以 pid="0" 表示根菜单（buildTree pid="0" 起），空值需回填为 "0"。
  // 组合授权条件行随表单一并提交（后端校验+规范序列化+编译 casbin 表达式）。
  const payload = {
    ...values,
    parentId: values.parentId || '0',
    grantRules: JSON.stringify(grantRows.value ?? []),
  };
  try {
    if (isEdit.value) {
      await updateMenuApi({ ...payload, id: editingId.value });
      message.success($t('sysmenu.editSuccess'));
    } else {
      await addMenuApi(payload);
      message.success($t('sysmenu.addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// ===== 删除 =====
async function onDeleteRow(row: any) {
  Modal.confirm({
    title: $t('sysmenu.confirmDeleteTitle'),
    content: $t('sysmenu.confirmDeleteContent', { name: row.title }),
    okText: $t('sysmenu.confirm'),
    cancelText: $t('sysmenu.cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteMenuApi([row.id]);
      message.success($t('sysmenu.deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 显示状态（isHide）行内切换：先取详情再全量更新，失败回滚 =====
async function onIsHideChange(row: any, checked: boolean) {
  const newIsHide = checked ? 0 : 1; // 显示=0，隐藏=1
  const prev = row.isHide;
  row.isHide = newIsHide;
  try {
    const res: any = await getMenuInfoApi(row.id);
    const rule: any = res?.rule ?? {};
    await updateMenuApi({
      id: row.id,
      parentId: rule.pid || '0',
      menuType: Number(rule.menuType ?? 0),
      menuName: rule.title ?? '',
      enName: rule.enName ?? '',
      name: rule.name ?? '',
      path: rule.path ?? '',
      component: rule.component ?? '',
      redirect: rule.redirect ?? '',
      icon: rule.icon ?? '',
      linkUrl: rule.linkUrl ?? '',
      grantRules: rule.grantRules ?? '[]',
      menuSort: Number(rule.weigh ?? 0),
      isHide: newIsHide,
      isKeepAlive: Number(rule.isCached ?? 1),
      isAffix: Number(rule.isAffix ?? 0),
      isLink: Number(rule.isLink ?? 0),
      isIframe: Number(rule.isIframe ?? 0),
    });
    message.success($t('sysmenu.editSuccess'));
  } catch {
    row.isHide = prev;
  }
}

// ===== 渲染辅助 =====
const menuTypeMap = computed<Record<number, { color: string; label: string; }>>(() => ({
  0: { label: $t('sysmenu.typeDir'), color: 'blue' },
  1: { label: $t('sysmenu.typeMenu'), color: 'magenta' },
  2: { label: $t('sysmenu.typeButton'), color: 'orange' },
}));
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd()">{{ $t('sysmenu.add') }}</Button>
      </template>
      <template #title="{ row }">
        <span v-if="row.icon" class="mr-1">{{ row.icon }}</span>
        <span>{{ row.title }}</span>
      </template>
      <template #menuType="{ row }">
        <span class="aa-enum" :class="[`aa-enum--${menuTypeMap[row.menuType]?.color || 'default'}`]">
          {{ menuTypeMap[row.menuType]?.label || '-' }}
        </span>
      </template>
      <template #isHide="{ row }">
        <Switch
          :checked="row.isHide === 0"
          :checked-children="$t('sysmenu.show')"
          :un-checked-children="$t('sysmenu.hide')"
          @change="(v: any) => onIsHideChange(row, v)"
        />
      </template>
      <template #action="{ row }">
        <Button
          v-if="row.menuType !== 2"
          type="primary" ghost
          size="small"
          @click="openAdd(row)"
        >
          {{ $t('sysmenu.addChild') }}
        </Button>
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('sysmenu.edit') }}</Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('sysmenu.delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="720"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <Form />
      <div class="mt-2">
        <div class="mb-1 font-medium">{{ $t('sysmenu.grantLabel') }}</div>
        <GrantEditor
          v-model="grantRows"
          :default-mods="['roles', 'users']"
        />
      </div>
    </Modal>
  </Page>
</template>

