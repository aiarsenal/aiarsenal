package config

import (
	"os"
	"strings"
	"testing"
)

// ---- bootstrap 段（F-002）：env 注入 + yaml 真值不安全默认值告警 ----
// 变异锚点：删 AIARSENAL_ADMIN_PASSWORD env 注入 → env 格红；删 insecureDefaults
// 的 bootstrap.adminPassword 项 → yaml 真值格红；DEMO 解析放宽（"1" 也算 true）→ 格红。

func TestBootstrapEnvOverrides(t *testing.T) {
	// M6 关闭：走生产 config.Load 全链（env→viper→Bootstrap 段），而非测试内重演接线。
	t.Setenv("AIARSENAL_ADMIN_PASSWORD", "env-secret-pw-1234567890")
	t.Setenv("AIARSENAL_DEMO", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Bootstrap.AdminPassword != "env-secret-pw-1234567890" {
		t.Fatalf("env 口令应覆盖 yaml: %q", c.Bootstrap.AdminPassword)
	}
	if !c.Bootstrap.Demo {
		t.Fatal("AIARSENAL_DEMO=true 应置 demo")
	}
}

func TestBootstrapDemoParsingStrict(t *testing.T) {
	// 仅字面 "true" 生效（"1"/"TRUE"/其他值不启用）——与 deploy.sh 的判断一致。
	// 走生产 config.Load 全链（第二轮审计缺口 1：重演表达式杀不死放宽变异）。
	for val, want := range map[string]bool{"true": true, "1": false, "TRUE": false, "yes": false, "": false} {
		t.Setenv("AIARSENAL_DEMO", val)
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.Bootstrap.Demo != want {
			t.Fatalf("AIARSENAL_DEMO=%q 解析=%v, 期望 %v", val, c.Bootstrap.Demo, want)
		}
	}
	os.Unsetenv("AIARSENAL_DEMO")
}

func TestInsecureDefaultsAdminPasswordItem(t *testing.T) {
	// yaml 真值（FromFile 标记）→ 告警项；env 注入后（yaml 原值与生效值不等）→ 不告警
	cfg := &Config{}
	cfg.Bootstrap.AdminPassword = "yaml-truth-value-0123456789"
	cfg.adminPasswordFromFile = true
	got := cfg.insecureDefaults()
	found := false
	for _, it := range got {
		if it == "bootstrap.adminPassword" {
			found = true
		}
	}
	if !found {
		t.Fatalf("yaml 真值口令应触发 bootstrap.adminPassword 告警项, got %v", got)
	}
	// env 覆盖后 yaml 残值不再告警（与 jwt/pepper 家族同语义）
	cfg2 := &Config{}
	cfg2.Bootstrap.AdminPassword = "env-value- different"
	cfg2.adminPasswordFromFile = false
	for _, it := range cfg2.insecureDefaults() {
		if it == "bootstrap.adminPassword" {
			t.Fatal("非 yaml 来源不应触发告警项")
		}
	}
	_ = strings.TrimSpace("")
}
