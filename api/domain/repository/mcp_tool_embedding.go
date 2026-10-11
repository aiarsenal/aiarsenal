package repository

import (
	"context"
)

// MCP 工具向量索引仓储接口（mcp 库，表 aa_mcp_tool_embedding）。
// 向量文本为 pgvector 文本形 [v1,v2,...]，由调用方（registry 同步/网关检索）经
// LocalEmbedding（256 维，与 aa_skill 同款 FNV hashing trick）现算，仓储只管存取。

type IMcpToolEmbeddingRepository interface {
	// Upsert 按 (server_name, tool_name) 冲突更新（描述/向量随最新一次发现刷新）。
	Upsert(ctx context.Context, serverName, toolName, description, vectorText string) error
	// DeleteStale 清理某 server 本次发现中未见的旧工具向量（上游工具下线即清理）；
	// keepToolNames 为空时清空该 server 全部向量。
	DeleteStale(ctx context.Context, serverName string, keepToolNames []string) error
	// DeleteByServers 按 server 名批量清理（server 配置删除时随删）。
	DeleteByServers(ctx context.Context, serverNames []string) error
	// SearchByVector 余弦相似度检索（cosine 距离 <=>，越小越相似）；
	// 仅命中启用中（enabled=1、未软删）server 的工具，返回 server_name/tool_name/description/score。
	SearchByVector(ctx context.Context, queryVectorText string, limit int) ([]map[string]interface{}, error)
}
