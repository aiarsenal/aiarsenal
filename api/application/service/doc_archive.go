package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/mholt/archives"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// 压缩包解包（zip 专用路径 + rar/7z/tar/gz 通用路径 + 嵌套递归）。
// 护栏（防 zip 炸弹 / 恶意包）：
//   - 单文件 ≤64MB、单包总量 ≤256MB、条数 ≤500（嵌套时跨层共享预算）
//   - 拒绝绝对路径与 ../ 越界（zip-slip）
//   - 跳过 __MACOSX / ._ / 点开头隐藏项
//   - 加密 zip 明确报错（密码包无法解）

const (
	zipMaxEntries = 2000      // 最多解出文件数（真实业务包可达千级）
	zipMaxTotal   = 512 << 20 // 解包总量上限 512MB（请求侧压缩包本身 ≤64MB）
	zipMaxOne     = 64 << 20  // 单文件上限 64MB（与单文件上传一致）
	archiveDepth  = 2         // 嵌套压缩包最大展开深度（0=顶层包，2=包中包中包不再展开）
)

// ZipEntry 解包后的一条文件（Name=文件名，Path=包内相对路径）
type ZipEntry struct {
	Name string
	Path string
	Data []byte
}

// ExtractBudget 跨层共享的解包预算（嵌套压缩包合计不超过总护栏）
type ExtractBudget struct {
	entries int
	bytes   int64
}

// IsArchiveName 按文件名判断是否为受支持的压缩包（zip/rar/7z/tar/tgz/txz/tbz2/gz/bz2/xz）
func IsArchiveName(name string) bool {
	n := strings.ToLower(name)
	for _, ext := range []string{
		".zip", ".rar", ".7z", ".tar", ".tgz", ".txz", ".tbz2", ".tar.gz", ".tar.xz", ".tar.bz2", ".gz", ".bz2", ".xz",
	} {
		if strings.HasSuffix(n, ext) {
			return true
		}
	}
	return false
}

// ExtractArchive 按文件名分发：zip 走专用路径（GBK 名 + 加密检测），
// 其余走内容嗅探解包器。条目名统一净化解码。
func ExtractArchive(data []byte, fileName string) ([]ZipEntry, error) {
	if strings.HasSuffix(strings.ToLower(fileName), ".zip") {
		return ExtractZip(data)
	}
	return extractBySniff(data, fileName)
}

// ExpandArchive 解包 + 嵌套展开：包内还是压缩包则继续解（深度 ≤ archiveDepth，
// 更深层的包按普通文件保留入库——抽取层自有兜底）；内层条目路径拼上外层所在目录前缀。
// 预算跨层共享：一个上传最终解出的所有文件合计不超总护栏。
func ExpandArchive(data []byte, fileName string, depth int, b *ExtractBudget) ([]ZipEntry, error) {
	entries, err := ExtractArchive(data, fileName)
	if err != nil {
		return nil, err
	}
	var out []ZipEntry
	for _, e := range entries {
		b.entries++
		b.bytes += int64(len(e.Data))
		if b.entries > zipMaxEntries {
			return nil, errors.New("解包文件总数超过 2000（含嵌套压缩包）")
		}
		if b.bytes > zipMaxTotal {
			return nil, errors.New("解包总量超过 512MB（含嵌套压缩包）")
		}
		if !IsArchiveName(e.Name) {
			out = append(out, e)
			continue
		}
		if depth >= archiveDepth {
			out = append(out, e) // 深层包不再展开，按普通文件保留
			continue
		}
		inner, err := ExpandArchive(e.Data, e.Name, depth+1, b)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", e.Name, err)
		}
		prefix := ""
		if dir := path.Dir(e.Path); dir != "." && dir != "/" {
			prefix = dir + "/"
		}
		for _, ie := range inner {
			ie.Path = prefix + ie.Path
			out = append(out, ie)
		}
	}
	return out, nil
}

// ExtractZip 内存解包（zip 专用）：目录项/隐藏文件/__MACOSX 跳过；越界路径整条丢弃；
// 文件名兼容：Windows 中文 zip 名为 GBK 且未标 UTF-8 flag，Go 原样给字节串——
// 按 GB18030（兼容 GBK/GB2312）解回；本身合法 UTF-8 则原样；再兜底净化非法字符。
func ExtractZip(data []byte) ([]ZipEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("压缩包读取失败：%w", err)
	}
	var out []ZipEntry
	var total int64
	for _, f := range zr.File {
		if f.Flags&0x1 != 0 {
			return nil, errors.New("压缩包带密码，无法解包；请去除密码后重新上传")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.ReplaceAll(decodeZipName(f.Name, f.NonUTF8), "\\", "/"))
		if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			continue // 绝对路径/越界路径（zip-slip），整条丢弃
		}
		if skipZipEntry(name) {
			continue
		}
		if len(out) >= zipMaxEntries {
			return nil, errors.New("压缩包内文件过多（上限 2000 个）")
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		buf, err := io.ReadAll(io.LimitReader(rc, zipMaxOne+1))
		rc.Close()
		if err != nil {
			continue
		}
		if int64(len(buf)) > zipMaxOne {
			return nil, fmt.Errorf("%s：单文件超过 64MB", path.Base(name))
		}
		total += int64(len(buf))
		if total > zipMaxTotal {
			return nil, errors.New("压缩包解包总量超过 512MB")
		}
		out = append(out, ZipEntry{Name: path.Base(name), Path: name, Data: buf})
	}
	return out, nil
}

// extractBySniff rar/7z/tar/gz 家族：内容嗅探定格式后逐条回调。
// 条目名（rar4 常见 GBK）与路径护栏同 zip；单条打不开跳过（包体损坏局部不拖垮整包）。
func extractBySniff(data []byte, fileName string) ([]ZipEntry, error) {
	format, _, err := archives.Identify(context.Background(), fileName, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("压缩包识别失败（格式不支持或文件损坏）：%w", err)
	}
	ex, ok := format.(archives.Extractor)
	if !ok {
		// 纯压缩单文件（如 notes.txt.gz，非 tar.gz 归档）：解压后按单文件返回；
		// 解出的若还是压缩包（包.zip.gz），上层 ExpandArchive 会继续展开。
		if decomp, ok := format.(archives.Decompressor); ok {
			return decompressOne(format.Extension(), decomp, data, fileName)
		}
		return nil, fmt.Errorf("不支持的压缩格式：%s", fileName)
	}
	var out []ZipEntry
	var total int64
	err = ex.Extract(context.Background(), bytes.NewReader(data), func(ctx context.Context, f archives.FileInfo) error {
		if f.IsDir() {
			return nil
		}
		// f.Name() 是 basename，包内完整路径在 NameInArchive（官方注释提醒不可信，须 Clean）
		name := path.Clean(strings.ReplaceAll(decodeEntryName(f.NameInArchive), "\\", "/"))
		if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			return nil
		}
		if skipZipEntry(name) {
			return nil
		}
		if len(out) >= zipMaxEntries {
			return errors.New("压缩包内文件过多（上限 2000 个）")
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		buf, err := io.ReadAll(io.LimitReader(rc, zipMaxOne+1))
		rc.Close()
		if err != nil {
			return nil
		}
		if int64(len(buf)) > zipMaxOne {
			return fmt.Errorf("%s：单文件超过 64MB", path.Base(name))
		}
		total += int64(len(buf))
		if total > zipMaxTotal {
			return errors.New("压缩包解包总量超过 512MB")
		}
		out = append(out, ZipEntry{Name: path.Base(name), Path: name, Data: buf})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// decompressOne 纯压缩流（gzip/xz/bz2 单文件）解压成一条 ZipEntry
func decompressOne(ext string, decomp archives.Decompressor, data []byte, fileName string) ([]ZipEntry, error) {
	rc, err := decomp.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解压失败：%w", err)
	}
	defer rc.Close()
	buf, err := io.ReadAll(io.LimitReader(rc, zipMaxOne+1))
	if err != nil {
		return nil, fmt.Errorf("解压失败：%w", err)
	}
	if int64(len(buf)) > zipMaxOne {
		return nil, errors.New("解压后单文件超过 64MB")
	}
	name := strings.TrimSuffix(strings.ToLower(fileName), strings.ToLower(ext))
	if name == "" {
		name = fileName
	}
	return []ZipEntry{{Name: path.Base(name), Path: path.Base(name), Data: buf}}, nil
}

// decodeZipName zip 条目名（NonUTF8 flag 可用时优先按它走）
func decodeZipName(name string, nonUTF8 bool) string {
	if !nonUTF8 || utf8.ValidString(name) {
		return name
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes([]byte(name)); err == nil {
		return string(out)
	}
	return strings.ToValidUTF8(name, "�")
}

// decodeEntryName 非 zip 家族条目名（rar4 老包常见 GBK，无标志位可依）：
// 字节本身合法 UTF-8 原样，否则按 GB18030 解，再兜底剥非法字节。
func decodeEntryName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes([]byte(name)); err == nil {
		return string(out)
	}
	return strings.ToValidUTF8(name, "�")
}

// skipZipEntry 跳过 macOS 元数据与点开头的隐藏文件/目录
func skipZipEntry(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if seg == "__MACOSX" || strings.HasPrefix(seg, "._") || (strings.HasPrefix(seg, ".") && seg != ".") {
			return true
		}
	}
	return false
}
