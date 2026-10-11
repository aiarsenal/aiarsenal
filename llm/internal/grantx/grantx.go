// Package grantx 是组合授权条件行（GrantRow）的最小求值子集，供网关按用户身份
// 匹配预算适用范围（aa_llm_budget.grant_rules）。
//
// ⚠️ 源出处：api/domain/model/grant.go（module aiarsenal）。两 module 独立构建、
// 不共享依赖，故此处按源复制求值核心（结构体/Hit/Match/MatchAny/SetHits/GroupHits/
// ParseGrantRows），**语义必须与 api 侧严格一致**——api 侧求值逻辑变更时须同步本文件。
// casbin 表达式编译（CompileExpr）为 api 侧菜单专用，网关不走 casbin，不复制。
package grantx

import (
	"encoding/json"
	"errors"
	"strings"
)

// Identity 权限判定主体（api 侧 model.Identity 的求值子集；网关经 internal/identity
// 从 aiarsenal 库实时解析填充，IsAdmin 不入——预算圈人与超管无关）。
type Identity struct {
	UserId     string
	DeptId     string
	RoleIds    []string
	PostIds    []string
	EmployeeNo string
	Groups     []string
	Sets       []string
}

// GrantRow 组合授权条件行：六个维度任意组合，非空维度全部 AND（维度内多值为"命中其一"），
// 多行 OR；Exclude=true 为排除行——条件不成立才算命中该行（黑名单语义）。
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
// 引用维度依赖 t.Groups/t.Sets——须先经 GroupHits/SetHits 解析填充。
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

// SetHits 员工集合命中解析：返回主体工号命中的集合 id（填 Identity.Sets）。
// 空工号 → 不命中任何集合。
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
// fail-closed）；集合引用依赖 t.Sets，先经 SetHits 解析。
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

// ParseGrantRows 解析 aa_llm_budget.grant_rules 的条件行 JSON。
// 空串/空数组 → 空切片（不圈人）；格式非法返回错误（快照加载侧记日志跳过该预算）。
func ParseGrantRows(s string) ([]GrantRow, error) {
	if strings.TrimSpace(s) == "" {
		return []GrantRow{}, nil
	}
	var rows []GrantRow
	if err := json.Unmarshal([]byte(s), &rows); err != nil {
		return nil, errors.New("grant rules 格式错误")
	}
	return rows, nil
}

// ParseEmpNos 解析员工集合成员 JSON（快照加载用）：丢弃空串、去重。
func ParseEmpNos(s string) []string {
	var nos []string
	if err := json.Unmarshal([]byte(s), &nos); err != nil {
		return nil
	}
	out := make([]string, 0, len(nos))
	for _, no := range nos {
		no = strings.TrimSpace(no)
		if no == "" || contains(out, no) {
			continue
		}
		out = append(out, no)
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}
