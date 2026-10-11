package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type MenuRepository struct{ db *gorm.DB }

func NewMenuRepository() *MenuRepository { return &MenuRepository{db: db.DB()} }

// CountGrantRef 菜单授权条件行 JSON 文本中引用了指定 id（分组/集合 32 位业务 id）的菜单数。
// 分组/集合删除保护用——id 定长 32 字符且字符集固定，无子串误报。
func (r *MenuRepository) CountGrantRef(ctx context.Context, id string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.Menu{}).
		Where("grant_rules::text like ?", `%"`+id+`"%`).Count(&n).Error
	return n, err
}

// GetMenuList 所有规则（order weigh desc, id asc）
func (r *MenuRepository) GetMenuList(ctx context.Context) ([]*model.MenuInfoRes, error) {
	var rows []*entity.Menu
	if err := r.db.WithContext(ctx).Model(&entity.Menu{}).
		Order("weigh desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toMenuInfoList(rows), nil
}

func (r *MenuRepository) GetIsMenuList(ctx context.Context) ([]*model.MenuInfoRes, error) {
	all, err := r.GetMenuList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.MenuInfoRes, 0, len(all))
	for _, v := range all {
		if v.MenuType == 0 || v.MenuType == 1 {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *MenuRepository) GetIsButtonList(ctx context.Context) ([]*model.MenuInfoRes, error) {
	all, err := r.GetMenuList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.MenuInfoRes, 0, len(all))
	for _, v := range all {
		if v.MenuType == 2 {
			out = append(out, v)
		}
	}
	return out, nil
}

// SearchList 菜单列表搜索（按 title/component 过滤）
func (r *MenuRepository) SearchList(ctx context.Context, title, component string) ([]*model.MenuInfoRes, error) {
	q := r.db.WithContext(ctx).Model(&entity.Menu{})
	if title != "" {
		q = q.Where("title like ?", "%"+title+"%")
	}
	if component != "" {
		q = q.Where("component like ?", "%"+component+"%")
	}
	var rows []*entity.Menu
	if err := q.Order("weigh desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toMenuInfoList(rows), nil
}

func (r *MenuRepository) Get(ctx context.Context, menuId string) (*entity.Menu, error) {
	var e entity.Menu
	if err := r.db.WithContext(ctx).Where("menu_id = ?", menuId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *MenuRepository) Add(ctx context.Context, m *entity.Menu) (string, error) {
	m.MenuId = idgen.New()
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return "", err
	}
	return m.MenuId, nil
}

func (r *MenuRepository) Update(ctx context.Context, m *entity.Menu) error {
	return r.db.WithContext(ctx).Model(&entity.Menu{}).Where("menu_id = ?", m.MenuId).Updates(m).Error
}

// Delete 删除菜单及其所有子孙（递归，按 menu_id）
func (r *MenuRepository) Delete(ctx context.Context, menuIds []string) error {
	if len(menuIds) == 0 {
		return nil
	}
	all, err := r.GetMenuList(ctx)
	if err != nil {
		return err
	}
	toDelete := map[string]bool{}
	var collect func(pid string)
	collect = func(pid string) {
		for _, m := range all {
			if m.Pid == pid {
				toDelete[m.Id] = true
				collect(m.Id)
			}
		}
	}
	for _, id := range menuIds {
		toDelete[id] = true
		collect(id)
	}
	ids2 := make([]string, 0, len(toDelete))
	for id := range toDelete {
		ids2 = append(ids2, id)
	}
	// 删除菜单 + 同步删除 casbin 中以 menuId 为 obj 的策略
	e, cerr := auth.Enforcer()
	if cerr == nil {
		for _, id := range ids2 {
			_, _ = e.RemoveFilteredPolicy(1, id)
		}
	}
	return r.db.WithContext(ctx).Where("menu_id in ?", ids2).Delete(&entity.Menu{}).Error
}

// GetMenuRoleIds 菜单已关联的角色 id（casbin p 策略中 obj=menuId、v3='g' 的 sub；表达式行 v0='cond' 排除）
func (r *MenuRepository) GetMenuRoleIds(ctx context.Context, menuId string) ([]string, error) {
	e, err := auth.Enforcer()
	if err != nil {
		return nil, err
	}
	policies, perr := e.GetFilteredNamedPolicy("p", 1, menuId)
	if perr != nil {
		return nil, perr
	}
	out := make([]string, 0, len(policies))
	for _, p := range policies {
		if len(p) >= 4 && p[3] == "g" {
			out = append(out, p[0])
		}
	}
	return out, nil
}

// WriteGrantPolicies 重写菜单的组合授权表达式行（v0='cond'，v3=CompileExpr 产物）：
// 先按 v0='cond' 精确清空该菜单旧表达式行（v0 匹配排除角色直授行），再写新行。
// 角色直授行（v0=roleId，v3='g'）归角色管理页维护，此处不触碰。
func (r *MenuRepository) WriteGrantPolicies(ctx context.Context, menuId string, exprs []string) error {
	e, err := auth.Enforcer()
	if err != nil {
		return err
	}
	if _, err := e.RemoveFilteredPolicy(0, "cond", menuId, "All"); err != nil {
		return err
	}
	for _, expr := range exprs {
		if expr == "" {
			continue
		}
		if _, err := e.AddPolicy("cond", menuId, "All", expr); err != nil {
			return err
		}
	}
	return nil
}

func toMenuInfoList(rows []*entity.Menu) []*model.MenuInfoRes {
	out := make([]*model.MenuInfoRes, 0, len(rows))
	for _, e := range rows {
		out = append(out, &model.MenuInfoRes{
			Id: e.MenuId, Pid: e.Pid, Name: e.Name, Title: e.Title, EnName: e.EnName, Icon: e.Icon,
			Condition: e.Condition, Remark: e.Remark, MenuType: e.MenuType, Weigh: e.Weigh,
			IsHide: e.IsHide, IsCached: e.IsCached, IsAffix: e.IsAffix, Path: e.Path,
			Redirect: e.Redirect, Component: e.Component, IsIframe: e.IsIframe, IsLink: e.IsLink, LinkUrl: e.LinkUrl,
		})
	}
	return out
}
