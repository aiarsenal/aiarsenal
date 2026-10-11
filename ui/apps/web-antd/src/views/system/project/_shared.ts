/**
 * 项目管理 — 项目选择器共享状态（项目内子页面通用）。
 * 模块级缓存：切换页面只拉一次项目列表。
 */
import { computed, ref } from 'vue';

import { getAllProjectsApi } from '#/api/system/project/project';
import { $t } from '#/locales';

const projects = ref<any[]>([]);
let loaded = false;

export async function ensureProjects() {
  if (!loaded) {
    try {
      const list = await getAllProjectsApi();
      projects.value = list ?? [];
    } catch {
      projects.value = [];
    }
    loaded = true;
  }
  return projects.value;
}

export function resetProjects() {
  loaded = false;
  projects.value = [];
}

// 优先级枚举（task/board 共用；computed 保语言切换实时变更）
export const PRIORITY_OPTIONS = computed(() => [
  { label: $t('project.priority_low'), value: 'low' },
  { label: $t('project.priority_medium'), value: 'medium' },
  { label: $t('project.priority_high'), value: 'high' },
  { label: $t('project.priority_urgent'), value: 'urgent' },
]);

/** 优先级 → 本地化标签（无匹配回落原值） */
export function priorityLabel(v: string): string {
  return PRIORITY_OPTIONS.value.find((o) => o.value === v)?.label ?? v;
}
