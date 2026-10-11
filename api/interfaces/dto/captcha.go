package dto

// CaptchaRes 验证码返回（字段名与旧 common.CaptchaRes 一致）
type CaptchaRes struct {
	Key string `json:"key"`
	Img string `json:"img"`
}
