package repository

import (
	"context"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// LLM 模块仓储接口（llm 库）。软删表统一 Where deleted_at is null。

type ILLMCredentialRepository interface {
	List(ctx context.Context, name, providerType string, pageNum, pageSize int) (list []*entity.LLMCredential, total int64, err error)
	Get(ctx context.Context, id string) (*entity.LLMCredential, error)
	Add(ctx context.Context, e *entity.LLMCredential) (string, error)
	Edit(ctx context.Context, e *entity.LLMCredential) error
	Delete(ctx context.Context, ids []string) error
}

type ILLMModelRepository interface {
	List(ctx context.Context, name, provider, group string, status *uint, pageNum, pageSize int) (list []*entity.LLMModel, total int64, err error)
	All(ctx context.Context) (list []*entity.LLMModel, err error)
	Get(ctx context.Context, id string) (*entity.LLMModel, error)
	Add(ctx context.Context, e *entity.LLMModel) (string, error)
	Edit(ctx context.Context, e *entity.LLMModel) error
	Delete(ctx context.Context, ids []string) error
	// BatchSyncModelTokenLimits 把同 (model_group, model_name) 下所有部署的 max_input/output_tokens
	// 同步成同一值——这俩是模型固有属性，编辑一个部署时批量对齐，避免下发富化取到参差值。
	BatchSyncModelTokenLimits(ctx context.Context, group, modelName string, maxInput, maxOutput int, updatedBy string) error
	ExistsDeploy(ctx context.Context, group, providerModel, credID string) (bool, error)
}

type ILLMBudgetRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMBudget, total int64, err error)
	Get(ctx context.Context, id string) (*entity.LLMBudget, error)
	Add(ctx context.Context, e *entity.LLMBudget) (string, error)
	Edit(ctx context.Context, e *entity.LLMBudget) error
	Delete(ctx context.Context, ids []string) error
	ListGrantBudgets(ctx context.Context) ([]*entity.LLMBudget, error)
	RollBudgetWindow(ctx context.Context, budgetID string) (*entity.LLMBudget, error)
	TryBudgetNoticeDedup(ctx context.Context, userId, budgetId, windowKey, level string) (bool, error)
}

// 临时预算申请仓储已上收统一审核中心（ILLMBudgetRequestRepository 删除，
// 见 repository/review.go 与 persistence/review_order_repo.go）。

type ILLMModelGroupRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMModelGroup, total int64, err error)
	Get(ctx context.Context, id string) (*entity.LLMModelGroup, error)
	GetByName(ctx context.Context, name string) (*entity.LLMModelGroup, error)
	Add(ctx context.Context, e *entity.LLMModelGroup) (string, error)
	Edit(ctx context.Context, e *entity.LLMModelGroup) error
	Delete(ctx context.Context, ids []string) error
}

type ILLMVirtualKeyRepository interface {
	List(ctx context.Context, alias, userId, teamId string, pageNum, pageSize int) (list []*entity.LLMVirtualKey, total int64, err error)
	Get(ctx context.Context, tokenHash string) (*entity.LLMVirtualKey, error)
	GetByAlias(ctx context.Context, alias string) (*entity.LLMVirtualKey, error)
	Add(ctx context.Context, e *entity.LLMVirtualKey) (string, error)
	Edit(ctx context.Context, e *entity.LLMVirtualKey) error
	UpdateBlocked(ctx context.Context, tokenHash string, blocked uint, by string) error // 单独更新 blocked（map 更新，避免 gorm 结构体 Updates 跳过零值致解封 blocked=0 不生效）
	Delete(ctx context.Context, hashes []string) error
	UpdateSpend(ctx context.Context, tokenHash string, delta types.Milli) error
	UpdateLastActive(ctx context.Context, tokenHash string) error
	// RotateHash 原地轮换：token_hash 从 oldHash 换成 newHash，旧 hash 暂存
	// deprecated_token_hash 并设 deprecate_expires_at=graceExpires（宽限期）。
	// rotation_count 自增；key_rotation_at 清空；其余字段（别名/预算/限速/归属）保留。
	RotateHash(ctx context.Context, oldHash, newHash string, graceExpires time.Time, by string) error
	// ListAffectedByModel 列出 /v1/models 会因该模型下线而变化的 key：allowed_models 为空/[]/["all"]
	// （不限制）或含 "group/modelName" 或含裸组 "group"（覆盖成员）。用于模型禁用/删除后定位需刷新配置的实例。
	ListAffectedByModel(ctx context.Context, group, modelName string) (list []*entity.LLMVirtualKey, err error)
}

type ILLMDeptProfileRepository interface {
	// List 列出部门（含 LLM 配置列）。spend 由 service 聚合后回填。
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMDeptProfile, total int64, err error)
	Get(ctx context.Context, deptId string) (*entity.LLMDeptProfile, error)
	// UpdateLLMConfig 只更新 aa_dept 的 LLM 扩展列。
	UpdateLLMConfig(ctx context.Context, e *entity.LLMDeptProfile) error
	// NameMapByIds 批量取 dept_id→dept_name 映射，供层级路径翻译复用。
	NameMapByIds(ctx context.Context, deptIds []string) (map[string]string, error)
	// LeaderMapByIds 批量取 user_id→{name, employee_no}，供负责人列回填（不 join，单独读 aa_user）。
	LeaderMapByIds(ctx context.Context, userIds []string) (map[string]entity.LeaderInfo, error)
}

type ILLMProjectRepository interface {
	List(ctx context.Context, alias, teamId string, pageNum, pageSize int) (list []*entity.LLMProject, total int64, err error)
	Get(ctx context.Context, id string) (*entity.LLMProject, error)
	Add(ctx context.Context, e *entity.LLMProject) (string, error)
	Edit(ctx context.Context, e *entity.LLMProject) error
	Delete(ctx context.Context, ids []string) error
}

type ILLMEndUserRepository interface {
	List(ctx context.Context, alias string, pageNum, pageSize int) (list []*entity.LLMEndUser, total int64, err error)
	Get(ctx context.Context, id string) (*entity.LLMEndUser, error)
	Add(ctx context.Context, e *entity.LLMEndUser) (string, error)
	Edit(ctx context.Context, e *entity.LLMEndUser) error
	Delete(ctx context.Context, ids []string) error
}

type ILLMTagRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMTag, total int64, err error)
	Get(ctx context.Context, name string) (*entity.LLMTag, error)
	Add(ctx context.Context, e *entity.LLMTag) error
	Edit(ctx context.Context, e *entity.LLMTag) error
	Delete(ctx context.Context, names []string) error
}

type ILLMSpendLogRepository interface {
	List(ctx context.Context, userId, teamId, model, apiKey string, dateRange []string, pageNum, pageSize int) (list []*entity.LLMSpendLog, total int64, err error)
	Get(ctx context.Context, requestId string) (*entity.LLMSpendLog, error)
	Add(ctx context.Context, e *entity.LLMSpendLog) error
}

type ILLMDailySpendRepository interface {
	List(ctx context.Context, scopeType, scopeId, dateRange []string, pageNum, pageSize int) (list []*entity.LLMDailySpend, total int64, err error)
	Upsert(ctx context.Context, e *entity.LLMDailySpend) error
	// Aggregate 花费归集：dimension 取 date/key/user/model，按维度聚合窗口内花费/请求/Token。
	// key 维度附带 key_alias（LEFT JOIN aa_llm_virtual_key）。
	Aggregate(ctx context.Context, dimension string, dateRange []string, limit int) (list []*entity.LLMSpendAggregate, err error)
	// AggregateTotals 窗口内总花费/请求/Token（按 key scope 计，避免多 scope 重复）。
	AggregateTotals(ctx context.Context, dateRange []string) (*entity.LLMSpendTotals, error)
	// SumScopeDaily 某 scope 日粒度消费聚合（我的预算·历史周期消费：日行→service 再按周/月分桶）。
	SumScopeDaily(ctx context.Context, scopeType, scopeId string, dateRange []string) (list []*entity.LLMSpendPoint, err error)
}

type ILLMErrorLogRepository interface {
	List(ctx context.Context, model, apiKey string, dateRange []string, pageNum, pageSize int) (list []*entity.LLMErrorLog, total int64, err error)
	Add(ctx context.Context, e *entity.LLMErrorLog) error
}

type ILLMAuditLogRepository interface {
	List(ctx context.Context, tableName, action string, pageNum, pageSize int) (list []*entity.LLMAuditLog, total int64, err error)
	Add(ctx context.Context, e *entity.LLMAuditLog) error
}

type ILLMHealthCheckRepository interface {
	List(ctx context.Context, modelName, status string, pageNum, pageSize int) (list []*entity.LLMHealthCheck, total int64, err error)
	Add(ctx context.Context, e *entity.LLMHealthCheck) (string, error)
}

type ILLMPromptRepository interface {
	List(ctx context.Context, name, environment string, pageNum, pageSize int) (list []*entity.LLMPrompt, total int64, err error)
	Get(ctx context.Context, promptId string) ([]*entity.LLMPrompt, error)
	Add(ctx context.Context, e *entity.LLMPrompt) (string, error)
	Edit(ctx context.Context, e *entity.LLMPrompt) error
	Delete(ctx context.Context, ids []string) error
}
