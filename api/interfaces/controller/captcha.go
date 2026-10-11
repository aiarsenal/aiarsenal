package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type CaptchaController struct {
	svc *service.CaptchaService
}

func NewCaptchaController(s *service.CaptchaService) *CaptchaController {
	return &CaptchaController{svc: s}
}

// Get 获取验证码  GET /api/v1/common/captcha/get
func (h *CaptchaController) Get(c *gin.Context) {
	id, b64, err := h.svc.GetVerifyImgString()
	if err != nil {
		middleware.Fail(c, "获取验证码失败")
		return
	}
	middleware.Success(c, dto.CaptchaRes{Key: id, Img: b64})
}
