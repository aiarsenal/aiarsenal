package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type DocDirRepository struct{ db *gorm.DB }

func NewDocDirRepository() *DocDirRepository { return &DocDirRepository{db: db.DocDB()} }

func (r *DocDirRepository) ListAll(ctx context.Context) ([]*entity.DocDir, error) {
	var list []*entity.DocDir
	err := r.db.WithContext(ctx).Order("id").Find(&list).Error
	return list, err
}

func (r *DocDirRepository) Get(ctx context.Context, dirId string) (*entity.DocDir, error) {
	var d entity.DocDir
	if err := r.db.WithContext(ctx).Where("dir_id = ?", dirId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DocDirRepository) GetByName(ctx context.Context, parentId, name string) (*entity.DocDir, error) {
	var d entity.DocDir
	if err := r.db.WithContext(ctx).Where("parent_id = ? and name = ?", parentId, name).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DocDirRepository) GetByKindRef(ctx context.Context, kind, refId string) (*entity.DocDir, error) {
	var d entity.DocDir
	if err := r.db.WithContext(ctx).Where("kind = ? and ref_id = ?", kind, refId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DocDirRepository) Add(ctx context.Context, d *entity.DocDir) error {
	if d.DirId == "" {
		d.DirId = idgen.New()
	}
	now := types.Time(time.Now())
	d.CreatedAt, d.UpdatedAt = &now, &now
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DocDirRepository) Rename(ctx context.Context, dirId, name string) error {
	return r.db.WithContext(ctx).Model(&entity.DocDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"name": name, "updated_at": gorm.Expr("now()")}).Error
}

// SetVisibility 目录可见性档位（0私有 1按条件 2全员）
func (r *DocDirRepository) SetVisibility(ctx context.Context, dirId string, visibility uint) error {
	return r.db.WithContext(ctx).Model(&entity.DocDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"visibility": visibility, "updated_at": gorm.Expr("now()")}).Error
}

func (r *DocDirRepository) Move(ctx context.Context, dirId, parentId string) error {
	return r.db.WithContext(ctx).Model(&entity.DocDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"parent_id": parentId, "updated_at": gorm.Expr("now()")}).Error
}

func (r *DocDirRepository) Delete(ctx context.Context, dirId string) error {
	return r.db.WithContext(ctx).Where("dir_id = ?", dirId).Delete(&entity.DocDir{}).Error
}

func (r *DocDirRepository) CountChildren(ctx context.Context, dirId string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.DocDir{}).
		Where("parent_id = ?", dirId).Count(&n).Error
	return n, err
}
