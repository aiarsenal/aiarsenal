// Package agentruntime — 本机 Docker 直连桥接层。
//
// 供 Agent 模块 v2 的交互式 shell（tty）、workspace 文件管理、持久卷、备份恢复等
// 「需要直连 Docker daemon」的能力使用：不经 runtime-go Adapter（其接口面向实例
// 生命周期，未覆盖 exec tty / archive / volume 元数据），直接用 docker client 库
// 走同一 daemon（unix socket / DOCKER_HOST 环境变量，与 main.go buildRuntimeAdapter
// 的连接方式一致）。
//
// 仅 localdocker 运行时可用：调用方（application/service）须先以
// adapter.Provider() == "localdocker" 判定，CVM/禁用运行时一律不支持。
package agentruntime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// WorkspaceRoot 实例工作区根目录（localdocker provision 约定：主卷挂 /aa，
// 镜像把 agent 安装与配置烤在 /aa）。workspace 文件管理与备份恢复都锚定它。
const WorkspaceRoot = "/aa"

// Client 本机 Docker 直连客户端。懒构造（首个请求用到才连 daemon），
// 并发安全（*client.Client 本身并发安全）。
type Client struct {
	cli *client.Client
}

// New 创建直连客户端。连接参数与 runtime-go localdocker 同源：
// client.FromEnv 读 DOCKER_HOST 环境变量（macOS colima/Docker Desktop 由它指定），
// 未设则默认 unix:///var/run/docker.sock。
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("agentruntime: new docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// NewEager 创建并立即 ping daemon（启动期自检用）。平时用 New 懒连即可。
func NewEager() (*Client, error) {
	c, err := New()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.cli.Ping(ctx); err != nil {
		_ = c.cli.Close()
		return nil, fmt.Errorf("agentruntime: ping daemon: %w", err)
	}
	return c, nil
}

// Close 释放底层连接。
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

// ContainerRef 实例的容器引用：优先用库里落的 ContainerId，缺省回退
// runtime-go localdocker 命名约定 aiarsenal-<instanceId>。
func ContainerRef(instanceId, storedContainerId string) string {
	if storedContainerId != "" {
		return storedContainerId
	}
	return "aiarsenal-" + instanceId
}

// WorkspacePath 把用户相对路径规约到工作区内绝对路径。
// 空 → 根；"/etc/passwd" 这类绝对输入被压到工作区相对；".." 逃逸被 Clean 消化
// （Clean("/../x")=="/x"），不存在越出 WorkspaceRoot 的结果。
func WorkspacePath(rel string) string {
	p := path.Clean("/" + strings.TrimPrefix(rel, "/"))
	if p == "/" {
		return WorkspaceRoot
	}
	return WorkspaceRoot + p
}

// execOutput 非交互执行容器内命令，同步收集 stdout/stderr（stdcopy 解复用），
// 退出码非 0 返回错误（带输出尾巴便于排障）。mkdir/rm/mv 等文件操作走它——
// 命令以 argv 数组下发（非 sh -c 拼串），路径天然不进 shell，无注入面。
func (c *Client) execOutput(ctx context.Context, containerRef string, cmd []string) ([]byte, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, containerRef, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agentruntime: exec create %v: %w", cmd, err)
	}
	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("agentruntime: exec attach %v: %w", cmd, err)
	}
	defer resp.Close()

	var out bytes.Buffer
	// Tty=false：stdout/stderr 以 stdcopy 复用帧下发，必须解复用，否则输出里混帧头。
	if _, err := stdcopy.StdCopy(&out, &out, resp.Reader); err != nil && err != io.EOF {
		return out.Bytes(), fmt.Errorf("agentruntime: exec stream %v: %w", cmd, err)
	}
	// 等进程退出并取退出码。
	inspCh := make(chan error, 1)
	go func() {
		for i := 0; i < 100; i++ { // 最多等 10s（文件操作都是毫秒级，兜底防挂）
			insp, err := c.cli.ContainerExecInspect(ctx, exec.ID)
			if err != nil {
				inspCh <- err
				return
			}
			if !insp.Running {
				if insp.ExitCode != 0 {
					inspCh <- fmt.Errorf("命令 %v 退出码 %d: %s", cmd, insp.ExitCode, tailRunes(out.String(), 400))
					return
				}
				inspCh <- nil
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		inspCh <- fmt.Errorf("命令 %v 执行超时", cmd)
	}()
	if err := <-inspCh; err != nil {
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}

// tailRunes 取字符串尾部（错误信息截断用）。
func tailRunes(s string, n int) string {
	if len(s) <= n {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[len(s)-n:])
}

// DiskUsage 拉一次 daemon 磁盘用量（卷真实占用）。列表页汇总用，一次调用带全卷。
func (c *Client) DiskUsage(ctx context.Context) (map[string]int64, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(du.Volumes))
	for _, v := range du.Volumes {
		if v == nil {
			continue
		}
		if v.UsageData != nil && v.UsageData.Size >= 0 {
			out[v.Name] = v.UsageData.Size
		}
	}
	return out, nil
}
