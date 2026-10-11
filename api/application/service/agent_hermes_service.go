package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"aiarsenal/domain/entity"
)

// agent_hermes_service.go — 实例（image/manager）场景的 hermes 下发（与 agent_openclaw_service.go 对称）。
//
// 仅实例侧的 dispatch 包装：构造 hermes config.yaml（调共享 agent_hermes.go 的
// FetchHermesDefaultModel + BuildHermesConfig）→ 经 config_push（reload=true 时追加 restart）下发给
// 容器内 manager（HermesManager.PushConfig 写 config.yaml，已实现）。共享 hermes 逻辑在 agent_hermes.go。

// deliverHermesConfig 向容器内 manager 下发 hermes config.yaml。
// reload=false（首次创建）：只下 config_push——executor 写盘后拉起 hermes（先配置后启动，带正确配置首次起）。
// reload=true（刷新密钥）：hermes 已在跑，config_push 仅写盘，需额外 restart 重读新 key。
// 模型拉取失败（网关不可达/无可用模型）→ error：不下发含假 default 的配置，错误冒到调用入口。
func (s *AgentInstanceService) deliverHermesConfig(ctx context.Context, instanceId, issuedBy, source, llmBaseURL, plainKey string, reload bool) error {
	defaultModel, provider, err := FetchHermesDefaultModel(ctx, s.llmModelRepo, llmBaseURL, plainKey)
	if err != nil {
		log.Printf("[instance:%s] 下发 hermes 配置跳过（拉取可用模型失败）: %v", instanceId, err)
		return fmt.Errorf("拉取可用模型失败（请先在【LLM 管理】配置模型部署）: %w", err)
	}
	opts := s.instanceHermesOpts(ctx, instanceId, llmBaseURL, plainKey, defaultModel)
	opts.APIMode = hermesAPIModeForProvider(provider) // anthropic 族须走 /v1/messages，写死 chat_completions 会 404
	cfgYAML := BuildHermesConfig(opts)
	if !reload {
		// 首次创建：先入队 SOUL 铁律（命令 FIFO，先于 config_push 写盘——SOUL 就位后 gateway 才首启）。
		s.pushHermesSoul(ctx, instanceId, issuedBy)
	}
	s.pushHermesConfigRaw(ctx, instanceId, issuedBy, source, cfgYAML, reload)
	log.Printf("[instance:%s] 已下发 hermes 配置（config_push%s，baseUrl=%s，default=%s，api_mode=%s）", instanceId, ifRestart(reload), llmBaseURL, defaultModel, opts.APIMode)
	return nil
}

// pushHermesSoul 入队 soul_push（AgentId=hermes）：把平台铁律 DefaultHermesSoulMandate 追加进实例
// SOUL.md（manager 端 HermesManager.PushSoul 幂等写入）。仅创建时下发一次；失败仅记日志（best-effort，
// 不阻断配置链路）。旧 manager（未含 soul_push 分支）会报未知命令，不影响 config_push 正常下发。
func (s *AgentInstanceService) pushHermesSoul(ctx context.Context, instanceId, issuedBy string) {
	if s.cmdRepo == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{"soulContent": DefaultHermesSoulMandate})
	if _, err := s.cmdRepo.Add(ctx, &entity.AgentInstanceCommand{
		InstanceId: instanceId, AgentId: "hermes", CommandType: "soul_push",
		PayloadJson: string(payload), Status: 0, IssuedBy: issuedBy, TimeoutSeconds: 60,
	}); err != nil {
		log.Printf("[instance:%s] 下发 hermes soul_push 失败: %v", instanceId, err)
	}
}

// instanceHermesOpts 组装实例全量 hermes 配置参数：LLM 网关 + 已绑 MCP 分组（仿 instanceOpenclawOpts）。
func (s *AgentInstanceService) instanceHermesOpts(ctx context.Context, instanceId, llmBaseURL, plainKey, defaultModel string) HermesConfigOpts {
	opts := HermesConfigOpts{
		LLMBaseURL: llmBaseURL, APIKey: plainKey, DefaultModel: defaultModel,
	}
	// 已绑 MCP 分组 → mcp_servers（无绑定 nil 不写段；best-effort 不阻断 LLM 配置下发）。
	if s.mcpSvc != nil {
		opts.McpServers = s.mcpSvc.InstanceMcpServersFor(ctx, instanceId, "hermes")
	}
	return opts
}

// pushHermesConfigRaw 入队 config_push（AgentId=hermes）+ reload 时追加 restart，
// 并即时落一条 dispatch 配置变更记录（source 细分场景，dispatcher=下发人）。
// 供 runtime 上报时对比 hash 判 applied/reverted，及配置变更列表溯源。失败仅记日志（best-effort）。
func (s *AgentInstanceService) pushHermesConfigRaw(ctx context.Context, instanceId, issuedBy, source, configContent string, reload bool) {
	if s.cmdRepo == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{"configContent": configContent})
	if _, err := s.cmdRepo.Add(ctx, &entity.AgentInstanceCommand{
		InstanceId: instanceId, AgentId: "hermes", CommandType: "config_push",
		PayloadJson: string(payload), Status: 0, IssuedBy: issuedBy, TimeoutSeconds: 60,
	}); err != nil {
		log.Printf("[instance:%s] 下发 hermes config_push 失败: %v", instanceId, err)
		return
	}
	// 即时落 dispatch 记录：runtime 上报时对比此 hash 判 hermes 是否接受。
	if s.configChangeRepo != nil && source != "" {
		_ = s.configChangeRepo.Insert(ctx, &entity.AgentInstanceConfigChange{
			InstanceId: instanceId, AgentId: "hermes", Source: source, Dispatcher: issuedBy,
			ConfigHash: sha256Hex(configContent), ConfigSize: int64(len(configContent)), Content: configContent,
		})
	}
	if reload {
		// hermes 在跑时 config_push 仅写盘，需 restart 重读新配置。
		if _, err := s.cmdRepo.Add(ctx, &entity.AgentInstanceCommand{
			InstanceId: instanceId, AgentId: "hermes", CommandType: "restart",
			PayloadJson: "{}", Status: 0, IssuedBy: issuedBy, TimeoutSeconds: 60,
		}); err != nil {
			log.Printf("[instance:%s] 下发 hermes restart 失败: %v", instanceId, err)
		}
	}
}

// redeliverHermesConfigFromStored 读最近一次 hermes config_push 的 config.yaml，取 base_url + api_key（ak 不变），
// 重新生成（重新选 default）后重推。模型禁用/参数编辑后刷新受影响 hermes 实例用。
// best-effort：无历史配置/解析失败/缺 key 都不阻断，仅记日志。
func (s *AgentInstanceService) redeliverHermesConfigFromStored(ctx context.Context, instanceId, issuedBy, source string) {
	if s.cmdRepo == nil {
		return
	}
	payloadJSON, err := s.cmdRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil || payloadJSON == "" {
		log.Printf("[instance:%s] 重推 hermes 配置：无历史 config_push: %v", instanceId, err)
		return
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[instance:%s] 重推 hermes 配置：解析 payload 失败: %v", instanceId, err)
		return
	}
	baseURL, apiKey, _, _ := parseHermesConfigKeys(payload.ConfigContent)
	if baseURL == "" || apiKey == "" {
		log.Printf("[instance:%s] 重推 hermes 配置：历史配置缺 base_url/api_key，跳过", instanceId)
		return
	}
	defaultModel, provider, err := FetchHermesDefaultModel(ctx, s.llmModelRepo, baseURL, apiKey)
	if err != nil {
		log.Printf("[instance:%s] 重推 hermes 配置跳过（拉取可用模型失败）: %v", instanceId, err)
		return
	}
	opts := HermesConfigOpts{
		LLMBaseURL:   baseURL,
		APIKey:       apiKey,
		APIMode:      hermesAPIModeForProvider(provider), // 重新选了模型，api_mode 随新模型 provider 走
		DefaultModel: defaultModel,
	}
	// 已绑 MCP 分组照常带上（同 instanceHermesOpts）：重推是全量重建整份 config.yaml，
	// 漏带 mcp_servers 会把 mcp-sync 推的段冲掉——gateway restart 后新会话 agent 按新配置
	// 重建，MCP 工具（demo1 组）全部消失，agent 找不到平台工具只能满容器乱探（踩过）。
	if s.mcpSvc != nil {
		opts.McpServers = s.mcpSvc.InstanceMcpServersFor(ctx, instanceId, "hermes")
	}
	cfgYAML := BuildHermesConfig(opts)
	s.pushHermesConfigRaw(ctx, instanceId, issuedBy, source, cfgYAML, true)
	log.Printf("[instance:%s] 已重推 hermes 配置（ak 不变，default=%s）", instanceId, defaultModel)
}

// SyncHermesMcpSection 把实例已绑 MCP 分组收敛进 hermes config.yaml 的顶层 mcp_servers 段后重推
// （AgentMcpService.SyncInstance 的 hermes 分派，对应 openclaw 的 repushMcpSection）。
// 读最近一次 config_push 复用 base_url/api_key/default——default 有历史值就不碰 LLM 网关
// （mcp 段同步不应因网关不可达而失败）；缺失才现拉，拉不到置空（BuildHermesConfig 省略 default 键）。
// hermes 配置整文件由平台生成、无守卫回滚问题，直接 BuildHermesConfig 全量重建（区别于 openclaw
// 的读改写）；restart=true——hermes 无热重载，mcp_servers 变更须 restart 生效。servers=nil 表示
// 绑定已清空：重建为不带 mcp_servers 段同样重推。
// best-effort：无历史 payload（provision 未完成）跳过，下次全量下发自动带上。
func (s *AgentInstanceService) SyncHermesMcpSection(ctx context.Context, instanceId, issuedBy string, servers map[string]interface{}) error {
	if s.cmdRepo == nil {
		return nil
	}
	payloadJSON, err := s.cmdRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil {
		log.Printf("[instance:%s] 读历史 config_push 失败，跳过 hermes mcp 段同步: %v", instanceId, err)
		return nil
	}
	if payloadJSON == "" {
		log.Printf("[instance:%s] 无历史 config_push，跳过 hermes mcp 段同步（下次全量下发自动带上）", instanceId)
		return nil
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[instance:%s] 解析 config_push payload 失败，跳过 hermes mcp 段同步: %v", instanceId, err)
		return nil
	}
	baseURL, apiKey, defaultModel, apiMode := parseHermesConfigKeys(payload.ConfigContent)
	if servers == nil && !strings.Contains(payload.ConfigContent, "\nmcp_servers:") {
		return nil // 本来就没有 mcp 段且已无绑定，无需重推（省一次 restart 打断会话）
	}
	if baseURL == "" || apiKey == "" {
		log.Printf("[instance:%s] 历史配置缺 base_url/api_key，跳过 hermes mcp 段同步", instanceId)
		return nil
	}
	if defaultModel == "" {
		if m, p, err := FetchHermesDefaultModel(ctx, s.llmModelRepo, baseURL, apiKey); err == nil {
			defaultModel = m
			apiMode = hermesAPIModeForProvider(p)
		} else {
			log.Printf("[instance:%s] 拉取默认模型失败，本次重推不带 default: %v", instanceId, err)
		}
	}
	// default 复用历史值时 apiMode 也复用历史的（不重拉模型列表，没有 provider 可查），丢了会把 anthropic 族重置回 404。
	s.pushHermesConfigRaw(ctx, instanceId, issuedBy, "dispatch:mcp-sync",
		BuildHermesConfig(HermesConfigOpts{
			LLMBaseURL: baseURL, APIKey: apiKey, DefaultModel: defaultModel, APIMode: apiMode, McpServers: servers,
		}), true)
	log.Printf("[instance:%s] 已同步 hermes mcp_servers 并重推配置（%d 个分组）", instanceId, len(servers))
	return nil
}

// ===== agent 分派辅助（配置下发按 agent 路由 openclaw/hermes；alias 去 openclaw 硬编码） =====

// agentAlias 归一化 agent 名（空→openclaw，向后兼容存量实例的 instance:<id>:openclaw key alias）。
func agentAlias(agent string) string {
	if agent == "" {
		return "openclaw"
	}
	return agent
}

// agentDeliversConfig 该 agent 是否由 api 下发运行配置（openclaw/hermes）。agent 空按 openclaw。
func agentDeliversConfig(agent string) bool {
	a := agentAlias(agent)
	return a == "openclaw" || a == "hermes"
}

// deliverAgentConfig 按 agent 分派配置下发（openclaw→openclaw.json，hermes→config.yaml）。
// 仅 openclaw/hermes 下发；其它 agent 无操作。统一 provision / RefreshLLMKey 的下发入口。
// 返回所派发实现的错误（如拉取可用模型失败），由调用方决定冒泡还是记日志。
func (s *AgentInstanceService) deliverAgentConfig(ctx context.Context, agent, instanceId, issuedBy, source, llmBaseURL, plainKey string, reload bool) error {
	switch agentAlias(agent) {
	case "hermes":
		return s.deliverHermesConfig(ctx, instanceId, issuedBy, source, llmBaseURL, plainKey, reload)
	case "openclaw":
		return s.deliverOpenclawConfig(ctx, instanceId, issuedBy, source, llmBaseURL, plainKey, reload)
	}
	return nil
}
