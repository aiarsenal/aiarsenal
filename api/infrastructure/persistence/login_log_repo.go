package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
)

type LoginLogRepository struct{ db *gorm.DB }

func NewLoginLogRepository() *LoginLogRepository { return &LoginLogRepository{db: db.DB()} }

func (r *LoginLogRepository) List(ctx context.Context, loginName, status, ip, location string, pageNum, pageSize int) (list []*entity.LoginLog, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LoginLog{})
	if loginName != "" {
		q = q.Where("login_name like ?", "%"+loginName+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if ip != "" {
		q = q.Where("ipaddr like ?", "%"+ip+"%")
	}
	if location != "" {
		q = q.Where("login_location like ?", "%"+location+"%")
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

func (r *LoginLogRepository) Delete(ctx context.Context, infoIds []string) error {
	if len(infoIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("info_id in ?", infoIds).Delete(&entity.LoginLog{}).Error
}

func (r *LoginLogRepository) Clear(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("1=1").Delete(&entity.LoginLog{}).Error
}
