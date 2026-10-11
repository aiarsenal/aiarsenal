package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// agent_openclaw_config_service.go — OpenClaw 配置页 API（v2 占位菜单落地）。
//
// 读：aa_agent_instance_command 里最近一条完成的 config_push（GetLastConfigPushPayload）
// payload = {"configContent":"<openclaw.json 全文>"}，反向读回给配置页展示；hash/size
// 与最近下发时间优先取 aa_agent_instance_config_change 的 dispatch 快照（与 runtime
// 上报对 hash 的判定口径一致）。
// 写：改完字段仍走既有 config_push 下发通道（pushOpenclawConfigRaw，与「刷新密钥」
// /模型重推同链路），source 记 "dispatch:config-page"（varchar(20) 恰好 20 字符，
// config_change 列表据此区分来源）。

// AgentOpenclawConfigService OpenClaw 配置页服务。
type AgentOpenclawConfigService struct {
	cmdRepo    repository.IAgentInstanceCommandRepository
	changeRepo repository.IAgentInstanceConfigChangeRepository
	instRepo   repository.IAgentInstanceRepository
	instSvc    *AgentInstanceService // 同包方法 pushOpenclawConfigRaw 在其上，main.go 装配后回指
}

// NewAgentOpenclawConfigService 构造。
func NewAgentOpenclawConfigService(
	cmdRepo repository.IAgentInstanceCommandRepository,
	changeRepo repository.IAgentInstanceConfigChangeRepository,
	instRepo repository.IAgentInstanceRepository,
) *AgentOpenclawConfigService {
	return &AgentOpenclawConfigService{cmdRepo: cmdRepo, changeRepo: changeRepo, instRepo: instRepo}
}

// SetInstanceService main.go 装配完 AgentInstanceService 后回指。
func (s *AgentOpenclawConfigService) SetInstanceService(v *AgentInstanceService) { s.instSvc = v }

// OpenclawConfigItem 配置页条目（列表行=无 Content；详情=含 Content 全文）。
type OpenclawConfigItem struct {
	InstanceId string      `json:"instanceId"`
	Name       string      `json:"name"`
	Agent      string      `json:"agent"`
	Version    string      `json:"version"`
	Status     uint        `json:"status"` // 同实例：0=creating 1=running 2=stopped 3=error
	ConfigHash string      `json:"configHash"`
	ConfigSize int64       `json:"configSize"`
	SyncedAt   *types.Time `json:"syncedAt"` // 最近一次下发/上报时间（无历史为 nil）
	Content    string      `json:"content"`  // 主档全文（列表可空串省流量）
	UpdatedAt  *types.Time `json:"updatedAt"`
}

// List 全部 openclaw 实例的配置概览（管理页列表行；主档读回逐实例 best-effort）。
func (s *AgentOpenclawConfigService) List(ctx context.Context) ([]*OpenclawConfigItem, error) {
	list, _, err := s.instRepo.ListAll(ctx, "", -1, 1, 1000)
	if err != nil {
		return nil, err
	}
	items := make([]*OpenclawConfigItem, 0, len(list))
	for _, ins := range list {
		if !agentDeliversConfig(ins.Agent) {
			continue // 仅平台下发配置的 agent（openclaw）入列
		}
		items = append(items, s.itemFor(ctx, ins))
	}
	return items, nil
}

// Get 单实例配置详情（含 content 全文）。
func (s *AgentOpenclawConfigService) Get(ctx context.Context, instanceId string) (*OpenclawConfigItem, error) {
	ins, err := s.instRepo.Get(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	return s.itemFor(ctx, ins), nil
}

// itemFor 单实例条目：主档读回（hash/size 兜底现算）+ 最近 dispatch 快照覆盖
// （快照与 runtime applied/reverted 判定同源，优先级更高）。
func (s *AgentOpenclawConfigService) itemFor(ctx context.Context, ins *entity.AgentInstance) *OpenclawConfigItem {
	it := &OpenclawConfigItem{
		InstanceId: ins.InstanceId, Name: ins.Name, Agent: ins.Agent, Version: ins.Version,
		Status: ins.Status, UpdatedAt: ins.UpdatedAt,
	}
	if payload, err := s.cmdRepo.GetLastConfigPushPayload(ctx, ins.InstanceId); err == nil && payload != "" {
		var parsed struct {
			ConfigContent string `json:"configContent"`
		}
		if json.Unmarshal([]byte(payload), &parsed) == nil && parsed.ConfigContent != "" {
			it.Content = parsed.ConfigContent
			it.ConfigHash = sha256Hex(it.Content)
			it.ConfigSize = int64(len(it.Content))
		}
		// 解析失败保持空：老版本 payload 结构差异，不阻断页面。
	}
	if ch, err := s.changeRepo.GetLatest(ctx, ins.InstanceId); err == nil && ch != nil {
		it.SyncedAt = ch.ChangedAt
		if ch.ConfigHash != "" {
			it.ConfigHash = ch.ConfigHash
		}
		if ch.ConfigSize > 0 {
			it.ConfigSize = ch.ConfigSize
		}
	}
	return it
}

// Save 保存并下发配置（全量覆盖）。reload=true 时 agent 接受后热重载，否则仅写盘
// （下次重启生效）。走既有 config_push 通道，与创建实例/刷新密钥同一链路。
// 返回落档后的配置 hash（与 config_change 快照同口径，前端展示对账用）。
func (s *AgentOpenclawConfigService) Save(ctx context.Context, instanceId, configContent, issuedBy string, reload bool) (string, error) {
	if strings.TrimSpace(configContent) == "" {
		return "", errors.New("配置内容不能为空")
	}
	inst, err := s.instRepo.Get(ctx, instanceId)
	if err != nil {
		return "", err
	}
	if !agentDeliversConfig(inst.Agent) {
		return "", fmt.Errorf("agent 类型 %s 不由平台下发 openclaw 配置", inst.Agent)
	}
	if s.instSvc == nil {
		return "", errors.New("实例服务未装配")
	}
	// source 必须 ≤20 字符（aa_agent_instance_config_change.source varchar(20)）。
	s.instSvc.pushOpenclawConfigRaw(ctx, instanceId, issuedBy, "dispatch:config-page", configContent, reload)
	return sha256Hex(configContent), nil
}

// Validate 校验配置文本（JSON 语法 + 顶层对象 + aiarsenal provider 存在性提示）。
// 仅语法/结构检查，不入库不下发。errors 为空即 valid。
func (s *AgentOpenclawConfigService) Validate(configContent string) ([]string, error) {
	trimmed := strings.TrimSpace(configContent)
	if trimmed == "" {
		return []string{"配置内容不能为空"}, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return []string{"不是合法 JSON：" + err.Error()}, nil
	}
	errs := make([]string, 0, 2)
	if cfg == nil {
		errs = append(errs, "顶层必须是 JSON 对象")
	}
	if nestedMap(cfg, "models", "providers") == nil {
		// 平台管配置的锚点段（providers.aiarsenal）；缺了 agent 侧守卫可能回滚，提示但不硬拒。
		errs = append(errs, "缺少 models.providers 段：平台下发配置以此为锚点，缺失可能被 agent 侧守卫回滚")
	}
	return errs, nil
}
