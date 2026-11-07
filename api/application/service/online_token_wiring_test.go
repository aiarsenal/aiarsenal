package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestInsertUserOnlineSessionIDWiring 接线守卫：登录落在线行必须存 jti 会话标识而非
// 明文令牌（F-025）。TestSessionIDOf 只钉得住函数本身——调用方手滑改回 Token: token
// 或新增第二写入点不会让它变红（审计建议的调用方级钉子，与 PR⑥ 接线守卫同型）。
func TestInsertUserOnlineSessionIDWiring(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "auth_service.go"))
	if err != nil {
		t.Fatalf("读取 auth_service.go: %v", err)
	}
	// 锚定完整接线行而非函数名子串，防 `// Token: ...` 注释化或参数换形绕过。
	if !strings.Contains(string(src), "Token:     auth.SessionIDOf(token),") {
		t.Fatal("auth_service.go 的在线行写入不再存 jti：aa_user_online.token 禁止回落明文令牌（F-025 回归）")
	}
}
