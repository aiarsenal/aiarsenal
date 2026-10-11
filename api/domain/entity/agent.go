package entity

import "aiarsenal/domain/types"

// Agent 模块实体（agent 库，表前缀 aa_agent_）。
// Agent 模块 schema（PostgreSQL + aa_agent_ 前缀）；
// jsonb 列用 string + gorm type:jsonb（沿用 llm.go 写法）。
// 跨库软引用：user_id → aiarsenal.aa_user.user_id（varchar(32) biz_id，无实际 FK）。

// ============ 配置中心 ============

// AgentConfigResource 可复用 OpenClaw channel 资源。
type AgentConfigResource struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ConfigResourceId string      `gorm:"column:config_resource_id;uniqueIndex" json:"configResourceId"`
	UserId           string      `gorm:"column:user_id" json:"userId"`
	ResourceType     string      `gorm:"column:resource_type" json:"resourceType"`
	ResourceKey      string      `gorm:"column:resource_key" json:"resourceKey"`
	Name             string      `gorm:"column:name" json:"name"`
	Description      string      `gorm:"column:description" json:"description"`
	Enabled          uint        `gorm:"column:enabled" json:"enabled"`
	Version          int         `gorm:"column:version" json:"version"`
	TagsJson         string      `gorm:"column:tags_json;type:jsonb" json:"tagsJson"`
	ContentJson      string      `gorm:"column:content_json;type:jsonb" json:"contentJson"`
	CreatedBy        string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy        string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt        *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentConfigResource) TableName() string { return "aa_agent_config_resource" }

// AgentConfigBundle 资源组合包。
type AgentConfigBundle struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ConfigBundleId string      `gorm:"column:config_bundle_id;uniqueIndex" json:"configBundleId"`
	UserId         string      `gorm:"column:user_id" json:"userId"`
	Name           string      `gorm:"column:name" json:"name"`
	Description    string      `gorm:"column:description" json:"description"`
	Enabled        uint        `gorm:"column:enabled" json:"enabled"`
	Version        int         `gorm:"column:version" json:"version"`
	CreatedBy      string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy      string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentConfigBundle) TableName() string { return "aa_agent_config_bundle" }

// AgentConfigBundleItem bundle 内 resource 成员项。
type AgentConfigBundleItem struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BundleItemId string      `gorm:"column:bundle_item_id;uniqueIndex" json:"bundleItemId"`
	BundleId     string      `gorm:"column:bundle_id" json:"bundleId"`
	ResourceId   string      `gorm:"column:resource_id" json:"resourceId"`
	SortOrder    int         `gorm:"column:sort_order" json:"sortOrder"`
	Required     uint        `gorm:"column:required" json:"required"`
	CreatedBy    string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentConfigBundleItem) TableName() string { return "aa_agent_config_bundle_item" }

// AgentConfigBundleSkill bundle 内 skill 成员项。
type AgentConfigBundleSkill struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BundleSkillId string      `gorm:"column:bundle_skill_id;uniqueIndex" json:"bundleSkillId"`
	BundleId      string      `gorm:"column:bundle_id" json:"bundleId"`
	SkillId       string      `gorm:"column:skill_id" json:"skillId"`
	SortOrder     int         `gorm:"column:sort_order" json:"sortOrder"`
	Required      uint        `gorm:"column:required" json:"required"`
	CreatedBy     string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentConfigBundleSkill) TableName() string { return "aa_agent_config_bundle_skill" }

// ============ 安全扫描 ============

// AgentSecurityScanConfig 扫描全局配置。
type AgentSecurityScanConfig struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ScanConfigId        string      `gorm:"column:scan_config_id;uniqueIndex" json:"scanConfigId"`
	DefaultMode         string      `gorm:"column:default_mode" json:"defaultMode"`
	QuickAnalyzersJson  string      `gorm:"column:quick_analyzers_json;type:jsonb" json:"quickAnalyzersJson"`
	DeepAnalyzersJson   string      `gorm:"column:deep_analyzers_json;type:jsonb" json:"deepAnalyzersJson"`
	QuickTimeoutSeconds int         `gorm:"column:quick_timeout_seconds" json:"quickTimeoutSeconds"`
	DeepTimeoutSeconds  int         `gorm:"column:deep_timeout_seconds" json:"deepTimeoutSeconds"`
	AllowFallback       uint        `gorm:"column:allow_fallback" json:"allowFallback"`
	UpdatedBy           string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt           *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentSecurityScanConfig) TableName() string { return "aa_agent_security_scan_config" }

// AgentSecurityScanJob 扫描任务。
type AgentSecurityScanJob struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ScanJobId       string      `gorm:"column:scan_job_id;uniqueIndex" json:"scanJobId"`
	AssetType       string      `gorm:"column:asset_type" json:"assetType"`
	ScanMode        string      `gorm:"column:scan_mode" json:"scanMode"`
	Status          uint        `gorm:"column:status" json:"status"`
	RequestedBy     string      `gorm:"column:requested_by" json:"requestedBy"`
	ScopeJson       string      `gorm:"column:scope_json;type:jsonb" json:"scopeJson"`
	TotalItems      int         `gorm:"column:total_items" json:"totalItems"`
	CompletedItems  int         `gorm:"column:completed_items" json:"completedItems"`
	FailedItems     int         `gorm:"column:failed_items" json:"failedItems"`
	CurrentItemName string      `gorm:"column:current_item_name" json:"currentItemName"`
	StartedAt       *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt      *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	CreatedBy       string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentSecurityScanJob) TableName() string { return "aa_agent_security_scan_job" }

// AgentSecurityScanJobItem 扫描任务项。
type AgentSecurityScanJobItem struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	JobItemId    string      `gorm:"column:job_item_id;uniqueIndex" json:"jobItemId"`
	JobId        string      `gorm:"column:job_id" json:"jobId"`
	AssetType    string      `gorm:"column:asset_type" json:"assetType"`
	AssetId      string      `gorm:"column:asset_id" json:"assetId"`
	AssetName    string      `gorm:"column:asset_name" json:"assetName"`
	Status       uint        `gorm:"column:status" json:"status"`
	ProgressPct  int         `gorm:"column:progress_pct" json:"progressPct"`
	RiskLevel    uint        `gorm:"column:risk_level" json:"riskLevel"`
	Summary      string      `gorm:"column:summary" json:"summary"`
	ScanResultId string      `gorm:"column:scan_result_id" json:"scanResultId"`
	CachedResult uint        `gorm:"column:cached_result" json:"cachedResult"`
	ErrorMessage string      `gorm:"column:error_message" json:"errorMessage"`
	StartedAt    *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt   *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentSecurityScanJobItem) TableName() string { return "aa_agent_security_scan_job_item" }

// AgentSecurityScanReport 扫描报告。
type AgentSecurityScanReport struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ScanReportId string      `gorm:"column:scan_report_id;uniqueIndex" json:"scanReportId"`
	JobId        string      `gorm:"column:job_id" json:"jobId"`
	SummaryJson  string      `gorm:"column:summary_json;type:jsonb" json:"summaryJson"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentSecurityScanReport) TableName() string { return "aa_agent_security_scan_report" }

// ============ AI Gateway 治理（模型注册 + 风险规则 + 只读记录） ============

// AgentModel admin 模型注册（api_key 加密密文）。
type AgentModel struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AgentModelId      string      `gorm:"column:agent_model_id;uniqueIndex" json:"agentModelId"`
	DisplayName       string      `gorm:"column:display_name" json:"displayName"`
	Description       string      `gorm:"column:description" json:"description"`
	ProviderType      string      `gorm:"column:provider_type" json:"providerType"`
	ProtocolType      string      `gorm:"column:protocol_type" json:"protocolType"`
	BaseUrl           string      `gorm:"column:base_url" json:"baseUrl"`
	ProviderModelName string      `gorm:"column:provider_model_name" json:"providerModelName"`
	ApiKey            string      `gorm:"column:api_key" json:"-"` // 加密密文，不外露
	ApiKeySecretRef   string      `gorm:"column:api_key_secret_ref" json:"apiKeySecretRef"`
	IsSecure          uint        `gorm:"column:is_secure" json:"isSecure"`
	IsActive          uint        `gorm:"column:is_active" json:"isActive"`
	InputPrice        float64     `gorm:"column:input_price" json:"inputPrice"`
	OutputPrice       float64     `gorm:"column:output_price" json:"outputPrice"`
	Currency          string      `gorm:"column:currency" json:"currency"`
	CreatedBy         string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy         string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt         *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentModel) TableName() string { return "aa_agent_model" }

// AgentRiskRule admin 风险规则。
type AgentRiskRule struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RiskRuleId  string      `gorm:"column:risk_rule_id;uniqueIndex" json:"riskRuleId"`
	RuleId      string      `gorm:"column:rule_id" json:"ruleId"`
	DisplayName string      `gorm:"column:display_name" json:"displayName"`
	Description string      `gorm:"column:description" json:"description"`
	Pattern     string      `gorm:"column:pattern" json:"pattern"`
	Severity    uint        `gorm:"column:severity" json:"severity"`
	Action      string      `gorm:"column:action" json:"action"`
	IsEnabled   uint        `gorm:"column:is_enabled" json:"isEnabled"`
	SortOrder   int         `gorm:"column:sort_order" json:"sortOrder"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentRiskRule) TableName() string { return "aa_agent_risk_rule" }

// AgentModelInvocation 模型调用记录（只读）。
type AgentModelInvocation struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InvocationId        string      `gorm:"column:invocation_id;uniqueIndex" json:"invocationId"`
	TraceId             string      `gorm:"column:trace_id" json:"traceId"`
	SessionId           string      `gorm:"column:session_id" json:"sessionId"`
	RequestId           string      `gorm:"column:request_id" json:"requestId"`
	UserId              string      `gorm:"column:user_id" json:"userId"`
	InstanceId          string      `gorm:"column:instance_id" json:"instanceId"`
	InstanceMode        string      `gorm:"column:instance_mode" json:"instanceMode"`
	RuntimeType         string      `gorm:"column:runtime_type" json:"runtimeType"`
	GatewayId           string      `gorm:"column:gateway_id" json:"gatewayId"`
	RuntimePodId        string      `gorm:"column:runtime_pod_id" json:"runtimePodId"`
	ModelId             string      `gorm:"column:model_id" json:"modelId"`
	ProviderType        string      `gorm:"column:provider_type" json:"providerType"`
	RequestedModel      string      `gorm:"column:requested_model" json:"requestedModel"`
	ActualProviderModel string      `gorm:"column:actual_provider_model" json:"actualProviderModel"`
	TrafficClass        string      `gorm:"column:traffic_class" json:"trafficClass"`
	RequestPayload      string      `gorm:"column:request_payload;type:jsonb" json:"requestPayload"`
	ResponsePayload     string      `gorm:"column:response_payload;type:jsonb" json:"responsePayload"`
	PromptTokens        int         `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens    int         `gorm:"column:completion_tokens" json:"completionTokens"`
	TotalTokens         int         `gorm:"column:total_tokens" json:"totalTokens"`
	CachedTokens        int         `gorm:"column:cached_tokens" json:"cachedTokens"`
	ReasoningTokens     int         `gorm:"column:reasoning_tokens" json:"reasoningTokens"`
	LatencyMs           int         `gorm:"column:latency_ms" json:"latencyMs"`
	IsStreaming         uint        `gorm:"column:is_streaming" json:"isStreaming"`
	Status              string      `gorm:"column:status" json:"status"`
	ErrorMessage        string      `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
	CompletedAt         *types.Time `gorm:"column:completed_at" json:"completedAt"`
}

func (AgentModelInvocation) TableName() string { return "aa_agent_model_invocation" }

// AgentAuditEvent 治理审计事件（只读）。
type AgentAuditEvent struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AuditEventId string      `gorm:"column:audit_event_id;uniqueIndex" json:"auditEventId"`
	TraceId      string      `gorm:"column:trace_id" json:"traceId"`
	SessionId    string      `gorm:"column:session_id" json:"sessionId"`
	RequestId    string      `gorm:"column:request_id" json:"requestId"`
	UserId       string      `gorm:"column:user_id" json:"userId"`
	InstanceId   string      `gorm:"column:instance_id" json:"instanceId"`
	InstanceMode string      `gorm:"column:instance_mode" json:"instanceMode"`
	RuntimeType  string      `gorm:"column:runtime_type" json:"runtimeType"`
	GatewayId    string      `gorm:"column:gateway_id" json:"gatewayId"`
	RuntimePodId string      `gorm:"column:runtime_pod_id" json:"runtimePodId"`
	InvocationId string      `gorm:"column:invocation_id" json:"invocationId"`
	EventType    string      `gorm:"column:event_type" json:"eventType"`
	TrafficClass string      `gorm:"column:traffic_class" json:"trafficClass"`
	Severity     uint        `gorm:"column:severity" json:"severity"`
	Message      string      `gorm:"column:message" json:"message"`
	Details      string      `gorm:"column:details;type:jsonb" json:"details"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentAuditEvent) TableName() string { return "aa_agent_audit_event" }

// AgentCostRecord 成本记录（只读）。
type AgentCostRecord struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	CostRecordId     string      `gorm:"column:cost_record_id;uniqueIndex" json:"costRecordId"`
	TraceId          string      `gorm:"column:trace_id" json:"traceId"`
	SessionId        string      `gorm:"column:session_id" json:"sessionId"`
	RequestId        string      `gorm:"column:request_id" json:"requestId"`
	UserId           string      `gorm:"column:user_id" json:"userId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	InstanceMode     string      `gorm:"column:instance_mode" json:"instanceMode"`
	RuntimeType      string      `gorm:"column:runtime_type" json:"runtimeType"`
	GatewayId        string      `gorm:"column:gateway_id" json:"gatewayId"`
	RuntimePodId     string      `gorm:"column:runtime_pod_id" json:"runtimePodId"`
	InvocationId     string      `gorm:"column:invocation_id" json:"invocationId"`
	ModelId          string      `gorm:"column:model_id" json:"modelId"`
	ProviderType     string      `gorm:"column:provider_type" json:"providerType"`
	ModelName        string      `gorm:"column:model_name" json:"modelName"`
	Currency         string      `gorm:"column:currency" json:"currency"`
	PromptTokens     int         `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens int         `gorm:"column:completion_tokens" json:"completionTokens"`
	TotalTokens      int         `gorm:"column:total_tokens" json:"totalTokens"`
	InputUnitPrice   float64     `gorm:"column:input_unit_price" json:"inputUnitPrice"`
	OutputUnitPrice  float64     `gorm:"column:output_unit_price" json:"outputUnitPrice"`
	EstimatedCost    float64     `gorm:"column:estimated_cost" json:"estimatedCost"`
	ActualCost       float64     `gorm:"column:actual_cost" json:"actualCost"`
	InternalCost     float64     `gorm:"column:internal_cost" json:"internalCost"`
	RecordedAt       *types.Time `gorm:"column:recorded_at" json:"recordedAt"`
}

func (AgentCostRecord) TableName() string { return "aa_agent_cost_record" }

// AgentRiskHit 风险命中（只读）。
type AgentRiskHit struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RiskHitId    string      `gorm:"column:risk_hit_id;uniqueIndex" json:"riskHitId"`
	TraceId      string      `gorm:"column:trace_id" json:"traceId"`
	SessionId    string      `gorm:"column:session_id" json:"sessionId"`
	RequestId    string      `gorm:"column:request_id" json:"requestId"`
	UserId       string      `gorm:"column:user_id" json:"userId"`
	InstanceId   string      `gorm:"column:instance_id" json:"instanceId"`
	InstanceMode string      `gorm:"column:instance_mode" json:"instanceMode"`
	RuntimeType  string      `gorm:"column:runtime_type" json:"runtimeType"`
	GatewayId    string      `gorm:"column:gateway_id" json:"gatewayId"`
	RuntimePodId string      `gorm:"column:runtime_pod_id" json:"runtimePodId"`
	InvocationId string      `gorm:"column:invocation_id" json:"invocationId"`
	RuleId       string      `gorm:"column:rule_id" json:"ruleId"`
	RuleName     string      `gorm:"column:rule_name" json:"ruleName"`
	Severity     uint        `gorm:"column:severity" json:"severity"`
	Action       string      `gorm:"column:action" json:"action"`
	MatchSummary string      `gorm:"column:match_summary" json:"matchSummary"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentRiskHit) TableName() string { return "aa_agent_risk_hit" }

// ============ 审计 / 配额 ============

// AgentAuditLog 用户操作审计日志。
type AgentAuditLog struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AuditLogId   string      `gorm:"column:audit_log_id;uniqueIndex" json:"auditLogId"`
	UserId       string      `gorm:"column:user_id" json:"userId"`
	Action       string      `gorm:"column:action" json:"action"`
	ResourceType string      `gorm:"column:resource_type" json:"resourceType"`
	ResourceId   string      `gorm:"column:resource_id" json:"resourceId"`
	Details      string      `gorm:"column:details;type:jsonb" json:"details"`
	IpAddress    string      `gorm:"column:ip_address" json:"ipAddress"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentAuditLog) TableName() string { return "aa_agent_audit_log" }

// AgentUserQuota 用户配额（软引用 aa_user）。
type AgentUserQuota struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	UserQuotaId  string      `gorm:"column:user_quota_id;uniqueIndex" json:"userQuotaId"`
	UserId       string      `gorm:"column:user_id;uniqueIndex" json:"userId"`
	MaxInstances int         `gorm:"column:max_instances" json:"maxInstances"`
	MaxCpuCores  float64     `gorm:"column:max_cpu_cores" json:"maxCpuCores"`
	MaxMemoryGb  int         `gorm:"column:max_memory_gb" json:"maxMemoryGb"`
	MaxStorageGb int         `gorm:"column:max_storage_gb" json:"maxStorageGb"`
	MaxGpuCount  int         `gorm:"column:max_gpu_count" json:"maxGpuCount"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentUserQuota) TableName() string { return "aa_agent_user_quota" }
