<script setup lang="ts">
import type { BasicOption } from '@vben/types';

import type { VbenFormSchema } from '#/adapter/form';

import { computed, onMounted, ref } from 'vue';

import { ProfileBaseSetting } from '@vben/common-ui';

import { $t } from '#/locales';
import { getUserInfoApi } from '#/api';

const profileBaseSettingRef = ref();

const MOCK_ROLES_OPTIONS = computed<BasicOption[]>(() => [
  {
    label: $t('core.profile.roleSuper'),
    value: 'super',
  },
  {
    label: $t('core.profile.roleUser'),
    value: 'user',
  },
  {
    label: $t('core.profile.roleTest'),
    value: 'test',
  },
]);

const formSchema = computed((): VbenFormSchema[] => {
  return [
    {
      fieldName: 'realName',
      component: 'Input',
      label: $t('core.profile.realName'),
    },
    {
      fieldName: 'username',
      component: 'Input',
      label: $t('core.profile.username'),
    },
    {
      fieldName: 'roles',
      component: 'Select',
      componentProps: {
        mode: 'tags',
        options: MOCK_ROLES_OPTIONS.value,
      },
      label: $t('core.profile.roles'),
    },
    {
      fieldName: 'introduction',
      component: 'Textarea',
      label: $t('core.profile.introduction'),
    },
  ];
});

onMounted(async () => {
  const data = await getUserInfoApi();
  profileBaseSettingRef.value.getFormApi().setValues(data);
});
</script>
<template>
  <ProfileBaseSetting ref="profileBaseSettingRef" :form-schema="formSchema" />
</template>
