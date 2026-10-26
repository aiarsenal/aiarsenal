package dto

import "aiarsenal/domain/model"

// UserLoginReq 登录请求（字段名与旧 system.UserLoginReq 一致）
type UserLoginReq struct {
	Username   string `json:"username" form:"username" binding:"required"`
	Password   string `json:"password" form:"password" binding:"required"`
	VerifyCode string `json:"verifyCode" form:"verifyCode" binding:"required"`
	VerifyKey  string `json:"verifyKey" form:"verifyKey"`
	TOTPCode   string `json:"totpCode" form:"totpCode"` // 已绑定双因子的账号必填
}

// UserLoginRes 登录返回（字段名与旧 system.UserLoginRes 一致）
type UserLoginRes struct {
	UserInfo    *model.LoginUserRes `json:"userInfo"`
	Token       string              `json:"token"`
	MenuList    []*model.UserMenus  `json:"menuList"`
	Permissions []string            `json:"permissions"`
}
