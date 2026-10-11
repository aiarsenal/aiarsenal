package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type PostRepository struct{ db *gorm.DB }

func NewPostRepository() *PostRepository { return &PostRepository{db: db.DB()} }

func (r *PostRepository) List(ctx context.Context, code, name, status string, pageNum, pageSize int) (list []*entity.Post, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.Post{})
	if code != "" {
		q = q.Where("post_code like ?", "%"+code+"%")
	}
	if name != "" {
		q = q.Where("post_name like ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
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
	err = q.Order("post_sort asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *PostRepository) Add(ctx context.Context, p *entity.Post) error {
	p.PostId = idgen.New()
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PostRepository) Edit(ctx context.Context, p *entity.Post) error {
	return r.db.WithContext(ctx).Model(&entity.Post{}).Where("post_id = ?", p.PostId).Updates(p).Error
}

func (r *PostRepository) Delete(ctx context.Context, postIds []string) error {
	if len(postIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("post_id in ?", postIds).Delete(&entity.Post{}).Error
}

// HoldsTopPost 最高职级判定：用户启用岗位里含全公司 post_sort 最小的那条（排序即职级，越小越高）。
// 任一岗位表为空/读取失败返回 false（判不准就照常走人工审批，宁可多审不可漏审）。
func (r *PostRepository) HoldsTopPost(ctx context.Context, userId string) bool {
	if userId == "" {
		return false
	}
	var ok bool
	err := r.db.WithContext(ctx).Raw(`
		select exists(
			select 1 from aa_user_post up
			join aa_post p on p.post_id = up.post_id and p.status = 1
			where up.user_id = ?
			  and p.post_sort = (select min(post_sort) from aa_post where status = 1)
		)`, userId).Scan(&ok).Error
	return err == nil && ok
}
