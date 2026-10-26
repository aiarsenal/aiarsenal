package controller

import (
	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// TOTPController 双因子（TOTP）绑定/激活/停用（本人）与管理员重置。
type TOTPController struct {
	svc *service.TOTPService
}

// NewTOTPController 构造。
func NewTOTPController(svc *service.TOTPService) *TOTPController {
	return &TOTPController{svc: svc}
}

// Setup 生成绑定密钥（POST /personal/totp/setup；明文密钥仅本次返回）。
func (h *TOTPController) Setup(c *gin.Context) {
	userId := middleware.GetUserId(c)
	secret, uri, err := h.svc.Setup(c, userId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.TotpSetupRes{Secret: secret, URI: uri})
}

// Activate 确认绑定（POST /personal/totp/activate，body {code}）。
func (h *TOTPController) Activate(c *gin.Context) {
	var req dto.TotpActivateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	if err := h.svc.Activate(c, userId, req.Code); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Disable 停用（POST /personal/totp/disable，body {code}）。
func (h *TOTPController) Disable(c *gin.Context) {
	var req dto.TotpActivateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	if err := h.svc.Disable(c, userId, req.Code); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Reset 管理员重置（PUT /system/user/totp/reset，body {userId}；仅超管）。
// 生成新密钥并一次性返回——enforced 部署的系统内恢复/入职通道。
func (h *TOTPController) Reset(c *gin.Context) {
	var req dto.TotpResetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	operatorId := middleware.GetUserId(c)
	secret, uri, err := h.svc.AdminReset(c, operatorId, req.UserId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.TotpSetupRes{Secret: secret, URI: uri})
}
