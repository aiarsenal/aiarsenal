package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/config"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
)

// selfScopedPaths 自 scopes 白名单（F-030 默认拒绝的例外）：仅由 JWT 身份直接圈定本人
// 数据、无横越权面的前端必需端点。挂在 protected 组（JWTAuth 先行），未登录仍被拦。
// 逐条依据：logout=吊销本人会话；personal/*=本人资料与改密；getUserMenus=本人菜单树
// （前端导航的取数口）；notice/*=本人站内信（service 层全部按 claims userId 过滤）。
// 白名单无条件前置（优先于菜单登记与 casbin）：四族路由即使已登记授权也直接放行。
var selfScopedPaths = []string{
	"/api/v1/system/logout",
	"/api/v1/system/personal/",
	"/api/v1/system/user/getUserMenus",
	"/api/v1/system/notice/",
}

// isSelfScoped 白名单匹配：目录型条目（尾斜杠）按前缀匹配，精确条目全串相等——
// 目录型防前缀攻击靠尾斜杠（/personal-x 不匹配 /personal/），精确条目不带通配。
func isSelfScoped(path string) bool {
	for _, p := range selfScopedPaths {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
		} else if path == p {
			return true
		}
	}
	return false
}

// CasbinAuth 权限校验中间件（对应旧 system/logic/middleware.Auth）。
// F-030：默认方向改为拒绝——URL→menu 未命中时不再放行（原 menu==nil 分支
// c.Next() 使 567 条注册路由中仅菜单表已登记的少数受控），改为白名单外一律 403。
// 路由匹配用 c.FullPath()（路由模板，含 :id 参数段）：带参数路由此前对静态菜单
// 表永不命中、实质无闸门；迁移 000008 已为全部路由登记模板菜单记录。
// 依赖 Menu 仓储做 URL→menuId 映射，casbin enforcer 做 Enforce。
func CasbinAuth(repo repository.IMenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// 超管（is_admin=1）免验证
		if u := GetUser(c); u != nil && u.IsAdmin == 1 {
			c.Next()
			return
		}

		// 自 scopes 白名单前置：仅 JWT 身份圈定的本人数据面（导航/站内信/资料），
		// 与菜单登记状态无关——已登记但未授权的自 scopes 路由同样放行。
		if isSelfScoped(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 路由模板（gin 已注册路由含 :id 参数段；未注册路径为空串）。
		// accessParams 是历史约定的查询串拼接口（菜单名可含查询串，menuFor 归一时剥离）。
		full := strings.TrimLeft(c.FullPath(), "/")
		accessParams := c.QueryArray("accessParams")
		accessParamsStr := ""
		if len(accessParams) > 0 && accessParams[0] != "undefined" {
			accessParamsStr = "?" + strings.Join(accessParams, "&")
		}
		if full == "" {
			// 未注册路由（理论到不了 gin 404，防御性回落原始路径——同样进默认拒绝）
			full = strings.TrimLeft(c.Request.URL.Path, "/")
		}
		url := full + accessParamsStr

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
			c.Next()
			return
		}

		// menu == nil：默认拒绝（F-030 核心）。
		Fail(c, "没有访问权限")
		c.Abort()
	}
}

// menuFor URL→菜单映射：比对前对请求路径与菜单名做斜杠归一（手册 F-14）。
// 种子数据里 /api/v1/system/role/edit、/api/v1/system/role/delete 两条按钮记录
// 带斜杠前缀，而请求路径先 TrimLeft——旧比对「去斜杠路径 == 原始菜单名」对这两条
// 永不成立，路由对非超管实际无闸门（casbin 直接放行）。归一双端后既有库无需
// 数据迁移即恢复匹配；首个命中即返回（repo 列表序， weighing 由数据侧保证）。
// F-030 起 url 传路由模板（含 :id 段），迁移 000008 按模板登记参数化路由。
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
