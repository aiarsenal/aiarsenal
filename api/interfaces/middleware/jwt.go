package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/auth"
)

// JWTAuth 解析 Bearer token，校验并写入当前用户到 context（替代旧 gtoken 中间件 + Ctx）
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			Unauthorized(c, "未登录或登录已过期")
			return
		}
		claims, err := auth.Parse(token)
		if err != nil {
			Unauthorized(c, "登录状态已失效，请重新登录")
			return
		}
		// 用 userId 回查用户信息写入上下文（供 CasbinAuth / 业务取用）
		u := &model.LoginUserRes{
			Id:         claims.UserId,
			UserName:   claims.UserName,
			IsAdmin:    claims.IsAdmin,
			DeptId:     claims.DeptId,
			PostIds:    claims.PostIds,
			EmployeeNo: claims.EmployeeNo,
			DeviceId:   claims.DeviceId,
		}
		SetUser(c, u)
		c.Next()
	}
}

// extractToken 从 Authorization 头取 Bearer token，回退到 query token
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if t := c.Query("token"); t != "" {
		return t
	}
	return ""
}
