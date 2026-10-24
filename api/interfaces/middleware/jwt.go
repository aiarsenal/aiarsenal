package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
)

const ctxClaimsKey = "currentClaims"

// JWTAuth 解析 Bearer token，校验并写入当前用户到 context（替代旧 gtoken 中间件 + Ctx）
//
// 两道有状态校验（替代"签名通过即放行"的 30 天裸奔）：
//  1. 会话校验：登录令牌的 jti 必须在会话注册表里（登出/管理员吊销即时生效）；
//     容器内 agent 控制面令牌（InstanceId 非空）走 InstanceAuth，不在此校验。
//  2. 身份回查：isAdmin/部门/岗位/状态按 userId 查库取最新值——改权限、封号下一跳生效，
//     token 只承担"证明你是谁"，不再携带"你有什么权限"的有效期承诺。
func JWTAuth(repo repository.IUserRepository) gin.HandlerFunc {
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
		if claims.InstanceId == "" && !auth.SessionValid(claims.UserId, claims.ID) {
			Unauthorized(c, "登录状态已失效，请重新登录")
			return
		}
		// 身份回查（状态/权限以库内为准；DeviceId 不入库，保留 claims 值）
		u, err := repo.GetSessionUser(c, claims.UserId)
		if err != nil || u == nil {
			Unauthorized(c, "登录状态已失效，请重新登录")
			return
		}
		if u.UserStatus == 0 {
			Unauthorized(c, "账号已被冻结，请联系管理员")
			return
		}
		u.DeviceId = claims.DeviceId
		c.Set(ctxClaimsKey, claims)
		SetUser(c, u)
		c.Next()
	}
}

// GetClaims 当前请求的 JWT 载荷（登出取 jti 用）。
func GetClaims(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ctxClaimsKey)
	if !ok {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
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
