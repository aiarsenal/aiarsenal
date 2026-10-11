/**
 * Agent 镜像二元组（agent / version）下拉选项。
 * - agent：实例承载的智能体
 * - version：版本号，手输（无下拉）
 *
 * 选项写死在此文件；新增 Agent 类型时改这里即可，无需动后端。
 */
export const AGENT_OPTIONS = [
  { label: 'openclaw', value: 'openclaw' },
  { label: 'common', value: 'common' },
  { label: 'hermes', value: 'hermes' },
];

/**
 * 实例规格预设：CPU / 内存 / 磁盘 打包成 4 档，用户选一档即可。
 * label 里已写明规格，选中即看到具体数值。
 * 调整档位改这里即可（onSubmit 按 value 查 cpuCores/memoryGb/diskGb），无需动后端。
 */
export const SPEC_PRESETS = [
  { label: '轻量 · 1C / 2G / 20G', value: 'lite', cpuCores: 1, memoryGb: 2, diskGb: 20 },
  { label: '标准 · 2C / 4G / 40G', value: 'standard', cpuCores: 2, memoryGb: 4, diskGb: 40 },
  { label: '进阶 · 4C / 8G / 80G', value: 'advanced', cpuCores: 4, memoryGb: 8, diskGb: 80 },
  { label: '旗舰 · 8C / 16G / 160G', value: 'flagship', cpuCores: 8, memoryGb: 16, diskGb: 160 },
];
