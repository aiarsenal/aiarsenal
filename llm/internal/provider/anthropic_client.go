package provider

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// 本文件支持 /v1/messages 客户端协议：客户端发 Anthropic Messages 格式，
// 网关内部转 OpenAI 跑标准路由/上游链路，再把 OpenAI 响应转回 Anthropic。

// AnthropicRequestToOpenAI 把 Anthropic Messages 请求体转成 OpenAI chat/completions 请求体。
// 返回 OpenAI body、model（客户端传的，用于路由匹配）、stream。
func AnthropicRequestToOpenAI(body []byte) ([]byte, string, bool, error) {
	var in struct {
		Model       string          `json:"model"`
		MaxTokens   int             `json:"max_tokens"`
		System      json.RawMessage `json:"system"`
		Messages    []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"top_p"`
		Stop        any      `json:"stop_sequences"`
		Stream      bool     `json:"stream"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, "", false, err
	}

	out := map[string]any{"model": in.Model}
	var msgs []map[string]any
	// system：Anthropic 顶层 system（string 或 content blocks）→ OpenAI 的 system 消息
	if sys := contentToString(in.System); sys != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": sys})
	}
	for _, m := range in.Messages {
		role := m.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		msgs = append(msgs, map[string]any{"role": role, "content": contentToString(m.Content)})
	}
	out["messages"] = msgs
	if in.MaxTokens > 0 {
		out["max_tokens"] = in.MaxTokens
	}
	if in.Temperature != nil {
		out["temperature"] = *in.Temperature
	}
	if in.TopP != nil {
		out["top_p"] = *in.TopP
	}
	if in.Stop != nil {
		out["stop"] = in.Stop
	}
	// /v1/messages 端点先只做非流式（上游非流式调用，响应转回 Anthropic 非流式）
	out["stream"] = false

	b, err := json.Marshal(out)
	if err != nil {
		return nil, "", false, err
	}
	return b, in.Model, in.Stream, nil
}

// OpenAIResponseToAnthropic 把 OpenAI chat/completions 响应转成 Anthropic Messages 响应。
func OpenAIResponseToAnthropic(body []byte, model string) ([]byte, Usage, error) {
	var in struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return body, Usage{}, nil
	}
	text := ""
	stop := "end_turn"
	if len(in.Choices) > 0 {
		text = in.Choices[0].Message.Content
		stop = openAIToAnthropicStop(in.Choices[0].FinishReason)
	}
	id := in.ID
	if id == "" {
		id = "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	mdl := in.Model
	if mdl == "" {
		mdl = model
	}
	out := map[string]any{
		"id":             id,
		"type":           "message",
		"role":           "assistant",
		"model":          mdl,
		"content":        []map[string]any{{"type": "text", "text": text}},
		"stop_reason":    stop,
		"stop_sequence":  nil,
		"usage": map[string]any{
			"input_tokens":  in.Usage.PromptTokens,
			"output_tokens": in.Usage.CompletionTokens,
		},
	}
	b, _ := json.Marshal(out)
	return b, Usage{PromptTokens: in.Usage.PromptTokens, CompletionTokens: in.Usage.CompletionTokens}, nil
}

// openAIToAnthropicStop OpenAI finish_reason → Anthropic stop_reason。
func openAIToAnthropicStop(r string) string {
	switch r {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return orDefault(r, "end_turn")
	}
}
