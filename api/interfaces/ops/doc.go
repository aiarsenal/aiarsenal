package ops

// 文档库 MCP（读写分开两个 server）：
//
//	buildDocServer       /mcp/doc        platform-doc        doc_search / doc_read / doc_list（只读）
//	buildDocWriteServer  /mcp/doc-write  platform-doc-write  doc_create / doc_update（写，自动重嵌）
//
// 工具是 DocService 的薄壳——与 Web UI 上传/检索同一套业务逻辑（清洗/分块/嵌入/检索
// 都在 application/service，单源）；本文件只做参数解析与输出格式化。
// 文档内容为业务资料：只在库与内存中流转，绝不打日志、绝不落仓库
// （aa_doc 连接的 gorm logger 已收 Silent，防 SQL 错误日志带出正文参数）。

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
	"aiarsenal/domain/model"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/grantmeta"
)

// identityFromCtx 从请求 ctx 取调用方身份（网关令牌交换验签后注入；见 server.go）。
// actor 的 owner（用户 id）→ 权限身份：dept/employee_no/is_admin=aa_user、posts=aa_user_post、
// roles=aa_rule g 行（与 api 侧 casbin GetImplicitRolesForUser 同源），
// groups/sets 经 grantmeta 快照实时解析（本进程无写路径，靠 TTL 兜底刷新）。
// 未取到 = 系统身份 nil（loopback 信任，判定层全放行，访问记录记 system）。
func identityFromCtx(ctx context.Context) *model.Identity {
	a, ok := ctx.Value(ctxActorKey{}).(actorInfo)
	if !ok || a.Owner == "" {
		return nil
	}
	return identityByUserId(a.Owner)
}

// identityByUserId 按用户 id 组装六维权限身份（identityFromCtx 的主体；
// 预算域「系统身份显式 user_id」路径复用同一组装逻辑）。
func identityByUserId(uid string) *model.Identity {
	t := &model.Identity{UserId: uid, RoleIds: []string{}, PostIds: []string{}}
	g := db.DB()
	var u struct {
		DeptId     string
		EmployeeNo string
		IsAdmin    int
	}
	if err := g.Raw("select dept_id, employee_no, is_admin from aa_user where user_id = ?", uid).Scan(&u).Error; err == nil {
		t.DeptId = u.DeptId
		t.EmployeeNo = u.EmployeeNo
		t.IsAdmin = u.IsAdmin == 1
	}
	_ = g.Raw("select v1 from aa_rule where ptype = 'g' and v0 = ?", "u_"+uid).Scan(&t.RoleIds).Error
	_ = g.Raw("select post_id from aa_user_post where user_id = ?", uid).Scan(&t.PostIds).Error
	grantmeta.ResolveIdentity(t)
	return t
}

// actorUserId 访问记录用：有身份记真实用户，无身份记 ""（列表回显为 MCP/system）
func actorUserId(t *model.Identity) string {
	if t == nil {
		return ""
	}
	return t.UserId
}

// ===== 读 server（platform-doc，/mcp/doc） =====

func buildDocServer(docSvc *service.DocService) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-doc", Version: "1.0"}, nil)

	// 语义检索：query → 向量 → 块 top-k（带文档标题聚合）
	opsTool(srv, "doc_search", "检索文档库（真实数据）：语义向量 top-k 命中块，带文档标题与相似度；用 doc_read 取全文",
		[]toolArg{{"query", "检索内容", true}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			q := argStr(a, "query", "")
			if q == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			t := identityFromCtx(ctx)
			rows, err := docSvc.Search(ctx, q, clampInt(argInt(a, "limit", 5), 1, 20), actorUserId(t), "mcp", t)
			return jsonOut(rows, err)
		})

	// 读全文（抽取清洗后的文本）
	opsTool(srv, "doc_read", "读文档全文（真实数据）：按 doc_id 或标题取抽取后的完整文本与元数据",
		[]toolArg{{"doc_id", "文档 id（优先）", false}, {"title", "标题精确匹配", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			docId, title := argStr(a, "doc_id", ""), argStr(a, "title", "")
			if docId == "" && title == "" {
				return "", fmt.Errorf("doc_id 与 title 至少给一个")
			}
			t := identityFromCtx(ctx)
			d, err := docSvc.ReadDoc(ctx, docId, title, t)
			if err != nil {
				return "未找到文档", nil
			}
			content, err := docSvc.GetContent(ctx, d.DocId, actorUserId(t), "mcp", t)
			if err != nil {
				return "", err
			}
			out := map[string]any{
				"doc_id":      d.DocId,
				"title":       d.Title,
				"summary":     d.Summary,
				"file_name":   d.FileName,
				"file_type":   d.FileType,
				"char_count":  d.CharCount,
				"chunk_count": d.ChunkCount,
				"content":     content,
				"created_at":  fmtTime(d.CreatedAt, "2006-01-02 15:04"),
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})

	// 文档列表（标题/摘要/文件名模糊）
	opsTool(srv, "doc_list", "查文档库列表（真实数据）：标题/摘要/文件名模糊匹配",
		[]toolArg{{"keyword", "关键词", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, _, err := docSvc.List(ctx, argStr(a, "keyword", ""), "", "", 1, clampInt(argInt(a, "limit", 10), 1, 50), identityFromCtx(ctx))
			if err != nil {
				return "", err
			}
			if len(list) == 0 {
				return "（无数据）", nil
			}
			rows := make([]map[string]any, 0, len(list))
			for _, d := range list {
				rows = append(rows, map[string]any{
					"doc_id":      d.DocId,
					"title":       d.Title,
					"summary":     d.Summary,
					"file_name":   d.FileName,
					"file_type":   d.FileType,
					"char_count":  d.CharCount,
					"chunk_count": d.ChunkCount,
					"updated_at":  fmtTime(d.UpdatedAt, "2006-01-02 15:04"),
				})
			}
			b, _ := json.MarshalIndent(rows, "", "  ")
			return string(b), nil
		})

	return srv
}

// ===== 写 server（platform-doc-write，/mcp/doc-write） =====

func buildDocWriteServer(docSvc *service.DocService) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-doc-write", Version: "1.0"}, nil)

	// 新建文档（纯文本内容；与 UI 上传的文件型文档同库同检索）
	opsTool(srv, "doc_create", "新建文档（写入真实库）：标题 + 文本内容，自动清洗分块并做向量嵌入；同 hash 内容跳过",
		[]toolArg{{"title", "文档标题", true}, {"text", "文档正文（纯文本）", true}, {"summary", "摘要（可选，参与检索）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			text, _ := a["text"].(string)
			// 有调用方身份则其为 owner（天然读写全权），否则记 mcp（系统）
			owner := actorUserId(identityFromCtx(ctx))
			if owner == "" {
				owner = "mcp"
			}
			d, created, err := docSvc.IngestText(ctx, argStr(a, "title", ""), argStr(a, "summary", ""), text, owner, "mcp")
			if err != nil {
				return "", err
			}
			if !created {
				return fmt.Sprintf("内容已存在，doc_id=%s（跳过）", d.DocId), nil
			}
			return fmt.Sprintf("已创建：doc_id=%s，%d 字 / %d 块", d.DocId, d.CharCount, d.ChunkCount), nil
		})

	// 更新文档（text 给了 → 全量重嵌；只改标题/摘要 → 也重嵌标题块）
	opsTool(srv, "doc_update", "更新文档（写入真实库）：按 doc_id 改标题/摘要/正文，正文变更即重新分块重嵌向量",
		[]toolArg{{"doc_id", "文档 id", true}, {"title", "新标题（可选）", false}, {"summary", "新摘要（可选）", false}, {"text", "新正文（可选；给了才替换）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			text, _ := a["text"].(string)
			d, err := docSvc.UpdateText(ctx, argStr(a, "doc_id", ""), argStr(a, "title", ""), argStr(a, "summary", ""), text, identityFromCtx(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已更新：%d 字 / %d 块（向量已重嵌）", d.CharCount, d.ChunkCount), nil
		})

	return srv
}

// fmtTime types.Time → 指定格式字符串（nil 安全）
func fmtTime(t *types.Time, layout string) string {
	if t == nil {
		return ""
	}
	return time.Time(*t).Format(layout)
}
