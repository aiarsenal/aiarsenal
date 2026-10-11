package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// llm_models.go — 通用模型拉取（free function，与具体 agent 无关）。
//
// 用 LLM key 调网关 /v1/models 拿【该 key 可用的】模型列表，供各 agent 配置下发共用
//（openclaw 整组 models 列表取数组、hermes 取首个 id 作 default；见各自 agent_*.go）。
// 任何 agent 的配置构造都只应依赖这里，不得自带智能体名的模型拉取副本。

// FetchLLMModels 严格语义：网关不可达/非 2xx/该 key 无可用具体模型 → error，绝不回退假模型。
// 配置下发了就必须能用——无可用模型直接让下发失败并报错（错误冒到操作入口，如实例「刷新密钥」），
// 而不是下发一个含不存在模型的配置、拖到运行期才以 404 暴露（微信回复变错误文案的教训）。
func FetchLLMModels(ctx context.Context, llmModelRepo repository.ILLMModelRepository, baseURL, bearerKey string) ([]interface{}, error) {
	return fetchLLMModels(ctx, llmModelRepo, baseURL, bearerKey)
}

// fetchLLMModels 模型拉取核心：成功返回具体模型列表；任一环节失败返回 error（调用方决定回退还是报错）。
// 只下发 group/model 具体模型（裸 group 池入口留给 /v1/chat/completions 做组内负载均衡，
// 不进下发列表），按 id 去重；再用 aa_llm_model 富化 /v1/models 拿不到的参数
// （name←model_name、cost←四种单价、contextWindow←max_input_tokens、maxTokens←max_output_tokens，
// 0 值不写——各 agent 适配器下的缺字段兜底在各自 Build*Config 里做）。
func fetchLLMModels(ctx context.Context, llmModelRepo repository.ILLMModelRepository, baseURL, bearerKey string) ([]interface{}, error) {
	if baseURL == "" || bearerKey == "" {
		return nil, fmt.Errorf("llmBaseURL 或 key 为空")
	}
	// 预取 aa_llm_model 全表，按 裸 group（池入口）与 group/model（具体）两种 id 建索引，
	// 用于补齐 /v1/models 拿不到的参数。失败不阻断：索引为空时每条 id 走稀疏默认。
	modelByKey := map[string]*entity.LLMModel{}
	if llmModelRepo != nil {
		if rows, e := llmModelRepo.All(ctx); e == nil {
			for _, m := range rows {
				if m == nil || m.ModelGroup == "" {
					continue
				}
				// 池入口（裸 group）：取组内代表模型（首个命中），仅用于预估展示；真实计费按命中部署
				if _, ok := modelByKey[m.ModelGroup]; !ok {
					modelByKey[m.ModelGroup] = m
				}
				// 具体 group/model：精确指向该部署
				if m.ModelName != "" {
					modelByKey[m.ModelGroup+"/"+m.ModelName] = m
				}
			}
		} else {
			log.Printf("[llm] load aa_llm_model for enrichment: %v", e)
		}
	}
	// API 进程自身回连网关时，host.docker.internal（容器专用域名）在宿主机不可直连——本机还会被
	// 代理 fake-ip DNS 劫持成 198.18.x.x 假 IP 黑洞（实测 EOF/超时）。API 与网关同机，这一趟改写回
	// 127.0.0.1；baseURL 本身不动——写进 agent 配置下发给容器的仍是容器可达地址。
	fetchBase := strings.Replace(baseURL, "://host.docker.internal", "://127.0.0.1", 1)
	url := strings.TrimRight(fetchBase, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 %s 请求: %w", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerKey)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("网关不可达 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("网关返回 %s status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应: %w", err)
	}
	var parsed struct {
		Data []struct {
			Id      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析响应: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("网关 %s 返回空模型列表（该 key 无可用模型）", url)
	}
	models := make([]interface{}, 0, len(parsed.Data))
	seen := map[string]bool{} // 同 id 多部署（如多 credential 都挂 auto/glm-5.2）会返回多次，按 id 去重
	for _, d := range parsed.Data {
		if d.Id == "" || seen[d.Id] {
			continue
		}
		seen[d.Id] = true
		isComposite := strings.Contains(d.Id, "/") // 具体模型 vs 池入口（裸 group）
		if !isComposite {
			// 裸 group（池入口，如 "auto"）不下发：agent 配置须具体到模型名（group/model），
			// 裸池入口留给 /v1/chat/completions 做组内负载均衡，不进下发模型列表。
			continue
		}
		item := map[string]interface{}{
			"id":        d.Id,
			"name":      d.Id,
			"provider":  d.OwnedBy,
			"reasoning": false,
			"input":     []string{"text"},
		}
		// /v1/models 拿不到的参数：从 aa_llm_model 按 id 补齐。
		if m, ok := modelByKey[d.Id]; ok && m != nil {
			if m.ModelName != "" {
				item["name"] = m.ModelName
			}
			// provider 兜底：/v1/models 未带 owned_by 时从 aa_llm_model 补——
			// hermes 的 api_mode 按它分流协议端点（anthropic→/v1/messages），缺失会错走 chat_completions 404。
			if p, _ := item["provider"].(string); p == "" {
				item["provider"] = m.Provider
			}
			// 对外保持 $/token 语义（消费方兼容）：毫/百万 tokens → $/token = 毫 / 1e10。
			item["cost"] = map[string]interface{}{
				"input":      float64(m.InputPricePerMillion) / 1e10,
				"output":     float64(m.OutputPricePerMillion) / 1e10,
				"cacheRead":  float64(m.CacheReadPricePerMillion) / 1e10,
				"cacheWrite": float64(m.CacheCreationPricePerMillion) / 1e10,
			}
			if m.MaxInputTokens > 0 {
				item["contextWindow"] = m.MaxInputTokens
			}
			if m.MaxOutputTokens > 0 {
				item["maxTokens"] = m.MaxOutputTokens
			}
		}
		models = append(models, item)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("网关 %s 只返回池入口（裸 group）、无具体 group/model 模型", url)
	}
	return models, nil
}

// FirstModelID 从 FetchLLMModels 返回的 models 列表取首个 id（作下发配置里的默认模型用）。
func FirstModelID(models []interface{}) string {
	for _, m := range models {
		if mm, ok := m.(map[string]interface{}); ok {
			if id, _ := mm["id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

// HasDeployableLLMModel 平台侧快速预检（不走网关、无需 key）：aa_llm_model 是否存在至少一个
// 可下发的具体模型——status=1 启用且带具体 model_name（裸 group 池入口不算，语义对齐
// fetchLLMModels 只下发 group/model）。供「创建实例」等入口在建记录前预判：无可用模型直接失败，
// 避免起一个拿不到模型的实例、拖到运行期才 404。与网关 AllModels 的启用口径一致
// （deleted_at is null and blocked=0 and status=1；repo.All 已滤前两条，这里补 status）。
func HasDeployableLLMModel(ctx context.Context, llmModelRepo repository.ILLMModelRepository) (bool, error) {
	if llmModelRepo == nil {
		return true, nil // 未接模型库的部署形态不拦截
	}
	rows, err := llmModelRepo.All(ctx)
	if err != nil {
		return false, err
	}
	for _, m := range rows {
		if m != nil && m.Status == 1 && m.ModelName != "" {
			return true, nil
		}
	}
	return false, nil
}
