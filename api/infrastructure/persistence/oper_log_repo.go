package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type OperLogRepository struct{ db *gorm.DB }

func NewOperLogRepository() *OperLogRepository { return &OperLogRepository{db: db.DB()} }

func (r *OperLogRepository) List(ctx context.Context, title, method, operName string, pageNum, pageSize int) (list []*model.OperLogListRes, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.OperLog{})
	if title != "" {
		q = q.Where("title like ?", "%"+title+"%")
	}
	if method != "" {
		q = q.Where("request_method = ?", method)
	}
	if operName != "" {
		q = q.Where("oper_name like ?", "%"+operName+"%")
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
	var rows []*entity.OperLog
	if err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return
	}
	list = make([]*model.OperLogListRes, 0, len(rows))
	for _, e := range rows {
		list = append(list, &model.OperLogListRes{
			OperId: e.OperId, Title: e.Title, RequestMethod: e.RequestMethod,
			OperName: e.OperName, DeptName: e.DeptName, OperUrl: e.OperUrl,
			OperIp: e.OperIp, OperLocation: e.OperLocation, OperParam: e.OperParam,
			OperTime: operTimeStr(e.OperTime),
		})
	}
	return
}

func (r *OperLogRepository) Get(ctx context.Context, operId string) (*model.OperLogInfoRes, error) {
	var e entity.OperLog
	if err := r.db.WithContext(ctx).Where("oper_id = ?", operId).First(&e).Error; err != nil {
		return nil, err
	}
	return &model.OperLogInfoRes{
		OperId: e.OperId, Title: e.Title, BusinessType: e.BusinessType, Method: e.Method,
		RequestMethod: e.RequestMethod, OperatorType: e.OperatorType, OperName: e.OperName,
		DeptName: e.DeptName, OperUrl: e.OperUrl, OperIp: e.OperIp, OperLocation: e.OperLocation,
		OperParam: e.OperParam, ErrorMsg: e.ErrorMsg, OperTime: operTimeStr(e.OperTime),
	}, nil
}

func (r *OperLogRepository) Insert(ctx context.Context, log *entity.OperLog) error {
	log.OperId = idgen.New()
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *OperLogRepository) Delete(ctx context.Context, operIds []string) error {
	if len(operIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("oper_id in ?", operIds).Delete(&entity.OperLog{}).Error
}

func (r *OperLogRepository) Clear(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("1=1").Delete(&entity.OperLog{}).Error
}

func operTimeStr(t *types.Time) string {
	if t == nil {
		return ""
	}
	return timeToStr(*t)
}
