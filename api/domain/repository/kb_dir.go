package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// IKbDirRepository 目录树仓储（aa_kb 库）
type IKbDirRepository interface {
	// ListAll 全量目录（树小，一次取回内存组树/算子树）
	ListAll(ctx context.Context) ([]*entity.KbDir, error)
	Get(ctx context.Context, dirId string) (*entity.KbDir, error)
	GetByName(ctx context.Context, parentId, name string) (*entity.KbDir, error)
	// GetByKindRef 托管目录反查（kind='project'/'personal' + ref_id；部分唯一索引兜底至多一条）
	GetByKindRef(ctx context.Context, kind, refId string) (*entity.KbDir, error)
	Add(ctx context.Context, d *entity.KbDir) error
	Rename(ctx context.Context, dirId, name string) error
	// SetVisibility 目录可见性档位（0私有 1按条件 2全员；子树文档可见性随之在判定层收紧）
	SetVisibility(ctx context.Context, dirId string, visibility uint) error
	// Move 改父（环检测在 service 层做完才允许调）
	Move(ctx context.Context, dirId, parentId string) error
	Delete(ctx context.Context, dirId string) error
	// CountChildren 直属子目录数（删除前校验空目录用）
	CountChildren(ctx context.Context, dirId string) (int64, error)
}
