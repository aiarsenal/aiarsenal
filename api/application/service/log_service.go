package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

type LoginLogService struct {
	repo repository.ILoginLogRepository
}

func NewLoginLogService(r repository.ILoginLogRepository) *LoginLogService {
	return &LoginLogService{repo: r}
}

func (s *LoginLogService) List(ctx context.Context, loginName, status, ip, location string, pageNum, pageSize int) ([]*entity.LoginLog, int64, error) {
	return s.repo.List(ctx, loginName, status, ip, location, pageNum, pageSize)
}
func (s *LoginLogService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}
func (s *LoginLogService) Clear(ctx context.Context) error { return s.repo.Clear(ctx) }

type OperLogService struct{ repo repository.IOperLogRepository }

func NewOperLogService(r repository.IOperLogRepository) *OperLogService {
	return &OperLogService{repo: r}
}

func (s *OperLogService) List(ctx context.Context, title, method, operName string, pageNum, pageSize int) ([]*model.OperLogListRes, int64, error) {
	return s.repo.List(ctx, title, method, operName, pageNum, pageSize)
}
func (s *OperLogService) Get(ctx context.Context, operId string) (*model.OperLogInfoRes, error) {
	return s.repo.Get(ctx, operId)
}
func (s *OperLogService) Insert(ctx context.Context, log *entity.OperLog) error {
	return s.repo.Insert(ctx, log)
}
func (s *OperLogService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}
func (s *OperLogService) Clear(ctx context.Context) error { return s.repo.Clear(ctx) }

type UserOnlineService struct {
	repo repository.IUserOnlineRepository
}

func NewUserOnlineService(r repository.IUserOnlineRepository) *UserOnlineService {
	return &UserOnlineService{repo: r}
}

func (s *UserOnlineService) List(ctx context.Context, username, ip string, pageNum, pageSize int) ([]*entity.UserOnline, int64, error) {
	return s.repo.List(ctx, username, ip, pageNum, pageSize)
}
func (s *UserOnlineService) Delete(ctx context.Context, uuid string) error {
	return s.repo.Delete(ctx, uuid)
}
