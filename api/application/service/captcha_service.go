package service

import (
	"net"
	"strings"

	"github.com/mojocn/base64Captcha"
)

// CaptchaService 验证码服务（对应旧 common/logic/captcha）
// 使用 base64Captcha 内存存储，与旧实现一致
type CaptchaService struct {
	driver *base64Captcha.DriverString
	store  base64Captcha.Store
}

func NewCaptchaService() *CaptchaService {
	d := &base64Captcha.DriverString{
		Height:          80,
		Width:           240,
		NoiseCount:      50,
		ShowLineOptions: 20,
		Length:          4,
		Source:          "abcdefghjkmnpqrstuvwxyz23456789",
		Fonts:           []string{"chromohv.ttf"},
	}
	return &CaptchaService{
		driver: d,
		store:  base64Captcha.DefaultMemStore,
	}
}

// GetVerifyImgString 生成验证码，返回 (id, base64PNG)
func (s *CaptchaService) GetVerifyImgString() (id, b64 string, err error) {
	driver := s.driver.ConvertFonts()
	c := base64Captcha.NewCaptcha(driver, s.store)
	id, b64, _, err = c.Generate()
	return
}

// VerifyString 校验验证码（不区分大小写）。
// 空白答案在进入上游库比较前显式判负（F-029）：base64Captcha v1.3.8 的 Verify
// 对取不到的 key 从 store 拿到空答案，与去空白后的空答案 EqualFold 相等——
// 缺这道守卫时「key 不存在 + 答案是空格」会被判通过，等于验证码形同虚设。
// 守卫放在比对边界而非 DTO 层：服务层守卫与调用方无关（未来任何新调用方自动
// 继承 fail-closed），DTO 绑定是 per-endpoint 的表单语义，且上游库升级替换时
// 这一层不随之上浮。
func (s *CaptchaService) VerifyString(id, answer string) bool {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(answer) == "" {
		return false
	}
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	return c.Verify(id, strings.ToLower(answer), true)
}

// captchaGate 验证码是否必须校验：debug 跳过仅限回环来源（F-007）。
// APP_DEBUG 曾是远程绕过闸门的钥匙——任何能打到 API 的来源，只要部署侧误开
// debug 就免验证码。收紧后 debug + 回环同时成立才跳过（本机开发体验不变），
// 不可解析/空的 IP 一律按非回环处理（fail-closed）。
func captchaGate(debug bool, ip string) bool {
	return !(debug && isLoopbackIP(ip))
}

// isLoopbackIP 判定来源是否回环地址（ParseIP 失败返回 false）。
func isLoopbackIP(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}
