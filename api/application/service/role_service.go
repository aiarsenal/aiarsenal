package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/cache"
)

type RoleService struct {
	repo     repository.IRoleRepository
	menuRepo repository.IMenuRepository
	cache    *cache.Cache
}

func NewRoleService(r repository.IRoleRepository, a repository.IMenuRepository, c *cache.Cache) *RoleService {
	return &RoleService{repo: r, menuRepo: a, cache: c}
}

func (s *RoleService) List(ctx context.Context, name, status string, pageNum, pageSize int) ([]*entity.Role, int64, error) {
	return s.repo.List(ctx, name, status, pageNum, pageSize)
}
func (s *RoleService) All(ctx context.Context) ([]*entity.Role, error) { return s.repo.All(ctx) }

// GetParams 角色编辑参数：菜单列表
func (s *RoleService) GetParams(ctx context.Context) ([]*model.MenuInfoRes, error) {
	return s.menuRepo.GetIsMenuList(ctx)
}

func (s *RoleService) Get(ctx context.Context, id string) (*entity.Role, []string, error) {
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	menuIds, _ := s.repo.GetMenuIds(ctx, id)
	return r, menuIds, nil
}

func (s *RoleService) Add(ctx context.Context, r *entity.Role, menuIds []string) error {
	if r.Name == "" {
		return errors.New("角色名称不能为空")
	}
	rid, err := s.repo.Add(ctx, r)
	if err != nil {
		return err
	}
	if len(menuIds) > 0 {
		if err := s.repo.BindMenus(ctx, rid, menuIds); err != nil {
			return err
		}
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysRole")
	return nil
}

func (s *RoleService) Edit(ctx context.Context, r *entity.Role, menuIds []string) error {
	if err := s.repo.Edit(ctx, r); err != nil {
		return err
	}
	if err := s.repo.ClearMenus(ctx, r.RoleId); err != nil {
		return err
	}
	if len(menuIds) > 0 {
		if err := s.repo.BindMenus(ctx, r.RoleId, menuIds); err != nil {
			return err
		}
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysRole")
	return nil
}

func (s *RoleService) Delete(ctx context.Context, roleIds []string) error {
	if err := s.repo.Delete(ctx, roleIds); err != nil {
		return err
	}
	for _, id := range roleIds {
		_ = s.repo.ClearMenus(ctx, id)
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysRole")
	return nil
}
