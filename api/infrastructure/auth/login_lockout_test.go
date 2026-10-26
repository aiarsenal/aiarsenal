package auth

import (
	"testing"

	"aiarsenal/infrastructure/db"
)

// TestLockoutFailOpenOnNilRedis redis 未初始化时三个入口都必须放行且不 panic
// ——fail-open 语义的单元级验证（redis 交互逻辑归真实运行矩阵）。
func TestLockoutFailOpenOnNilRedis(t *testing.T) {
	if db.Redis() != nil {
		t.Skip("redis 已初始化，跳过 nil 场景（本用例要求未初始化环境）")
	}
	if LoginLocked("prverify-nobody") {
		t.Fatal("redis 未初始化时 LoginLocked 必须放行")
	}
	RecordLoginFailure("prverify-nobody") // 不得 panic
	ClearLoginFailures("prverify-nobody") // 不得 panic
}

// TestLockoutEmptyUsername 空用户名直接短路（不产生键、不放行语义混淆）。
func TestLockoutEmptyUsername(t *testing.T) {
	if LoginLocked("") {
		t.Fatal("空用户名不应判定为锁定")
	}
	RecordLoginFailure("")
	ClearLoginFailures("")
}
