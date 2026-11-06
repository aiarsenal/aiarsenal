package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"aiarsenal/config"
)

// ---- SessionIDOf 从已签发令牌解析 jti（F-025）----
// aa_user_online.token 只存该标识不存明文令牌：读库不再可重放。
// 变异锚点：把解析失败分支改成返回原令牌 → 垃圾输入格红；把返回值改成整段
// token → 防重放钉子红。

func TestSessionIDOf(t *testing.T) {
	key := []byte(config.Get().Jwt.EncryptKey)
	signed := func(k []byte, jti string) string {
		c := Claims{
			UserId: "u-1",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        jti,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(k)
		if err != nil {
			t.Fatalf("签发测试令牌失败: %v", err)
		}
		return tok
	}

	cases := []struct {
		name  string
		token string
		want  string
	}{
		{name: "正常令牌解析出 jti", token: signed(key, "sess-test-0001"), want: "sess-test-0001"},
		{name: "密钥不符拒解析（fail-closed）", token: signed([]byte("wrong-key-000"), "sess-test-0002"), want: ""},
		{name: "垃圾串拒解析", token: "garbage", want: ""},
		{name: "空串拒解析", token: "", want: ""},
		{name: "形似 JWT 的三段垃圾拒解析", token: "eyJhbGciOiJIUzI1NiJ9.notajwt.sig", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SessionIDOf(tc.token); got != tc.want {
				t.Fatalf("SessionIDOf(%q...) = %q, want %q", tc.token[:min(len(tc.token), 24)], got, tc.want)
			}
		})
	}

	// 防重放钉子：解析结果绝不能等于原令牌本体（调用方拿它落库，等号成立即等于存明文）。
	tok := signed(key, "sess-test-0003")
	if got := SessionIDOf(tok); got == "" || got == tok {
		t.Fatalf("SessionIDOf 必须返回与原令牌不同的 jti，got %q", got)
	}
}
