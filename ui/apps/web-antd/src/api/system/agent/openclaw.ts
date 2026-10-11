/**
 * Agent OpenClaw 配置接口（/api/v1/agent/openclaw/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 * OpenClaw 配置 = openclaw 实例容器内的 openclaw.json（控制面库中留有主档，
 * 编辑保存后经 manager 命令通道下发，容器内落盘并按需重启生效）。
 * 字段结构为前端约定契约（INTEGRATION），后端按此实现。
 */
import { requestClient } from '#/api/request';

export namespace AgentOpenclawApi {
  /** 配置列表行（GET /agent/openclaw/configs） */
  export interface ConfigItem {
    instanceId: string;
    name: string;
    agent: string; // openclaw
    version?: string;
    status: number; // 同实例：0=creating 1=running 2=stopped 3=error
    /** 容器内配置文件路径（展示用，如 /root/.openclaw/openclaw.json） */
    configPath?: string;
    configHash?: string;
    configSize?: number;
    /** 库中主档最近一次更新时间 */
    updatedAt?: string;
    /** 最近一次成功下发到实例的时间（后端富化） */
    syncedAt?: string;
    /** 当前编辑器内容（=主档 content，列表接口可不给以省流量，详情接口必给） */
    content?: string;
  }

  /** 详情响应（GET /agent/openclaw/configs/:instanceId） */
  export interface ConfigDetail extends ConfigItem {
    content: string;
  }

  /** 保存下发响应 */
  export interface SaveResult {
    instanceId: string;
    configHash?: string;
    /** manager 命令 id（执行进度可在实例运行详情·命令页签查看） */
    commandId?: string;
  }

  /** 校验响应（POST /agent/openclaw/configs/validate） */
  export interface ValidateResult {
    valid: boolean;
    errors: string[];
  }
}

const BASE = '/api/v1/agent/openclaw';

/** OpenClaw 实例配置列表（全部 openclaw 实例，含主档元信息） */
export function getAgentOpenclawConfigsApi() {
  return requestClient.get<{ list: AgentOpenclawApi.ConfigItem[] }>(
    `${BASE}/configs`,
  );
}

/** 单实例配置详情（含 content 全文） */
export function getAgentOpenclawConfigApi(instanceId: string) {
  return requestClient.get<AgentOpenclawApi.ConfigDetail>(
    `${BASE}/configs/${instanceId}`,
  );
}

/** 保存并下发（body: { content }；后端校验 JSON 后落主档 + enqueue manager 命令） */
export function saveAgentOpenclawConfigApi(instanceId: string, content: string) {
  return requestClient.put<AgentOpenclawApi.SaveResult>(
    `${BASE}/configs/${instanceId}`,
    { content },
  );
}

/** 校验配置文本（JSON 语法 + openclaw 必备段；不入库） */
export function validateAgentOpenclawConfigApi(content: string) {
  return requestClient.post<AgentOpenclawApi.ValidateResult>(
    `${BASE}/configs/validate`,
    { content },
  );
}
