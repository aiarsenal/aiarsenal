package service

import (
	"context"
	"errors"
	"strconv"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/alert"
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

// Delete / Clear 删除/清空登录日志。
//
// operator 由 controller 从登录态取（middleware.GetClaims），用于告警埋点：审计日志
// 的删除/清空是等保 8.1.4.3c 的敏感动作，操作者+行数必须进 aa_alert_record
// （event=log_tamper_attempt）。此处只负责如实上报，不做授权判定。
func (s *LoginLogService) Delete(ctx context.Context, operator string, ids []string) error {
	err := s.repo.Delete(ctx, ids)
	alertLogTamper(operator, "登录日志", "删除", len(ids), err)
	return err
}
func (s *LoginLogService) Clear(ctx context.Context, operator string) error {
	err := s.repo.Clear(ctx)
	alertLogTamper(operator, "登录日志", "清空", -1, err)
	return err
}

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
func (s *OperLogService) Delete(ctx context.Context, operator string, ids []string) error {
	err := s.repo.Delete(ctx, ids)
	alertLogTamper(operator, "操作日志", "删除", len(ids), err)
	return err
}
func (s *OperLogService) Clear(ctx context.Context, operator string) error {
	err := s.repo.Clear(ctx)
	alertLogTamper(operator, "操作日志", "清空", -1, err)
	return err
}

// alertLogTamper 日志篡改告警：落库必达（等保 8.1.4.3c），外送受 alert.enabled 与节流控制。
// 行数 -1 表示清空（全表）；操作者为空时如实标注「未知操作者」。repo 调用**之后**发出
// 并带结果——「尝试」语义=请求已受理，成败如实入正文（失败尝试对审计更有价值），
// 行数是提交数而非实际删除数（失败时不应留下既成事实的记录）。
func alertLogTamper(operator, logKind, action string, rows int, opErr error) {
	who := operator
	if who == "" {
		who = "未知操作者"
	}
	detail := who + " 提交" + action + logKind
	if rows < 0 {
		detail += "（全表清空）"
	} else {
		detail += " " + strconv.Itoa(rows) + " 行"
	}
	if opErr != nil {
		detail += "：执行失败（" + opErr.Error() + "）"
	} else {
		detail += "：执行成功"
	}
	alert.Dispatch("log_tamper_attempt", "error", "审计日志"+action+"尝试", detail, "")
}

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
