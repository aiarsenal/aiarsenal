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

// ---- F-032 补口：单请求体字节上限（条数闸只数条数，1 条+巨大 arguments 不受限）----
// 变异锚点：删字节闸 → 巨 body 格红；阈值判断反转 → 正常格红。

func TestEndpointRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Service{}
	huge := `{"jsonrpc":"2.0","id":1,"params":{"blob":"` + strings.Repeat("x", 2*1024*1024+1) // 与 config 默认 2MB 对齐（gateway 包不引 config 常量） + `"}}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(huge))
	s.Endpoint(c)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "maxRequestBytes") {
		b := w.Body.String()
		if len(b) > 120 {
			b = b[:120]
		}
		t.Fatalf("超限字节应 400 且点名 maxRequestBytes, got %d body=%s", w.Code, b)
	}
}

// TestRequestBodyRoundtrip readRequestBody 的读-回填约定（与 countJSONRPCBatch 同款）。
func TestRequestBodyRoundtrip(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1}`))
	raw, err := readRequestBody(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"jsonrpc":"2.0","id":1}` {
		t.Fatalf("读出内容不符: %q", string(raw))
	}
	again, _ := io.ReadAll(c.Request.Body)
	if string(again) != `{"jsonrpc":"2.0","id":1}` {
		t.Fatalf("请求体未被原样回填: %q", string(again))
	}
}

// TestByteGateWiring 接线守卫：Endpoint 必须先读体并做字节上限判定（F-032 补口）。
// 纯函数表测杀不掉「接线被删」，以源码断言兜底（与 countJSONRPCBatch 守卫同款）。
func TestByteGateWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "sse.go"))
	if err != nil {
		t.Fatalf("读取 sse.go: %v", err)
	}
	if !strings.Contains(string(src), `raw, err := readRequestBody(c)`) ||
		!strings.Contains(string(src), `int64(len(raw)) > int64(config.Get().Mcp.MaxRequestBytes)`) {
		t.Fatal("Endpoint 的字节上限接线丢失——「1 条子请求+巨大 arguments」的放大面复活（F-032 补口回归）")
	}
	// 有界读：MaxBytesReader 在读取层封顶（乱序/超大 chunked body 不得先全量缓冲）。
	if !strings.Contains(string(src), `http.MaxBytesReader(c.Writer, c.Request.Body, int64(config.Get().Mcp.MaxRequestBytes))`) {
		t.Fatal("有界读缺失——超限 body 会先被 io.ReadAll 全量缓冲再判（内存放大面复活）")
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
