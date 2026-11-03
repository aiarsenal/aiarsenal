package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"

	"aiarsenal/config"
)

// TestMain chdir 到 api/：originAllowed→config.Get() 的 viper AddConfigPath 相对
// cwd，测试目录下找不到 config/config.yaml（与 service/config 包的 chdir 模式同源）。
func TestMain(m *testing.M) {
	if _, filename, _, ok := runtime.Caller(0); ok {
		_ = os.Chdir(filepath.Join(filepath.Dir(filename), "..", ".."))
	}
	os.Exit(m.Run())
}

// ---- 纯函数白名单矩阵（变异锚点：删任一分支必有格红）----

func TestOriginAllowedFor(t *testing.T) {
	allowed := []string{"http://localhost:5666", "https://ui.example.com"}
	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"显式清单命中", "http://localhost:5666", true},
		{"显式清单第二条命中", "https://ui.example.com", true},
		{"public 推导命中（标准根域 UI）", "https://aiarsenal.cc", true},
		{"scheme 不匹配不放行（http≠https）", "http://aiarsenal.cc", false},
		{"子域≠根域（api 子域不是 UI 源）", "https://api.aiarsenal.cc", false},
		{"任意未列来源不放行", "https://evil.example.net", false},
		{"无 Origin（同源/curl）无跨域可言", "", false},
		{"端口不同的同 host 不放行", "https://aiarsenal.cc:8443", false},
		{"后缀拼接域不放行（前缀攻击）", "https://aiarsenal.cc.evil.com", false},
		{"白名单来源后缀拼接不放行", "http://localhost:5666.evil.net", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := originAllowedFor(tc.origin, allowed, "https", "aiarsenal.cc"); got != tc.want {
				t.Fatalf("origin=%q got %v want %v", tc.origin, got, tc.want)
			}
		})
	}
	// domain 为空（未配置 public）时只靠显式清单
	if originAllowedFor("https://aiarsenal.cc", allowed, "https", "") {
		t.Fatal("domain 为空时 public 推导不得放行")
	}
}

// ---- httptest 集成（真 config 接线；前提断言失败会显式报错而非静默漂移）----

func TestSetCORSHeaders(t *testing.T) {
	cfg := config.Get()
	if cfg.Cors.AllowedOrigins == nil || len(cfg.Cors.AllowedOrigins) == 0 {
		t.Fatal("前提失效：仓库 config.yaml 的 cors.allowedOrigins 应含开发前端来源")
	}
	devOrigin := cfg.Cors.AllowedOrigins[0]

	gin.SetMode(gin.TestMode)
	assertHeaders := func(t *testing.T, name, origin string, wantOrigin bool) {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/login", nil)
		if origin != "" {
			c.Request.Header.Set("Origin", origin)
		}
		SetCORSHeaders(c)
		got := w.Header().Get("Access-Control-Allow-Origin")
		if wantOrigin && got != origin {
			t.Fatalf("%s: ACAO=%q want 精确回显 %q", name, got, origin)
		}
		if !wantOrigin {
			if got != "" {
				t.Fatalf("%s: 不应发 CORS 头，实际 ACAO=%q", name, got)
			}
			for _, h := range []string{"Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age"} {
				if w.Header().Get(h) != "" {
					t.Fatalf("%s: 拒绝路径不应发 %s", name, h)
				}
			}
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("%s: 不应再发 Allow-Credentials（手册 F-11 移除项）", name)
		}
	}

	t.Run("白名单来源精确回显", func(t *testing.T) { assertHeaders(t, "dev", devOrigin, true) })
	t.Run("public 推导来源放行", func(t *testing.T) {
		assertHeaders(t, "public", cfg.PublicScheme()+"://"+cfg.PublicDomain(), true)
	})
	t.Run("任意来源不发头", func(t *testing.T) { assertHeaders(t, "evil", "https://evil.example.net", false) })
	t.Run("无 Origin 不发头", func(t *testing.T) { assertHeaders(t, "curl", "", false) })
}

// CORS 中间件预检路径：白名单内 OPTIONS 204+头，白名单外 204 无头（浏览器侧拒跨域）。
func TestCORSPreflight(t *testing.T) {
	cfg := config.Get()
	devOrigin := cfg.Cors.AllowedOrigins[0]
	gin.SetMode(gin.TestMode)

	preflight := func(origin string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodOptions, "/api/v1/system/login", nil)
		if origin != "" {
			c.Request.Header.Set("Origin", origin)
		}
		CORS()(c)
		return w
	}

	w := preflight(devOrigin)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != devOrigin {
		t.Fatalf("白名单预检应 204+ACAO，实际 %d ACAO=%q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
	w = preflight("https://evil.example.net")
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("白名单外预检应 204 且无 ACAO，实际 %d ACAO=%q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}
