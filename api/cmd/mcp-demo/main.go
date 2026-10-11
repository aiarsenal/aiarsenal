package main

// MCP 演示 server：内置演示上游，供管理页「连接测试 / 工具预览 / 试调用」与网关聚合演示。
// 单二进制双模式：--transport stdio（默认，网关按需拉起子进程）| http（streamable-http 远程上游）。
// 工具：echo / now / calc / longtext / slow —— 分别演示连通性、动态返回、结构化参数与失败路径、
// 长文本（网关结果压缩）、慢调用（试调用超时控制）。

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// stdio 模式下 stdout 是协议通道，日志只写 stderr
	log.SetOutput(os.Stderr)

	transport := flag.String("transport", "stdio", "transport: stdio | http")
	host := flag.String("host", "127.0.0.1", "http 模式监听地址")
	port := flag.Int("port", 43219, "http 模式监听端口")
	// --name：只暴露该演示画像（demo-weather 等 15 个，见 profiles.go）。
	// stdio 模式网关为每个 server 拉一个子进程，各自互不可见；不传=全量演示工具。
	name := flag.String("name", "", "演示画像名（demo-weather | demo-search | ...，空=全量演示）")
	flag.Parse()

	var srv *mcp.Server
	if *name != "" {
		p := findProfile(*name)
		if p == nil {
			log.Fatalf("未知演示画像：%s（可用：%s）", *name, strings.Join(profileNames(), ", "))
		}
		srv = buildProfileServer(p)
	} else {
		srv = mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-demo", Version: "1.0"}, nil)
		registerTools(srv)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch *transport {
	case "stdio":
		err := srv.Run(ctx, &mcp.StdioTransport{})
		// 管道关闭（网关回收/上游断开）属正常退出，不刷错误
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closing") {
			log.Fatalf("stdio run: %v", err)
		}
	case "http":
		runHTTP(ctx, srv, *host, *port)
	default:
		log.Fatalf("不支持的 transport：%s（stdio | http）", *transport)
	}
}

// profileNames 全部画像名（错误提示用）
func profileNames() []string {
	out := make([]string, 0, len(demoProfiles))
	for i := range demoProfiles {
		out = append(out, demoProfiles[i].name)
	}
	return out
}

// runHTTP 以 streamable-http 暴露（与网关 /mcp 同款 handler），另带 /health。
// /mcp = 全量演示工具（--name 时为指定画像）；/mcp/{画像名} = 该画像独立实例（供 http 模式注册）。
func runHTTP(ctx context.Context, srv *mcp.Server, host string, port int) {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
	mux.Handle("/mcp/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := findProfile(strings.TrimPrefix(r.URL.Path, "/mcp/"))
		if p == nil {
			http.Error(w, "unknown demo profile: "+r.URL.Path, http.StatusNotFound)
			return
		}
		// 每请求新实例：streamable-http handler 要求每次返回独立 *mcp.Server（会话态互不串）
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return buildProfileServer(p)
		}, nil).ServeHTTP(w, r)
	}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"aiarsenal-mcp-demo"}`))
	})

	s := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}()
	log.Printf("aiarsenal-mcp-demo (http) listening on %s/mcp", s.Addr)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http listen: %v", err)
	}
}

// registerTools 注册演示工具。泛型 AddTool 从 struct tag 自动推导 inputSchema，
// 管理页「工具预览」里能看到完整参数结构；handler 返回 error 会成为 tool error（IsError），
// 恰好演示失败路径落审计。
func registerTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "原样返回输入文本（可重复）"},
		func(_ context.Context, _ *mcp.CallToolRequest, in struct {
			Message string `json:"message" jsonschema:"要原样返回的文本"`
			Repeat  int    `json:"repeat,omitempty" jsonschema:"重复次数，默认 1，最大 10"`
		}) (*mcp.CallToolResult, any, error) {
			n := in.Repeat
			if n < 1 {
				n = 1
			}
			if n > 10 {
				return nil, nil, fmt.Errorf("repeat 超出上限 10：%d", n)
			}
			return textResult(strings.Repeat(in.Message, n)), nil, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "now", Description: "返回当前时间（动态结果）"},
		func(_ context.Context, _ *mcp.CallToolRequest, in struct {
			Format string `json:"format,omitempty" jsonschema:"输出格式：datetime | date | time | unix，默认 datetime"`
		}) (*mcp.CallToolResult, any, error) {
			now := time.Now()
			var out string
			switch in.Format {
			case "", "datetime":
				out = now.Format("2006-01-02 15:04:05 -0700 MST")
			case "date":
				out = now.Format("2006-01-02")
			case "time":
				out = now.Format("15:04:05")
			case "unix":
				out = fmt.Sprintf("%d", now.Unix())
			default:
				return nil, nil, fmt.Errorf("不支持的 format：%s（datetime | date | time | unix）", in.Format)
			}
			return textResult(out), nil, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "calc", Description: "简单四则运算；除零返回错误（演示失败路径）"},
		func(_ context.Context, _ *mcp.CallToolRequest, in struct {
			A  float64 `json:"a" jsonschema:"左操作数"`
			Op string  `json:"op" jsonschema:"运算符：add | sub | mul | div"`
			B  float64 `json:"b" jsonschema:"右操作数"`
		}) (*mcp.CallToolResult, any, error) {
			var v float64
			switch in.Op {
			case "add":
				v = in.A + in.B
			case "sub":
				v = in.A - in.B
			case "mul":
				v = in.A * in.B
			case "div":
				if in.B == 0 {
					return nil, nil, fmt.Errorf("除零：%v / %v", in.A, in.B)
				}
				v = in.A / in.B
			default:
				return nil, nil, fmt.Errorf("不支持的 op：%s（add | sub | mul | div）", in.Op)
			}
			return textResult(fmt.Sprintf("%g %s %g = %g", in.A, in.Op, in.B, v)), nil, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "longtext", Description: "生成长文本（演示网关工具结果压缩）"},
		func(_ context.Context, _ *mcp.CallToolRequest, in struct {
			Length int `json:"length,omitempty" jsonschema:"目标字符数，默认 2000，上限 20000"`
		}) (*mcp.CallToolResult, any, error) {
			n := in.Length
			if n < 1 {
				n = 2000
			}
			if n > 20000 {
				return nil, nil, fmt.Errorf("length 超出上限 20000：%d", n)
			}
			para := "这是 aiarsenal-mcp-demo 生成的演示段落，用于验证 MCP 网关的工具结果压缩策略。"
			var b strings.Builder
			for b.Len() < n {
				b.WriteString(para)
			}
			return textResult(b.String()[:n]), nil, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "slow", Description: "延迟后返回（演示调用超时控制）"},
		func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
			DelayMs int `json:"delay_ms,omitempty" jsonschema:"延迟毫秒数，默认 1000，上限 20000"`
		}) (*mcp.CallToolResult, any, error) {
			d := time.Duration(in.DelayMs) * time.Millisecond
			if d <= 0 {
				d = time.Second
			}
			if d > 20*time.Second {
				return nil, nil, fmt.Errorf("delay_ms 超出上限 20000：%d", in.DelayMs)
			}
			select {
			case <-time.After(d):
				return textResult(fmt.Sprintf("延迟 %s 后返回", d)), nil, nil
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		})
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
