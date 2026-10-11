<script lang="ts" setup>
/**
 * 运行详情·时间筛选：RangePicker（带时分秒）+ 重置按钮。
 * 用于「运行详情」Drawer 内 history/config/commands 三个 tab 的顶部；
 * current tab 为快照无时间列，不渲染本组件。
 * v-model 绑时间区间；点重置即清空（父 watch 会把分页重置到第 1 页）。
 */
import type { Dayjs } from 'dayjs';

import { Button, DatePicker } from 'ant-design-vue';

import { $t } from '#/locales';

const { RangePicker } = DatePicker;

const model = defineModel<[Dayjs, Dayjs] | undefined>();

function reset() {
  model.value = undefined;
}
</script>

<template>
  <div class="mb-3 flex flex-wrap items-center gap-2">
    <RangePicker
      v-model:value="model"
      show-time
      format="YYYY-MM-DD HH:mm:ss"
      :placeholder="[$t('agent.instance_reportTimeStart'), $t('agent.instance_reportTimeEnd')]"
    />
    <Button size="small" @click="reset">{{ $t('agent.instance_reportReset') }}</Button>
  </div>
</template>
