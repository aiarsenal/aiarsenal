package service

import (
	"context"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

type NoticeService struct{ repo repository.INoticeRepository }

func NewNoticeService(r repository.INoticeRepository) *NoticeService {
	return &NoticeService{repo: r}
}

func (s *NoticeService) List(ctx context.Context, userId, isRead, ntype, startTime, endTime string, pageNum, pageSize int) ([]*entity.Notice, int64, error) {
	return s.repo.List(ctx, userId, isRead, ntype, startTime, endTime, pageNum, pageSize)
}

func (s *NoticeService) UnreadCount(ctx context.Context, userId string) (int64, error) {
	return s.repo.UnreadCount(ctx, userId)
}

func (s *NoticeService) MarkRead(ctx context.Context, noticeId, userId string) error {
	return s.repo.MarkRead(ctx, noticeId, userId)
}

func (s *NoticeService) MarkAllRead(ctx context.Context, userId string) error {
	return s.repo.MarkAllRead(ctx, userId)
}

func (s *NoticeService) Delete(ctx context.Context, ids []string, userId string) error {
	return s.repo.Delete(ctx, ids, userId)
}

// Push 给 userId 发一条个人通知（落 aa_notice，UI 右上角【通知】读取）。NoticeId 由 repo.Add 生成。
// 供需要主动通知用户的场景（如实例绑定的 LLM 密钥被封禁/解封）调用。link: http*=外链 /xxx=内部路由 #xxx=弹窗。
func (s *NoticeService) Push(ctx context.Context, userId, title, content, link, ntype, createdBy string) error {
	if userId == "" {
		return nil
	}
	if ntype == "" {
		ntype = "info"
	}
	// CreatedAt 显式置值：entity 的 *types.Time 不走 gorm autoCreateTime，
	// 不设则落 NULL（列表按 created_at desc 排序会乱，时间列也显示不出）。
	now := types.Time(time.Now())
	return s.repo.Add(ctx, &entity.Notice{
		UserId:    userId,
		Title:     title,
		Content:   content,
		Link:      link,
		Type:      ntype,
		CreatedBy: createdBy,
		CreatedAt: &now,
	})
}
