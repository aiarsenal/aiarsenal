package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// IDocAccessLogRepository 访问记录仓储（aa_doc 库）
type IDocAccessLogRepository interface {
	Add(ctx context.Context, l *entity.DocAccessLog) error
	// List 分页查询（action/docId 可选过滤；按时间倒序；回填 doc_title 由 service 做）
	List(ctx context.Context, action, docId string, pageNum, pageSize int) ([]*entity.DocAccessLog, int64, error)
}
