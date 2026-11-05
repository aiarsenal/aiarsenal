package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain chdir 到 api/：Endpoint 的批量上限读 config.Get().Mcp.MaxBatchItems，
// viper AddConfigPath 相对 cwd，本目录下找不到 config/config.yaml。
func TestMain(m *testing.M) {
	if _, filename, _, ok := runtime.Caller(0); ok {
		_ = os.Chdir(filepath.Join(filepath.Dir(filename), "..", ".."))
	}
	os.Exit(m.Run())
}

// ---- countJSONRPCBatch（F-032）----
// 变异锚点：删数组首字节判定 → 单请求格误报；删 Body 回填 → 下游格红（此处以回填后可重读钉住）。

func TestCountJSONRPCBatch(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantN  int
		wantOK bool
	}{
		{name: "批量数组返回条数", body: `[{"a":1},{"b":2},{"c":3}]`, wantN: 3, wantOK: true},
		{name: "单请求对象不视为批量", body: `{"jsonrpc":"2.0","id":1}`, wantN: 0, wantOK: false},
		{name: "空体不视为批量", body: ``, wantN: 0, wantOK: false},
		{name: "坏 JSON 交由下游报错", body: `[{"a":1`, wantN: 0, wantOK: false},
		{name: "空数组是合法批量（0 条）", body: `[]`, wantN: 0, wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(tc.body))
			gotN, gotOK := countJSONRPCBatch(c)
			if gotN != tc.wantN || gotOK != tc.wantOK {
				t.Fatalf("got (%d,%v) want (%d,%v)", gotN, gotOK, tc.wantN, tc.wantOK)
			}
			// 回填钉子：读出的字节必须原样回到请求体（下游 streamable-http 还要解析）
			if tc.body != "" {
				again, _ := io.ReadAll(c.Request.Body)
				if string(again) != tc.body {
					t.Fatalf("请求体未被原样回填: %q", string(again))
				}
			}
		})
	}
}

// ---- Endpoint 超限拒绝（需要 config，走 TestMain chdir）----

func TestEndpointRejectsOversizedBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Service{}
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 25; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"jsonrpc":"2.0","id":1}`)
	}
	sb.WriteString("]")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(sb.String()))
	s.Endpoint(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("超限批量应 400，got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "maxBatchItems") {
		t.Fatalf("响应应说明上限语义: %s", w.Body.String())
	}
}

// TestEndpointBatchCapWiring 接线守卫：Endpoint 必须先过批量上限再放行（F-032）。
// 纯函数表测钉不住「调用被删」的回归，源码断言兜底。
func TestEndpointBatchCapWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "sse.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "countJSONRPCBatch(c)") {
		t.Fatal("sse.go 的批量上限接线丢失：MCP 批量放大回归（F-032）")
	}
}
