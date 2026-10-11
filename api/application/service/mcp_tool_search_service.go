package service

import (
	"context"
	"strconv"
	"strings"

	"aiarsenal/domain/repository"
)

// MCP 工具向量检索服务（$smart 端点 meta-tool「tool_search」的后端）。
// 索引数据来自 aa_mcp_tool_embedding（registry 发现上游工具时同步 upsert）；
// 查询向量与索引向量同用 LocalEmbedding（256 维 FNV hashing trick，与 aa_skill 同款），
// 检索为 pgvector 余弦相似度（embedding <=> query 向量）。

// ToolEmbeddingDim 工具向量维度（mcp.sql aa_mcp_tool_embedding.embedding vector(256)）。
const ToolEmbeddingDim = 256

// SmartToolName $smart 端点唯一暴露的 meta-tool 名。
const SmartToolName = "tool_search"

// SmartToolDefaultK tool_search 缺省返回条数。
const SmartToolDefaultK = 8

type McpToolSearchService struct {
	repo  repository.IMcpToolEmbeddingRepository
	embed EmbeddingProvider
}

func NewMcpToolSearchService(r repository.IMcpToolEmbeddingRepository) *McpToolSearchService {
	return &McpToolSearchService{repo: r, embed: NewLocalEmbedding(ToolEmbeddingDim)}
}

// ToolHit 单个检索命中。
type ToolHit struct {
	ServerName  string  `json:"serverName"`
	ToolName    string  `json:"toolName"`
	FullName    string  `json:"fullName"` // 聚合命名空间全名：server__tool
	Description string  `json:"description"`
	Score       float64 `json:"score"`
}

// Search 语义检索全部已聚合工具：query → 向量 → pgvector 余弦 TopK。
func (s *McpToolSearchService) Search(ctx context.Context, query string, k int) ([]*ToolHit, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if k <= 0 {
		k = SmartToolDefaultK
	}
	vec, err := s.embed.Embed(query)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SearchByVector(ctx, VectorToPGText(vec), k)
	if err != nil {
		return nil, err
	}
	hits := make([]*ToolHit, 0, len(rows))
	for _, r := range rows {
		server, _ := r["server_name"].(string)
		tool, _ := r["tool_name"].(string)
		desc, _ := r["description"].(string)
		score, _ := r["score"].(float64)
		if server == "" || tool == "" {
			continue
		}
		hits = append(hits, &ToolHit{
			ServerName:  server,
			ToolName:    tool,
			FullName:    server + "__" + tool,
			Description: desc,
			Score:       score,
		})
	}
	return hits, nil
}

// RenderText 把命中渲染为 meta-tool 的文本返回（AI 客户端直读）。
// 附引导语：确定目标工具后改连 /mcp/{server} 直接使用原生工具。
func RenderText(query string, hits []*ToolHit) string {
	var b strings.Builder
	if len(hits) == 0 {
		b.WriteString("未检索到与「" + query + "」相关的工具。可换一个更贴近工具名称/功能的检索词再试。")
		return b.String()
	}
	b.WriteString("共检索到 " + strconv.Itoa(len(hits)) + " 个工具（按相似度降序）。")
	b.WriteString("$smart 端点只提供检索，不直接转发调用：确定目标工具后，请改连 /mcp/{server} 端点（Bearer 鉴权不变）使用其原生工具。\n\n")
	for i, h := range hits {
		b.WriteString(strconv.Itoa(i+1) + ". " + h.FullName + "（相似度 " + strconv.FormatFloat(h.Score, 'f', 3, 64) + "）\n")
		if h.Description != "" {
			b.WriteString("   " + h.Description + "\n")
		}
		b.WriteString("   所属 server：" + h.ServerName + " —— 调用请连接 /mcp/" + h.ServerName + "\n")
	}
	return b.String()
}
