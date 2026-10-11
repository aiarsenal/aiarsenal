package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHermesPushSoul 幂等追加语义：已有 SOUL.md（人格）→ 追加铁律且人格保留；重复推送 → 不重复追加。
func TestHermesPushSoul(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	const persona = "You are Hermes Agent, built by Nous Research. Be direct."
	const mandate = "## 技能安装铁律（强制默认）\n- 一律走平台技能工具；\n- 禁止 curl|bash。"

	if err := os.WriteFile(filepath.Join(home, "SOUL.md"), []byte(persona+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewHermesManager()
	if err := m.PushSoul(mandate); err != nil {
		t.Fatalf("PushSoul: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, "SOUL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), persona) {
		t.Fatalf("人格段被覆盖：\n%s", got)
	}
	if !strings.Contains(string(got), mandate) {
		t.Fatalf("铁律未追加：\n%s", got)
	}
	if strings.Count(string(got), "## 技能安装铁律") != 1 {
		t.Fatalf("铁律段应恰有一段：\n%s", got)
	}

	// 幂等：重复推送不重复追加。
	if err := m.PushSoul(mandate); err != nil {
		t.Fatalf("PushSoul 重发: %v", err)
	}
	got, _ = os.ReadFile(filepath.Join(home, "SOUL.md"))
	if strings.Count(string(got), "## 技能安装铁律") != 1 {
		t.Fatalf("重发后铁律重复：\n%s", got)
	}
}

// TestHermesPushSoul_NoExistingFile SOUL.md 不存在（极端：镜像种入失败）→ 直接写入铁律，不报错。
func TestHermesPushSoul_NoExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	m := NewHermesManager()
	if err := m.PushSoul("## 铁律\n- 规则"); err != nil {
		t.Fatalf("PushSoul: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, "SOUL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "## 铁律") {
		t.Fatalf("无既有文件时应以铁律开头：\n%s", got)
	}
}
