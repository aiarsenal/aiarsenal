package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Identity 权限判定主体：菜单组合授权、文档/目录读写判定、MCP 令牌交换共用同一主体。
// RoleIds 由 casbin enforcer 请求时实时解析（内存 g 表，免查库且永不过期）；
// DeptId/PostIds/EmployeeNo 取自登录 claims（登录时新鲜解析，换部门/岗位后重新登录生效）；
// Groups/Sets 为请求时实时解析的命中分组/员工集合 id（grantmeta.ResolveIdentity 填充，
// 引用行的表达式引用 r.sub.Groups/Sets，故改分组内容即时生效、无需级联刷新）。
type Identity struct {
	UserId     string   // aa_user.user_id
	DeptId     string   // aa_dept.dept_id；''=无部门
	RoleIds    []string // casbin 角色业务 id
	PostIds    []string // aa_post.post_id
	IsAdmin    bool     // aa_user.is_admin==1
	EmployeeNo string   // aa_user.employee_no 工号；''=未填/老 token
	Groups     []string // 命中的权限分组 id
	Sets       []string // 命中的员工集合 id
}

// Sub casbin ABAC 主体（Enforce 的 r.sub 实参）。
// 字段名即表达式里的 r.sub.X：govaluate fork 经 FieldByName 反射取导出字段，大小写敏感；
// Roles/Posts/Groups/Sets 必须是 []interface{}——in 操作符对集合硬断言该类型（evaluationStage.go inStage），
// 传 []string 会 panic，转换收敛在 Sub() 一处完成。
type Sub struct {
	Id     string
	Dept   string
	Roles  []interface{}
	Posts  []interface{}
	Groups []interface{}
	Sets   []interface{}
}

// Sub 身份 → casbin 主体。
func (t Identity) Sub() Sub {
	return Sub{
		Id:     t.UserId,
		Dept:   t.DeptId,
		Roles:  toAny(t.RoleIds),
		Posts:  toAny(t.PostIds),
		Groups: toAny(t.Groups),
		Sets:   toAny(t.Sets),
	}
}

func toAny(ss []string) []interface{} {
	out := make([]interface{}, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

// GrantRow 组合授权条件行：六个维度任意组合，非空维度全部 AND（维度内多值为"命中其一"），
// 多行 OR；Exclude=true 为排除行——条件不成立才算命中该行（黑名单语义，表达式编译为 !(...)）。
// 维度含两类原子：条件维度（用户/部门/角色/岗位）与引用维度（分组 GroupId / 集合 SetId），
// 引用可单独成行也可与条件维度同行 AND（如「员工集合 & 角色」一行即达）。
// 菜单存 aa_menu.grant_rules（JSON 数组）；文档/目录存 aa_doc_grant（带 action read|write）；
// 权限分组自身词条也用本结构（aa_perm_group.terms）。
type GrantRow struct {
	UserIds []string `json:"userIds"`           // 空=不限
	DeptId  string   `json:"deptId"`            // ''=不限
	RoleIds []string `json:"roleIds"`           // 空=不限
	PostIds []string `json:"postIds"`           // 空=不限
	Exclude bool     `json:"exclude"`           // true=条件取反
	GroupId string   `json:"groupId,omitempty"` // 分组引用（权限分组 id）
	SetId   string   `json:"setId,omitempty"`   // 集合引用（员工集合 id）
}

// Hit 条件本体是否成立（不含 Exclude 取反）：非空维度逐一 AND。
// 引用维度依赖 t.Groups/t.Sets——须先经 grantmeta.ResolveIdentity 解析填充
// （GroupHits 递归解析分组词条时剔除分组引用原子、经闭包替代求值，不走此处）。
func (r GrantRow) Hit(t Identity) bool {
	if len(r.UserIds) > 0 && !contains(r.UserIds, t.UserId) {
		return false
	}
	if r.DeptId != "" && r.DeptId != t.DeptId {
		return false
	}
	if len(r.RoleIds) > 0 && !intersects(t.RoleIds, r.RoleIds) {
		return false
	}
	if len(r.PostIds) > 0 && !intersects(t.PostIds, r.PostIds) {
		return false
	}
	if r.GroupId != "" && !contains(t.Groups, r.GroupId) {
		return false
	}
	if r.SetId != "" {
		// 空工号短路：集合语义依赖工号，无工号主体不命中任何集合
		if t.EmployeeNo == "" || !contains(t.Sets, r.SetId) {
			return false
		}
	}
	return true
}

// Match 该行是否命中主体：条件成立 XOR 排除标志。
func (r GrantRow) Match(t Identity) bool { return r.Hit(t) != r.Exclude }

// MatchAny 多行 OR：任一行命中即 true。
func MatchAny(rows []GrantRow, t Identity) bool {
	for _, r := range rows {
		if r.Match(t) {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// SetHits 员工集合命中解析：返回主体工号命中的集合 id（填 Identity.Sets）。
// 空工号 → 不命中任何集合（保存侧已拒绝空串成员，双保险防 ” 误全员命中）。
func SetHits(sets map[string][]string, t Identity) []string {
	if t.EmployeeNo == "" {
		return nil
	}
	hits := make([]string, 0, len(sets))
	for id, nos := range sets {
		if contains(nos, t.EmployeeNo) {
			hits = append(hits, id)
		}
	}
	return hits
}

// GroupHits 权限分组命中解析：返回主体命中的分组 id（填 Identity.Groups）。
// 分组词条行多行 OR、行内各原子 AND；分组引用递归解析（memo+visiting 防环，环=不命中
// fail-closed，保存侧另有 DFS 拒环，此处仅保运行期终止）；集合引用依赖 t.Sets，先经 SetHits 解析。
func GroupHits(groups map[string][]GrantRow, sets map[string][]string, t Identity) []string {
	t.Sets = SetHits(sets, t)
	t.Groups = nil
	memo := make(map[string]bool, len(groups))
	visiting := make(map[string]bool)
	var groupHit func(id string) bool
	rowMatch := func(r GrantRow) bool {
		// 分组引用原子经闭包递归求值（t.Groups 尚未得出，不能走 Hit）；
		// 混行剔除该原子后其余维度（含集合引用，t.Sets 已填）仍按 AND 组合
		gid := r.GroupId
		r.GroupId = ""
		ok := r.Hit(t)
		if gid != "" {
			ok = ok && groupHit(gid)
		}
		return ok != r.Exclude
	}
	groupHit = func(id string) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		if visiting[id] {
			return false
		}
		visiting[id] = true
		v := false
		for _, r := range groups[id] {
			if rowMatch(r) {
				v = true
				break
			}
		}
		visiting[id] = false
		memo[id] = v
		return v
	}
	hits := make([]string, 0, len(groups))
	for id := range groups {
		if groupHit(id) {
			hits = append(hits, id)
		}
	}
	return hits
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

func sanitizeID(id string) string { return strings.ReplaceAll(id, `"`, "") }

// CompileExpr 编译为 casbin eval 表达式（存 aa_rule.v3，由菜单保存流程调用）。
// 行内非空原子逐一 AND，维度内多值 OR。例：{DeptId:"d1", RoleIds:["r2"]} →
// `r.sub.Dept=="d1" && ("r2" in r.sub.Roles)`；分组/集合引用原子 → `("gid" in r.sub.Groups)`
// （引用请求时实时解析的 r.sub.Groups/Sets，改分组内容即时生效）；
// 排除行整体外包一层 !( )。与 Hit/Match 语义严格一致（二者有单测互证）。
func (r GrantRow) CompileExpr() string {
	parts := make([]string, 0, 6)
	if len(r.UserIds) > 0 {
		terms := make([]string, 0, len(r.UserIds))
		for _, id := range r.UserIds {
			terms = append(terms, `r.sub.Id=="`+sanitizeID(id)+`"`)
		}
		parts = append(parts, "("+strings.Join(terms, " || ")+")")
	}
	if r.DeptId != "" {
		parts = append(parts, `r.sub.Dept=="`+sanitizeID(r.DeptId)+`"`)
	}
	if len(r.RoleIds) > 0 {
		parts = append(parts, "("+inExpr("r.sub.Roles", r.RoleIds)+")")
	}
	if len(r.PostIds) > 0 {
		parts = append(parts, "("+inExpr("r.sub.Posts", r.PostIds)+")")
	}
	if r.GroupId != "" {
		parts = append(parts, `"`+sanitizeID(r.GroupId)+`" in r.sub.Groups`)
	}
	if r.SetId != "" {
		parts = append(parts, `"`+sanitizeID(r.SetId)+`" in r.sub.Sets`)
	}
	expr := "true"
	if len(parts) > 0 {
		expr = strings.Join(parts, " && ")
	}
	if r.Exclude {
		expr = "!(" + expr + ")"
	}
	return expr
}

func inExpr(field string, ids []string) string {
	terms := make([]string, 0, len(ids))
	for _, id := range ids {
		terms = append(terms, `"`+sanitizeID(id)+`" in `+field)
	}
	return strings.Join(terms, " || ")
}

// ParseGrantRows 解析前端提交的条件行 JSON（菜单/文档/权限分组保存等入口）。
// 空串/空数组 → 空切片；行内六个维度（用户/部门/角色/岗位/分组引用/集合引用）任意组合、
// 非空维度全部 AND；仅拒绝全空行（多为误加行）。旧 JSON 无新字段 = 零值 = 纯条件行。
func ParseGrantRows(s string) ([]GrantRow, error) {
	if strings.TrimSpace(s) == "" {
		return []GrantRow{}, nil
	}
	var rows []GrantRow
	if err := json.Unmarshal([]byte(s), &rows); err != nil {
		return nil, errors.New("可见条件格式错误")
	}
	for i, r := range rows {
		empty := len(r.UserIds) == 0 && r.DeptId == "" && len(r.RoleIds) == 0 &&
			len(r.PostIds) == 0 && r.GroupId == "" && r.SetId == ""
		if empty {
			return nil, fmt.Errorf("第 %d 行可见条件未填写任何维度", i+1)
		}
	}
	return rows, nil
}

// ParseEmpNos 解析员工集合成员 JSON（保存侧与快照加载共用）：
// trim、丢弃空串、去重，保持首现顺序。格式非法返回错误。
func ParseEmpNos(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}
	var nos []string
	if err := json.Unmarshal([]byte(s), &nos); err != nil {
		return nil, errors.New("集合成员格式错误")
	}
	out := make([]string, 0, len(nos))
	for _, no := range nos {
		no = strings.TrimSpace(no)
		if no == "" || contains(out, no) {
			continue
		}
		out = append(out, no)
	}
	return out, nil
}

// EmpNosJSON 集合成员规范序列化（入库与回显统一形态；空为 "[]"）。
func EmpNosJSON(nos []string) string {
	if len(nos) == 0 {
		return "[]"
	}
	b, err := json.Marshal(nos)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// GrantRowsJSON 条件行规范序列化（入库与回显统一形态；空为 "[]"，避免 NULL/空串两态）。
func GrantRowsJSON(rows []GrantRow) string {
	if len(rows) == 0 {
		return "[]"
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// GrantExprs 每行编译为 casbin 表达式（供 WriteGrantPolicies 逐行落 aa_rule）。
func GrantExprs(rows []GrantRow) []string {
	exprs := make([]string, 0, len(rows))
	for _, r := range rows {
		exprs = append(exprs, r.CompileExpr())
	}
	return exprs
}
