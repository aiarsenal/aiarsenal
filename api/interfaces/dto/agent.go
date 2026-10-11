package dto

import "aiarsenal/domain/entity"

// Agent 模块请求/响应 DTO（镜像 llm.go 写法）。

// ===== 通用 =====
type AgentIdsReq struct {
	Ids []string `json:"ids" form:"ids"`
}

// ===== 配置中心 =====
type AgentConfigResourceSearchReq struct {
	Name         string `json:"name" form:"name"`
	ResourceType string `json:"resourceType" form:"resourceType"`
	PageReq
}
type AgentConfigResourceSearchRes struct {
	ListRes
	List []*entity.AgentConfigResource `json:"list"`
}
type AgentConfigResourceReq struct {
	ResourceType string `json:"resourceType" form:"resourceType" binding:"required"`
	ResourceKey  string `json:"resourceKey" form:"resourceKey"`
	Name         string `json:"name" form:"name" binding:"required"`
	Description  string `json:"description" form:"description"`
	Enabled      uint   `json:"enabled" form:"enabled"`
	Version      int    `json:"version" form:"version"`
	TagsJson     string `json:"tagsJson" form:"tagsJson"`
	ContentJson  string `json:"contentJson" form:"contentJson"`
}
type AgentConfigResourceAddReq struct{ *AgentConfigResourceReq }
type AgentConfigResourceEditReq struct {
	ConfigResourceId string `json:"configResourceId" form:"configResourceId" binding:"required"`
	*AgentConfigResourceReq
}

type AgentConfigBundleSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type AgentConfigBundleSearchRes struct {
	ListRes
	List []*entity.AgentConfigBundle `json:"list"`
}
type AgentConfigBundleReq struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Description string `json:"description" form:"description"`
	Enabled     uint   `json:"enabled" form:"enabled"`
	Version     int    `json:"version" form:"version"`
}
type AgentConfigBundleAddReq struct{ *AgentConfigBundleReq }
type AgentConfigBundleEditReq struct {
	ConfigBundleId string `json:"configBundleId" form:"configBundleId" binding:"required"`
	*AgentConfigBundleReq
}

type AgentConfigBundleItemSearchReq struct {
	BundleId string `json:"bundleId" form:"bundleId"`
	PageReq
}
type AgentConfigBundleItemSearchRes struct {
	ListRes
	List []*entity.AgentConfigBundleItem `json:"list"`
}
type AgentConfigBundleItemReq struct {
	BundleId   string `json:"bundleId" form:"bundleId" binding:"required"`
	ResourceId string `json:"resourceId" form:"resourceId" binding:"required"`
	SortOrder  int    `json:"sortOrder" form:"sortOrder"`
	Required   uint   `json:"required" form:"required"`
}
type AgentConfigBundleItemAddReq struct{ *AgentConfigBundleItemReq }
type AgentConfigBundleItemEditReq struct {
	BundleItemId string `json:"bundleItemId" form:"bundleItemId" binding:"required"`
	*AgentConfigBundleItemReq
}

type AgentConfigBundleSkillSearchReq struct {
	BundleId string `json:"bundleId" form:"bundleId"`
	PageReq
}
type AgentConfigBundleSkillSearchRes struct {
	ListRes
	List []*entity.AgentConfigBundleSkill `json:"list"`
}
type AgentConfigBundleSkillReq struct {
	BundleId  string `json:"bundleId" form:"bundleId" binding:"required"`
	SkillId   string `json:"skillId" form:"skillId" binding:"required"`
	SortOrder int    `json:"sortOrder" form:"sortOrder"`
	Required  uint   `json:"required" form:"required"`
}
type AgentConfigBundleSkillAddReq struct{ *AgentConfigBundleSkillReq }
type AgentConfigBundleSkillEditReq struct {
	BundleSkillId string `json:"bundleSkillId" form:"bundleSkillId" binding:"required"`
	*AgentConfigBundleSkillReq
}

// ===== 安全扫描 =====
type AgentScanConfigSearchReq struct {
	PageReq
}
type AgentScanConfigSearchRes struct {
	ListRes
	List []*entity.AgentSecurityScanConfig `json:"list"`
}
type AgentScanConfigReq struct {
	DefaultMode         string `json:"defaultMode" form:"defaultMode"`
	QuickAnalyzersJson  string `json:"quickAnalyzersJson" form:"quickAnalyzersJson"`
	DeepAnalyzersJson   string `json:"deepAnalyzersJson" form:"deepAnalyzersJson"`
	QuickTimeoutSeconds int    `json:"quickTimeoutSeconds" form:"quickTimeoutSeconds"`
	DeepTimeoutSeconds  int    `json:"deepTimeoutSeconds" form:"deepTimeoutSeconds"`
	AllowFallback       uint   `json:"allowFallback" form:"allowFallback"`
}
type AgentScanConfigAddReq struct{ *AgentScanConfigReq }
type AgentScanConfigEditReq struct {
	ScanConfigId string `json:"scanConfigId" form:"scanConfigId" binding:"required"`
	*AgentScanConfigReq
}

type AgentScanJobSearchReq struct {
	AssetType string `json:"assetType" form:"assetType"`
	Status    string `json:"status" form:"status"`
	PageReq
}
type AgentScanJobSearchRes struct {
	ListRes
	List []*entity.AgentSecurityScanJob `json:"list"`
}
type AgentScanJobReq struct {
	AssetType  string `json:"assetType" form:"assetType" binding:"required"`
	ScanMode   string `json:"scanMode" form:"scanMode"`
	Status     uint   `json:"status" form:"status"`
	ScopeJson  string `json:"scopeJson" form:"scopeJson"`
	TotalItems int    `json:"totalItems" form:"totalItems"`
}
type AgentScanJobAddReq struct{ *AgentScanJobReq }
type AgentScanJobEditReq struct {
	ScanJobId string `json:"scanJobId" form:"scanJobId" binding:"required"`
	*AgentScanJobReq
}

type AgentScanJobItemSearchReq struct {
	JobId string `json:"jobId" form:"jobId"`
	PageReq
}
type AgentScanJobItemSearchRes struct {
	ListRes
	List []*entity.AgentSecurityScanJobItem `json:"list"`
}

type AgentScanReportSearchReq struct {
	JobId string `json:"jobId" form:"jobId"`
	PageReq
}
type AgentScanReportSearchRes struct {
	ListRes
	List []*entity.AgentSecurityScanReport `json:"list"`
}

// ===== AI Gateway 治理 =====
type AgentModelSearchReq struct {
	Name         string `json:"name" form:"name"`
	ProviderType string `json:"providerType" form:"providerType"`
	PageReq
}
type AgentModelSearchRes struct {
	ListRes
	List []*entity.AgentModel `json:"list"`
}
type AgentModelReq struct {
	DisplayName       string  `json:"displayName" form:"displayName" binding:"required"`
	Description       string  `json:"description" form:"description"`
	ProviderType      string  `json:"providerType" form:"providerType" binding:"required"`
	ProtocolType      string  `json:"protocolType" form:"protocolType"`
	BaseUrl           string  `json:"baseUrl" form:"baseUrl"`
	ProviderModelName string  `json:"providerModelName" form:"providerModelName" binding:"required"`
	ApiKey            string  `json:"apiKey" form:"apiKey"` // 明文，service 加密落库
	ApiKeySecretRef   string  `json:"apiKeySecretRef" form:"apiKeySecretRef"`
	IsSecure          uint    `json:"isSecure" form:"isSecure"`
	IsActive          uint    `json:"isActive" form:"isActive"`
	InputPrice        float64 `json:"inputPrice" form:"inputPrice"`
	OutputPrice       float64 `json:"outputPrice" form:"outputPrice"`
	Currency          string  `json:"currency" form:"currency"`
}
type AgentModelAddReq struct{ *AgentModelReq }
type AgentModelEditReq struct {
	AgentModelId string `json:"agentModelId" form:"agentModelId" binding:"required"`
	*AgentModelReq
}

type AgentRiskRuleSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type AgentRiskRuleSearchRes struct {
	ListRes
	List []*entity.AgentRiskRule `json:"list"`
}
type AgentRiskRuleReq struct {
	RuleId      string `json:"ruleId" form:"ruleId" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
	Description string `json:"description" form:"description"`
	Pattern     string `json:"pattern" form:"pattern" binding:"required"`
	Severity    uint   `json:"severity" form:"severity"`
	Action      string `json:"action" form:"action"`
	IsEnabled   uint   `json:"isEnabled" form:"isEnabled"`
	SortOrder   int    `json:"sortOrder" form:"sortOrder"`
}
type AgentRiskRuleAddReq struct{ *AgentRiskRuleReq }
type AgentRiskRuleEditReq struct {
	RiskRuleId string `json:"riskRuleId" form:"riskRuleId" binding:"required"`
	*AgentRiskRuleReq
}

// 只读治理记录
type AgentInvocationSearchReq struct {
	UserId  string `json:"userId" form:"userId"`
	ModelId string `json:"modelId" form:"modelId"`
	PageReq
}
type AgentInvocationSearchRes struct {
	ListRes
	List []*entity.AgentModelInvocation `json:"list"`
}
type AgentAuditEventSearchReq struct {
	UserId    string `json:"userId" form:"userId"`
	EventType string `json:"eventType" form:"eventType"`
	PageReq
}
type AgentAuditEventSearchRes struct {
	ListRes
	List []*entity.AgentAuditEvent `json:"list"`
}
type AgentCostRecordSearchReq struct {
	UserId string `json:"userId" form:"userId"`
	PageReq
}
type AgentCostRecordSearchRes struct {
	ListRes
	List []*entity.AgentCostRecord `json:"list"`
}
type AgentRiskHitSearchReq struct {
	RuleId string `json:"ruleId" form:"ruleId"`
	PageReq
}
type AgentRiskHitSearchRes struct {
	ListRes
	List []*entity.AgentRiskHit `json:"list"`
}

// ===== 审计 / 配额 =====
type AgentAuditLogSearchReq struct {
	UserId string `json:"userId" form:"userId"`
	Action string `json:"action" form:"action"`
	PageReq
}
type AgentAuditLogSearchRes struct {
	ListRes
	List []*entity.AgentAuditLog `json:"list"`
}
type AgentUserQuotaSearchRes struct {
	ListRes
	List []*entity.AgentUserQuota `json:"list"`
}
type AgentUserQuotaReq struct {
	UserId       string  `json:"userId" form:"userId" binding:"required"`
	MaxInstances int     `json:"maxInstances" form:"maxInstances"`
	MaxCpuCores  float64 `json:"maxCpuCores" form:"maxCpuCores"`
	MaxMemoryGb  int     `json:"maxMemoryGb" form:"maxMemoryGb"`
	MaxStorageGb int     `json:"maxStorageGb" form:"maxStorageGb"`
	MaxGpuCount  int     `json:"maxGpuCount" form:"maxGpuCount"`
}
type AgentUserQuotaAddReq struct{ *AgentUserQuotaReq }
type AgentUserQuotaEditReq struct {
	UserQuotaId string `json:"userQuotaId" form:"userQuotaId" binding:"required"`
	*AgentUserQuotaReq
}
