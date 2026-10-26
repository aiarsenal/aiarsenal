package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// totp.go — RFC 6238 TOTP 本地实现（SHA1/6 位/30 秒，验证器 App 的通用默认组合）。
//
// 选型决策（PR 正文有完整取舍）：TOTP 是 RFC 冻结协议，官方附录自带测试向量可锁单测，
// 消费面只有「验码/出码/绑定 URI」三个函数，与 SM3 同理由本地实现保持全 PR 零第三方
// 依赖。动态码属于「持有因子」，与口令（知识因子）组合满足等保三级鉴别 d 项。
//
// 安全语义：验码用常数时间比较；±1 时间窗容忍设备漂移；错误的动态码在 Login 流程中
// 计入登录失败锁定（复用 login_lockout.go），不给独立的爆破面。

// totpPeriod RFC 6238 默认时间步长（秒）。
const totpPeriod = 30

// totpDigits 验证码位数（验证器 App 通用默认）。
const totpDigits = 6

// totpWindow 验码时允许的时间窗偏移（±1 步 = ±30 秒，容纳设备时钟漂移）。
const totpWindow = 1

// b32Secret 密钥编码：RFC 4648 base32 无 padding（otpauth URI 标准形态）。
var b32Secret = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret 生成 20 字节随机密钥的 base32 编码（RFC 4226 建议 ≥16 字节）。
// 落库长度算式（aa_user.totp_secret varchar(128) 的余量依据）：
// base32(20B)=32 字符 → AES-GCM(12 iv+32 ct+16 tag)=60B → hex 120 字符。
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return b32Secret.EncodeToString(raw), nil
}

// TOTPCode 计算给定时刻的动态码。secret 为 base32 编码。
func TOTPCode(secretBase32 string, t time.Time) (string, error) {
	key, err := b32Secret.DecodeString(strings.ToUpper(strings.TrimRight(secretBase32, "=")))
	if err != nil {
		return "", fmt.Errorf("totp 密钥解码失败: %w", err)
	}
	counter := uint64(t.Unix()) / totpPeriod
	return hotp(key, counter), nil
}

// VerifyTOTP 校验动态码，容忍 ±totpWindow 个时间窗。常数时间比较，任何异常一律 false。
func VerifyTOTP(secretBase32, code string, now time.Time) bool {
	if len(code) != totpDigits {
		return false
	}
	key, err := b32Secret.DecodeString(strings.ToUpper(strings.TrimRight(secretBase32, "=")))
	if err != nil {
		return false
	}
	counter := uint64(now.Unix()) / totpPeriod
	for i := -int64(totpWindow); i <= totpWindow; i++ {
		want := hotp(key, uint64(int64(counter)+i))
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// TOTPProvisioningURI 生成验证器 App 的绑定 URI（手动录入用：
// 密钥 + 账号名 + 发行方；SHA1/6 位/30 秒为默认参数，显式写入 URI）。
func TOTPProvisioningURI(secretBase32, issuer, accountName string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(accountName)
	q := url.Values{}
	q.Set("secret", secretBase32)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", strconv.Itoa(totpDigits))
	q.Set("period", strconv.Itoa(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// totpMod 6 位验证码的取模基数（totpDigits 恒为 6）。
const totpMod = 1_000_000

// hotp RFC 4226：HMAC-SHA1 截断（dynamic truncation）后取 totpDigits 位十进制。
func hotp(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%0*d", totpDigits, bin%totpMod)
}

// MustTOTPCode Must 风格便捷封装：密钥来自本系统生成（base32 必然合法），
// 解码失败属程序员错误，直接 panic 暴露。限测试与非请求路径使用。
func MustTOTPCode(secretBase32 string, t time.Time) string {
	code, err := TOTPCode(secretBase32, t)
	if err != nil {
		panic(err)
	}
	return code
}
