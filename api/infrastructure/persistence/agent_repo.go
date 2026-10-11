package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// Agent 模块仓储实现（agent 库）。统一走 db.AgentDB()。
// 写入时 idgen.New() 生成业务 id；软删表 Where deleted_at is null；删除=置 deleted_at。
// 只读治理记录表（invocation/audit_event/cost_record/risk_hit/scan_report）仅 List/Get。

// ==================== 配置中心 ====================

type AgentConfigResourceRepository struct{ db *gorm.DB }

func NewAgentConfigResourceRepository() *AgentConfigResourceRepository {
	return &AgentConfigResourceRepository{db: db.AgentDB()}
}

func (r *AgentConfigResourceRepository) List(ctx context.Context, name, resourceType string, pageNum, pageSize int) (list []*entity.AgentConfigResource, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentConfigResource{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentConfigResourceRepository) Get(ctx context.Context, id string) (*entity.AgentConfigResource, error) {
	var e entity.AgentConfigResource
	if err := r.db.WithContext(ctx).Where("config_resource_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentConfigResourceRepository) Add(ctx context.Context, e *entity.AgentConfigResource) (string, error) {
	e.ConfigResourceId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ConfigResourceId, nil
}

func (r *AgentConfigResourceRepository) Edit(ctx context.Context, e *entity.AgentConfigResource) error {
	return r.db.WithContext(ctx).Model(&entity.AgentConfigResource{}).
		Where("config_resource_id = ?", e.ConfigResourceId).Updates(e).Error
}

func (r *AgentConfigResourceRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentConfigResource{}).
		Where("config_resource_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- ConfigBundle ----

type AgentConfigBundleRepository struct{ db *gorm.DB }

func NewAgentConfigBundleRepository() *AgentConfigBundleRepository {
	return &AgentConfigBundleRepository{db: db.AgentDB()}
}

func (r *AgentConfigBundleRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.AgentConfigBundle, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentConfigBundle{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentConfigBundleRepository) Get(ctx context.Context, id string) (*entity.AgentConfigBundle, error) {
	var e entity.AgentConfigBundle
	if err := r.db.WithContext(ctx).Where("config_bundle_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentConfigBundleRepository) Add(ctx context.Context, e *entity.AgentConfigBundle) (string, error) {
	e.ConfigBundleId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ConfigBundleId, nil
}

func (r *AgentConfigBundleRepository) Edit(ctx context.Context, e *entity.AgentConfigBundle) error {
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundle{}).
		Where("config_bundle_id = ?", e.ConfigBundleId).Updates(e).Error
}

func (r *AgentConfigBundleRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundle{}).
		Where("config_bundle_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- ConfigBundleItem ----

type AgentConfigBundleItemRepository struct{ db *gorm.DB }

func NewAgentConfigBundleItemRepository() *AgentConfigBundleItemRepository {
	return &AgentConfigBundleItemRepository{db: db.AgentDB()}
}

func (r *AgentConfigBundleItemRepository) List(ctx context.Context, bundleId string, pageNum, pageSize int) (list []*entity.AgentConfigBundleItem, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentConfigBundleItem{}).Where("deleted_at is null")
	if bundleId != "" {
		q = q.Where("bundle_id = ?", bundleId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentConfigBundleItemRepository) Get(ctx context.Context, id string) (*entity.AgentConfigBundleItem, error) {
	var e entity.AgentConfigBundleItem
	if err := r.db.WithContext(ctx).Where("bundle_item_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentConfigBundleItemRepository) Add(ctx context.Context, e *entity.AgentConfigBundleItem) (string, error) {
	e.BundleItemId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.BundleItemId, nil
}

func (r *AgentConfigBundleItemRepository) Edit(ctx context.Context, e *entity.AgentConfigBundleItem) error {
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundleItem{}).
		Where("bundle_item_id = ?", e.BundleItemId).Updates(e).Error
}

func (r *AgentConfigBundleItemRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundleItem{}).
		Where("bundle_item_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- ConfigBundleSkill ----

type AgentConfigBundleSkillRepository struct{ db *gorm.DB }

func NewAgentConfigBundleSkillRepository() *AgentConfigBundleSkillRepository {
	return &AgentConfigBundleSkillRepository{db: db.AgentDB()}
}

func (r *AgentConfigBundleSkillRepository) List(ctx context.Context, bundleId string, pageNum, pageSize int) (list []*entity.AgentConfigBundleSkill, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentConfigBundleSkill{}).Where("deleted_at is null")
	if bundleId != "" {
		q = q.Where("bundle_id = ?", bundleId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentConfigBundleSkillRepository) Get(ctx context.Context, id string) (*entity.AgentConfigBundleSkill, error) {
	var e entity.AgentConfigBundleSkill
	if err := r.db.WithContext(ctx).Where("bundle_skill_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentConfigBundleSkillRepository) Add(ctx context.Context, e *entity.AgentConfigBundleSkill) (string, error) {
	e.BundleSkillId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.BundleSkillId, nil
}

func (r *AgentConfigBundleSkillRepository) Edit(ctx context.Context, e *entity.AgentConfigBundleSkill) error {
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundleSkill{}).
		Where("bundle_skill_id = ?", e.BundleSkillId).Updates(e).Error
}

func (r *AgentConfigBundleSkillRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentConfigBundleSkill{}).
		Where("bundle_skill_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ==================== 安全扫描 ====================

type AgentSecurityScanConfigRepository struct{ db *gorm.DB }

func NewAgentSecurityScanConfigRepository() *AgentSecurityScanConfigRepository {
	return &AgentSecurityScanConfigRepository{db: db.AgentDB()}
}

func (r *AgentSecurityScanConfigRepository) List(ctx context.Context, pageNum, pageSize int) (list []*entity.AgentSecurityScanConfig, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentSecurityScanConfig{}).Where("deleted_at is null")
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentSecurityScanConfigRepository) Get(ctx context.Context, id string) (*entity.AgentSecurityScanConfig, error) {
	var e entity.AgentSecurityScanConfig
	if err := r.db.WithContext(ctx).Where("scan_config_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentSecurityScanConfigRepository) Add(ctx context.Context, e *entity.AgentSecurityScanConfig) (string, error) {
	e.ScanConfigId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ScanConfigId, nil
}

func (r *AgentSecurityScanConfigRepository) Edit(ctx context.Context, e *entity.AgentSecurityScanConfig) error {
	return r.db.WithContext(ctx).Model(&entity.AgentSecurityScanConfig{}).
		Where("scan_config_id = ?", e.ScanConfigId).Updates(e).Error
}

func (r *AgentSecurityScanConfigRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentSecurityScanConfig{}).
		Where("scan_config_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- SecurityScanJob ----

type AgentSecurityScanJobRepository struct{ db *gorm.DB }

func NewAgentSecurityScanJobRepository() *AgentSecurityScanJobRepository {
	return &AgentSecurityScanJobRepository{db: db.AgentDB()}
}

func (r *AgentSecurityScanJobRepository) List(ctx context.Context, assetType, status string, pageNum, pageSize int) (list []*entity.AgentSecurityScanJob, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentSecurityScanJob{}).Where("deleted_at is null")
	if assetType != "" {
		q = q.Where("asset_type = ?", assetType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentSecurityScanJobRepository) Get(ctx context.Context, id string) (*entity.AgentSecurityScanJob, error) {
	var e entity.AgentSecurityScanJob
	if err := r.db.WithContext(ctx).Where("scan_job_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentSecurityScanJobRepository) Add(ctx context.Context, e *entity.AgentSecurityScanJob) (string, error) {
	e.ScanJobId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ScanJobId, nil
}

func (r *AgentSecurityScanJobRepository) Edit(ctx context.Context, e *entity.AgentSecurityScanJob) error {
	return r.db.WithContext(ctx).Model(&entity.AgentSecurityScanJob{}).
		Where("scan_job_id = ?", e.ScanJobId).Updates(e).Error
}

func (r *AgentSecurityScanJobRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentSecurityScanJob{}).
		Where("scan_job_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- SecurityScanJobItem ----

type AgentSecurityScanJobItemRepository struct{ db *gorm.DB }

func NewAgentSecurityScanJobItemRepository() *AgentSecurityScanJobItemRepository {
	return &AgentSecurityScanJobItemRepository{db: db.AgentDB()}
}

func (r *AgentSecurityScanJobItemRepository) List(ctx context.Context, jobId string, pageNum, pageSize int) (list []*entity.AgentSecurityScanJobItem, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentSecurityScanJobItem{})
	if jobId != "" {
		q = q.Where("job_id = ?", jobId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentSecurityScanJobItemRepository) Get(ctx context.Context, id string) (*entity.AgentSecurityScanJobItem, error) {
	var e entity.AgentSecurityScanJobItem
	if err := r.db.WithContext(ctx).Where("job_item_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentSecurityScanJobItemRepository) Add(ctx context.Context, e *entity.AgentSecurityScanJobItem) (string, error) {
	e.JobItemId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.JobItemId, nil
}

func (r *AgentSecurityScanJobItemRepository) Edit(ctx context.Context, e *entity.AgentSecurityScanJobItem) error {
	return r.db.WithContext(ctx).Model(&entity.AgentSecurityScanJobItem{}).Where("job_item_id = ?", e.JobItemId).Updates(e).Error
}

func (r *AgentSecurityScanJobItemRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("job_item_id in ?", ids).Delete(&entity.AgentSecurityScanJobItem{}).Error
}

// ---- SecurityScanReport（只读） ----

type AgentSecurityScanReportRepository struct{ db *gorm.DB }

func NewAgentSecurityScanReportRepository() *AgentSecurityScanReportRepository {
	return &AgentSecurityScanReportRepository{db: db.AgentDB()}
}

func (r *AgentSecurityScanReportRepository) List(ctx context.Context, jobId string, pageNum, pageSize int) (list []*entity.AgentSecurityScanReport, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentSecurityScanReport{})
	if jobId != "" {
		q = q.Where("job_id = ?", jobId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentSecurityScanReportRepository) Get(ctx context.Context, id string) (*entity.AgentSecurityScanReport, error) {
	var e entity.AgentSecurityScanReport
	if err := r.db.WithContext(ctx).Where("scan_report_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ==================== AI Gateway 治理 ====================

// ---- AgentModel（api_key 由 service 层加解密） ----

type AgentModelRepository struct{ db *gorm.DB }

func NewAgentModelRepository() *AgentModelRepository { return &AgentModelRepository{db: db.AgentDB()} }

func (r *AgentModelRepository) List(ctx context.Context, name, providerType string, pageNum, pageSize int) (list []*entity.AgentModel, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentModel{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("display_name like ?", "%"+name+"%")
	}
	if providerType != "" {
		q = q.Where("provider_type = ?", providerType)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentModelRepository) Get(ctx context.Context, id string) (*entity.AgentModel, error) {
	var e entity.AgentModel
	if err := r.db.WithContext(ctx).Where("agent_model_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentModelRepository) Add(ctx context.Context, e *entity.AgentModel) (string, error) {
	e.AgentModelId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.AgentModelId, nil
}

func (r *AgentModelRepository) Edit(ctx context.Context, e *entity.AgentModel) error {
	return r.db.WithContext(ctx).Model(&entity.AgentModel{}).Where("agent_model_id = ?", e.AgentModelId).Updates(e).Error
}

func (r *AgentModelRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentModel{}).
		Where("agent_model_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- RiskRule ----

type AgentRiskRuleRepository struct{ db *gorm.DB }

func NewAgentRiskRuleRepository() *AgentRiskRuleRepository {
	return &AgentRiskRuleRepository{db: db.AgentDB()}
}

func (r *AgentRiskRuleRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.AgentRiskRule, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentRiskRule{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("display_name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentRiskRuleRepository) Get(ctx context.Context, id string) (*entity.AgentRiskRule, error) {
	var e entity.AgentRiskRule
	if err := r.db.WithContext(ctx).Where("risk_rule_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentRiskRuleRepository) Add(ctx context.Context, e *entity.AgentRiskRule) (string, error) {
	e.RiskRuleId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.RiskRuleId, nil
}

func (r *AgentRiskRuleRepository) Edit(ctx context.Context, e *entity.AgentRiskRule) error {
	return r.db.WithContext(ctx).Model(&entity.AgentRiskRule{}).Where("risk_rule_id = ?", e.RiskRuleId).Updates(e).Error
}

func (r *AgentRiskRuleRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentRiskRule{}).
		Where("risk_rule_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ---- 只读治理记录 ----

type AgentModelInvocationRepository struct{ db *gorm.DB }

func NewAgentModelInvocationRepository() *AgentModelInvocationRepository {
	return &AgentModelInvocationRepository{db: db.AgentDB()}
}

func (r *AgentModelInvocationRepository) List(ctx context.Context, userId, modelId string, pageNum, pageSize int) (list []*entity.AgentModelInvocation, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentModelInvocation{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if modelId != "" {
		q = q.Where("model_id = ?", modelId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentModelInvocationRepository) Get(ctx context.Context, id string) (*entity.AgentModelInvocation, error) {
	var e entity.AgentModelInvocation
	if err := r.db.WithContext(ctx).Where("invocation_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ---- AuditEvent ----

type AgentAuditEventRepository struct{ db *gorm.DB }

func NewAgentAuditEventRepository() *AgentAuditEventRepository {
	return &AgentAuditEventRepository{db: db.AgentDB()}
}

func (r *AgentAuditEventRepository) List(ctx context.Context, userId, eventType string, pageNum, pageSize int) (list []*entity.AgentAuditEvent, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentAuditEvent{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentAuditEventRepository) Get(ctx context.Context, id string) (*entity.AgentAuditEvent, error) {
	var e entity.AgentAuditEvent
	if err := r.db.WithContext(ctx).Where("audit_event_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ---- CostRecord ----

type AgentCostRecordRepository struct{ db *gorm.DB }

func NewAgentCostRecordRepository() *AgentCostRecordRepository {
	return &AgentCostRecordRepository{db: db.AgentDB()}
}

func (r *AgentCostRecordRepository) List(ctx context.Context, userId string, pageNum, pageSize int) (list []*entity.AgentCostRecord, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentCostRecord{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentCostRecordRepository) Get(ctx context.Context, id string) (*entity.AgentCostRecord, error) {
	var e entity.AgentCostRecord
	if err := r.db.WithContext(ctx).Where("cost_record_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ---- RiskHit ----

type AgentRiskHitRepository struct{ db *gorm.DB }

func NewAgentRiskHitRepository() *AgentRiskHitRepository {
	return &AgentRiskHitRepository{db: db.AgentDB()}
}

func (r *AgentRiskHitRepository) List(ctx context.Context, ruleId string, pageNum, pageSize int) (list []*entity.AgentRiskHit, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentRiskHit{})
	if ruleId != "" {
		q = q.Where("rule_id = ?", ruleId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentRiskHitRepository) Get(ctx context.Context, id string) (*entity.AgentRiskHit, error) {
	var e entity.AgentRiskHit
	if err := r.db.WithContext(ctx).Where("risk_hit_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ==================== 审计 / 配额 ====================

// ---- AuditLog（只读 + 硬删除） ----

type AgentAuditLogRepository struct{ db *gorm.DB }

func NewAgentAuditLogRepository() *AgentAuditLogRepository {
	return &AgentAuditLogRepository{db: db.AgentDB()}
}

func (r *AgentAuditLogRepository) List(ctx context.Context, userId, action string, pageNum, pageSize int) (list []*entity.AgentAuditLog, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentAuditLog{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentAuditLogRepository) Get(ctx context.Context, id string) (*entity.AgentAuditLog, error) {
	var e entity.AgentAuditLog
	if err := r.db.WithContext(ctx).Where("audit_log_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentAuditLogRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("audit_log_id in ?", ids).Delete(&entity.AgentAuditLog{}).Error
}

// ---- UserQuota ----

type AgentUserQuotaRepository struct{ db *gorm.DB }

func NewAgentUserQuotaRepository() *AgentUserQuotaRepository {
	return &AgentUserQuotaRepository{db: db.AgentDB()}
}

func (r *AgentUserQuotaRepository) List(ctx context.Context, pageNum, pageSize int) (list []*entity.AgentUserQuota, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentUserQuota{})
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentUserQuotaRepository) GetByUser(ctx context.Context, userId string) (*entity.AgentUserQuota, error) {
	var e entity.AgentUserQuota
	if err := r.db.WithContext(ctx).Where("user_id = ?", userId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentUserQuotaRepository) Add(ctx context.Context, e *entity.AgentUserQuota) (string, error) {
	e.UserQuotaId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.UserQuotaId, nil
}

func (r *AgentUserQuotaRepository) Edit(ctx context.Context, e *entity.AgentUserQuota) error {
	return r.db.WithContext(ctx).Model(&entity.AgentUserQuota{}).Where("user_quota_id = ?", e.UserQuotaId).Updates(e).Error
}

func (r *AgentUserQuotaRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("user_quota_id in ?", ids).Delete(&entity.AgentUserQuota{}).Error
}
