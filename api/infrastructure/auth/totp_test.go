package auth

import (
	"strings"
	"testing"
	"time"
)

// 向量来源：RFC 6238 Appendix B（SHA1，密钥为 ASCII "12345678901234567890" 的
// base32 GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ）。RFC 原表为 8 位口径；验证器 App 通用
// 默认是 6 位（十进制取尾，与 8 位向量的后 6 位一致）。code6 列即此换算结果。
// 实现写错时这些断言必然红——本地实现 TOTP 的安全网，不许删。
var totpGolden = []struct {
	unix  int64
	code6 string
}{
	{59, "287082"},
	{1111111109, "081804"},
	{1111111111, "050471"},
	{1234567890, "005924"},
	{2000000000, "279037"},
	{20000000000, "353130"},
}

const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestTOTPRFCVectors(t *testing.T) {
	for _, tc := range totpGolden {
		got, err := TOTPCode(rfcSecret, time.Unix(tc.unix, 0).UTC())
		if err != nil {
			t.Fatalf("T=%d: %v", tc.unix, err)
		}
		if got != tc.code6 {
			t.Fatalf("T=%d: got %s want %s", tc.unix, got, tc.code6)
		}
	}
}

// TestTOTPVerifyWindow ±1 窗内验过、窗外与错码拒绝；坏密钥/坏码不 panic。
func TestTOTPVerifyWindow(t *testing.T) {
	now := time.Unix(1111111109, 0).UTC() // 该步正确码 081804
	mustCode := func(at time.Time) string {
		t.Helper()
		code, err := TOTPCode(rfcSecret, at)
		if err != nil {
			t.Fatal(err)
		}
		return code
	}
	if !VerifyTOTP(rfcSecret, mustCode(now), now) {
		t.Fatal("本窗正确码应通过")
	}
	if !VerifyTOTP(rfcSecret, mustCode(now.Add(-30*time.Second)), now) {
		t.Fatal("前一窗（-30s）应通过（漂移容忍）")
	}
	if !VerifyTOTP(rfcSecret, mustCode(now.Add(30*time.Second)), now) {
		t.Fatal("后一窗（+30s）应通过（漂移容忍）")
	}
	if VerifyTOTP(rfcSecret, mustCode(now.Add(-60*time.Second)), now) {
		t.Fatal("-60s（超出 ±1 窗）应拒绝")
	}
	if VerifyTOTP(rfcSecret, "081805", now) {
		t.Fatal("错码应拒绝")
	}
	if VerifyTOTP(rfcSecret, "08180", now) {
		t.Fatal("位数不对应拒绝")
	}
	if VerifyTOTP(rfcSecret, "", now) {
		t.Fatal("空码应拒绝")
	}
	if VerifyTOTP("!!notbase32!!", "081804", now) {
		t.Fatal("坏密钥应拒绝")
	}
}

// TestTOTPProvisioningURIBind 绑定 URI 的形态（App 手动录入兼容）。
func TestTOTPProvisioningURIBind(t *testing.T) {
	uri := TOTPProvisioningURI(rfcSecret, "AI Arsenal", "hi")
	for _, want := range []string{
		"otpauth://totp/AI%20Arsenal:hi?",
		"secret=" + rfcSecret,
		"issuer=AI+Arsenal",
		"algorithm=SHA1", "digits=6", "period=30",
	} {
		if !strings.Contains(uri, want) {
			t.Fatalf("URI 缺少 %q：%s", want, uri)
		}
	}
}

// TestGenerateTOTPSecretUniqueness 密钥随机性：两次生成不同、可被 TOTPCode 消费。
func TestGenerateTOTPSecretUniqueness(t *testing.T) {
	a, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("两次生成的密钥不应相同")
	}
	if _, err := TOTPCode(a, time.Now()); err != nil {
		t.Fatalf("生成密钥应可解码使用：%v", err)
	}
}
