package service

import (
	"testing"
	"time"

	"aiarsenal/domain/types"
)

// TestPasswordExpired 口令定期更换判定的纯函数表：档位关闭、无记录保守语义、
// 窗口边界（恰好到期不算过期，超过一天才算）。
func TestPasswordExpired(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local)
	day := func(n int) *types.Time {
		tm := now.Add(time.Duration(-n) * 24 * time.Hour)
		tt := types.Time(tm)
		return &tt
	}

	cases := []struct {
		name      string
		changedAt *types.Time
		days      int
		want      bool
	}{
		{"档位关闭一律不判过期", day(999), 0, false},
		{"档位关闭即使无记录", nil, 0, false},
		{"无记录视为已过期（保守）", nil, 90, true},
		{"零值时间视为已过期", &types.Time{}, 90, true},
		{"窗口内不判过期", day(89), 90, false},
		{"恰好到期不算过期（边界）", day(90), 90, false},
		{"超窗一天判过期", day(91), 90, true},
	}
	for _, tc := range cases {
		if got := passwordExpired(tc.changedAt, tc.days, now); got != tc.want {
			t.Fatalf("%s：got %v want %v", tc.name, got, tc.want)
		}
	}
}

// TestTOTPLoginCheck 登录双因子判定的状态矩阵全格（K1：闸门逻辑收成纯函数后
// 双向可测——删除任一分支本表必红）。verify 注入把解密/验码的副作用挡在外面。
func TestTOTPLoginCheck(t *testing.T) {
	okVerify := func(code string) bool { return code == "123456" }
	badVerify := func(string) bool { return false }
	cases := []struct {
		name     string
		enabled  uint
		enforced bool
		code     string
		verify   func(string) bool
		wantErr  string // 空=放行
	}{
		{"未绑定+未强制：放行", 0, false, "", okVerify, ""},
		{"未绑定+强制：拒", 0, true, "", okVerify, "账号未绑定双因子验证，请联系管理员绑定"},
		{"已绑定+缺码：拒", 1, false, "", okVerify, "请输入动态验证码"},
		{"已绑定+正确码：放行", 1, false, "123456", okVerify, ""},
		{"已绑定+强制+正确码：放行", 1, true, "123456", okVerify, ""},
		{"已绑定+错码：拒（调用方计锁定）", 1, false, "654321", okVerify, "动态验证码错误"},
		{"verify 注入失败等同错码", 1, false, "123456", badVerify, "动态验证码错误"},
		{"未绑定+强制+带了码也拒", 0, true, "123456", okVerify, "账号未绑定双因子验证，请联系管理员绑定"},
	}
	for _, tc := range cases {
		err := totpLoginCheck(tc.enabled, tc.enforced, tc.code, tc.verify)
		if tc.wantErr == "" && err != nil {
			t.Fatalf("%s：不应拒绝（%v）", tc.name, err)
		}
		if tc.wantErr != "" {
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("%s：got %v want %q", tc.name, err, tc.wantErr)
			}
		}
	}
}
