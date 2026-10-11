<script setup lang="ts">
import { computed } from 'vue';

import { ProfileSecuritySetting } from '@vben/common-ui';

import { $t } from '#/locales';

const formSchema = computed(() => {
  return [
    {
      value: true,
      fieldName: 'accountPassword',
      label: $t('core.profile.accountPassword'),
      description: $t('core.profile.passwordStrengthStrong'),
    },
    {
      value: true,
      fieldName: 'securityPhone',
      label: $t('core.profile.securityPhone'),
      description: $t('core.profile.boundPhone'),
    },
    {
      value: true,
      fieldName: 'securityQuestion',
      label: $t('core.profile.securityQuestion'),
      description: $t('core.profile.securityQuestionDesc'),
    },
    {
      value: true,
      fieldName: 'securityEmail',
      label: $t('core.profile.securityEmail'),
      description: $t('core.profile.boundEmail'),
    },
    {
      value: false,
      fieldName: 'securityMfa',
      label: $t('core.profile.securityMfa'),
      description: $t('core.profile.securityMfaDesc'),
    },
  ];
});
</script>
<template>
  <ProfileSecuritySetting :form-schema="formSchema" />
</template>
