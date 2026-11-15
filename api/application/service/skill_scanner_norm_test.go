package service

import (
	"strings"
	"testing"

	"aiarsenal/domain/entity"
)

// F-013 扩展名归一化表测：审计报告 16 组变体的四类绕过必须全数拦截。
// 变异背书——去 NFKC 折叠（全角格红）、去尾点尾空格剥离（尾缀格红）、
// 去逐段比对（双扩展格红）、改回 filepath.Ext 单段比对（多格红）。
func TestHasBannedExtension(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"基线纯小写", "tool.exe", true},
		{"大小写变体", "tool.exE", true},
		{"尾点(Windows抽取剥尾点)", "tool.exe.", true},
		{"尾空格(Windows抽取剥尾空格)", "tool.exe ", true},
		{"尾点尾空格连用", "tool.exe. . ", true},
		{"双扩展名", "tool.exe.txt", true},
		{"全角形近字符", "tool.eｘe", true},
		{"全角句点", "tool．exe", true},
		{"多级扩展中间段命中", "payload.jar.zip", true},
		{"目录前缀", "libs/bin/tool.so", true},
		{"隐藏文件形态", ".exe", true},
		{"带版本目录+全角+双扩展叠加", "v1.2/tool.eｘe.txt", true},
		// 合法名不得误杀
		{"普通脚本", "scripts/run.py", false},
		{"文档", "README.md", false},
		{"多级合法扩展", "archive.tar.gz", false},
		{"无扩展名", "Dockerfile", false},
		{"点开头无扩展名", ".gitignore", false},
		{"全角但非黑名单", "说明．md", false},
	}
	for _, c := range cases {
		if got := hasBannedExtension(c.path); got != c.want {
			t.Errorf("hasBannedExtension(%q)=%v, 期望 %v", c.path, got, c.want)
		}
	}
}

// F-013 内容聚合哈希：确定性、与文件列表顺序无关（排序钉死）、单文件敏感。
// 变异背书——去掉 sort.Strings 后乱序输入格必红。
func TestSkillContentHash(t *testing.T) {
	mk := func(p, sha string) *entity.SkillFile {
		return &entity.SkillFile{FilePath: p, Sha256: sha}
	}
	a := mk("a.md", "111")
	b := mk("dir/b.exe", "222")
	c := mk("c.txt", "333")

	h1 := skillContentHash([]*entity.SkillFile{a, b, c})
	h2 := skillContentHash([]*entity.SkillFile{c, a, b}) // 乱序同集合
	if h1 == "" || h1 != h2 {
		t.Fatalf("聚合哈希应与列表顺序无关：h1=%s h2=%s", h1, h2)
	}
	if len(h1) != 64 || strings.ToLower(h1) != h1 {
		t.Fatalf("应为 64 位小写 hex: %s", h1)
	}
	// 单文件内容变化 → 哈希必变
	b2 := mk("dir/b.exe", "666")
	h3 := skillContentHash([]*entity.SkillFile{a, b2, c})
	if h3 == h1 {
		t.Fatal("文件内容哈希变化后聚合哈希未变——门禁内容绑定失效")
	}
	// 文件集增减 → 哈希必变
	h4 := skillContentHash([]*entity.SkillFile{a, b, c, mk("d.py", "444")})
	if h4 == h1 {
		t.Fatal("文件集增删后聚合哈希未变")
	}
	// 空集 → 空串（扫描期不该发生，门禁侧由「无审计/哈希空」格拦截）
	if skillContentHash(nil) != "" {
		t.Fatal("空文件集应返回空串")
	}
}
