package agentruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// tty.go — 交互式 shell（websocket tty）的 Docker 侧桥接。
//
// 原理：docker exec 起一个带 TTY 的 shell 进程并 attach（Tty=true 时
// stdout/stderr 合流、无 stdcopy 帧头，原始字节流），WS 二进制帧双向透传；
// 前端 resize 消息走 ContainerExecResize 调整 pty 尺寸。
// 连接生命周期由调用方（controller）管理：读侧/写侧各单 goroutine，两端任一断开即关。

// TTYStream 一条已建立的交互式终端连接。
type TTYStream struct {
	ExecID string
	Hr     types.HijackedResponse
	cli    *client.Client
}

// Write 向容器内 shell 的 stdin 写入（WS 下行帧）。
func (t *TTYStream) Write(p []byte) (int, error) { return t.Hr.Conn.Write(p) }

// Read 从容器 pty 读输出（WS 上行帧）。Tty=true 合流原始字节，无帧头。
func (t *TTYStream) Read(p []byte) (int, error) { return t.Hr.Reader.Read(p) }

// Resize 调整 pty 尺寸（width/cols 与 height/rows）。
// 用独立短 ctx：会话 ctx 在关闭前一直持有，resize 不能挂在它下面。
func (t *TTYStream) Resize(width, height uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), resizeTimeout)
	defer cancel()
	return t.cli.ContainerExecResize(ctx, t.ExecID, container.ResizeOptions{Width: width, Height: height})
}

// Close 关闭 exec 连接（连接断开 docker 会自动回收 exec 进程，无需额外 kill）。
func (t *TTYStream) Close() error {
	t.Hr.Close()
	return nil
}

// resizeTimeout resize 等控制面小请求的兜底超时。
const resizeTimeout = 5 * time.Second

// ExecTTY 在实例容器内起交互式 shell 并 attach。width/height 为初始 pty 尺寸（0 用默认）。
// shell 选型：/bin/sh 兜底（busybox/alpine 均有）；bash 存在与否不假设。
// AttachStdin 必开——交互式终端要下行键入；Tty=true 让 docker 分配 pty（合流输出 + 行编辑/回显）。
func (c *Client) ExecTTY(ctx context.Context, containerRef string, width, height int) (*TTYStream, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, containerRef, container.ExecOptions{
		Cmd:          []string{"/bin/sh"},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true, // Tty=true 时 stderr 合进 stdout，此字段仅 API 完整性
	})
	if err != nil {
		return nil, fmt.Errorf("agentruntime: exec tty create: %w", err)
	}
	attach := container.ExecAttachOptions{}
	if width > 0 && height > 0 {
		h, w := uint(height), uint(width)
		attach.ConsoleSize = &[2]uint{h, w}
	}
	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, attach)
	if err != nil {
		return nil, fmt.Errorf("agentruntime: exec tty attach: %w", err)
	}
	return &TTYStream{ExecID: exec.ID, Hr: resp, cli: c.cli}, nil
}
