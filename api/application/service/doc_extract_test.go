package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 合成样本（非任何真实文档内容）验证抽取/清洗/分块行为。
func TestCleanText(t *testing.T) {
	raw := "商业秘密\n（普通商密）\n标题行内容足够\n字\n---\n123\nABC定义说明条目\n \n正文第二段"
	got := CleanText(raw)
	want := "标题行内容足够\nABC定义说明条目\n正文第二段"
	if got != want {
		t.Fatalf("CleanText = %q, want %q", got, want)
	}
}

func TestChunkText(t *testing.T) {
	// 20 行 × 100 字 = 2000 字 → minRunes 700 应切出 3 块左右，每块 ≤ 800
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(strings.Repeat("段", 99) + "\n")
	}
	chunks := ChunkText(b.String(), 700, 800)
	if len(chunks) < 2 || len(chunks) > 4 {
		t.Fatalf("块数 %d 不合预期（2-4）", len(chunks))
	}
	total := 0
	for i, c := range chunks {
		n := utf8.RuneCountInString(c)
		if n > 800 {
			t.Fatalf("块 %d 超长：%d 字", i, n)
		}
		total += n
	}
	if total < 1900 {
		t.Fatalf("分块丢字：总字数 %d", total)
	}
	// 重组应等于原文（去空行后）
	joined := strings.Join(chunks, "\n")
	if strings.Count(joined, "段") != 20*99 {
		t.Fatalf("内容丢失：%d", strings.Count(joined, "段"))
	}
}

func TestChunkTextShort(t *testing.T) {
	if got := ChunkText("  \n", 700, 800); got != nil {
		t.Fatalf("空文本应无块，得 %v", got)
	}
	one := ChunkText("短文本一行", 700, 800)
	if len(one) != 1 || one[0] != "短文本一行" {
		t.Fatalf("短文本应整块返回，得 %v", one)
	}
}

func TestExtractTextUnknown(t *testing.T) {
	got, err := ExtractText("txt", []byte("hello 世界内容\n单\n"))
	if err != nil || got != "hello 世界内容" {
		t.Fatalf("txt 抽取：%v %q", err, got)
	}
	if got, _ := ExtractText("ppt", []byte("whatever")); got != "" {
		t.Fatalf("ppt 应只留空正文，得 %q", got)
	}
}
