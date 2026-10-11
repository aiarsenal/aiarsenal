package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"aiarsenal/llm/internal/auth"
	"aiarsenal/llm/internal/model"
	"aiarsenal/llm/internal/provider"
)

// Gemini 原生协议入口（/v1beta）：说 Gemini 协议的客户端 → 网关内翻成 OpenAI chat/completions
// 跑 openai 兼容族路由/上游（候选选取与跨凭据 failover 复用 proxyChat 同一套），响应翻回 Gemini 格式。
// 鉴权与 /v1 同一套虚拟 Key 中间件（main.go 分组挂载；auth 侧另兼容 x-goog-api-key 头与 ?key= 参数）。
// 翻译函数见 provider/gemini_client.go（与 anthropic_client.go 同构）。

// GeminiGenerateContent POST /v1beta/models/{model}:generateContent 与
// POST /v1beta/models/{model}:streamGenerateContent 共用入口：path 末段形如 "{model}:{action}"
// （gin 单段参数 modelAction 整段捕获，客户端把冒号编码成 %3A 也能对上），按 action 分流流式/非流式。
// 流式即 alt=sse 帧序列——Gemini 的 streamGenerateContent 不带 alt=sse 时官方返回 JSON 数组，
// 这里统一按 SSE 回（分帧语义一致，主流 SDK 均按流读）。
func (h *Handler) GeminiGenerateContent(c *gin.Context) {
	seg := c.Param("modelAction")
	idx := strings.LastIndex(seg, ":")
	if idx <= 0 {
		geminiError(c, http.StatusNotFound, 404, "NOT_FOUND",
			"expect /v1beta/models/{model}:generateContent or :streamGenerateContent")
		return
	}
	reqModel, action := seg[:idx], seg[idx+1:]
	stream := action == "streamGenerateContent"
	if action != "generateContent" && !stream {
		geminiError(c, http.StatusNotFound, 404, "NOT_FOUND", "unsupported action: "+action)
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		geminiError(c, http.StatusBadRequest, 400, "INVALID_ARGUMENT", "read body: "+err.Error())
		return
	}
	k := c.MustGet("key").(*auth.Key)
	hash := c.GetString("token_hash")
	if !auth.ModelAllowed(k.VirtualKey, reqModel) {
		geminiError(c, http.StatusForbidden, 403, "PERMISSION_DENIED", "Model not allowed for this key")
		return
	}
	// Gemini 请求体无 model 字段，路由 id 取 path；上游限定 openai 兼容族（其余 provider 族协议不匹配）。
	openaiBody, err := provider.GeminiRequestToOpenAI(raw, reqModel, stream)
	if err != nil {
		geminiError(c, http.StatusBadRequest, 400, "INVALID_ARGUMENT", "invalid generateContent request: "+err.Error())
		return
	}
	candidates, err := h.st.ListModelsByGroup(c, reqModel, openaiCompatibleProviders)
	if err != nil {
		geminiError(c, http.StatusNotFound, 404, "NOT_FOUND", "no model deployed for: "+reqModel)
		return
	}
	// 按 ak 的 allowed_models 收窄：组池分发时只在 ak 支持的模型间分发
	candidates = auth.FilterAllowedModels(candidates, k.VirtualKey)
	if len(candidates) == 0 {
		geminiError(c, http.StatusNotFound, 404, "NOT_FOUND", "no model deployed for: "+reqModel)
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
		last = h.attemptGemini(c, m, openaiBody, stream, k.VirtualKey, hash)
		if last.committed {
			return
		}
	}
	writeAttemptFailure(c, last)
}

// geminiError 按 Gemini 错误形制回客户端：{"error":{"code":..,"message":..,"status":..}}。
func geminiError(c *gin.Context, httpCode, code int, status, msg string) {
	c.JSON(httpCode, gin.H{"error": gin.H{"code": code, "message": msg, "status": status}})
}

// attemptGemini Gemini 入口的单次上游尝试：把已翻好的 OpenAI body 走常规 BuildUpstream 发上游，
// 成功则把 OpenAI 响应/SSE 翻回 Gemini 格式回写并计费；失败（凭据缺失/上游 4xx 5xx/网络错）
// 不写客户端，返回失败供 failover 续试（与 attemptChat 同款骨架，差别只在响应侧回译）。
func (h *Handler) attemptGemini(c *gin.Context, m *model.Model, openaiBody []byte, stream bool, k *model.VirtualKey, hash string) attemptResult {
	reqID := uuid.NewString()
	start := time.Now()
	apiKey, cred, err := h.st.GetCredentialAPIKey(c, m.CredentialId)
	if err != nil {
		h.recordError(c, m, k, hash, reqID, start, "chat", "", "credential_missing", err.Error(), "503", capStr(string(openaiBody)))
		return attemptResult{failStatus: http.StatusServiceUnavailable, failBody: []byte("credential missing: " + err.Error())}
	}
	p := h.registry.Pick(m.Provider)
	baseURL := cred.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(m.Provider)
	}
	provReq := provider.Request{Body: openaiBody, Stream: stream, APIKey: apiKey, BaseURL: baseURL, ProviderModel: m.ProviderModel}
	url, method, headers, upBody, err := p.BuildUpstream(c, provReq)
	if err != nil {
		h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "build_upstream_error", err.Error(), "500", capStr(string(openaiBody)))
		return attemptResult{failStatus: http.StatusInternalServerError, failBody: []byte(err.Error())}
	}
	auth.TouchActive(h.st, hash)
	resp, br, cancelUpstream, err := h.doUpstream(c, method, url, headers, upBody)
	if err != nil {
		if resp == nil {
			h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_error", err.Error(), "502", capStr(string(openaiBody)))
			return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error()), rawUpstream: true}
		}
		// 首字节没等到（超时/零字节空流）：不伪装成成功空流，按上游失败走 failover。
		h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_no_data", err.Error(), "502", capStr(string(openaiBody)))
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, 0, 0, 0, "failed", capStr(string(openaiBody)), "")
		return attemptResult{failStatus: http.StatusBadGateway, failBody: []byte(err.Error())}
	}
	defer cancelUpstream()
	defer resp.Body.Close()
	// 上游 4xx/5xx：错误体原样透传（OpenAI 的 {"error":{...}} 与 Gemini 错误形制同构，
	// 客户端都能读到 error.message），未写客户端可安全换候选重试。
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(br)
		h.recordError(c, m, k, hash, reqID, start, "chat", baseURL, "upstream_http_error", capStr(string(errBody)), fmt.Sprintf("%d", resp.StatusCode), capStr(string(openaiBody)))
		if cd := parseRetryAfter(resp.Header.Get("Retry-After")); cd > 0 {
			h.strategy.MarkCooldownFor(c, m.ModelId, cd) // 上游建议的退避时长覆盖默认冷却
		}
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, 0, 0, 0, "failed", capStr(string(openaiBody)), capStr(string(errBody)))
		return attemptResult{failStatus: resp.StatusCode, failBody: errBody, rawUpstream: true}
	}
	if stream {
		// 200：提交流式响应（已确定成功，本请求不再 failover）。OpenAI SSE → Gemini SSE 帧序列，
		// tee 把写给客户端的分帧同步进缓冲供 spend_log 记录输出。
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.WriteHeader(resp.StatusCode)
		var sbuf bytes.Buffer
		tee := &teeRespWriter{ResponseWriter: c.Writer, buf: &sbuf}
		usage, _ := provider.OpenAIStreamToGemini(br, tee, m.ProviderModel)
		cost := model.CostMilli(usage.PromptTokens, usage.CompletionTokens, m.InputPricePerMillion, m.OutputPricePerMillion)
		h.bill(c, m, k, hash, reqID, start, "chat", baseURL, cost, usage.PromptTokens, usage.CompletionTokens, "success", capStr(string(openaiBody)), capStr(sbuf.String()))
		return attemptResult{committed: true}
	}
	// 非流式：OpenAI chat completion → Gemini GenerateContentResponse，usage 从翻译层带回。
	respBody, _ := io.ReadAll(br)
	out, usage, _ := provider.OpenAIResponseToGemini(respBody, m.ProviderModel)
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(out)
	cost := model.CostMilli(usage.PromptTokens, usage.CompletionTokens, m.InputPricePerMillion, m.OutputPricePerMillion)
	h.bill(c, m, k, hash, reqID, start, "chat", baseURL, cost, usage.PromptTokens, usage.CompletionTokens, "success", capStr(string(openaiBody)), capStr(string(out)))
	return attemptResult{committed: true}
}
