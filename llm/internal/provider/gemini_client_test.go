package provider

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// 请求翻译：contents / systemInstruction / generationConfig 逐项核对。
func TestGeminiRequestToOpenAI(t *testing.T) {
	in := `{
		"systemInstruction": {"parts": [{"text": "你是助手"}]},
		"contents": [
			{"role": "user", "parts": [{"text": "你好"}]},
			{"role": "model", "parts": [{"text": "你好！"}]},
			{"role": "user", "parts": [{"text": "看图"}, {"inlineData": {"mimeType": "image/png", "data": "aGk="}}]}
		],
		"generationConfig": {"temperature": 0.5, "topP": 0.9, "maxOutputTokens": 1024, "stopSequences": ["END"], "responseMimeType": "application/json"}
	}`
	body, err := GeminiRequestToOpenAI([]byte(in), "auto/glm-5.2", false)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var out struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		TopP        float64 `json:"top_p"`
		Stop        []string `json:"stop"`
		ResponseFormat map[string]string `json:"response_format"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Model != "auto/glm-5.2" || out.Stream {
		t.Fatalf("model/stream: %s %v", out.Model, out.Stream)
	}
	if len(out.Messages) != 4 {
		t.Fatalf("messages: %s", body)
	}
	if out.Messages[0].Role != "system" || string(out.Messages[0].Content) != `"你是助手"` {
		t.Fatalf("system 消息: %s", out.Messages[0].Content)
	}
	if out.Messages[1].Role != "user" || out.Messages[2].Role != "assistant" {
		t.Fatalf("role 映射: %s", body)
	}
	if out.Messages[3].Role != "user" || !strings.Contains(string(out.Messages[3].Content), "image_url") {
		t.Fatalf("inlineData → image_url: %s", out.Messages[3].Content)
	}
	if out.MaxTokens != 1024 || out.Temperature != 0.5 || out.TopP != 0.9 || len(out.Stop) != 1 {
		t.Fatalf("generationConfig 映射: %s", body)
	}
	if out.ResponseFormat["type"] != "json_object" {
		t.Fatalf("responseMimeType 映射: %s", body)
	}
}

// 函数调用：tools/toolConfig、functionCall → tool_calls、functionResponse → tool 消息（同名对上 id）。
func TestGeminiRequestToOpenAIFunctionCalling(t *testing.T) {
	in := `{
		"contents": [
			{"role": "user", "parts": [{"text": "北京天气如何"}]},
			{"role": "model", "parts": [{"functionCall": {"name": "get_weather", "args": {"city":"北京"}}}]},
			{"role": "user", "parts": [{"functionResponse": {"name": "get_weather", "response": {"temp": 25}}}]}
		],
		"tools": [{"functionDeclarations": [{"name": "get_weather", "description": "查天气", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}}]}],
		"toolConfig": {"functionCallingConfig": {"mode": "ANY"}}
	}`
	body, err := GeminiRequestToOpenAI([]byte(in), "m", false)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var out struct {
		Messages []struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		ToolChoice any `json:"tool_choice"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("messages: %s", body)
	}
	if out.Messages[1].Role != "assistant" || len(out.Messages[1].ToolCalls) != 1 {
		t.Fatalf("functionCall → tool_calls: %s", body)
	}
	if out.Messages[1].ToolCalls[0].Function.Name != "get_weather" ||
		out.Messages[1].ToolCalls[0].Function.Arguments != `{"city":"北京"}` {
		t.Fatalf("tool_calls 内容: %s", body)
	}
	if out.Messages[2].Role != "tool" || out.Messages[2].ToolCallID != out.Messages[1].ToolCalls[0].ID {
		t.Fatalf("functionResponse → tool 消息（id 没对上）: %s", body)
	}
	if len(out.Tools) != 1 || out.Tools[0].Type != "function" || out.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("tools 映射: %s", body)
	}
	if out.ToolChoice != "required" {
		t.Fatalf("toolConfig ANY → required: %s", body)
	}
}

// 响应回译：choices → candidates、finish_reason → finishReason、usage → usageMetadata、tool_calls → functionCall。
func TestOpenAIResponseToGemini(t *testing.T) {
	up := []byte(`{
		"model": "glm-5.2",
		"choices": [{"index": 0, "finish_reason": "tool_calls", "message": {
			"role": "assistant", "content": null,
			"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"北京\"}"}}]
		}}],
		"usage": {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
	}`)
	body, usage, err := OpenAIResponseToGemini(up, "auto/x")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if usage.PromptTokens != 11 || usage.CompletionTokens != 7 {
		t.Fatalf("usage: %+v", usage)
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text         string         `json:"text"`
					FunctionCall map[string]any `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
			Index        int    `json:"index"`
		} `json:"candidates"`
		UsageMetadata map[string]int `json:"usageMetadata"`
		ModelVersion  string         `json:"modelVersion"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Candidates) != 1 || out.Candidates[0].FinishReason != "STOP" || out.Candidates[0].Index != 0 {
		t.Fatalf("candidates: %s", body)
	}
	if out.Candidates[0].Content.Role != "model" || len(out.Candidates[0].Content.Parts) != 1 {
		t.Fatalf("parts: %s", body)
	}
	fc := out.Candidates[0].Content.Parts[0].FunctionCall
	if fc["name"] != "get_weather" {
		t.Fatalf("functionCall.name: %s", body)
	}
	if args, ok := fc["args"].(map[string]any); !ok || args["city"] != "北京" {
		t.Fatalf("arguments → args 对象: %s", body)
	}
	if out.UsageMetadata["promptTokenCount"] != 11 || out.UsageMetadata["candidatesTokenCount"] != 7 || out.UsageMetadata["totalTokenCount"] != 18 {
		t.Fatalf("usageMetadata: %s", body)
	}
	if out.ModelVersion != "glm-5.2" {
		t.Fatalf("modelVersion 应取上游 model: %s", body)
	}
}

// 流式：OpenAI SSE chunk → Gemini 帧（text 增量帧 + 末帧 finishReason/usageMetadata，无 [DONE]）。
func TestOpenAIStreamToGemini(t *testing.T) {
	up := strings.Join([]string{
		`data: {"model":"glm-5.2","choices":[{"index":0,"delta":{"role":"assistant","content":"你"}}]}`,
		``,
		`data: {"model":"glm-5.2","choices":[{"index":0,"delta":{"content":"好"}}]}`,
		``,
		`data: {"model":"glm-5.2","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	w := httptest.NewRecorder()
	usage, err := OpenAIStreamToGemini(strings.NewReader(up), w, "fallback")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if usage.PromptTokens != 3 || usage.CompletionTokens != 2 {
		t.Fatalf("usage: %+v", usage)
	}
	out := w.Body.String()
	if strings.Contains(out, "[DONE]") {
		t.Fatalf("Gemini SSE 不应有 [DONE]: %s", out)
	}
	var frames []map[string]any
	for _, line := range strings.Split(out, "\n") {
		if d, ok := strings.CutPrefix(line, "data:"); ok && strings.TrimSpace(d) != "" {
			var f map[string]any
			if err := json.Unmarshal([]byte(d), &f); err != nil {
				t.Fatalf("帧不是合法 JSON: %s", line)
			}
			frames = append(frames, f)
		}
	}
	if len(frames) != 3 {
		t.Fatalf("应 3 帧（两文本增量 + 末帧），got %d: %s", len(frames), out)
	}
	getText := func(f map[string]any) string {
		cs, _ := f["candidates"].([]any)
		c0, _ := cs[0].(map[string]any)
		ct, _ := c0["content"].(map[string]any)
		ps, _ := ct["parts"].([]any)
		p0, _ := ps[0].(map[string]any)
		t, _ := p0["text"].(string)
		return t
	}
	if getText(frames[0]) != "你" || getText(frames[1]) != "好" {
		t.Fatalf("文本增量帧: %s", out)
	}
	if frames[2]["modelVersion"] != "glm-5.2" {
		t.Fatalf("末帧 modelVersion: %s", out)
	}
	cs, _ := frames[2]["candidates"].([]any)
	c0, _ := cs[0].(map[string]any)
	if c0["finishReason"] != "STOP" {
		t.Fatalf("末帧 candidates[0].finishReason: %s", out)
	}
	if um, ok := frames[2]["usageMetadata"].(map[string]any); !ok || um["totalTokenCount"].(float64) != 5 {
		t.Fatalf("末帧 usageMetadata: %s", out)
	}
	if getText(frames[2]) != "" {
		t.Fatalf("末帧 content 应为空 text part: %s", out)
	}
}
