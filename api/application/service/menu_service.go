package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/cache"
)

type MenuService struct {
	repo  repository.IMenuRepository
	cache *cache.Cache
}

func NewMenuService(r repository.IMenuRepository, c *cache.Cache) *MenuService {
	return &MenuService{repo: r, cache: c}
}

// Search 菜单列表（flat）
func (s *MenuService) Search(ctx context.Context, title, component string) ([]*model.MenuInfoRes, error) {
	return s.repo.SearchList(ctx, title, component)
}

// GetParams 添加/编辑菜单参数：角色列表 + 菜单列表（由 controller 组装，service 只暴露 repo）
func (s *MenuService) GetIsMenuList(ctx context.Context) ([]*model.MenuInfoRes, error) {
	return s.repo.GetIsMenuList(ctx)
}

func (s *MenuService) Get(ctx context.Context, id string) (*entity.Menu, []string, error) {
	rule, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	roleIds, _ := s.repo.GetMenuRoleIds(ctx, id)
	return rule, roleIds, nil
}

// Add 新增菜单；grantJSON 为组合授权条件行（[]model.GrantRow 的 JSON，可空），
// 入库前统一校验并规范序列化到 grant_rules，条件编译为 casbin 表达式行。
// 角色直授行不在此管理（统一走角色管理页），菜单侧只写表达式行。
func (s *MenuService) Add(ctx context.Context, m *entity.Menu, grantJSON string) error {
	if m.Name == "" {
		return errors.New("请填写规则名称")
	}
	rows, err := model.ParseGrantRows(grantJSON)
	if err != nil {
		return err
	}
	m.GrantRules = model.GrantRowsJSON(rows)
	menuId, err := s.repo.Add(ctx, m)
	if err != nil {
		return err
	}
	if err := s.repo.WriteGrantPolicies(ctx, menuId, model.GrantExprs(rows)); err != nil {
		return err
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysAuthMenu")
	return nil
}

// Update 编辑菜单；表达式行由 WriteGrantPolicies 自清自写（按 v0='cond' 精确匹配），
// 角色直授行归角色管理页维护，菜单保存不触碰——两边互不干扰。
func (s *MenuService) Update(ctx context.Context, m *entity.Menu, grantJSON string) error {
	rows, err := model.ParseGrantRows(grantJSON)
	if err != nil {
		return err
	}
	m.GrantRules = model.GrantRowsJSON(rows)
	if err := s.repo.Update(ctx, m); err != nil {
		return err
	}
	if err := s.repo.WriteGrantPolicies(ctx, m.MenuId, model.GrantExprs(rows)); err != nil {
		return err
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysAuthMenu")
	return nil
}

func (s *MenuService) Delete(ctx context.Context, ids []string) error {
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	s.cache.Del(ctx, "aiArsenalV3Cache:sysAuthMenu")
	return nil
}

// BuildTree 菜单树（pid="0" 起，嵌套 children）
func (s *MenuService) BuildTree(list []*model.MenuInfoRes) []*model.MenuTreeRes {
	return buildMenuInfoTree(list, "0")
}

func buildMenuInfoTree(list []*model.MenuInfoRes, pid string) []*model.MenuTreeRes {
	out := make([]*model.MenuTreeRes, 0)
	for _, m := range list {
		if m.Pid == pid {
			t := &model.MenuTreeRes{MenuInfoRes: m}
			t.Children = buildMenuInfoTree(list, m.Id)
			out = append(out, t)
		}
	}
	return out
}
