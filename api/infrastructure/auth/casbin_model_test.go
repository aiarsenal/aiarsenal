package auth

import (
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"

	"aiarsenal/domain/model"
)

// 双形态 matcher 集成测试：存量角色行（v3='g'）与表达式行（eval）混合加载后均正确判定，
// 且 GrantRow.CompileExpr 产物在 casbin 侧与 Go 侧 Match 语义一致。
// matcher 波及全平台鉴权，此处是防回归底线。

func newModelEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromFile("../../resource/casbin/rbac_model.conf")
	if err != nil {
		t.Fatalf("load model: %v", err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return e
}

func enforce(t *testing.T, e *casbin.Enforcer, id model.Identity, obj string) bool {
	t.Helper()
	ok, err := e.Enforce(id.Sub(), obj, "All")
	if err != nil {
		t.Fatalf("enforce %s: %v", obj, err)
	}
	return ok
}

func TestDualFormMatcher(t *testing.T) {
	e := newModelEnforcer(t)

	// 存量形态：角色直授（v3='g'）+ 用户-角色 g 行
	if _, err := e.AddPolicy("role1", "menuA", "All", "g"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddGroupingPolicy("user:u1", "role1"); err != nil {
		t.Fatal(err)
	}

	// 表达式形态：(d1&&r2) || (d3&&r5) || !(d4&&r3) || (u4 白名单) —— 由 CompileExpr 生成
	rows := []model.GrantRow{
		{DeptId: "d1", RoleIds: []string{"r2"}},
		{DeptId: "d3", RoleIds: []string{"r5"}},
		{DeptId: "d4", RoleIds: []string{"r3"}, Exclude: true},
		{UserIds: []string{"u4"}},
	}
	for _, r := range rows {
		if _, err := e.AddPolicy("cond", "menuB", "All", r.CompileExpr()); err != nil {
			t.Fatal(err)
		}
	}

	// 同一菜单新旧两种行并存
	if _, err := e.AddPolicy("role1", "menuC", "All", "g"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddPolicy("cond", "menuC", "All", model.GrantRow{UserIds: []string{"u9"}}.CompileExpr()); err != nil {
		t.Fatal(err)
	}

	hasRole1 := model.Identity{UserId: "u1", RoleIds: []string{"role1"}}
	noRole1 := model.Identity{UserId: "u2", RoleIds: []string{"r7"}}

	// 角色行：有角色可见、无角色不可见
	if !enforce(t, e, hasRole1, "menuA") {
		t.Error("menuA: 有 role1 应可见")
	}
	if enforce(t, e, noRole1, "menuA") {
		t.Error("menuA: 无 role1 不应可见")
	}

	// 表达式行：与 TestMatchAnyOrSemantics 相同的四类主体
	yes := model.Identity{UserId: "u1", DeptId: "d1", RoleIds: []string{"r2"}}
	yesExclude := model.Identity{UserId: "u2", DeptId: "d9", RoleIds: []string{}}
	yesBoth := model.Identity{UserId: "u4", DeptId: "d4", RoleIds: []string{"r3"}}
	no := model.Identity{UserId: "u3", DeptId: "d4", RoleIds: []string{"r3"}}

	for _, c := range []struct {
		id   model.Identity
		want bool
	}{{yes, true}, {yesExclude, true}, {yesBoth, true}, {no, false}} {
		if got := enforce(t, e, c.id, "menuB"); got != c.want {
			t.Errorf("menuB %+v: got %v want %v", c.id, got, c.want)
		}
		// Go 侧判定与 casbin 侧互证
		if got := model.MatchAny(rows, c.id); got != c.want {
			t.Errorf("MatchAny %+v: got %v want %v", c.id, got, c.want)
		}
	}

	// 并存菜单：任一形态命中即可
	if !enforce(t, e, hasRole1, "menuC") {
		t.Error("menuC: 角色行应命中")
	}
	if !enforce(t, e, model.Identity{UserId: "u9"}, "menuC") {
		t.Error("menuC: 表达式行应命中")
	}
	if enforce(t, e, model.Identity{UserId: "u8"}, "menuC") {
		t.Error("menuC: 两者皆不中不应可见")
	}

	// 空角色集不 panic、未授权菜单不可见
	if enforce(t, e, model.Identity{UserId: "u1"}, "menuZ") {
		t.Error("menuZ: 无策略不应可见")
	}
}

// 引用行（权限分组/员工集合）集成：表达式引用请求时实时解析的 r.sub.Groups/Sets，
// 与 Go 侧 Hit/Match 互证（含 include/exclude 两种形态）。分组/集合分两个菜单隔离测，
// 避免排除行的"广泛放行"语义交叉干扰断言。
func TestRefMatcher(t *testing.T) {
	e := newModelEnforcer(t)

	groupRows := []model.GrantRow{
		{GroupId: "g1"},
		{GroupId: "g2", Exclude: true},
	}
	setRows := []model.GrantRow{
		{SetId: "s1"},
		{SetId: "s2", Exclude: true},
	}
	for _, r := range append(append([]model.GrantRow{}, groupRows...), setRows...) {
		menu := "menuG"
		if r.SetId != "" {
			menu = "menuS"
		}
		if _, err := e.AddPolicy("cond", menu, "All", r.CompileExpr()); err != nil {
			t.Fatal(err)
		}
	}

	groupCases := []struct {
		name string
		id   model.Identity
		want bool
	}{
		{"组内", model.Identity{UserId: "u1", Groups: []string{"g1"}}, true},
		{"组外且不在排除组", model.Identity{UserId: "u1", Groups: []string{"g9"}}, true}, // !g2 成立
		{"仅在排除组", model.Identity{UserId: "u1", Groups: []string{"g2"}}, false},
		{"空命中集", model.Identity{UserId: "u1"}, true}, // !g2 成立
	}
	for _, c := range groupCases {
		if got := enforce(t, e, c.id, "menuG"); got != c.want {
			t.Errorf("%s: casbin got %v want %v", c.name, got, c.want)
		}
		if got := model.MatchAny(groupRows, c.id); got != c.want {
			t.Errorf("%s: MatchAny got %v want %v", c.name, got, c.want)
		}
	}

	setCases := []struct {
		name string
		id   model.Identity
		want bool
	}{
		{"集合成员", model.Identity{UserId: "u1", EmployeeNo: "E1", Sets: []string{"s1"}}, true},
		{"非成员且不在排除集", model.Identity{UserId: "u1", EmployeeNo: "E2", Sets: []string{"s9"}}, true},
		{"仅在排除集", model.Identity{UserId: "u1", EmployeeNo: "E2", Sets: []string{"s2"}}, false},
		{"空工号空命中集", model.Identity{UserId: "u1"}, true}, // !s2 成立
	}
	for _, c := range setCases {
		if got := enforce(t, e, c.id, "menuS"); got != c.want {
			t.Errorf("%s: casbin got %v want %v", c.name, got, c.want)
		}
		if got := model.MatchAny(setRows, c.id); got != c.want {
			t.Errorf("%s: MatchAny got %v want %v", c.name, got, c.want)
		}
	}

	// 混行（员工集合 & 角色）：行内引用原子与条件原子 AND 组合
	mixedRows := []model.GrantRow{{SetId: "s1", RoleIds: []string{"r2"}}}
	for _, r := range mixedRows {
		if _, err := e.AddPolicy("cond", "menuM", "All", r.CompileExpr()); err != nil {
			t.Fatal(err)
		}
	}
	mixedCases := []struct {
		name string
		id   model.Identity
		want bool
	}{
		{"集合成员且持角色", model.Identity{UserId: "u1", EmployeeNo: "E1", Sets: []string{"s1"}, RoleIds: []string{"r2"}}, true},
		{"集合成员但无角色", model.Identity{UserId: "u1", EmployeeNo: "E1", Sets: []string{"s1"}, RoleIds: []string{"r9"}}, false},
		{"持角色但非成员", model.Identity{UserId: "u1", EmployeeNo: "E2", RoleIds: []string{"r2"}}, false},
	}
	for _, c := range mixedCases {
		if got := enforce(t, e, c.id, "menuM"); got != c.want {
			t.Errorf("%s: casbin got %v want %v", c.name, got, c.want)
		}
		if got := model.MatchAny(mixedRows, c.id); got != c.want {
			t.Errorf("%s: MatchAny got %v want %v", c.name, got, c.want)
		}
	}
}
