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

// GeminiProvider 把 OpenAI chat 请求翻成 Google Gemini generateContent，
// 并把 Gemini 响应（含 SSE 流）翻回 OpenAI 格式。
type GeminiProvider struct{}

func (GeminiProvider) Name() string { return "gemini" }

func (GeminiProvider) UpstreamURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func (GeminiProvider) BuildUpstream(ctx context.Context, req Request) (string, string, map[string]string, []byte, error) {
	body, err := openaiToGemini(req.Body, req.ProviderModel)
	if err != nil {
		return "", "", nil, nil, err
	}
	base := defaultBaseURL0("gemini", req.BaseURL)
	action := "generateContent"
	if req.Stream {
		action = "streamGenerateContent?alt=sse"
	}
	model := req.ProviderModel
	if model == "" {
		model = "gemini-1.5-flash"
	}
	url := GeminiProvider{}.UpstreamURL(base, "/v1beta/models/"+model+":"+action) + "?key=" + req.APIKey
	headers := map[string]string{"Content-Type": "application/json"}
	for k, v := range req.ExtraHeaders {
		headers[k] = v
	}
	return url, "POST", headers, body, nil
}

func openaiToGemini(body []byte, providerModel string) ([]byte, error) {
	var in struct {
		Model     string `json:"model"`
		Messages  []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		MaxTokens   *int     `json:"max_tokens"`
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"top_p"`
		Stop        []string `json:"stop"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	var systemParts []string
	var contents []map[string]any
	for _, m := range in.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, contentToString(m.Content))
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": contentToString(m.Content)}},
		})
	}
	out := map[string]any{"contents": contents}
	if len(systemParts) > 0 {
		out["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": strings.Join(systemParts, "\n\n")}}}
	}
	gc := map[string]any{}
	if in.MaxTokens != nil {
		gc["maxOutputTokens"] = *in.MaxTokens
	}
	if in.Temperature != nil {
		gc["temperature"] = *in.Temperature
	}
	if in.TopP != nil {
		gc["topP"] = *in.TopP
	}
	if len(in.Stop) > 0 {
		gc["stopSequences"] = in.Stop
	}
	if len(gc) > 0 {
		out["generationConfig"] = gc
	}
	return json.Marshal(out)
}

func (GeminiProvider) TransformResponse(upstreamBody []byte, req Request) ([]byte, Usage, error) {
	var in struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(upstreamBody, &in); err != nil {
		return upstreamBody, Usage{}, nil
	}
	var text strings.Builder
	fr := "stop"
	if len(in.Candidates) > 0 {
		c := in.Candidates[0]
		for _, p := range c.Content.Parts {
			text.WriteString(p.Text)
		}
		fr = geminiStopToOpenAI(c.FinishReason)
	}
	out := map[string]any{
		"id":     "chatcmpl-" + uuid.NewString(),
		"object": "chat.completion",
		"model":  orDefault(req.ProviderModel, "gemini"),
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text.String()},
			"finish_reason": fr,
		}},
		"usage": map[string]any{
			"prompt_tokens":     in.UsageMetadata.PromptTokenCount,
			"completion_tokens": in.UsageMetadata.CandidatesTokenCount,
			"total_tokens":       in.UsageMetadata.TotalTokenCount,
		},
	}
	b, _ := json.Marshal(out)
	return b, Usage{
		PromptTokens:     in.UsageMetadata.PromptTokenCount,
		CompletionTokens: in.UsageMetadata.CandidatesTokenCount,
	}, nil
}

func (GeminiProvider) TransformStream(upstream io.Reader, w http.ResponseWriter, req Request) (Usage, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	id := "chatcmpl-" + uuid.NewString()
	model := orDefault(req.ProviderModel, "gemini")
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
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount      int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		if !sentRole {
			writeSSE(w, flusher, openAIChunk(id, model, map[string]any{"role": "assistant"}, nil, ""))
			sentRole = true
		}
		if len(ev.Candidates) > 0 {
			c := ev.Candidates[0]
			for _, p := range c.Content.Parts {
				if p.Text != "" {
					writeSSE(w, flusher, openAIChunk(id, model, map[string]any{"content": p.Text}, nil, ""))
				}
			}
			if c.FinishReason != "" {
				usage.PromptTokens = ev.UsageMetadata.PromptTokenCount
				usage.CompletionTokens = ev.UsageMetadata.CandidatesTokenCount
				fr := geminiStopToOpenAI(c.FinishReason)
				writeSSE(w, flusher, openAIChunk(id, model, map[string]any{}, strPtr(fr), ""))
			}
		}
	}
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		writeSSE(w, flusher, openAIChunk(id, model, map[string]any{}, strPtr("stop"), "", usage))
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
	return usage, sc.Err()
}

func geminiStopToOpenAI(r string) string {
	switch r {
	case "STOP", "MAX_TOKENS":
		if r == "MAX_TOKENS" {
			return "length"
		}
		return "stop"
	case "SAFETY":
		return "content_filter"
	}
	return "stop"
}
