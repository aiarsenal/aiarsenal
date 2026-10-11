package entity

import "aiarsenal/domain/types"

// McpToolEmbedding 已聚合工具的向量索引（mcp 库，表 aa_mcp_tool_embedding）。
// registry 发现上游工具时同步 upsert（server_name+tool_name 唯一），上游工具下线即清理；
// /mcp/$smart 端点的 meta-tool「tool_search」以 pgvector 余弦相似度检索本表。
// Embedding 为 pgvector 文本形（[v1,v2,...]，与 aa_skill.semantic_vector 同写法）：
// 写入经 ?::vector 强转、读取按文本扫描，实体上以 string 承载（256 维 LocalEmbedding，
// 与 application/service/skill_embedding.go 同款 FNV hashing trick）。
type McpToolEmbedding struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ServerName  string      `gorm:"column:server_name" json:"serverName"`
	ToolName    string      `gorm:"column:tool_name" json:"toolName"`
	Description string      `gorm:"column:description" json:"description"`
	Embedding   string      `gorm:"column:embedding;type:vector(256)" json:"-"` // pgvector 文本形，不外露
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpToolEmbedding) TableName() string { return "aa_mcp_tool_embedding" }
