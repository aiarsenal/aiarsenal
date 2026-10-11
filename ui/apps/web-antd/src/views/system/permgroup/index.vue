<script lang="ts" setup>
/**
 * 权限分组管理（系统管理）：权限分组 / 员工集合 两个可复用授权字典。
 *
 * 页签切换用按钮组 + v-if 单实例挂载（grid-in-tabs 无先例且高度链不可控，
 * 这样布局与 post 页同构）；弹窗照 menu 页模式：vben form（name/remark）+
 * GrantEditor（分组词条，exclude-self 防直接自引）挂在 Form 下方。
 * 员工集合成员双入口：选人搜索 + 粘贴工号，合并为可删标签列表（工号去重）。
 */
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';

import type { VbenFormSchema } from '#/adapter/form';

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onErrorCaptured,
  reactive,
  ref,
  watch,
} from 'vue';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, RadioGroup, Select, Textarea } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  addEmpSetApi,
  addPermGroupApi,
  deleteEmpSetApi,
  deletePermGroupApi,
  editEmpSetApi,
  editPermGroupApi,
  getEmpSetListApi,
  getPermGroupListApi,
  type PermGroupApi,
  resolveEmpNosApi,
} from '#/api/system/permgroup';
import { searchUserOptionsApi } from '#/api/system/user';
import GrantEditor, { type GrantRowValue } from '#/components/GrantEditor.vue';
import { $t } from '#/locales';

defineOptions({ name: 'SystemPermGroup' });

// ===== 父级错误边界（排查弹窗内 GrantEditor 整体消失）=====
// GrantEditor 内部哨兵随组件一起被卸载、无法自救；错误边界放父级弹窗层，
// 子树任何渲染错误就地显示红条（不再无声白掉），组件树保住。
const grantFatal = ref('');
onErrorCaptured((err: unknown) => {
  grantFatal.value = `${String(err)}\n${String((err as any)?.stack ?? '').slice(0, 400)}`;
  return false;
});

// ===== 页签（按钮组切换，v-if 单实例挂载）=====
const activeTab = ref<'group' | 'set'>('group');
const tabOptions = computed(() => [
  { label: $t('syspermgroup.tabGroups'), value: 'group' },
  { label: $t('syspermgroup.tabSets'), value: 'set' },
]);

function parseJSONArr(s?: string): any[] {
  try {
    const v = JSON.parse(s ?? '[]');
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

// =========================================================
// 权限分组
// =========================================================
const groupSearchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('syspermgroup.groupName'),
    componentProps: { placeholder: $t('syspermgroup.groupPlaceholder') },
  },
]);

const groupGridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('syspermgroup.seq'), width: 60 },
    { field: 'name', title: $t('syspermgroup.groupName'), minWidth: 160 },
    {
      field: 'terms',
      title: $t('syspermgroup.terms'),
      width: 90,
      slots: { default: 'terms' },
    },
    { field: 'remark', title: $t('syspermgroup.remark'), minWidth: 180 },
    { field: 'createdByDesc', title: $t('syspermgroup.createdBy'), minWidth: 140 },
    { field: 'createdAt', title: $t('syspermgroup.createdAt'), minWidth: 160 },
    { field: 'updatedAt', title: $t('syspermgroup.updatedAt'), minWidth: 160 },
    {
      field: 'action',
      title: '',
      width: 160,
      fixed: 'right',
      slots: { default: 'groupAction' },
    },
  ],
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async ({ form }: any) => {
        const res = await getPermGroupListApi({ name: form?.name ?? '' });
        return { items: res?.list ?? [], total: 0 };
      },
    },
  },
}));

const [GroupGrid, groupGridApi] = useVbenVxeGrid({
  gridOptions: groupGridOptions,
  formOptions: { schema: groupSearchSchema },
});

// 分组弹窗：name/remark 走 vben form，词条走 GrantEditor
const groupIsEdit = ref(false);
const editingGroupId = ref('');
const groupModalVisible = ref(false);
const groupRows = ref<GrantRowValue[]>([]);
const groupModalTitle = computed(() =>
  groupIsEdit.value
    ? $t('syspermgroup.modalGroupTitleEdit')
    : $t('syspermgroup.modalGroupTitleAdd'),
);

const groupFormSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('syspermgroup.groupName'),
    rules: 'required',
    componentProps: { placeholder: $t('syspermgroup.groupName') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'remark',
    label: $t('syspermgroup.remark'),
    componentProps: {
      placeholder: $t('syspermgroup.remarkPlaceholder'),
      rows: 2,
    },
  },
]);

const [GroupForm, groupFormApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: groupFormSchema,
    showDefaultActions: false,
  }),
);

async function openGroupAdd() {
  groupIsEdit.value = false;
  editingGroupId.value = '';
  groupRows.value = [];
  groupModalVisible.value = true;
  await nextTick();
  await groupFormApi.resetForm();
}

async function openGroupEdit(row: any) {
  groupIsEdit.value = true;
  editingGroupId.value = row.groupId;
  groupRows.value = parseJSONArr(row.terms);
  groupModalVisible.value = true;
  await nextTick();
  await groupFormApi.resetForm();
  groupFormApi.setValues({ name: row.name, remark: row.remark ?? '' });
}

async function submitGroup() {
  const { valid } = await groupFormApi.validate();
  if (!valid) return;
  const values: any = await groupFormApi.getValues();
  const payload = {
    ...values,
    id: editingGroupId.value || undefined,
    terms: JSON.stringify(groupRows.value ?? []),
  };
  try {
    if (groupIsEdit.value) {
      await editPermGroupApi(payload);
      message.success($t('syspermgroup.editSuccess'));
    } else {
      await addPermGroupApi(payload);
      message.success($t('syspermgroup.addSuccess'));
    }
    groupModalVisible.value = false;
    groupGridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

function onDeleteGroupBatch() {
  const rows = groupGridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('syspermgroup.selectToDelete'));
    return;
  }
  confirmDelete(
    rows.map((r: any) => r.groupId),
    deletePermGroupApi,
    () => groupGridApi.query(),
  );
}

function onDeleteGroupRow(row: any) {
  confirmDelete([row.groupId], deletePermGroupApi, () => groupGridApi.query());
}

// =========================================================
// 员工集合
// =========================================================
const setSearchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('syspermgroup.setName'),
    componentProps: { placeholder: $t('syspermgroup.setPlaceholder') },
  },
]);

const setGridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('syspermgroup.seq'), width: 60 },
    { field: 'name', title: $t('syspermgroup.setName'), minWidth: 160 },
    {
      field: 'empNos',
      title: $t('syspermgroup.members'),
      width: 90,
      slots: { default: 'members' },
    },
    { field: 'remark', title: $t('syspermgroup.remark'), minWidth: 180 },
    { field: 'createdByDesc', title: $t('syspermgroup.createdBy'), minWidth: 140 },
    { field: 'createdAt', title: $t('syspermgroup.createdAt'), minWidth: 160 },
    { field: 'updatedAt', title: $t('syspermgroup.updatedAt'), minWidth: 160 },
    {
      field: 'action',
      title: '',
      width: 160,
      fixed: 'right',
      slots: { default: 'setAction' },
    },
  ],
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async ({ form }: any) => {
        const res = await getEmpSetListApi({ name: form?.name ?? '' });
        return { items: res?.list ?? [], total: 0 };
      },
    },
  },
}));

const [SetGrid, setGridApi] = useVbenVxeGrid({
  gridOptions: setGridOptions,
  formOptions: { schema: setSearchSchema },
});

// 集合弹窗：name/remark 走 vben form，成员双入口合并为工号标签列表
const setIsEdit = ref(false);
const editingSetId = ref('');
const setModalVisible = ref(false);
const setModalTitle = computed(() =>
  setIsEdit.value
    ? $t('syspermgroup.modalSetTitleEdit')
    : $t('syspermgroup.modalSetTitleAdd'),
);

const setFormSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input' as const,
    fieldName: 'name',
    label: $t('syspermgroup.setName'),
    rules: z
      .string()
      .min(1, $t('syspermgroup.nameRequired')),
    componentProps: { placeholder: $t('syspermgroup.setName') },
  },
  {
    component: 'Textarea' as const,
    fieldName: 'remark',
    label: $t('syspermgroup.remark'),
    componentProps: {
      placeholder: $t('syspermgroup.remarkPlaceholder'),
      rows: 2,
    },
  },
]);

const [SetForm, setFormApi] = useVbenForm(
  reactive({
    commonConfig: { fullWidth: true, hideLabel: false },
    schema: setFormSchema,
    showDefaultActions: false,
  }),
);

// 成员工号（去重标签列表）：选人 Select 直接绑定（× 可删），粘贴导入做并集
const setMembers = ref<string[]>([]);
const memberPaste = ref('');

// 成员明细：工号 → {name, deptName}（弹窗内确认查看用；防抖批量解析，未解析到=在册外）
const memberInfoMap = ref<Record<string, PermGroupApi.MemberInfo>>({});
const memberResolving = ref(false);
let memberResolveTimer: ReturnType<typeof setTimeout> | undefined;

async function resolveMembers(nos: string[]) {
  if (!nos.length) {
    memberInfoMap.value = {};
    return;
  }
  memberResolving.value = true;
  try {
    const res: any = await resolveEmpNosApi(nos);
    const m: Record<string, PermGroupApi.MemberInfo> = {};
    for (const it of res?.list ?? []) {
      m[it.employeeNo] = it;
    }
    memberInfoMap.value = m;
  } catch {
    /* 拦截器已提示；明细留空仅显示工号 */
  } finally {
    memberResolving.value = false;
  }
}

watch(
  setMembers,
  (nos) => {
    clearTimeout(memberResolveTimer);
    memberResolveTimer = setTimeout(() => resolveMembers(nos), 400);
  },
  { deep: true },
);
onBeforeUnmount(() => clearTimeout(memberResolveTimer));

function memberName(no: string): string {
  return memberInfoMap.value[no]?.name || $t('syspermgroup.memberUnmatched');
}
function memberDept(no: string): string {
  return memberInfoMap.value[no]?.deptName || '—';
}
function removeMember(no: string) {
  setMembers.value = setMembers.value.filter((m) => m !== no);
}

// 选人远程搜索（搜的是用户，落的是工号）
const memberUserOptions = ref<{ label: string; value: string }[]>([]);
const memberSearching = ref(false);
let memberSearchTimer: ReturnType<typeof setTimeout> | undefined;

function onMemberSearch(kw: string) {
  clearTimeout(memberSearchTimer);
  const q = (kw ?? '').trim();
  if (!q) {
    memberUserOptions.value = [];
    return;
  }
  memberSearchTimer = setTimeout(async () => {
    memberSearching.value = true;
    try {
      const res: any = await searchUserOptionsApi({
        keyWords: q,
        pageNum: 1,
        pageSize: 20,
      });
      memberUserOptions.value = (res?.list ?? [])
        .filter((u: any) => u.employeeNo)
        .map((u: any) => ({
          value: u.employeeNo,
          label: `${u.name} / ${u.employeeNo}${u.deptName ? ` / ${u.deptName}` : ''}`,
        }));
    } catch {
      /* 拦截器已提示 */
    } finally {
      memberSearching.value = false;
    }
  }, 300);
}

// 已选但不在搜索结果里的工号：经明细解析 map 拼三段标签（姓名/工号/部门全级），未解析到兜底工号
const memberOptionsWithSelected = computed(() => {
  const known = new Set(memberUserOptions.value.map((o) => o.value));
  const extra = setMembers.value
    .filter((no) => no && !known.has(no))
    .map((no) => {
      const m = memberInfoMap.value[no];
      return {
        value: no,
        label: m
          ? `${m.name} / ${no}${m.deptName ? ` / ${m.deptName}` : ''}`
          : no,
      };
    });
  return [...extra, ...memberUserOptions.value];
});

// 粘贴导入：逗号（全半角）/分号/空白分隔，去空去重，并入现有列表（保序）
function importPasted() {
  const pasted = (memberPaste.value ?? '')
    .split(/[\s,，;；]+/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (!pasted.length) return;
  const seen = new Set(setMembers.value);
  const added: string[] = [];
  for (const no of pasted) {
    if (!seen.has(no)) {
      seen.add(no);
      added.push(no);
    }
  }
  if (added.length) {
    setMembers.value = [...setMembers.value, ...added];
  }
  memberPaste.value = '';
}

async function openSetAdd() {
  setIsEdit.value = false;
  editingSetId.value = '';
  setMembers.value = [];
  memberPaste.value = '';
  setModalVisible.value = true;
  await nextTick();
  await setFormApi.resetForm();
}

async function openSetEdit(row: any) {
  setIsEdit.value = true;
  editingSetId.value = row.setId;
  setMembers.value = parseJSONArr(row.empNos).filter(
    (no) => typeof no === 'string',
  );
  memberPaste.value = '';
  setModalVisible.value = true;
  await nextTick();
  await setFormApi.resetForm();
  setFormApi.setValues({ name: row.name, remark: row.remark ?? '' });
}

async function submitSet() {
  const { valid } = await setFormApi.validate();
  if (!valid) return;
  const values: any = await setFormApi.getValues();
  const payload = {
    ...values,
    id: editingSetId.value || undefined,
    members: JSON.stringify(setMembers.value ?? []),
  };
  try {
    if (setIsEdit.value) {
      await editEmpSetApi(payload);
      message.success($t('syspermgroup.editSuccess'));
    } else {
      await addEmpSetApi(payload);
      message.success($t('syspermgroup.addSuccess'));
    }
    setModalVisible.value = false;
    setGridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

function onDeleteSetBatch() {
  const rows = setGridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('syspermgroup.selectToDelete'));
    return;
  }
  confirmDelete(
    rows.map((r: any) => r.setId),
    deleteEmpSetApi,
    () => setGridApi.query(),
  );
}

function onDeleteSetRow(row: any) {
  confirmDelete([row.setId], deleteEmpSetApi, () => setGridApi.query());
}

// ===== 删除确认（两字典共用）=====
function confirmDelete(
  ids: string[],
  api: (ids: string[]) => Promise<any>,
  refresh: () => void,
) {
  Modal.confirm({
    title: $t('syspermgroup.confirmDeleteTitle'),
    content: $t('syspermgroup.confirmDeleteContent', { count: ids.length }),
    okText: $t('syspermgroup.confirm'),
    cancelText: $t('syspermgroup.cancel'),
    okType: 'danger',
    onOk: async () => {
      await api(ids);
      message.success($t('syspermgroup.deleteSuccess'));
      refresh();
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="mb-3">
      <RadioGroup
        v-model:value="activeTab"
        option-type="button"
        button-style="solid"
        :options="tabOptions"
      />
    </div>

    <GroupGrid v-if="activeTab === 'group'">
      <template #toolbar-actions>
        <Button type="primary" @click="openGroupAdd">
          {{ $t('syspermgroup.addGroup') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteGroupBatch">
          {{ $t('syspermgroup.addBatchDelete') }}
        </Button>
      </template>
      <template #terms="{ row }">
        {{ parseJSONArr(row.terms).length }}
      </template>
      <template #groupAction="{ row }">
        <Button type="primary" ghost size="small" @click="openGroupEdit(row)">
          {{ $t('syspermgroup.edit') }}
        </Button>
        <Button
          type="primary"
          ghost
          size="small"
          danger
          @click="onDeleteGroupRow(row)"
        >
          {{ $t('syspermgroup.delete') }}
        </Button>
      </template>
    </GroupGrid>

    <SetGrid v-else>
      <template #toolbar-actions>
        <Button type="primary" @click="openSetAdd">
          {{ $t('syspermgroup.addSet') }}
        </Button>
        <Button danger class="ml-2" @click="onDeleteSetBatch">
          {{ $t('syspermgroup.addBatchDelete') }}
        </Button>
      </template>
      <template #members="{ row }">
        {{ parseJSONArr(row.empNos).length }}
      </template>
      <template #setAction="{ row }">
        <Button type="primary" ghost size="small" @click="openSetEdit(row)">
          {{ $t('syspermgroup.edit') }}
        </Button>
        <Button
          type="primary"
          ghost
          size="small"
          danger
          @click="onDeleteSetRow(row)"
        >
          {{ $t('syspermgroup.delete') }}
        </Button>
      </template>
    </SetGrid>

    <!-- 分组弹窗 -->
    <Modal
      v-model:open="groupModalVisible"
      :title="groupModalTitle"
      :width="720"
      :destroy-on-close="true"
      @ok="submitGroup"
    >
      <GroupForm />
      <div v-if="grantFatal" class="grant-fatal-host">
        {{ $t('syspermgroup.fatalPrefix') }}{{ grantFatal }}
      </div>
      <div class="mt-2">
        <div class="mb-1 font-medium">
          {{ $t('syspermgroup.termsLabel') }}
        </div>
        <GrantEditor
          v-model="groupRows"
          :exclude-self="editingGroupId"
          :default-mods="['roles', 'posts']"
        />
      </div>
    </Modal>

    <!-- 集合弹窗 -->
    <Modal
      v-model:open="setModalVisible"
      :title="setModalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="submitSet"
    >
      <SetForm />
      <div class="mt-2">
        <div class="mb-1 font-medium">
          {{ $t('syspermgroup.memberLabel') }}
        </div>
        <Select
          v-model:value="setMembers"
          class="member-select"
          mode="multiple"
          show-search
          :filter-option="false"
          :options="memberOptionsWithSelected"
          :loading="memberSearching"
          :placeholder="$t('syspermgroup.memberSearchPlaceholder')"
          style="width: 100%"
          allow-clear
          @search="onMemberSearch"
        />
        <div class="mt-2 flex gap-2">
          <Textarea
            v-model:value="memberPaste"
            :rows="2"
            :placeholder="$t('syspermgroup.memberPastePlaceholder')"
            style="flex: 1"
          />
          <Button @click="importPasted">
            {{ $t('syspermgroup.memberImport') }}
          </Button>
        </div>
        <div class="mt-1 text-xs opacity-70">
          {{ $t('syspermgroup.setMembersHint') }}
        </div>
        <!-- 成员明细：姓名/工号/部门 确认查看 + 行内移除（未解析到 = 在册外工号） -->
        <div v-if="setMembers.length" class="mt-3 border-t pt-2">
          <div class="mb-1 text-xs opacity-70">
            {{ $t('syspermgroup.memberInfoLabel') }}（{{ setMembers.length }}{{ memberResolving ? '…' : '' }}）
          </div>
          <div class="max-h-56 overflow-y-auto">
            <div
              v-for="no in setMembers"
              :key="no"
              class="flex items-center gap-2 border-b border-dashed py-1 text-xs last:border-b-0"
              :class="memberInfoMap[no] ? '' : 'opacity-80'"
            >
              <span
                class="flex-1 truncate"
                :class="memberInfoMap[no] ? '' : 'text-red-500'"
              >
                {{ memberName(no) }}
              </span>
              <span style="flex: 0 0 120px" class="opacity-80">{{ no }}</span>
              <span class="flex-1 truncate opacity-80">
                {{ memberDept(no) }}
              </span>
              <Button
                type="link"
                danger
                size="small"
                @click="removeMember(no)"
              >
                {{ $t('syspermgroup.memberRemove') }}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Modal>
  </Page>
</template>

<style scoped>
/* 父级错误边界红条：子树渲染错误就地显示（destructive 红调，浅深主题均可读） */
.grant-fatal-host {
  margin-top: 8px;
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
/* 成员标签完整展示：不折叠（无 maxTagCount）、不省略——长标签（含部门全级路径）在标签内换行 */
.member-select :deep(.ant-select-selection-item) {
  max-width: 100%;
  height: auto;
  min-height: 24px;
}
.member-select :deep(.ant-select-selection-item-content) {
  white-space: normal;
  word-break: break-all;
}
</style>
