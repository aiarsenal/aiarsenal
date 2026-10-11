package persistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type SkillSecurityRepository struct{ db *gorm.DB }

func NewSkillSecurityRepository() *SkillSecurityRepository {
	return &SkillSecurityRepository{db: db.SkillDB()}
}

func (r *SkillSecurityRepository) ListAudits(ctx context.Context, versionId, status string, pageNum, pageSize int) (list []*entity.SkillSecurityAudit, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.SkillSecurityAudit{})
	if versionId != "" {
		q = q.Where("skill_version_id = ?", versionId)
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
	err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *SkillSecurityRepository) GetAudit(ctx context.Context, auditId string) (*entity.SkillSecurityAudit, error) {
	var e entity.SkillSecurityAudit
	if err := r.db.WithContext(ctx).Where("audit_id = ?", auditId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillSecurityRepository) AddAudit(ctx context.Context, a *entity.SkillSecurityAudit) (string, error) {
	a.AuditId = idgen.New()
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return "", err
	}
	return a.AuditId, nil
}

// Rescan 触发重新扫描：创建一条 PENDING 审计记录（结果由 ScannerService 后台 goroutine 回填）。
func (r *SkillSecurityRepository) Rescan(ctx context.Context, versionId string) (string, error) {
	a := &entity.SkillSecurityAudit{
		SkillVersionId: versionId,
		Status:         0, // PENDING
		Severity:       0,
	}
	a.AuditId = idgen.New()
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return "", err
	}
	return a.AuditId, nil
}

// UpdateAudit 扫描完成后回写结论（status 见 entity.AuditStatus*，severity 取最高，findings jsonb）。
func (r *SkillSecurityRepository) UpdateAudit(ctx context.Context, auditId string, status, severity uint, findings json.RawMessage, scannerRaw string) error {
	return r.db.WithContext(ctx).Model(&entity.SkillSecurityAudit{}).Where("audit_id = ?", auditId).
		Updates(map[string]interface{}{
			"status":        status,
			"severity":      severity,
			"findings_json": findings,
			"scanner_raw":   scannerRaw,
			"scanned_at":    time.Now(),
		}).Error
}

// LatestAuditBySkillVersion 取版本最新一条审计（同版本多次重扫取 id 最大=最近一次）。
func (r *SkillSecurityRepository) LatestAuditBySkillVersion(ctx context.Context, versionId string) (*entity.SkillSecurityAudit, error) {
	var e entity.SkillSecurityAudit
	if err := r.db.WithContext(ctx).Where("skill_version_id = ?", versionId).
		Order("id desc").First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// LatestAuditsByVersions 批量取各版本最新审计：按 id 升序灌 map，后写覆盖前写=每版本留最大 id。
func (r *SkillSecurityRepository) LatestAuditsByVersions(ctx context.Context, versionIds []string) (map[string]*entity.SkillSecurityAudit, error) {
	res := make(map[string]*entity.SkillSecurityAudit, len(versionIds))
	if len(versionIds) == 0 {
		return res, nil
	}
	var list []*entity.SkillSecurityAudit
	if err := r.db.WithContext(ctx).Where("skill_version_id IN ?", versionIds).
		Order("id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	for _, a := range list {
		res[a.SkillVersionId] = a
	}
	return res, nil
}
