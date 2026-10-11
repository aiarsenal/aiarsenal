package persistence

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type RoleRepository struct{ db *gorm.DB }

func NewRoleRepository() *RoleRepository { return &RoleRepository{db: db.DB()} }

func (r *RoleRepository) List(ctx context.Context, name, status string, pageNum, pageSize int) (list []*entity.Role, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.Role{})
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		slog.Error("role 列表计数失败", "name", name, "status", status, "err", err)
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Order("list_order asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	// 过滤诊断：status 为前端传入的字符串（""/"0"/"1"），日志可见筛选条件是否生效、结果数。
	slog.Debug("role 列表查询", "name", name, "status", status, "pageNum", pageNum, "pageSize", pageSize, "total", total, "returned", len(list), "err", err)
	return
}

func (r *RoleRepository) All(ctx context.Context) (list []*entity.Role, err error) {
	err = r.db.WithContext(ctx).Order("id asc").Find(&list).Error
	return
}

func (r *RoleRepository) Get(ctx context.Context, roleId string) (*entity.Role, error) {
	var e entity.Role
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *RoleRepository) Add(ctx context.Context, role *entity.Role) (string, error) {
	role.RoleId = idgen.New()
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return "", err
	}
	return role.RoleId, nil
}

func (r *RoleRepository) Edit(ctx context.Context, role *entity.Role) error {
	return r.db.WithContext(ctx).Model(&entity.Role{}).Where("role_id = ?", role.RoleId).
		Updates(map[string]interface{}{
			"name": role.Name, "status": role.Status, "list_order": role.ListOrder, "remark": role.Remark,
		}).Error
}

func (r *RoleRepository) Delete(ctx context.Context, roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("role_id in ?", roleIds).Delete(&entity.Role{}).Error
}

// --- casbin 角色-菜单策略 ---

func (r *RoleRepository) BindMenus(ctx context.Context, roleId string, menuIds []string) error {
	e, err := auth.Enforcer()
	if err != nil {
		return err
	}
	for _, mid := range menuIds {
		if _, err := e.AddPolicy(roleId, mid, "All", "g"); err != nil {
			return err
		}
	}
	return nil
}

func (r *RoleRepository) ClearMenus(ctx context.Context, roleId string) error {
	e, err := auth.Enforcer()
	if err != nil {
		return err
	}
	_, err = e.RemoveFilteredPolicy(0, roleId)
	return err
}

func (r *RoleRepository) GetMenuIds(ctx context.Context, roleId string) ([]string, error) {
	e, err := auth.Enforcer()
	if err != nil {
		return nil, err
	}
	gp, gerr := e.GetFilteredPolicy(0, roleId)
	if gerr != nil {
		return nil, gerr
	}
	out := make([]string, 0, len(gp))
	for _, v := range gp {
		if len(v) >= 2 {
			out = append(out, v[1])
		}
	}
	return out, nil
}
