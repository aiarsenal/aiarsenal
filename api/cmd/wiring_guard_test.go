package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestTrustedProxyWiring 接线守卫：gin 引擎必须以配置的可信代理清单初始化（F-022）。
// 行为表测钉不住 main.go 的装配——把 SetTrustedProxies 参数换成 nil 或删掉调用，
// 表测依旧绿；这里以源码断言兜底（与 captchaGate/SessionIDOf 接线守卫同型）。
func TestTrustedProxyWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "main.go"))
	if err != nil {
		t.Fatalf("读取 main.go: %v", err)
	}
	if !strings.Contains(string(src), "r.SetTrustedProxies(config.Get().Server.TrustedProxies)") {
		t.Fatal("main.go 未以配置初始化可信代理清单：X-Forwarded-For 将可伪造审计 IP（F-022 回归）")
	}
}

// TestMcpTrustedProxyWiring 同款守卫（审计 B1）：mcp 网关引擎的 Endpoint 把
// c.ClientIP() 落进 aa_mcp_activity.source_ip——mcp main.go 也必须以配置初始化。
func TestMcpTrustedProxyWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "mcp", "main.go"))
	if err != nil {
		t.Fatalf("读取 mcp/main.go: %v", err)
	}
	if !strings.Contains(string(src), "r.SetTrustedProxies(config.Get().Server.TrustedProxies)") {
		t.Fatal("mcp/main.go 未以配置初始化可信代理清单：工具调用审计 source_ip 可伪造（F-022 回归）")
	}
}
