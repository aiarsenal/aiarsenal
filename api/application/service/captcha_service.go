package service

import (
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

// VerifyString 校验验证码（不区分大小写）
func (s *CaptchaService) VerifyString(id, answer string) bool {
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	return c.Verify(id, strings.ToLower(answer), true)
}
