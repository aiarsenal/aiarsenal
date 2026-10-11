package persistence

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type SkillNamespaceRepository struct{ db *gorm.DB }

func NewSkillNamespaceRepository() *SkillNamespaceRepository {
	return &SkillNamespaceRepository{db: db.SkillDB()}
}

func (r *SkillNamespaceRepository) List(ctx context.Context, slug, name, nsType, status string, pageNum, pageSize int) (list []*entity.SkillNamespace, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.SkillNamespace{})
	if slug != "" {
		q = q.Where("slug like ?", "%"+slug+"%")
	}
	if name != "" {
		q = q.Where("display_name like ?", "%"+name+"%")
	}
	if nsType != "" {
		q = q.Where("type = ?", nsType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		slog.Error("skill namespace 列表计数失败", "slug", slug, "name", name, "type", nsType, "status", status, "err", err)
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	// 过滤诊断：status 为前端传入字符串（""/"0"/"1"），日志可见筛选条件是否生效、结果数。
	slog.Debug("skill namespace 列表查询", "slug", slug, "name", name, "type", nsType, "status", status, "pageNum", pageNum, "pageSize", pageSize, "total", total, "returned", len(list), "err", err)
	return
}

func (r *SkillNamespaceRepository) All(ctx context.Context) (list []*entity.SkillNamespace, err error) {
	err = r.db.WithContext(ctx).Where("status = ?", 1).Order("id asc").Find(&list).Error
	return
}

func (r *SkillNamespaceRepository) Get(ctx context.Context, namespaceId string) (*entity.SkillNamespace, error) {
	var e entity.SkillNamespace
	if err := r.db.WithContext(ctx).Where("namespace_id = ?", namespaceId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillNamespaceRepository) GetBySlug(ctx context.Context, slug string) (*entity.SkillNamespace, error) {
	var e entity.SkillNamespace
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillNamespaceRepository) Add(ctx context.Context, n *entity.SkillNamespace) (string, error) {
	n.NamespaceId = idgen.New()
	if err := r.db.WithContext(ctx).Create(n).Error; err != nil {
		return "", err
	}
	return n.NamespaceId, nil
}

func (r *SkillNamespaceRepository) Edit(ctx context.Context, n *entity.SkillNamespace) error {
	return r.db.WithContext(ctx).Model(&entity.SkillNamespace{}).
		Where("namespace_id = ?", n.NamespaceId).Updates(n).Error
}

func (r *SkillNamespaceRepository) Delete(ctx context.Context, namespaceId string) error {
	return r.db.WithContext(ctx).Where("namespace_id = ?", namespaceId).
		Delete(&entity.SkillNamespace{}).Error
}

func (r *SkillNamespaceRepository) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	var list []*entity.SkillNamespaceMember
	err := r.db.WithContext(ctx).Where("namespace_id = ?", namespaceId).Order("id asc").Find(&list).Error
	return list, err
}

func (r *SkillNamespaceRepository) AddMember(ctx context.Context, m *entity.SkillNamespaceMember) error {
	m.MemberId = idgen.New()
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *SkillNamespaceRepository) RemoveMember(ctx context.Context, memberId string) error {
	return r.db.WithContext(ctx).Where("member_id = ?", memberId).Delete(&entity.SkillNamespaceMember{}).Error
}
