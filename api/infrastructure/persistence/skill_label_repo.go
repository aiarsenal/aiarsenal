package persistence

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type SkillLabelRepository struct{ db *gorm.DB }

func NewSkillLabelRepository() *SkillLabelRepository {
	return &SkillLabelRepository{db: db.SkillDB()}
}

func (r *SkillLabelRepository) List(ctx context.Context, slug, ltype string, pageNum, pageSize int) (list []*entity.SkillLabelDefinition, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.SkillLabelDefinition{})
	if slug != "" {
		q = q.Where("slug like ?", "%"+slug+"%")
	}
	if ltype != "" {
		q = q.Where("type = ?", ltype)
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
	err = q.Order("sort_order asc, id asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	// 过滤诊断 + created_by 空值统计（用户反馈「创建人」列空）。
	emptyCreator := 0
	for _, l := range list {
		if l.CreatedBy == "" {
			emptyCreator++
		}
	}
	slog.Debug("skill label 列表查询", "slug", slug, "type", ltype, "pageNum", pageNum, "pageSize", pageSize, "total", total, "returned", len(list), "created_by_empty", emptyCreator, "err", err)
	return
}

// CreatorMapByIds 批量取 user_id→name（aa_user 在 aiarsenal 库，跨库单独读 db.DB()）。
func (r *SkillLabelRepository) CreatorMapByIds(ctx context.Context, userIds []string) (map[string]string, error) {
	out := make(map[string]string, len(userIds))
	if len(userIds) == 0 {
		return out, nil
	}
	var rows []struct {
		UserId string `gorm:"column:user_id"`
		Name   string `gorm:"column:name"`
	}
	if err := db.DB().WithContext(ctx).Table("aa_user").
		Select("user_id, name").
		Where("user_id in ? and deleted_at is null", userIds).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserId] = row.Name
	}
	return out, nil
}

func (r *SkillLabelRepository) All(ctx context.Context) (list []*entity.SkillLabelDefinition, err error) {
	err = r.db.WithContext(ctx).Where("visible_in_filter = ?", 1).Order("sort_order asc, id asc").Find(&list).Error
	return
}

func (r *SkillLabelRepository) Get(ctx context.Context, labelId string) (*entity.SkillLabelDefinition, error) {
	var e entity.SkillLabelDefinition
	if err := r.db.WithContext(ctx).Where("label_id = ?", labelId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillLabelRepository) Add(ctx context.Context, l *entity.SkillLabelDefinition) (string, error) {
	l.LabelId = idgen.New()
	if err := r.db.WithContext(ctx).Create(l).Error; err != nil {
		return "", err
	}
	return l.LabelId, nil
}

func (r *SkillLabelRepository) Edit(ctx context.Context, l *entity.SkillLabelDefinition) error {
	return r.db.WithContext(ctx).Model(&entity.SkillLabelDefinition{}).
		Where("label_id = ?", l.LabelId).Updates(l).Error
}

func (r *SkillLabelRepository) Delete(ctx context.Context, labelId string) error {
	return r.db.WithContext(ctx).Where("label_id = ?", labelId).Delete(&entity.SkillLabelDefinition{}).Error
}

func (r *SkillLabelRepository) ListTranslations(ctx context.Context, labelId string) ([]*entity.SkillLabelTranslation, error) {
	var list []*entity.SkillLabelTranslation
	err := r.db.WithContext(ctx).Where("label_id = ?", labelId).Find(&list).Error
	return list, err
}

func (r *SkillLabelRepository) UpsertTranslation(ctx context.Context, t *entity.SkillLabelTranslation) error {
	if t.TransId == "" {
		t.TransId = idgen.New()
	}
	// 按 (label_id, locale) 唯一约束 upsert
	return r.db.WithContext(ctx).Raw(`
		insert into aa_skill_label_translation (trans_id, label_id, locale, display_name, created_at, updated_at)
		values (?, ?, ?, ?, now(), now())
		on conflict (label_id, locale) do update set display_name = excluded.display_name, updated_at = now()`,
		t.TransId, t.LabelId, t.Locale, t.DisplayName).Error
}
