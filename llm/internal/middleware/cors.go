package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 允许公开域名（及本地开发）跨域。生产/测试域名由 config.Domain 注入。
func CORS(allowOrigin string) gin.HandlerFunc {
	// 允许的来源：公开域名 + 本地管理后台常用端口。
	allowed := map[string]bool{
		allowOrigin: true,
		"http://localhost:8810": true,
		"http://127.0.0.1:8810": true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, x-session-id")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}
		if strings.EqualFold(c.Request.Method, http.MethodOptions) {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
