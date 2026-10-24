package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type LoginController struct {
	svc *service.AuthService
}

func NewLoginController(s *service.AuthService) *LoginController {
	return &LoginController{svc: s}
}

// Login 用户登录  POST /api/v1/system/login
func (h *LoginController) Login(c *gin.Context) {
	var req dto.UserLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	ip := c.ClientIP()
	ua := c.Request.UserAgent()
	user, token, menus, perms, err := h.svc.Login(c, req.Username, req.Password, req.VerifyKey, req.VerifyCode, ip, ua)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.UserLoginRes{
		UserInfo:    user,
		Token:       token,
		MenuList:    menus,
		Permissions: perms,
	})
}

// Logout 退出登录  GET /api/v1/system/logout
// 会话有服务端状态（jti 注册表）：吊销当前会话，已签发令牌立即失效，前端再丢弃 token。
func (h *LoginController) Logout(c *gin.Context) {
	if claims := middleware.GetClaims(c); claims != nil {
		auth.RevokeSession(claims.UserId, claims.ID)
	}
	middleware.Success(c, struct{}{})
}

// GetUserMenus 当前登录用户的菜单/权限  GET /api/v1/system/user/getUserMenus
func (h *LoginController) GetUserMenus(c *gin.Context) {
	uid := middleware.GetUserId(c)
	isAdmin := false
	if u := middleware.GetUser(c); u != nil {
		isAdmin = u.IsAdmin == 1
	}
	menuList, permissions, err := h.svc.GetAdminRules(c, uid, isAdmin)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"menuList": menuList, "permissions": permissions})
}
