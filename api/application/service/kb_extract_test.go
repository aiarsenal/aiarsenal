package service

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// 合成样本（非任何真实文档内容）验证抽取/清洗/分块行为。
func TestKbCleanText(t *testing.T) {
	raw := "商业秘密\n（普通商密）\n标题行内容足够\n字\n---\n123\nABC定义说明条目\n \n正文第二段"
	got := KbCleanText(raw)
	want := "标题行内容足够\nABC定义说明条目\n正文第二段"
	if got != want {
		t.Fatalf("KbCleanText = %q, want %q", got, want)
	}
}

func TestKbChunkText(t *testing.T) {
	// 20 行 × 100 字 = 2000 字 → minRunes 700 应切出 3 块左右，每块 ≤ 800
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(strings.Repeat("段", 99) + "\n")
	}
	chunks := KbChunkText(b.String(), 700, 800)
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

func TestKbChunkTextShort(t *testing.T) {
	if got := KbChunkText("  \n", 700, 800); got != nil {
		t.Fatalf("空文本应无块，得 %v", got)
	}
	one := KbChunkText("短文本一行", 700, 800)
	if len(one) != 1 || one[0] != "短文本一行" {
		t.Fatalf("短文本应整块返回，得 %v", one)
	}
}

func TestKbExtractTextUnknown(t *testing.T) {
	got, err := KbExtractText("txt", []byte("hello 世界内容\n单\n"))
	if err != nil || got != "hello 世界内容" {
		t.Fatalf("txt 抽取：%v %q", err, got)
	}
	if got, _ := KbExtractText("ppt", []byte("whatever")); got != "" {
		t.Fatalf("ppt 应只留空正文，得 %q", got)
	}
}

func TestKbHTMLishToText(t *testing.T) {
	got := kbHTMLishToText("<p>甲乙丙&nbsp;</p><p>丁戊己&nbsp;</p>")
	if got != "甲乙丙\u00a0\n丁戊己\u00a0\n" { // &nbsp; 按 HTML5 解码为 U+00A0，行尾清理由 KbCleanText 做
		t.Fatalf("kbHTMLishToText = %q", got)
	}
}

// kbExtractPDF 同款管线（剥标记→清洗→去乱码→去重复行）对合成水印样本的效果。
func TestKbPDFNoisePipeline(t *testing.T) {
	// 平铺水印：每页 12 枚同一文案的 <p> 水印（真实企业 PDF 的实测形态），共 17 页
	stamp := "<p>某局内部资料，仅限ABC123使用，2023-05-24&nbsp;</p>"
	var b strings.Builder
	for p := 0; p < 17; p++ {
		b.WriteString(strings.Repeat(stamp, 12))
		fmt.Fprintf(&b, "\n第%d章 内容各不相同\n", p+1)
	}
	b.WriteString("5\t1Í,~1×*<-$=>Öp<\t;5\t1Íü.¤]=,~]¸\n") // 字体损坏的公式残字
	b.WriteString("Total Quality Management 2024\n")       // 正常英文行应保留
	raw := b.String()

	got := kbDropRepeatedLines(kbDropGarbledLines(KbCleanText(kbHTMLishToText(raw))), kbWatermarkRepeatMin)
	if strings.Contains(got, "内部资料") {
		t.Fatalf("水印未除净：%q", got)
	}
	if strings.Contains(got, "<p>") || strings.Contains(got, "&nbsp;") {
		t.Fatalf("HTML 标记未剥净：%q", got)
	}
	if !strings.Contains(got, "第1章 内容各不相同") || !strings.Contains(got, "第17章 内容各不相同") || !strings.Contains(got, "Total Quality Management 2024") {
		t.Fatalf("正文被误伤：%q", got)
	}
	if strings.ContainsAny(got, "ÍÖ¤¸") {
		t.Fatalf("乱码行未除净：%q", got)
	}
}

func TestKbDropRepeatedLinesThreshold(t *testing.T) {
	// 表格短词重复 5 次（实测企业手册上限）应保留；水印级重复（≥8）才整行丢弃
	text := strings.Repeat("序号\n", 5) + strings.Repeat("表格编号\n", 8) + "正文行\n"
	got := kbDropRepeatedLines(text, kbWatermarkRepeatMin)
	if !strings.Contains(got, "序号") {
		t.Fatalf("5 次重复的表格词被误删：%q", got)
	}
	if strings.Contains(got, "表格编号") {
		t.Fatalf("8 次重复的水印行未删：%q", got)
	}
	if !strings.Contains(got, "正文行") {
		t.Fatalf("正文被误伤：%q", got)
	}
}

func TestKbLineLooksGarbled(t *testing.T) {
	garbled := []string{
		"5\t1Í,~1×*<-$=>Öp<\t;5\t1Íü.¤]=,~]¸",
		"=2<\t;5\t1ÍÔ*<",
		"ßÑ7P5#„F:[C^HÃ\t}¸",
		"(¯lÐb'=K>",
	}
	for _, g := range garbled {
		if !kbLineLooksGarbled(g) {
			t.Fatalf("应判乱码：%q", g)
		}
	}
	keep := []string{
		"分包结算资料报送要求，附全套表单",              // 中文正文
		"Total Quality Management 2024", // 纯 ASCII 英文
		"尺寸 50×30mm，误差±0.5°",            // 白名单符号（×±°）且含 CJK
		"编号 XJ-2024-05",                 // 无可疑字符的编号行
	}
	for _, k := range keep {
		if kbLineLooksGarbled(k) {
			t.Fatalf("不应判乱码：%q", k)
		}
	}
}
