package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// LLM 模块请求/响应 DTO。

// ===== 通用 =====
type LLMIdsReq struct {
	Ids []string `json:"ids" form:"ids"`
}

// ===== Credential =====
type LLMCredentialSearchReq struct {
	Name         string `json:"name" form:"name"`
	ProviderType string `json:"providerType" form:"providerType"`
	PageReq
}
type LLMCredentialSearchRes struct {
	ListRes
	List []*entity.LLMCredential `json:"list"`
}
type LLMCredentialReq struct {
	CredentialName string `json:"credentialName" form:"credentialName" binding:"required"`
	ProviderType   string `json:"providerType" form:"providerType" binding:"required"`
	BaseURL        string `json:"baseUrl" form:"baseUrl"`
	APIKey         string `json:"apiKey" form:"apiKey"` // 明文，service 加密落库
	CredentialInfo string `json:"credentialInfo" form:"credentialInfo"`
	Status         uint   `json:"status" form:"status"`
}
type LLMCredentialAddReq struct{ *LLMCredentialReq }
type LLMCredentialEditReq struct {
	CredentialId string `json:"credentialId" form:"credentialId" binding:"required"`
	*LLMCredentialReq
}

// 按凭据导入支持模型：拿凭据调上游 /v1/models，把返回的模型批量写成模型部署。
type LLMDiscoverModelsReq struct {
	CredentialId string `json:"credentialId" form:"credentialId" binding:"required"`
	ModelGroup   string `json:"modelGroup" form:"modelGroup"` // 空 → "auto"
}
type LLMDiscoverModelsRes struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Models   []string `json:"models"` // 本次写入的 provider_model 列表
}

// ===== Model =====
type LLMModelSearchReq struct {
	Name     string `json:"name" form:"name"`
	Provider string `json:"provider" form:"provider"`
	Group    string `json:"group" form:"group"`
	// Status 用指针：0（停用）是有效筛选值，nil 表示不筛。
	Status *uint `json:"status" form:"status"`
	PageReq
}
type LLMModelSearchRes struct {
	ListRes
	List []*entity.LLMModel `json:"list"`
}
type LLMModelReq struct {
	ModelName                  string  `json:"modelName" form:"modelName" binding:"required"`
	ModelGroup                 string  `json:"modelGroup" form:"modelGroup" binding:"required"`
	Provider                   string  `json:"provider" form:"provider" binding:"required"`
	ProviderModel              string  `json:"providerModel" form:"providerModel" binding:"required"`
	CredentialId               string  `json:"credentialId" form:"credentialId"`
	ExtraParams                string  `json:"extraParams" form:"extraParams"`
	ModelInfo                  string  `json:"modelInfo" form:"modelInfo"`
	InputPricePerMillion         types.Milli `json:"inputPricePerMillion" form:"inputPricePerMillion"`         // $/M（Milli JSON 边界自动换算）
	OutputPricePerMillion        types.Milli `json:"outputPricePerMillion" form:"outputPricePerMillion"`
	CacheReadPricePerMillion     types.Milli `json:"cacheReadPricePerMillion" form:"cacheReadPricePerMillion"`
	CacheCreationPricePerMillion types.Milli `json:"cacheCreationPricePerMillion" form:"cacheCreationPricePerMillion"`
	MaxInputTokens             int     `json:"maxInputTokens" form:"maxInputTokens"`
	MaxOutputTokens            int     `json:"maxOutputTokens" form:"maxOutputTokens"`
	TpmLimit                   int64   `json:"tpmLimit" form:"tpmLimit"`
	RpmLimit                   int64   `json:"rpmLimit" form:"rpmLimit"`
	Weight                     int     `json:"weight" form:"weight"`
	Blocked                    uint    `json:"blocked" form:"blocked"`
	Status                     uint    `json:"status" form:"status"`
}
type LLMModelAddReq struct{ *LLMModelReq }
type LLMModelEditReq struct {
	ModelId string `json:"modelId" form:"modelId" binding:"required"`
	*LLMModelReq
}

// ===== Budget =====
type LLMBudgetSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type LLMBudgetSearchRes struct {
	ListRes
	List []*entity.LLMBudget `json:"list"`
}
type LLMBudgetReq struct {
	BudgetName          string  `json:"budgetName" form:"budgetName" binding:"required"`
	MaxBudget           types.Milli `json:"maxBudget" form:"maxBudget"`
	SoftBudget          types.Milli `json:"softBudget" form:"softBudget"`
	MaxParallelRequests int     `json:"maxParallelRequests" form:"maxParallelRequests"`
	TpmLimit            int64   `json:"tpmLimit" form:"tpmLimit"`
	RpmLimit            int64   `json:"rpmLimit" form:"rpmLimit"`
	ModelMaxBudget      string  `json:"modelMaxBudget" form:"modelMaxBudget"`
	BudgetDuration      string  `json:"budgetDuration" form:"budgetDuration"`
	AllowedModels       string  `json:"allowedModels" form:"allowedModels"`
	GrantRules          string  `json:"grantRules" form:"grantRules"` // 适用范围条件行 JSON 字符串（同菜单 grant_rules 保存形态）
}
type LLMBudgetAddReq struct{ *LLMBudgetReq }
type LLMBudgetEditReq struct {
	BudgetId string `json:"budgetId" form:"budgetId" binding:"required"`
	*LLMBudgetReq
}

// 我的预算：当前用户命中的圈人预算 + 本周期消费/剩余（service 组装，非实体直出）。
type LLMMyBudgetItem struct {
	BudgetId       string      `json:"budgetId"`
	BudgetName     string      `json:"budgetName"`
	MaxBudget      *types.Milli `json:"maxBudget"`     // nil = 不限额
	SoftBudget     *types.Milli `json:"softBudget"`
	Spend          types.Milli `json:"spend"`          // 本人本周期消费（user 维度日聚合，展示层计算，不写预算）
	Remaining      *types.Milli `json:"remaining"`     // 上限 − 本人本周期消费；nil = 不限额
	UsedPercent    float64     `json:"usedPercent"`    // 本人已用比例 0-100（不限额为 0）
	BudgetDuration string      `json:"budgetDuration"`
	BudgetResetAt  *types.Time `json:"budgetResetAt"`
	IsLow          bool        `json:"isLow"`          // 本人剩余 <10%（≥90%）
	IsExhausted    bool        `json:"isExhausted"`    // 本人已用尽（≥100%）
}
type LLMMyBudgetRes struct {
	List        []LLMMyBudgetItem `json:"list"`
	HitBudgetId string            `json:"hitBudgetId"` // 命中预算（生效上限最大的一条，唯一展示/提醒对象）
	Usage       LLMMyUsageSummary `json:"usage"`       // 本人本月消费汇总（tokens/金额/次数，与预算解耦的真实消费）
}

// LLMMyUsageSummary 本人自然月消费汇总（【我的预算】页顶部统计条）。
type LLMMyUsageSummary struct {
	Month            string  `json:"month"`            // 所属自然月 YYYY-MM
	Spend            types.Milli `json:"spend"`        // 消费金额（$，Milli JSON 边界换算）
	PromptTokens     int64   `json:"promptTokens"`     // 输入 tokens（上游报 0 时为 0）
	CompletionTokens int64   `json:"completionTokens"` // 输出 tokens
	Requests         int64   `json:"requests"`         // 调用次数
}

// 我的预算·历史周期消费：日/周/月分桶的消费曲线。
type LLMSpendHistoryReq struct {
	Period string `json:"period" form:"period"` // day/week/month，缺省 month
}
type LLMSpendHistoryPoint struct {
	Period string      `json:"period"` // day→2026-09-05 / week→2026-W36 / month→2026-09
	Spend  types.Milli `json:"spend"`
}
type LLMSpendHistoryRes struct {
	Period string                `json:"period"`
	Points []LLMSpendHistoryPoint `json:"points"`
}

// 临时预算申请 DTO 已上收统一审核中心（dto/review.go，order_type=llm_temp_budget）。

// ===== ModelGroup（分组字典）=====
type LLMModelGroupSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type LLMModelGroupSearchRes struct {
	ListRes
	List []*entity.LLMModelGroup `json:"list"`
}
type LLMModelGroupReq struct {
	GroupName   string `json:"groupName" form:"groupName" binding:"required"`
	Description string `json:"description" form:"description"`
	Status      uint   `json:"status" form:"status"`
}
type LLMModelGroupAddReq struct{ *LLMModelGroupReq }
type LLMModelGroupEditReq struct {
	GroupId string `json:"groupId" form:"groupId" binding:"required"`
	*LLMModelGroupReq
}

// ===== VirtualKey =====
type LLMKeySearchReq struct {
	Alias  string `json:"alias" form:"alias"`
	UserId string `json:"userId" form:"userId"`
	TeamId string `json:"teamId" form:"teamId"`
	PageReq
}
type LLMKeySearchRes struct {
	ListRes
	List []*entity.LLMVirtualKey `json:"list"`
}
type LLMKeyReq struct {
	KeyAlias            string  `json:"keyAlias" form:"keyAlias" binding:"required"`
	UserId              string  `json:"userId" form:"userId"`
	TeamId              string  `json:"teamId" form:"teamId"`
	ProjectId           string  `json:"projectId" form:"projectId"`
	OrgId               string  `json:"orgId" form:"orgId"`
	Models              string  `json:"models" form:"models"`
	MaxBudget           types.Milli `json:"maxBudget" form:"maxBudget"`
	SoftBudget          types.Milli `json:"softBudget" form:"softBudget"`
	BudgetId            string  `json:"budgetId" form:"budgetId"`
	TpmLimit            int64   `json:"tpmLimit" form:"tpmLimit"`
	RpmLimit            int64   `json:"rpmLimit" form:"rpmLimit"`
	MaxParallelRequests int     `json:"maxParallelRequests" form:"maxParallelRequests"`
	BudgetDuration      string  `json:"budgetDuration" form:"budgetDuration"`
	AllowedModels       string  `json:"allowedModels" form:"allowedModels"`
	AllowedRoutes       string  `json:"allowedRoutes" form:"allowedRoutes"`
	Expires             string  `json:"expires" form:"expires"` // RFC3339
	Blocked             uint    `json:"blocked" form:"blocked"`
}
type LLMKeyAddReq struct{ *LLMKeyReq }
type LLMKeyEditReq struct {
	TokenHash string `json:"tokenHash" form:"tokenHash" binding:"required"`
	*LLMKeyReq
}
type LLMKeyCreateRes struct {
	TokenHash    string `json:"tokenHash"`
	Key          string `json:"key"`          // 明文，仅此一次返回
	GraceMinutes int    `json:"graceMinutes"` // 轮换时旧 key 宽限分钟数（>0 表示旧 key 仍有效这么多分钟）
}

// ===== Dept LLM 配置 =====
type LLMDeptSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type LLMDeptSearchRes struct {
	ListRes
	List []*entity.LLMDeptProfile `json:"list"`
}
type LLMDeptReq struct {
	LlmModels              string `json:"llmModels" form:"llmModels"`
	LlmTpmLimit            int64  `json:"llmTpmLimit" form:"llmTpmLimit"`
	LlmRpmLimit            int64  `json:"llmRpmLimit" form:"llmRpmLimit"`
	LlmMaxParallelRequests int    `json:"llmMaxParallelRequests" form:"llmMaxParallelRequests"`
	LlmBudgetDuration      string `json:"llmBudgetDuration" form:"llmBudgetDuration"`
	LlmRouterSettings      string `json:"llmRouterSettings" form:"llmRouterSettings"`
	LlmBlocked             uint   `json:"llmBlocked" form:"llmBlocked"`
}
type LLMDeptEditReq struct {
	DeptId string `json:"deptId" form:"deptId" binding:"required"`
	*LLMDeptReq
}

// ===== Project =====
type LLMProjectSearchReq struct {
	Alias  string `json:"alias" form:"alias"`
	TeamId string `json:"teamId" form:"teamId"`
	PageReq
}
type LLMProjectSearchRes struct {
	ListRes
	List []*entity.LLMProject `json:"list"`
}
type LLMProjectReq struct {
	ProjectAlias string `json:"projectAlias" form:"projectAlias" binding:"required"`
	Description  string `json:"description" form:"description"`
	TeamId       string `json:"teamId" form:"teamId"`
	BudgetId     string `json:"budgetId" form:"budgetId"`
	Models       string `json:"models" form:"models"`
	Blocked      uint   `json:"blocked" form:"blocked"`
}
type LLMProjectAddReq struct{ *LLMProjectReq }
type LLMProjectEditReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	*LLMProjectReq
}

// ===== EndUser =====
type LLMEndUserSearchReq struct {
	Alias string `json:"alias" form:"alias"`
	PageReq
}
type LLMEndUserSearchRes struct {
	ListRes
	List []*entity.LLMEndUser `json:"list"`
}
type LLMEndUserReq struct {
	EndUserId          string `json:"endUserId" form:"endUserId" binding:"required"`
	Alias              string `json:"alias" form:"alias"`
	BudgetId           string `json:"budgetId" form:"budgetId"`
	AllowedModelRegion string `json:"allowedModelRegion" form:"allowedModelRegion"`
	DefaultModel       string `json:"defaultModel" form:"defaultModel"`
	Blocked            uint   `json:"blocked" form:"blocked"`
}
type LLMEndUserAddReq struct{ *LLMEndUserReq }
type LLMEndUserEditReq struct {
	EndUserId string `json:"endUserId" form:"endUserId" binding:"required"`
	*LLMEndUserReq
}

// ===== Tag =====
type LLMTagSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type LLMTagSearchRes struct {
	ListRes
	List []*entity.LLMTag `json:"list"`
}
type LLMTagReq struct {
	TagName     string `json:"tagName" form:"tagName" binding:"required"`
	Description string `json:"description" form:"description"`
	Models      string `json:"models" form:"models"`
	BudgetId    string `json:"budgetId" form:"budgetId"`
}
type LLMTagAddReq struct{ *LLMTagReq }
type LLMTagEditReq struct {
	TagName string `json:"tagName" form:"tagName" binding:"required"`
	*LLMTagReq
}

// ===== 日志（只读） =====
type LLMSpendLogSearchReq struct {
	UserId string `json:"userId" form:"userId"`
	TeamId string `json:"teamId" form:"teamId"`
	Model  string `json:"model" form:"model"`
	ApiKey string `json:"apiKey" form:"apiKey"`
	PageReq
}
type LLMSpendLogSearchRes struct {
	ListRes
	List []*entity.LLMSpendLog `json:"list"`
}
type LLMSpendAggregateReq struct {
	Dimension string `json:"dimension" form:"dimension"` // date/key/user/model
	PageReq
}
type LLMSpendAggregateRes struct {
	List   []*entity.LLMSpendAggregate `json:"list"`
	Totals *entity.LLMSpendTotals      `json:"totals"`
}
type LLMErrorLogSearchReq struct {
	Model  string `json:"model" form:"model"`
	ApiKey string `json:"apiKey" form:"apiKey"`
	PageReq
}
type LLMErrorLogSearchRes struct {
	ListRes
	List []*entity.LLMErrorLog `json:"list"`
}
type LLMAuditLogSearchReq struct {
	TableName string `json:"tableName" form:"tableName"`
	Action    string `json:"action" form:"action"`
	PageReq
}
type LLMAuditLogSearchRes struct {
	ListRes
	List []*entity.LLMAuditLog `json:"list"`
}
type LLMHealthCheckSearchReq struct {
	ModelName string `json:"modelName" form:"modelName"`
	Status    string `json:"status" form:"status"`
	PageReq
}
type LLMHealthCheckSearchRes struct {
	ListRes
	List []*entity.LLMHealthCheck `json:"list"`
}
type LLMDailySpendSearchReq struct {
	ScopeType string `json:"scopeType" form:"scopeType"`
	ScopeId   string `json:"scopeId" form:"scopeId"`
	PageReq
}
type LLMDailySpendSearchRes struct {
	ListRes
	List []*entity.LLMDailySpend `json:"list"`
}
type LLMPromptSearchReq struct {
	Name        string `json:"name" form:"name"`
	Environment string `json:"environment" form:"environment"`
	PageReq
}
type LLMPromptSearchRes struct {
	ListRes
	List []*entity.LLMPrompt `json:"list"`
}
