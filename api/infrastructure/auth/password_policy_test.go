package auth

import (
	"strings"
	"testing"

	"aiarsenal/config"
)

// setPolicyLevel 临时改档位并在用例结束后恢复（config.Get() 是全局单例）。
func setPolicyLevel(t *testing.T, level string) {
	t.Helper()
	old := config.Get().Password.Policy.Level
	config.Get().Password.Policy.Level = level
	t.Cleanup(func() { config.Get().Password.Policy.Level = old })
}

// TestPolicyStandard 等保基线档：长度/字符类/账号名/黑名单逐条覆盖，含聚合报错文案。
func TestPolicyStandard(t *testing.T) {
	setPolicyLevel(t, "standard")

	cases := []struct {
		name     string
		password string
		username string
		wantErr  bool
		wantMsg  []string // 命中时报错文案必须含这些条目
	}{
		{"标准合规口令", "Aiarsenal@2026", "hi", false, nil},
		{"长度不足", "Ab1@x", "hi", true, []string{"长度至少 8 位"}},
		{"单一字符类", "12345678", "hi", true, []string{"至少 3 类"}},
		{"两类字符", "abcd1234", "hi", true, []string{"至少 3 类"}},
		{"缺特殊字符但长度够且三类", "Abcdefg1", "hi", false, nil},
		{"含账号名（大小写变体）", "HI123456a!", "hi", true, []string{"不能包含账号名"}},
		{"黑名单命中", "password", "hi", true, []string{"黑名单"}},
		{"黑名单大小写变体", "Password", "hi", true, []string{"黑名单"}},
		{"聚合多条", "hi", "hi", true, []string{"长度至少 8 位", "至少 3 类", "不能包含账号名"}},
	}
	for _, tc := range cases {
		err := ValidatePasswordPolicy(tc.password, tc.username)
		if tc.wantErr && err == nil {
			t.Fatalf("%s：应当被拒绝", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s：不应被拒绝（%v）", tc.name, err)
		}
		for _, want := range tc.wantMsg {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s：报错文案缺少条目 %q（err=%v）", tc.name, want, err)
			}
		}
	}
}

// TestPolicyLoose loose 档只看长度：三类/账号名/黑名单都不管。
func TestPolicyLoose(t *testing.T) {
	setPolicyLevel(t, "loose")

	if err := ValidatePasswordPolicy("abcdefgh", "hi"); err != nil {
		t.Fatalf("loose 档下纯小写 8 位应通过（%v）", err)
	}
	if err := ValidatePasswordPolicy("password", "hi"); err != nil {
		t.Fatalf("loose 档下黑名单口令也应通过（仅长度语义）：%v", err)
	}
	if err := ValidatePasswordPolicy("1234567", "hi"); err == nil {
		t.Fatal("loose 档下 7 位仍应拒绝")
	}
}

// TestPolicyStrict strict 档：12 位 + 四类全含。
func TestPolicyStrict(t *testing.T) {
	setPolicyLevel(t, "strict")

	if err := ValidatePasswordPolicy("Aiarsenal@2026", "hi"); err != nil {
		t.Fatalf("14 位四类口令应过 strict：%v", err)
	}
	if err := ValidatePasswordPolicy("Aa123456!", "hi"); err == nil {
		t.Fatal("9 位四类口令应被 strict 拒绝")
	}
	// 12 位但只三类（缺特殊）：strict 拒、standard 过——档位差异的边界
	if err := ValidatePasswordPolicy("Abcdefgh1234", "hi"); err == nil {
		t.Fatal("12 位三类应被 strict 拒绝")
	}
	setPolicyLevel(t, "standard")
	if err := ValidatePasswordPolicy("Abcdefgh1234", "hi"); err != nil {
		t.Fatalf("12 位三类在 standard 档应通过：%v", err)
	}
}

// TestPolicyLengthCap 超长口令拒绝：PBKDF2 的 CPU 成本不能被请求方无限放大。
func TestPolicyLengthCap(t *testing.T) {
	setPolicyLevel(t, "loose") // 即便最宽松档也拦长度上限
	long := strings.Repeat("a", 129)
	if err := ValidatePasswordPolicy(long, "hi"); err == nil {
		t.Fatal("129 字符口令应被拒绝")
	}
	if err := ValidatePasswordPolicy(strings.Repeat("a", 128), "hi"); err != nil {
		t.Fatalf("128 字符口令应通过（%v）", err)
	}
}

// TestBlacklistLoaded 黑名单实际载入（资源内嵌正确性）。
func TestBlacklistLoaded(t *testing.T) {
	if len(passwordBlacklist) < 900 {
		t.Fatalf("黑名单条目数异常：%d（应≈1000）", len(passwordBlacklist))
	}
	if _, ok := passwordBlacklist["123456"]; !ok {
		t.Fatal("黑名单缺 123456")
	}
}
