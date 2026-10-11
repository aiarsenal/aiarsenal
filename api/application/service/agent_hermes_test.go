package service

import (
	"strings"
	"testing"
)

// api_mode 按模型 provider 分流协议端点——anthropic 族只能走 /v1/messages，
// 写死 chat_completions 会让网关 /v1/chat/completions 404「no model deployed」
// （auto/glm-5.3 微信回复错误文案踩过）。这里锁三件事：
//  1. provider→api_mode 映射；
//  2. BuildHermesConfig 按 opts.APIMode 落 api_mode 行（空默认 chat_completions）；
//  3. parseHermesConfigKeys 能把 api_mode 取回来（mcp 段同步复用历史 default 时不再丢它）。
func TestHermesAPIModeByProvider(t *testing.T) {
	if got := hermesAPIModeForProvider("anthropic"); got != "anthropic_messages" {
		t.Fatalf("anthropic → %s, want anthropic_messages", got)
	}
	for _, p := range []string{"openai", "deepseek", "moonshot", "groq", "openrouter", "ollama", "", "unknown"} {
		if got := hermesAPIModeForProvider(p); got != "chat_completions" {
			t.Fatalf("%q → %s, want chat_completions", p, got)
		}
	}
}

func TestBuildHermesConfigAPIMode(t *testing.T) {
	cfg := BuildHermesConfig(HermesConfigOpts{
		LLMBaseURL: "http://gw:43214", APIKey: "sk-test",
		APIMode: hermesAPIModeForProvider("anthropic"), DefaultModel: "auto/glm-5.3",
	})
	want := `api_mode: "anthropic_messages"`
	if !containsLine(cfg, want) {
		t.Fatalf("配置缺 %s 行:\n%s", want, cfg)
	}
	baseURL, apiKey, defaultModel, apiMode := parseHermesConfigKeys(cfg)
	if apiMode != "anthropic_messages" {
		t.Fatalf("parse api_mode = %q, want anthropic_messages", apiMode)
	}
	if baseURL != "http://gw:43214" || apiKey != "sk-test" || defaultModel != "auto/glm-5.3" {
		t.Fatalf("parse 回环不符: base=%q key=%q default=%q", baseURL, apiKey, defaultModel)
	}
	// 空 APIMode 兜底 chat_completions（存量调用方未显式传时行为不变）。
	cfgDefault := BuildHermesConfig(HermesConfigOpts{LLMBaseURL: "http://gw:43214", APIKey: "k", DefaultModel: "m"})
	if !containsLine(cfgDefault, `api_mode: "chat_completions"`) {
		t.Fatalf("空 APIMode 未兜底 chat_completions:\n%s", cfgDefault)
	}
	// display.language=zh：hermes i18n（locales/zh.yaml）覆盖的静态消息中文化。
	if !containsLine(cfg, `language: "zh"`) {
		t.Fatalf("配置缺 display.language=zh:\n%s", cfg)
	}
	// onboarding 预置：profile_build=off + seen.busy_input_prompt——压掉首条建档提议与
	// 忙碌时直发的英文 tip（busy-ack 不走 LLM，SOUL 管不到）。带引号的 "off" 防 YAML 1.1
	// 裸 off 解析成 bool。
	if !containsLine(cfg, `profile_build: "off"`) || !containsLine(cfg, `busy_input_prompt: true`) {
		t.Fatalf("配置缺 onboarding 预置块:\n%s", cfg)
	}
}

func containsLine(s, line string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
