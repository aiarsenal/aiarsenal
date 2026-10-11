package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"aiarsenal/llm/internal/auth"
	"aiarsenal/llm/internal/model"
	"aiarsenal/llm/internal/notify"
	"aiarsenal/llm/internal/provider"
	"aiarsenal/llm/internal/routing"
	"aiarsenal/llm/internal/store"
)

// Handler LLM 网关请求处理（OpenAI 兼容）。
type Handler struct {
	st       *store.Store
	registry *provider.Registry
	strategy routing.Strategy
	httpc    *http.Client
	// firstByteTimeout 上游首字节耐心（从发请求起算，覆盖响应头 + 首个 body 字节两段静默）。
	firstByteTimeout time.Duration
}

func New(st *store.Store, reg *provider.Registry, strat routing.Strategy) *Handler {
	return &Handler{st: st, registry: reg, strategy: strat,
		httpc: &http.Client{Timeout: 10 * time.Minute}, firstByteTimeout: firstByteTimeoutFromEnv()}
}

// openaiCompatibleProviders 共用 OpenAI 协议、经 /v1/chat/completions 入口可达的 provider 集合。
// OpenAI 兼容族统一适配；其余原生协议需各自端点——gemini 走 /v1beta/models/{model}:generateContent
//（gemini.go，入站 Gemini 请求同样翻成本族跑）、bedrock/vertex_ai 等暂未实现。
var openaiCompatibleProviders = []string{"openai", "deepseek", "moonshot", "groq", "openrouter", "ollama"}

// ChatCompletions POST /v1/chat/completions。
func (h *Handler) ChatCompletions(c *gin.Context) { h.proxyChat(c, "chat") }

// Messages POST /v1/messages —— Anthropic Messages 协议客户端端点。
// 原生透传（仅把 model 重写成命中部署的 provider_model）：请求与响应保持 Anthropic 原生格式——
// thinking 块 / system / tools / SSE 事件流原样保留，流式请求透传 SSE（事件里解析 usage 计费）。
// 之前 anthropic→openai→anthropic 双重转换丢了 thinking 块、且不支持流式（stream:true 被忽略，
// 流式客户端拿到非流式 JSON 解析不出内容）。
func (h *Handler) Messages(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "read body: " + err.Error()}})
		return
	}
	reqModel, stream := parseModelAndStream(raw)
	k := c.MustGet("key").(*auth.Key)
	hash := c.GetString("token_hash")
	if !auth.ModelAllowed(k.VirtualKey, reqModel) {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "model_not_allowed", "message": "Model not allowed for this key"}})
		return
	}
	// /v1/messages 入口 → provider 集 [anthropic]
	candidates, err := h.st.ListModelsByGroup(c, reqModel, []string{"anthropic"})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "no model deployed for: " + reqModel}})
		return
	}
	// 按 ak 的 allowed_models 收窄：组池分发时只在 ak 支持的模型间分发
	candidates = auth.FilterAllowedModels(candidates, k.VirtualKey)
	if len(candidates) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "no model deployed for: " + reqModel}})
		return
	}
	// 单请求内跨凭据 failover（同 proxyChat）：逐候选尝试，成功即返回；全失败透传最后一次上游错误。
	tried := map[string]struct{}{}
	last := attemptResult{failStatus: http.StatusServiceUnavailable, failBody: []byte(`{"error":{"message":"no deployment succeeded"}}`)}
	for {
		m, perr := h.strategy.PickExcluding(c, candidates, tried)
		if m == nil || perr != nil {
			break
		}
		tried[m.ModelId] = struct{}{}
		last = h.attemptMessages(c, m, raw, stream, k.VirtualKey, hash)
		if last.committed {
			return
		}
	}
	writeAttemptFailure(c, last)
}

// attemptMessages /v1/messages（anthropic）单次尝试：原生透传。
// 重写 model 为部署的 provider_model 后转发上游；非流式响应原样回写、流式 SSE 逐行转发，
// 两者都从响应里解析 usage 计费。失败（凭据缺失/上游 4xx 5xx/网络错）不写客户端，返回失败供 failover 续试。
func (h *Handler) attemptMessages(c *gin.Context, m *model.Model, raw []byte, stream bool, k *model.VirtualKey, hash string) attemptResult {
	reqID := uuid.NewString()
	start := time.Now()
	apiKey, cred, err := h.st.GetCredentialAPIKey(c, m.CredentialId)
	if err != nil {
		h.recordError(c, m, k, hash, reqID, start, "chat", "", "credential_missing", err.Error(), "503", capStr(string(raw)))
		return attemptResult{failStatus: http.StatusServiceUnavailable, failBody: []byte("credential missing: " + err.Error())}
	}
	// 只重写 model（客户端传 group/model 复合 id，上游只认 provider_model）；其余字段原样透传。
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return attemptResult{failStatus: http.StatusBadRequest, failBody: []byte("invalid anthropic request: " + err.Error())}
	}
	if m.ProviderModel != "" {
		b, _ := json.Marshal(m.ProviderModel)
		body["model"] = b
	}
	upBody, _ := json.Marshal(body)
	baseURL := cred.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL("anthropic")
	}
	url := provider.AnthropicProvider{}.UpstreamURL(baseURL, "/v1/messages")
	auth.TouchActive(h.st, hash)
	resp, br, cancelUpstream, err := h.doUpstream(c, http.MethodPost, url, map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}, upBody)
	if err != nil {
		if resp == nil {
			h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_error", err.Error(), "502", capStr(string(raw)))
			return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error()), rawUpstream: true}
		}
		// 首字节没等到（超时/零字节空流）：不伪装成成功空流，按上游失败走 failover。
		h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_no_data", err.Error(), "502", capStr(string(raw)))
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, 0, 0, 0, "failed", capStr(string(raw)), "")
		return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error())}
	}
	defer cancelUpstream()
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(br)
		h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_http_error", capStr(string(respBody)), fmt.Sprintf("%d", resp.StatusCode), capStr(string(raw)))
		if cd := parseRetryAfter(resp.Header.Get("Retry-After")); cd > 0 {
			h.strategy.MarkCooldownFor(c, m.ModelId, cd) // 上游建议的退避时长覆盖默认冷却
		}
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, 0, 0, 0, "failed", capStr(string(raw)), capStr(string(respBody)))
		return attemptResult{failStatus: resp.StatusCode, failBody: respBody, rawUpstream: true}
	}
	if stream {
		// SSE 透传：tee 逐行写客户端 + 缓冲（spend_log 记录输出），data: 行解析 usage 计费。
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.WriteHeader(resp.StatusCode)
		var sbuf bytes.Buffer
		usage := copyAnthropicSSE(br, c.Writer, &sbuf)
		cost := model.CostMilli(usage.PromptTokens, usage.CompletionTokens, m.InputPricePerMillion, m.OutputPricePerMillion)
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, cost, usage.PromptTokens, usage.CompletionTokens, "success", capStr(string(raw)), capStr(sbuf.String()))
		return attemptResult{committed: true}
	}
	// 非流式：响应体原样回写，usage 从 JSON 解析。
	respBody, _ := io.ReadAll(br)
	var nr struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(respBody, &nr)
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(respBody)
	var pt, ot int
	if nr.Usage != nil {
		pt, ot = nr.Usage.InputTokens, nr.Usage.OutputTokens
	}
	cost := model.CostMilli(pt, ot, m.InputPricePerMillion, m.OutputPricePerMillion)
	h.bill(c, m, k, hash, reqID, start, "chat", baseURL, cost, pt, ot, "success", capStr(string(raw)), capStr(string(respBody)))
	return attemptResult{committed: true}
}

// copyAnthropicSSE 把上游 Anthropic SSE 逐行拷贝到客户端 w（遇 Flush 即时下发）与 buf（spend_log），
// 并从事件 data 行解析 usage：message_start.message.usage.input_tokens、message_delta.usage.output_tokens。
// r 是 doUpstream 预读首字节的 bufio.Reader，这里直接续读。
func copyAnthropicSSE(br *bufio.Reader, w http.ResponseWriter, buf *bytes.Buffer) provider.Usage {
	flusher, _ := w.(http.Flusher)
	var u provider.Usage
	for {
		line, rerr := br.ReadString('\n')
		if line != "" {
			buf.WriteString(line)
			_, _ = io.WriteString(w, line)
			if flusher != nil {
				flusher.Flush()
			}
			if trimmed := strings.TrimSpace(strings.TrimPrefix(line, "data:")); trimmed != "" && strings.HasPrefix(strings.TrimSpace(line), "data:") {
				var ev struct {
					Type    string `json:"type"`
					Message *struct {
						Usage *struct {
							InputTokens          int `json:"input_tokens"`
							CacheReadInputTokens int `json:"cache_read_input_tokens"`
							CacheCreationTokens  int `json:"cache_creation_input_tokens"`
						} `json:"usage"`
					} `json:"message"`
					Usage *struct {
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(trimmed), &ev) == nil {
					if ev.Message != nil && ev.Message.Usage != nil {
						u.PromptTokens = ev.Message.Usage.InputTokens
						u.CacheReadTokens = ev.Message.Usage.CacheReadInputTokens
						u.CacheCreateTokens = ev.Message.Usage.CacheCreationTokens
					}
					if ev.Usage != nil && ev.Usage.OutputTokens > 0 {
						u.CompletionTokens = ev.Usage.OutputTokens
					}
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	return u
}

// Embeddings POST /v1/embeddings。
func (h *Handler) Embeddings(c *gin.Context) { h.proxyChat(c, "embedding") }

// Models GET /v1/models —— 返回可见的调用 id：每个分组池（裸 group）+ 每个 group/model 具体模型。
// provider 无关（同一 id 跨 provider 去重为一条），调用方靠入口端点（/chat/completions→openai、/messages→anthropic）选 provider。
func (h *Handler) Models(c *gin.Context) {
	list, err := h.st.AllModels(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	type entry struct{ id, owner string }
	seen := map[string]bool{}
	out := make([]entry, 0, len(list)*2)
	for _, m := range list {
		if m.ModelGroup == "" {
			continue
		}
		// 分组池入口（裸 group）：组内固定 provider 下所有 model 负载分发
		if !seen[m.ModelGroup] {
			seen[m.ModelGroup] = true
			out = append(out, entry{m.ModelGroup, m.Provider})
		}
		// 具体 group/model
		if m.ModelName != "" {
			composite := m.ModelGroup + "/" + m.ModelName
			if !seen[composite] {
				seen[composite] = true
				out = append(out, entry{composite, m.Provider})
			}
		}
	}
	// 按调用方 key 的 allowed_models 过滤（空/["all"]=不限制，与 chat 的 ModelAllowed 同逻辑）。
	// 否则 /v1/models 永远返全量，下游（如实例 openclaw 配置经 FetchOpenclawModels）拿不到受限清单，
	// 表现成"改了 allowed_models 但实例可用模型没变"。
	k := c.MustGet("key").(*auth.Key)
	data := make([]gin.H, 0, len(out))
	for _, e := range out {
		if auth.ModelAllowed(k.VirtualKey, e.id) {
			data = append(data, gin.H{"id": e.id, "object": "model", "owned_by": e.owner})
		}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (h *Handler) proxyChat(c *gin.Context, callType string) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "read body: " + err.Error()}})
		return
	}
	reqModel, stream := parseModelAndStream(body)
	k := c.MustGet("key").(*auth.Key)
	hash := c.GetString("token_hash")

	if !auth.ModelAllowed(k.VirtualKey, reqModel) {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "model_not_allowed", "message": "Model not allowed for this key"}})
		return
	}
	// /v1/chat/completions、/v1/embeddings 入口 → openai 兼容族（openai/deepseek/moonshot/groq/openrouter/ollama）
	candidates, err := h.st.ListModelsByGroup(c, reqModel, openaiCompatibleProviders)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "no model deployed for: " + reqModel}})
		return
	}
	// 按 ak 的 allowed_models 收窄：组池分发时只在 ak 支持的模型间分发
	candidates = auth.FilterAllowedModels(candidates, k.VirtualKey)
	if len(candidates) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "no model deployed for: " + reqModel}})
		return
	}
	// 单请求内跨凭据 failover：逐个尝试候选（跳过已试过 + 冷却），成功即返回；
	// 全失败透传最后一次上游错误（保留真实 status/body，让客户端看到真正的 429 而非笼统 503）。
	tried := map[string]struct{}{}
	last := attemptResult{failStatus: http.StatusServiceUnavailable, failBody: []byte(`{"error":{"message":"no deployment succeeded"}}`)}
	for {
		m, perr := h.strategy.PickExcluding(c, candidates, tried)
		if m == nil || perr != nil {
			break
		}
		tried[m.ModelId] = struct{}{}
		last = h.attemptChat(c, m, body, stream, k, hash, callType)
		if last.committed {
			return
		}
	}
	writeAttemptFailure(c, last)
}

// attemptResult 一次上游尝试的结果。committed=true 表示已向客户端写完响应（成功），调用方停止重试；
// 否则 failStatus/failBody 供 failover 最终回写——rawUpstream=true 时 failBody 是上游原始响应体（直传，
// 保留真实 429 错误体），false 时为内部错误（包成 JSON error 回写）。
type attemptResult struct {
	committed   bool
	failStatus  int
	failBody    []byte
	rawUpstream bool
}

// attemptChat 对单个候选做一次完整尝试：取凭据 → 构上游请求 → 按流式/非流式调上游。
// 成功（已写客户端）返回 committed=true；失败（凭据缺失/上游 4xx 5xx/网络错）不写客户端，返回失败供 failover 续试。
func (h *Handler) attemptChat(c *gin.Context, m *model.Model, body []byte, stream bool, k *auth.Key, hash, callType string) attemptResult {
	reqID := uuid.NewString()
	start := time.Now()
	apiKey, cred, err := h.st.GetCredentialAPIKey(c, m.CredentialId)
	if err != nil {
		h.recordError(c, m, k.VirtualKey, hash, reqID, start, callType, "", "credential_missing", err.Error(), "503", capStr(string(body)))
		return attemptResult{failStatus: http.StatusServiceUnavailable, failBody: []byte("credential missing: " + err.Error())}
	}
	p := h.registry.Pick(m.Provider)
	baseURL := cred.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(m.Provider)
	}
	provReq := provider.Request{Body: body, Stream: stream, APIKey: apiKey, BaseURL: baseURL, ProviderModel: m.ProviderModel}
	url, method, headers, upBody, err := p.BuildUpstream(c, provReq)
	if err != nil {
		h.recordError(c, m, k.VirtualKey, hash, reqID, start, callType, baseURL, "build_upstream_error", err.Error(), "500", capStr(string(body)))
		return attemptResult{failStatus: http.StatusInternalServerError, failBody: []byte(err.Error())}
	}
	auth.TouchActive(h.st, hash)
	if stream {
		return h.attemptStream(c, p, provReq, method, url, headers, upBody, m, k.VirtualKey, hash, reqID, start, callType, baseURL)
	}
	return h.attemptNonStream(c, p, provReq, method, url, headers, upBody, m, k.VirtualKey, hash, reqID, start, callType, baseURL)
}

// attemptNonStream 非流式单次尝试。成功写响应并计费；上游 4xx/5xx 或网络错时不写客户端，返回失败供 failover。
func (h *Handler) attemptNonStream(c *gin.Context, p provider.Provider, provReq provider.Request, method, url string, headers map[string]string, body []byte,
	m *model.Model, k *model.VirtualKey, hash, reqID string, start time.Time, callType, apiBase string) attemptResult {
	resp, br, cancelUpstream, err := h.doUpstream(c, method, url, headers, body)
	if err != nil {
		if resp == nil {
			h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_error", err.Error(), "502", capStr(string(body)))
			return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error()), rawUpstream: true}
		}
		// 首字节没等到（超时/零字节空体）：不伪装成成功空响应，按上游失败走 failover。
		h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_no_data", err.Error(), "502", capStr(string(body)))
		h.bill(c, m, k, hash, reqID, start, callType, apiBase, 0, 0, 0, "failed", capStr(string(provReq.Body)), "")
		return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error())}
	}
	defer cancelUpstream()
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(br)
	if resp.StatusCode >= 400 {
		// 上游 4xx/5xx（HTTP 正常、body 是业务错误）：记错误日志 + 计费 failed，不写客户端，换候选重试。
		h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_http_error", capStr(string(respBody)), fmt.Sprintf("%d", resp.StatusCode), capStr(string(body)))
		if cd := parseRetryAfter(resp.Header.Get("Retry-After")); cd > 0 {
			h.strategy.MarkCooldownFor(c, m.ModelId, cd) // 上游建议的退避时长覆盖默认冷却
		}
		h.bill(c, m, k, hash, reqID, start, callType, apiBase, 0, 0, 0, "failed", capStr(string(provReq.Body)), capStr(string(respBody)))
		return attemptResult{failStatus: resp.StatusCode, failBody: respBody, rawUpstream: true}
	}
	out, usage, _ := p.TransformResponse(respBody, provReq)
	if out == nil {
		out = respBody
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(out)
	cost := model.CostMilli(usage.PromptTokens, usage.CompletionTokens, m.InputPricePerMillion, m.OutputPricePerMillion)
	h.bill(c, m, k, hash, reqID, start, callType, apiBase, cost, usage.PromptTokens, usage.CompletionTokens, "success", capStr(string(provReq.Body)), capStr(string(out)))
	return attemptResult{committed: true}
}

// attemptStream 流式单次尝试。关键：4xx/5xx（错误 JSON 非 SSE）时不提交流、不写 header，返回失败换候选重试；
// 拿到 200 才 WriteHeader + TransformStream（此时已确定成功，本请求不再 failover）。
func (h *Handler) attemptStream(c *gin.Context, p provider.Provider, provReq provider.Request, method, url string, headers map[string]string, body []byte,
	m *model.Model, k *model.VirtualKey, hash, reqID string, start time.Time, callType, apiBase string) attemptResult {
	resp, br, cancelUpstream, err := h.doUpstream(c, method, url, headers, body)
	if err != nil {
		if resp == nil {
			h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_error", err.Error(), "502", capStr(string(body)))
			return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error()), rawUpstream: true}
		}
		// 首字节没等到（超时/零字节空流）：不伪装成成功空流，按上游失败走 failover。
		h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_no_data", err.Error(), "502", capStr(string(body)))
		h.bill(c, m, k, hash, reqID, start, callType, apiBase, 0, 0, 0, "failed", capStr(string(provReq.Body)), "")
		return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error())}
	}
	defer cancelUpstream()
	defer resp.Body.Close()
	// 上游 4xx/5xx：响应体是错误 JSON 而非 SSE，未向客户端写任何字节，可安全换候选重试。
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(br)
		h.recordError(c, m, k, hash, reqID, start, callType, apiBase, "upstream_http_error", capStr(string(errBody)), fmt.Sprintf("%d", resp.StatusCode), capStr(string(body)))
		if cd := parseRetryAfter(resp.Header.Get("Retry-After")); cd > 0 {
			h.strategy.MarkCooldownFor(c, m.ModelId, cd) // 上游建议的退避时长覆盖默认冷却
		}
		h.bill(c, m, k, hash, reqID, start, callType, apiBase, 0, 0, 0, "failed", capStr(string(provReq.Body)), capStr(string(errBody)))
		return attemptResult{failStatus: resp.StatusCode, failBody: errBody, rawUpstream: true}
	}
	// 200：提交流式响应（已确定成功，本请求不再 failover）。tee 把流分片同时写客户端与缓冲，供 spend_log 记录输出。
	c.Writer.WriteHeader(resp.StatusCode)
	var sbuf bytes.Buffer
	tee := &teeRespWriter{ResponseWriter: c.Writer, buf: &sbuf}
	usage, _ := p.TransformStream(br, tee, provReq)
	cost := model.CostMilli(usage.PromptTokens, usage.CompletionTokens, m.InputPricePerMillion, m.OutputPricePerMillion)
	h.bill(c, m, k, hash, reqID, start, callType, apiBase, cost, usage.PromptTokens, usage.CompletionTokens, "success", capStr(string(provReq.Body)), capStr(sbuf.String()))
	return attemptResult{committed: true}
}

// doUpstream 发上游请求并等到第一个字节才返回，把 wedged 上游从成功路径里剥离：
// 响应头迟迟不来（Do 阻塞）或 200 后 body 零字节静默/干净关流，统一由首字节超时
// cancel 上游请求并报错——此时还没向客户端写任何字节，调用方可安全走 failover 换候选
// （客户端看到的是 5xx 而非伪装成正常结束的空流，其现成重试逻辑能接住）。
// 返回的 bufio.Reader 已含预读字节，供错误体 ReadAll / SSE 透传续读。
//
// 返回的 cancel 是请求 context 的取消函数，所有权移交调用方：成功路径这里绝不
// cancel——cancel 会立刻掐断响应体，流式响应只能透传出 Peek 时恰好缓冲到的前几段
// 就半途夭折（9/7 踩过：所有流在首个 thinking delta 后被腰斩，agent 空转重试 7 轮）。
// 调用方读完体（Body.Close / copy 结束）后 defer cancelUpstream() 收尾；错误路径内部
// 已 cancel，再调一次幂等无害。
func (h *Handler) doUpstream(parent context.Context, method, url string, headers map[string]string, body []byte) (*http.Response, *bufio.Reader, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(parent)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, nil, cancel, err
	}
	for kk, vv := range headers {
		req.Header.Set(kk, vv)
	}
	timer := time.AfterFunc(h.firstByteTimeout, cancel)
	defer timer.Stop()
	resp, err := h.httpc.Do(req)
	if err != nil {
		cancel()
		return nil, nil, cancel, err
	}
	br := bufio.NewReaderSize(resp.Body, 16384)
	if _, perr := br.Peek(1); perr != nil {
		status := resp.StatusCode
		_ = resp.Body.Close()
		cancel()
		return resp, nil, cancel, fmt.Errorf("upstream %d produced no data within %s: %w", status, h.firstByteTimeout, perr)
	}
	return resp, br, cancel, nil
}

// writeAttemptFailure failover 全候选失败时，把最后一次失败透传给客户端：
// 上游原始响应（rawUpstream）直传 status+body（保留真实 429/错误体）；内部错误包成 JSON error。
func writeAttemptFailure(c *gin.Context, r attemptResult) {
	if r.rawUpstream {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(r.failStatus)
		_, _ = c.Writer.Write(r.failBody)
		return
	}
	c.JSON(r.failStatus, gin.H{"error": gin.H{"message": string(r.failBody)}})
}

// teeRespWriter 包装 gin 响应写器，Write 时把内容同步追加到 buf（用于流式响应落 spend_log）。
// 嵌入 http.ResponseWriter 使 Header/WriteHeader 等透传；实现 Flush 让 provider 的类型断言命中。
type teeRespWriter struct {
	http.ResponseWriter
	buf *bytes.Buffer
}

func (t *teeRespWriter) Write(p []byte) (int, error) {
	if t.buf.Len() < streamCaptureCap {
		t.buf.Write(p)
	}
	return t.ResponseWriter.Write(p)
}

func (t *teeRespWriter) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// bill 异步写花费日志 + 日聚合 + key.spend（异步语义）。
func (h *Handler) bill(c *gin.Context, m *model.Model, k *model.VirtualKey, hash, reqID string, start time.Time,
	callType, apiBase string, cost model.Milli, pt, ct int, status, messages, response string) {
	rec := store.SpendRecord{
		RequestID: reqID, CallType: callType, ApiKey: hash, Spend: cost, PromptTokens: pt, CompletionTokens: ct,
		StartTime: start, EndTime: time.Now(), DurationMs: int(time.Since(start).Milliseconds()),
		Model: m.ProviderModel, ModelID: m.ModelId, ModelGroup: m.ModelGroup, Provider: m.Provider,
		ApiBase: apiBase, UserId: k.UserId, TeamId: k.TeamId, OrgId: k.OrgId, ProjectId: k.ProjectId,
		SessionID: c.GetString("session_id"), IP: c.ClientIP(), Status: status,
		Messages: messages, Response: response,
	}
	date := start.Format("2006-01-02")
	// 鉴权时命中的用户权限组预算：goroutine 里不能碰 gin.Context（请求返回即回收），先捕获。
	userBudgetIDs := c.GetStringSlice("user_budget_ids")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.st.WriteSpendLog(ctx, rec)
		if k.UserId != "" {
			_ = h.st.UpsertDailySpend(ctx, rec, "user", k.UserId, date)
		}
		if k.TeamId != "" {
			_ = h.st.UpsertDailySpend(ctx, rec, "team", k.TeamId, date)
		}
		_ = h.st.UpsertDailySpend(ctx, rec, "key", hash, date)
		if cost > 0 {
			_ = h.st.IncrementKeySpend(ctx, hash, cost)
			// 预算增量打点：key 挂靠的共享预算 ∪ 鉴权时命中的用户权限组预算，去重后逐份扣减
			// （同一预算既被挂靠又命中圈人时只计一次，避免双计）。
			budgetIDs := map[string]bool{}
			if k.BudgetId != "" {
				budgetIDs[k.BudgetId] = true
			}
			for _, id := range userBudgetIDs {
				budgetIDs[id] = true
			}
			for id := range budgetIDs {
				_ = h.st.IncrementBudgetSpend(ctx, id, cost)
			}
		}
		// 预算低额/耗尽提醒：重查 DB 最新值判档（去重表保证每窗口每人每预算每档至多一条）。
		if k.UserId != "" && len(userBudgetIDs) > 0 {
			notify.BudgetAlerts(ctx, h.st, k.UserId, userBudgetIDs)
		}
	}()
}

// capStr 把超长 body 截断到 64KB，避免 spend_log 的 messages/response 单条过大。
const streamCaptureCap = 64 * 1024

func capStr(s string) string {
	if len(s) > streamCaptureCap {
		return s[:streamCaptureCap] + "\n...[truncated]"
	}
	return s
}

func (h *Handler) recordError(c *gin.Context, m *model.Model, k *model.VirtualKey, hash, reqID string, start time.Time,
	callType, apiBase, excType, excStr, statusCode, requestBody string) {
	rec := store.SpendRecord{
		RequestID: reqID, CallType: callType, ApiKey: hash, StartTime: start, EndTime: time.Now(),
		Model: m.ProviderModel, ModelID: m.ModelId, ModelGroup: m.ModelGroup, Provider: m.Provider,
		ApiBase: apiBase, UserId: k.UserId, TeamId: k.TeamId, OrgId: k.OrgId, ProjectId: k.ProjectId, IP: c.ClientIP(),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.st.WriteErrorLog(ctx, rec, excType, excStr, statusCode, requestBody)
	}()
	// 失败时给该部署置默认冷却（到期自动恢复路由；上游 Retry-After 由调用方用 MarkCooldownFor 覆盖）。
	h.strategy.MarkCooldown(c, m.ModelId)
}

// ---- 工具 ----

func parseModelAndStream(body []byte) (string, bool) {
	var v struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(body, &v)
	return v.Model, v.Stream
}

// firstByteTimeoutFromEnv 上游首字节超时（LLM_UPSTREAM_FIRST_BYTE_TIMEOUT_SECONDS，默认 120s）。
// 120s 对云上游足够宽松——首字节≠生成完成，anthropic/openai 流式协议都是 token 一到就发；
// 又能把 wedged 连接（实测上游挂 165s 后回空流）在拖满 10 分钟总超时前砍掉。
func firstByteTimeoutFromEnv() time.Duration {
	if s := os.Getenv("LLM_UPSTREAM_FIRST_BYTE_TIMEOUT_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 120 * time.Second
}

// parseRetryAfter 解析上游 Retry-After 头（秒数或 HTTP 日期），返回冷却时长；
// 缺失/无效/超 60s 返回 0（只认 0<ra<=60，避免过长冷却卡死恢复）。
func parseRetryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if n, err := strconv.Atoi(header); err == nil {
		if n > 0 && n <= 60 {
			return time.Duration(n) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 && d <= 60*time.Second {
			return d
		}
	}
	return 0
}

func defaultBaseURL(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com"
	case "deepseek":
		return "https://api.deepseek.com"
	case "moonshot":
		return "https://api.moonshot.cn"
	case "groq":
		return "https://api.groq.com/openai"
	case "openrouter":
		return "https://openrouter.ai/api"
	case "ollama":
		return "http://127.0.0.1:11434"
	default:
		return ""
	}
}
