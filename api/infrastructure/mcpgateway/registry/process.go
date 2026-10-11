package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// cmdWrap 包装 stdio 子进程，用于进程组管理。
// setProcessGroup / killProcessTree 的平台差异见 process_unix.go / process_windows.go。
type cmdWrap struct {
	cmd *exec.Cmd
}

func (c *cmdWrap) kill() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return killProcessTree(c.cmd.Process.Pid)
}

// ===== transport 构造 =====

func newStdioTransport(command string, args []string, env map[string]string) (mcp.Transport, *cmdWrap, error) {
	if command == "" {
		return nil, nil, errors.New("command 为空")
	}
	cmd := exec.Command(command, args...)
	cmd.Env = mergeEnv(env)
	setProcessGroup(cmd)
	// CommandTransport 自行 Start 该 cmd；这里只持有引用用于停止时强杀进程组
	t := &mcp.CommandTransport{Command: cmd}
	return t, &cmdWrap{cmd: cmd}, nil
}

func newSSEClientTransport(url string, headers map[string]string) (mcp.Transport, error) {
	if url == "" {
		return nil, errors.New("url 为空")
	}
	return &mcp.SSEClientTransport{Endpoint: url, HTTPClient: headerClient(headers)}, nil
}

func newStreamableHTTPClientTransport(url string, headers map[string]string) (mcp.Transport, error) {
	if url == "" {
		return nil, errors.New("url 为空")
	}
	return &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: headerClient(headers)}, nil
}

// headerClient 构造注入自定义 header 的 http.Client（上游鉴权等）。
// 无条件包一层 headerRT：出站请求带 actor ctx 时注入令牌交换头（见 WithActor），
// 没有则与默认 Client 行为一致——stdio server 走 env，不经过这里。
func headerClient(headers map[string]string) *http.Client {
	return &http.Client{Transport: &headerRT{base: http.DefaultTransport, headers: headers}}
}

// ===== 令牌交换（网关→第二跳身份透传） =====

type ctxActorKey struct{}

// WithActor 把网关签名的调用方身份放进 ctx（sse 入口在 bearer 认证后调用）。
// payload 为 JSON（keyId/owner/kind/exp），sig 为 HMAC-SHA256(hex)。
func WithActor(ctx context.Context, payload, sig string) context.Context {
	return context.WithValue(ctx, ctxActorKey{}, [2]string{payload, sig})
}

// ActorFrom 取 ctx 中的调用方身份（无 = 未认证/系统路径，返回空串）
func ActorFrom(ctx context.Context) (payload, sig string) {
	v, _ := ctx.Value(ctxActorKey{}).([2]string)
	return v[0], v[1]
}

// headerRT 往每个请求注入自定义 header（静态配置头 + 令牌交换头）。
type headerRT struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for k, v := range h.headers {
		clone.Header.Set(k, v)
	}
	// ctx 带 actor（bearer 认证过的用户调用）才注入——内部系统调用与无身份路径不出网
	if p, sig := ActorFrom(req.Context()); p != "" && sig != "" {
		clone.Header.Set("X-AA-Actor", p)
		clone.Header.Set("X-AA-Signature", sig)
	}
	return h.base.RoundTrip(clone)
}

// mergeEnv 在继承网关进程环境的基础上追加 server 配置的 env（同名覆盖）。
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
