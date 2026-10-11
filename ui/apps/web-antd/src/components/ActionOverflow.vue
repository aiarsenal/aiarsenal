<script lang="ts" setup>
/**
 * 操作列按钮组：最多展示 `max` 个，超出折叠进「⋯」下拉。
 * - ≤ max：全部直接展示。
 * - > max：展示前 max-1 个 + 一个「⋯」触发器，点击展开剩余项（共 max 个可见单元）。
 * 按钮沿用列表操作列统一的 ghost 描边风格（普通蓝 / danger 红）；
 * 间距由全局 .vxe-cell/.ant-table-tbody 的 `.ant-btn + .ant-btn` 规则提供。
 */
import { computed } from 'vue';

import { Button, Dropdown, Menu } from 'ant-design-vue';

interface ActionItem {
  key: string;
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}

const props = withDefaults(defineProps<{ items: ActionItem[]; max?: number }>(), {
  max: 4,
});

const hasOverflow = computed(() => props.items.length > props.max);
const visible = computed(() =>
  hasOverflow.value ? props.items.slice(0, props.max - 1) : props.items,
);
const hidden = computed(() =>
  hasOverflow.value ? props.items.slice(props.max - 1) : [],
);

const menuItems = computed(() =>
  hidden.value.map((it) => ({
    key: it.key,
    label: it.label,
    danger: it.danger,
    disabled: it.disabled,
  })),
);

function onMenuClick(info: { key: string }) {
  hidden.value.find((it) => it.key === info.key)?.onClick?.();
}
</script>

<template>
  <div class="action-overflow">
    <Button
      v-for="it in visible"
      :key="it.key"
      type="primary"
      ghost
      size="small"
      :danger="it.danger"
      :disabled="it.disabled"
      @click="it.onClick?.()"
    >
      {{ it.label }}
    </Button>
    <Dropdown
      v-if="hasOverflow"
      placement="bottomRight"
      :trigger="['click']"
    >
      <Button type="primary" ghost size="small">⋯</Button>
      <template #overlay>
        <Menu :items="menuItems" @click="onMenuClick" />
      </template>
    </Dropdown>
  </div>
</template>
