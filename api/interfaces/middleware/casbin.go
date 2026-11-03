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
		menu := menuFor(url, menus)

		if menu != nil {
			// 白名单（无需登录可访问）
			for _, p := range cfg.Jwt.ExcludePaths {
				if strings.EqualFold(strings.TrimLeft(menu.Name, "/"), strings.TrimLeft(p, "/")) {
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

// menuFor URL→菜单映射：比对前对请求路径与菜单名做斜杠归一（手册 F-14）。
// 种子数据里 /api/v1/system/role/edit、/api/v1/system/role/delete 两条按钮记录
// 带斜杠前缀，而请求路径先 TrimLeft——旧比对「去斜杠路径 == 原始菜单名」对这两条
// 永不成立，路由对非超管实际无闸门（casbin 直接放行）。归一双端后既有库无需
// 数据迁移即恢复匹配；首个命中即返回（repo 列表序， weighing 由数据侧保证）。
func menuFor(url string, menus []*model.MenuInfoRes) *model.MenuInfoRes {
	for _, m := range menus {
		ms := strings.TrimLeft(m.Name, "/")
		if j := strings.Index(ms, "?"); j >= 0 {
			ms = ms[:j]
		}
		if ms == url {
			return m
		}
	}
	return nil
}
