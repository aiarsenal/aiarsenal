package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// IMenuRepository 菜单/规则数据访问（aa_menu）
type IMenuRepository interface {
	// GetMenuList 所有启用的规则（供 Auth 中间件做 URL→menuId 映射）
	GetMenuList(ctx context.Context) ([]*model.MenuInfoRes, error)
	// GetIsMenuList 所有启用菜单（menu_type 0/1，is_hide=0）
	GetIsMenuList(ctx context.Context) ([]*model.MenuInfoRes, error)
	// GetIsButtonList 所有启用按钮（menu_type=2）
	GetIsButtonList(ctx context.Context) ([]*model.MenuInfoRes, error)

	// 菜单管理 CRUD
	SearchList(ctx context.Context, title, component string) ([]*model.MenuInfoRes, error)
	Get(ctx context.Context, menuId string) (*entity.Menu, error)
	Add(ctx context.Context, r *entity.Menu) (string, error)
	Update(ctx context.Context, r *entity.Menu) error
	Delete(ctx context.Context, menuIds []string) error
	// 菜单关联的角色 id（详情回显用；角色行归角色管理页维护）
	GetMenuRoleIds(ctx context.Context, menuId string) ([]string, error)
	// 重写组合授权表达式行（casbin: cond, menuId, All, expr；自清自写，不碰角色直授行）
	WriteGrantPolicies(ctx context.Context, menuId string, exprs []string) error
	// 授权条件行 JSON 引用扫描（分组/集合删除保护用）
	CountGrantRef(ctx context.Context, id string) (int64, error)
}
