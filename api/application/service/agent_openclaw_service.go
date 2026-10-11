package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"aiarsenal/domain/entity"
)

// agent_openclaw_service.go — 实例（image/manager）场景的 openclaw 下发。
//
// 仅实例侧的 dispatch 包装：构造 openclaw.json（调共享 agent_openclaw.go 的 FetchLLMModels +
// BuildOpenclawConfig）→ 经 config_push（reload=true 时追加 restart）下发给容器内 manager。
// 共享 openclaw 逻辑（模型拉取、配置构造）在 agent_openclaw.go；桌面设备场景在 desktop_service.go。

// deliverOpenclawConfig 向容器内 manager 下发 openclaw.json。
// reload=false（首次创建）：只下 config_push——executor 写盘后 agent 未运行会自动拉起（先配置后启动，
// 带正确配置首次起）。**不下 restart**：config_push 已拉起 openclaw，多余 restart 的 `openclaw gateway
// restart` 在刚启动的 gateway 上易卡超时（60s）反而把进程搞死。
// reload=true（刷新密钥）：openclaw 已在跑，config_push 仅写盘（不会重启），需额外 restart 重读新 key。
// 模型拉取失败（网关不可达/无可用模型）→ error：不下发含假 default 模型的配置，错误冒到调用入口。
func (s *AgentInstanceService) deliverOpenclawConfig(ctx context.Context, instanceId, issuedBy, source, llmBaseURL, plainKey string, reload bool) error {
	models, err := FetchLLMModels(ctx, s.llmModelRepo, llmBaseURL, plainKey)
	if err != nil {
		log.Printf("[instance:%s] 下发 openclaw 配置跳过（拉取可用模型失败）: %v", instanceId, err)
		return fmt.Errorf("拉取可用模型失败（请先在【LLM 管理】配置模型部署）: %w", err)
	}
	defaultsModel := ""
	if firstID := FirstModelID(models); firstID != "" {
		defaultsModel = "aiarsenal/" + firstID
	}
	cfgJSON := BuildOpenclawConfig(s.instanceOpenclawOpts(ctx, instanceId, llmBaseURL, plainKey, models, defaultsModel))
	s.pushOpenclawConfigRaw(ctx, instanceId, issuedBy, source, cfgJSON, reload)
	log.Printf("[instance:%s] 已下发 openclaw 配置（config_push%s，baseUrl=%s，default=%s）", instanceId, ifRestart(reload), llmBaseURL, defaultsModel)
	return nil
}

// instanceOpenclawOpts 组装实例全量 openclaw 配置参数：LLM 网关 + 微信渠道 + 已绑 MCP 分组。
func (s *AgentInstanceService) instanceOpenclawOpts(ctx context.Context, instanceId, llmBaseURL, plainKey string, models []interface{}, defaultsModel string) OpenclawConfigOpts {
	opts := OpenclawConfigOpts{
		LLMBaseURL:    llmBaseURL,
		APIKey:        plainKey,
		API:           OpenclawProviderAPI(ctx, s.llmModelRepo),
		Models:        models,
		Workspace:     "/aa/.openclaw/workspace",
		DefaultsModel: defaultsModel,
		// 声明微信渠道：channels login 须在 openclaw.json 见到已配置渠道才走非交互出码路径，
		// 否则弹「Install plugin?」交互提示、容器内无 TTY 直接 exit 13。插件由镜像预装（Dockerfile）。
		Channels: map[string]interface{}{"openclaw-weixin": map[string]interface{}{
			"dmPolicy": "open", "allowFrom": []interface{}{"*"}, "groupPolicy": "open",
		}},
		PluginsEntries: map[string]interface{}{"openclaw-weixin": map[string]interface{}{"enabled": true}},
	}
	// 已绑 MCP 分组 → mcp.servers（无绑定 nil 不写段；best-effort 不阻断 LLM 配置下发）。
	if s.mcpSvc != nil {
		opts.McpServers = s.mcpSvc.InstanceMcpServersFor(ctx, instanceId, "openclaw")
	}
	return opts
}

// pushOpenclawConfigRaw 入队 config_push（configContent 由调用方提供）+ reload 时追加 restart，
// 并即时落一条 dispatch 配置变更记录（source 细分场景，dispatcher=下发人）。
// 供 runtime 上报时对比 hash 判 applied/reverted，及配置变更列表溯源。失败仅记日志（best-effort）。
// source 取值：dispatch:create / dispatch:key-rotate / dispatch:model-edit / dispatch:model-offline。
func (s *AgentInstanceService) pushOpenclawConfigRaw(ctx context.Context, instanceId, issuedBy, source, configContent string, reload bool) {
	if s.cmdRepo == nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{"configContent": configContent})
	if _, err := s.cmdRepo.Add(ctx, &entity.AgentInstanceCommand{
		InstanceId: instanceId, AgentId: "openclaw", CommandType: "config_push",
		PayloadJson: string(payload), Status: 0, IssuedBy: issuedBy, TimeoutSeconds: 60,
	}); err != nil {
		log.Printf("[instance:%s] 下发 config_push 失败: %v", instanceId, err)
		return
	}
	// 即时落 dispatch 记录：runtime 上报时对比此 hash 判 openclaw 是否接受（applied/reverted）。
	if s.configChangeRepo != nil && source != "" {
		_ = s.configChangeRepo.Insert(ctx, &entity.AgentInstanceConfigChange{
			InstanceId: instanceId, AgentId: "openclaw", Source: source, Dispatcher: issuedBy,
			ConfigHash: sha256Hex(configContent), ConfigSize: int64(len(configContent)), Content: configContent,
		})
	}
	if reload {
		// openclaw 在跑时 config_push 仅写盘，需 restart 重读新配置。
		if _, err := s.cmdRepo.Add(ctx, &entity.AgentInstanceCommand{
			InstanceId: instanceId, AgentId: "openclaw", CommandType: "restart",
			PayloadJson: "{}", Status: 0, IssuedBy: issuedBy, TimeoutSeconds: 60,
		}); err != nil {
			log.Printf("[instance:%s] 下发 restart 失败: %v", instanceId, err)
		}
	}
}

// RemoveModelFromInstanceConfig 读实例最近一次 config_push 的配置（含 ak），删掉 modelId 这条模型条目，
// 重新 config_push（ak 与其它模型条目原样保留）。模型禁用/删除后刷新受影响实例的本地清单用。
// best-effort：无历史配置/解析失败/配置中无该模型 都不阻断，仅记日志。
func (s *AgentInstanceService) RemoveModelFromInstanceConfig(ctx context.Context, instanceId, modelId, issuedBy string) {
	if s.cmdRepo == nil || modelId == "" {
		return
	}
	payloadJSON, err := s.cmdRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil {
		log.Printf("[instance:%s] 读历史 config_push 失败，跳过移除模型 %s: %v", instanceId, modelId, err)
		return
	}
	if payloadJSON == "" {
		log.Printf("[instance:%s] 无历史 config_push，跳过移除模型 %s", instanceId, modelId)
		return
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[instance:%s] 解析 config_push payload 失败，跳过移除模型 %s: %v", instanceId, modelId, err)
		return
	}
	newCfg, removed, err := removeModelFromOpenclawConfig(payload.ConfigContent, modelId)
	if err != nil {
		log.Printf("[instance:%s] 改配置移除模型 %s 失败: %v", instanceId, modelId, err)
		return
	}
	if !removed {
		log.Printf("[instance:%s] 配置中无模型 %s，无需重推", instanceId, modelId)
		return
	}
	s.pushOpenclawConfigRaw(ctx, instanceId, issuedBy, "dispatch:model-offline", newCfg, true)
	log.Printf("[instance:%s] 已移除模型 %s 并重推 openclaw 配置（ak 不变）", instanceId, modelId)
}

// RedeliverInstanceOpenclawConfig 读实例最近一次 config_push 的【现有配置】，在其上用最新 DB 重新
// 富化 models 列表（maxTokens/contextWindow/name 等随模型编辑变）后覆盖回 models 字段，其余字段
// （gateway/agents/channels/plugins/meta 等）原样保留 → 重推（ak 不变，不轮换）。
// 在现有配置上改而非重新生成：保留 openclaw 自加的 meta 等字段，避免守卫 missing-meta-vs-last-good 回滚。
// 模型参数编辑（max_input/output_tokens 等）后刷新受影响实例用。best-effort。
func (s *AgentInstanceService) RedeliverInstanceOpenclawConfig(ctx context.Context, instanceId, issuedBy string) {
	if s.cmdRepo == nil {
		return
	}
	payloadJSON, err := s.cmdRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil || payloadJSON == "" {
		log.Printf("[instance:%s] 重推 openclaw 配置：无历史 config_push: %v", instanceId, err)
		return
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[instance:%s] 重推 openclaw 配置：解析 payload 失败: %v", instanceId, err)
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(payload.ConfigContent), &cfg); err != nil {
		log.Printf("[instance:%s] 重推 openclaw 配置：解析历史配置失败: %v", instanceId, err)
		return
	}
	prov := nestedMap(cfg, "models", "providers", "aiarsenal")
	if prov == nil {
		log.Printf("[instance:%s] 重推 openclaw 配置：历史配置无 aiarsenal provider", instanceId)
		return
	}
	// 只更新 models：用最新 DB 重新 FetchLLMModels 富化，覆盖回 providers.aiarsenal.models。
	// 其余字段（含 openclaw 自加的 meta）原样保留。
	rawBase, _ := prov["baseUrl"].(string)
	apiKey, _ := prov["apiKey"].(string)
	models, merr := FetchLLMModels(ctx, s.llmModelRepo, strings.TrimSuffix(rawBase, "/v1"), apiKey)
	if merr != nil {
		log.Printf("[instance:%s] 重推 openclaw 配置跳过（拉取可用模型失败）: %v", instanceId, merr)
		return
	}
	prov["models"] = models
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("[instance:%s] 重推 openclaw 配置：序列化失败: %v", instanceId, err)
		return
	}
	s.pushOpenclawConfigRaw(ctx, instanceId, issuedBy, "dispatch:model-edit", string(out), true)
	log.Printf("[instance:%s] 已在现有配置上更新模型参数并重推（ak/meta 不变）", instanceId)
}

// removeModelFromOpenclawConfig 从 openclaw.json 文本里删掉 models.providers.aiarsenal.models 中 id==modelId 的条目，
// 返回（新配置文本, 是否删到, err）。若删的正是 agents.defaults.model 指向的模型，顺带把 defaults 改成剩余首个。
func removeModelFromOpenclawConfig(cfgText, modelId string) (string, bool, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(cfgText), &cfg); err != nil {
		return "", false, err
	}
	prov := nestedMap(cfg, "models", "providers", "aiarsenal")
	if prov == nil {
		return "", false, nil
	}
	arr, ok := prov["models"].([]interface{})
	if !ok || len(arr) == 0 {
		return "", false, nil
	}
	next := make([]interface{}, 0, len(arr))
	removed := false
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if id, _ := m["id"].(string); id == modelId {
				removed = true
				continue
			}
		}
		next = append(next, item)
	}
	if !removed {
		return "", false, nil
	}
	prov["models"] = next
	// 默认模型若指向被删的，改成剩余首个，避免 openclaw 起不来默认模型。
	if def := nestedMap(cfg, "agents", "defaults"); def != nil {
		if dm, _ := def["model"].(string); dm == "aiarsenal/"+modelId || dm == modelId {
			if first := firstRemainingModelID(next); first != "" {
				def["model"] = "aiarsenal/" + first
			}
		}
	}
	// 缩进格式必须与 BuildOpenclawConfig (agent_openclaw.go:212) 的 MarshalIndent 保持一致。
	// 若用紧凑 json.Marshal，字节总数相对 openclaw 维护的缩进 last-good baseline 会腰斩，
	// 删一个模型时叠加格式差会跌破 size-drop-vs-last-good 的 50% 阈值，配置被守卫回滚。
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out), true, nil
}

// nestedMap 按 keys 逐层下钻 map[string]interface{}，任一层缺失/类型不符返回 nil。
func nestedMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	cur := m
	for _, k := range keys {
		v, ok := cur[k]
		if !ok {
			return nil
		}
		cur, ok = v.(map[string]interface{})
		if !ok {
			return nil
		}
	}
	return cur
}

// firstRemainingModelID 取 models 数组里首个条目的 id（删模型后重置默认模型用）。
func firstRemainingModelID(arr []interface{}) string {
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if id, _ := m["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

func ifRestart(reload bool) string {
	if reload {
		return "+restart"
	}
	return ""
}
