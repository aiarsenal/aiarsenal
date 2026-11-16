package proxy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// F-031 残余收口（审计建议项）：审计行脱敏——Gemini 上游 URL 内嵌 ?key=<APIKey>，
// 传输层错误文本进 aa_llm_error_log 会落活凭据；只剥查询串、保留 host/path。
func TestRedactURLQueries(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Gemini 错误形态：URL 内嵌 key",
			in:   `Post "https://host/v1beta/models/m:generateContent?key=SECRET": dial tcp: bad`,
			want: `Post "https://host/v1beta/models/m:generateContent?<redacted>": dial tcp: bad`,
		},
		{
			name: "无查询串的 URL 原样保留",
			in:   `Post "https://host/v1/chat/completions": dial tcp: bad`,
			want: `Post "https://host/v1/chat/completions": dial tcp: bad`,
		},
		{
			name: "非 URL 文本原样保留",
			in:   `credential missing for provider`,
			want: `credential missing for provider`,
		},
		{
			name: "单引号/无引号 URL 也剥",
			in:   `Get http://host/path?key=abc failed`,
			want: `Get http://host/path?<redacted> failed`,
		},
	}
	for _, tc := range cases {
		if got := redactURLQueries(tc.in); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

// TestRecordErrorRedactWiring 接线守卫：recordError 必须在写审计行前调用脱敏
// （纯函数测试杀不掉「接线被删」，源码断言兜底——与字节闸守卫同范式）。
func TestRecordErrorRedactWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "handler.go"))
	if err != nil {
		t.Fatalf("读取 handler.go: %v", err)
	}
	if !strings.Contains(string(src), "excStr = redactURLQueries(excStr)") {
		t.Fatal("recordError 的脱敏接线丢失——Gemini ?key= 凭据落库面复活（F-031 补口回归）")
	}
}
