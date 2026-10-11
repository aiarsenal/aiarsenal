package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// 本文件支持 /v1beta/models/{model}:generateContent（及 :streamGenerateContent?alt=sse）客户端协议：
// 客户端发 Gemini 原生请求，网关内转 OpenAI 跑标准路由/上游链路（openai 兼容族），
// 再把 OpenAI 响应（含 SSE 流）翻回 Gemini 格式。与 gemini.go 方向相反（那是 OpenAI 客户端 →
// Gemini 上游），结构对齐 anthropic_client.go。

// GeminiRequestToOpenAI 把 Gemini generateContent 请求体转成 OpenAI chat/completions 请求体。
// model 取 URL path 里的调用 id（Gemini 请求体没有 model 字段），BuildUpstream 会重写成 provider_model；
// stream 由入口端点决定（streamGenerateContent），直接写进结果 body。
// 映射范围：contents（role user|model + parts text/inlineData/functionCall/functionResponse）、
// systemInstruction、generationConfig（temperature/topP/maxOutputTokens/stopSequences/candidateCount/
// seed/penalty/responseMimeType）、tools.functionDeclarations、toolConfig.functionCallingConfig。
// topK / responseSchema / fileData 等无对得上口的字段不翻译（忽略）。
func GeminiRequestToOpenAI(body []byte, model string, stream bool) ([]byte, error) {
	var in geminiGenerateRequest
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, err
	}
	out := map[string]any{"model": model, "stream": stream}
	// 预置空数组：contents 为空时回 [] 而非 null（上游对 messages:null 直接 400）
	msgs := []map[string]any{}
	// systemInstruction → system 消息（多 part 以空行拼接，与 gemini.go 出站方向的合并策略一致）
	if in.SystemInstruction != nil {
		if sys := geminiPartsText(in.SystemInstruction.Parts); sys != "" {
			msgs = append(msgs, map[string]any{"role": "system", "content": sys})
		}
	}
	// functionCall/functionResponse 靠名字对上 tool_call_id（Gemini 协议无 id 概念）
	toolCallIDByName := map[string]string{}
	for _, ct := range in.Contents {
		msgs = append(msgs, geminiContentToOpenAI(ct, toolCallIDByName)...)
	}
	out["messages"] = msgs
	gc := in.GenerationConfig
	if gc.MaxOutputTokens != nil && *gc.MaxOutputTokens > 0 {
		out["max_tokens"] = *gc.MaxOutputTokens
	}
	if gc.Temperature != nil {
		out["temperature"] = *gc.Temperature
	}
	if gc.TopP != nil {
		out["top_p"] = *gc.TopP
	}
	if len(gc.StopSequences) > 0 {
		out["stop"] = gc.StopSequences
	}
	if gc.CandidateCount != nil && *gc.CandidateCount > 1 {
		out["n"] = *gc.CandidateCount
	}
	if gc.Seed != nil {
		out["seed"] = *gc.Seed
	}
	if gc.PresencePenalty != nil {
		out["presence_penalty"] = *gc.PresencePenalty
	}
	if gc.FrequencyPenalty != nil {
		out["frequency_penalty"] = *gc.FrequencyPenalty
	}
	// responseMimeType=application/json → JSON 模式；responseSchema 各上游兼容面参差，暂不映射
	if gc.ResponseMimeType == "application/json" {
		out["response_format"] = map[string]any{"type": "json_object"}
	}
	if tools := geminiToolsToOpenAI(in.Tools); len(tools) > 0 {
		out["tools"] = tools
	}
	if in.ToolConfig != nil {
		if tc := geminiToolChoice(in.ToolConfig.FunctionCallingConfig.Mode, in.ToolConfig.FunctionCallingConfig.AllowedFunctionNames); tc != nil {
			out["tool_choice"] = tc
		}
	}
	return json.Marshal(out)
}

// geminiGenerateRequest Gemini generateContent 请求体（只取需翻译的字段，其余忽略）。
type geminiGenerateRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *struct {
		Parts []geminiPart `json:"parts"`
	} `json:"systemInstruction"`
	GenerationConfig struct {
		Temperature      *float64 `json:"temperature"`
		TopP             *float64 `json:"topP"`
		MaxOutputTokens  *int     `json:"maxOutputTokens"`
		StopSequences    []string `json:"stopSequences"`
		CandidateCount   *int     `json:"candidateCount"`
		Seed             *int     `json:"seed"`
		PresencePenalty  *float64 `json:"presencePenalty"`
		FrequencyPenalty *float64 `json:"frequencyPenalty"`
		ResponseMimeType string   `json:"responseMimeType"`
	} `json:"generationConfig"`
	Tools      []geminiTool      `json:"tools"`
	ToolConfig *geminiToolConfig `json:"toolConfig"`
}

// geminiContent contents[] 元素：role 取 user|model，parts 为若干块。
type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart parts[] 元素四选一：text / inlineData / functionCall / functionResponse。
// fileData、thought 等其他块落到 default 分支按空 text 处理（不翻译、不阻断）。
type geminiPart struct {
	Text string `json:"text"`
	InlineData *struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"` // base64
	} `json:"inlineData"`
	FunctionCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"functionCall"`
	FunctionResponse *struct {
		Name     string          `json:"name"`
		Response json.RawMessage `json:"response"`
	} `json:"functionResponse"`
}

// geminiTool tools[] 元素：函数声明列表。
type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

// geminiFuncDecl Gemini 函数声明：parameters 已是 JSON Schema，原样透传。
type geminiFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// geminiToolConfig toolConfig.functionCallingConfig：调用模式与白名单。
type geminiToolConfig struct {
	FunctionCallingConfig struct {
		Mode                 string   `json:"mode"`
		AllowedFunctionNames []string `json:"allowedFunctionNames"`
	} `json:"functionCallingConfig"`
}

// geminiContentToOpenAI 把一条 Gemini content 翻成 0..n 条 OpenAI 消息：
//   - model 角色 → assistant（functionCall parts 合成 tool_calls）
//   - user 角色 → tool 消息（functionResponse，函数结果回传）在前 + user 消息（text/inlineData）在后
//     （OpenAI 要求 tool 消息紧跟对应 tool_calls，Gemini 的函数结果在下一轮 user content 里）
//
// 空内容（无 text、无图、无函数）返回 nil 直接跳过。
func geminiContentToOpenAI(ct geminiContent, toolCallIDByName map[string]string) []map[string]any {
	if len(ct.Parts) == 0 {
		return nil
	}
	var texts []string
	var imageParts []map[string]any
	var toolCalls []map[string]any
	var toolMsgs []map[string]any
	hasFunctionCall := false
	for _, p := range ct.Parts {
		switch {
		case p.FunctionCall != nil:
			hasFunctionCall = true
			id := toolCallIDByName[p.FunctionCall.Name]
			if id == "" {
				id = "call_" + strconv.Itoa(len(toolCallIDByName)+1)
				toolCallIDByName[p.FunctionCall.Name] = id
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      p.FunctionCall.Name,
					"arguments": string(argsToRaw(p.FunctionCall.Args)),
				},
			})
		case p.FunctionResponse != nil:
			toolMsgs = append(toolMsgs, map[string]any{
				"role":         "tool",
				"tool_call_id": toolCallIDByName[p.FunctionResponse.Name],
				"content":      string(p.FunctionResponse.Response),
			})
		case p.InlineData != nil:
			imageParts = append(imageParts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": "data:" + p.InlineData.MimeType + ";base64," + p.InlineData.Data},
			})
		default:
			texts = append(texts, p.Text)
		}
	}
	role := "user"
	if ct.Role == "model" || ct.Role == "assistant" || hasFunctionCall {
		role = "assistant"
	}
	if role == "assistant" {
		msg := map[string]any{"role": "assistant", "content": strings.Join(texts, "")}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		return []map[string]any{msg}
	}
	out := toolMsgs
	if len(imageParts) > 0 {
		// 多模态：content 用数组形（text 前置 + 图片 data URI）
		parts := append([]map[string]any{{"type": "text", "text": strings.Join(texts, "")}}, imageParts...)
		out = append(out, map[string]any{"role": "user", "content": parts})
	} else if txt := strings.Join(texts, ""); txt != "" {
		out = append(out, map[string]any{"role": "user", "content": txt})
	}
	return out
}

// geminiPartsText 拍平 parts 里的 text 字段（空行连接，与 contentToString 的出站合并策略一致）。
func geminiPartsText(parts []geminiPart) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// argsToRaw 把 Gemini 的 args / parameters（应为 JSON object）归一成合法对象：
// 空、null、非对象一律退 "{}"——OpenAI 的 arguments 是字符串、parameters 必填，不能是裸 null。
func argsToRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil && m != nil {
		return raw
	}
	return json.RawMessage("{}")
}

// geminiToolsToOpenAI tools[].functionDeclarations → OpenAI tools[{type:function,function:{...}}]。
func geminiToolsToOpenAI(tools []geminiTool) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		for _, fd := range t.FunctionDeclarations {
			out = append(out, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        fd.Name,
					"description": fd.Description,
					"parameters":  argsToRaw(fd.Parameters),
				},
			})
		}
	}
	return out
}

// geminiToolChoice functionCallingConfig → tool_choice：AUTO→省略（OpenAI 默认 auto）、
// ANY→required（白名单唯一函数时定点指定）、NONE→none。
func geminiToolChoice(mode string, allowed []string) any {
	switch strings.ToUpper(mode) {
	case "ANY":
		if len(allowed) == 1 {
			return map[string]any{"type": "function", "function": map[string]any{"name": allowed[0]}}
		}
		return "required"
	case "NONE":
		return "none"
	}
	return nil
}

// OpenAIResponseToGemini 把 OpenAI chat/completions 非流式响应转成 Gemini GenerateContentResponse。
// 解析失败原样透传（与 TransformResponse 同策略：上游已 200 的怪体不二次包装），用量置零。
func OpenAIResponseToGemini(body []byte, fallbackModel string) ([]byte, Usage, error) {
	var in struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return body, Usage{}, nil
	}
	cands := make([]map[string]any, 0, len(in.Choices))
	for i, ch := range in.Choices {
		var parts []map[string]any
		if ch.Message.Content != nil && *ch.Message.Content != "" {
			parts = append(parts, map[string]any{"text": *ch.Message.Content})
		}
		for _, tc := range ch.Message.ToolCalls {
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": tc.Function.Name,
					"args": openAIArgsToObject(tc.Function.Arguments),
				},
			})
		}
		if len(parts) == 0 {
			parts = append(parts, map[string]any{"text": ""})
		}
		cands = append(cands, map[string]any{
			"content":      map[string]any{"role": "model", "parts": parts},
			"finishReason": openAIStopToGemini(ch.FinishReason),
			"index":        i,
		})
	}
	out := map[string]any{"candidates": cands}
	if in.Model != "" || fallbackModel != "" {
		out["modelVersion"] = orDefault(in.Model, fallbackModel)
	}
	out["usageMetadata"] = geminiUsageMetadata(Usage{
		PromptTokens:     in.Usage.PromptTokens,
		CompletionTokens: in.Usage.CompletionTokens,
	})
	b, _ := json.Marshal(out)
	return b, Usage{PromptTokens: in.Usage.PromptTokens, CompletionTokens: in.Usage.CompletionTokens}, nil
}

// OpenAIStreamToGemini 读上游 OpenAI SSE 流，翻成 Gemini streamGenerateContent（alt=sse）帧序列
// 写 w（每帧即时 Flush），返回解析到的 token 用量。帧格式：
//
//	data: {"candidates":[{"content":{"role":"model","parts":[{"text":"增量"}]},"index":0}],"modelVersion":"..."}
//
// 文本增量每 delta 一帧；tool_calls 增量按 index 聚合、finishReason 帧一次性以 functionCall parts
// 下发（Gemini 协议没有 tool_call 增量，functionCall 以完整对象出现在 parts 里）；
// 末帧附 finishReason 与 usageMetadata。不发 OpenAI 的 [DONE]——Gemini SSE 无哨兵帧，流尽即止。
func OpenAIStreamToGemini(upstream io.Reader, w http.ResponseWriter, fallbackModel string) (Usage, error) {
	flusher, _ := w.(http.Flusher)
	var usage Usage
	modelVersion := fallbackModel
	// tool_calls 增量聚合：key=choice 序:tool_call 序（n>1 时按 choice 区分）
	type toolAcc struct {
		name string
		args strings.Builder
	}
	acc := map[string]*toolAcc{}
	toolCount := map[string]int{}
	// finishCalls choice 序下的聚合结果翻成 functionCall parts（保持 tool_call index 序）
	finishCalls := func(ci string) []map[string]any {
		var parts []map[string]any
		for ti := 0; ti < toolCount[ci]; ti++ {
			a := acc[fmt.Sprintf("%s:%d", ci, ti)]
			if a == nil || a.name == "" {
				continue
			}
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{"name": a.name, "args": openAIArgsToObject(a.args.String())},
			})
		}
		return parts
	}

	sc := bufio.NewScanner(upstream)
	sc.Buffer(make([]byte, 0, 65536), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ch struct {
			Model string `json:"model"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content string `json:"content"`
					ToolCalls []struct {
						Index    *int `json:"index"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &ch) != nil {
			continue
		}
		if ch.Model != "" {
			modelVersion = ch.Model
		}
		if ch.Usage != nil && (ch.Usage.PromptTokens > 0 || ch.Usage.CompletionTokens > 0) {
			usage = Usage{PromptTokens: ch.Usage.PromptTokens, CompletionTokens: ch.Usage.CompletionTokens}
		}
		for _, chs := range ch.Choices {
			ci := strconv.Itoa(chs.Index)
			if chs.Delta.Content != "" {
				writeGeminiSSE(w, flusher, geminiStreamFrame(chs.Index, modelVersion,
					[]map[string]any{{"text": chs.Delta.Content}}, "", usage))
			}
			for _, tc := range chs.Delta.ToolCalls {
				ti := 0
				if tc.Index != nil {
					ti = *tc.Index
				}
				key := fmt.Sprintf("%s:%d", ci, ti)
				a := acc[key]
				if a == nil {
					a = &toolAcc{}
					acc[key] = a
					if ti+1 > toolCount[ci] {
						toolCount[ci] = ti + 1
					}
				}
				if tc.Function.Name != "" {
					a.name += tc.Function.Name
				}
				a.args.WriteString(tc.Function.Arguments)
			}
			if chs.FinishReason != "" {
				// 末帧：functionCall parts（有函数调用时）+ finishReason + usageMetadata；
				// 纯文本结束也给 content（空 text part），对齐 Google 每帧都带 content 的形制。
				parts := finishCalls(ci)
				if len(parts) == 0 {
					parts = []map[string]any{{"text": ""}}
				}
				writeGeminiSSE(w, flusher, geminiStreamFrame(chs.Index, modelVersion,
					parts, openAIStopToGemini(chs.FinishReason), usage))
			}
		}
	}
	return usage, sc.Err()
}

// geminiStreamFrame 构造一帧 Gemini streamGenerateContent（alt=sse）响应。
func geminiStreamFrame(index int, modelVersion string, parts []map[string]any, finishReason string, usage Usage) map[string]any {
	cand := map[string]any{
		"content": map[string]any{"role": "model", "parts": parts},
		"index":   index,
	}
	if finishReason != "" {
		cand["finishReason"] = finishReason
	}
	fr := map[string]any{"candidates": []map[string]any{cand}}
	if modelVersion != "" {
		fr["modelVersion"] = modelVersion
	}
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		fr["usageMetadata"] = geminiUsageMetadata(usage)
	}
	return fr
}

// geminiUsageMetadata OpenAI usage → Gemini usageMetadata（total 缺失用 prompt+completion 补）。
func geminiUsageMetadata(u Usage) map[string]any {
	total := u.PromptTokens + u.CompletionTokens
	return map[string]any{
		"promptTokenCount":     u.PromptTokens,
		"candidatesTokenCount": u.CompletionTokens,
		"totalTokenCount":      total,
	}
}

// writeGeminiSSE 写一帧 Gemini SSE（data: <json>\n\n + Flush）；writeSSE 只负责写与 Flush，
// 帧包裹在这里拼（与 openAIChunk 同套路）。
func writeGeminiSSE(w http.ResponseWriter, flusher http.Flusher, v map[string]any) {
	b, _ := json.Marshal(v)
	writeSSE(w, flusher, []byte("data: "+string(b)+"\n\n"))
}

// openAIArgsToObject 把 OpenAI tool_call 的 arguments（JSON 字符串）解成 Gemini args 对象；
// 解不出（空/残缺）退空对象——Gemini 的 args 必须是对象。
func openAIArgsToObject(args string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(args), &m) == nil && m != nil {
		return m
	}
	return map[string]any{}
}

// openAIStopToGemini OpenAI finish_reason → Gemini finishReason。
// tool_calls 归 STOP（函数调用在 Gemini 里以 parts.functionCall 表达，结束原因就是 STOP）。
func openAIStopToGemini(r string) string {
	switch r {
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	default:
		return "STOP"
	}
}
