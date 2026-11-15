package proxy

import (
	"testing"

	"aiarsenal/llm/internal/model"
)

// F-024 预估函数表测：预留量决定闸门咬合精度。
func TestEstimateRequestMilli(t *testing.T) {
	in := model.Milli(333333)
	out := model.Milli(333333)

	// max_tokens 显式给定：est = (rune×in + mt×out)/1M
	body := []byte(`{"model":"m1","messages":[{"role":"user","content":"hello"}],"max_tokens":100}`)
	want := model.CostMilli(len(string(body)), 100, in, out)
	if got := estimateRequestMilli(body, in, out); got != want {
		t.Fatalf("est=%d want=%d", got, want)
	}

	// 无 max_tokens → 缺省 4096（保守预留）
	body2 := []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)
	want2 := model.CostMilli(len(string(body2)), 4096, in, out)
	if got := estimateRequestMilli(body2, in, out); got != want2 {
		t.Fatalf("缺省输出上限 est=%d want=%d", got, want2)
	}

	// CJK 正文 rune 数 > 字节数/4：估量按 rune（保守方向）
	body3 := []byte(`{"model":"m1","messages":[{"role":"user","content":"你好世界预算探针"}]}`)
	if est3 := estimateRequestMilli(body3, in, out); est3 <= 0 {
		t.Fatal("CJK 请求预估应>0")
	}

	// 未配价模型：不预估（无成本面）
	if got := estimateRequestMilli(body, 0, 0); got != 0 {
		t.Fatalf("未配价 est 应=0, 得 %d", got)
	}
}
