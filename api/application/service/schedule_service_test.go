package service

import "testing"

// 单场事件（freq 空）全链路放行——validateRRule 曾按 default 分支拒绝空 freq，
// 导致网页 repeat=none（rrule=""）与 MCP repeat 不传的 schedule_create 一律报
// 「freq 取值：DAILY/WEEKLY/MONTHLY」，hermes 只能 weekly&count=1 绕行建会。
func TestValidateRRuleAllowsSingleEvent(t *testing.T) {
	for _, r := range []RRule{{}, {Freq: " ", Interval: 1}, {Freq: "", Count: 0}} {
		if err := validateRRule(r); err != nil {
			t.Fatalf("单场规则 %+v 不应报错，got %v", r, err)
		}
	}
	for _, r := range []RRule{{Freq: "abc"}, {Freq: "YEARLY"}, {Freq: "WEEKLY", ByDay: []string{"XX"}}} {
		if err := validateRRule(r); err == nil {
			t.Fatalf("非法规则 %+v 应报错", r)
		}
	}
	// 空/{} 规则 roundtrip：canonical 落 "{}"，parse 回空 freq → expandEvent 单场分支
	if got := canonicalRRule(RRule{}); got != "{}" {
		t.Fatalf("canonicalRRule(空) = %q, want {}", got)
	}
	back, err := parseRRule("{}")
	if err != nil || back.Freq != "" {
		t.Fatalf("parseRRule({}) = %+v, %v；want 空 freq 无错", back, err)
	}
}
