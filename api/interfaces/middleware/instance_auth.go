package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/infrastructure/auth"
)

// InstanceAuth 容器内 agent 控制面鉴权：校验 Bearer token 的 Claims.InstanceId 非空。
// 实例创建时签发 instance token（auth.GenerateForInstance），通过环境变量注入容器。
// 命中后把 instanceId 写入 context（GetGetInstanceId）。
func InstanceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			Unauthorized(c, "缺少实例凭证")
			return
		}
		claims, err := auth.Parse(token)
		if err != nil || claims.InstanceId == "" {
			Unauthorized(c, "实例凭证无效")
			return
		}
		c.Set(ctxInstanceIDKey, claims.InstanceId)
		c.Set(ctxInstanceUserIDKey, claims.UserId)
		c.Next()
	}
}

const ctxInstanceIDKey = "instance_id"
const ctxInstanceUserIDKey = "instance_user_id"

// GetInstanceId 从 context 取 InstanceAuth 写入的 instanceId。
func GetInstanceId(c *gin.Context) string {
	if v, ok := c.Get(ctxInstanceIDKey); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	// 回退：query/path 参数
	return c.Param("id")
}
