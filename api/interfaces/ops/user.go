package ops

// 员工信息查询域（aa_user/aa_dept，主库）：只读目录查询，给 agent 解析「人」用。
// 端点 platform-user（/mcp/user），无写端点——员工档案由管理端维护，agent 只查不改。
// 纯 queryRows 读模型（UI 的用户服务走密钥/会话路径，不适合 MCP 复用）；列一律显式，
// 绝不 SELECT *——aa_user 有 user_password/user_salt/last_login_ip 等敏感列，禁止带出。

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/infrastructure/db"
)

// userSexName 性别枚举转中文（与 UI sysuser 字典一致：1=男 0=女 2=未知）
func userSexName(sex any) string {
	switch v := sex.(type) {
	case int64:
		switch v {
		case 1:
			return "男"
		case 0:
			return "女"
		}
	case float64:
		switch int(v) {
		case 1:
			return "男"
		case 0:
			return "女"
		}
	}
	return "未知"
}

// userStatusName 账号状态（user_status：1=正常，其余=停用）
func userStatusName(st any) string {
	if v, ok := st.(int64); ok && v == 1 {
		return "正常"
	}
	if v, ok := st.(float64); ok && int(v) == 1 {
		return "正常"
	}
	return "停用"
}

// buildUserServer 员工信息查询（platform-user，/mcp/user）
func buildUserServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-user", Version: "1.0"}, nil)

	// 查当前调用方自己：身份来自网关令牌交换（bearer key 归属人，server.go actorCtxFromRequest
	// 注入 ctx），不查发消息的渠道账号——「谁在问」在 MCP 链路上=「谁的 key 在调用」。
	opsTool(srv, "user_me", "查我自己（当前调用方）的档案（真实数据）：无需参数，按调用令牌归属人返回姓名、部门、工号、岗位、角色、联系方式；用户问「我是谁/我的信息」、写个人介绍、需要当前用户身份时先用它，不用再让用户报姓名工号",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			t := identityFromCtx(ctx)
			if t == nil {
				return "当前调用无用户身份（系统路径）——本工具按调用令牌归属人返回档案；直连 ops 或未带 bearer key 的调用查不到", nil
			}
			rows, err := queryRows(db.DB(), `
SELECT u.user_id, u.name, u.employee_no, u.user_name, u.mobile, u.user_email, u.sex,
       u.is_admin,
       to_char(u.created_at, 'YYYY-MM-DD') AS joined_at,
       coalesce(d.dept_name, '') AS dept_name,
       coalesce(d2.dept_name, '') AS parent_dept_name
FROM aa_user u
LEFT JOIN aa_dept d  ON d.dept_id = u.dept_id
LEFT JOIN aa_dept d2 ON d2.dept_id = d.parent_id
WHERE u.user_id = $1 AND u.deleted_at IS NULL`, t.UserId)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return fmt.Sprintf("令牌归属用户 %s 在员工表里不存在（可能已删除）", t.UserId), nil
			}
			r := rows[0]
			r["sex"] = userSexName(r["sex"])
			isAdmin := false
			if v, ok := r["is_admin"].(int64); ok && v == 1 {
				isAdmin = true
			}
			delete(r, "is_admin")
			r["isAdmin"] = isAdmin
			// 岗位/角色补名称（identityFromCtx 只解析 id 集，名称在此按 id 现查）
			if posts, e := queryRows(db.DB(), `
SELECT p.post_name FROM aa_post p
JOIN aa_user_post up ON up.post_id = p.post_id
WHERE up.user_id = $1 ORDER BY p.post_sort`, t.UserId); e == nil {
				names := make([]string, 0, len(posts))
				for _, p := range posts {
					names = append(names, fmt.Sprint(p["post_name"]))
				}
				r["posts"] = names
			}
			if roles, e := queryRows(db.DB(), `
SELECT r.name FROM aa_role r
WHERE r.role_id IN (SELECT v1 FROM aa_rule WHERE ptype = 'g' AND v0 = $1)
ORDER BY r.list_order`, "u_"+t.UserId); e == nil {
				names := make([]string, 0, len(roles))
				for _, x := range roles {
					names = append(names, fmt.Sprint(x["name"]))
				}
				r["roles"] = names
			}
			return jsonOutAny(r)
		})

	opsTool(srv, "user_lookup", "查员工（真实数据）：按姓名/工号/手机号/邮箱/账号名模糊匹配，返回工号、部门、联系方式；发通知、找负责人前先来这里解析人",
		[]toolArg{{"keyword", "姓名/工号/手机号/邮箱/账号名（任一模糊匹配）", true}, {"dept_id", "限定部门 id（dept_list 查得，可选）", false}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			kw := argStr(a, "keyword", "")
			if kw == "" {
				return "", fmt.Errorf("keyword 不能为空")
			}
			q := `
SELECT u.user_id, u.name, u.employee_no, u.mobile, u.user_email, u.sex, u.user_status,
       coalesce(d.dept_name, '') AS dept_name
FROM aa_user u
LEFT JOIN aa_dept d ON d.dept_id = u.dept_id
WHERE (u.name ILIKE '%' || $1 || '%' OR u.employee_no ILIKE '%' || $1 || '%'
       OR u.mobile ILIKE '%' || $1 || '%' OR u.user_email ILIKE '%' || $1 || '%' OR u.user_name ILIKE '%' || $1 || '%')
  AND u.deleted_at IS NULL`
			args := []any{kw}
			if dept := argStr(a, "dept_id", ""); dept != "" {
				q += ` AND u.dept_id = $2`
				args = append(args, dept)
			}
			q += ` ORDER BY u.employee_no LIMIT ` + fmt.Sprint(clampInt(argInt(a, "limit", 5), 1, 20))
			rows, err := queryRows(db.DB(), q, args...)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				r["sex"] = userSexName(r["sex"])
				r["userStatus"] = userStatusName(r["user_status"])
				delete(r, "user_status")
				items = append(items, r)
			}
			return jsonOut(items, nil)
		})

	opsTool(srv, "user_get", "查员工详情（真实数据）：按 user_id 返回完整员工档案（部门、联系方式、备注、最近登录），不含任何凭据",
		[]toolArg{{"user_id", "用户 user_id（user_lookup 查得）", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid := argStr(a, "user_id", "")
			if uid == "" {
				return "", fmt.Errorf("user_id 不能为空")
			}
			rows, err := queryRows(db.DB(), `
SELECT u.user_id, u.name, u.employee_no, u.user_name, u.mobile, u.user_email, u.sex,
       u.address, u.describe, u.remark,
       to_char(u.last_login_time, 'YYYY-MM-DD HH24:MI') AS last_login_at,
       to_char(u.created_at, 'YYYY-MM-DD')              AS joined_at,
       coalesce(d.dept_name, '') AS dept_name,
       coalesce(d2.dept_name, '') AS parent_dept_name
FROM aa_user u
LEFT JOIN aa_dept d  ON d.dept_id = u.dept_id
LEFT JOIN aa_dept d2 ON d2.dept_id = d.parent_id
WHERE u.user_id = $1 AND u.deleted_at IS NULL`, uid)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return fmt.Sprintf("未找到 user_id=%s 的员工", uid), nil
			}
			r := rows[0]
			r["sex"] = userSexName(r["sex"])
			return jsonOutAny(r)
		})

	opsTool(srv, "dept_list", "查部门列表（真实数据）：全量部门树（含在编人数），可按名称过滤；dept_id 供 user_lookup/dept_user_list 使用",
		[]toolArg{{"keyword", "部门名称模糊过滤（可选，缺省全量）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			q := `
SELECT d.dept_id, d.parent_id, d.dept_name, d.order_num, d.status,
       coalesce(lead.name, '') AS leader_name,
       (SELECT count(*) FROM aa_user u
         WHERE u.dept_id = d.dept_id AND u.deleted_at IS NULL AND u.user_status = 1) AS member_count
FROM aa_dept d
LEFT JOIN aa_user lead ON lead.user_id = d.leader_id AND lead.deleted_at IS NULL
WHERE d.deleted_at IS NULL`
			args := []any{}
			if kw := argStr(a, "keyword", ""); kw != "" {
				q += ` AND d.dept_name ILIKE '%' || $1 || '%'`
				args = append(args, kw)
			}
			q += ` ORDER BY d.ancestors, d.order_num LIMIT 200`
			rows, err := queryRows(db.DB(), q, args...)
			if err != nil {
				return "", err
			}
			return jsonOut(rows, nil)
		})

	opsTool(srv, "dept_user_list", "查部门花名册（真实数据）：某部门（可含全部下级）的在编员工列表，分页",
		[]toolArg{
			{"dept_id", "部门 id（dept_list 查得）", true},
			{"include_children", "true=连同全部下级部门（缺省仅本部门）", false},
			{"page", "页码，默认 1", false},
			{"limit", "条数 1-50，默认 20", false},
		},
		func(ctx context.Context, a map[string]any) (string, error) {
			dept := argStr(a, "dept_id", "")
			if dept == "" {
				return "", fmt.Errorf("dept_id 不能为空")
			}
			page := clampInt(argInt(a, "page", 1), 1, 10000)
			limit := clampInt(argInt(a, "limit", 20), 1, 50)
			// 部门范围：仅本部门，或按 aa_dept.ancestors 展开整棵子树
			//（ancestors 形如 0,a,b —— 子孙部门的祖先链必含本部门 id）。
			scope := `u.dept_id = $1`
			if v := argStr(a, "include_children", ""); v == "true" || v == "1" || v == "yes" {
				scope = `(u.dept_id = $1 OR u.dept_id IN (
  SELECT dept_id FROM aa_dept
  WHERE deleted_at IS NULL AND (',' || ancestors || ',') LIKE '%,' || $1 || ',%'))`
			}
			total, err := scalar(db.DB(), `SELECT count(*) FROM aa_user u WHERE `+scope+` AND u.deleted_at IS NULL`, dept)
			if err != nil {
				return "", err
			}
			rows, err := queryRows(db.DB(), `
SELECT u.user_id, u.name, u.employee_no, u.mobile, u.user_email, u.sex, u.user_status,
       coalesce(d.dept_name, '') AS dept_name
FROM aa_user u
LEFT JOIN aa_dept d ON d.dept_id = u.dept_id
WHERE `+scope+` AND u.deleted_at IS NULL
ORDER BY d.ancestors, u.employee_no
LIMIT `+fmt.Sprint(limit)+` OFFSET `+fmt.Sprint((page-1)*limit), dept)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				r["sex"] = userSexName(r["sex"])
				r["userStatus"] = userStatusName(r["user_status"])
				delete(r, "user_status")
				items = append(items, r)
			}
			return jsonOutAny(map[string]any{"total": total, "page": page, "items": items})
		})

	return srv
}
