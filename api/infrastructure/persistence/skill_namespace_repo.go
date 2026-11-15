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

func (r *SkillNamespaceRepository) List(ctx context.Context, slug, name, nsType, status string, pageNum, pageSize int, viewerId string, viewerAdmin bool, memberNs []string) (list []*entity.SkillNamespace, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.SkillNamespace{})
	// F-017b A-03 收口：非超管只见 GLOBAL 命名空间 / 自己创建 / 所在成员命名空间。
	// 单括号 Where（审计 B-01 教训：q.Or 会平铺破坏后续筛选链）。
	if !viewerAdmin {
		if viewerId == "" {
			q = q.Where("type = 'GLOBAL'")
		} else if len(memberNs) > 0 {
			q = q.Where("(type = 'GLOBAL' OR created_by = ? OR namespace_id IN ?)", viewerId, memberNs)
		} else {
			q = q.Where("(type = 'GLOBAL' OR created_by = ?)", viewerId)
		}
	}
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

// All 系统路径全量（ops 工具等平台通道用）；用户面走 AllForCaller。
func (r *SkillNamespaceRepository) All(ctx context.Context) (list []*entity.SkillNamespace, err error) {
	err = r.db.WithContext(ctx).Where("status = ?", 1).Order("id asc").Find(&list).Error
	return
}

// AllForCaller 用户面 /skill/namespace/all（F-017b A-03）：同 List 的可见性 scope。
func (r *SkillNamespaceRepository) AllForCaller(ctx context.Context, viewerId string, viewerAdmin bool, memberNs []string) (list []*entity.SkillNamespace, err error) {
	q := r.db.WithContext(ctx).Where("status = ?", 1)
	if !viewerAdmin {
		if viewerId == "" {
			q = q.Where("type = 'GLOBAL'")
		} else if len(memberNs) > 0 {
			q = q.Where("(type = 'GLOBAL' OR created_by = ? OR namespace_id IN ?)", viewerId, memberNs)
		} else {
			q = q.Where("(type = 'GLOBAL' OR created_by = ?)", viewerId)
		}
	}
	err = q.Order("id asc").Find(&list).Error
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

// MemberNamespaceIds 用户所在的全部命名空间 id（F-017b 技能列表可见性 scope 用）。
func (r *SkillNamespaceRepository) MemberNamespaceIds(ctx context.Context, userId string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Model(&entity.SkillNamespaceMember{}).
		Where("user_id = ?", userId).Pluck("namespace_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
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
