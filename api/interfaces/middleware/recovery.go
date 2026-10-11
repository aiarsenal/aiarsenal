package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery 兜底 panic 恢复
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
				c.AbortWithStatusJSON(http.StatusInternalServerError, Body{
					Code: -1, Data: nil, Message: "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}
