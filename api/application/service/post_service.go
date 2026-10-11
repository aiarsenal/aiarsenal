package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

type PostService struct{ repo repository.IPostRepository }

func NewPostService(r repository.IPostRepository) *PostService { return &PostService{repo: r} }

func (s *PostService) List(ctx context.Context, code, name, status string, pageNum, pageSize int) ([]*entity.Post, int64, error) {
	return s.repo.List(ctx, code, name, status, pageNum, pageSize)
}
func (s *PostService) Add(ctx context.Context, p *entity.Post) error {
	// 校验编码唯一
	list, _, _ := s.repo.List(ctx, p.PostCode, "", "", 1, 1)
	if len(list) > 0 {
		return errors.New("岗位编码已存在")
	}
	return s.repo.Add(ctx, p)
}
func (s *PostService) Edit(ctx context.Context, p *entity.Post) error {
	return s.repo.Edit(ctx, p)
}
func (s *PostService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}
