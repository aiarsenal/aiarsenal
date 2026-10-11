package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// IDocRepository 文档管理仓储（aa_doc 库）
type IDocRepository interface {
	// List 分页；dirIds 非空 = 限定这些目录（含子树展开后的集合），空 = 全部
	List(ctx context.Context, keyword, fileType string, dirIds []string, pageNum, pageSize int) ([]*entity.Doc, int64, error)
	// ListVisible 同 List，叠加组合授权读谓词 + 目录门（t nil = 不限制；readableDirIds nil = 仅根下文档）
	ListVisible(ctx context.Context, keyword, fileType string, dirIds []string, pageNum, pageSize int,
		t *model.Identity, readableDirIds []string) ([]*entity.Doc, int64, error)
	Get(ctx context.Context, docId string) (*entity.Doc, error)
	GetByTitle(ctx context.Context, title string) (*entity.Doc, error)
	GetByHash(ctx context.Context, fileHash string) (*entity.Doc, error)
	Add(ctx context.Context, d *entity.Doc, chunks []*entity.DocChunk) error
	Edit(ctx context.Context, d *entity.Doc) error
	// SetVisibility 文档可见性档位（0私有 1按条件 2全员）
	SetVisibility(ctx context.Context, docId string, visibility uint) error
	ReplaceChunks(ctx context.Context, docId string, d *entity.Doc, chunks []*entity.DocChunk) error
	Delete(ctx context.Context, docIds []string) error
	// SearchChunks 向量检索；t 非-nil 叠加授权读谓词 + 目录门
	SearchChunks(ctx context.Context, queryVectorText string, limit int, t *model.Identity, readableDirIds []string) ([]map[string]interface{}, error)
	ListChunkContents(ctx context.Context, docId string) (string, error)
	// MoveDocs 批量改所属目录（dirId ''=根）
	MoveDocs(ctx context.Context, docIds []string, dirId string) error
	// ListByDirIds 取目录集合内全量文档（重嵌目录用；不分页）
	ListByDirIds(ctx context.Context, dirIds []string) ([]*entity.Doc, error)
	// CountByDir 各目录直属文档数（dir_id → 数量）
	CountByDir(ctx context.Context) (map[string]int64, error)
}
