package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type PermGroupRepository struct{ db *gorm.DB }

func NewPermGroupRepository() *PermGroupRepository { return &PermGroupRepository{db: db.DB()} }

func (r *PermGroupRepository) List(ctx context.Context, name string) ([]*entity.PermGroup, error) {
	q := r.db.WithContext(ctx).Model(&entity.PermGroup{})
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	var list []*entity.PermGroup
	err := q.Order("id desc").Find(&list).Error
	return list, err
}

func (r *PermGroupRepository) GetById(ctx context.Context, groupId string) (*entity.PermGroup, error) {
	var g entity.PermGroup
	if err := r.db.WithContext(ctx).Where("group_id = ?", groupId).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// NameExists 名称精确重名检查（excludeId 非空时排除自身，编辑场景）
func (r *PermGroupRepository) NameExists(ctx context.Context, name, excludeId string) (bool, error) {
	var n int64
	q := r.db.WithContext(ctx).Model(&entity.PermGroup{}).Where("name = ?", name)
	if excludeId != "" {
		q = q.Where("group_id <> ?", excludeId)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (r *PermGroupRepository) TermsLike(ctx context.Context, id string) ([]*entity.PermGroup, error) {
	var list []*entity.PermGroup
	err := r.db.WithContext(ctx).Where("terms::text like ?", `%"`+id+`"%`).Find(&list).Error
	return list, err
}

func (r *PermGroupRepository) Add(ctx context.Context, g *entity.PermGroup) (string, error) {
	g.GroupId = idgen.New()
	return g.GroupId, r.db.WithContext(ctx).Create(g).Error
}

func (r *PermGroupRepository) Update(ctx context.Context, g *entity.PermGroup) error {
	return r.db.WithContext(ctx).Model(&entity.PermGroup{}).
		Where("group_id = ?", g.GroupId).
		Updates(map[string]interface{}{
			"name":       g.Name,
			"remark":     g.Remark,
			"terms":      g.Terms,
			"updated_at": gorm.Expr("now()"),
		}).Error
}

func (r *PermGroupRepository) Delete(ctx context.Context, groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("group_id in ?", groupIds).Delete(&entity.PermGroup{}).Error
}

type EmpSetRepository struct {
	db     *gorm.DB
	groups *PermGroupRepository // 引用扫描（删除保护）
}

func NewEmpSetRepository() *EmpSetRepository {
	return &EmpSetRepository{db: db.DB(), groups: &PermGroupRepository{db: db.DB()}}
}

func (r *EmpSetRepository) List(ctx context.Context, name string) ([]*entity.EmpSet, error) {
	q := r.db.WithContext(ctx).Model(&entity.EmpSet{})
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	var list []*entity.EmpSet
	err := q.Order("id desc").Find(&list).Error
	return list, err
}

func (r *EmpSetRepository) GetById(ctx context.Context, setId string) (*entity.EmpSet, error) {
	var s entity.EmpSet
	if err := r.db.WithContext(ctx).Where("set_id = ?", setId).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// NameExists 名称精确重名检查（excludeId 非空时排除自身，编辑场景）
func (r *EmpSetRepository) NameExists(ctx context.Context, name, excludeId string) (bool, error) {
	var n int64
	q := r.db.WithContext(ctx).Model(&entity.EmpSet{}).Where("name = ?", name)
	if excludeId != "" {
		q = q.Where("set_id <> ?", excludeId)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (r *EmpSetRepository) TermsLike(ctx context.Context, id string) ([]*entity.PermGroup, error) {
	return r.groups.TermsLike(ctx, id)
}

func (r *EmpSetRepository) Add(ctx context.Context, s *entity.EmpSet) (string, error) {
	s.SetId = idgen.New()
	return s.SetId, r.db.WithContext(ctx).Create(s).Error
}

func (r *EmpSetRepository) Update(ctx context.Context, s *entity.EmpSet) error {
	return r.db.WithContext(ctx).Model(&entity.EmpSet{}).
		Where("set_id = ?", s.SetId).
		Updates(map[string]interface{}{
			"name":       s.Name,
			"remark":     s.Remark,
			"emp_nos":    s.EmpNos,
			"updated_at": gorm.Expr("now()"),
		}).Error
}

func (r *EmpSetRepository) Delete(ctx context.Context, setIds []string) error {
	if len(setIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("set_id in ?", setIds).Delete(&entity.EmpSet{}).Error
}
