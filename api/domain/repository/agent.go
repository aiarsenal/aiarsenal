package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// Agent 模块仓储接口（agent 库）。
// 软删表统一 Where deleted_at is null；只读治理记录表（invocation/audit_event/
// cost_record/risk_hit）仅 List/Get，由网关写入，admin 端只读。

// ===== 配置中心 =====

type IAgentConfigResourceRepository interface {
	List(ctx context.Context, name, resourceType string, pageNum, pageSize int) (list []*entity.AgentConfigResource, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentConfigResource, error)
	Add(ctx context.Context, e *entity.AgentConfigResource) (string, error)
	Edit(ctx context.Context, e *entity.AgentConfigResource) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentConfigBundleRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.AgentConfigBundle, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentConfigBundle, error)
	Add(ctx context.Context, e *entity.AgentConfigBundle) (string, error)
	Edit(ctx context.Context, e *entity.AgentConfigBundle) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentConfigBundleItemRepository interface {
	List(ctx context.Context, bundleId string, pageNum, pageSize int) (list []*entity.AgentConfigBundleItem, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentConfigBundleItem, error)
	Add(ctx context.Context, e *entity.AgentConfigBundleItem) (string, error)
	Edit(ctx context.Context, e *entity.AgentConfigBundleItem) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentConfigBundleSkillRepository interface {
	List(ctx context.Context, bundleId string, pageNum, pageSize int) (list []*entity.AgentConfigBundleSkill, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentConfigBundleSkill, error)
	Add(ctx context.Context, e *entity.AgentConfigBundleSkill) (string, error)
	Edit(ctx context.Context, e *entity.AgentConfigBundleSkill) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 安全扫描 =====

type IAgentSecurityScanConfigRepository interface {
	List(ctx context.Context, pageNum, pageSize int) (list []*entity.AgentSecurityScanConfig, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentSecurityScanConfig, error)
	Add(ctx context.Context, e *entity.AgentSecurityScanConfig) (string, error)
	Edit(ctx context.Context, e *entity.AgentSecurityScanConfig) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentSecurityScanJobRepository interface {
	List(ctx context.Context, assetType, status string, pageNum, pageSize int) (list []*entity.AgentSecurityScanJob, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentSecurityScanJob, error)
	Add(ctx context.Context, e *entity.AgentSecurityScanJob) (string, error)
	Edit(ctx context.Context, e *entity.AgentSecurityScanJob) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentSecurityScanJobItemRepository interface {
	List(ctx context.Context, jobId string, pageNum, pageSize int) (list []*entity.AgentSecurityScanJobItem, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentSecurityScanJobItem, error)
	Add(ctx context.Context, e *entity.AgentSecurityScanJobItem) (string, error)
	Edit(ctx context.Context, e *entity.AgentSecurityScanJobItem) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentSecurityScanReportRepository interface {
	List(ctx context.Context, jobId string, pageNum, pageSize int) (list []*entity.AgentSecurityScanReport, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentSecurityScanReport, error)
}

// ===== AI Gateway 治理 =====

type IAgentModelRepository interface {
	List(ctx context.Context, name, providerType string, pageNum, pageSize int) (list []*entity.AgentModel, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentModel, error)
	Add(ctx context.Context, e *entity.AgentModel) (string, error)
	Edit(ctx context.Context, e *entity.AgentModel) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentRiskRuleRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.AgentRiskRule, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentRiskRule, error)
	Add(ctx context.Context, e *entity.AgentRiskRule) (string, error)
	Edit(ctx context.Context, e *entity.AgentRiskRule) error
	Delete(ctx context.Context, ids []string) error
}

// 只读治理记录
type IAgentModelInvocationRepository interface {
	List(ctx context.Context, userId, modelId string, pageNum, pageSize int) (list []*entity.AgentModelInvocation, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentModelInvocation, error)
}

type IAgentAuditEventRepository interface {
	List(ctx context.Context, userId, eventType string, pageNum, pageSize int) (list []*entity.AgentAuditEvent, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentAuditEvent, error)
}

type IAgentCostRecordRepository interface {
	List(ctx context.Context, userId string, pageNum, pageSize int) (list []*entity.AgentCostRecord, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentCostRecord, error)
}

type IAgentRiskHitRepository interface {
	List(ctx context.Context, ruleId string, pageNum, pageSize int) (list []*entity.AgentRiskHit, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentRiskHit, error)
}

// ===== 审计 / 配额 =====

type IAgentAuditLogRepository interface {
	List(ctx context.Context, userId, action string, pageNum, pageSize int) (list []*entity.AgentAuditLog, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentAuditLog, error)
	Delete(ctx context.Context, ids []string) error
}

type IAgentUserQuotaRepository interface {
	List(ctx context.Context, pageNum, pageSize int) (list []*entity.AgentUserQuota, total int64, err error)
	GetByUser(ctx context.Context, userId string) (*entity.AgentUserQuota, error)
	Add(ctx context.Context, e *entity.AgentUserQuota) (string, error)
	Edit(ctx context.Context, e *entity.AgentUserQuota) error
	Delete(ctx context.Context, ids []string) error
}
