package service

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"html"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// 文档文本抽取 + 清洗 + 分块。支持：
//   - PDF（ledongthuc/pdf 纯 Go；扫描件无文本层 → 抽空，入库时标注说明）
//   - DOCX / XLSX / PPTX（OOXML = zip+XML，全文抽取）
//   - OFD（电子发票/公文，zip+XML 的 TextCode 节点）
//   - TXT·MD·CSV·LOG·JSON（BOM 剥除；非 UTF-8 自动按 GB18030/Big5 解码——国内 Windows
//     记事本 ANSI=GBK、港台 Big5、UTF-16 记事本「Unicode」都有）
//   - HTML/HTM（去标签 + 实体解码）
//   - DOC·XLS·PPT（97-2003 老二进制：纯 Go 生态无可靠抽取库，仅标题可检索，抽空时入库标注）
//
// 抽取产物只进本机 aa_kb 库——文档内容属于业务资料，绝不进代码/种子/仓库。
//
// 清洗规则（针对企业 PDF 的实测噪声：水印散字、竖排碎片、页眉页脚残字）：
//   - 去单字行 / 纯符号数字行（无有效词项）
//   - 去已知水印行（整行精确匹配）
//   - 压缩连续空行
//
// PDF 专项（kbExtractPDF 管线内）：
//   - 剥文本层里夹带的 HTML 标记（水印/转换工具把 <p>、&nbsp; 等字面量盖进了文本层）
//   - 去平铺水印/页眉页脚行：同一整行全篇重复 ≥8 次（正常正文整行极少重复这么多次）
//   - 去字体映射损坏的乱码行（公式/艺术字的私有编码字形解码成 Latin 区杂烩）

// 水印整行黑名单（trim 后精确匹配即丢弃；只放公共水印词，不放任何业务内容）
var kbWatermarkLines = map[string]struct{}{
	"商业秘密":          {},
	"（普通商密）":        {},
	"商业秘密（普通商密）":    {},
	"秘密":            {},
	"内部资料":          {},
	"内部资料请注意保密":     {},
	"- 商业秘密（普通商密）-": {},
}

var kbTagRe = regexp.MustCompile(`<[^>]+>`)

// kbWatermarkRepeatMin 平铺水印判定的整行重复次数阈值：实测企业手册的表格短词
// 最多重复 5 次（「详章节」「序号」级），每页盖十几枚的水印动辄上百次，取 8 分界。
const kbWatermarkRepeatMin = 8

// kbCommonLatin1 真实文本里常见的 Latin 区字符白名单（乱码判据里放行，不算可疑）。
var kbCommonLatin1 = map[rune]struct{}{
	'×': {}, '÷': {}, '°': {}, '±': {}, '·': {}, '•': {}, '…': {}, '‰': {},
	'–': {}, '—': {}, '‘': {}, '’': {}, '“': {}, '”': {}, '„': {}, '‹': {}, '›': {},
	'«': {}, '»': {}, '€': {}, '£': {}, '¥': {}, '§': {}, '¶': {}, '©': {}, '®': {},
	'™': {}, '²': {}, '³': {}, '¹': {}, '¼': {}, '½': {}, '¾': {}, 'µ': {},
}

// kbHTMLishToText 把文本里夹带的 HTML 标记还原成纯文本（</p>/<br>/</div> 断行 +
// 去其余标签 + 实体解码）。水印/HTML 转 PDF 的工具会把字面 <p>、&nbsp; 写进文本层，
// 不剥的话标签会整段混进全文与向量。
func kbHTMLishToText(s string) string {
	txt := strings.ReplaceAll(s, "</p>", "\n")
	txt = strings.ReplaceAll(txt, "<br>", "\n")
	txt = strings.ReplaceAll(txt, "</div>", "\n")
	txt = kbTagRe.ReplaceAllString(txt, "")
	return html.UnescapeString(txt)
}

// kbDropRepeatedLines 丢全篇重复 ≥min 次的整行（平铺水印：同一文案每页盖十几枚；
// 页眉页脚、复用表头同理）。正常正文的相同整行很少重复这么多次。
func kbDropRepeatedLines(text string, min int) string {
	lines := strings.Split(text, "\n")
	count := map[string]int{}
	for _, l := range lines {
		count[l]++
	}
	keep := make([]string, 0, len(lines))
	for _, l := range lines {
		if count[l] < min {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// kbDropGarbledLines 丢字体映射损坏的乱码行（企业 PDF 常见：公式/艺术字的私有编码
// 字形被解码成 Latin 区杂烩）。判据见 kbLineLooksGarbled。
func kbDropGarbledLines(text string) string {
	lines := strings.Split(text, "\n")
	keep := make([]string, 0, len(lines))
	for _, l := range lines {
		if !kbLineLooksGarbled(l) {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// kbLineLooksGarbled 判乱码行：无 CJK + 无 3 连 ASCII 字母（不成词）+ 含「可疑字符」。
// 可疑区 = U+0080–U+02FF（Latin-1 补充、Latin 扩展 A/B、IPA、附加符号，白名单除外）
// 与 U+FFFD——字体损坏残字的典型产物。带重音的正常西文（café/résumé/Über）有连串
// 字母成词，白名单又放行了 ×÷°±· 等常见符号，不会被误伤。
func kbLineLooksGarbled(l string) bool {
	cjk := false
	hasWord := false // 3 个连续 ASCII 字母 = 一个真词
	run := 0
	rare := 0
	for _, r := range l {
		if unicode.Is(unicode.Han, r) {
			cjk = true
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			if run++; run >= 3 {
				hasWord = true
			}
		} else {
			run = 0
		}
		if _, ok := kbCommonLatin1[r]; ok {
			continue
		}
		if (r >= 0x80 && r <= 0x2FF) || r == utf8.RuneError {
			rare++
		}
	}
	return !cjk && !hasWord && rare >= 1
}

// KbExtractText 按文件类型抽取文本。fileType 为小写扩展名（pdf/docx/xlsx/pptx/ofd/txt…）。
func KbExtractText(fileType string, data []byte) (string, error) {
	switch fileType {
	case "pdf":
		return kbExtractPDF(data)
	case "docx":
		return kbExtractDOCX(data)
	case "xlsx", "xlsm":
		return kbExtractXLSX(data)
	case "pptx":
		return kbExtractPPTX(data)
	case "ofd":
		return kbExtractOFD(data)
	case "txt", "md", "markdown", "text", "csv", "log", "json":
		return KbCleanText(kbDecodeTextBytes(data)), nil
	case "html", "htm":
		return KbCleanText(kbExtractHTML(data)), nil
	case "ppt", "doc", "xls":
		// 老二进制 Office 格式：纯 Go 生态没有可靠抽取库；保留元数据（标题）参与检索，
		// 正文留空（入库时标注「未抽取到文本」提示）。需正文时先另存为 pdf/docx 再上传。
		return "", nil
	case "png", "jpg", "jpeg", "gif", "webp", "bmp":
		// 图片：无文本层（OCR 未做），原件可预览；标题/摘要照常参与检索
		return "", nil
	default:
		// 未知类型当纯文本试一把（解码器会滤掉非法字节，二进制会被清洗规则滤成空串）
		return KbCleanText(kbDecodeTextBytes(data)), nil
	}
}

func kbExtractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf 打开失败：%w", err)
	}
	rd, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf 文本层读取失败：%w", err)
	}
	b, err := io.ReadAll(io.LimitReader(rd, 32<<20))
	if err != nil {
		return "", fmt.Errorf("pdf 读取失败：%w", err)
	}
	// 文本层噪声专项：剥夹带的 HTML 标记 → 常规清洗 → 去乱码行 → 去平铺水印/页眉页脚重复行
	return kbDropRepeatedLines(
		kbDropGarbledLines(KbCleanText(kbHTMLishToText(string(b)))),
		kbWatermarkRepeatMin), nil
}

func kbExtractDOCX(data []byte) (string, error) {
	var sb strings.Builder
	err := kbWalkZipXML(data, func(name string) bool { return name == "word/document.xml" },
		[][2]string{{"</w:p>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("docx 内无 word/document.xml")
	}
	return KbCleanText(sb.String()), nil
}

// kbExtractXLSX 表格全文：共享字符串表（</si> 断行）+ 各 sheet（</row> 断行）内联串。
func kbExtractXLSX(data []byte) (string, error) {
	var sb strings.Builder
	err := kbWalkZipXML(data, func(name string) bool {
		return name == "xl/sharedStrings.xml" ||
			(strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml"))
	}, [][2]string{{"</si>", "\n"}, {"</row>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return KbCleanText(sb.String()), nil
}

// kbExtractPPTX 幻灯全文：ppt/slides/slideN.xml 文本节点（</a:p> 断段，按页序）。
func kbExtractPPTX(data []byte) (string, error) {
	var sb strings.Builder
	err := kbWalkZipXML(data, func(name string) bool {
		return strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml")
	}, [][2]string{{"</a:p>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return KbCleanText(sb.String()), nil
}

// kbExtractOFD 电子发票/公文：zip 内全部 xml 的文本节点（</TextCode> 断行，按文件名排序）。
func kbExtractOFD(data []byte) (string, error) {
	var sb strings.Builder
	err := kbWalkZipXML(data, func(name string) bool { return strings.HasSuffix(name, ".xml") },
		[][2]string{{"</TextCode>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return KbCleanText(sb.String()), nil
}

func kbExtractHTML(data []byte) string {
	return kbHTMLishToText(kbDecodeTextBytes(data))
}

// kbWalkZipXML OOXML 家族通用抽取：解包 zip → 按名匹配（名排序保稳定页序）→
// 先按段落闭合标签断行（否则整份 XML 挤一行，分块退化为硬切）→ 去标签 + 实体解码。
func kbWalkZipXML(data []byte, match func(name string) bool, breaks [][2]string, sb *strings.Builder) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("文件解包失败：%w", err)
	}
	var names []string
	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		if match(f.Name) {
			names = append(names, f.Name)
			byName[f.Name] = f
		}
	}
	sort.Strings(names)
	for _, name := range names {
		f := byName[name]
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rc, 32<<20))
		rc.Close()
		if err != nil {
			continue
		}
		xml := string(b)
		for _, r := range breaks {
			xml = strings.ReplaceAll(xml, r[0], r[1])
		}
		txt := kbTagRe.ReplaceAllString(xml, "")
		sb.WriteString(html.UnescapeString(txt))
		sb.WriteString("\n")
	}
	return nil
}

// kbDecodeTextBytes 文本字节解码（真实业务四种来源都接）：
// UTF-8 BOM 剥除 / UTF-16（BOM 识别 LE·BE，Windows 记事本「Unicode」）/
// 非 UTF-8 依次试 GB18030（兼容 GBK/GB2312，大陆 ANSI）与 Big5（港台）——
// 取替换符更少的一版；都失败则剥非法字节兜底。
func kbDecodeTextBytes(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return kbDecodeUTF16(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return kbDecodeUTF16(data[2:], binary.BigEndian)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	best := strings.ToValidUTF8(string(data), "�")
	bestBad := strings.Count(best, "�")
	for _, dec := range []func([]byte) ([]byte, error){
		simplifiedchinese.GB18030.NewDecoder().Bytes,
		traditionalchinese.Big5.NewDecoder().Bytes,
	} {
		out, err := dec(data)
		if err != nil {
			continue
		}
		s := string(out)
		if bad := strings.Count(s, "�"); bad < bestBad {
			best, bestBad = s, bad
		}
	}
	return best
}

func kbDecodeUTF16(b []byte, order binary.ByteOrder) string {
	u16 := make([]uint16, 0, len(b)/2)
	for len(b) >= 2 {
		u16 = append(u16, order.Uint16(b[:2]))
		b = b[2:]
	}
	return string(utf16.Decode(u16))
}

// KbTidyPlainText 纯文本写入清洗：去空行 + 压行首尾空白。MCP 写入的是 agent 产出的
// 结构化文本，不套 KbCleanText 那套逐行噪声过滤——那会误伤短行列表。
func KbTidyPlainText(raw string) string {
	lines := strings.Split(raw, "\n")
	keep := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.Trim(l, " \t\r")
		if l != "" {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// KbCleanText 清洗：逐行过滤噪声后以换行拼接。
func KbCleanText(raw string) string {
	lines := strings.Split(raw, "\n")
	keep := make([]string, 0, len(lines))
	for _, l := range lines {
		// 行内剔除控制字符（NUL/DEL 等，除 Tab）：二进制兜底走文本分支时，
		// GB18030 解码可产出内部 \x00——PG text 列拒收 NUL（22021），必须整行清除
		l = strings.Map(func(r rune) rune {
			if r == 0x7f || (r < 0x20 && r != '\t') {
				return -1
			}
			return r
		}, l)
		l = strings.Trim(l, " \t　\r\x0c\x00\u00a0\u200b")
		if l == "" {
			continue
		}
		if _, bad := kbWatermarkLines[l]; bad {
			continue
		}
		if !kbLineHasWords(l) {
			continue // 单字/纯符号行：竖排碎片、页眉残字、页码
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}

// kbLineHasWords 行内至少 2 个 CJK 字，或至少 2 个非 CJK 字母（英文词干）。
// 纯数字短行（页码/年份残字）与纯符号行一律视为噪声。
func kbLineHasWords(l string) bool {
	cjk, letters := 0, 0
	for _, r := range l {
		if unicode.Is(unicode.Han, r) {
			cjk++
		} else if unicode.IsLetter(r) {
			letters++
		}
	}
	return cjk >= 2 || letters >= 2
}

// KbChunkText 分块：块首块尾参数控制大小，优先在换行处断块（保段落完整），超长行硬切。
// 返回的块不含标题块（调用方决定是否加 title/summary 块）。
func KbChunkText(text string, minRunes, maxRunes int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var chunks []string
	var cur []rune
	for _, line := range strings.Split(text, "\n") {
		lr := []rune(strings.TrimSpace(line))
		if len(lr) == 0 {
			continue
		}
		if len(lr) > maxRunes { // 超长行（PDF 长段落常见）：硬切成 maxRunes 段
			for len(lr) > 0 {
				n := maxRunes
				if len(cur)+n < minRunes {
					n = minRunes - len(cur)
				}
				take := lr
				if len(take) > n {
					take = lr[:n]
				}
				lr = lr[len(take):]
				if len(cur)+len(take) >= maxRunes {
					cur = append(cur, take...)
					chunks = append(chunks, strings.TrimSpace(string(cur)))
					cur = cur[:0]
				} else {
					cur = append(cur, take...)
				}
			}
			continue
		}
		cur = append(cur, lr...)
		if len(cur) >= minRunes {
			chunks = append(chunks, strings.TrimSpace(string(cur)))
			cur = cur[:0]
		}
	}
	if len(cur) > 0 {
		if len(cur) < minRunes/2 && len(chunks) > 0 {
			chunks[len(chunks)-1] += "\n" + string(cur)
		} else {
			chunks = append(chunks, strings.TrimSpace(string(cur)))
		}
	}
	return chunks
}
