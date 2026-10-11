package main

// skill_install.go — install_skill 命令的下载+解压辅助（runtime 中立，结果交给各 Manager 的 SkillInstaller）。
//
// 流程（对应用户语义「把 skill 的 https 链接当成参考传过去，让 manager 自己解压安装」）：
//  1. downloadAndExtract：用 instance token GET 取包 zip → 解压到临时目录 → 定位含 SKILL.md 的根；
//  2. executor 调 OpenclawManager.InstallSkill(dir, slug) 跑 `openclaw skills install <dir> --as <slug> --global`。
//
// openclaw skills install 只收本地目录（不收 URL/ZIP），故必须先在 manager 侧解压。

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// downloadAndExtract 用 instance token 从 api 拉取 skill 整包 zip，解压到临时目录，
// 返回含 SKILL.md 的根目录 + cleanup（调用方须 defer 调用清理临时目录）。
func downloadAndExtract(apiClient *API, url string, pb *progressBuffer) (skillDir string, cleanup func(), err error) {
	writef := func(format string, args ...interface{}) {
		if pb != nil {
			pb.Write(fmt.Sprintf(format, args...))
		}
	}
	writef("下载 skill 包: %s", url)
	data, err := apiClient.GetBinary(url)
	if err != nil {
		writef("下载失败: %v", err)
		return "", nil, fmt.Errorf("下载 skill 包失败: %w", err)
	}
	writef("下载完成 %d 字节，解压中", len(data))

	tmp, err := os.MkdirTemp("", "aa-skill-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	if err := extractZip(data, tmp); err != nil {
		writef("解压失败: %v", err)
		return "", cleanup, fmt.Errorf("解压失败: %w", err)
	}
	root, err := locateSkillRoot(tmp)
	if err != nil {
		return "", cleanup, err
	}
	writef("解压完成，skill 根目录: %s", root)
	return root, cleanup, nil
}

// extractZip 把 zip 字节解压到 dest（带路径穿越防护：跳过含 .. 的条目）。
func extractZip(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("读取 zip 失败: %w", err)
	}
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		// 路径穿越防护：拒绝逃出 dest 的条目。
		if strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, dest) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := copyZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

// copyZipEntry 把单个 zip 文件条目写到 target（调用方确保父目录存在）。
func copyZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, rc)
	return err
}

// locateSkillRoot 在解压目录里定位含 SKILL.md 的根：优先目录本身，否则取最浅含 SKILL.md 的子目录。
func locateSkillRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return dir, nil
	}
	var found string
	foundDepth := -1
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if _, e := os.Stat(filepath.Join(p, "SKILL.md")); e != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		depth := strings.Count(rel, string(filepath.Separator))
		if foundDepth < 0 || depth < foundDepth {
			found = p
			foundDepth = depth
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("包内未找到 SKILL.md")
	}
	return found, nil
}
