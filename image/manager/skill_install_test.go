package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeZip 构造内存 zip：files 是 path→content 映射（path 用正斜杠）。
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestExtractZipAndLocateRoot_SkillMdAtRoot 包内 SKILL.md 在根 → 根即 skill 根。
func TestExtractZipAndLocateRoot_SkillMdAtRoot(t *testing.T) {
	data := makeZip(t, map[string]string{
		"SKILL.md":         "---\nname: my-skill\n---\nhello",
		"scripts/run.sh":   "#!/bin/sh\necho hi",
	})
	dir, err := os.MkdirTemp("", "aa-skill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := extractZip(data, dir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	root, err := locateSkillRoot(dir)
	if err != nil {
		t.Fatalf("locateSkillRoot: %v", err)
	}
	if root != dir {
		t.Fatalf("应定位到根 %s，实得 %s", dir, root)
	}
	// 校验子文件已解压。
	if _, err := os.Stat(filepath.Join(dir, "scripts", "run.sh")); err != nil {
		t.Fatal("子文件未解压")
	}
}

// TestExtractZipAndLocateRoot_Nested 包内单一顶层目录套 SKILL.md → 定位到该子目录。
func TestExtractZipAndLocateRoot_Nested(t *testing.T) {
	data := makeZip(t, map[string]string{
		"my-skill/SKILL.md": "---\nname: my-skill\n---\n",
		"my-skill/a.txt":    "a",
	})
	dir, err := os.MkdirTemp("", "aa-skill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := extractZip(data, dir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	root, err := locateSkillRoot(dir)
	if err != nil {
		t.Fatalf("locateSkillRoot: %v", err)
	}
	want := filepath.Join(dir, "my-skill")
	if root != want {
		t.Fatalf("应定位到 %s，实得 %s", want, root)
	}
}

// TestExtractZip_PathTraversal 含 .. 的穿越条目被跳过，不逃出 dest。
func TestExtractZip_PathTraversal(t *testing.T) {
	data := makeZip(t, map[string]string{
		"SKILL.md":      "---\nname: s\n---\n",
		"../escaped.sh": "evil",
	})
	dir, err := os.MkdirTemp("", "aa-skill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := extractZip(data, dir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	parent := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(parent, "escaped.sh")); err == nil {
		t.Fatal("穿越条目被写到 dest 之外")
	}
}

// TestLocateSkillRoot_NotFound 无 SKILL.md → 报错。
func TestLocateSkillRoot_NotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "aa-skill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if _, err := locateSkillRoot(dir); err == nil {
		t.Fatal("缺 SKILL.md 应报错")
	}
}

// TestParseManagedSkillsDir 命中 slug → 返回 managedDir + present=true。
func TestParseManagedSkillsDir(t *testing.T) {
	json := `{"workspaceDir":"/ws","managedSkillsDir":"/aa/.openclaw/skills","skills":[{"name":"foo"},{"name":"bar"}]}`
	dir, present := parseManagedSkillsDir(json, "bar")
	if dir != "/aa/.openclaw/skills" {
		t.Fatalf("managedDir 实得 %q", dir)
	}
	if !present {
		t.Fatal("bar 应在列表")
	}
}

// TestParseManagedSkillsDir_Absent slug 不在列表 → present=false（卸载幂等成功用）。
func TestParseManagedSkillsDir_Absent(t *testing.T) {
	json := `{"managedSkillsDir":"/m","skills":[{"name":"foo"}]}`
	_, present := parseManagedSkillsDir(json, "gone")
	if present {
		t.Fatal("gone 不应在列表")
	}
}
