package auth

import (
	"encoding/hex"
	"strings"
	"testing"
)

// 向量来源：GB/T 32905-2016 官方示例（与 gmsm v0.45.1 sm3_test.go 的 golden 表
// 逐字核对一致；开发期另做过随机输入对拍）。实现写错时这些断言必然红——这是
// 「本地实现 SM3」选型的安全网，不许删。
var sm3Golden = []struct {
	digest  string // 64 位十六进制
	message string
}{
	{"66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0", "abc"},
	{"debe9ff92275b8a138604889c18e5a4d6fdb70e5387e5765293dcba39c0c5732", strings.Repeat("abcd", 16)},                          // 恰好 64 字节
	{"952eb84cacee9c10bde4d6882d29d63140ba72af6fe485085095dccd5b872453", strings.Repeat("abcd", 16) + "abc"},                  // 67 字节：跨两个填充块
	{"90d52a2e85631a8d6035262626941fa11b85ce570cec1e3e991e2dd7ed258148", strings.Repeat("abcd", 32)},                          // 128 字节
	{"e1c53f367a9c5d19ab6ddd30248a7dafcc607e74e6bcfa52b00e0ba35e470421", strings.Repeat("abcd", 32) + "abc"},                  // 131 字节
	{"520472cafdaf21d994c5849492ba802459472b5206503389fc81ff73adbec1b4", strings.Repeat(strings.Repeat("abcd", 32)+"abc", 2)}, // 262 字节：四块消息+跨两块填充
}

func TestSM3OfficialVectors(t *testing.T) {
	for _, tc := range sm3Golden {
		h := sm3New()
		h.Write([]byte(tc.message))
		got := hex.EncodeToString(h.Sum(nil))
		if got != tc.digest {
			t.Fatalf("SM3(%q) = %s，期望 %s", tc.message, got, tc.digest)
		}
	}
}

// TestSM3StreamingEqualsSingle 分块写入必须与一次写入等价（增量接口正确性）。
func TestSM3StreamingEqualsSingle(t *testing.T) {
	msg := []byte(strings.Repeat("aiarsenal 口令加固 ", 50))
	single := sm3New()
	single.Write(msg)
	want := hex.EncodeToString(single.Sum(nil))

	stream := sm3New()
	for i := 0; i < len(msg); i += 7 { // 非对齐小块
		stream.Write(msg[i:min(i+7, len(msg))])
	}
	got := hex.EncodeToString(stream.Sum(nil))
	if got != want {
		t.Fatalf("分块写入与整段写入不一致：stream=%s single=%s", got, want)
	}
}

// TestSM3SumKeepsState Sum 不得消费状态：追加写后继续算，结果必须与一次写齐平。
func TestSM3SumKeepsState(t *testing.T) {
	h := sm3New()
	h.Write([]byte("abc"))
	_ = h.Sum(nil)
	h.Write([]byte("def"))
	got := hex.EncodeToString(h.Sum(nil))

	want := sm3New()
	want.Write([]byte("abcdef"))
	if got != hex.EncodeToString(want.Sum(nil)) {
		t.Fatalf("Sum 后状态被破坏：got %s", got)
	}
}

// TestSM3ResetReuse Reset 后可复用，结果与全新实例一致。
func TestSM3ResetReuse(t *testing.T) {
	h := sm3New()
	h.Write([]byte("stale state"))
	h.Reset()
	h.Write([]byte("abc"))
	if got := hex.EncodeToString(h.Sum(nil)); got != sm3Golden[0].digest {
		t.Fatalf("Reset 后结果错误：%s", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
