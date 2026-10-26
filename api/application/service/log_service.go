package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
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
	repo     repository.IUserOnlineRepository
	userRepo repository.IUserRepository
}

func NewUserOnlineService(r repository.IUserOnlineRepository, u repository.IUserRepository) *UserOnlineService {
	return &UserOnlineService{repo: r, userRepo: u}
}

func (s *UserOnlineService) List(ctx context.Context, username, ip string, pageNum, pageSize int) ([]*entity.UserOnline, int64, error) {
	return s.repo.List(ctx, username, ip, pageNum, pageSize)
}
func (s *UserOnlineService) Delete(ctx context.Context, uuid string) error {
	return s.repo.Delete(ctx, uuid)
}

// Kick 管理员吊销某用户全部会话（离职/封号的即时回收）。
//
// callerId 必须是超管（库内判定）：吊销会话是接管类管理动作，不能只靠菜单层兜底——
// 路由未登记进菜单表时中间件默认放行，谁登录谁就能踢人。
// 会话注册表整体删除后，该用户所有已签发登录令牌在下一跳 401；
// 在线表行是展示数据，best-effort 清掉，失败不影响吊销本身。
func (s *UserOnlineService) Kick(ctx context.Context, callerId, userId string) error {
	caller, err := s.userRepo.GetEntityById(ctx, callerId)
	if err != nil || caller == nil || caller.IsAdmin != 1 {
		return errors.New("没有访问权限：只有超级管理员可以吊销会话")
	}
	u, err := s.userRepo.GetEntityById(ctx, userId)
	if err != nil || u == nil {
		return errors.New("用户不存在或已被删除")
	}
	auth.RevokeAllSessions(userId)
	if rows, total, err := s.repo.List(ctx, u.UserName, "", 1, 1000); err == nil && total > 0 {
		for _, row := range rows {
			_ = s.repo.Delete(ctx, row.Uuid)
		}
	}
	return nil
}
