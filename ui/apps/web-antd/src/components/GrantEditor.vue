<script lang="ts" setup>
/**
 * 组合授权条件卡编辑器（菜单可见条件 / 文档·目录读写条件 / 权限分组词条共用的通用模块）。
 *
 * 数据形态与后端 model.GrantRow 一一对应：
 *   { userIds[], deptId, roleIds[], postIds[], exclude, groupId, setId }
 * 一张条件卡内六个模块（角色/岗位/用户/部门/权限分组/员工集合）任意组合，非空模块全部 AND
 * （如「员工集合 + 角色」一卡 = 集合成员且持该角色）；卡与卡之间 OR；
 * exclude=true 为排除卡（整卡条件取反，黑名单语义）。留空的模块 = 不限。
 *
 * 通用化配置（消费方按场景定制）：
 *   mod-order     模块勾选框与卡内的展示顺序（key 数组，缺省=内置顺序）
 *   default-mods  新卡默认勾选展示的模块（回显卡按已有值自动勾选）
 * 卡内复选框勾选的模块才展示输入；未勾选=不参与本卡条件（取消勾选即清空该模块已填值）。
 *
 * 父组件用法：v-model 绑 GrantRow[]，提交时 JSON.stringify（grantRules/terms 字段）；
 * 回显时 JSON.parse 后直接赋值（旧数据无 groupId/setId = 纯条件卡，向后兼容）。
 * 不进 vben form schema（自拉数据、自成一块）。
 * 权限分组编辑自身词条时传 exclude-self=自身 groupId，选择器隐藏自身防直接自引。
 */
import type { Recordable } from '@vben/types';

import {
  computed,
  onBeforeUnmount,
  onErrorCaptured,
  onMounted,
  ref,
  watch,
} from 'vue';

import {
  Button,
  Checkbox,
  message,
  Select,
  Tooltip,
  TreeSelect,
} from 'ant-design-vue';

import { getDeptTreeApi } from '#/api/system/dept';
import { getPermGroupOptionsApi } from '#/api/system/permgroup';
import {
  getUserParamsApi,
  resolveUserOptionsApi,
  searchUserOptionsApi,
  type UserOptionItem,
} from '#/api/system/user';
import { $t } from '#/locales';

export interface GrantRowValue {
  deptId: string;
  exclude: boolean;
  groupId: string;
  postIds: string[];
  roleIds: string[];
  setId: string;
  userIds: string[];
}

const props = defineProps<{
  /** 新卡默认勾选（展示）的模块；回显卡按已有值自动勾选 */
  defaultMods?: string[];
  /** 编辑某分组自身词条时传其 groupId：分组选择器隐藏自身（防直接自引；深环后端 DFS 兜底） */
  excludeSelf?: string;
  /** 模块展示顺序（mod key 数组）；缺省用内置顺序 */
  modOrder?: string[];
}>();

const rows = defineModel<GrantRowValue[]>({ required: true });

// ===== rows 历史哨兵（排查「行消失」）：记录每次变化的长度轨迹 =====
// 行消失时看这行小字即可区分：数据被清（轨迹回退）还是被挤出视口（轨迹正常）
const rowsTrace = ref('');
watch(
  rows,
  (rs) => {
    const n = Array.isArray(rs) ? rs.length : 'x';
    rowsTrace.value = `${rowsTrace.value}→${n}`.split('→').slice(-12).join('→');
  },
  { deep: false, immediate: true },
);

// ===== 模块定义：key / 标题词条 / 用法注释词条 / 色点（卡片配色由 c-<key> 类承担）=====
interface ModDef {
  color: string;
  key: 'dept' | 'group' | 'posts' | 'roles' | 'set' | 'users';
  labelKey: string;
  tipKey: string;
}
const MOD_DEFS: ModDef[] = [
  {
    key: 'roles',
    labelKey: 'sysgrant.rolesLabel',
    tipKey: 'sysgrant.rolesTip',
    color: '#6366f1',
  },
  {
    key: 'posts',
    labelKey: 'sysgrant.postsLabel',
    tipKey: 'sysgrant.postsTip',
    color: '#06b6d4',
  },
  {
    key: 'users',
    labelKey: 'sysgrant.usersLabel',
    tipKey: 'sysgrant.usersTip',
    color: '#3b82f6',
  },
  {
    key: 'dept',
    labelKey: 'sysgrant.deptLabel',
    tipKey: 'sysgrant.deptTip',
    color: '#10b981',
  },
  {
    key: 'group',
    labelKey: 'sysgrant.groupLabel',
    tipKey: 'sysgrant.groupTip',
    color: '#f59e0b',
  },
  {
    key: 'set',
    labelKey: 'sysgrant.setLabel',
    tipKey: 'sysgrant.setTip',
    color: '#8b5cf6',
  },
];
/** 展示顺序：消费方 mod-order 定制，缺省内置顺序；未知 key 忽略 */
const modList = computed<ModDef[]>(() => {
  const order = props.modOrder?.length
    ? props.modOrder
    : MOD_DEFS.map((m) => m.key);
  return order
    .map((k) => MOD_DEFS.find((m) => m.key === k))
    .filter((m): m is ModDef => !!m);
});

// ===== 模块勾选：勾选的才展示输入；未勾选=不参与本卡条件（取消勾选清空已填值）=====
const modOn = ref<Record<string, boolean>>({});
function modHasValue(row: GrantRowValue, mod: string): boolean {
  switch (mod) {
    case 'dept': {
      return !!row.deptId;
    }
    case 'group': {
      return !!row.groupId;
    }
    case 'posts': {
      return row.postIds.length > 0;
    }
    case 'roles': {
      return row.roleIds.length > 0;
    }
    case 'set': {
      return !!row.setId;
    }
    case 'users': {
      return row.userIds.length > 0;
    }
  }
  return false;
}
/** 模块是否参与本卡：用户勾选记录优先；否则回显卡看已有值、新卡看 default-mods */
function modActive(i: number, mod: string, row: GrantRowValue): boolean {
  const k = `${i}:${mod}`;
  if (k in modOn.value) return modOn.value[k] ?? false;
  return modHasValue(row, mod) || (props.defaultMods ?? []).includes(mod);
}
function clearMod(row: GrantRowValue, mod: string) {
  if (mod === 'dept') row.deptId = '';
  else if (mod === 'group') row.groupId = '';
  else if (mod === 'posts') row.postIds = [];
  else if (mod === 'roles') row.roleIds = [];
  else if (mod === 'set') row.setId = '';
  else if (mod === 'users') row.userIds = [];
}
function toggleMod(i: number, mod: string, row: GrantRowValue) {
  const k = `${i}:${mod}`;
  const next = !modActive(i, mod, row);
  modOn.value[k] = next;
  if (next) {
    openCells.value[k] = true; // 勾选即展开，引导输入
  } else {
    // 未勾选=不参与本卡条件：清空已填值，杜绝「隐藏的值被提交」
    clearMod(row, mod);
    rows.value = [...(rows.value ?? [])]; // 同步 emit 链
  }
}
function activeMods(i: number, row: GrantRowValue): ModDef[] {
  return modList.value.filter((m) => modActive(i, m.key, row));
}

// ===== 选项数据：模块级缓存（同页多处编辑器只拉一次）=====
interface GrantMeta {
  deptTree: { children?: any[]; label: string; value: string }[];
  groups: { label: string; value: string }[];
  posts: { label: string; value: string }[];
  roles: { label: string; value: string }[];
  sets: { label: string; value: string }[];
}
let metaCache: GrantMeta | null = null;
let metaPromise: null | Promise<GrantMeta> = null;

// 分组/集合字典改动即时生效，选项不能长期缓存：短 TTL（与后端 grantmeta 快照同量级），
// 过期后下次挂载重拉；部门/角色/岗位是稳定字典，维持原有常驻缓存。
const REF_OPTIONS_TTL = 5000;
let refOptionsAt = 0;

function ensureMeta(force = false): Promise<GrantMeta> {
  const refStale = !metaCache || Date.now() - refOptionsAt > REF_OPTIONS_TTL;
  if (force || refStale || !metaPromise) {
    metaPromise = Promise.all([
      getUserParamsApi(),
      getDeptTreeApi(),
      getPermGroupOptionsApi(),
    ])
      .then(([up, dt, opts]) => {
        const mapTree = (nodes: any[]): any[] =>
          (nodes ?? []).map((n) => ({
            value: n.deptId,
            label: n.deptName,
            children: n.children?.length ? mapTree(n.children) : undefined,
          }));
        refOptionsAt = Date.now();
        metaCache = {
          roles: (up?.roleList ?? []).map((r) => ({
            value: r.id,
            label: r.name,
          })),
          posts: (up?.posts ?? []).map((p) => ({
            value: p.postId,
            label: p.postName,
          })),
          deptTree: mapTree((dt as any)?.deps ?? []),
          groups: (opts?.groups ?? []).map((g) => ({
            value: g.id,
            label: g.name,
          })),
          sets: (opts?.sets ?? []).map((s: any) => ({
            value: s.id,
            label: s.name,
          })),
        };
        return metaCache;
      })
      .catch(() => {
        metaPromise = null; // 失败允许下次重试
        return {
          roles: [],
          posts: [],
          deptTree: [],
          groups: [],
          sets: [],
        };
      });
  }
  return metaPromise;
}

const meta = ref<GrantMeta>({
  roles: [],
  posts: [],
  deptTree: [],
  groups: [],
  sets: [],
});
onMounted(async () => {
  meta.value = await ensureMeta();
});

// 编辑中的分组自身不出现在分组选择器里（excludeSelf）
const groupOptions = computed(() =>
  (meta.value.groups ?? []).filter((g) => g.value !== props.excludeSelf),
);

// ===== 用户远程搜索 + 已选用户解析回显 =====
// userMap：id → 用户信息。搜索结果顺手入 map；卡里已选但从未搜过的 id（编辑回显）
// 经 /user/resolve 批量解析补齐，保证标签始终显示「姓名 / 工号」而非裸 id。
const userOptions = ref<Recordable<string>[]>([]);
const userSearching = ref(false);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
const userMap = ref<Record<string, UserOptionItem>>({});

// 标签与下拉同显三段「姓名 / 工号 / 部门（全级）」；配合不折叠+可换行样式完整展示
function userLabel(u: UserOptionItem): string {
  let s = u.name || u.id;
  if (u.employeeNo) s += ` / ${u.employeeNo}`;
  if (u.deptName) s += ` / ${u.deptName}`;
  return s;
}

function onUserSearch(kw: string) {
  clearTimeout(searchTimer);
  const q = (kw ?? '').trim();
  if (!q) {
    userOptions.value = [];
    return;
  }
  searchTimer = setTimeout(async () => {
    userSearching.value = true;
    try {
      const res: any = await searchUserOptionsApi({
        keyWords: q,
        pageNum: 1,
        pageSize: 20,
      });
      const list: UserOptionItem[] = res?.list ?? [];
      for (const u of list) {
        userMap.value[u.id] = u;
      }
      userOptions.value = list.map((u) => ({
        value: u.id,
        label: userLabel(u),
      }));
    } catch {
      /* 拦截器已提示 */
    } finally {
      userSearching.value = false;
    }
  }, 300);
}
onBeforeUnmount(() => clearTimeout(searchTimer));

// 已选用户回显解析：rows 里 map 未覆盖的 id 防抖批量拉取（含弹窗打开时的存量卡）
let userResolveTimer: ReturnType<typeof setTimeout> | undefined;
watch(
  rows,
  (rs) => {
    const missing: string[] = [];
    const seen = new Set<string>();
    for (const r of rs ?? []) {
      for (const id of r.userIds ?? []) {
        if (id && !userMap.value[id] && !seen.has(id)) {
          seen.add(id);
          missing.push(id);
        }
      }
    }
    if (!missing.length) return;
    clearTimeout(userResolveTimer);
    userResolveTimer = setTimeout(async () => {
      try {
        const res: any = await resolveUserOptionsApi(missing);
        for (const u of res?.list ?? []) {
          userMap.value[u.id] = u;
        }
      } catch {
        /* 拦截器已提示；回显退化为裸 id */
      }
    }, 200);
  },
  { deep: true, immediate: true },
);
onBeforeUnmount(() => clearTimeout(userResolveTimer));

const userOptionsWithSelected = computed(() => {
  const known = new Set(userOptions.value.map((o) => o.value));
  const extra: { label: string; value: string }[] = [];
  for (const r of rows.value ?? []) {
    for (const id of r.userIds ?? []) {
      if (id && !known.has(id)) {
        known.add(id);
        const u = userMap.value[id];
        extra.push({ value: id, label: u ? userLabel(u) : id });
      }
    }
  }
  return [...extra, ...userOptions.value];
});

// ===== 渲染黑匣子：线上「行消失」问题本地无法复现，先捕错误就地显示 =====
// onErrorCaptured 捕获后代组件（Select/TreeSelect 等）的渲染错误，阻止向上炸掉整个弹窗；
// 错误内容直接显示在编辑器顶部，用户可转述定位。
const fatalErr = ref('');
onErrorCaptured((err: unknown) => {
  fatalErr.value = `${String(err)}\n${String((err as any)?.stack ?? '').slice(0, 300)}`;
  return false;
});

// ===== 卡操作 =====
// push（mutate 父数组，引用不变）+ 整体替换（走 defineModel emit 链）双保险：
// 无论 emit/props 回流在哪个中间态，父组件的数组里卡都不会丢。
function newRowValue(): GrantRowValue {
  return {
    userIds: [],
    deptId: '',
    roleIds: [],
    postIds: [],
    exclude: false,
    groupId: '',
    setId: '',
  };
}
// 卡区滚动容器：添加卡后滚到底，保证新卡立即可见（不被折叠在滚动区外）
const rowsBox = ref<HTMLElement | null>(null);
function addRow() {
  const cur = rows.value ?? (rows.value = []);
  cur.push(newRowValue());
  rows.value = [...cur];
  requestAnimationFrame(() => {
    const el = rowsBox.value;
    if (el) el.scrollTop = el.scrollHeight;
  });
}
function removeRow(i: number) {
  const cur = rows.value ?? [];
  cur.splice(i, 1);
  rows.value = [...cur];
  // 明确反馈：卡被删除时给出提示（防止误删被当成「卡自己消失」）
  message.success($t('sysgrant.rowRemoved', { n: i + 1 }));
}

// ===== 卡内模块折叠（照 client/device 运行详情 module-card：标题行可点、默认收起）=====
// 折叠态下标题右侧显示已选摘要（「不限」/已选数/所选项名），不展开也能看清卡配置。
const openCells = ref<Record<string, boolean>>({});
function cellOpen(i: number, mod: string): boolean {
  return !!openCells.value[`${i}:${mod}`];
}
function toggleCell(i: number, mod: string) {
  const k = `${i}:${mod}`;
  openCells.value[k] = !openCells.value[k];
}
function findDeptLabel(nodes: any[], id: string): string {
  for (const n of nodes ?? []) {
    if (n.value === id) return n.label;
    const hit = findDeptLabel(n.children ?? [], id);
    if (hit) return hit;
  }
  return '';
}
function cellSummary(row: GrantRowValue, mod: string): string {
  switch (mod) {
    case 'dept': {
      return row.deptId
        ? (findDeptLabel(meta.value.deptTree, row.deptId) || row.deptId)
        : '';
    }
    case 'group': {
      return row.groupId
        ? (groupOptions.value.find((g) => g.value === row.groupId)?.label ??
          row.groupId)
        : '';
    }
    case 'posts': {
      return row.postIds.length
        ? $t('sysgrant.nItems', { n: row.postIds.length })
        : '';
    }
    case 'roles': {
      return row.roleIds.length
        ? $t('sysgrant.nItems', { n: row.roleIds.length })
        : '';
    }
    case 'set': {
      return row.setId
        ? (meta.value.sets.find((s) => s.value === row.setId)?.label ??
          row.setId)
        : '';
    }
    case 'users': {
      return row.userIds.length
        ? $t('sysgrant.nItems', { n: row.userIds.length })
        : '';
    }
  }
  return '';
}
</script>

<template>
  <div>
    <!-- 渲染黑匣子：出错就地显示（含数据异常），不再无声消失 -->
    <div v-if="fatalErr || !Array.isArray(rows)" class="grant-fatal">
      {{ $t('sysgrant.fatalPrefix') }}{{ fatalErr || 'rows is not an array' }}
    </div>
    <!-- 添加按钮固定在卡区上方：位置永不随卡数变化，连点永远点同一处，
         杜绝「按钮被新卡顶走后原地二连击误触卡内删除」 -->
    <Button block dashed size="small" class="mb-2" @click="addRow">
      + {{ $t('sysgrant.addRow') }}
    </Button>
    <!-- 卡区滚动容器：多卡时在容器内滚动，弹窗高度恒定，卡不会被撑出视口 -->
    <div ref="rowsBox" class="grant-rows">
      <template v-for="(row, i) in rows ?? []" :key="i">
        <div
          class="grant-row"
          :class="{ even: i % 2 === 1, exclude: row.exclude }"
        >
      <!-- 卡头：序号 + 排除 + 删除（卡间界限由彩色卡片边框 + || 连接符承担） -->
      <div class="mb-1.5 flex items-center gap-2">
        <span class="grant-row-no">{{ i + 1 }}</span>
        <Tooltip :title="$t('sysgrant.excludeTip')">
          <Checkbox v-model:checked="row.exclude">
            {{ $t('sysgrant.exclude') }}
          </Checkbox>
        </Tooltip>
        <div class="flex-1"></div>
        <Button danger ghost size="small" @click="removeRow(i)">
          {{ $t('sysgrant.removeRow') }}
        </Button>
      </div>
      <!-- 参与模块复选框：勾选的模块才展示输入，未勾选=不参与本卡条件；
           顺序与展示顺序一致（mod-order 可定制） -->
      <div class="grant-mods" :title="$t('sysgrant.modsTip')">
        <span class="grant-mods-label">{{ $t('sysgrant.modsLabel') }}</span>
        <Checkbox
          v-for="m in modList"
          :key="m.key"
          class="grant-mod-ck"
          :checked="modActive(i, m.key, row)"
          @change="toggleMod(i, m.key, row)"
        >
          <span class="grant-mod-name">
            <span
              class="grant-mod-dot"
              :style="{ background: m.color }"
            ></span>
            {{ $t(m.labelKey) }}
          </span>
        </Checkbox>
      </div>
      <!-- 已勾选模块卡片（照 client/device 运行详情 module-card 形式：彩色标题行
           默认折叠、标题后带用法注释、内容区浅底；模块间「且」连接符表达卡内相与） -->
      <div class="flex flex-col gap-1">
        <div v-if="!activeMods(i, row).length" class="grant-mods-empty">
          {{ $t('sysgrant.modsEmpty') }}
        </div>
        <template v-for="(m, mi) in activeMods(i, row)" :key="m.key">
          <div class="grant-cell" :class="`c-${m.key}`">
            <div class="grant-cell-head" @click="toggleCell(i, m.key)">
              <span class="grant-toggle">
                {{ cellOpen(i, m.key) ? '▾' : '▸' }}
              </span>
              <span class="grant-cell-name">{{ $t(m.labelKey) }}</span>
              <span class="grant-cell-note">{{ $t(m.tipKey) }}</span>
              <span
                class="grant-cell-sum"
                :class="{ any: !modHasValue(row, m.key) }"
              >{{ cellSummary(row, m.key) }}</span>
            </div>
            <div v-show="cellOpen(i, m.key)" class="grant-cell-body">
              <Select
                v-if="m.key === 'roles'"
                v-model:value="row.roleIds"
                class="w-full"
                mode="multiple"
                :options="meta.roles"
                max-tag-count="responsive"
                :placeholder="$t('sysgrant.rolesPlaceholder')"
                allow-clear
              />
              <Select
                v-else-if="m.key === 'posts'"
                v-model:value="row.postIds"
                class="w-full"
                mode="multiple"
                :options="meta.posts"
                max-tag-count="responsive"
                :placeholder="$t('sysgrant.postsPlaceholder')"
                allow-clear
              />
              <Select
                v-else-if="m.key === 'users'"
                v-model:value="row.userIds"
                class="grant-users w-full"
                mode="multiple"
                show-search
                :filter-option="false"
                :options="userOptionsWithSelected"
                :loading="userSearching"
                :placeholder="$t('sysgrant.usersPlaceholder')"
                allow-clear
                @search="onUserSearch"
              />
              <TreeSelect
                v-else-if="m.key === 'dept'"
                v-model:value="row.deptId"
                class="w-full"
                :tree-data="meta.deptTree"
                :placeholder="$t('sysgrant.deptPlaceholder')"
                tree-default-expand-all
                allow-clear
              />
              <Select
                v-else-if="m.key === 'group'"
                v-model:value="row.groupId"
                class="w-full"
                show-search
                option-filter-prop="label"
                :options="groupOptions"
                :placeholder="$t('sysgrant.groupPlaceholder')"
                :not-found-content="$t('sysgrant.groupNotFound')"
                allow-clear
              />
              <Select
                v-else-if="m.key === 'set'"
                v-model:value="row.setId"
                class="w-full"
                show-search
                option-filter-prop="label"
                :options="meta.sets"
                :placeholder="$t('sysgrant.setPlaceholder')"
                :not-found-content="$t('sysgrant.setNotFound')"
                allow-clear
              />
            </div>
          </div>
          <!-- 模块间「且」连接符（非末个已勾选模块） -->
          <div
            v-if="mi < activeMods(i, row).length - 1"
            class="grant-and"
            :title="$t('sysgrant.andTip')"
          >
            <span class="grant-and-badge">{{ $t('sysgrant.and') }}</span>
          </div>
        </template>
      </div>
      </div>
      <!-- 卡间「或」连接符：独立置于卡与卡之间（与卡内模块间「且」连接符同构）；
           多卡之间任一命中即生效 -->
      <div
        v-if="i < (rows ?? []).length - 1"
        class="grant-or"
        :title="$t('sysgrant.orTip')"
      >
        <span class="grant-or-badge">{{ $t('sysgrant.or') }}</span>
      </div>
      </template>
    </div>

    <div class="mt-1 text-xs opacity-70">
      {{ (rows ?? []).length === 0 ? $t('sysgrant.empty') : $t('sysgrant.hint') }}
    </div>
    <!-- rows 历史哨兵（排查期临时）：行消失时据此区分数据被清/视口挤出 -->
    <div class="grant-trace">rows:{{ rowsTrace || '0' }}</div>
  </div>
</template>

<style scoped>
/* 渲染黑匣子错误条：destructive 红调，浅深主题均可读 */
.grant-fatal {
  margin-bottom: 8px;
  padding: 6px 10px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  color: hsl(var(--destructive));
  border: 1px solid
    color-mix(in srgb, hsl(var(--destructive)) 45%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, hsl(var(--destructive)) 6%, transparent);
}
/* 卡区滚动容器：弹窗高度恒定（多卡不再把 Modal 撑出视口） */
.grant-rows {
  max-height: 46vh;
  overflow-y: auto;
  padding-right: 2px;
}
/* rows 历史哨兵（排查期临时小字）：muted 前景，浅深主题自适应 */
.grant-trace {
  margin-top: 4px;
  font-size: 11px;
  color: hsl(var(--muted-foreground));
  opacity: 0.65;
}
/* 条件卡：主色调边框（color-mix 浅深主题自适应），卡与卡界限一目了然 */
.grant-row {
  padding: 8px 10px 10px;
  margin-bottom: 2px;
  border: 1px solid
    color-mix(in srgb, hsl(var(--primary)) 35%, hsl(var(--border)));
  border-radius: 8px;
}
/* 卡间明暗交替（同 device 运行详情内容区白/浅绿交替，亮暗主题各一套成对）；
   按行序号绑 even 类而非 nth-child——卡间「或」连接符是同级兄弟节点，会打乱 nth 计数 */
.grant-row {
  background: #ffffff;
}
html.dark .grant-row {
  background: #1f1f1f;
}
.grant-row.even {
  background: #eaf8f1;
}
html.dark .grant-row.even {
  background: #1d2e26;
}
/* 排除卡：destructive 红调边框 + 淡红底（压过明暗交替，黑名单语义一眼可辨） */
.grant-row.exclude {
  border-color: color-mix(
    in srgb,
    hsl(var(--destructive)) 45%,
    hsl(var(--border))
  );
  background: color-mix(in srgb, hsl(var(--destructive)) 6%, var(--aa-bg, #fff));
}
.grant-row-no {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 4px;
  font-size: 12px;
  background: hsl(var(--muted));
  color: hsl(var(--muted-foreground));
  border-radius: 4px;
}
/* 参与模块复选框条：虚线框弱化层级，勾选即出现对应模块卡片 */
.grant-mods {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 12px;
  align-items: center;
  margin-bottom: 8px;
  padding: 4px 8px;
  border: 1px dashed hsl(var(--border));
  border-radius: 6px;
}
.grant-mods-label {
  font-size: 12px;
  color: hsl(var(--muted-foreground));
}
.grant-mod-ck :deep(.ant-checkbox + span) {
  padding-inline-start: 4px;
  font-size: 12px;
}
.grant-mod-name {
  display: inline-flex;
  gap: 4px;
  align-items: center;
}
/* 模块色点：与模块卡片主色一一对应，勾选框→卡片可对照 */
.grant-mod-dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
/* 一张卡未勾选任何模块的空态提示 */
.grant-mods-empty {
  padding: 6px 8px;
  font-size: 12px;
  color: hsl(var(--muted-foreground));
  border: 1px dashed hsl(var(--border));
  border-radius: 6px;
  opacity: 0.8;
}
/* 卡间「或」连接符（独立于卡与卡之间，非末卡后）：两侧细线 + || 徽标 */
.grant-or {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 2px 0 10px;
}
.grant-or::before,
.grant-or::after {
  content: '';
  flex: 1;
  height: 1px;
  background: hsl(var(--border));
}
.grant-or-badge {
  padding: 0 10px;
  font-size: 12px;
  font-weight: 700;
  line-height: 20px;
  color: hsl(var(--primary));
  background: color-mix(in srgb, hsl(var(--primary)) 10%, transparent);
  border-radius: 10px;
}
/* 卡内模块卡片（照 client/device 运行详情 module-card 形式）：
   30% 半透主色边框 + 8% 渐变填充；标题行 = 主色 40% 混底色的深色标题、默认折叠；
   内容区浅底（--aa-bg 亮#fff/暗#111827）。 */
.grant-cell {
  --c: #6366f1;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--c) 30%, transparent);
  border-radius: 6px;
  background-image: linear-gradient(
    160deg,
    color-mix(in srgb, var(--c) 8%, transparent),
    transparent 60%
  );
}
.grant-cell.c-roles {
  --c: #6366f1;
}
.grant-cell.c-posts {
  --c: #06b6d4;
}
.grant-cell.c-users {
  --c: #3b82f6;
}
.grant-cell.c-dept {
  --c: #10b981;
}
.grant-cell.c-group {
  --c: #f59e0b;
}
.grant-cell.c-set {
  --c: #8b5cf6;
}
/* 标题行：整行可点折叠；深色标题底 = 模块主色 40% 混 --aa-bg（亮暗自适应） */
.grant-cell-head {
  display: flex;
  gap: 6px;
  align-items: center;
  min-height: 30px;
  padding: 0 8px;
  cursor: pointer;
  user-select: none;
  background: color-mix(in srgb, var(--c) 40%, var(--aa-bg, #fff));
}
.grant-toggle {
  width: 18px;
  font-size: 13px;
  line-height: 1;
  text-align: center;
}
/* 标题字色：浅色主题主色加深版（62% 主色 + 黑），暗色主题主色提亮版（72% 主色 + 白） */
.grant-cell-name {
  font-size: 12px;
  font-weight: 600;
  color: color-mix(in srgb, var(--c) 62%, #000);
}
html.dark .grant-cell-name {
  color: color-mix(in srgb, var(--c) 72%, #fff);
}
/* 标题后用法注释：小字弱化，主色混 muted 自适应，长注释省略号截断 */
.grant-cell-note {
  flex: 1;
  overflow: hidden;
  min-width: 0;
  font-size: 11px;
  font-weight: 400;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: color-mix(in srgb, var(--c) 50%, hsl(var(--muted-foreground)));
}
/* 收起时标题右侧摘要：已选（主色同标题）/ 未选「不限」（muted 弱化） */
.grant-cell-sum {
  overflow: hidden;
  margin-left: auto;
  max-width: 45%;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: color-mix(in srgb, var(--c) 62%, #000);
}
html.dark .grant-cell-sum {
  color: color-mix(in srgb, var(--c) 72%, #fff);
}
.grant-cell-sum.any {
  font-weight: 400;
  color: hsl(var(--muted-foreground));
  opacity: 0.75;
}
/* 内容区：浅色内容（--aa-bg），顶部主色细分界线与深色标题分隔 */
.grant-cell-body {
  padding: 8px;
  border-top: 1px solid color-mix(in srgb, var(--c) 25%, transparent);
  background: var(--aa-bg, #fff);
}
/* 卡内「且」连接符：模块之间相与（与卡间「或」连接符同构成对出现） */
.grant-and {
  display: flex;
  gap: 8px;
  align-items: center;
}
.grant-and::before,
.grant-and::after {
  content: '';
  flex: 1;
  height: 1px;
  background: hsl(var(--border));
}
.grant-and-badge {
  padding: 0 10px;
  font-size: 12px;
  font-weight: 700;
  line-height: 18px;
  color: hsl(var(--primary));
  background: color-mix(in srgb, hsl(var(--primary)) 10%, transparent);
  border-radius: 9px;
}
/* 指定用户标签完整展示：不折叠（无 maxTagCount）、不省略——长标签（含部门全级路径）
   在标签内换行，任何字符不隐藏 */
.grant-users :deep(.ant-select-selection-item) {
  max-width: 100%;
  height: auto;
  min-height: 24px;
}
.grant-users :deep(.ant-select-selection-item-content) {
  white-space: normal;
  word-break: break-all;
}
</style>
