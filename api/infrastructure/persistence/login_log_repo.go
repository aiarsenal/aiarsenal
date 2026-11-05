package persistence

import (
	"context"
	"time"

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

// PurgeBefore 留存清理专用：物理删除 login_time 早于 cutoff 的行（PR③ 留存任务）。
func (r *LoginLogRepository) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	q := r.db.WithContext(ctx).Where("login_time < ?", cutoff).Delete(&entity.LoginLog{})
	return q.RowsAffected, q.Error
}
