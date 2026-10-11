package agentruntime

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

// files.go — workspace 文件管理（localdocker 直连桥接）。
//
// 列目录/下载用 CopyFromContainer（docker 以 tar 流返回），上传/覆盖用
// CopyToContainer（内存里拼单条目 tar），mkdir/rm/mv 用非交互 exec（argv
// 数组下发、不经 shell，路径无注入面）。CVM 运行时不支持，调用方先判 Provider。

// FileEntry 工作区文件条目（list 响应行）。
type FileEntry struct {
	Path    string    `json:"path"`    // 相对于被列目录的路径
	Name    string    `json:"name"`    // 文件/目录名
	Size    int64     `json:"size"`    // 字节（目录为 0）
	IsDir   bool      `json:"isDir"`   // 是否目录
	Mode    string    `json:"mode"`    // 权限串（如 -rw-r--r--）
	ModTime time.Time `json:"modTime"` // 修改时间
}

// ListFiles 列工作区目录。dir 为工作区相对路径（空=根 /aa）。
// 实现：CopyFromContainer 返回以 <basename>/ 为根的 tar，剥掉根段得到相对路径。
func (c *Client) ListFiles(ctx context.Context, containerRef, dir string) ([]FileEntry, error) {
	abs := WorkspacePath(dir)
	rc, _, err := c.cli.CopyFromContainer(ctx, containerRef, abs)
	if err != nil {
		return nil, fmt.Errorf("agentruntime: copy from %s: %w", abs, err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	var prefix string
	var out []FileEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("agentruntime: parse tar of %s: %w", abs, err)
		}
		if prefix == "" {
			// 首条目即被拷贝目录本身（名字与 src basename 一致），作根段剥离基准。
			prefix = strings.TrimSuffix(hdr.Name, "/")
			continue
		}
		rel := strings.TrimPrefix(hdr.Name, prefix+"/")
		if rel == "" {
			continue
		}
		isDir := hdr.Typeflag == tar.TypeDir
		out = append(out, FileEntry{
			Path:    rel,
			Name:    path.Base(strings.TrimSuffix(rel, "/")),
			Size:    hdr.Size,
			IsDir:   isDir,
			Mode:    hdr.FileInfo().Mode().String(),
			ModTime: hdr.ModTime,
		})
	}
	return out, nil
}

// DownloadFile 流式取工作区文件内容。返回的 ReadCloser 读到的是文件原始字节
// （tar 外壳已被剥掉），Close 时连同底层 docker 流一并释放。目录不支持下载。
func (c *Client) DownloadFile(ctx context.Context, containerRef, relPath string) (io.ReadCloser, int64, error) {
	abs := WorkspacePath(relPath)
	rc, _, err := c.cli.CopyFromContainer(ctx, containerRef, abs)
	if err != nil {
		return nil, 0, fmt.Errorf("agentruntime: copy from %s: %w", abs, err)
	}
	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	if err != nil {
		_ = rc.Close()
		return nil, 0, fmt.Errorf("agentruntime: parse tar of %s: %w", abs, err)
	}
	if hdr.Typeflag == tar.TypeDir {
		_ = rc.Close()
		return nil, 0, fmt.Errorf("agentruntime: %s 是目录，不支持下载", relPath)
	}
	return &singleFileReader{tr: tr, src: rc}, hdr.Size, nil
}

// singleFileReader 把 tar 里第一条目内容剥成原始字节流。
type singleFileReader struct {
	tr  *tar.Reader
	src io.ReadCloser
}

func (r *singleFileReader) Read(p []byte) (int, error) { return r.tr.Read(p) }

func (r *singleFileReader) Close() error { return r.src.Close() }

// UploadFile 上传/覆盖工作区文件。relPath 为工作区相对路径（含文件名），
// content 为文件内容。实现：先 exec mkdir -p 父目录，再内存拼单条目 tar
// CopyToContainer 到父目录（AllowOverwriteDirWithFile 允许覆盖既有文件）。
func (c *Client) UploadFile(ctx context.Context, containerRef, relPath string, content []byte) error {
	abs := WorkspacePath(relPath)
	if abs == WorkspaceRoot {
		return fmt.Errorf("agentruntime: 上传路径不能是工作区根")
	}
	parent := path.Dir(abs)
	// mkdir -p 父目录（幂等；argv 数组、无 shell）。
	if _, err := c.execOutput(ctx, containerRef, []string{"mkdir", "-p", parent}); err != nil {
		return err
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name:    path.Base(abs), // 目标目录下的单文件条目
		Mode:    0o644,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("agentruntime: tar header: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("agentruntime: tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("agentruntime: tar close: %w", err)
	}
	return c.cli.CopyToContainer(ctx, containerRef, parent, &buf,
		container.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
}

// Mkdir 递归建目录（exec mkdir -p）。
func (c *Client) Mkdir(ctx context.Context, containerRef, relPath string) error {
	if _, err := c.execOutput(ctx, containerRef, []string{"mkdir", "-p", WorkspacePath(relPath)}); err != nil {
		return fmt.Errorf("agentruntime: mkdir %s: %w", relPath, err)
	}
	return nil
}

// DeleteFile 删除文件/目录（exec rm -rf）。路径已由 WorkspacePath 规约在工作区内。
func (c *Client) DeleteFile(ctx context.Context, containerRef, relPath string) error {
	abs := WorkspacePath(relPath)
	if abs == WorkspaceRoot {
		return fmt.Errorf("agentruntime: 不能删除工作区根")
	}
	if _, err := c.execOutput(ctx, containerRef, []string{"rm", "-rf", abs}); err != nil {
		return fmt.Errorf("agentruntime: rm %s: %w", relPath, err)
	}
	return nil
}

// RenameFile 重命名/移动（exec mv）。
func (c *Client) RenameFile(ctx context.Context, containerRef, from, to string) error {
	absFrom, absTo := WorkspacePath(from), WorkspacePath(to)
	if absFrom == WorkspaceRoot || absTo == WorkspaceRoot {
		return fmt.Errorf("agentruntime: 不能操作工作区根")
	}
	// 目标父目录必须存在：mv 不建目录，先兜底 mkdir -p。
	if _, err := c.execOutput(ctx, containerRef, []string{"mkdir", "-p", path.Dir(absTo)}); err != nil {
		return err
	}
	if _, err := c.execOutput(ctx, containerRef, []string{"mv", absFrom, absTo}); err != nil {
		return fmt.Errorf("agentruntime: mv %s -> %s: %w", from, to, err)
	}
	return nil
}
