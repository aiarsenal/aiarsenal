<script lang="ts" setup>
import type { VbenFormSchema } from '#/adapter/form';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Card, message } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';
import {
  getMcpConfigApi,
  getMcpUserConfigApi,
  saveMcpConfigApi,
  saveMcpUserConfigApi,
} from '#/api/system/mcp/config';

defineOptions({ name: 'SystemMcpConfig' });

// JSON 文本域字段的内联校验：提交前就红字提示，不等后端 22P02。
// 注意：vben 表单 rules 走 Zod 栈（form.vue 按 ZodTypeAny._def 解析默认值），
// 传 async-validator 数组会在渲染期抛错、整个表单渲染不出来。
const isJSON = (v: string) => {
  try {
    JSON.parse(v);
    return true;
  } catch {
    return false;
  }
};

const jsonField = (
  fieldName: string,
  label: string,
  placeholder = '{}',
): VbenFormSchema => ({
  component: 'Textarea',
  fieldName,
  label,
  componentProps: { rows: 3, placeholder },
  rules: z.string().refine((v) => !v || isJSON(v), $t('mcp.config_invalidJson')),
});

const saving = ref(false);
// 注意：Card 不能挂 :loading 骨架屏——骨架会卸载卡内的表单组件，
// formApi.setValues 等表单挂载永挂起，loading 永远回不到 false（骨架屏卡死）。

// ===== 系统配置表单（entity.McpConfig）=====
const systemSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'nameSeparator',
    label: $t('mcp.config_nameSeparator'),
    componentProps: {
      placeholder: $t('mcp.config_nameSeparatorPlaceholder'),
      style: { width: '120px' },
    },
  },
  jsonField('routing', $t('mcp.config_routing')),
  jsonField('install', $t('mcp.config_install')),
  jsonField('toolResultCompression', $t('mcp.config_toolResultCompression'), '{"enabled":false,"maxLength":20000}'),
  jsonField('mcpRouter', $t('mcp.config_mcpRouter')),
  jsonField('auth', $t('mcp.config_auth')),
  jsonField('discovery', $t('mcp.config_discovery')),
  jsonField('activityLog', $t('mcp.config_activityLog')),
  {
    component: 'Switch',
    fieldName: 'enableSessionRebuild',
    label: $t('mcp.config_enableSessionRebuild'),
    componentProps: {
      checkedChildren: $t('mcp.config_switchOn'),
      unCheckedChildren: $t('mcp.config_switchOff'),
    },
  },
]);

const [SystemForm, systemFormApi] = useVbenForm(
  reactive({
    commonConfig: { hideLabel: false },
    schema: systemSchema,
    showDefaultActions: false,
  }) as any,
);

// ===== 用户级配置表单（entity.McpUserConfig；userId 后端 JWT 注入）=====
const userSchema = computed<VbenFormSchema[]>(() => [
  jsonField('routing', $t('mcp.config_userRouting')),
  jsonField('additionalConfig', $t('mcp.config_additionalConfig')),
]);

const [UserForm, userFormApi] = useVbenForm(
  reactive({
    commonConfig: { hideLabel: false },
    schema: userSchema,
    showDefaultActions: false,
  }) as any,
);

// ===== 加载 =====
async function loadSystem() {
  try {
    const res: any = await getMcpConfigApi();
    const d = res ?? {};
    await systemFormApi.setValues({
      nameSeparator: d.nameSeparator ?? '/',
      routing: d.routing ?? '{}',
      install: d.install ?? '{}',
      toolResultCompression: d.toolResultCompression ?? '{}',
      mcpRouter: d.mcpRouter ?? '{}',
      auth: d.auth ?? '{}',
      discovery: d.discovery ?? '{}',
      activityLog: d.activityLog ?? '{}',
      // entity uint 0/1 → Switch 布尔
      enableSessionRebuild: Number(d.enableSessionRebuild) === 1,
    });
  } catch {
    /* 拦截器已提示 */
  }
}

async function loadUser() {
  try {
    const res: any = await getMcpUserConfigApi();
    const d = res ?? {};
    await userFormApi.setValues({
      routing: d.routing ?? '{}',
      additionalConfig: d.additionalConfig ?? '{}',
    });
  } catch {
    /* 拦截器已提示 */
  }
}

async function loadAll() {
  await Promise.all([loadSystem(), loadUser()]);
}

onMounted(loadAll);

// ===== 保存 =====
async function onSaveSystem() {
  const { valid } = await systemFormApi.validate();
  if (!valid) return;
  saving.value = true;
  try {
    const values: any = await systemFormApi.getValues();
    // Switch 布尔 → entity uint 0/1
    values.enableSessionRebuild = values.enableSessionRebuild ? 1 : 0;
    await saveMcpConfigApi(values);
    message.success($t('mcp.config_systemSaved'));
  } catch {
    /* 拦截器已提示 */
  } finally {
    saving.value = false;
  }
}

async function onSaveUser() {
  const { valid } = await userFormApi.validate();
  if (!valid) return;
  saving.value = true;
  try {
    const values: any = await userFormApi.getValues();
    await saveMcpUserConfigApi(values);
    message.success($t('mcp.config_userSaved'));
  } catch {
    /* 拦截器已提示 */
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Page :auto-content-height="true">
    <Card :title="$t('mcp.config_systemTitle')">
      <SystemForm />
      <div class="mt-4">
        <Button type="primary" :loading="saving" @click="onSaveSystem">
          {{ $t('mcp.config_save') }}
        </Button>
        <Button class="ml-2" @click="loadSystem">
          {{ $t('mcp.config_reload') }}
        </Button>
      </div>
    </Card>

    <Card :title="$t('mcp.config_userTitle')" class="mt-4">
      <UserForm />
      <div class="mt-4">
        <Button type="primary" :loading="saving" @click="onSaveUser">
          {{ $t('mcp.config_save') }}
        </Button>
        <Button class="ml-2" @click="loadUser">
          {{ $t('mcp.config_reload') }}
        </Button>
      </div>
    </Card>
  </Page>
</template>
