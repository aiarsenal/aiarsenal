package middleware

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
)

// ClientDeviceAuth 桌面端设备登录态校验（在 JWTAuth 之后运行）。
//   - JWTAuth 已解析 claims 并把 DeviceId 写入 context（LoginUserRes.DeviceId）。
//   - deviceId 为空（web 端登录 token）→ 直接放行，不影响 web 端。
//   - deviceId 非空（桌面 App token）→ 校验设备是否仍 active 且未封禁；
//     revoked/blocked 返回 401，App 收到后清 token 重新登录。
func ClientDeviceAuth(svc *service.ClientService) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := GetUser(c)
		if u == nil {
			// JWTAuth 未通过本不该到这；兜底拦截
			Unauthorized(c, "未登录")
			return
		}
		if u.DeviceId == "" {
			c.Next()
			return
		}
		active, err := svc.IsDeviceActive(c, u.DeviceId)
		if err != nil {
			Unauthorized(c, "登录状态校验失败，请重新登录")
			return
		}
		if !active {
			Unauthorized(c, "登录状态已失效，请重新登录")
			return
		}
		c.Next()
	}
}
