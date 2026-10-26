package service

import (
	"context"
	"testing"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/auth"
)

// newTestTOTPService 复用 user_service_test 的 fakeUserRepo（同包）。
func newTestTOTPService(repo *fakeUserRepo) *TOTPService {
	return NewTOTPService(repo)
}

func ordinaryUserForTOTP(id string) *entity.User {
	return &entity.User{UserId: id, UserName: id, IsAdmin: 0}
}

func superUserForTOTP(id string) *entity.User {
	return &entity.User{UserId: id, UserName: id, IsAdmin: 1}
}

// TestTOTPSetupActivateFlow 全流程：setup 落待激活密钥 → 错码拒 → 正确码激活 →
// 已激活拒绝重复 setup。
func TestTOTPSetupActivateFlow(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUserForTOTP("u-1")}}
	svc := newTestTOTPService(repo)

	secret, uri, err := svc.Setup(context.Background(), "u-1")
	if err != nil {
		t.Fatal(err)
	}
	if uri == "" || len(secret) < 16 {
		t.Fatalf("setup 返回异常: secret=%q uri=%q", secret, uri)
	}
	if !contains(uri, "otpauth://totp/") || !contains(uri, "issuer=") {
		t.Fatalf("otpauth URI 形态错误: %q", uri)
	}
	enabled := repo.curUsers["u-1"].TOTPEnabled
	if enabled != 0 {
		t.Fatal("setup 后应保持待激活（enabled=0）")
	}

	// 错码不激活
	if err := svc.Activate(context.Background(), "u-1", "000000"); err == nil {
		t.Fatal("错误动态码不应激活")
	}
	if repo.curUsers["u-1"].TOTPEnabled == 1 {
		t.Fatal("错码后不应置位")
	}
	// 正确码激活（当场计算）
	code, err := auth.TOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Activate(context.Background(), "u-1", code); err != nil {
		t.Fatalf("正确动态码应激活: %v", err)
	}
	if repo.curUsers["u-1"].TOTPEnabled != 1 {
		t.Fatal("激活后未置位")
	}
	// 已激活再 setup 拒绝
	if _, _, err := svc.Setup(context.Background(), "u-1"); err == nil {
		t.Fatal("已绑定账号不应允许重复 setup")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// TestTOTPDisableRequiresCode 已激活账号的停用必须验一次有效动态码
// （防会话劫持关闭双因子）；待激活（enabled=0）状态下停用直接拒绝——
// 放弃绑定走重新 Setup 换新密钥即可。
func TestTOTPDisableRequiresCode(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUserForTOTP("u-1")}}
	svc := newTestTOTPService(repo)
	secret, _, err := svc.Setup(context.Background(), "u-1")
	if err != nil {
		t.Fatal(err)
	}
	// 待激活状态：停用拒绝
	if err := svc.Disable(context.Background(), "u-1", "000000"); err == nil {
		t.Fatal("待激活状态不应允许停用")
	}
	// 激活后再停用
	code, _ := auth.TOTPCode(secret, time.Now())
	if err := svc.Activate(context.Background(), "u-1", code); err != nil {
		t.Fatalf("激活失败: %v", err)
	}
	// 已激活 + 错码：拒绝且绑定仍在（审计 K2 变异锚：删 VerifyTOTP 判断时必红）
	if err := svc.Disable(context.Background(), "u-1", "000000"); err == nil {
		t.Fatal("已激活账号错码不应停用")
	}
	if repo.curUsers["u-1"].TOTPEnabled != 1 {
		t.Fatal("错码被拒后绑定不应变化")
	}
	if err := svc.Disable(context.Background(), "u-1", auth.MustTOTPCode(secret, time.Now())); err != nil {
		t.Fatalf("正确码应停用: %v", err)
	}
	if repo.curUsers["u-1"].TOTPSecret != "" || repo.curUsers["u-1"].TOTPEnabled != 0 {
		t.Fatal("停用后密钥与标志位未清")
	}
	if err := svc.Disable(context.Background(), "u-1", code); err == nil {
		t.Fatal("未绑定状态下再停用应拒绝")
	}
}

// TestTOTPActivateWithoutSetup 未 setup 就激活应拒绝。
func TestTOTPActivateWithoutSetup(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUserForTOTP("u-1")}}
	svc := newTestTOTPService(repo)
	if err := svc.Activate(context.Background(), "u-1", "123456"); err == nil {
		t.Fatal("未获取密钥不应激活")
	}
}

// TestTOTPAdminResetGate 管理员重置双因子：仅超管；非超管用例**目标必须存在**，
// 否则拒绝可能来自目标不存在而非闸门（审计 B2：删闸门用例仍绿的假证据）。
// 超管重置语义 = 下发新密钥并置已激活（enforced 死局的系统内恢复通道）。
func TestTOTPAdminResetGate(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-9": superUserForTOTP("u-9"),
		"u-1": ordinaryUserForTOTP("u-1"),
	}}
	repo.curUsers["u-1"].TOTPSecret = "old-cipher"
	repo.curUsers["u-1"].TOTPEnabled = 1
	svc := newTestTOTPService(repo)

	// 非超管 + 目标存在：拒绝只能来自闸门——删掉闸门本用例必红
	if _, _, err := svc.AdminReset(context.Background(), "u-1", "u-1"); err == nil {
		t.Fatal("非超管不应重置双因子")
	}
	if repo.curUsers["u-1"].TOTPSecret != "old-cipher" || repo.curUsers["u-1"].TOTPEnabled != 1 {
		t.Fatal("被拒后目标绑定不应变化")
	}
	if _, _, err := svc.AdminReset(context.Background(), "ghost", "u-1"); err == nil {
		t.Fatal("操作者记录缺失应拒绝")
	}
	if _, _, err := svc.AdminReset(context.Background(), "u-9", "ghost"); err == nil {
		t.Fatal("目标不存在应拒绝")
	}
	secret, uri, err := svc.AdminReset(context.Background(), "u-9", "u-1")
	if err != nil {
		t.Fatalf("超管重置应通过: %v", err)
	}
	if len(secret) < 16 || uri == "" {
		t.Fatal("重置应返回新密钥与 otpauth URI（下发通道）")
	}
	if repo.curUsers["u-1"].TOTPEnabled != 1 {
		t.Fatal("重置后应置已激活（否则 enforced 下仍进不来）")
	}
	if repo.curUsers["u-1"].TOTPSecret == "old-cipher" {
		t.Fatal("重置后应为新密文")
	}
}
