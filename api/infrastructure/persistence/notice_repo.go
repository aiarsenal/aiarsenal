package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type NoticeRepository struct{ db *gorm.DB }

func NewNoticeRepository() *NoticeRepository { return &NoticeRepository{db: db.DB()} }

func (r *NoticeRepository) List(ctx context.Context, userId, isRead, ntype, startTime, endTime string, pageNum, pageSize int) (list []*entity.Notice, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.Notice{}).Where("user_id = ?", userId)
	if isRead != "" {
		q = q.Where("is_read = ?", isRead)
	}
	if ntype != "" {
		q = q.Where("type = ?", ntype)
	}
	if startTime != "" {
		q = q.Where("created_at >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("created_at <= ?", endTime)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Order("is_read asc, created_at desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *NoticeRepository) UnreadCount(ctx context.Context, userId string) (int64, error) {
	var c int64
	err := r.db.WithContext(ctx).Model(&entity.Notice{}).Where("user_id = ? AND is_read = 0", userId).Count(&c).Error
	return c, err
}

func (r *NoticeRepository) Add(ctx context.Context, n *entity.Notice) error {
	n.NoticeId = idgen.New()
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *NoticeRepository) MarkRead(ctx context.Context, noticeId, userId string) error {
	return r.db.WithContext(ctx).Model(&entity.Notice{}).
		Where("notice_id = ? AND user_id = ?", noticeId, userId).
		Update("is_read", 1).Error
}

func (r *NoticeRepository) MarkAllRead(ctx context.Context, userId string) error {
	return r.db.WithContext(ctx).Model(&entity.Notice{}).
		Where("user_id = ? AND is_read = 0", userId).
		Update("is_read", 1).Error
}

func (r *NoticeRepository) Delete(ctx context.Context, noticeIds []string, userId string) error {
	if len(noticeIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("notice_id IN ? AND user_id = ?", noticeIds, userId).Delete(&entity.Notice{}).Error
}
