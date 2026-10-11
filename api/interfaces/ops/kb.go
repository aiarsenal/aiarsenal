package ops

// 知识库 MCP（读写分开两个 server）：
//
//	buildKbServer       /mcp/kb        platform-kb        kb_search / kb_read / kb_list（只读）
//	buildKbWriteServer  /mcp/kb-write  platform-kb-write  kb_create / kb_update（写，自动重嵌）
//
// 工具是 KbService 的薄壳——与 Web UI 上传/检索同一套业务逻辑（清洗/分块/嵌入/检索
// 都在 application/service，单源）；本文件只做参数解析与输出格式化。
// 文档内容为业务资料：只在库与内存中流转，绝不打日志、绝不落仓库
// （aa_kb 连接的 gorm logger 已收 Silent，防 SQL 错误日志带出正文参数）。

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
)

// ===== 读 server（platform-kb，/mcp/kb） =====

func buildKbServer(kbSvc *service.KbService) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-kb", Version: "1.0"}, nil)

	// 语义检索：query → 向量 → 块 top-k（带文档标题聚合）
	opsTool(srv, "kb_search", "检索知识库（真实数据）：语义向量 top-k 命中块，带文档标题与相似度；用 kb_read 取全文",
		[]toolArg{{"query", "检索内容", true}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			q := argStr(a, "query", "")
			if q == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			t := identityFromCtx(ctx)
			rows, err := kbSvc.Search(ctx, q, clampInt(argInt(a, "limit", 5), 1, 20), actorUserId(t), "mcp", t)
			return jsonOut(rows, err)
		})

	// 读全文（抽取清洗后的文本）
	opsTool(srv, "kb_read", "读文档全文（真实数据）：按 doc_id 或标题取抽取后的完整文本与元数据",
		[]toolArg{{"doc_id", "文档 id（优先）", false}, {"title", "标题精确匹配", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			docId, title := argStr(a, "doc_id", ""), argStr(a, "title", "")
			if docId == "" && title == "" {
				return "", fmt.Errorf("doc_id 与 title 至少给一个")
			}
			t := identityFromCtx(ctx)
			d, err := kbSvc.ReadDoc(ctx, docId, title, t)
			if err != nil {
				return "未找到文档", nil
			}
			content, err := kbSvc.GetContent(ctx, d.DocId, actorUserId(t), "mcp", t)
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
	opsTool(srv, "kb_list", "查知识库列表（真实数据）：标题/摘要/文件名模糊匹配",
		[]toolArg{{"keyword", "关键词", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, _, err := kbSvc.List(ctx, argStr(a, "keyword", ""), "", "", 1, clampInt(argInt(a, "limit", 10), 1, 50), identityFromCtx(ctx))
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

// ===== 写 server（platform-kb-write，/mcp/kb-write） =====

func buildKbWriteServer(kbSvc *service.KbService) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-kb-write", Version: "1.0"}, nil)

	// 新建文档（纯文本内容；与 UI 上传的文件型文档同库同检索）
	opsTool(srv, "kb_create", "新建文档（写入真实库）：标题 + 文本内容，自动清洗分块并做向量嵌入；同 hash 内容跳过",
		[]toolArg{{"title", "文档标题", true}, {"text", "文档正文（纯文本）", true}, {"summary", "摘要（可选，参与检索）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			text, _ := a["text"].(string)
			// 有调用方身份则其为 owner（天然读写全权），否则记 mcp（系统）
			owner := actorUserId(identityFromCtx(ctx))
			if owner == "" {
				owner = "mcp"
			}
			d, created, err := kbSvc.IngestText(ctx, argStr(a, "title", ""), argStr(a, "summary", ""), text, owner, "mcp")
			if err != nil {
				return "", err
			}
			if !created {
				return fmt.Sprintf("内容已存在，doc_id=%s（跳过）", d.DocId), nil
			}
			return fmt.Sprintf("已创建：doc_id=%s，%d 字 / %d 块", d.DocId, d.CharCount, d.ChunkCount), nil
		})

	// 更新文档（text 给了 → 全量重嵌；只改标题/摘要 → 也重嵌标题块）
	opsTool(srv, "kb_update", "更新文档（写入真实库）：按 doc_id 改标题/摘要/正文，正文变更即重新分块重嵌向量",
		[]toolArg{{"doc_id", "文档 id", true}, {"title", "新标题（可选）", false}, {"summary", "新摘要（可选）", false}, {"text", "新正文（可选；给了才替换）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			text, _ := a["text"].(string)
			d, err := kbSvc.UpdateText(ctx, argStr(a, "doc_id", ""), argStr(a, "title", ""), argStr(a, "summary", ""), text, identityFromCtx(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已更新：%d 字 / %d 块（向量已重嵌）", d.CharCount, d.ChunkCount), nil
		})

	return srv
}
