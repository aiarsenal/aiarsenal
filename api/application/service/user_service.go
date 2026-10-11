package service

import (
	"context"
	"errors"

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

func (s *UserService) Add(ctx context.Context, u *entity.User, password string, roleIds, postIds []string) error {
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

func (s *UserService) Edit(ctx context.Context, u *entity.User, roleIds, postIds []string) error {
	if err := s.repo.Edit(ctx, u, roleIds, postIds); err != nil {
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

func (s *UserService) ResetPwd(ctx context.Context, id string, password string) error {
	if password == "" {
		return errors.New("密码不能为空")
	}
	salt := persistence.GenerateSalt()
	return s.repo.ResetPwd(ctx, id, auth.EncryptPassword(password, salt), salt)
}

func (s *UserService) ChangeStatus(ctx context.Context, id string, status uint) error {
	return s.repo.ChangeStatus(ctx, id, status)
}

func (s *UserService) Delete(ctx context.Context, ids []string) error {
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
