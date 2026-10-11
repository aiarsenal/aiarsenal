<script lang="ts" setup>
import type { VbenFormSchema } from '#/adapter/form';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, Card, message, Modal } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';
import {
  editPersonalApi,
  getPersonalInfoApi,
  resetPersonalPwdApi,
  type PersonalApi,
} from '#/api/system/personal';

defineOptions({ name: 'SystemPersonal' });

const info = ref<PersonalApi.InfoResult | null>(null);
const submitting = ref(false);

// ===== 个人资料表单 =====
const profileSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'Input',
    fieldName: 'nickname',
    label: $t('syspersonal.nickname'),
    rules: 'required',
    componentProps: { placeholder: $t('syspersonal.nicknamePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'userEmail',
    label: $t('syspersonal.userEmail'),
    rules: z.string().email($t('syspersonal.userEmailInvalid')),
    componentProps: { placeholder: $t('syspersonal.userEmailPlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'mobile',
    label: $t('syspersonal.mobile'),
    rules: 'required',
    componentProps: { placeholder: $t('syspersonal.mobilePlaceholder') },
  },
  {
    component: 'Select',
    fieldName: 'sex',
    label: $t('syspersonal.sex'),
    defaultValue: 0,
    componentProps: {
      options: [
        { label: $t('syspersonal.sexMale'), value: 1 },
        { label: $t('syspersonal.sexFemale'), value: 2 },
        { label: $t('syspersonal.sexSecret'), value: 0 },
      ],
    },
  },
  {
    component: 'Select',
    fieldName: 'remark',
    label: $t('syspersonal.profession'),
    componentProps: {
      allowClear: true,
      options: [
        { label: $t('syspersonal.professionIt'), value: '1' },
        { label: $t('syspersonal.professionManufacturing'), value: '2' },
        { label: $t('syspersonal.professionMedical'), value: '3' },
      ],
    },
  },
  {
    component: 'Input',
    fieldName: 'describe',
    label: $t('syspersonal.signature'),
    componentProps: { placeholder: $t('syspersonal.signaturePlaceholder') },
  },
  {
    component: 'Input',
    fieldName: 'avatar',
    label: $t('syspersonal.avatar'),
    componentProps: { placeholder: $t('syspersonal.avatarPlaceholder') },
  },
]);

const [ProfileForm, profileApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: profileSchema,
    showDefaultActions: false,
  }) as any,
);

// ===== 修改密码表单 =====
const pwdSchema = computed<VbenFormSchema[]>(() => [
  {
    component: 'InputPassword',
    fieldName: 'newPassword',
    label: $t('syspersonal.newPassword'),
    rules: z
      .string()
      .min(6, $t('syspersonal.newPasswordMin'))
      .nonempty($t('syspersonal.newPasswordRequired')),
    componentProps: { placeholder: $t('syspersonal.newPasswordPlaceholder') },
  },
  {
    component: 'InputPassword',
    fieldName: 'confirmPassword',
    label: $t('syspersonal.confirmPassword'),
    rules: z
      .string()
      .nonempty($t('syspersonal.confirmPasswordRequired'))
      .refine(async (v: string) => {
        const values = await pwdApi.getValues();
        return v === values?.newPassword;
      }, $t('syspersonal.confirmPasswordMismatch')),
    componentProps: { placeholder: $t('syspersonal.confirmPasswordPlaceholder') },
  },
]);

const [PwdForm, pwdApi] = useVbenForm(
  reactive({
    commonConfig: {
      fullWidth: true,
      hideLabel: false,
    },
    schema: pwdSchema,
    showDefaultActions: false,
  }) as any,
);

// ===== 加载个人资料 =====
async function loadInfo() {
  try {
    const res = await getPersonalInfoApi();
    info.value = res;
    const u = res?.user ?? ({} as PersonalApi.UserEntity);
    profileApi.setValues({
      nickname: u.userNickname ?? '',
      userEmail: u.userEmail ?? '',
      mobile: u.mobile ?? '',
      sex: u.sex ?? 0,
      remark: u.remark || undefined,
      describe: u.describe ?? '',
      avatar: u.avatar ?? '',
    });
  } catch {
    message.error($t('syspersonal.loadInfoFailed'));
  }
}

onMounted(loadInfo);

// ===== 提交资料 =====
async function onSubmitProfile() {
  const { valid } = await profileApi.validate();
  if (!valid) return;
  const values: any = await profileApi.getValues();
  submitting.value = true;
  try {
    await editPersonalApi({
      nickname: values.nickname,
      mobile: values.mobile,
      remark: values.remark ?? '',
      sex: Number(values.sex ?? 0),
      userEmail: values.userEmail,
      describe: values.describe ?? '',
      avatar: values.avatar ?? '',
    });
    message.success($t('syspersonal.profileUpdated'));
    await loadInfo();
  } catch {
    /* 拦截器已提示 */
  } finally {
    submitting.value = false;
  }
}

// ===== 提交修改密码 =====
async function onSubmitPwd() {
  const { valid } = await pwdApi.validate();
  if (!valid) return;
  const values: any = await pwdApi.getValues();
  Modal.confirm({
    title: $t('syspersonal.tipTitle'),
    content: $t('syspersonal.confirmResetPwdContent'),
    okText: $t('syspersonal.confirm'),
    cancelText: $t('syspersonal.cancel'),
    onOk: async () => {
      try {
        await resetPersonalPwdApi({ password: values.newPassword });
        message.success($t('syspersonal.pwdChanged'));
        await pwdApi.resetForm();
      } catch {
        /* 拦截器已提示 */
      }
    },
  });
}
</script>

<template>
  <Page :auto-content-height="true">
    <div class="personal-page">
      <!-- 顶部资料概览 -->
      <Card class="mb-4">
        <div class="personal-user">
          <div class="personal-avatar">
            <img
              v-if="info?.user?.avatar"
              :src="info.user.avatar"
              alt="avatar"
            />
            <span v-else class="avatar-placeholder">
              {{ info?.user?.userNickname?.charAt(0) || 'U' }}
            </span>
          </div>
          <div class="personal-meta">
            <div class="personal-title">
              {{ info?.user?.userNickname || '-' }}
              <span class="personal-sub">
                （{{ info?.user?.userName || '-' }}）
              </span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.mobileLabel') }}</span>
              <span>{{ info?.user?.mobile || '-' }}</span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.emailLabel') }}</span>
              <span>{{ info?.user?.userEmail || '-' }}</span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.deptLabel') }}</span>
              <span>{{ info?.deptName || '-' }}</span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.roleLabel') }}</span>
              <span>{{ info?.roles?.join('，') || '-' }}</span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.lastLoginIpLabel') }}</span>
              <span>{{ info?.user?.lastLoginIp || '-' }}</span>
            </div>
            <div class="personal-row">
              <span class="label">{{ $t('syspersonal.lastLoginTimeLabel') }}</span>
              <span>{{ info?.user?.lastLoginTime || '-' }}</span>
            </div>
          </div>
        </div>
      </Card>

      <!-- 编辑资料 -->
      <Card :title="$t('syspersonal.profileCardTitle')" class="mb-4">
        <ProfileForm />
        <div class="form-actions">
          <Button
            type="primary"
            :loading="submitting"
            @click="onSubmitProfile"
          >
            {{ $t('syspersonal.saveProfile') }}
          </Button>
        </div>
      </Card>

      <!-- 修改密码 -->
      <Card :title="$t('syspersonal.securityCardTitle')">
        <div class="security-item">
          <div class="security-left">
            <div class="security-label">{{ $t('syspersonal.accountPassword') }}</div>
            <div class="security-value">{{ $t('syspersonal.securityTip') }}</div>
          </div>
          <Button type="link" @click="onSubmitPwd">{{ $t('syspersonal.changeNow') }}</Button>
        </div>
        <PwdForm />
        <div class="form-actions">
          <Button type="primary" @click="onSubmitPwd">{{ $t('syspersonal.confirmChangePwd') }}</Button>
        </div>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.personal-page {
  padding: 0;
}

.personal-user {
  display: flex;
  gap: 24px;
  align-items: center;
}

.personal-avatar {
  width: 96px;
  height: 96px;
  border-radius: 6px;
  overflow: hidden;
  background: #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.personal-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-placeholder {
  font-size: 36px;
  color: #999;
}

.personal-meta {
  flex: 1;
  min-width: 0;
}

.personal-title {
  font-size: 18px;
  font-weight: 500;
  margin-bottom: 8px;
}

.personal-sub {
  font-size: 13px;
  color: #999;
  font-weight: normal;
}

.personal-row {
  font-size: 13px;
  line-height: 24px;
  color: #333;
}

.personal-row .label {
  color: #999;
  display: inline-block;
  min-width: 96px;
}

.form-actions {
  margin-top: 16px;
}

.security-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
  margin-bottom: 16px;
}

.security-label {
  font-size: 14px;
  color: #333;
  margin-bottom: 4px;
}

.security-value {
  font-size: 12px;
  color: #999;
}

/* 正文深字仅浅色主题可用：暗色主题卡片区是深底，须换浅字（否则 #333 深底不可见） */
html.dark .personal-row,
html.dark .security-label {
  color: rgba(255, 255, 255, 0.88);
}

.mb-4 {
  margin-bottom: 16px;
}
</style>
