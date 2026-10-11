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

type KbDirRepository struct{ db *gorm.DB }

func NewKbDirRepository() *KbDirRepository { return &KbDirRepository{db: db.KbDB()} }

func (r *KbDirRepository) ListAll(ctx context.Context) ([]*entity.KbDir, error) {
	var list []*entity.KbDir
	err := r.db.WithContext(ctx).Order("id").Find(&list).Error
	return list, err
}

func (r *KbDirRepository) Get(ctx context.Context, dirId string) (*entity.KbDir, error) {
	var d entity.KbDir
	if err := r.db.WithContext(ctx).Where("dir_id = ?", dirId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *KbDirRepository) GetByName(ctx context.Context, parentId, name string) (*entity.KbDir, error) {
	var d entity.KbDir
	if err := r.db.WithContext(ctx).Where("parent_id = ? and name = ?", parentId, name).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *KbDirRepository) Add(ctx context.Context, d *entity.KbDir) error {
	if d.DirId == "" {
		d.DirId = idgen.New()
	}
	now := types.Time(time.Now())
	d.CreatedAt, d.UpdatedAt = &now, &now
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *KbDirRepository) Rename(ctx context.Context, dirId, name string) error {
	return r.db.WithContext(ctx).Model(&entity.KbDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"name": name, "updated_at": gorm.Expr("now()")}).Error
}

// SetVisibility 目录可见性档位（0私有 1按条件 2全员）
func (r *KbDirRepository) SetVisibility(ctx context.Context, dirId string, visibility uint) error {
	return r.db.WithContext(ctx).Model(&entity.KbDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"visibility": visibility, "updated_at": gorm.Expr("now()")}).Error
}

func (r *KbDirRepository) Move(ctx context.Context, dirId, parentId string) error {
	return r.db.WithContext(ctx).Model(&entity.KbDir{}).
		Where("dir_id = ?", dirId).
		Updates(map[string]interface{}{"parent_id": parentId, "updated_at": gorm.Expr("now()")}).Error
}

func (r *KbDirRepository) Delete(ctx context.Context, dirId string) error {
	return r.db.WithContext(ctx).Where("dir_id = ?", dirId).Delete(&entity.KbDir{}).Error
}

func (r *KbDirRepository) CountChildren(ctx context.Context, dirId string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.KbDir{}).
		Where("parent_id = ?", dirId).Count(&n).Error
	return n, err
}
