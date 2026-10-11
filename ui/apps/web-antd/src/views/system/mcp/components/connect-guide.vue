<script lang="ts" setup>
/**
 * MCP 客户端接入引导弹窗：展示 Claude Code / Cursor / Claude Desktop 三种客户端的
 * 接入配置模板（Bearer token 由用户粘贴，明文只在签发/轮换时可见一次）。
 * scope 为空 → /mcp（全部 server 聚合）；传 server 名 → /mcp/{server}。
 */
import { computed, ref, watch } from 'vue';

import { Button, Input, message, Modal, Tabs } from 'ant-design-vue';

import { $t } from '#/locales';

const props = defineProps<{
  /** 端点 scope：空 = 全部聚合；server 名 = 单 server */
  scope?: string;
  /** 预填 token（key 页签发/轮换后带入） */
  initialToken?: string;
}>();

const open = defineModel<boolean>('open', { default: false });
const token = ref('');

watch(
  () => props.initialToken,
  (v) => {
    if (v) token.value = v;
  },
);

const endpoint = computed(
  () => `http://mcp.aiarsenal.cc/mcp${props.scope ? `/${props.scope}` : ''}`,
);

const claudeCodeCmd = computed(() => {
  const t = token.value || '<TOKEN>';
  return `claude mcp add --transport http aiarsenal ${endpoint.value} --header "Authorization: Bearer ${t}"`;
});

const cursorJson = computed(() => {
  const t = token.value || '<TOKEN>';
  return JSON.stringify(
    {
      mcpServers: {
        aiarsenal: {
          url: endpoint.value,
          headers: { Authorization: `Bearer ${t}` },
        },
      },
    },
    null,
    2,
  );
});

const desktopJson = computed(() => {
  const t = token.value || '<TOKEN>';
  return JSON.stringify(
    {
      mcpServers: {
        aiarsenal: {
          command: 'npx',
          args: [
            '-y',
            'mcp-remote',
            endpoint.value,
            '--header',
            `Authorization:Bearer ${t}`,
          ],
        },
      },
    },
    null,
    2,
  );
});

function copy(text: string) {
  navigator.clipboard?.writeText(text);
  message.success($t('mcp.connect_copied'));
}
</script>

<template>
  <Modal
    v-model:open="open"
    :title="$t('mcp.connect_title')"
    :width="720"
    :footer="null"
    :destroy-on-close="true"
  >
    <div class="mb-3 flex items-center gap-2">
      <span class="w-24 shrink-0">{{ $t('mcp.connect_token') }}</span>
      <Input
        v-model:value="token"
        type="password"
        allow-clear
        :placeholder="$t('mcp.connect_tokenPlaceholder')"
      />
    </div>
    <div class="mb-3 flex items-center gap-2">
      <span class="w-24 shrink-0">{{ $t('mcp.connect_endpoint') }}</span>
      <code class="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-gray-800">
        {{ endpoint }}
      </code>
    </div>
    <div v-if="scope" class="mb-3 text-xs opacity-70">
      {{ $t('mcp.connect_scopeHint', { scope }) }}
    </div>

    <Tabs>
      <Tabs.TabPane key="claude-code" :tab="$t('mcp.connect_claudeCode')">
        <div class="mb-1 text-xs opacity-70">
          {{ $t('mcp.connect_claudeCodeHint') }}
        </div>
        <pre
          class="overflow-x-auto rounded bg-zinc-900 p-3 text-xs leading-5 text-zinc-100"
          >{{ claudeCodeCmd }}</pre
        >
        <Button size="small" class="mt-2" @click="copy(claudeCodeCmd)">
          {{ $t('mcp.connect_copy') }}
        </Button>
      </Tabs.TabPane>
      <Tabs.TabPane key="cursor" :tab="$t('mcp.connect_cursor')">
        <div class="mb-1 text-xs opacity-70">
          {{ $t('mcp.connect_cursorHint') }}
        </div>
        <pre
          class="overflow-x-auto rounded bg-zinc-900 p-3 text-xs leading-5 text-zinc-100"
          >{{ cursorJson }}</pre
        >
        <Button size="small" class="mt-2" @click="copy(cursorJson)">
          {{ $t('mcp.connect_copy') }}
        </Button>
      </Tabs.TabPane>
      <Tabs.TabPane key="desktop" :tab="$t('mcp.connect_claudeDesktop')">
        <div class="mb-1 text-xs opacity-70">
          {{ $t('mcp.connect_claudeDesktopHint') }}
        </div>
        <pre
          class="overflow-x-auto rounded bg-zinc-900 p-3 text-xs leading-5 text-zinc-100"
          >{{ desktopJson }}</pre
        >
        <Button size="small" class="mt-2" @click="copy(desktopJson)">
          {{ $t('mcp.connect_copy') }}
        </Button>
      </Tabs.TabPane>
    </Tabs>

    <div class="mt-2 text-xs opacity-60">
      {{ $t('mcp.connect_docsHint') }}
    </div>
  </Modal>
</template>
