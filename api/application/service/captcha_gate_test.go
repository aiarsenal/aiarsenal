package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---- captchaGate 验证码是否必须校验（F-007：debug 跳过仅限回环来源）----
// 变异锚点：删回环条件（debug 单独跳过）→ 远端格红；&& 改 || → 回环非 debug 格红。

func TestCaptchaGate(t *testing.T) {
	cases := []struct {
		name  string
		debug bool
		ip    string
		want  bool // true=必须校验验证码
	}{
		{name: "非 debug 一律校验", debug: false, ip: "127.0.0.1", want: true},
		{name: "debug 加回环 v4 跳过（开发体验保留）", debug: true, ip: "127.0.0.1", want: false},
		{name: "debug 加回环 v6 跳过", debug: true, ip: "::1", want: false},
		{name: "debug 加局域网来源仍校验（F-007 主场景）", debug: true, ip: "192.168.1.9", want: true},
		{name: "debug 加公网来源仍校验", debug: true, ip: "8.8.8.8", want: true},
		{name: "debug 加不可解析 IP 仍校验（fail-closed）", debug: true, ip: "not-an-ip", want: true},
		{name: "debug 加空 IP 仍校验", debug: true, ip: "", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := captchaGate(tc.debug, tc.ip); got != tc.want {
				t.Fatalf("captchaGate(debug=%v, ip=%q) = %v, want %v", tc.debug, tc.ip, got, tc.want)
			}
		})
	}
}

func TestIsLoopbackIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.9.9.9", true},
		{"::1", true},
		{"192.168.1.9", false},
		{"0.0.0.0", false},
		{"", false},
		{"garbage", false},
	}
	for _, tc := range cases {
		if got := isLoopbackIP(tc.ip); got != tc.want {
			t.Fatalf("isLoopbackIP(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

// ---- VerifyString 空白答案守卫（F-029）----
// 上游 base64Captcha 的 Verify 对取不到的 key 返回空答案，与去空白后的空答案
// EqualFold 相等——缺守卫时「key 不存在 + 答案是空格」判通过。变异锚点：删空白
// 早退，空白格全红。

func TestVerifyStringBlankAnswerGuard(t *testing.T) {
	s := NewCaptchaService()
	id, _, err := s.GetVerifyImgString()
	if err != nil {
		t.Fatalf("生成验证码失败: %v", err)
	}

	cases := []struct {
		name   string
		key    string
		answer string
		want   bool
	}{
		{name: "key 不存在加单空格（F-029 攻击形态）", key: "attacker-key", answer: " ", want: false},
		{name: "key 不存在加全角空格", key: "attacker-key", answer: "　", want: false},
		{name: "key 存在加单空格", key: id, answer: " ", want: false},
		{name: "key 存在加空串", key: id, answer: "", want: false},
		{name: "key 为空", key: "", answer: " ", want: false},
		{name: "key 不存在加正常答案（既有语义回归）", key: "attacker-key", answer: "ab12", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.VerifyString(tc.key, tc.answer); got != tc.want {
				t.Fatalf("VerifyString(%q, %q) = %v, want %v", tc.key, tc.answer, got, tc.want)
			}
		})
	}

	// 正样本对照（恒拒绝变异体的唯一击杀者）：独立生成 id——共享的 DefaultMemStore
	// 会在错误答案格被 clear=true 烧码，复用上面的 id 会让本格依赖用例执行顺序。
	t.Run("key 存在加正确答案（正样本对照，独立 id）", func(t *testing.T) {
		id2, _, err := s.GetVerifyImgString()
		if err != nil {
			t.Fatalf("生成验证码失败: %v", err)
		}
		real := s.store.Get(id2, false)
		if strings.TrimSpace(real) == "" {
			t.Fatalf("store 中的答案不应为空")
		}
		if !s.VerifyString(id2, real) {
			t.Fatalf("正确答案应判通过（守卫不得把闸门改成恒拒绝）")
		}
	})
}

// TestLoginCaptchaGateWiring 接线守卫：Login 的验证码分支必须经
// captchaGate(s.isDebug(), ip) 判定（F-007）。纯函数测试杀不掉「接线被删」的
// 变异（审计变异 D 实测：接线改 if false 全包测试依旧绿），此守卫以源码断言兜底。
func TestLoginCaptchaGateWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "auth_service.go"))
	if err != nil {
		t.Fatalf("读取 auth_service.go: %v", err)
	}
	// 锚定完整接线行而非子串：`if false && captchaGate(...)` 这类禁用式改写
	// 仍包含函数名子串，只有整行匹配才杀得掉（审计变异 D 实测教训）。
	if !strings.Contains(string(src), "if captchaGate(s.isDebug(), ip) {") {
		t.Fatal("auth_service.go 的验证码接线丢失：Login 必须经 captchaGate(isDebug, ip) 判定（F-007 回归）")
	}
}
