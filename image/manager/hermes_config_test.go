package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEffectiveConfig 须无条件叠加审批关闭块与 onboarding 关闭块（消息端审批卡/查户口用户不可达）；
// base 显式声明同名顶级键时尊重 base 不重复叠加；已绑定时再叠 weixin 平台块且互不破坏。
func TestWriteEffectiveConfigApprovals(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)

	writeBase := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "config.base.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	readEffective := func() string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	// 未绑定微信：base + approvals off + onboarding off 两块。
	writeBase("model:\n  provider: custom\n")
	if err := writeEffectiveConfig(); err != nil {
		t.Fatal(err)
	}
	out := readEffective()
	if !strings.Contains(out, "approvals:") || !strings.Contains(out, `mode: "off"`) {
		t.Fatalf("config.yaml 缺审批关闭块:\n%s", out)
	}
	if !strings.Contains(out, "onboarding:") || !strings.Contains(out, `profile_build: "off"`) {
		t.Fatalf("config.yaml 缺 onboarding 关闭块:\n%s", out)
	}
	if !strings.Contains(out, "provider: custom") {
		t.Fatalf("config.yaml 丢了 base 内容:\n%s", out)
	}

	// base 已显式声明 approvals: → 尊重 base，不叠加 off（onboarding 仍独立叠加）。
	writeBase("approvals:\n  mode: \"smart\"\n")
	if err := writeEffectiveConfig(); err != nil {
		t.Fatal(err)
	}
	out = readEffective()
	if strings.Count(out, "approvals:") != 1 || strings.Contains(out, `mode: "off"`) {
		t.Fatalf("approvals 块叠加异常（应尊重 base 的 smart）:\n%s", out)
	}
	if !strings.Contains(out, `profile_build: "off"`) {
		t.Fatalf("base 声明 approvals 不应影响 onboarding 叠加:\n%s", out)
	}

	// base 显式声明 onboarding: → 尊重 base，不叠加 off（approvals 仍独立叠加）。
	writeBase("onboarding:\n  profile_build: \"ask\"\n")
	if err := writeEffectiveConfig(); err != nil {
		t.Fatal(err)
	}
	out = readEffective()
	if strings.Count(out, "onboarding:") != 1 || !strings.Contains(out, "profile_build: \"ask\"") {
		t.Fatalf("onboarding 块叠加异常（应尊重 base 的 ask）:\n%s", out)
	}
	if !strings.Contains(out, `mode: "off"`) {
		t.Fatalf("base 声明 onboarding 不应影响 approvals 叠加:\n%s", out)
	}

	// 已绑定微信：全部策略块 + weixin 平台块共存，YAML 顶层段完整、顺序正确。
	writeBase("model:\n  provider: custom\n")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("WEIXIN_TOKEN=t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeEffectiveConfig(); err != nil {
		t.Fatal(err)
	}
	out = readEffective()
	for _, want := range []string{`mode: "off"`, `profile_build: "off"`, "platforms:", "dm_policy:", "provider: custom"} {
		if !strings.Contains(out, want) {
			t.Fatalf("绑定态 config.yaml 缺 %q:\n%s", want, out)
		}
	}
	// approvals → onboarding → gateway 依序拼接，缩进正确互不吞并。
	if !strings.Contains(out, "mode: \"off\"\nonboarding:\n  profile_build: \"off\"\ngateway:") {
		t.Fatalf("策略块与 gateway 块拼接异常:\n%s", out)
	}
}

// channelNotifyText link 只拼 http(s) 绝对地址：站内相对路径（/agent/…）在微信里不可点
// 也读不懂，拼出去是一行英文噪音；http(s) 照拼；空/其它 scheme 一律不拼。
func TestChannelNotifyText(t *testing.T) {
	cases := []struct {
		name, title, content, link, want string
	}{
		{"相对路径不拼", "技能已安装", "技能「kb」已安装到实例「demo」。", "/agent/instance/list",
			"技能已安装\n技能「kb」已安装到实例「demo」。"},
		{"https 照拼", "提醒", "预算已批", "https://aiarsenal.cc/agent/instance/list",
			"提醒\n预算已批\nhttps://aiarsenal.cc/agent/instance/list"},
		{"http 照拼", "提醒", "预算已批", "http://aiarsenal.cc/x", "提醒\n预算已批\nhttp://aiarsenal.cc/x"},
		{"空 link 不拼", "标题", "内容", "", "标题\n内容"},
	}
	for _, c := range cases {
		if got := channelNotifyText(c.title, c.content, c.link); got != c.want {
			t.Fatalf("%s: channelNotifyText=%q want %q", c.name, got, c.want)
		}
	}
}

// regenEffectiveConfigIfProvisioned 启动期折叠：存量实例（base+config 均在）补叠加块；
// base 缺失不动 config.yaml（防误清模型配置）；未下发配置的新实例不生成 config.yaml。
func TestRegenEffectiveConfigIfProvisioned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	configPath := filepath.Join(dir, "config.yaml")
	basePath := filepath.Join(dir, "config.base.yaml")
	writeFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// 存量实例：老格式 config.yaml（无 approvals）+ base → 重启折叠后带上 approvals off。
	writeFile(basePath, "model:\n  provider: custom\n")
	writeFile(configPath, "model:\n  provider: custom\n")
	regenEffectiveConfigIfProvisioned()
	data, _ := os.ReadFile(configPath)
	if !strings.Contains(string(data), `mode: "off"`) {
		t.Fatalf("存量实例重启未补审批块:\n%s", data)
	}

	// base 缺失（更早的存量实例）：config.yaml 原样不动。
	os.Remove(basePath)
	writeFile(configPath, "model:\n  provider: custom\n")
	regenEffectiveConfigIfProvisioned()
	data, _ = os.ReadFile(configPath)
	if string(data) != "model:\n  provider: custom\n" {
		t.Fatalf("base 缺失时 config.yaml 被改动:\n%s", data)
	}

	// 未下发配置的新实例：不生成 config.yaml。
	os.Remove(configPath)
	regenEffectiveConfigIfProvisioned()
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("未就绪实例不应生成 config.yaml")
	}
}
