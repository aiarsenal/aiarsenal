package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// ---- isSelfScoped：自 scopes 白名单（F-030 默认拒绝的唯一代码级例外） ----

func TestIsSelfScoped(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/v1/system/logout", true},
		{"/api/v1/system/personal/getPersonalInfo", true},
		{"/api/v1/system/personal/edit", true},
		{"/api/v1/system/personal/resetPwd", true},
		{"/api/v1/system/user/getUserMenus", true},
		{"/api/v1/system/notice/list", true},
		{"/api/v1/system/notice/delete", true},
		{"/api/v1/system/user/list", false},             // 非白名单管理面
		{"/api/v1/system/personal-x/evil", false},       // 前缀攻击：personal-x ≠ personal/
		{"/api/v1/system/user/getUserMenus/all", false}, // 白名单精确路径不带通配
		{"/api/v1/skill/skill/list", false},
	}
	for _, c := range cases {
		if got := isSelfScoped(c.path); got != c.want {
			t.Errorf("isSelfScoped(%q)=%v, 期望 %v", c.path, got, c.want)
		}
	}
}

// ---- 默认拒绝行为（F-030 核心）：menu 未命中一律拒，白名单与超管例外 ----

type fakeMenuRepo struct {
	repository.IMenuRepository // 嵌入 nil=哨兵：路径多调一个未实现方法即 panic
	menus                      []*model.MenuInfoRes
}

func (f *fakeMenuRepo) GetMenuList(context.Context) ([]*model.MenuInfoRes, error) {
	return f.menus, nil
}

func setupDenyRouter(repo *fakeMenuRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.String(200, "ok") }
	guarded := r.Group("/api/v1")
	guarded.Use(CasbinAuth(repo))
	// 自 scopes 白名单路径（真实应用中同样挂在 JWTAuth+CasbinAuth 的 protected 组内）
	guarded.GET("/system/logout", ok)
	guarded.GET("/system/personal/getPersonalInfo", ok)
	guarded.GET("/system/notice/list", ok)
	guarded.GET("/system/user/getUserMenus", ok)
	// 受控路由
	guarded.GET("/system/role/edit", ok)
	guarded.GET("/system/never/registered", ok)
	guarded.GET("/system/instance/:id/exec", func(c *gin.Context) { c.String(200, "ok:"+c.Param("id")) })
	guarded.GET("/system/instance/:id/status", ok)
	guarded.GET("/system/instance/:id/logs", ok)
	return r
}

// menuSliceC 带 Condition 的菜单构造（nocheck 接线格用）
func menuSliceC(names map[string]string) []*model.MenuInfoRes {
	out := make([]*model.MenuInfoRes, 0, len(names))
	for n, cond := range names {
		out = append(out, &model.MenuInfoRes{Name: n, Id: "menu-" + n, Condition: cond})
	}
	return out
}

func TestCasbinAuthDefaultDeny(t *testing.T) {
	// 空菜单库（未登记任何路由的极端形态）：默认拒绝 + 自 scopes 放行
	r := setupDenyRouter(&fakeMenuRepo{})

	// 零权限用户 + 未登记路由 → 拒（信封 code=-1，沿用本仓库既有拒绝形态）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/never/registered", nil)
	r.ServeHTTP(w, req)
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应非信封 JSON: %s", w.Body.String())
	}
	if body.Code != -1 || !strings.Contains(body.Message, "没有访问权限") {
		t.Fatalf("未登记路由应被拒: code=%d msg=%s", body.Code, body.Message)
	}

	// 带参数路由未登记 → 同样拒（此前该类路由对静态菜单永不命中、实质无闸门）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/instance/abc/exec", nil)
	r.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != -1 {
		t.Fatalf("带参数未登记路由应被拒: code=%d msg=%s", body.Code, body.Message)
	}

	// 自 scopes 白名单：未登录（GetUser=nil）也被 JWT 层拦——中间件层放行到下一环节
	// （本测试无 JWTAuth，直接断言中间件不拦白名单路径）
	for _, p := range []string{"/api/v1/system/logout", "/api/v1/system/personal/getPersonalInfo",
		"/api/v1/system/user/getUserMenus", "/api/v1/system/notice/list"} {
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, p, nil)
		r.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), "没有访问权限") {
			t.Fatalf("自 scopes 白名单路径 %s 不应被默认拒绝", p)
		}
	}

	// FullPath 接线正样本：参数模板已登记且 nocheck，真实 id 请求命中模板 → 放行。
	// 若中间件回退用 URL.Path（真实 id ≠ :id 模板），menu==nil → 默认拒绝，本格红。
	r3 := setupDenyRouter(&fakeMenuRepo{menus: menuSliceC(map[string]string{
		"api/v1/system/instance/:id/status": "nocheck",
		"api/v1/system/instance/:id/logs":   "",
	})})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/instance/abc/status", nil)
	r3.ServeHTTP(w, req)
	if w.Body.String() != "ok" {
		t.Fatalf("nocheck 参数模板经 FullPath 应命中放行, 得 %s", w.Body.String())
	}
	// 以下两格（nocheck 对照组、已登记未授权格）依赖真实 Enforcer（gorm 连库，
	// 无库环境 gorm open 直接 panic——CI Backend job 以 -short 运行且无 PG）：
	// short 模式在此跳过，全量模式（本地/Postgres job）执行断言。
	if testing.Short() {
		t.Log("short 模式跳过 Enforcer 依赖格（需真库）")
		return
	}

	// 对照组：同型模板无 nocheck（空 Condition）→ Enforce 无授权 → 拒（证明命中的是模板而非误放行）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/instance/abc/logs", nil)
	r3.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "没有访问权限") {
		t.Fatalf("无 nocheck 的参数模板应走 casbin 拒绝, 得 %s", w.Body.String())
	}

	// 已登记 + 无授权 → casbin 判定拒绝（与「未登记默认拒绝」同文案但不同分支）：
	// 此处必须拒绝证明登记路由不会静默放行。
	r2 := setupDenyRouter(&fakeMenuRepo{menus: menuSlice("api/v1/system/role/edit")})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/role/edit", nil)
	r2.ServeHTTP(w, req)
	var body2 struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body2)
	if body2.Code != -1 || !strings.Contains(body2.Message, "没有访问权限") {
		t.Fatalf("已登记但未授权的路应被 casbin 拒绝: code=%d msg=%s", body2.Code, body2.Message)
	}
}

// ---- 静态完整性门禁（报告 R-10 的自动化形态）：全部 CasbinAuth 后的路由模板
// 必须已登记为菜单记录。任一新增路由漏登记 → 本测试红（合并门禁）。 ----

func TestRouteMenuCoverageComplete(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // middleware -> interfaces -> api
	routerSrc, err := os.ReadFile(filepath.Join(root, "interfaces", "router", "router.go"))
	if err != nil {
		t.Fatalf("读取 router.go: %v", err)
	}

	// 1) 组定义（行首 name := parent.Group("...")，含中间件参数）
	groupDefs := map[string]string{} // name -> parent
	groupPath := map[string]string{} // name -> path
	groupCasbin := map[string]bool{} // name -> 定义链上是否有 CasbinAuth
	defRe := regexp.MustCompile(`^(\w+)\s*:=\s*([\w.]+)\.Group\("([^"]*)"`)
	for _, ln := range strings.Split(string(routerSrc), "\n") {
		ln = strings.TrimSpace(ln)
		if i := strings.Index(ln, "//"); i >= 0 {
			ln = strings.TrimSpace(ln[:i])
		}
		if m := defRe.FindStringSubmatch(ln); m != nil {
			groupDefs[m[1]] = m[2]
			groupPath[m[1]] = m[3]
			groupCasbin[m[1]] = strings.Contains(ln, "CasbinAuth")
		}
	}

	// 2) 路由注册 + 组链解析
	type route struct{ path string }
	var gated []string
	seen := map[string]bool{}
	routeRe := regexp.MustCompile(`^(\w+)\.(GET|POST|PUT|DELETE|PATCH|Any|HEAD)\("([^"]*)"`)
	for _, ln := range strings.Split(string(routerSrc), "\n") {
		ln = strings.TrimSpace(ln)
		if i := strings.Index(ln, "//"); i >= 0 {
			ln = strings.TrimSpace(ln[:i])
		}
		m := routeRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		g, path := m[1], m[3]
		if _, ok := groupDefs[g]; !ok {
			continue
		}
		gatedChain := false
		full := ""
		cur := g
		for {
			full = groupPath[cur] + full
			if groupCasbin[cur] {
				gatedChain = true
			}
			parent, ok := groupDefs[cur]
			if !ok || parent == cur {
				break
			}
			cur = parent
		}
		if strings.Contains(ln, "CasbinAuth") {
			gatedChain = true
		}
		if !gatedChain {
			continue
		}
		tmpl := strings.Trim(full+path, "/")
		if !seen[tmpl] {
			seen[tmpl] = true
			gated = append(gated, tmpl)
		}
	}
	if len(gated) < 400 {
		t.Fatalf("路由解析异常：仅解析出 %d 条 CasbinAuth 后路由（应 ~500+），解析器与 router.go 形态漂移", len(gated))
	}

	// 3) 菜单名全集：基线 COPY + 迁移 000008
	menuNames := map[string]bool{}
	baseSQL, err := os.ReadFile(filepath.Join(root, "resource", "data", "aiarsenal.sql"))
	if err != nil {
		t.Fatalf("读取 aiarsenal.sql: %v", err)
	}
	copyRe := regexp.MustCompile(`(?s)COPY public\.aa_menu \([^)]*\) FROM stdin;\n(.*?)\n\\.`)
	if m := copyRe.FindSubmatch(baseSQL); m != nil {
		for _, row := range strings.Split(string(m[1]), "\n") {
			f := strings.Split(row, "\t")
			if len(f) > 3 {
				menuNames[strings.Trim(f[3], "/")] = true
			}
		}
	}
	migSQL, err := os.ReadFile(filepath.Join(root, "infrastructure", "db", "migrations", "aiarsenal",
		"000008_register_all_routes_menu.up.sql"))
	if err != nil {
		t.Fatalf("读取迁移 000008: %v", err)
	}
	// 菜单名一律以 api/ 开头（按钮型记录 name=路由路径），按形态无关的引号串提取
	valRe := regexp.MustCompile(`'(api/[^']*)'`)
	for _, m := range valRe.FindAllStringSubmatch(string(migSQL), -1) {
		menuNames[strings.Trim(m[1], "/")] = true
	}
	if len(menuNames) < 500 {
		t.Fatalf("菜单名解析异常：仅 %d 个（应 ~570）", len(menuNames))
	}

	// 4) 断言：全部受影响路由已登记（自 scopes 白名单路径豁免）
	var missing []string
	for _, tmpl := range gated {
		if menuNames[tmpl] || isSelfScoped("/"+tmpl) {
			continue
		}
		missing = append(missing, tmpl)
	}
	if len(missing) > 0 {
		t.Fatalf("存在 %d 条 CasbinAuth 后路由未登记菜单（默认拒绝后将被误拦）——请在迁移 000008/基线补齐或确认白名单：%v",
			len(missing), missing[:min(10, len(missing))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
