package service

import (
	"context"
	"errors"
	"log/slog"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/persistence"
)

type UserService struct {
	repo     repository.IUserRepository
	roleRepo repository.IRoleRepository
	postRepo repository.IPostRepository
	docSpace *DocSpaceService // 个人空间目录同步（延迟接线；仅 api 进程）
	kbSpace  *KbSpaceService  // KB 个人空间目录同步（延迟接线，与文档空间同构）
}

func NewUserService(u repository.IUserRepository, r repository.IRoleRepository, p repository.IPostRepository) *UserService {
	return &UserService{repo: u, roleRepo: r, postRepo: p}
}

// SetDocSpaceSync 接入个人空间同步器（api 装配期；ops 不构建 UserService）
func (s *UserService) SetDocSpaceSync(d *DocSpaceService) { s.docSpace = d }

// SetKbSpaceSync 接入 KB 个人空间同步器（api 装配期）
func (s *UserService) SetKbSpaceSync(x *KbSpaceService) { s.kbSpace = x }

// syncKbSpace KB 个人空间目录同步（nil=未接线静默跳过）
func (s *UserService) syncKbSpace(ctx context.Context, userID string) {
	if s.kbSpace != nil {
		s.kbSpace.SyncUser(ctx, userID)
	}
}

// SearchOptions 人员选择器搜索（姓名/工号/手机/邮箱 OR）。
func (s *UserService) SearchOptions(ctx context.Context, keyWords string, pageNum, pageSize int) ([]*model.UserOptionRes, int64, error) {
	return s.repo.SearchOptions(ctx, keyWords, pageNum, pageSize)
}

// ResolveOptions 按 id 批量解析（授权条件行已选用户回显）
func (s *UserService) ResolveOptions(ctx context.Context, ids []string) ([]*model.UserOptionRes, error) {
	return s.repo.GetUserOptionsByIds(ctx, ids)
}

func (s *UserService) List(ctx context.Context, deptId, mobile, status, keyWords string, pageNum, pageSize int) ([]*model.UserRoleDeptRes, int64, error) {
	users, total, err := s.repo.List(ctx, deptId, mobile, status, keyWords, pageNum, pageSize)
	if err != nil {
		return nil, 0, err
	}
	enriched, err := s.repo.EnrichUsers(ctx, users)
	if err != nil {
		return nil, 0, err
	}
	return enriched, total, nil
}

// GetParams 用户维护参数：角色列表 + 岗位列表
func (s *UserService) GetParams(ctx context.Context) (roles []*entity.Role, posts []*entity.Post, err error) {
	roles, err = s.roleRepo.All(ctx)
	if err != nil {
		return
	}
	posts, _, err = s.postRepo.List(ctx, "", "", "", 1, 1000)
	return
}

// loadCaller 读取操作者的库内记录。
//
// 特权判定一律以库内为准而不是 token claims：claims 有效期长，管理员被降级后旧令牌
// 仍带着旧标志，闸门若信 claims，降权就不生效。操作者查无记录时按无权限拒绝——
// 闸门的零值必须是拒绝。
func (s *UserService) loadCaller(ctx context.Context, callerId string) (*entity.User, error) {
	caller, err := s.repo.GetEntityById(ctx, callerId)
	if err != nil || caller == nil {
		slog.Error("校验操作者身份失败", "operatorId", callerId, "err", err)
		return nil, errors.New("没有访问权限")
	}
	return caller, nil
}

// 用户维护接口的授权模型（分层细化，与 README「权限说明」一节对应）：
//
//	R1 isAdmin（超管标志）仅超管可写——"管理员不能设置超管"靠这条保证；
//	R2 任何人不得借编辑接口修改自己的 isAdmin/角色/岗位——普通档案字段不受限，
//	   防的是把 roleIds 换成更高权限角色这类自提权（编辑表单会原样回传角色/岗位，
//	   所以"改别人"不拦、"改自己"必须与现状一致）；
//	R3 目标保护：目标是超管时仅超管可编辑/停用/删除；
//	R4 角色/岗位分配：持有用户编辑菜单权限的人可以对普通用户分配——这层能力
//	   归菜单授权管，服务层不再额外收紧（自提权与越权由 R1/R2/R3 兜住）。

// sameStringSet 判断两个字符串切片是否元素相同（顺序、重复无关）。
// nil 与空切片视为相同——前端没勾选时两种形态都可能出现。
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[string]int, len(a))
	for _, v := range a {
		count[v]++
	}
	for _, v := range b {
		if count[v] == 0 {
			return false
		}
		count[v]--
	}
	return true
}

func (s *UserService) Add(ctx context.Context, u *entity.User, password string, roleIds, postIds []string, callerId string) error {
	caller, err := s.loadCaller(ctx, callerId)
	if err != nil {
		return err
	}
	// 新建账号缺省是普通用户；isAdmin 只能由超管写入（R1）。
	if u.IsAdmin != 0 && caller.IsAdmin != 1 {
		return errors.New("没有访问权限：只有超级管理员可以设置管理员标识")
	}
	if u.UserName == "" {
		return errors.New("用户账号不能为空")
	}
	if password == "" {
		return errors.New("密码不能为空")
	}
	// 用户名/手机号唯一
	existing, _ := s.repo.GetUserByUsername(ctx, u.UserName)
	if existing != nil {
		return errors.New("用户名已存在")
	}
	u.UserSalt = persistence.GenerateSalt()
	u.UserPassword = auth.EncryptPassword(password, u.UserSalt)
	uid, err := s.repo.Add(ctx, u, roleIds, postIds)
	if err != nil {
		return err
	}
	// 个人空间目录（aa_doc / aa_kb 库；best-effort，失败下次对账自愈）
	if s.docSpace != nil {
		s.docSpace.SyncUser(ctx, uid)
	}
	s.syncKbSpace(ctx, uid)
	return nil
}

// Edit 编辑用户档案。
//
// setAdmin 是 isAdmin 的指针语义：nil=请求体没带该字段（UI 回填白名单不含它），
// 仓储不写该列——绝不能把缺省当 0 落库，否则超管在界面上改任何管理员资料都会
// 把 is_admin 静默写成 0（且基线只有一个超管账号，界面授不回来）。非 nil 才按
// 值落库：超管收权必须显式传 0，收回管理员因此仍然可达（修 GORM 零值跳过）。
func (s *UserService) Edit(ctx context.Context, u *entity.User, roleIds, postIds []string, setAdmin *int, callerId string) error {
	caller, err := s.loadCaller(ctx, callerId)
	if err != nil {
		return err
	}
	if caller.IsAdmin != 1 {
		target, curRoles, curPosts, err := s.repo.GetEditUser(ctx, u.UserId)
		if err != nil {
			// 不把存储层原文（"record not found" 之类）透给调用方，只留日志。
			slog.Error("校验用户角色/岗位现状失败", "userId", u.UserId, "err", err)
			return errors.New("用户不存在或已被删除")
		}
		if target.IsAdmin == 1 { // R3 目标保护
			return errors.New("没有访问权限：只有超级管理员可以操作管理员账号")
		}
		if setAdmin != nil && *setAdmin != target.IsAdmin { // R1：非超管显式改 isAdmin
			return errors.New("没有访问权限：只有超级管理员可以设置管理员标识")
		}
		if u.UserId == callerId && // R2 自助保护：自己的角色/岗位不得借编辑接口改动
			(!sameStringSet(curRoles, roleIds) || !sameStringSet(curPosts, postIds)) {
			return errors.New("没有访问权限：不能修改自己的角色与岗位")
		}
	}
	if err := s.repo.Edit(ctx, u, roleIds, postIds, setAdmin); err != nil {
		return err
	}
	// 姓名/账号变化同步个人目录名（名字未变则 no-op；禁用走 ChangeStatus 不挂钩）
	if s.docSpace != nil {
		s.docSpace.SyncUser(ctx, u.UserId)
	}
	s.syncKbSpace(ctx, u.UserId)
	return nil
}

func (s *UserService) GetEditUser(ctx context.Context, id string) (*entity.User, []string, []string, error) {
	return s.repo.GetEditUser(ctx, id)
}

// ResetPwd 重置目标账号口令。
//
// operatorId 来自登录态（控制器传 middleware.GetUserId），不取自请求体。能否重置
// 他人密码以库里的 is_admin 为准而不是 token claims：claims 有效期长，管理员被降级
// 后旧令牌仍带着旧标志（该缺口另行处理）。操作者查无记录时按无权限拒绝——闸门的
// 零值必须是拒绝，不能把"读不到操作者"当放行。
// 审计留痕由 OperLog 中间件覆盖（PUT 非 GET、不在跳过清单），不另起写通道。
func (s *UserService) ResetPwd(ctx context.Context, operatorId, id, password string) error {
	if password == "" {
		return errors.New("密码不能为空")
	}
	op, err := s.repo.GetEntityById(ctx, operatorId)
	if err != nil || op == nil {
		slog.Error("校验口令重置操作者失败", "operatorId", operatorId, "err", err)
		return errors.New("没有访问权限：只能重置本人密码")
	}
	if op.IsAdmin != 1 && operatorId != id {
		return errors.New("没有访问权限：只能重置本人密码")
	}
	salt := persistence.GenerateSalt()
	return s.repo.ResetPwd(ctx, id, auth.EncryptPassword(password, salt), salt)
}

func (s *UserService) ChangeStatus(ctx context.Context, callerId, id string, status uint) error {
	caller, err := s.loadCaller(ctx, callerId)
	if err != nil {
		return err
	}
	if caller.IsAdmin != 1 {
		target, err := s.repo.GetEntityById(ctx, id)
		if err != nil || target == nil {
			return errors.New("用户不存在或已被删除")
		}
		if target.IsAdmin == 1 { // R3 目标保护
			return errors.New("没有访问权限：只有超级管理员可以操作管理员账号")
		}
	}
	return s.repo.ChangeStatus(ctx, id, status)
}

func (s *UserService) Delete(ctx context.Context, callerId string, ids []string) error {
	caller, err := s.loadCaller(ctx, callerId)
	if err != nil {
		return err
	}
	if caller.IsAdmin != 1 {
		for _, id := range ids {
			target, err := s.repo.GetEntityById(ctx, id)
			if err != nil || target == nil {
				return errors.New("用户不存在或已被删除")
			}
			if target.IsAdmin == 1 { // R3 目标保护
				return errors.New("没有访问权限：只有超级管理员可以操作管理员账号")
			}
		}
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	// 硬删后个人目录保留，名字追加（已注销）
	if s.docSpace != nil {
		for _, id := range ids {
			s.docSpace.SyncUserDeleted(ctx, id)
		}
	}
	if s.kbSpace != nil {
		for _, id := range ids {
			s.kbSpace.SyncUserDeleted(ctx, id)
		}
	}
	return nil
}

func (s *UserService) GetUsersByIds(ctx context.Context, ids []string) ([]*model.UserSimpleRes, error) {
	return s.repo.GetUsersByIds(ctx, ids)
}
