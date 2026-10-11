package model

import "testing"

func TestGrantRowHit(t *testing.T) {
	dept1r2 := Identity{UserId: "u1", DeptId: "d1", RoleIds: []string{"r2"}}
	dept3r5 := Identity{UserId: "u2", DeptId: "d3", RoleIds: []string{"r5"}}
	dept4r3 := Identity{UserId: "u3", DeptId: "d4", RoleIds: []string{"r3"}}
	dept4r3r2 := Identity{UserId: "u4", DeptId: "d4", RoleIds: []string{"r3", "r2"}}

	cases := []struct {
		name string
		row  GrantRow
		id   Identity
		want bool
	}{
		{"d1且r2_命中", GrantRow{DeptId: "d1", RoleIds: []string{"r2"}}, dept1r2, true},
		{"d1且r2_部门不符", GrantRow{DeptId: "d1", RoleIds: []string{"r2"}}, dept3r5, false},
		{"d1且r2_角色不符", GrantRow{DeptId: "d1", RoleIds: []string{"r2"}}, dept4r3, false},
		{"多角色任一命中", GrantRow{DeptId: "d1", RoleIds: []string{"r2", "r5"}}, dept1r2, true},
		{"指定用户命中", GrantRow{UserIds: []string{"u1", "u9"}}, dept1r2, true},
		{"指定用户未命中", GrantRow{UserIds: []string{"u9"}}, dept1r2, false},
		{"全空=不限", GrantRow{}, dept4r3, true},
		{"岗位维度", GrantRow{DeptId: "d4", PostIds: []string{"p1"}}, Identity{UserId: "u5", DeptId: "d4", PostIds: []string{"p1", "p2"}}, true},
		{"排除行_条件成立不命中", GrantRow{DeptId: "d4", RoleIds: []string{"r3"}, Exclude: true}, dept4r3, false},
		{"排除行_条件不成立命中", GrantRow{DeptId: "d4", RoleIds: []string{"r3"}, Exclude: true}, dept1r2, true},
		{"排除行_跨维度不成立也命中", GrantRow{DeptId: "d4", RoleIds: []string{"r3"}, Exclude: true}, dept3r5, true},
	}
	for _, c := range cases {
		if got := c.row.Match(c.id); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	_ = dept4r3r2
}

func TestMatchAnyOrSemantics(t *testing.T) {
	// (d1&&r2) || (d3&&r5) || !(d4&&r3) || (用户u4 白名单)
	rows := []GrantRow{
		{DeptId: "d1", RoleIds: []string{"r2"}},
		{DeptId: "d3", RoleIds: []string{"r5"}},
		{DeptId: "d4", RoleIds: []string{"r3"}, Exclude: true},
		{UserIds: []string{"u4"}},
	}
	yes := Identity{UserId: "u1", DeptId: "d1", RoleIds: []string{"r2"}}
	yesExclude := Identity{UserId: "u2", DeptId: "d9", RoleIds: []string{}}  // 走排除行
	yesBoth := Identity{UserId: "u4", DeptId: "d4", RoleIds: []string{"r3"}} // 被排除但命中白名单行（OR 允许条款优先）
	no := Identity{UserId: "u3", DeptId: "d4", RoleIds: []string{"r3"}}      // 仅被排除

	if !MatchAny(rows, yes) || !MatchAny(rows, yesExclude) || !MatchAny(rows, yesBoth) {
		t.Error("应命中")
	}
	if MatchAny(rows, no) {
		t.Error("d4&&r3 且无允许条款，不应命中")
	}
}

func TestRefRowHitAndMatch(t *testing.T) {
	inG := Identity{UserId: "u1", Groups: []string{"g1", "g2"}}
	inS := Identity{UserId: "u1", EmployeeNo: "E001", Sets: []string{"s1"}}
	inSR2 := Identity{UserId: "u1", DeptId: "d1", EmployeeNo: "E001", Sets: []string{"s1"}, RoleIds: []string{"r2"}}
	noEmp := Identity{UserId: "u1"} // 空工号：集合一律不命中

	cases := []struct {
		name string
		row  GrantRow
		id   Identity
		want bool
	}{
		{"分组引用_命中", GrantRow{GroupId: "g1"}, inG, true},
		{"分组引用_未命中", GrantRow{GroupId: "g9"}, inG, false},
		{"分组排除_命中内不命中", GrantRow{GroupId: "g1", Exclude: true}, inG, false},
		{"分组排除_命中外命中", GrantRow{GroupId: "g9", Exclude: true}, inG, true},
		{"集合引用_命中", GrantRow{SetId: "s1"}, inS, true},
		{"集合引用_未命中", GrantRow{SetId: "s9"}, inS, false},
		{"集合引用_空工号不命中", GrantRow{SetId: "s1"}, noEmp, false},
		{"集合排除_成员被排除", GrantRow{SetId: "s1", Exclude: true}, inS, false},
		{"集合排除_非成员放行", GrantRow{SetId: "s1", Exclude: true}, Identity{UserId: "u2", EmployeeNo: "E002"}, true},
		// 混行：行内全部非空维度 AND
		{"集合且角色_双命中", GrantRow{SetId: "s1", RoleIds: []string{"r2"}}, inSR2, true},
		{"集合且角色_角色不符", GrantRow{SetId: "s1", RoleIds: []string{"r2"}}, inS, false},
		{"集合且角色_集合不符", GrantRow{SetId: "s9", RoleIds: []string{"r2"}}, inSR2, false},
		{"集合且部门", GrantRow{SetId: "s1", DeptId: "d1"}, inSR2, true},
		{"分组且集合_双命中", GrantRow{GroupId: "g1", SetId: "s1"}, Identity{Groups: []string{"g1"}, EmployeeNo: "E001", Sets: []string{"s1"}}, true},
		{"分组且集合_仅分组", GrantRow{GroupId: "g1", SetId: "s1"}, Identity{Groups: []string{"g1"}, EmployeeNo: "E002"}, false},
		{"混行排除_条件成立被排除", GrantRow{SetId: "s1", RoleIds: []string{"r2"}, Exclude: true}, inSR2, false},
		{"混行排除_条件不成立放行", GrantRow{SetId: "s1", RoleIds: []string{"r2"}, Exclude: true}, inS, true},
	}
	for _, c := range cases {
		if got := c.row.Match(c.id); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestSetHits(t *testing.T) {
	sets := map[string][]string{
		"s1": {"E001", "E002"},
		"s2": {"E003"},
	}
	got := SetHits(sets, Identity{UserId: "u1", EmployeeNo: "E002"})
	if len(got) != 1 || got[0] != "s1" {
		t.Errorf("E002 应只命中 s1，got %v", got)
	}
	if got := SetHits(sets, Identity{EmployeeNo: ""}); len(got) != 0 {
		t.Errorf("空工号不应命中任何集合，got %v", got)
	}
}

func TestGroupHits(t *testing.T) {
	sets := map[string][]string{"s1": {"E001"}}
	// g1 = (d1&&r2) || g2；g2 = ||!s1（排除集合成员）；g3 = ||s1；g4 = ||(s1&&r2)（混行 AND）；g5 = ||(g2&&d5)（引用与条件同行）
	groups := map[string][]GrantRow{
		"g1": {
			{DeptId: "d1", RoleIds: []string{"r2"}},
			{GroupId: "g2"},
		},
		"g2": {{SetId: "s1", Exclude: true}},
		"g3": {{SetId: "s1"}},
		"g4": {{SetId: "s1", RoleIds: []string{"r2"}}},
		"g5": {{GroupId: "g2", DeptId: "d5"}},
	}

	// d1&&r2 命中 g1；非 d1&&r2 但非 s1 成员 → 经 g2 命中 g1；s1 成员且不满足条件行 → 不命中
	if got := GroupHits(groups, sets, Identity{UserId: "u1", DeptId: "d1", RoleIds: []string{"r2"}}); !contains(got, "g1") || contains(got, "g3") {
		t.Errorf("d1&&r2 应命中 g1 不命中 g3，got %v", got)
	}
	got := GroupHits(groups, sets, Identity{UserId: "u2", DeptId: "d9", EmployeeNo: "E002"})
	if !contains(got, "g1") || !contains(got, "g2") {
		t.Errorf("非成员应经 g2 递归命中 g1/g2，got %v", got)
	}
	got = GroupHits(groups, sets, Identity{UserId: "u3", DeptId: "d9", EmployeeNo: "E001"})
	if contains(got, "g1") || !contains(got, "g3") {
		t.Errorf("s1 成员且条件不成立：g1 不应命中、g3 应命中，got %v", got)
	}
	// 混行 AND：s1 成员且 r2 → g4；仅其一 → 不命中
	got = GroupHits(groups, sets, Identity{UserId: "u4", DeptId: "d9", EmployeeNo: "E001", RoleIds: []string{"r2"}})
	if !contains(got, "g4") || !contains(got, "g3") {
		t.Errorf("s1 成员且 r2 应命中 g4（g3 也命中），got %v", got)
	}
	got = GroupHits(groups, sets, Identity{UserId: "u5", DeptId: "d9", EmployeeNo: "E001", RoleIds: []string{"r9"}})
	if contains(got, "g4") {
		t.Errorf("s1 成员但角色不符不应命中 g4，got %v", got)
	}
	// 分组引用与条件同行：非 s1 成员（g2 命中）且 d5 → g5；g2 命中但部门不符 → 不命中
	got = GroupHits(groups, sets, Identity{UserId: "u6", DeptId: "d5", EmployeeNo: "E002"})
	if !contains(got, "g5") {
		t.Errorf("g2 命中且 d5 应经混行命中 g5，got %v", got)
	}
	got = GroupHits(groups, sets, Identity{UserId: "u7", DeptId: "d9", EmployeeNo: "E002"})
	if contains(got, "g5") {
		t.Errorf("g2 命中但部门不符不应命中 g5，got %v", got)
	}
}

func TestGroupHitsCycleFailClosed(t *testing.T) {
	// a→b→c→a 环 + c 含一条对所有人的条件排除行之外的允许行兜底验证环路径本身不贡献命中
	cyclic := map[string][]GrantRow{
		"a": {{GroupId: "b"}},
		"b": {{GroupId: "c"}},
		"c": {{GroupId: "a"}, {DeptId: "dX"}},
	}
	if got := GroupHits(cyclic, nil, Identity{UserId: "u1", DeptId: "dY"}); len(got) != 0 {
		t.Errorf("纯环路径应 fail-closed 全不命中，got %v", got)
	}
	if got := GroupHits(cyclic, nil, Identity{UserId: "u1", DeptId: "dX"}); !contains(got, "c") {
		t.Errorf("c 有独立允许行应命中自身，got %v", got)
	}
}

func TestCompileExprRefs(t *testing.T) {
	cases := []struct {
		row  GrantRow
		want string
	}{
		{GrantRow{GroupId: "g1"}, `"g1" in r.sub.Groups`},
		{GrantRow{GroupId: "g1", Exclude: true}, `!("g1" in r.sub.Groups)`},
		{GrantRow{SetId: "s1"}, `"s1" in r.sub.Sets`},
		{GrantRow{SetId: "s1", Exclude: true}, `!("s1" in r.sub.Sets)`},
		{GrantRow{SetId: "s1", RoleIds: []string{"r2"}}, `("r2" in r.sub.Roles) && "s1" in r.sub.Sets`},
		{GrantRow{GroupId: "g1", SetId: "s1"}, `"g1" in r.sub.Groups && "s1" in r.sub.Sets`},
		{GrantRow{DeptId: "d1", SetId: "s1", Exclude: true}, `!(r.sub.Dept=="d1" && "s1" in r.sub.Sets)`},
	}
	for _, c := range cases {
		if got := c.row.CompileExpr(); got != c.want {
			t.Errorf("%+v: got %s want %s", c.row, got, c.want)
		}
	}
}

func TestParseGrantRowsValidation(t *testing.T) {
	// 旧 payload（无新字段）零迁移兼容
	legacy := `[{"userIds":["u1"],"deptId":"","roleIds":[],"postIds":[],"exclude":false}]`
	rows, err := ParseGrantRows(legacy)
	if err != nil || len(rows) != 1 || rows[0].GroupId != "" || rows[0].SetId != "" {
		t.Errorf("旧 payload 应原样解析: rows=%v err=%v", rows, err)
	}

	ok, err := ParseGrantRows(`[{"groupId":"g1"},{"setId":"s1","exclude":true},{"deptId":"d1"},{"setId":"s1","roleIds":["r2"]}]`)
	if err != nil || len(ok) != 4 {
		t.Errorf("纯引用/纯条件/混行均应合法: err=%v", err)
	}

	bad := map[string]string{
		"全空行":   `[{"groupId":"","setId":"","deptId":""}]`,
		"空数组元素": `[{"userIds":[]}]`,
	}
	for name, s := range bad {
		if _, err := ParseGrantRows(s); err == nil {
			t.Errorf("%s 应报错", name)
		}
	}
}
