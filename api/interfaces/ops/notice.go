package ops

// 通知域（aa_notice，主库）：站内个人通知的读与写。
// 读端点 platform-notice（/mcp/notice）、写端点 platform-notice-write（/mcp/notice-write）。
// 收件人锁定：有调用方身份（网关令牌交换）时一律锁定为 actor 本人，工具不收 user_id 参数；
// 系统身份（console 探针/stdio 冒烟）才允许显式 user_id——网关路径恒有 actor，安全不降。

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/infrastructure/db"
)

// noticeSelfUid 解析「操作自己收件箱」类工具的 user_id：actor 锁定，系统身份显式传参
func noticeSelfUid(ctx context.Context, a map[string]any) (string, error) {
	if t := identityFromCtx(ctx); t != nil {
		return t.UserId, nil
	}
	uid := argStr(a, "user_id", "")
	if uid == "" {
		return "", fmt.Errorf("缺少调用方身份，须显式传 user_id")
	}
	return uid, nil
}

// noticeValidType 校验通知类型枚举（与 aa_notice.type 一致）
func noticeValidType(t string) bool {
	switch t {
	case "info", "warning", "success", "error":
		return true
	}
	return false
}

// ===== 读 server（platform-notice，/mcp/notice） =====

func buildNoticeServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-notice", Version: "1.0"}, nil)

	opsTool(srv, "notice_list", "查通知列表（真实数据）：当前调用方的站内通知，按时间倒序；type=info|warning|success|error，is_read=0|1（缺省全部）",
		[]toolArg{{"type", "按类型过滤", false}, {"is_read", "按已读过滤：0=未读 1=已读（缺省全部）", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := noticeSelfUid(ctx, a)
			if err != nil {
				return "", err
			}
			t := argStr(a, "type", "")
			if t != "" && !noticeValidType(t) {
				return "", fmt.Errorf("type 取值：info | warning | success | error")
			}
			list, total, err := d.Notice.List(ctx, uid, argStr(a, "is_read", ""), t, "", "",
				clampInt(argInt(a, "page", 1), 1, 10000), clampInt(argInt(a, "limit", 10), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, n := range list {
				items = append(items, map[string]any{
					"noticeId": n.NoticeId, "title": n.Title, "content": n.Content,
					"type": n.Type, "link": n.Link, "isRead": n.IsRead,
					"createdBy": n.CreatedBy, "createdAt": fmtTime(n.CreatedAt, "2006-01-02 15:04"),
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "notice_unread_count", "查未读通知数量（真实数据）：当前调用方的站内未读数",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := noticeSelfUid(ctx, a)
			if err != nil {
				return "", err
			}
			n, err := d.Notice.UnreadCount(ctx, uid)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("未读 %d 条", n), nil
		})

	opsTool(srv, "notice_user_lookup", "查平台用户（真实数据）：按姓名/工号模糊匹配得 user_id 与部门，发通知前先用来解析收件人",
		[]toolArg{{"keyword", "姓名或工号", true}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			kw := argStr(a, "keyword", "")
			if kw == "" {
				return "", fmt.Errorf("keyword 不能为空")
			}
			rows, err := queryRows(db.DB(), `
SELECT u.user_id, u.name, u.employee_no, u.user_status,
       coalesce(d.dept_name, '')                AS dept_name,
       to_char(u.created_at, 'YYYY-MM-DD')      AS created_at
FROM aa_user u
LEFT JOIN aa_dept d ON d.dept_id = u.dept_id
WHERE (u.name ILIKE '%' || $1 || '%' OR u.employee_no ILIKE '%' || $1 || '%' OR u.user_id ILIKE '%' || $1 || '%')
  AND u.deleted_at IS NULL
ORDER BY u.name LIMIT $2`, kw, clampInt(argInt(a, "limit", 5), 1, 20))
			return jsonOut(rows, err)
		})

	return srv
}

// ===== 写 server（platform-notice-write，/mcp/notice-write） =====

func buildNoticeWriteServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-notice-write", Version: "1.0"}, nil)

	opsTool(srv, "notice_send", "发站内通知（写真实库）：给指定 user_id 的用户发一条通知，UI 右上角【通知】可见；发前先用 notice_user_lookup 解析收件人；只发必要通知，勿刷屏",
		[]toolArg{{"user_id", "收件人 user_id（notice_user_lookup 查得）", true}, {"title", "标题", true}, {"content", "正文", true}, {"link", "跳转链接：http*=外链 /xxx=平台内路由 #xxx=弹窗（可选）", false}, {"type", "类型：info|warning|success|error，默认 info", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid := argStr(a, "user_id", "")
			if uid == "" {
				return "", fmt.Errorf("user_id 不能为空")
			}
			title, content := argStr(a, "title", ""), argStr(a, "content", "")
			if title == "" || content == "" {
				return "", fmt.Errorf("title / content 不能为空")
			}
			ntype := argStr(a, "type", "info")
			if !noticeValidType(ntype) {
				return "", fmt.Errorf("type 取值：info | warning | success | error")
			}
			link := argStr(a, "link", "")
			if link != "" && !(len(link) >= 4 && link[:4] == "http") && link[0] != '/' && link[0] != '#' {
				return "", fmt.Errorf("link 约定：http*（外链）/ /xxx（平台内路由）/ #xxx（弹窗），收到：%q", link)
			}
			by := actorUserId(identityFromCtx(ctx))
			if by == "" {
				by = "mcp"
			}
			if err := d.Notice.Push(ctx, uid, title, content, link, ntype, by); err != nil {
				return "", err
			}
			return fmt.Sprintf("已发送给 %s：%s", uid, title), nil
		})

	opsTool(srv, "notice_mark_read", "标记通知已读（写真实库）：按 notice_id 标记当前调用方的一条通知",
		[]toolArg{{"notice_id", "通知 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := noticeSelfUid(ctx, a)
			if err != nil {
				return "", err
			}
			if err := d.Notice.MarkRead(ctx, argStr(a, "notice_id", ""), uid); err != nil {
				return "", err
			}
			return "已标记已读", nil
		})

	opsTool(srv, "notice_mark_all_read", "全部标记已读（写真实库）：当前调用方的全部未读通知置为已读",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := noticeSelfUid(ctx, a)
			if err != nil {
				return "", err
			}
			if err := d.Notice.MarkAllRead(ctx, uid); err != nil {
				return "", err
			}
			return "已全部标记已读", nil
		})

	return srv
}
