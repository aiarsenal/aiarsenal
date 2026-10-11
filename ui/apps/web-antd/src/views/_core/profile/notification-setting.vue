<script setup lang="ts">
import { computed } from 'vue';

import { ProfileNotificationSetting } from '@vben/common-ui';

import { $t } from '#/locales';

const formSchema = computed(() => {
  return [
    {
      value: true,
      fieldName: 'accountPassword',
      label: $t('core.profile.accountPassword'),
      description: $t('core.profile.notifyAccountPasswordDesc'),
    },
    {
      value: true,
      fieldName: 'systemMessage',
      label: $t('core.profile.systemMessage'),
      description: $t('core.profile.systemMessageDesc'),
    },
    {
      value: true,
      fieldName: 'todoTask',
      label: $t('core.profile.todoTask'),
      description: $t('core.profile.todoTaskDesc'),
    },
  ];
});
</script>
<template>
  <ProfileNotificationSetting :form-schema="formSchema" />
</template>
