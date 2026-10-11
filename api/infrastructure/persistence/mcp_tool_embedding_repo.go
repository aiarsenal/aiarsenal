package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
)

// MCP 工具向量索引仓储实现（mcp 库，表 aa_mcp_tool_embedding）。
// 向量列为 pgvector：写入经 ?::vector 强转（照 aa_skill UpsertSearchDocument），
// 检索用余弦距离 <=> 排序（照 aa_skill SearchByVector）。向量文本由调用方现算，
// 仓储只管存取，不感知嵌入算法。

type McpToolEmbeddingRepository struct{ db *gorm.DB }

func NewMcpToolEmbeddingRepository() *McpToolEmbeddingRepository {
	return &McpToolEmbeddingRepository{db: db.McpDB()}
}

// Upsert 按 (server_name, tool_name) 冲突更新：描述/向量随最新一次发现刷新。
func (r *McpToolEmbeddingRepository) Upsert(ctx context.Context, serverName, toolName, description, vectorText string) error {
	return r.db.WithContext(ctx).Exec(`
		insert into aa_mcp_tool_embedding (server_name, tool_name, description, embedding, updated_at)
		values (?, ?, ?, ?::vector, now())
		on conflict (server_name, tool_name) do update set
			description=excluded.description, embedding=excluded.embedding, updated_at=now()`,
		serverName, toolName, description, vectorText).Error
}

// DeleteStale 清理某 server 本次发现中未见的旧工具向量（上游工具下线即清理）；
// keepToolNames 为空时清空该 server 全部向量。
func (r *McpToolEmbeddingRepository) DeleteStale(ctx context.Context, serverName string, keepToolNames []string) error {
	q := r.db.WithContext(ctx).Where("server_name = ?", serverName)
	if len(keepToolNames) > 0 {
		q = q.Where("tool_name not in ?", keepToolNames)
	}
	return q.Delete(&entity.McpToolEmbedding{}).Error
}

// DeleteByServers 按 server 名批量清理（server 配置删除时随删）。
func (r *McpToolEmbeddingRepository) DeleteByServers(ctx context.Context, serverNames []string) error {
	if len(serverNames) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("server_name in ?", serverNames).
		Delete(&entity.McpToolEmbedding{}).Error
}

// SearchByVector 余弦相似度检索（cosine 距离 <=>，越小越相似，score=1-距离）。
// join aa_mcp_server 过滤：已删除/停用 server 的存量向量不参与检索（对账之外的兜底）。
func (r *McpToolEmbeddingRepository) SearchByVector(ctx context.Context, queryVectorText string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 8
	}
	var rows []map[string]interface{}
	err := r.db.WithContext(ctx).Raw(`
		select t.server_name as server_name, t.tool_name as tool_name, t.description as description,
			1 - (t.embedding <=> ?::vector) as score
		from aa_mcp_tool_embedding t
		join aa_mcp_server s on s.name = t.server_name and s.deleted_at is null and s.enabled = 1
		where t.embedding is not null
		order by t.embedding <=> ?::vector
		limit ?`, queryVectorText, queryVectorText, limit).Scan(&rows).Error
	return rows, err
}
