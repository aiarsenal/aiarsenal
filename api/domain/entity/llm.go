package entity

import "aiarsenal/domain/types"

// LLM 网关模块实体（llm 库，表前缀 aa_llm_）。
// LLM 模块 schema，按本期范围裁剪；jsonb 列用 json.RawMessage + type:jsonb
// （沿用 skill_version.go 的写法）。跨库软引用：user_id→aiarsenal.aa_user；org_id→aa_dept。

// ============ 凭据 ============
type LLMCredential struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	CredentialId     string      `gorm:"column:credential_id;uniqueIndex" json:"credentialId"`
	CredentialName   string      `gorm:"column:credential_name" json:"credentialName"`
	ProviderType     string      `gorm:"column:provider_type" json:"providerType"`
	BaseURL          string      `gorm:"column:base_url" json:"baseUrl"`
	CredentialValues string      `gorm:"column:credential_values;type:text" json:"-"` // AES-GCM 密文（hex），不外露
	CredentialInfo   string      `gorm:"column:credential_info;type:text" json:"credentialInfo"`
	Status           uint        `gorm:"column:status" json:"status"`
	CreatedBy        string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy        string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt        *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMCredential) TableName() string { return "aa_llm_credential" }

// ============ 模型部署 ============
type LLMModel struct {
	Id                         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ModelId                    string      `gorm:"column:model_id;uniqueIndex" json:"modelId"`
	ModelName                  string      `gorm:"column:model_name" json:"modelName"`
	ModelGroup                 string      `gorm:"column:model_group" json:"modelGroup"`
	Provider                   string      `gorm:"column:provider" json:"provider"`
	ProviderModel              string      `gorm:"column:provider_model" json:"providerModel"`
	CredentialId               string      `gorm:"column:credential_id" json:"credentialId"`
	ExtraParams                string      `gorm:"column:extra_params;type:text" json:"extraParams"`
	ModelInfo                  string      `gorm:"column:model_info;type:text" json:"modelInfo"`
	InputPricePerMillion         types.Milli `gorm:"column:input_price_per_million" json:"inputPricePerMillion"`         // 输入单价（毫/百万 tokens；1 毫 = $0.0001，JSON 边界自动换算回 $/M）
	OutputPricePerMillion        types.Milli `gorm:"column:output_price_per_million" json:"outputPricePerMillion"`       // 输出单价（毫/百万 tokens）
	CacheReadPricePerMillion     types.Milli `gorm:"column:cache_read_price_per_million" json:"cacheReadPricePerMillion"` // 缓存读单价（毫/百万 tokens）
	CacheCreationPricePerMillion types.Milli `gorm:"column:cache_creation_price_per_million" json:"cacheCreationPricePerMillion"` // 缓存写单价（毫/百万 tokens）
	MaxInputTokens             int         `gorm:"column:max_input_tokens" json:"maxInputTokens"`
	MaxOutputTokens            int         `gorm:"column:max_output_tokens" json:"maxOutputTokens"`
	TpmLimit                   int64       `gorm:"column:tpm_limit" json:"tpmLimit"`
	RpmLimit                   int64       `gorm:"column:rpm_limit" json:"rpmLimit"`
	Weight                     int         `gorm:"column:weight" json:"weight"`
	Blocked                    uint        `gorm:"column:blocked" json:"blocked"`
	Status                     uint        `gorm:"column:status" json:"status"`
	CreatedBy                  string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy                  string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt                  *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                  *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt                  *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMModel) TableName() string { return "aa_llm_model" }

// ============ 模型分组（字典，软引用 aa_llm_model.model_group）============
// group_name 即模型 model_group 列存的值（同名分组做负载均衡/故障转移）；
// 本表只维护分组字典与启停，网关仍按 model_group 字符串路由，无强外键。
type LLMModelGroup struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	GroupId     string      `gorm:"column:group_id;uniqueIndex" json:"groupId"`
	GroupName   string      `gorm:"column:group_name" json:"groupName"`
	Description string      `gorm:"column:description;type:text" json:"description"`
	Status      uint        `gorm:"column:status" json:"status"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMModelGroup) TableName() string { return "aa_llm_model_group" }

// ============ 预算 ============
type LLMBudget struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BudgetId            string      `gorm:"column:budget_id;uniqueIndex" json:"budgetId"`
	BudgetName          string      `gorm:"column:budget_name" json:"budgetName"`
	Spend               types.Milli `gorm:"column:spend" json:"spend"` // 当前窗口已消费（毫，网关计费累计，窗口懒滚存清零）
	MaxBudget           *types.Milli `gorm:"column:max_budget" json:"maxBudget"`
	SoftBudget          *types.Milli `gorm:"column:soft_budget" json:"softBudget"`
	MaxParallelRequests *int        `gorm:"column:max_parallel_requests" json:"maxParallelRequests"`
	TpmLimit            *int64      `gorm:"column:tpm_limit" json:"tpmLimit"`
	RpmLimit            *int64      `gorm:"column:rpm_limit" json:"rpmLimit"`
	ModelMaxBudget      string      `gorm:"column:model_max_budget;type:text" json:"modelMaxBudget"`
	BudgetDuration      string      `gorm:"column:budget_duration" json:"budgetDuration"`
	BudgetResetAt       *types.Time `gorm:"column:budget_reset_at" json:"budgetResetAt"`
	AllowedModels       string      `gorm:"column:allowed_models;type:text" json:"allowedModels"`
	GrantRules          string      `gorm:"column:grant_rules" json:"grantRules"` // 适用范围条件行 JSON（[]model.GrantRow，与 aa_menu.grant_rules 同构）
	CreatedBy           string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy           string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt           *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMBudget) TableName() string { return "aa_llm_budget" }

// 临时预算申请（LLMBudgetRequest/aa_llm_budget_request）已上收统一审核中心
// entity.ReviewOrder（主库 aa_review_order，order_type=llm_temp_budget），本实体与表废弃归档。

// ============ 虚拟密钥 ============
type LLMVirtualKey struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TokenHash           string      `gorm:"column:token_hash;uniqueIndex" json:"tokenHash"` // sha256(明文) hex，单向不可逆，作为密钥公开标识外露供前端引用（block/rotate/edit/delete 均以之为键）
	KeyAlias            string      `gorm:"column:key_alias" json:"keyAlias"`
	UserId              string      `gorm:"column:user_id" json:"userId"`
	TeamId              string      `gorm:"column:team_id" json:"teamId"`
	ProjectId           string      `gorm:"column:project_id" json:"projectId"`
	OrgId               string      `gorm:"column:org_id" json:"orgId"`
	Models              string      `gorm:"column:models;type:text" json:"models"`
	Aliases             string      `gorm:"column:aliases;type:text" json:"aliases"`
	Config              string      `gorm:"column:config;type:text" json:"config"`
	RouterSettings      string      `gorm:"column:router_settings;type:text" json:"routerSettings"`
	Permissions         string      `gorm:"column:permissions;type:text" json:"permissions"`
	Spend               types.Milli `gorm:"column:spend" json:"spend"`
	MaxBudget           *types.Milli `gorm:"column:max_budget" json:"maxBudget"`
	SoftBudget          *types.Milli `gorm:"column:soft_budget" json:"softBudget"`
	BudgetId            string      `gorm:"column:budget_id" json:"budgetId"`
	TpmLimit            *int64      `gorm:"column:tpm_limit" json:"tpmLimit"`
	RpmLimit            *int64      `gorm:"column:rpm_limit" json:"rpmLimit"`
	MaxParallelRequests *int        `gorm:"column:max_parallel_requests" json:"maxParallelRequests"`
	BudgetDuration      string      `gorm:"column:budget_duration" json:"budgetDuration"`
	BudgetResetAt       *types.Time `gorm:"column:budget_reset_at" json:"budgetResetAt"`
	ModelSpend          string      `gorm:"column:model_spend;type:text" json:"modelSpend"`
	ModelMaxBudget      string      `gorm:"column:model_max_budget;type:text" json:"modelMaxBudget"`
	AllowedModels       string      `gorm:"column:allowed_models;type:text" json:"allowedModels"`
	AllowedRoutes       string      `gorm:"column:allowed_routes;type:text" json:"allowedRoutes"`
	Expires             *types.Time `gorm:"column:expires" json:"expires"`
	Blocked             uint        `gorm:"column:blocked" json:"blocked"`
	LastActive          *types.Time `gorm:"column:last_active" json:"lastActive"`
	AutoRotate          uint        `gorm:"column:auto_rotate" json:"autoRotate"`
	RotationInterval    string      `gorm:"column:rotation_interval" json:"rotationInterval"`
	LastRotationAt      *types.Time `gorm:"column:last_rotation_at" json:"lastRotationAt"`
	KeyRotationAt       *types.Time `gorm:"column:key_rotation_at" json:"keyRotationAt"`
	RotationCount       int         `gorm:"column:rotation_count" json:"rotationCount"`
	DeprecatedTokenHash string      `gorm:"column:deprecated_token_hash" json:"deprecatedTokenHash"`
	DeprecateExpiresAt  *types.Time `gorm:"column:deprecate_expires_at" json:"deprecateExpiresAt"`
	Metadata            string      `gorm:"column:metadata;type:text" json:"metadata"`
	CreatedBy           string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy           string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt           *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMVirtualKey) TableName() string { return "aa_llm_virtual_key" }

// ============ 部门 LLM 配置（复用 aiarsenal.aa_dept，同表只读 LLM 扩展列） ============
// LeaderInfo 负责人姓名/工号（从 aa_user 单独读出回填，非持久化）。
type LeaderInfo struct {
	Name       string
	EmployeeNo string
}

// 团队实体 = 部门：aa_llm_virtual_key.team_id / aa_llm_project.team_id 软引用 aa_dept.dept_id。
// 本实体映射 aa_dept，仅暴露部门基础字段 + LLM 配置列；累计花费从 aa_llm_daily_spend 聚合，不存此表。
type LLMDeptProfile struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DeptId            string      `gorm:"column:dept_id" json:"deptId"`
	ParentId          string      `gorm:"column:parent_id" json:"parentId"`
	DeptName          string      `gorm:"column:dept_name" json:"deptName"`
	Ancestors         string      `gorm:"column:ancestors" json:"ancestors"`
	OrderNum          int         `gorm:"column:order_num" json:"orderNum"`
	LeaderId          string      `gorm:"column:leader_id" json:"leaderId"`
	Status            uint        `gorm:"column:status" json:"status"`
	LlmModels         string      `gorm:"column:llm_models;type:text" json:"llmModels"`
	LlmTpmLimit       *int64      `gorm:"column:llm_tpm_limit" json:"llmTpmLimit"`
	LlmRpmLimit       *int64      `gorm:"column:llm_rpm_limit" json:"llmRpmLimit"`
	LlmMaxParallelReq *int        `gorm:"column:llm_max_parallel_requests" json:"llmMaxParallelRequests"`
	LlmBudgetDuration string      `gorm:"column:llm_budget_duration" json:"llmBudgetDuration"`
	LlmBudgetResetAt  *types.Time `gorm:"column:llm_budget_reset_at" json:"llmBudgetResetAt"`
	LlmRouterSettings string      `gorm:"column:llm_router_settings;type:text" json:"llmRouterSettings"`
	LlmBlocked        uint        `gorm:"column:llm_blocked" json:"llmBlocked"`
	Spend             types.Milli `gorm:"column:spend" json:"spend"` // 由 service 从 daily_spend 聚合后回填，非列
	LeaderName        string      `gorm:"-" json:"leaderName"`       // join aa_user 取负责人姓名，非列
	LeaderEmpNo       string      `gorm:"-" json:"leaderEmpNo"`      // join aa_user 取工号，非列
	LevelPath         string      `gorm:"-" json:"levelPath"`        // service 把 ancestors 翻译成 部门名/部门名，非列
}

func (LLMDeptProfile) TableName() string { return "aa_dept" }

// ============ 项目 ============
type LLMProject struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ProjectId     string      `gorm:"column:project_id;uniqueIndex" json:"projectId"`
	ProjectAlias  string      `gorm:"column:project_alias" json:"projectAlias"`
	Description   string      `gorm:"column:description" json:"description"`
	TeamId        string      `gorm:"column:team_id" json:"teamId"`
	BudgetId      string      `gorm:"column:budget_id" json:"budgetId"`
	Models        string      `gorm:"column:models;type:text" json:"models"`
	ModelRpmLimit string      `gorm:"column:model_rpm_limit;type:text" json:"modelRpmLimit"`
	ModelTpmLimit string      `gorm:"column:model_tpm_limit;type:text" json:"modelTpmLimit"`
	Spend         types.Milli `gorm:"column:spend" json:"spend"`
	ModelSpend    string      `gorm:"column:model_spend;type:text" json:"modelSpend"`
	Blocked       uint        `gorm:"column:blocked" json:"blocked"`
	Metadata      string      `gorm:"column:metadata;type:text" json:"metadata"`
	CreatedBy     string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy     string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMProject) TableName() string { return "aa_llm_project" }

// ============ 终端用户 ============
type LLMEndUser struct {
	Id                 uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	EndUserId          string      `gorm:"column:end_user_id;uniqueIndex" json:"endUserId"`
	Alias              string      `gorm:"column:alias" json:"alias"`
	BudgetId           string      `gorm:"column:budget_id" json:"budgetId"`
	AllowedModelRegion string      `gorm:"column:allowed_model_region" json:"allowedModelRegion"`
	DefaultModel       string      `gorm:"column:default_model" json:"defaultModel"`
	Spend              types.Milli `gorm:"column:spend" json:"spend"`
	Blocked            uint        `gorm:"column:blocked" json:"blocked"`
	CreatedAt          *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LLMEndUser) TableName() string { return "aa_llm_end_user" }

// ============ 标签 ============
type LLMTag struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TagName     string      `gorm:"column:tag_name;uniqueIndex" json:"tagName"`
	Description string      `gorm:"column:description" json:"description"`
	Models      string      `gorm:"column:models;type:text" json:"models"`
	ModelInfo   string      `gorm:"column:model_info;type:text" json:"modelInfo"`
	Spend       types.Milli `gorm:"column:spend" json:"spend"`
	BudgetId    string      `gorm:"column:budget_id" json:"budgetId"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LLMTag) TableName() string { return "aa_llm_tag" }

// ============ 请求花费日志 ============
type LLMSpendLog struct {
	Id                       uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RequestId                string      `gorm:"column:request_id;uniqueIndex" json:"requestId"`
	CallType                 string      `gorm:"column:call_type" json:"callType"`
	ApiKey                   string      `gorm:"column:api_key" json:"apiKey"`
	Spend                    types.Milli `gorm:"column:spend" json:"spend"`
	TotalTokens              int         `gorm:"column:total_tokens" json:"totalTokens"`
	PromptTokens             int         `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens         int         `gorm:"column:completion_tokens" json:"completionTokens"`
	CacheReadInputTokens     int         `gorm:"column:cache_read_input_tokens" json:"cacheReadInputTokens"`
	CacheCreationInputTokens int         `gorm:"column:cache_creation_input_tokens" json:"cacheCreationInputTokens"`
	StartTime                *types.Time `gorm:"column:start_time" json:"startTime"`
	EndTime                  *types.Time `gorm:"column:end_time" json:"endTime"`
	RequestDurationMs        *int        `gorm:"column:request_duration_ms" json:"requestDurationMs"`
	CompletionStartTime      *types.Time `gorm:"column:completion_start_time" json:"completionStartTime"`
	Model                    string      `gorm:"column:model" json:"model"`
	ModelId                  string      `gorm:"column:model_id" json:"modelId"`
	ModelGroup               string      `gorm:"column:model_group" json:"modelGroup"`
	CustomLlmProvider        string      `gorm:"column:custom_llm_provider" json:"customLlmProvider"`
	ApiBase                  string      `gorm:"column:api_base" json:"apiBase"`
	UserId                   string      `gorm:"column:user_id" json:"userId"`
	TeamId                   string      `gorm:"column:team_id" json:"teamId"`
	OrgId                    string      `gorm:"column:org_id" json:"orgId"`
	ProjectId                string      `gorm:"column:project_id" json:"projectId"`
	EndUser                  string      `gorm:"column:end_user" json:"endUser"`
	SessionId                string      `gorm:"column:session_id" json:"sessionId"`
	RequesterIpAddress       string      `gorm:"column:requester_ip_address" json:"requesterIpAddress"`
	CacheHit                 string      `gorm:"column:cache_hit" json:"cacheHit"`
	CacheKey                 string      `gorm:"column:cache_key" json:"cacheKey"`
	RequestTags              string      `gorm:"column:request_tags;type:text" json:"requestTags"`
	Metadata                 string      `gorm:"column:metadata;type:text" json:"metadata"`
	Messages                 string      `gorm:"column:messages;type:text" json:"messages,omitempty"`
	Response                 string      `gorm:"column:response;type:text" json:"response,omitempty"`
	Status                   string      `gorm:"column:status" json:"status"`
}

func (LLMSpendLog) TableName() string { return "aa_llm_spend_log" }

// ============ 日聚合 ============
type LLMDailySpend struct {
	Id                       uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ScopeType                string      `gorm:"column:scope_type" json:"scopeType"`
	ScopeId                  string      `gorm:"column:scope_id" json:"scopeId"`
	Date                     string      `gorm:"column:date" json:"date"`
	ApiKey                   string      `gorm:"column:api_key" json:"apiKey"`
	Model                    string      `gorm:"column:model" json:"model"`
	ModelGroup               string      `gorm:"column:model_group" json:"modelGroup"`
	CustomLlmProvider        string      `gorm:"column:custom_llm_provider" json:"customLlmProvider"`
	Endpoint                 string      `gorm:"column:endpoint" json:"endpoint"`
	McpNamespacedToolName    string      `gorm:"column:mcp_namespaced_tool_name" json:"mcpNamespacedToolName"`
	PromptTokens             int64       `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens         int64       `gorm:"column:completion_tokens" json:"completionTokens"`
	CacheReadInputTokens     int64       `gorm:"column:cache_read_input_tokens" json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64       `gorm:"column:cache_creation_input_tokens" json:"cacheCreationInputTokens"`
	Spend                    types.Milli `gorm:"column:spend" json:"spend"`
	ApiRequests              int64       `gorm:"column:api_requests" json:"apiRequests"`
	SuccessfulRequests       int64       `gorm:"column:successful_requests" json:"successfulRequests"`
	FailedRequests           int64       `gorm:"column:failed_requests" json:"failedRequests"`
	CreatedAt                *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LLMDailySpend) TableName() string { return "aa_llm_daily_spend" }

// LLMSpendAggregate 花费归集聚合行（非表实体，按维度 GROUP BY 的投影）：
// 维度值落在 Date（按日）/ ScopeId+KeyAlias（按 key/用户）/ Model（按模型）之一，
// 其余维度字段为空。Token/请求为该维度窗口内合计。
type LLMSpendAggregate struct {
	Date             string  `json:"date"`
	ScopeId          string  `json:"scopeId"`
	KeyAlias         string  `json:"keyAlias"`
	Model            string  `json:"model"`
	Spend            types.Milli `json:"spend"`
	ApiRequests      int64   `json:"apiRequests"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
}

// LLMSpendTotals 日期范围内总花费/请求/Token（汇总卡片用）。
type LLMSpendTotals struct {
	Spend            types.Milli `json:"spend"`
	ApiRequests      int64   `json:"apiRequests"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
}

// LLMSpendPoint 某 scope 日粒度消费聚合点（我的预算·历史周期消费，非表实体）。
type LLMSpendPoint struct {
	Date             string      `gorm:"column:date" json:"date"`
	Spend            types.Milli `gorm:"column:spend" json:"spend"`
	Requests         int64   `gorm:"column:requests" json:"requests"`
	PromptTokens     int64   `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens int64   `gorm:"column:completion_tokens" json:"completionTokens"`
}

// ============ 错误日志 ============
type LLMErrorLog struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RequestId       string      `gorm:"column:request_id" json:"requestId"`
	StartTime       *types.Time `gorm:"column:start_time" json:"startTime"`
	EndTime         *types.Time `gorm:"column:end_time" json:"endTime"`
	ApiBase         string      `gorm:"column:api_base" json:"apiBase"`
	ModelGroup      string      `gorm:"column:model_group" json:"modelGroup"`
	ModelName       string      `gorm:"column:model_name" json:"modelName"`
	ModelId         string      `gorm:"column:model_id" json:"modelId"`
	RequestKwargs   string      `gorm:"column:request_kwargs;type:text" json:"requestKwargs"`
	ExceptionType   string      `gorm:"column:exception_type" json:"exceptionType"`
	ExceptionString string      `gorm:"column:exception_string" json:"exceptionString"`
	StatusCode      string      `gorm:"column:status_code" json:"statusCode"`
	ApiKey          string      `gorm:"column:api_key" json:"apiKey"`
	UserId          string      `gorm:"column:user_id" json:"userId"`
	TeamId          string      `gorm:"column:team_id" json:"teamId"`
}

func (LLMErrorLog) TableName() string { return "aa_llm_error_log" }

// ============ 审计日志 ============
type LLMAuditLog struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	Action          string      `gorm:"column:action" json:"action"`
	TblName         string      `gorm:"column:table_name" json:"tableName"`
	ObjectId        string      `gorm:"column:object_id" json:"objectId"`
	BeforeValue     string      `gorm:"column:before_value;type:text" json:"beforeValue,omitempty"`
	UpdatedValues   string      `gorm:"column:updated_values;type:text" json:"updatedValues,omitempty"`
	ChangedBy       string      `gorm:"column:changed_by" json:"changedBy"`
	ChangedByApiKey string      `gorm:"column:changed_by_api_key" json:"changedByApiKey"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LLMAuditLog) TableName() string { return "aa_llm_audit_log" }

// ============ 健康检查 ============
type LLMHealthCheck struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	HealthCheckId  string      `gorm:"column:health_check_id;uniqueIndex" json:"healthCheckId"`
	ModelName      string      `gorm:"column:model_name" json:"modelName"`
	ModelId        string      `gorm:"column:model_id" json:"modelId"`
	Status         string      `gorm:"column:status" json:"status"`
	HealthyCount   int         `gorm:"column:healthy_count" json:"healthyCount"`
	UnhealthyCount int         `gorm:"column:unhealthy_count" json:"unhealthyCount"`
	ErrorMessage   string      `gorm:"column:error_message" json:"errorMessage"`
	ResponseTimeMs *float64    `gorm:"column:response_time_ms" json:"responseTimeMs"`
	Details        string      `gorm:"column:details;type:text" json:"details"`
	CheckedBy      string      `gorm:"column:checked_by" json:"checkedBy"`
	CheckedAt      *types.Time `gorm:"column:checked_at" json:"checkedAt"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LLMHealthCheck) TableName() string { return "aa_llm_health_check" }

// ============ 提示词 ============
type LLMPrompt struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PromptId    string      `gorm:"column:prompt_id" json:"promptId"`
	Name        string      `gorm:"column:name" json:"name"`
	Version     int         `gorm:"column:version" json:"version"`
	Environment string      `gorm:"column:environment" json:"environment"`
	ExtraParams string      `gorm:"column:extra_params;type:text" json:"extraParams"`
	PromptInfo  string      `gorm:"column:prompt_info;type:text" json:"promptInfo"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (LLMPrompt) TableName() string { return "aa_llm_prompt" }
