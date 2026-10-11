package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// IKbAccessLogRepository 访问记录仓储（aa_kb 库）
type IKbAccessLogRepository interface {
	Add(ctx context.Context, l *entity.KbAccessLog) error
	// List 分页查询（action/docId 可选过滤；按时间倒序；回填 doc_title 由 service 做）
	List(ctx context.Context, action, docId string, pageNum, pageSize int) ([]*entity.KbAccessLog, int64, error)
}
