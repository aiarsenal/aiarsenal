package service

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// F-017b G5：skill/mcp 读面端点的门禁清单守卫（漂移门禁）——router.go 里每一个
// GET 端点都必须在下方清单登记处置类别；新增端点漏登记即红（防止「修复面之外
// 又长出一个同型端点」）。处置类别语义：
//
//	skill-scope：service 层可见性/归属谓词（canReadSkill 族，含列表 scope）
//	taxonomy-open：跨技能共享分类法元数据（label 定义——设计上公开）
//	admin-menu：管理面资源，菜单授权即语义（casbin 闸门 + 服务层 canManage 等）
//	self-scope：本人数据面（claims userId 圈定）
//	market-open：技能市场公开浏览面
var endpointGateManifest = map[string]string{
	// ---- skill ----
	"/api/v1/skill/skill/list":              "skill-scope",
	"/api/v1/skill/skill/get":               "skill-scope",
	"/api/v1/skill/skill/search":            "skill-scope",
	"/api/v1/skill/skill/version/list":      "skill-scope",
	"/api/v1/skill/skill/version/get":       "skill-scope",
	"/api/v1/skill/skill/version/file/list": "skill-scope",
	"/api/v1/skill/skill/version/download":  "skill-scope",
	"/api/v1/skill/skill/file/download":     "skill-scope",
	"/api/v1/skill/skill/tag/list":          "skill-scope",
	"/api/v1/skill/security/list":           "skill-scope",
	"/api/v1/skill/namespace/list":          "skill-scope",
	"/api/v1/skill/namespace/all":           "skill-scope",
	"/api/v1/skill/namespace/get":           "skill-scope",
	"/api/v1/skill/namespace/member/list":   "skill-scope",
	"/api/v1/skill/label/all":               "taxonomy-open",
	"/api/v1/skill/label/get":               "taxonomy-open",
	"/api/v1/skill/label/list":              "taxonomy-open",
	"/api/v1/skill/label/trans/list":        "taxonomy-open",
	"/api/v1/skill/market/search":           "market-open",
	"/api/v1/skill/market/site/all":         "market-open",
	"/api/v1/skill/market/site/list":        "market-open",
	"/api/v1/skill/market/sources":          "market-open",
	// ---- mcp ----
	"/api/v1/mcp/server/list":       "skill-scope", // 属主过滤（非超管仅自己）
	"/api/v1/mcp/server/all":        "skill-scope",
	"/api/v1/mcp/server/get":        "skill-scope",
	"/api/v1/mcp/activity/list":     "admin-menu",
	"/api/v1/mcp/config/get":        "admin-menu",
	"/api/v1/mcp/config/user/get":   "self-scope",
	"/api/v1/mcp/group/list":        "admin-menu",
	"/api/v1/mcp/group/all":         "admin-menu",
	"/api/v1/mcp/group/get":         "admin-menu",
	"/api/v1/mcp/group/grant/get":   "admin-menu", // 服务层 canManage
	"/api/v1/mcp/key/list":          "admin-menu",
	"/api/v1/mcp/key/get":           "admin-menu",
	"/api/v1/mcp/oauth/client/list": "admin-menu",
	"/api/v1/mcp/oauth/token/list":  "admin-menu",
	"/api/v1/mcp/prompt/list":       "admin-menu",
	"/api/v1/mcp/prompt/get":        "admin-menu",
	"/api/v1/mcp/resource/list":     "admin-menu",
	"/api/v1/mcp/resource/get":      "admin-menu",
}

// TestSkillMcpEndpointGateManifest 漂移门禁：新增未登记端点 → 红。
func TestSkillMcpEndpointGateManifest(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // application -> api
	src, err := os.ReadFile(filepath.Join(root, "interfaces", "router", "router.go"))
	if err != nil {
		t.Fatalf("读取 router.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	groups := map[string][2]string{}
	defRe := regexp.MustCompile(`^(\w+)\s*:=\s*([\w.]+)\.Group\("([^"]*)"`)
	routeRe := regexp.MustCompile(`^(\w+)\.GET\("([^"]*)"`)
	for _, raw := range lines {
		l := strings.TrimSpace(strings.SplitN(raw, "//", 2)[0])
		if m := defRe.FindStringSubmatch(l); m != nil {
			groups[m[1]] = [2]string{m[2], m[3]}
		}
	}
	full := func(g string) string {
		parts := []string{}
		cur := g
		for {
			def, ok := groups[cur]
			if !ok {
				break
			}
			parts = append([]string{def[1]}, parts...)
			cur = def[0]
		}
		return strings.Join(parts, "")
	}
	var missing []string
	seen := map[string]bool{}
	for _, raw := range lines {
		l := strings.TrimSpace(strings.SplitN(raw, "//", 2)[0])
		m := routeRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		g := m[1]
		if _, ok := groups[g]; !ok {
			continue
		}
		p := full(g) + m[2]
		if !strings.HasPrefix(p, "/api/v1/skill") && !strings.HasPrefix(p, "/api/v1/mcp") {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, ok := endpointGateManifest[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("存在未登记处置类别的 skill/mcp 读面端点（新增端点必须登记门禁清单）：%v", missing)
	}
	if len(seen) < 35 {
		t.Fatalf("解析异常：仅解析出 %d 条 skill/mcp GET 端点（应 ~40），解析器与 router.go 形态漂移", len(seen))
	}
}
