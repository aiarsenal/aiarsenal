<script lang="ts" setup>
import type { VxeTableGridOptions } from '@vben/plugins/vxe-table';
import type { VbenFormSchema } from '#/adapter/form';

import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Collapse,
  CollapsePanel,
  Descriptions,
  DescriptionsItem,
  Drawer,
  Input,
  message,
  Modal,
  Switch,
  Table,
  Textarea,
  Tooltip,
  Upload,
} from 'ant-design-vue';

import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import {
  addSkillApi,
  addSkillVersionApi,
  createSkillPackageApi,
  deleteSkillApi,
  downloadSkillFileApi,
  downloadSkillVersionApi,
  editSkillApi,
  getAllSkillNamespacesApi,
  getSkillFilesApi,
  getSkillListApi,
  getSkillVersionsApi,
  parseSkillPackageApi,
  publishSkillVersionApi,
  setSkillHiddenApi,
  yankSkillVersionApi,
} from '#/api/system/skill/skill';

defineOptions({ name: 'SystemSkillSkill' });

// ===== 命名空间下拉 =====
const namespaceOptions = ref<{ value: string; label: string }[]>([]);

async function loadNamespaces() {
  if (namespaceOptions.value.length) return;
  try {
    const res: any = await getAllSkillNamespacesApi();
    namespaceOptions.value = (res?.list ?? []).map((n: any) => ({
      value: n.namespaceId,
      label: n.displayName,
    }));
  } catch {
    /* 忽略，拦截器已提示 */
  }
}
onMounted(loadNamespaces);

// ===== 搜索表单（computed：随语言切换实时变更 label/placeholder） =====
const searchSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Select',
    fieldName: 'namespaceId',
    label: $t('skill.skill_namespace'),
    componentProps: () => ({
      allowClear: true,
      options: namespaceOptions.value,
      placeholder: $t('skill.skill_namespacePlaceholder'),
      style: { width: '100%' },
    }),
  },
  {
    component: 'Input',
    fieldName: 'slug',
    label: $t('skill.skill_slug'),
    componentProps: { placeholder: $t('skill.skill_slugPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'name',
    label: $t('skill.skill_name'),
    componentProps: { placeholder: $t('skill.skill_namePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: $t('skill.skill_status'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('skill.skill_enabled'), value: 1 },
        { label: $t('skill.skill_disabled'), value: 0 },
      ],
    },
  },
]);

// ===== 表格（columns 用 computed，title 随语言切换） =====
const gridOptions = computed<VxeTableGridOptions>(() => ({
  columns: [
    { type: 'checkbox', width: 50, fixed: 'left' },
    { type: 'seq', title: $t('skill.skill_seq'), width: 60 },
    { field: 'slug', title: $t('skill.skill_slug'), minWidth: 140 },
    { field: 'displayName', title: $t('skill.skill_name'), minWidth: 140 },
    { field: 'summary', title: $t('skill.skill_summary'), minWidth: 200 },
    { field: 'ownerName', title: $t('skill.skill_owner'), width: 150, slots: { default: 'owner' } },
    { field: 'downloadCount', title: $t('skill.skill_download'), width: 90 },
    { field: 'starCount', title: '★', width: 70 },
    {
      field: 'status',
      title: $t('skill.skill_status'),
      width: 90,
      slots: { default: 'status' },
    },
    {
      field: 'hidden',
      title: $t('skill.skill_hidden'),
      width: 90,
      slots: { default: 'hidden' },
    },
    {
      field: 'source',
      title: $t('skill.skill_source'),
      width: 120,
      slots: { default: 'source' },
    },
    {
      field: 'action',
      title: $t('skill.skill_action'),
      width: 320,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ],
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 30] },
  proxyConfig: {
    ajax: {
      query: async ({ page, form }: any) => {
        const res = await getSkillListApi({
          pageNum: page.currentPage,
          pageSize: page.pageSize || 20,
          ...(form as any),
        });
        // gin: SkillSearchRes → { list, total }
        return { items: res?.list ?? [], total: res?.total ?? 0 };
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
  formOptions: { schema: searchSchema },
});

// ===== 新增/编辑弹窗 =====
const isEdit = ref(false);
const editingId = ref('');
const modalVisible = ref(false);
const modalTitle = computed(() =>
  isEdit.value ? $t('skill.skill_modalTitleEdit') : $t('skill.skill_modalTitleAdd'),
);

const formSchema = computed<VbenFormSchema[]>(() => {
  const fields: VbenFormSchema[] = [];
  // 新增时显示版本号字段（由技能包解析回填或手填）
  if (!isEdit.value) {
    fields.push({
      component: 'Input',
      fieldName: 'version',
      label: $t('skill.skill_versionCol'),
      rules: 'required',
      componentProps: { placeholder: $t('skill.skill_versionPlaceholder') },
    });
  }
  fields.push(
    {
      component: 'Select',
      fieldName: 'namespaceId',
      label: $t('skill.skill_namespace'),
      rules: 'required',
      componentProps: () => ({
        allowClear: true,
        options: namespaceOptions.value,
        placeholder: $t('skill.skill_namespacePlaceholder'),
        style: { width: '100%' },
      }),
    },
    {
      component: 'Input',
      fieldName: 'slug',
      label: $t('skill.skill_slug'),
      rules: 'required',
      componentProps: { placeholder: $t('skill.skill_slugPlaceholder') },
    },
    {
      component: 'Input',
      fieldName: 'displayName',
      label: $t('skill.skill_name'),
      rules: 'required',
      componentProps: { placeholder: $t('skill.skill_namePlaceholder') },
    },
    {
      component: 'Textarea',
      fieldName: 'summary',
      label: $t('skill.skill_summary'),
      componentProps: { placeholder: $t('skill.skill_summaryPlaceholder'), rows: 3 },
    },
    {
      component: 'RadioGroup',
      fieldName: 'visibility',
      label: $t('skill.skill_visibility'),
      componentProps: {
        options: [
          { label: $t('skill.skill_visibilityPublic'), value: 0 },
          { label: $t('skill.skill_visibilityPrivate'), value: 1 },
        ],
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('skill.skill_status'),
      componentProps: {
        options: [
          { label: $t('skill.skill_enabled'), value: 1 },
          { label: $t('skill.skill_disabled'), value: 0 },
        ],
      },
    },
  );
  return fields;
});

const [Form, formApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: formSchema,
    showDefaultActions: false,
  }),
);

// ===== 技能包上传（新增时）：选 zip → 解析 SKILL.md 回填字段 + 暂存 tempId =====
const packageTempId = ref('');

async function handlePackageParse(file: any) {
  try {
    const res: any = await parseSkillPackageApi(file);
    packageTempId.value = res.tempId;
    await formApi.setValues({
      slug: res.slug,
      displayName: res.name || res.slug,
      summary: res.summary || '',
      version: res.version || '1.0.0',
    });
    message.success($t('skill.skill_parseOk'));
  } catch {
    packageTempId.value = '';
    /* 拦截器已提示 */
  }
  return false; // 阻止 antd 自动上传，由我们手动调 parse
}

async function openAdd() {
  isEdit.value = false;
  editingId.value = '';
  packageTempId.value = '';
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({ visibility: 0, status: 1 });
}

async function openEdit(row: any) {
  isEdit.value = true;
  editingId.value = row.skillId;
  modalVisible.value = true;
  await nextTick();
  await formApi.resetForm();
  await formApi.setValues({
    namespaceId: row.namespaceId,
    slug: row.slug,
    displayName: row.displayName,
    summary: row.summary ?? '',
    visibility: row.visibility,
    status: row.status,
  });
}

async function onSubmit() {
  const { valid } = await formApi.validate();
  if (!valid) return;
  const values: any = await formApi.getValues();
  try {
    if (isEdit.value) {
      // gin edit 体: { skillId, ...SkillReq }
      await editSkillApi({ ...values, skillId: editingId.value });
      message.success($t('skill.skill_editSuccess'));
    } else if (packageTempId.value) {
      // 上传包新增：凭 tempId 创建 skill + 版本 + 持久化包
      await createSkillPackageApi({
        tempId: packageTempId.value,
        namespaceId: values.namespaceId,
        slug: values.slug,
        displayName: values.displayName,
        summary: values.summary,
        visibility: values.visibility,
        status: values.status,
        version: values.version,
        changelog: '',
      });
      message.success($t('skill.skill_addSuccess'));
    } else {
      // 无包手填新增（fallback）
      await addSkillApi(values);
      message.success($t('skill.skill_addSuccess'));
    }
    modalVisible.value = false;
    gridApi.query();
  } catch {
    /* 拦截器已提示 */
  }
}

// 行内状态切换：edit 整行（镜像 openEdit 字段），失败回滚。
async function onStatusChange(row: any, checked: boolean) {
  const newStatus = checked ? 1 : 0;
  const prev = row.status;
  row.status = newStatus;
  try {
    await editSkillApi({
      namespaceId: row.namespaceId,
      slug: row.slug,
      displayName: row.displayName,
      summary: row.summary ?? '',
      visibility: row.visibility,
      status: newStatus,
      skillId: row.skillId,
    });
    message.success($t('skill.skill_editSuccess'));
  } catch {
    row.status = prev;
  }
}

// ===== 删除 =====
async function onDeleteBatch() {
  const rows = gridApi.grid?.getCheckboxRecords?.() ?? [];
  if (!rows.length) {
    message.warning($t('skill.skill_selectToDelete'));
    return;
  }
  await doDelete(rows.map((r: any) => r.skillId), rows.length);
}
async function onDeleteRow(row: any) {
  await doDelete([row.skillId], 1);
}
async function doDelete(ids: string[], count: number) {
  Modal.confirm({
    title: $t('skill.skill_tipTitle'),
    content: $t('skill.skill_confirmDeleteContent', { count }),
    okText: $t('skill.skill_confirm'),
    cancelText: $t('skill.skill_cancel'),
    okType: 'danger',
    onOk: async () => {
      await deleteSkillApi(ids);
      message.success($t('skill.skill_deleteSuccess'));
      gridApi.query();
    },
  });
}

// ===== 隐藏切换 =====
async function onHiddenChange(row: any, checked: boolean) {
  const newHidden = checked ? 1 : 0;
  try {
    await setSkillHiddenApi(row.skillId, newHidden);
    row.hidden = newHidden;
    message.success(newHidden === 1 ? $t('skill.skill_hideSuccess') : $t('skill.skill_showSuccess'));
  } catch {
    row.hidden = newHidden === 1 ? 0 : 1;
  }
}

// ===== 版本管理弹窗 =====
const versionVisible = ref(false);
const currentSkill = ref<any>(null);
const versions = ref<any[]>([]);
const versionAdd = reactive({ version: '', changelog: '' });

async function openVersions(row: any) {
  currentSkill.value = row;
  versionAdd.version = '';
  versionAdd.changelog = '';
  versionVisible.value = true;
  await loadVersions(row.skillId);
}
async function loadVersions(skillId: string) {
  try {
    const res: any = await getSkillVersionsApi(skillId);
    versions.value = res?.list ?? [];
  } catch {
    /* 忽略 */
  }
}
async function onAddVersion() {
  if (!versionAdd.version) {
    message.warning($t('skill.skill_versionRequired'));
    return;
  }
  try {
    await addSkillVersionApi({
      skillId: currentSkill.value.skillId,
      version: versionAdd.version,
      changelog: versionAdd.changelog,
    });
    message.success($t('skill.skill_versionAddSuccess'));
    versionAdd.version = '';
    versionAdd.changelog = '';
    await loadVersions(currentSkill.value.skillId);
  } catch {
    /* 忽略 */
  }
}
async function onPublish(row: any) {
  try {
    await publishSkillVersionApi(row.versionId);
    message.success($t('skill.skill_publishSuccess'));
    await loadVersions(currentSkill.value.skillId);
  } catch {
    /* 忽略 */
  }
}
function onYank(row: any) {
  Modal.confirm({
    title: $t('skill.skill_yankTitle'),
    content: $t('skill.skill_yankContent'),
    okText: $t('skill.skill_confirm'),
    cancelText: $t('skill.skill_cancel'),
    onOk: async () => {
      try {
        await yankSkillVersionApi(row.versionId, '');
        message.success($t('skill.skill_yankSuccess'));
        await loadVersions(currentSkill.value.skillId);
      } catch {
        /* 忽略 */
      }
    },
  });
}

// ===== 详情 Drawer =====
const detailVisible = ref(false);
const detailRow = ref<any>(null);
const detailVersions = ref<any[]>([]);
const versionFiles = ref<Record<string, any[]>>({});

async function openDetail(row: any) {
  detailRow.value = row;
  detailVersions.value = [];
  versionFiles.value = {};
  detailVisible.value = true;
  try {
    const res: any = await getSkillVersionsApi(row.skillId);
    detailVersions.value = res?.list ?? [];
  } catch {
    /* 拦截器已提示 */
  }
}

async function onCollapseChange(keys: any) {
  const arr = Array.isArray(keys) ? keys : keys ? [keys] : [];
  for (const k of arr) {
    if (k && !versionFiles.value[k]) {
      try {
        const res: any = await getSkillFilesApi(k as string);
        versionFiles.value = { ...versionFiles.value, [k]: res?.list ?? [] };
      } catch {
        /* 拦截器已提示 */
      }
    }
  }
}

function versionStatusText(s: number) {
  return s === 0 ? 'DRAFT' : s === 1 ? 'PUBLISHED' : 'YANKED';
}

// 安全审计状态（后端 entity.AuditStatus*）：1=安全 2=不安全 3=扫描中 4=扫描失败；
// undefined/0=待扫描（无记录也按待扫描口径展示——后端门禁同样拦）。
function auditTag(s: number | undefined) {
  if (s === 1) return { color: 'success', text: $t('skill.sec_statusPass') };
  if (s === 2) return { color: 'error', text: $t('skill.sec_statusFail') };
  if (s === 3) return { color: 'blue', text: $t('skill.sec_statusScanning') };
  if (s === 4) return { color: 'orange', text: $t('skill.sec_statusError') };
  return { color: 'default', text: $t('skill.sec_statusPending') };
}
function formatSize(bytes: number) {
  if (!bytes) return '0';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1024 / 1024).toFixed(2) + ' MB';
}
</script>

<template>
  <Page :auto-content-height="true">
    <Grid>
      <template #toolbar-actions>
        <Button type="primary" @click="openAdd">{{ $t('skill.skill_add') }}</Button>
        <Button danger class="ml-2" @click="onDeleteBatch">{{ $t('skill.skill_batchDelete') }}</Button>
      </template>
      <template #owner="{ row }">
        <span>{{ row.ownerName || row.ownerId || '-' }}</span>
        <span v-if="row.ownerEmpNo" class="ml-1 text-xs opacity-60">({{ row.ownerEmpNo }})</span>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          :checked-children="$t('skill.skill_enabled')"
          :un-checked-children="$t('skill.skill_disabled')"
          @change="(v: any) => onStatusChange(row, v)"
        />
      </template>
      <template #hidden="{ row }">
        <Switch
          :checked="row.hidden === 1"
          :checked-children="$t('skill.skill_hiddenShort')"
          :un-checked-children="$t('skill.skill_shownShort')"
          @change="(v: any) => onHiddenChange(row, v)"
        />
      </template>
      <template #source="{ row }">
        <div class="flex flex-col">
          <a v-if="row.sourceUrl" :href="row.sourceUrl" target="_blank" class="text-sm">{{
            row.source
          }}</a>
          <span v-else-if="row.source" class="text-sm text-gray-500">{{ row.source }}</span>
          <span v-else class="text-xs text-gray-400">—</span>
          <span v-if="row.upstreamAuthor" class="text-xs text-gray-400">
            {{ $t('skill.skill_upstreamAuthor') }}: {{ row.upstreamAuthor }}
          </span>
        </div>
      </template>
      <template #action="{ row }">
        <Button type="primary" ghost size="small" @click="openDetail(row)">{{ $t('skill.skill_detail') }}</Button>
        <Button type="primary" ghost size="small" @click="openEdit(row)">{{ $t('skill.skill_edit') }}</Button>
        <Button type="primary" ghost size="small" @click="openVersions(row)">
          {{ $t('skill.skill_version') }}
        </Button>
        <Button type="primary" ghost size="small" danger @click="onDeleteRow(row)">
          {{ $t('skill.skill_delete') }}
        </Button>
      </template>
    </Grid>

    <Modal
      v-model:open="modalVisible"
      :title="modalTitle"
      :width="640"
      :destroy-on-close="true"
      @ok="onSubmit"
    >
      <div v-if="!isEdit" class="mb-3">
        <div class="mb-2">{{ $t('skill.skill_package') }}</div>
        <Upload
          accept=".zip"
          :max-count="1"
          :before-upload="handlePackageParse"
        >
          <Button type="primary" ghost size="small">{{ $t('skill.skill_selectZip') }}</Button>
        </Upload>
        <div v-if="packageTempId" class="mt-1" style="color: #42b983">
          {{ $t('skill.skill_parseOkTip') }}
        </div>
      </div>
      <Form />
    </Modal>

    <Modal
      v-model:open="versionVisible"
      :title="$t('skill.skill_versionManageTitle', { name: currentSkill?.displayName || '' })"
      :width="780"
      :destroy-on-close="true"
      :footer="null"
    >
      <div class="mb-3 flex items-center gap-2">
        <Input
          v-model:value="versionAdd.version"
          :placeholder="$t('skill.skill_versionPlaceholder')"
          style="width: 30%"
        />
        <Textarea
          v-model:value="versionAdd.changelog"
          :placeholder="$t('skill.skill_changelogPlaceholder')"
          :rows="1"
          style="flex: 1"
        />
        <Button type="primary" @click="onAddVersion">{{ $t('skill.skill_addBtn') }}</Button>
      </div>
      <Table
        :data-source="versions"
        :pagination="false"
        size="small"
        :columns="[
          { title: $t('skill.skill_versionCol'), dataIndex: 'version', width: 120 },
          { title: $t('skill.skill_changelog'), dataIndex: 'changelog', ellipsis: true },
          { title: $t('skill.skill_status'), dataIndex: 'status', width: 110 },
          { title: $t('skill.skill_auditCol'), key: 'audit', width: 110 },
          { title: $t('skill.skill_createdAt'), dataIndex: 'createdAt', width: 160 },
          { title: $t('skill.skill_action'), key: 'action', width: 120 },
        ]"
        row-key="versionId"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'status'">
            <span v-if="record.status === 0" class="aa-enum aa-enum--default">DRAFT</span>
            <span v-else-if="record.status === 1" class="aa-enum aa-enum--success">
              PUBLISHED
            </span>
            <span v-else class="aa-enum aa-enum--error">YANKED</span>
          </template>
          <template v-else-if="column.key === 'audit'">
            <span :class="['aa-enum', `aa-enum--${auditTag(record.auditStatus).color}`]">
              {{ auditTag(record.auditStatus).text }}
            </span>
          </template>
          <template v-else-if="column.key === 'action'">
            <!-- 发布门禁（前端预禁）：安全审计非「安全」不可发布，悬停说明原因；后端同拦 -->
            <Tooltip
              v-if="record.status !== 1"
              :title="record.auditStatus !== 1 ? $t('skill.skill_publishBlockedAudit') : ''"
            >
              <span>
                <Button
                  type="primary" ghost
                  size="small"
                  :disabled="record.auditStatus !== 1"
                  @click="onPublish(record)"
                >
                  {{ $t('skill.skill_publish') }}
                </Button>
              </span>
            </Tooltip>
            <Button
              v-if="record.status === 1"
              type="primary" ghost
              size="small"
              danger
              @click="onYank(record)"
            >
              {{ $t('skill.skill_yank') }}
            </Button>
          </template>
        </template>
      </Table>
    </Modal>
    <Drawer
      v-model:open="detailVisible"
      :title="$t('skill.skill_detailTitle')"
      :width="720"
    >
      <Descriptions v-if="detailRow" :column="2" size="small" bordered>
        <DescriptionsItem :label="$t('skill.skill_slug')">{{ detailRow.slug }}</DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_name')">{{ detailRow.displayName }}</DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_owner')">
          {{ detailRow.ownerName || detailRow.ownerId || '-' }}
          <span v-if="detailRow.ownerEmpNo" class="opacity-60">({{ detailRow.ownerEmpNo }})</span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_status')">
          {{ detailRow.status === 1 ? $t('skill.skill_enabled') : $t('skill.skill_disabled') }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_visibility')">
          {{ detailRow.visibility === 0 ? $t('skill.skill_visibilityPublic') : $t('skill.skill_visibilityPrivate') }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_createdAt')">{{ detailRow.createdAt }}</DescriptionsItem>
        <DescriptionsItem :label="$t('skill.skill_summary')" :span="2">{{ detailRow.summary || '-' }}</DescriptionsItem>
      </Descriptions>
      <div class="mt-4 mb-2 font-medium">{{ $t('skill.skill_version') }}</div>
      <Collapse @change="onCollapseChange">
        <CollapsePanel
          v-for="v in detailVersions"
          :key="v.versionId"
          :header="`${v.version} · ${versionStatusText(v.status)} · ${auditTag(v.auditStatus).text}`"
        >
          <div class="mb-2">
            <Button
              size="small"
              type="primary"
              ghost
              @click="downloadSkillVersionApi(v.versionId, `${v.version}.zip`)"
            >
              {{ $t('skill.skill_downloadPackage') }}
            </Button>
          </div>
          <Table
            :data-source="versionFiles[v.versionId] || []"
            :pagination="false"
            size="small"
            row-key="fileId"
            :columns="[
              { title: $t('skill.skill_filePath'), dataIndex: 'filePath', ellipsis: true },
              { title: $t('skill.skill_fileSize'), dataIndex: 'fileSize', width: 90 },
              { title: $t('skill.skill_sha256'), dataIndex: 'sha256', width: 110 },
              { title: $t('skill.skill_action'), key: 'action', width: 70 },
            ]"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.dataIndex === 'fileSize'">{{ formatSize(record.fileSize) }}</template>
              <template v-else-if="column.dataIndex === 'sha256'">{{ record.sha256 ? record.sha256.slice(0, 8) : '' }}</template>
              <template v-else-if="column.key === 'action'">
                <Button size="small" type="link" @click="downloadSkillFileApi(v.versionId, record.filePath)">
                  {{ $t('skill.skill_downloadFile') }}
                </Button>
              </template>
            </template>
          </Table>
        </CollapsePanel>
      </Collapse>
    </Drawer>
  </Page>
</template>

<style scoped>
.mb-3 {
  margin-bottom: 12px;
}
.gap-2 {
  gap: 8px;
}
.flex {
  display: flex;
}
.items-center {
  align-items: center;
}
</style>
