package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/config"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
)

// CasbinAuth 权限校验中间件（对应旧 system/logic/middleware.Auth）
// 依赖 Menu 仓储做 URL→menuId 映射，casbin enforcer 做 Enforce
func CasbinAuth(repo repository.IMenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// 超管（is_admin=1）免验证
		if u := GetUser(c); u != nil && u.IsAdmin == 1 {
			c.Next()
			return
		}

		// 拼 url（path + accessParams）
		path := strings.TrimLeft(c.Request.URL.Path, "/")
		accessParams := c.QueryArray("accessParams")
		accessParamsStr := ""
		if len(accessParams) > 0 && accessParams[0] != "undefined" {
			accessParamsStr = "?" + strings.Join(accessParams, "&")
		}
		url := path + accessParamsStr

		// URL→menu 映射
		menus, err := repo.GetMenuList(c)
		if err != nil {
			Fail(c, "请求数据失败")
			c.Abort()
			return
		}
		var menu *model.MenuInfoRes
		for _, m := range menus {
			ms := m.Name
			if i := strings.Index(m.Name, "?"); i >= 0 {
				ms = m.Name[:i]
			}
			if m.Name == url || ms == url {
				menu = m
				break
			}
		}

		if menu != nil {
			// 白名单（无需登录可访问）
			for _, p := range cfg.Jwt.ExcludePaths {
				if strings.EqualFold(menu.Name, strings.TrimLeft(p, "/")) {
					c.Next()
					return
				}
			}
			if strings.EqualFold(menu.Condition, "nocheck") {
				c.Next()
				return
			}
			if menu.Id != "" {
				e, err := auth.Enforcer()
				if err != nil {
					Fail(c, "获取权限失败")
					c.Abort()
					return
				}
				// 主体为五元组结构体：角色直授行（v3='g'）走 in r.sub.Roles，
				// 组合授权表达式行（v3=expr）走 eval，同一 Enforce 双形态判定
				ok, err := e.Enforce(GetIdentity(c).Sub(), menu.Id, "All")
				if err != nil {
					Fail(c, "判断权限失败")
					c.Abort()
					return
				}
				if !ok {
					Fail(c, "没有访问权限")
					c.Abort()
					return
				}
			}
		} else if menu == nil && accessParamsStr != "" {
			Fail(c, "没有访问权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
