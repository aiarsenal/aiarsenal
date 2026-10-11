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
// 抽取产物只进本机 aa_doc 库——文档内容属于业务资料，绝不进代码/种子/仓库。
//
// 清洗规则（针对企业 PDF 的实测噪声：水印散字、竖排碎片、页眉页脚残字）：
//   - 去单字行 / 纯符号数字行（无有效词项）
//   - 去已知水印行（整行精确匹配）
//   - 压缩连续空行

// 水印整行黑名单（trim 后精确匹配即丢弃；只放公共水印词，不放任何业务内容）
var docWatermarkLines = map[string]struct{}{
	"商业秘密":          {},
	"（普通商密）":        {},
	"商业秘密（普通商密）":    {},
	"秘密":            {},
	"内部资料":          {},
	"内部资料请注意保密":     {},
	"- 商业秘密（普通商密）-": {},
}

var docTagRe = regexp.MustCompile(`<[^>]+>`)

// ExtractText 按文件类型抽取文本。fileType 为小写扩展名（pdf/docx/xlsx/pptx/ofd/txt…）。
func ExtractText(fileType string, data []byte) (string, error) {
	switch fileType {
	case "pdf":
		return extractPDF(data)
	case "docx":
		return extractDOCX(data)
	case "xlsx", "xlsm":
		return extractXLSX(data)
	case "pptx":
		return extractPPTX(data)
	case "ofd":
		return extractOFD(data)
	case "txt", "md", "markdown", "text", "csv", "log", "json":
		return CleanText(decodeTextBytes(data)), nil
	case "html", "htm":
		return CleanText(extractHTML(data)), nil
	case "ppt", "doc", "xls":
		// 老二进制 Office 格式：纯 Go 生态没有可靠抽取库；保留元数据（标题）参与检索，
		// 正文留空（入库时标注「未抽取到文本」提示）。需正文时先另存为 pdf/docx 再上传。
		return "", nil
	case "png", "jpg", "jpeg", "gif", "webp", "bmp":
		// 图片：无文本层（OCR 未做），原件可预览；标题/摘要照常参与检索
		return "", nil
	default:
		// 未知类型当纯文本试一把（解码器会滤掉非法字节，二进制会被清洗规则滤成空串）
		return CleanText(decodeTextBytes(data)), nil
	}
}

func extractPDF(data []byte) (string, error) {
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
	return CleanText(string(b)), nil
}

func extractDOCX(data []byte) (string, error) {
	var sb strings.Builder
	err := walkZipXML(data, func(name string) bool { return name == "word/document.xml" },
		[][2]string{{"</w:p>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("docx 内无 word/document.xml")
	}
	return CleanText(sb.String()), nil
}

// extractXLSX 表格全文：共享字符串表（</si> 断行）+ 各 sheet（</row> 断行）内联串。
func extractXLSX(data []byte) (string, error) {
	var sb strings.Builder
	err := walkZipXML(data, func(name string) bool {
		return name == "xl/sharedStrings.xml" ||
			(strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml"))
	}, [][2]string{{"</si>", "\n"}, {"</row>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return CleanText(sb.String()), nil
}

// extractPPTX 幻灯全文：ppt/slides/slideN.xml 文本节点（</a:p> 断段，按页序）。
func extractPPTX(data []byte) (string, error) {
	var sb strings.Builder
	err := walkZipXML(data, func(name string) bool {
		return strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml")
	}, [][2]string{{"</a:p>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return CleanText(sb.String()), nil
}

// extractOFD 电子发票/公文：zip 内全部 xml 的文本节点（</TextCode> 断行，按文件名排序）。
func extractOFD(data []byte) (string, error) {
	var sb strings.Builder
	err := walkZipXML(data, func(name string) bool { return strings.HasSuffix(name, ".xml") },
		[][2]string{{"</TextCode>", "\n"}}, &sb)
	if err != nil {
		return "", err
	}
	return CleanText(sb.String()), nil
}

func extractHTML(data []byte) string {
	txt := strings.ReplaceAll(decodeTextBytes(data), "</p>", "\n")
	txt = strings.ReplaceAll(txt, "<br>", "\n")
	txt = strings.ReplaceAll(txt, "</div>", "\n")
	txt = docTagRe.ReplaceAllString(txt, "")
	return html.UnescapeString(txt)
}

// walkZipXML OOXML 家族通用抽取：解包 zip → 按名匹配（名排序保稳定页序）→
// 先按段落闭合标签断行（否则整份 XML 挤一行，分块退化为硬切）→ 去标签 + 实体解码。
func walkZipXML(data []byte, match func(name string) bool, breaks [][2]string, sb *strings.Builder) error {
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
		txt := docTagRe.ReplaceAllString(xml, "")
		sb.WriteString(html.UnescapeString(txt))
		sb.WriteString("\n")
	}
	return nil
}

// decodeTextBytes 文本字节解码（真实业务四种来源都接）：
// UTF-8 BOM 剥除 / UTF-16（BOM 识别 LE·BE，Windows 记事本「Unicode」）/
// 非 UTF-8 依次试 GB18030（兼容 GBK/GB2312，大陆 ANSI）与 Big5（港台）——
// 取替换符更少的一版；都失败则剥非法字节兜底。
func decodeTextBytes(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeUTF16(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeUTF16(data[2:], binary.BigEndian)
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

func decodeUTF16(b []byte, order binary.ByteOrder) string {
	u16 := make([]uint16, 0, len(b)/2)
	for len(b) >= 2 {
		u16 = append(u16, order.Uint16(b[:2]))
		b = b[2:]
	}
	return string(utf16.Decode(u16))
}

// TidyPlainText 纯文本写入清洗：去空行 + 压行首尾空白。MCP 写入的是 agent 产出的
// 结构化文本，不套 CleanText 那套逐行噪声过滤——那会误伤短行列表。
func TidyPlainText(raw string) string {
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

// CleanText 清洗：逐行过滤噪声后以换行拼接。
func CleanText(raw string) string {
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
		l = strings.Trim(l, " \t　\r\x0c\x00")
		if l == "" {
			continue
		}
		if _, bad := docWatermarkLines[l]; bad {
			continue
		}
		if !lineHasWords(l) {
			continue // 单字/纯符号行：竖排碎片、页眉残字、页码
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}

// lineHasWords 行内至少 2 个 CJK 字，或至少 2 个非 CJK 字母（英文词干）。
// 纯数字短行（页码/年份残字）与纯符号行一律视为噪声。
func lineHasWords(l string) bool {
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

// ChunkText 分块：块首块尾参数控制大小，优先在换行处断块（保段落完整），超长行硬切。
// 返回的块不含标题块（调用方决定是否加 title/summary 块）。
func ChunkText(text string, minRunes, maxRunes int) []string {
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
