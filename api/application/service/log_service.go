package service

import (
	"context"
	"errors"
	"strconv"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/alert"
	"aiarsenal/infrastructure/auth"
)

type LoginLogService struct {
	repo     repository.ILoginLogRepository
	userRepo repository.IUserRepository
}

func NewLoginLogService(r repository.ILoginLogRepository, u repository.IUserRepository) *LoginLogService {
	return &LoginLogService{repo: r, userRepo: u}
}

func (s *LoginLogService) List(ctx context.Context, loginName, status, ip, location string, pageNum, pageSize int) ([]*entity.LoginLog, int64, error) {
	return s.repo.List(ctx, loginName, status, ip, location, pageNum, pageSize)
}

// Delete / Clear 删除/清空登录日志。
//
// callerId 由 controller 从登录态取（claims.UserId）。闸门：audit.protection 档位
// （strict=一律拒绝；flexible=仅超管），判定基于**查库身份**（GetEntityById），不信
// 请求体。被拒的尝试同样进告警（log_tamper_attempt）——strict 档下埋点仍在，
// flexible 档下告警是主要可见面（PLAN PR③ 拍板）。
func (s *LoginLogService) Delete(ctx context.Context, callerId string, ids []string) error {
	callerName, gateErr := s.auditGate(ctx, callerId)
	if gateErr != nil {
		alertLogTamper(callerName, "登录日志", "删除", len(ids), gateErr)
		return gateErr
	}
	err := s.repo.Delete(ctx, ids)
	alertLogTamper(callerName, "登录日志", "删除", len(ids), err)
	return err
}
func (s *LoginLogService) Clear(ctx context.Context, callerId string) error {
	callerName, gateErr := s.auditGate(ctx, callerId)
	if gateErr != nil {
		alertLogTamper(callerName, "登录日志", "清空", -1, gateErr)
		return gateErr
	}
	err := s.repo.Clear(ctx)
	alertLogTamper(callerName, "登录日志", "清空", -1, err)
	return err
}

// auditGate 审计日志删除/清空的档位闸门（委托包级 auditGateFor，两 service 共享
// 单一实现——防双份接线漂移，审计 P1）。
func (s *LoginLogService) auditGate(ctx context.Context, callerId string) (string, error) {
	return auditGateFor(ctx, s.userRepo, callerId)
}

type OperLogService struct {
	repo     repository.IOperLogRepository
	userRepo repository.IUserRepository
}

func NewOperLogService(r repository.IOperLogRepository, u repository.IUserRepository) *OperLogService {
	return &OperLogService{repo: r, userRepo: u}
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
func (s *OperLogService) Delete(ctx context.Context, callerId string, ids []string) error {
	callerName, gateErr := s.auditGate(ctx, callerId)
	if gateErr != nil {
		alertLogTamper(callerName, "操作日志", "删除", len(ids), gateErr)
		return gateErr
	}
	err := s.repo.Delete(ctx, ids)
	alertLogTamper(callerName, "操作日志", "删除", len(ids), err)
	return err
}
func (s *OperLogService) Clear(ctx context.Context, callerId string) error {
	callerName, gateErr := s.auditGate(ctx, callerId)
	if gateErr != nil {
		alertLogTamper(callerName, "操作日志", "清空", -1, gateErr)
		return gateErr
	}
	err := s.repo.Clear(ctx)
	alertLogTamper(callerName, "操作日志", "清空", -1, err)
	return err
}

func (s *OperLogService) auditGate(ctx context.Context, callerId string) (string, error) {
	return auditGateFor(ctx, s.userRepo, callerId)
}

// auditGateFor 档位闸门唯一实现：查库身份（fail-closed）→ 纯函数判定。
// 返回 (操作者名, 错误)：错误非 nil=拒绝（身份不可得时绝不放开删除权）。
func auditGateFor(ctx context.Context, userRepo repository.IUserRepository, callerId string) (string, error) {
	callerName, isAdmin := "", false
	if callerId != "" && userRepo != nil {
		caller, err := userRepo.GetEntityById(ctx, callerId)
		callerName, isAdmin = resolveCaller(caller, err)
	}
	return callerName, auditProtectionCheck(config.Get().Audit.Protection, isAdmin)
}

// resolveCaller 查库结果→(操作者名, 是否超管) 的判定收口（变异 M3 补网）：
// caller 为 nil 或查库出错一律 ("", false)——身份不可得绝不视作超管，
// 与 auditProtectionCheck 的 flexible 档组合成完整 fail-closed 链。
func resolveCaller(caller *entity.User, err error) (string, bool) {
	if err != nil || caller == nil {
		return "", false
	}
	return caller.UserName, caller.IsAdmin == 1
}

// 档位闸门哨兵错误（controller 据此返回 HTTP 403，其余错误走通用 Fail）。
var (
	ErrAuditProtection = errors.New("审计日志保护已开启（strict 档）：登录/操作日志不允许删除或清空")
	ErrAuditNotAdmin   = errors.New("没有访问权限：只有超级管理员可以删除或清空审计日志（flexible 档）")
)

// auditProtectionCheck 审计日志删除/清空档位闸门的判定收口（表测五格钉住：
// strict×flexible×超管×非超管×调用者身份缺失）。strict=等保合规态一律拒绝；
// flexible=仅超管（删除权是补偿控制的核心，宁可错拒不可错放）；未知档位按 strict
// （config 归一已保证不出现，防御性兜底=配错只会更紧）。
func auditProtectionCheck(protection string, isCallerAdmin bool) error {
	switch protection {
	case "flexible":
		if !isCallerAdmin {
			return ErrAuditNotAdmin
		}
		return nil
	default:
		return ErrAuditProtection
	}
}

// alertLogTamper 日志篡改告警：落库必达（等保 8.1.4.3c），外送受 alert.enabled 与节流控制。
// 行数 -1 表示清空（全表）；操作者为空时如实标注「未知操作者」。repo 调用**之后**发出
// 并带结果——「尝试」语义=请求已受理，成败如实入正文（失败尝试对审计更有价值），
// 行数是提交数而非实际删除数（审计 B6：失败时不应留下既成事实的记录）。
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
