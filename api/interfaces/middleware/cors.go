package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"aiarsenal/config"
)

// SetCORSHeaders 设置跨域响应头：仅对白名单来源精确放行（CORS 中间件与
// NoMethod/NoRoute 预检兜底共用）。
//
// 安全语义（手册 F-11）：不反射任意 Origin、不发 Access-Control-Allow-Credentials。
// 旧实现对任何来源回显 Origin 并允许带凭据跨域——任意第三方页面可借用户浏览器
// 携凭据调用 API。实际鉴权走 Authorization 头（无 cookie 消费者），credentials
// 头是纯多余的风险面，直接移除。
// 白名单 = config cors.allowedOrigins 显式清单 ∪ 本部署 UI 源
// （public.scheme://public.domain 自动推导，标准「根域 UI + api. 子域」部署零配置）。
// 白名单外与无 Origin 的请求（同源浏览、curl、服务间调用）一律不发 CORS 头——
// 同源请求本就不需要，浏览器侧等价于被拒绝跨域。
func SetCORSHeaders(c *gin.Context) {
	// 响应随 Origin 变化（放行/不放行给出不同头集），必须声明 Vary 让缓存层
	// 不把 A 源的响应串给 B 源——无论是否放行都要带。
	c.Header("Vary", "Origin")
	origin := c.Request.Header.Get("Origin")
	if !originAllowed(origin) {
		return
	}
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Trace-Id, X-Request-Id")
	c.Header("Access-Control-Max-Age", "86400")
}

// originAllowed 白名单判定（config 接线）。精确匹配：Origin 是 scheme+host+port
// 全串，子域（api.aiarsenal.cc）不等于根域（aiarsenal.cc），不存在前缀放行。
func originAllowed(origin string) bool {
	cfg := config.Get()
	return originAllowedFor(origin, cfg.Cors.AllowedOrigins, cfg.PublicScheme(), cfg.PublicDomain())
}

// originAllowedFor 纯函数判定（变异锚点：删显式清单分支或删 public 推导分支必红）。
func originAllowedFor(origin string, allowed []string, scheme, domain string) bool {
	if origin == "" {
		// 纵深防御：即使有人误配清单项为空串，「无 Origin 的请求没有跨域语义」
		// 也必须由本函数自己保证，不依赖清单内容成立。
		return false
	}
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	if domain != "" && origin == scheme+"://"+domain {
		return true
	}
	return false
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
