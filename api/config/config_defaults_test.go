package config

import "testing"

// TestApplyPasswordDefaults 归一逻辑：空/零值取默认，非法值回落并改写为生效值。
func TestApplyPasswordDefaults(t *testing.T) {
	cases := []struct {
		name string
		in   PasswordConfig
		want PasswordConfig
	}{
		{
			name: "全空取默认",
			in:   PasswordConfig{},
			want: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sm3", Iterations: 60000},
				Policy:  PasswordPolicyConfig{Level: "standard"},
				Lockout: PasswordLockoutConfig{MaxFailures: 5, LockMinutes: 30},
			},
		},
		{
			name: "sha512 档零迭代取算法默认",
			in:   PasswordConfig{Hash: PasswordHashConfig{Algorithm: "pbkdf2-sha512"}},
			want: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sha512", Iterations: 220000},
				Policy:  PasswordPolicyConfig{Level: "standard"},
				Lockout: PasswordLockoutConfig{MaxFailures: 5, LockMinutes: 30},
			},
		},
		{
			name: "非法算法与档位回落默认",
			in: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "md5", Iterations: 12345},
				Policy:  PasswordPolicyConfig{Level: "paranoid"},
				Lockout: PasswordLockoutConfig{MaxFailures: 3, LockMinutes: 7},
			},
			want: PasswordConfig{
				// 迭代数显式给出则保留（回落的是算法，不是操作者的调参）
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sm3", Iterations: 12345},
				Policy:  PasswordPolicyConfig{Level: "standard"},
				Lockout: PasswordLockoutConfig{MaxFailures: 3, LockMinutes: 7},
			},
		},
		{
			name: "rotationDays 负值归一为 0（不启用）",
			in: PasswordConfig{
				Policy: PasswordPolicyConfig{Level: "standard", RotationDays: -90},
			},
			want: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sm3", Iterations: 60000},
				Policy:  PasswordPolicyConfig{Level: "standard", RotationDays: 0},
				Lockout: PasswordLockoutConfig{MaxFailures: 5, LockMinutes: 30},
			},
		},
		{
			name: "显式配置原样保留",
			in: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sha512", Iterations: 300000},
				Policy:  PasswordPolicyConfig{Level: "strict"},
				Lockout: PasswordLockoutConfig{MaxFailures: 8, LockMinutes: 60},
			},
			want: PasswordConfig{
				Hash:    PasswordHashConfig{Algorithm: "pbkdf2-sha512", Iterations: 300000},
				Policy:  PasswordPolicyConfig{Level: "strict"},
				Lockout: PasswordLockoutConfig{MaxFailures: 8, LockMinutes: 60},
			},
		},
	}
	for _, tc := range cases {
		c := Config{Password: tc.in}
		c.applyPasswordDefaults()
		got := c.Password
		if got.Hash != tc.want.Hash || got.Policy != tc.want.Policy || got.Lockout != tc.want.Lockout {
			t.Fatalf("%s：归一结果错误\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}
}
