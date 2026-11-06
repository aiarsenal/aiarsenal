package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---- gin ClientIP 可信代理语义（F-022 的行为钉子）----
// 不经 config：直接构造引擎钉住 gin 语义与我们选择的默认方向。
// 变异锚点：默认方向反转（信任全部代理）→ 前两格红。

func newIPRouter(t *testing.T, trusted []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	return r
}

func clientIP(t *testing.T, r *gin.Engine, remoteAddr string, xff string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	r.ServeHTTP(w, req)
	return w.Body.String()
}

func TestTrustedProxyClientIPBehavior(t *testing.T) {
	cases := []struct {
		name       string
		trusted    []string
		remoteAddr string
		xff        string
		want       string
	}{
		{
			// F-022 主场景：默认不信任任何代理，伪造 XFF 不再进审计
			name: "默认信任 none：XFF 被忽略取 TCP 对端", trusted: nil,
			remoteAddr: "203.0.113.9:12345", xff: "8.8.8.8", want: "203.0.113.9",
		},
		{
			// F-022×F-007 组合：声称回环再也骗不过任何以 ClientIP 为输入的闸门
			name: "默认信任 none：伪造回环 XFF 被忽略", trusted: nil,
			remoteAddr: "203.0.113.9:12345", xff: "127.0.0.1", want: "203.0.113.9",
		},
		{
			// 官方 nginx（$proxy_add_x_forwarded_for 追加型）同机反代拓扑：
			// 攻击者自带 XFF 前缀 + nginx 追加真实 IP，右端非可信条目胜出
			name: "信任回环代理：追加型链右端真实 IP 胜出", trusted: []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:5555", xff: "1.2.3.4, 8.8.8.8", want: "8.8.8.8",
		},
		{
			name: "信任回环代理：无 XFF 落代理地址", trusted: []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:5555", xff: "", want: "127.0.0.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientIP(t, newIPRouter(t, tc.trusted), tc.remoteAddr, tc.xff); got != tc.want {
				t.Fatalf("ClientIP = %q, want %q (trusted=%v remote=%q xff=%q)", got, tc.want, tc.trusted, tc.remoteAddr, tc.xff)
			}
		})
	}
}
