package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
)

type UserOnlineRepository struct{ db *gorm.DB }

func NewUserOnlineRepository() *UserOnlineRepository { return &UserOnlineRepository{db: db.DB()} }

func (r *UserOnlineRepository) List(ctx context.Context, username, ip string, pageNum, pageSize int) (list []*entity.UserOnline, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.UserOnline{})
	if username != "" {
		q = q.Where("user_name like ?", "%"+username+"%")
	}
	if ip != "" {
		q = q.Where("ip like ?", "%"+ip+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *UserOnlineRepository) Delete(ctx context.Context, uuid string) error {
	return r.db.WithContext(ctx).Where("uuid = ?", uuid).Delete(&entity.UserOnline{}).Error
}
