package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetCORSHeaders 设置跨域响应头（供 CORS 中间件与 NoMethod 预检共用）
func SetCORSHeaders(c *gin.Context) {
	origin := c.Request.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Trace-Id, X-Request-Id")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Max-Age", "86400")
}

// CORS 跨域中间件；OPTIONS 预检直接 204（仅对匹配到的路由生效）
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetCORSHeaders(c)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// NoMethodHandler 兜底方法不允许：CORS 预检（OPTIONS）走这里返回 204 + CORS 头，
// 其它错误方法返回 405。gin 的 NoMethod 不走中间件链，故在此手动设置 CORS 头。
func NoMethodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetCORSHeaders(c)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.AbortWithStatus(http.StatusMethodNotAllowed)
	}
}

// NoRouteHandler 兜底路由不存在：同样补 CORS 头（gin 的 NoRoute 不走分组中间件链）。
// 否则前端旧页面调已下线的接口时，跨域预检 OPTIONS 拿到无 CORS 头的 404，
// 浏览器判跨域失败、axios 报「网络异常」，误导排查方向；补头后旧页面能拿到明确的 404。
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetCORSHeaders(c)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	}
}
