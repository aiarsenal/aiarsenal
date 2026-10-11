package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AnthropicProvider 把 OpenAI chat 请求翻成 Anthropic Messages API（/v1/messages），
// 并把 Anthropic 响应（含 SSE 流）翻回 OpenAI 格式。
type AnthropicProvider struct{}

func (AnthropicProvider) Name() string { return "anthropic" }

func (AnthropicProvider) UpstreamURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func (AnthropicProvider) BuildUpstream(ctx context.Context, req Request) (string, string, map[string]string, []byte, error) {
	body, err := openaiToAnthropic(req.Body, req.ProviderModel)
	if err != nil {
		return "", "", nil, nil, err
	}
	url := AnthropicProvider{}.UpstreamURL(defaultBaseURL0("anthropic", req.BaseURL), "/v1/messages")
	headers := map[string]string{
		"Content-Type":    "application/json",
		"x-api-key":       req.APIKey,
		"anthropic-version": "2023-06-01",
	}
	for k, v := range req.ExtraHeaders {
		headers[k] = v
	}
	return url, "POST", headers, body, nil
}

// openaiToAnthropic 转换请求体。
func openaiToAnthropic(body []byte, providerModel string) ([]byte, error) {
	var in struct {
		Model     string `json:"model"`
		Messages  []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		MaxTokens  *int     `json:"max_tokens"`
		Temperature *float64 `json:"temperature"`
		TopP       *float64 `json:"top_p"`
		Stop       any      `json:"stop"`
		Stream     bool     `json:"stream"`
		System     string   `json:"system"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	out := map[string]any{}
	if providerModel != "" {
		out["model"] = providerModel
	} else {
		out["model"] = in.Model
	}
	// 抽取 system 消息
	var systemParts []string
	var msgs []map[string]any
	for _, m := range in.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, contentToString(m.Content))
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		// Anthropic 只接受 user/assistant；tool 等此处简化为 user
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, map[string]any{"role": role, "content": toAnthropicContent(m.Content)})
	}
	if in.System != "" {
		systemParts = append(systemParts, in.System)
	}
	if len(systemParts) > 0 {
		out["system"] = strings.Join(systemParts, "\n\n")
	}
	out["messages"] = msgs
	maxTok := 4096
	if in.MaxTokens != nil {
		maxTok = *in.MaxTokens
	}
	out["max_tokens"] = maxTok
	if in.Temperature != nil {
		out["temperature"] = *in.Temperature
	}
	if in.TopP != nil {
		out["top_p"] = *in.TopP
	}
	if in.Stop != nil {
		out["stop_sequences"] = in.Stop
	}
	if in.Stream {
		out["stream"] = true
	}
	return json.Marshal(out)
}

// contentToString 把 message.content（string 或数组）拍平为纯文本。
func contentToString(c json.RawMessage) string {
	var s string
	if json.Unmarshal(c, &s) == nil {
		return s
	}
	var arr []map[string]any
	if json.Unmarshal(c, &arr) == nil {
		var b strings.Builder
		for _, p := range arr {
			if t, ok := p["text"].(string); ok {
				b.WriteString(t)
			}
		}
		return b.String()
	}
	return string(c)
}

// toAnthropicContent string 直接用；数组透传（已是 content blocks）。
func toAnthropicContent(c json.RawMessage) any {
	var s string
	if json.Unmarshal(c, &s) == nil {
		return s
	}
	var arr any
	if json.Unmarshal(c, &arr) == nil {
		return arr
	}
	return string(c)
}

func (AnthropicProvider) TransformResponse(upstreamBody []byte, req Request) ([]byte, Usage, error) {
	var in struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(upstreamBody, &in); err != nil {
		return upstreamBody, Usage{}, nil
	}
	var text strings.Builder
	for _, b := range in.Content {
		if b.Type == "text" {
			text.WriteString(b.Text)
		}
	}
	out := map[string]any{
		"id":     orDefault(in.ID, "chatcmpl-"+uuid.NewString()),
		"object": "chat.completion",
		"model":  orDefault(in.Model, req.ProviderModel),
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text.String()},
			"finish_reason": anthropicStopToOpenAI(in.StopReason),
		}},
		"usage": map[string]any{
			"prompt_tokens":     in.Usage.InputTokens,
			"completion_tokens": in.Usage.OutputTokens,
			"total_tokens":      in.Usage.InputTokens + in.Usage.OutputTokens,
		},
	}
	b, _ := json.Marshal(out)
	u := Usage{
		PromptTokens: in.Usage.InputTokens, CompletionTokens: in.Usage.OutputTokens,
		CacheReadTokens: in.Usage.CacheReadInputTokens, CacheCreateTokens: in.Usage.CacheCreationInputTokens,
	}
	return b, u, nil
}

func (AnthropicProvider) TransformStream(upstream io.Reader, w http.ResponseWriter, req Request) (Usage, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	id := "chatcmpl-" + uuid.NewString()
	model := req.ProviderModel
	var usage Usage
	sentRole := false

	sc := bufio.NewScanner(upstream)
	sc.Buffer(make([]byte, 0, 65536), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
				InputTokens int `json:"input_tokens"`
				CacheReadInputTokens int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			usage.PromptTokens = ev.Message.Usage.InputTokens
			if !sentRole {
				writeSSE(w, flusher, openAIChunk(id, model, map[string]any{"role": "assistant"}, nil, ""))
				sentRole = true
			}
		case "content_block_delta":
			if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
				writeSSE(w, flusher, openAIChunk(id, model, map[string]any{"content": ev.Delta.Text}, nil, ""))
			}
		case "message_delta":
			usage.CompletionTokens = ev.Usage.OutputTokens
			usage.CacheReadTokens = ev.Usage.CacheReadInputTokens
			usage.CacheCreateTokens = ev.Usage.CacheCreationInputTokens
			fr := anthropicStopToOpenAI(ev.Delta.StopReason)
			if fr == "" {
				fr = "stop"
			}
			writeSSE(w, flusher, openAIChunk(id, model, map[string]any{}, &fr, ""))
		case "message_stop":
			writeSSE(w, flusher, openAIChunk(id, model, map[string]any{}, nil, ""))
		}
	}
	// 末尾 usage 帧 + [DONE]
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		writeSSE(w, flusher, openAIChunk(id, model, map[string]any{}, strPtr("stop"), "", usage))
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
	return usage, sc.Err()
}

// openAIChunk 构造一个 OpenAI chat.completion.chunk。
func openAIChunk(id, model string, delta map[string]any, finishReason *string, content string, usage ...Usage) []byte {
	choice := map[string]any{"index": 0, "delta": delta}
	if finishReason != nil {
		choice["finish_reason"] = *finishReason
	}
	chunk := map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": 0, "model": model, "choices": []map[string]any{choice},
	}
	if len(usage) > 0 {
		chunk["usage"] = map[string]any{
			"prompt_tokens":     usage[0].PromptTokens,
			"completion_tokens": usage[0].CompletionTokens,
			"total_tokens":       usage[0].PromptTokens + usage[0].CompletionTokens,
		}
	}
	b, _ := json.Marshal(chunk)
	return []byte("data: " + string(b) + "\n\n")
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, data []byte) {
	_, _ = w.Write(data)
	if flusher != nil {
		flusher.Flush()
	}
}

func anthropicStopToOpenAI(r string) string {
	switch r {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	}
	return ""
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func strPtr(s string) *string { return &s }

// DefaultBaseURL 凭据未填 base_url 时按 provider 回官方默认地址（导出给探测等包复用）。
func DefaultBaseURL(provider, base string) string { return defaultBaseURL0(provider, base) }

// defaultBaseURL0 本包内 provider 共用的默认 base（与 proxy 包同义，避免互相依赖）。
func defaultBaseURL0(provider, base string) string {
	if base != "" {
		return base
	}
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	}
	return ""
}
