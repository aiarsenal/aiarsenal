<script lang="ts" setup>
import { Page } from '@vben/common-ui';

import { Button, Card, message, notification, Space } from 'ant-design-vue';

import { $t } from '#/locales';

type NotificationType = 'error' | 'info' | 'success' | 'warning';

function info() {
  message.info('How many roads must a man walk down');
}

function error() {
  message.error({
    content: 'Once upon a time you dressed so fine',
    duration: 2500,
  });
}

function warning() {
  message.warning('How many roads must a man walk down');
}
function success() {
  message.success('Cause you walked hand in hand With another man in my place');
}

function notify(type: NotificationType) {
  notification[type]({
    duration: 2500,
    message: $t('core.demos.notifyMessage'),
    type,
  });
}
</script>

<template>
  <Page
    :description="$t('core.demos.pageDesc')"
    :title="$t('core.demos.pageTitle')"
  >
    <Card class="mb-5" :title="$t('core.demos.button')">
      <Space>
        <Button>Default</Button>
        <Button type="primary"> Primary </Button>
        <Button> Info </Button>
        <Button danger> Error </Button>
      </Space>
    </Card>
    <Card class="mb-5" :title="$t('core.demos.message')">
      <Space>
        <Button @click="info"> {{ $t('core.demos.info') }} </Button>
        <Button danger @click="error"> {{ $t('core.demos.error') }} </Button>
        <Button @click="warning"> {{ $t('core.demos.warning') }} </Button>
        <Button @click="success"> {{ $t('core.demos.success') }} </Button>
      </Space>
    </Card>

    <Card class="mb-5" :title="$t('core.demos.notification')">
      <Space>
        <Button @click="notify('info')"> {{ $t('core.demos.info') }} </Button>
        <Button danger @click="notify('error')"> {{ $t('core.demos.error') }} </Button>
        <Button @click="notify('warning')"> {{ $t('core.demos.warning') }} </Button>
        <Button @click="notify('success')"> {{ $t('core.demos.success') }} </Button>
      </Space>
    </Card>
  </Page>
</template>
