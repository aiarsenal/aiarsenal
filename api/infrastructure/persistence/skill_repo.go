package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type SkillRepository struct{ db *gorm.DB }

func NewSkillRepository() *SkillRepository { return &SkillRepository{db: db.SkillDB()} }

func (r *SkillRepository) List(ctx context.Context, namespaceId, slug, name, status, keyWords string, pageNum, pageSize int) (list []*entity.Skill, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.Skill{}).Where("deleted_at is null")
	if namespaceId != "" {
		q = q.Where("namespace_id = ?", namespaceId)
	}
	if slug != "" {
		q = q.Where("slug like ?", "%"+slug+"%")
	}
	if name != "" {
		q = q.Where("display_name like ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyWords != "" {
		q = q.Where("display_name like ? or summary like ?", "%"+keyWords+"%", "%"+keyWords+"%")
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

// ListBySlug slug 精确匹配（slug 非全局唯一，同 slug 多条按 id desc 返回，调用方择优）。
func (r *SkillRepository) ListBySlug(ctx context.Context, slug string) ([]*entity.Skill, error) {
	var list []*entity.Skill
	err := r.db.WithContext(ctx).Where("deleted_at is null and slug = ?", slug).Order("id desc").Find(&list).Error
	return list, err
}

func (r *SkillRepository) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	var e entity.Skill
	if err := r.db.WithContext(ctx).Where("skill_id = ?", skillId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillRepository) Add(ctx context.Context, s *entity.Skill) (string, error) {
	s.SkillId = idgen.New()
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return "", err
	}
	return s.SkillId, nil
}

func (r *SkillRepository) Edit(ctx context.Context, s *entity.Skill) error {
	return r.db.WithContext(ctx).Model(&entity.Skill{}).Where("skill_id = ?", s.SkillId).Updates(s).Error
}

// EditFields 显式列更新：Updates(struct) 会跳过零值字段（visibility=0 公开、
// status=0 下架、清空简介），需要把零值写回的编辑一律走这里。
func (r *SkillRepository) EditFields(ctx context.Context, skillId string, fields map[string]any) error {
	if skillId == "" || len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.Skill{}).Where("skill_id = ?", skillId).Updates(fields).Error
}

func (r *SkillRepository) Delete(ctx context.Context, skillIds []string) error {
	if len(skillIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.Skill{}).
		Where("skill_id in ?", skillIds).Update("deleted_at", time.Now()).Error
}

func (r *SkillRepository) SetHidden(ctx context.Context, skillId string, hidden uint, by string) error {
	upd := map[string]interface{}{"hidden": hidden, "hidden_by": by}
	if hidden == 1 {
		upd["hidden_at"] = time.Now()
	} else {
		upd["hidden_at"] = nil
	}
	return r.db.WithContext(ctx).Model(&entity.Skill{}).Where("skill_id = ?", skillId).Updates(upd).Error
}

func (r *SkillRepository) ListVersions(ctx context.Context, skillId string) ([]*entity.SkillVersion, error) {
	var list []*entity.SkillVersion
	err := r.db.WithContext(ctx).Where("skill_id = ?", skillId).Order("id desc").Find(&list).Error
	return list, err
}

func (r *SkillRepository) GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error) {
	var e entity.SkillVersion
	if err := r.db.WithContext(ctx).Where("version_id = ?", versionId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillRepository) AddVersion(ctx context.Context, v *entity.SkillVersion) (string, error) {
	v.VersionId = idgen.New()
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return "", err
	}
	return v.VersionId, nil
}

func (r *SkillRepository) PublishVersion(ctx context.Context, versionId string) error {
	return r.db.WithContext(ctx).Model(&entity.SkillVersion{}).Where("version_id = ?", versionId).
		Updates(map[string]interface{}{"status": 1, "published_at": time.Now(), "download_ready": 1}).Error
}

func (r *SkillRepository) YankVersion(ctx context.Context, versionId, by, reason string) error {
	return r.db.WithContext(ctx).Model(&entity.SkillVersion{}).Where("version_id = ?", versionId).
		Updates(map[string]interface{}{"status": 2, "yanked_at": time.Now(), "yanked_by": by, "yank_reason": reason, "download_ready": 0}).Error
}

func (r *SkillRepository) SetLatestVersion(ctx context.Context, skillId, versionId string) error {
	return r.db.WithContext(ctx).Model(&entity.Skill{}).Where("skill_id = ?", skillId).
		Update("latest_version_id", versionId).Error
}

func (r *SkillRepository) ListTags(ctx context.Context, skillId string) ([]*entity.SkillTag, error) {
	var list []*entity.SkillTag
	err := r.db.WithContext(ctx).Where("skill_id = ?", skillId).Find(&list).Error
	return list, err
}

func (r *SkillRepository) AddTag(ctx context.Context, t *entity.SkillTag) error {
	t.TagId = idgen.New()
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *SkillRepository) RemoveTag(ctx context.Context, tagId string) error {
	return r.db.WithContext(ctx).Where("tag_id = ?", tagId).Delete(&entity.SkillTag{}).Error
}

func (r *SkillRepository) ListFiles(ctx context.Context, versionId string) ([]*entity.SkillFile, error) {
	var list []*entity.SkillFile
	err := r.db.WithContext(ctx).Where("version_id = ?", versionId).Order("file_path asc").Find(&list).Error
	return list, err
}

func (r *SkillRepository) GetFile(ctx context.Context, versionId, filePath string) (*entity.SkillFile, error) {
	var f entity.SkillFile
	if err := r.db.WithContext(ctx).Where("version_id = ? AND file_path = ?", versionId, filePath).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *SkillRepository) AddFile(ctx context.Context, f *entity.SkillFile) error {
	f.FileId = idgen.New()
	return r.db.WithContext(ctx).Create(f).Error
}

// UpsertSearchDocument upsert 搜索文档（含 tsvector 生成列自动更新 + semantic_vector 向量列）。
// vectorText 为 pgvector 文本形 [v1,v2,...]；为空则不写向量列。
func (r *SkillRepository) UpsertSearchDocument(ctx context.Context, skillId, title, summary, keywords, searchText, namespaceId, namespaceSlug, ownerId string, visibility, status uint, vectorText string) error {
	vecExpr := "null"
	if vectorText != "" {
		vecExpr = "?::vector"
	}
	sql := `insert into aa_skill_search_document
		(skill_id, namespace_id, namespace_slug, owner_id, title, summary, keywords, search_text, visibility, status, semantic_vector, updated_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ` + vecExpr + `, now())
		on conflict (skill_id) do update set
			namespace_id=excluded.namespace_id, namespace_slug=excluded.namespace_slug, owner_id=excluded.owner_id,
			title=excluded.title, summary=excluded.summary, keywords=excluded.keywords, search_text=excluded.search_text,
			visibility=excluded.visibility, status=excluded.status,` + func() string {
		if vectorText != "" {
			return " semantic_vector=excluded.semantic_vector,"
		}
		return ""
	}() + ` updated_at=now()`
	args := []interface{}{skillId, namespaceId, namespaceSlug, ownerId, title, summary, keywords, searchText, visibility, status}
	if vectorText != "" {
		args = append(args, vectorText)
	}
	return r.db.WithContext(ctx).Exec(sql, args...).Error
}

// SearchByVector 向量相似度检索（cosine 距离 <=>，越小越相似）。
func (r *SkillRepository) SearchByVector(ctx context.Context, queryVectorText string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows []map[string]interface{}
	err := r.db.WithContext(ctx).Raw(`
		select s.skill_id as skill_id, s.title as title, s.summary as summary, s.namespace_slug as namespace_slug,
			1 - (s.semantic_vector <=> ?::vector) as score
		from aa_skill_search_document s
		where s.semantic_vector is not null and s.status = 1
		order by s.semantic_vector <=> ?::vector
		limit ?`, queryVectorText, queryVectorText, limit).Scan(&rows).Error
	return rows, err
}
