package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type DeptRepository struct{ db *gorm.DB }

func NewDeptRepository() *DeptRepository { return &DeptRepository{db: db.DB()} }

// List 部门列表，left join aa_user 带出负责人的姓名 / 工号（按 leader_id 关联）。
func (r *DeptRepository) List(ctx context.Context, name, status string) (list []*model.DeptInfo, err error) {
	q := r.db.WithContext(ctx).Table("aa_dept as d").
		Select("d.dept_id, d.parent_id, d.dept_name, d.order_num, d.leader_id, d.status, d.created_at, " +
			"u.name as leader_name, u.employee_no as leader_employee_no").
		Joins("left join aa_user u on u.user_id = d.leader_id and u.deleted_at is null").
		Where("d.deleted_at is null")
	if name != "" {
		q = q.Where("d.dept_name like ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("d.status = ?", status)
	}
	err = q.Order("d.order_num asc").Find(&list).Error
	return
}

func (r *DeptRepository) Get(ctx context.Context, deptId string) (*entity.Dept, error) {
	var d entity.Dept
	if err := r.db.WithContext(ctx).Where("dept_id = ?", deptId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeptRepository) Add(ctx context.Context, d *entity.Dept) error {
	d.DeptId = idgen.New()
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DeptRepository) Edit(ctx context.Context, d *entity.Dept) error {
	return r.db.WithContext(ctx).Model(&entity.Dept{}).Where("dept_id = ?", d.DeptId).Updates(d).Error
}

func (r *DeptRepository) Delete(ctx context.Context, deptId string) error {
	return r.db.WithContext(ctx).Where("dept_id = ?", deptId).Delete(&entity.Dept{}).Error
}

func (r *DeptRepository) HasChildren(ctx context.Context, parentId string) (bool, error) {
	var cnt int64
	if err := r.db.WithContext(ctx).Model(&entity.Dept{}).Where("parent_id = ?", parentId).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}
