<script lang="ts" setup>
/**
 * 配置对比弹窗：两个配置快照（config_change 行）逐行双栏 diff。
 * - 左=较早(旧)、右=较晚(新)，由调用方按 changedAt 排序后传入。
 * - 行级 LCS diff（手写，零依赖），产出左右对齐行：same/del(左独有)/add(右独有)。
 * - 用于「运行详情 → 配置变更」tab 勾选 2 条后对比 openclaw.json 的演变。
 */
import { computed } from 'vue';

import { Modal } from 'ant-design-vue';

import { $t } from '#/locales';

interface Snapshot {
  changeId?: string;
  changedAt?: string;
  source?: string;
  dispatcher?: string;
  configHash?: string;
  configSize?: number;
  content?: string;
}

const props = defineProps<{
  open: boolean;
  left?: Snapshot;
  right?: Snapshot;
  title?: string;
}>();
const emit = defineEmits<{ 'update:open': [boolean] }>();

type DiffRow = {
  kind: 'add' | 'del' | 'same';
  leftNo?: number;
  rightNo?: number;
  left?: string;
  right?: string;
};

// 行级 LCS diff：产出左右对齐行（same=两边相同 / del=左独有 / add=右独有），各带行号。
// O(n*m) DP，配置文件通常百行级，完全够用。
function lineDiff(a: string[], b: string[]): DiffRow[] {
  const n = a.length;
  const m = b.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const rows: DiffRow[] = [];
  let i = 0;
  let j = 0;
  let ln = 1;
  let rn = 1;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      rows.push({ kind: 'same', leftNo: ln, rightNo: rn, left: a[i], right: b[j] });
      i++;
      j++;
      ln++;
      rn++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      rows.push({ kind: 'del', leftNo: ln, left: a[i] });
      i++;
      ln++;
    } else {
      rows.push({ kind: 'add', rightNo: rn, right: b[j] });
      j++;
      rn++;
    }
  }
  while (i < n) {
    rows.push({ kind: 'del', leftNo: ln, left: a[i] });
    i++;
    ln++;
  }
  while (j < m) {
    rows.push({ kind: 'add', rightNo: rn, right: b[j] });
    j++;
    rn++;
  }
  return rows;
}

const rows = computed<DiffRow[]>(() => {
  const a = (props.left?.content ?? '').split('\n');
  const b = (props.right?.content ?? '').split('\n');
  return lineDiff(a, b);
});

const stats = computed(() => {
  let add = 0;
  let del = 0;
  for (const r of rows.value) {
    if (r.kind === 'add') add++;
    else if (r.kind === 'del') del++;
  }
  return { add, del };
});

const sourceText = (s?: string) => {
  if (s === 'runtime') return $t('agent.instance_diffSrcRuntime');
  if (s === 'dispatch') return $t('agent.instance_diffSrcDispatch');
  return s || '-';
};

function onClose() {
  emit('update:open', false);
}
</script>

<template>
  <Modal
    :open="open"
    :title="title || $t('agent.instance_diffTitle')"
    :width="1100"
    :footer="null"
    :destroy-on-close="true"
    wrap-class-name="config-diff-modal"
    @cancel="onClose"
  >
    <!-- 两侧元信息：左=旧 / 右=新 -->
    <div class="mb-3 grid grid-cols-2 gap-3 text-xs">
      <div class="rounded border border-solid border-gray-200 p-2 dark:border-gray-700">
        <div class="mb-1 flex items-center gap-2">
          <span class="aa-enum aa-enum--default">{{ $t('agent.instance_diffOld') }}</span>
          <span class="text-gray-500">{{ left?.changedAt || '-' }}</span>
        </div>
        <div class="text-gray-500">
          {{ $t('agent.instance_reportColSource') }}：{{ sourceText(left?.source) }}
          ｜ {{ $t('agent.instance_reportColDispatcher') }}：{{ left?.dispatcher || '-' }}
        </div>
        <div class="mt-0.5 font-mono text-gray-400">
          hash：{{ left?.configHash?.slice(0, 12) || '-' }} ｜ size：{{ left?.configSize ?? '-' }} B
        </div>
      </div>
      <div class="rounded border border-solid border-gray-200 p-2 dark:border-gray-700">
        <div class="mb-1 flex items-center gap-2">
          <span class="aa-enum aa-enum--magenta">{{ $t('agent.instance_diffNew') }}</span>
          <span class="text-gray-500">{{ right?.changedAt || '-' }}</span>
        </div>
        <div class="text-gray-500">
          {{ $t('agent.instance_reportColSource') }}：{{ sourceText(right?.source) }}
          ｜ {{ $t('agent.instance_reportColDispatcher') }}：{{ right?.dispatcher || '-' }}
        </div>
        <div class="mt-0.5 font-mono text-gray-400">
          hash：{{ right?.configHash?.slice(0, 12) || '-' }} ｜ size：{{ right?.configSize ?? '-' }} B
        </div>
      </div>
    </div>

    <!-- 统计 -->
    <div class="mb-2 flex items-center gap-3 text-xs">
      <span class="font-medium text-green-600">{{ $t('agent.instance_diffStatAdd', { n: stats.add }) }}</span>
      <span class="font-medium text-red-600">{{ $t('agent.instance_diffStatDel', { n: stats.del }) }}</span>
    </div>

    <!-- 双栏逐行 diff（grid 四列：左行号 | 左内容 | 右行号 | 右内容，天然对齐、单容器滚动） -->
    <div class="max-h-[60vh] overflow-auto rounded border border-solid border-gray-200 dark:border-gray-700">
      <div v-if="!rows.length" class="p-4 text-center text-xs text-gray-400">
        {{ $t('agent.instance_diffNoContent') }}
      </div>
      <div
        v-for="(r, idx) in rows"
        v-else
        :key="idx"
        class="diff-row grid"
        :class="{ 'diff-del': r.kind === 'del', 'diff-add': r.kind === 'add' }"
      >
        <span class="diff-ln">{{ r.leftNo ?? '' }}</span>
        <span class="diff-cell" :class="{ 'is-del': r.kind === 'del' }">{{ r.left ?? '' }}</span>
        <span class="diff-ln">{{ r.rightNo ?? '' }}</span>
        <span class="diff-cell" :class="{ 'is-add': r.kind === 'add' }">{{ r.right ?? '' }}</span>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.diff-row {
  grid-template-columns: 3rem 1fr 3rem 1fr;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 20px;
}
.diff-ln {
  padding: 0 6px;
  color: #9ca3af;
  text-align: right;
  user-select: none;
  border-right: 1px solid rgb(0 0 0 / 6%);
}
.diff-cell {
  padding: 0 8px;
  white-space: pre;
  overflow: hidden;
}
.diff-row.diff-add {
  background: rgb(34 197 94 / 12%);
}
.diff-row.diff-del {
  background: rgb(239 68 68 / 12%);
}
.is-add {
  color: rgb(22 163 74);
  font-weight: 600;
}
.is-del {
  color: rgb(220 38 38);
  font-weight: 600;
}
</style>
