package agentruntime

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
)

// backup.go — 实例工作区备份与恢复（localdocker 直连桥接）。
//
// 设计取舍：不落 K8s PVC 快照、也不起临时容器做冷备——直接对实例容器
// CopyFromContainer(WorkspaceRoot) 拿 tar 流，经 gzip 落到本机备份目录
// （<backupDir>/<backupId>.tar.gz）；恢复即反向：gunzip 后 CopyToContainer
// 根路径（tar 条目以 aa/ 为根，落回 /aa）。适用单机 localdocker 场景，
// CVM 运行时不支持（调用方先判 Provider）。一致性：运行中实例备份是
// 「热备」（文件系统瞬时不一致风险由调用方权衡，重要备份建议先停实例）。
// 范围：只覆盖主工作区卷 /aa（挂载的附加持久卷不在备份范围内，注释说明）。

// BackupWorkspace 把实例容器 /aa 打包 gzip 写到 dstPath（完整文件路径）。
// 返回落盘文件字节数。src 侧 tar 流式透传，不整包驻留内存。
func (c *Client) BackupWorkspace(ctx context.Context, containerRef, dstPath string) (int64, error) {
	rc, _, err := c.cli.CopyFromContainer(ctx, containerRef, WorkspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("agentruntime: backup copy from %s: %w", WorkspaceRoot, err)
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return 0, fmt.Errorf("agentruntime: backup mkdir: %w", err)
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("agentruntime: backup create %s: %w", dstPath, err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	if _, err := io.Copy(gz, rc); err != nil {
		return 0, fmt.Errorf("agentruntime: backup stream: %w", err)
	}
	if err := gz.Close(); err != nil {
		return 0, fmt.Errorf("agentruntime: backup gzip close: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("agentruntime: backup stat: %w", err)
	}
	return fi.Size(), nil
}

// RestoreWorkspace 从备份文件（BackupWorkspace 产物）恢复实例容器 /aa。
// 调用方须先停实例（避免恢复期间运行中 agent 写坏数据），恢复完再按原状态启停。
// tar 条目以 aa/ 为根，解到容器根路径即落回 /aa；AllowOverwriteDirWithFile
// 容许既有同名目录被逐条目覆盖。
func (c *Client) RestoreWorkspace(ctx context.Context, containerRef, srcPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("agentruntime: restore open %s: %w", srcPath, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("agentruntime: restore gunzip: %w", err)
	}
	defer gz.Close()
	if err := c.cli.CopyToContainer(ctx, containerRef, "/", gz,
		container.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return fmt.Errorf("agentruntime: restore copy to %s: %w", WorkspaceRoot, err)
	}
	return nil
}
