package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// 兼容性回归：全部夹具程序化合成（合成内容，非业务文档）。

func TestDecodeTextBytesEncodings(t *testing.T) {
	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("你好，世界。合同编号：A-1024"))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeTextBytes(gbk); !strings.Contains(got, "合同编号") {
		t.Fatalf("GBK 解码失败：%q", got)
	}
	big5, err := traditionalchinese.Big5.NewEncoder().Bytes([]byte("台北公司年度報告"))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeTextBytes(big5); !strings.Contains(got, "年度報告") {
		t.Fatalf("Big5 解码失败：%q", got)
	}
	// UTF-8 BOM 剥除
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte("带BOM的文本")...)
	if got := decodeTextBytes(bom); got != "带BOM的文本" {
		t.Fatalf("UTF-8 BOM 未剥除：%q", got)
	}
	// UTF-16LE（Windows 记事本「Unicode」）
	u16 := utf16.Encode([]rune("UTF16编码文本"))
	le := make([]byte, 0, 2+2*len(u16))
	le = append(le, 0xFF, 0xFE)
	buf := make([]byte, 2)
	for _, c := range u16 {
		binary.LittleEndian.PutUint16(buf, c)
		le = append(le, buf...)
	}
	if got := decodeTextBytes(le); !strings.Contains(got, "UTF16编码文本") {
		t.Fatalf("UTF-16LE 解码失败：%q", got)
	}
}

func TestExtractXLSXPPTXOFD(t *testing.T) {
	// 合成 xlsx：共享字符串表 + 一个 sheet
	xlsx := buildZip(map[string]string{
		"xl/sharedStrings.xml": `<sst><si><t>部门</t></si><si><t>季度报销汇总表</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c r="A1" t="s"><v>0</v></c></row>` +
			`<row><c r="B2"><v>1024</v></c></row></sheetData></worksheet>`,
	})
	if got, err := ExtractText("xlsx", xlsx); err != nil || !strings.Contains(got, "季度报销汇总表") {
		t.Fatalf("xlsx 抽取失败：%v %q", err, got)
	}
	// 合成 pptx：两页幻灯
	pptx := buildZip(map[string]string{
		"ppt/slides/slide1.xml": `<p:sld><a:p><a:t>第一页：项目背景</a:t></a:p></p:sld>`,
		"ppt/slides/slide2.xml": `<p:sld><a:p><a:t>第二页：里程碑计划</a:t></a:p></p:sld>`,
	})
	got, err := ExtractText("pptx", pptx)
	if err != nil || !strings.Contains(got, "项目背景") || !strings.Contains(got, "里程碑计划") {
		t.Fatalf("pptx 抽取失败：%v %q", err, got)
	}
	if strings.Index(got, "项目背景") > strings.Index(got, "里程碑计划") {
		t.Fatalf("pptx 页序错乱：%q", got)
	}
	// 合成 ofd（电子发票结构）：TextCode 节点
	ofd := buildZip(map[string]string{
		"OFD.xml": `<ofd:OFD><ofd:DocBody/></ofd:OFD>`,
		"Doc_0/Attachs/Original.xml": `<File><TextCode>发票代码：044032000110</TextCode>` +
			`<TextCode>价税合计：壹佰元整</TextCode></File>`,
	})
	if got, err := ExtractText("ofd", ofd); err != nil || !strings.Contains(got, "价税合计") {
		t.Fatalf("ofd 抽取失败：%v %q", err, got)
	}
}

func TestExtractHTMLCSV(t *testing.T) {
	htmlSrc := `<html><body><h1>服务器运维规范</h1><p>值班表每日更新 &amp; 归档</p></body></html>`
	if got, err := ExtractText("html", []byte(htmlSrc)); err != nil || !strings.Contains(got, "运维规范") || !strings.Contains(got, "更新 & 归档") {
		t.Fatalf("html 抽取失败：%v %q", err, got)
	}
	csv := "名称,数量\n服务器,12\n交换机,4\n"
	if got, err := ExtractText("csv", []byte(csv)); err != nil || !strings.Contains(got, "交换机") {
		t.Fatalf("csv 抽取失败：%v %q", err, got)
	}
}

func TestExpandArchiveNestedZip(t *testing.T) {
	inner := buildZip(map[string]string{"inner/说明.txt": "内部包文本：嵌套展开验证"})
	outer := buildZip(map[string]string{
		"outer/直接文件.txt": "外层直传文件",
		"outer/内包.zip":   string(inner),
	})
	entries, err := ExpandArchive(outer, "outer.zip", 0, &ExtractBudget{})
	if err != nil {
		t.Fatal(err)
	}
	var sawInner bool
	for _, e := range entries {
		if e.Name == "说明.txt" {
			sawInner = true
			if e.Path != "outer/inner/说明.txt" {
				t.Fatalf("嵌套路径前缀错误：%q", e.Path)
			}
			if !strings.Contains(string(e.Data), "嵌套展开验证") {
				t.Fatalf("嵌套内容错误：%q", string(e.Data))
			}
		}
	}
	if !sawInner {
		t.Fatalf("嵌套 zip 未展开：%v", entries)
	}
}

func TestExpandArchiveTarGz(t *testing.T) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := "tar包里的文本内容"
	if err := tw.WriteHeader(&tar.Header{Name: "归档/财务制度.txt", Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := ExpandArchive(gzBuf.Bytes(), "归档.tar.gz", 0, &ExtractBudget{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "归档/财务制度.txt" || string(entries[0].Data) != content {
		t.Fatalf("tar.gz 解包错误：%+v", entries)
	}
}

func TestExtractZipRejectsZipSlipAndHidden(t *testing.T) {
	z := buildZip(map[string]string{
		"正常文件.txt":      "ok",
		"__MACOSX/junk": "junk",
		"._resource":    "junk",
		".hidden":       "junk",
		"../../越界.txt":  "bad",
	})
	// buildZip 会 Clean 掉 ../ 吗？archive/zip 原样写入名字，这里直接验证跳过逻辑
	entries, err := ExtractZip(z)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "正常文件.txt" {
		t.Fatalf("zip 过滤逻辑错误：%+v", entries)
	}
}

func TestExtractZipEncryptedRejected(t *testing.T) {
	z := buildZip(map[string]string{"机密.txt": "x"})
	// 置 central directory 各记录的加密位（flag bit0，签名 PK\x01\x02 偏移 +8）
	for i := 0; i+10 <= len(z); i++ {
		if z[i] == 'P' && z[i+1] == 'K' && z[i+2] == 1 && z[i+3] == 2 {
			binary.LittleEndian.PutUint16(z[i+8:i+10], 1)
		}
	}
	if _, err := ExtractZip(z); err == nil || !strings.Contains(err.Error(), "密码") {
		t.Fatalf("加密 zip 未明确报错：%v", err)
	}
}

func TestExpandArchiveSingleGz(t *testing.T) {
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write([]byte("单文件压缩的内容")); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := ExpandArchive(gzBuf.Bytes(), "说明.txt.gz", 0, &ExtractBudget{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "说明.txt" || string(entries[0].Data) != "单文件压缩的内容" {
		t.Fatalf("单文件 gz 解压错误：%+v", entries)
	}
}

// buildZip 内存造 zip（名字→内容）
func buildZip(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestCleanTextStripsInteriorNUL(t *testing.T) {
	// 二进制兜底走文本分支：GB18030 解码可产出内部 NUL，PG 拒收（22021）
	in := "正常文字\x00中间夹NUL\n下一行\x7f带DEL\n"
	out := CleanText(in)
	if strings.ContainsRune(out, 0) || strings.ContainsRune(out, 0x7f) {
		t.Fatalf("控制字符未剔除：%q", out)
	}
	if !strings.Contains(out, "正常文字中间夹NUL") {
		t.Fatalf("内容丢失：%q", out)
	}
}

// TestExtractPDFUniGBCMap 回归：STSong-Light + UniGB-UCS2-H 预定义 CMap 的 PDF
// （未内嵌字体、无 ToUnicode）。上游 pdf 库不认这个编码名会原样吐字节，
// 中文全文变替换符——本地 fork 补了 UCS2 双字节解码，此用例锁住该行为。
func TestExtractPDFUniGBCMap(t *testing.T) {
	data := buildPDFUniGB("4e2d65874e16754c") // 中(4E2D)文(6587)世(4E16)界(754C)
	got, err := ExtractText("pdf", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "中文世界") {
		t.Fatalf("UniGB-UCS2-H 中文未解出：%q", got)
	}
}

// buildPDFUniGB 内存造单页 PDF：正文用 hex 串写 UCS2 双字节码，字体走
// STSong-Light + UniGB-UCS2-H（Obj 索引从 1 起：Catalog/Pages/Page/Font/Content/CIDFont）
func buildPDFUniGB(hexCodes string) []byte {
	content := "BT /F1 12 Tf 72 720 Td <" + hexCodes + "> Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /STSong-Light /Encoding /UniGB-UCS2-H /DescendantFonts [6 0 R] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /CIDFontType0 /BaseFont /STSong-Light /CIDSystemInfo << /Registry (Adobe) /Ordering (GB1) /Supplement 5 >> /DW 1000 >>",
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}
