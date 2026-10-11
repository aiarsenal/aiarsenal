package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 路由烟测：/v1beta/models/:modelAction 与 /v1 各路由并存不冲突（gin 路由树注册期即 panic，
// main.go 启动会炸，故这里用同款路由形制先行验证）；并核对 path 末段 {model}:{action} 的捕获与
// %3A 编码回解码——冒号在段中是普通字符，整段落进参数由 serveGemini 自行按最后一个 ":" 拆分。
func TestGeminiRouteModelActionParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 与 cmd/main.go 同形制的路由树（/v1 组 + /v1beta 组），验证无注册冲突
	v1 := r.Group("/v1")
	v1.POST("/chat/completions", func(c *gin.Context) {})
	v1.POST("/messages", func(c *gin.Context) {})
	v1beta := r.Group("/v1beta")
	v1beta.POST("/models/:modelAction", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("modelAction"))
	})

	for _, tc := range []struct {
		path string // 请求路径
		want string // 期望捕获的 modelAction 整段
	}{
		{"/v1beta/models/gemini-2.0-flash:generateContent", "gemini-2.0-flash:generateContent"},
		{"/v1beta/models/gemini-2.0-flash%3AstreamGenerateContent", "gemini-2.0-flash:streamGenerateContent"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("{}"))
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || w.Body.String() != tc.want {
			t.Fatalf("%s → code=%d body=%q want %q", tc.path, w.Code, w.Body.String(), tc.want)
		}
		// handler 侧拆分逻辑与 serveGemini 同一实现路径
		seg := w.Body.String()
		idx := strings.LastIndex(seg, ":")
		if idx <= 0 {
			t.Fatalf("%s 拆分失败: %q", tc.path, seg)
		}
	}
}
