package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// INoticeRepository 个人消息通知仓储
type INoticeRepository interface {
	List(ctx context.Context, userId, isRead, ntype, startTime, endTime string, pageNum, pageSize int) (list []*entity.Notice, total int64, err error)
	UnreadCount(ctx context.Context, userId string) (int64, error)
	Add(ctx context.Context, n *entity.Notice) error
	MarkRead(ctx context.Context, noticeId, userId string) error
	MarkAllRead(ctx context.Context, userId string) error
	Delete(ctx context.Context, noticeIds []string, userId string) error
}
