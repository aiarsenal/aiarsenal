package persistence

import (
	"testing"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// TestEntityToLoginCopiesAllIdentityFields 防回归：entityToLogin 漏复制字段曾让
// rotation 判定拿到永远为 nil 的时间戳（接线层 bug 单测测不到、真实运行才现形）。
// 新增 LoginUserRes 投影字段时必须同步进本表。
func TestEntityToLoginCopiesAllIdentityFields(t *testing.T) {
	now := types.Time(time.Now())
	u := &entity.User{
		UserId: "u-1", UserName: "hi", EmployeeNo: "EMP1", Name: "n",
		UserNickname: "nn", UserPassword: "pw", UserSalt: "salt",
		UserStatus: 1, IsAdmin: 1, Avatar: "a", DeptId: "d1",
		PasswordChangedAt: &now, TOTPSecret: "cipher", TOTPEnabled: 1,
	}
	got := entityToLogin(u)
	// 全量断言（审计 A3：只锁一半字段时，删掉 IsAdmin 复制行测试仍绿）。
	// LoginUserRes 新增投影字段时必须同步：定义/写入/复制/消费四落点的「复制」。
	if got.Id != u.UserId {
		t.Fatalf("UserId 未复制: %q", got.Id)
	}
	if got.UserName != u.UserName || got.EmployeeNo != u.EmployeeNo || got.Name != u.Name {
		t.Fatalf("基础字段漏复制: %+v", got)
	}
	if got.UserNickname != u.UserNickname || got.UserPassword != u.UserPassword || got.UserSalt != u.UserSalt {
		t.Fatalf("口令字段漏复制: %+v", got)
	}
	if got.UserStatus != u.UserStatus || got.IsAdmin != u.IsAdmin || got.Avatar != u.Avatar || got.DeptId != u.DeptId {
		t.Fatalf("身份字段漏复制（IsAdmin 进 JWT 鉴权链）: %+v", got)
	}
	if got.PasswordChangedAt == nil || got.TOTPSecret != u.TOTPSecret || got.TOTPEnabled != u.TOTPEnabled {
		t.Fatalf("口令治理字段漏复制: %+v", got)
	}
}
