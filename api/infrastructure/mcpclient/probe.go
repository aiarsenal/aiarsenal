package mcpclient

// 管理台上游探测客户端：直连 MCP server（stdio/sse/streamable-http）做连接测试、
// 能力发现（tools/prompts/resources）与工具试调用。
// 与 mcp/ 网关各自实现、零代码耦合（仓内既有约定）；短生命周期连接，随 ctx 取消回收。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/domain/entity"
)

// ProbeResult 一次连接测试的发现结果。
type ProbeResult struct {
	Tools     []*mcp.Tool
	Prompts   []*mcp.Prompt
	Resources []*mcp.Resource
}

// session 带兜底回收的连接（session.Close 不足以保证 stdio 子进程退出，补进程组强杀）。
type session struct {
	cs  *mcp.ClientSession
	cmd *exec.Cmd
}

func (s *session) close() {
	if s.cs != nil {
		_ = s.cs.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		// 独立进程组整组强杀（防孙进程残留）；平台差异见 probe_proc_unix.go / probe_proc_windows.go
		killProcessTree(s.cmd.Process.Pid)
	}
}

// resolveStdioCommand 解析 stdio 命令：网关约定相对路径以其工作目录（部署布局的 mcp/）为基准，
// 而本 api 进程 cwd 是 api/，故含 / 的相对命令统一挂到 ../mcp 下再探测（如 bin/mcp-demo）。
// 纯命令名（npx/uvx/node）走 PATH 查找，不动。
func resolveStdioCommand(command string) string {
	if filepath.IsAbs(command) || !strings.Contains(command, "/") {
		return command
	}
	return filepath.Join("..", "mcp", command)
}

// dial 按配置建立上游连接（Connect 即完成 initialize 握手），返回会话与回收函数。
func dial(ctx context.Context, e *entity.McpServer) (*session, func(), error) {
	var t mcp.Transport
	var cmd *exec.Cmd
	switch e.Type {
	case "stdio":
		if e.Command == "" {
			return nil, nil, errors.New("stdio 类型必须指定 command")
		}
		cmd = exec.Command(resolveStdioCommand(e.Command), parseStrArray(e.Args)...)
		cmd.Env = mergeEnv(parseStrMap(e.Env))
		setProcGroupAttr(cmd)
		t = &mcp.CommandTransport{Command: cmd}
	case "sse":
		if e.Url == "" {
			return nil, nil, errors.New("sse 类型必须指定 url")
		}
		t = &mcp.SSEClientTransport{Endpoint: e.Url, HTTPClient: headerClient(parseStrMap(e.Headers))}
	case "streamable-http":
		if e.Url == "" {
			return nil, nil, errors.New("streamable-http 类型必须指定 url")
		}
		// 探测是短生命周期请求-响应：关 standalone SSE 长连接 + 有限超时，否则测试会挂到 ctx 超时
		t = &mcp.StreamableClientTransport{
			Endpoint:             e.Url,
			HTTPClient:           headerClientTimeout(parseStrMap(e.Headers), 60*time.Second),
			MaxRetries:           0,
			DisableStandaloneSSE: true,
		}
	default:
		return nil, nil, errors.New("不支持的 type：" + e.Type)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "aiarsenal-console-probe", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, t, nil)
	if err != nil {
		// 连接失败也要回收可能已拉起的子进程
		if cmd != nil && cmd.Process != nil {
			killProcessTree(cmd.Process.Pid)
		}
		return nil, nil, err
	}
	s := &session{cs: cs, cmd: cmd}
	return s, s.close, nil
}

// Probe 连接测试 + 能力发现。
func Probe(ctx context.Context, e *entity.McpServer) (*ProbeResult, error) {
	s, cleanup, err := dial(ctx, e)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	res := &ProbeResult{}
	tools, err := s.cs.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	res.Tools = tools.Tools
	prompts, err := s.cs.ListPrompts(ctx, nil)
	if err != nil {
		return nil, err
	}
	res.Prompts = prompts.Prompts
	resources, err := s.cs.ListResources(ctx, nil)
	if err != nil {
		return nil, err
	}
	res.Resources = resources.Resources
	return res, nil
}

// CallTool 试调用（返回原始结果，文本抽取由 service 层做）。
func CallTool(ctx context.Context, e *entity.McpServer, tool string, args map[string]any) (*mcp.CallToolResult, error) {
	s, cleanup, err := dial(ctx, e)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return s.cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}

// ===== header 注入与 env/args 解析（与 mcp/ 网关各写一份） =====

// headerClient 构造注入自定义 header 的 http.Client（无 header 返回 nil 走 SDK 默认）。
func headerClient(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return nil
	}
	return &http.Client{Transport: &headerRT{base: http.DefaultTransport, headers: headers}}
}

func headerClientTimeout(headers map[string]string, timeout time.Duration) *http.Client {
	var rt http.RoundTripper = http.DefaultTransport
	if len(headers) > 0 {
		rt = &headerRT{base: http.DefaultTransport, headers: headers}
	}
	return &http.Client{Transport: rt, Timeout: timeout}
}

// headerRT 往每个请求注入自定义 header。
type headerRT struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for k, v := range h.headers {
		clone.Header.Set(k, v)
	}
	return h.base.RoundTrip(clone)
}

func mergeEnv(env map[string]string) []string {
	out := os.Environ()
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func parseStrArray(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

func parseStrMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	out := map[string]string{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
