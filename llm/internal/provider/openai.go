package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// OpenAIProvider 兼容所有暴露 OpenAI 风格 chat 接口的上游（OpenAI/DeepSeek/Moonshot/Groq/Together/Ollama/vLLM/OpenRouter）。
// 翻译：重写 model 字段后透传；响应（含 SSE）原样回传，仅解析用量计费。
// 约定：base_url 已含 /v1（OpenAI SDK 约定，如 https://api.openai.com/v1 或
// 阿里云 https://.../compatible-mode/v1），本层只追加 /chat/completions。
type OpenAIProvider struct{}

func (OpenAIProvider) Name() string { return "openai" }

func (OpenAIProvider) UpstreamURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func (OpenAIProvider) BuildUpstream(ctx context.Context, req Request) (string, string, map[string]string, []byte, error) {
	body := rewriteModelField(req.Body, req.ProviderModel)
	url := OpenAIProvider{}.UpstreamURL(req.BaseURL, "/chat/completions")
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + req.APIKey,
	}
	for k, v := range req.ExtraHeaders {
		headers[k] = v
	}
	return url, "POST", headers, body, nil
}

func (OpenAIProvider) TransformResponse(upstreamBody []byte, req Request) ([]byte, Usage, error) {
	u, _ := parseOpenAIUsage(upstreamBody)
	return upstreamBody, u, nil
}

func (OpenAIProvider) TransformStream(upstream io.Reader, w http.ResponseWriter, req Request) (Usage, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	var usage Usage
	for {
		n, rerr := upstream.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = w.Write(chunk)
			if flusher != nil {
				flusher.Flush()
			}
			if bytes.Contains(chunk, []byte(`"usage"`)) {
				if u, ok := parseOpenAIUsage(chunk); ok {
					usage = u
				}
			}
		}
		if rerr == io.EOF || rerr != nil {
			break
		}
	}
	return usage, nil
}

// parseOpenAIUsage 从响应 chunk 解析 usage（流式逐行 / 非流式整体）。
func parseOpenAIUsage(data []byte) (Usage, bool) {
	if len(data) == 0 {
		return Usage{}, false
	}
	try := func(b []byte) (Usage, bool) {
		var v struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(b, &v) == nil && (v.Usage.PromptTokens > 0 || v.Usage.CompletionTokens > 0) {
			return Usage{PromptTokens: v.Usage.PromptTokens, CompletionTokens: v.Usage.CompletionTokens}, true
		}
		return Usage{}, false
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("data:")) {
			line = bytes.TrimSpace(line[5:])
		}
		if !bytes.HasPrefix(line, []byte("{")) {
			continue
		}
		if u, ok := try(line); ok {
			return u, true
		}
	}
	if u, ok := try(data); ok {
		return u, true
	}
	return Usage{}, false
}

// rewriteModelField 把 body 里的 model 替换为上游真实模型名。
func rewriteModelField(body []byte, providerModel string) []byte {
	if providerModel == "" {
		return body
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	m["model"] = providerModel
	if b, err := json.Marshal(m); err == nil {
		return b
	}
	return body
}

// 用 http 包：TransformStream 形参为 http.ResponseWriter/http.Flusher。
