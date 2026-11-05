package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoRawUpstreamErrorEcho 源码守卫（F-031）：传输层错误文本（*url.Error 内嵌完整
// 上游 URL：host/端口/路径）禁止直传客户端。所有网络层失败必须走 upstreamFailBody()
// 归一化，细节只进 recordError 审计。reintroduce 一个直传点本测试即红。
func TestNoRawUpstreamErrorEcho(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	dir := filepath.Dir(file)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "failBody: []byte(err.Error())") {
			t.Fatalf("%s 仍有传输层错误文本直传客户端（F-031 回归）：改用 upstreamFailBody()", name)
		}
	}
}

// TestUpstreamFailBodyShape 归一化体自身不含任何上游地址形态。
func TestUpstreamFailBodyShape(t *testing.T) {
	body := upstreamFailBody()
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("归一化体必须是合法 JSON: %v", err)
	}
	if strings.Contains(string(body), "http://") || strings.Contains(string(body), "://") {
		t.Fatalf("归一化体不应含 URL: %s", body)
	}
}
