package model

import "time"

// router 端只读 llm 库中网关运行所需的实体（字段与 llm.sql 对齐）。

type VirtualKey struct {
	Id                 uint       `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TokenHash          string     `gorm:"column:token_hash" json:"-"`
	KeyAlias           string     `gorm:"column:key_alias" json:"keyAlias"`
	UserId             string     `gorm:"column:user_id" json:"userId"`
	TeamId             string     `gorm:"column:team_id" json:"teamId"`
	ProjectId          string     `gorm:"column:project_id" json:"projectId"`
	OrgId              string     `gorm:"column:org_id" json:"orgId"`
	Models             string     `gorm:"column:models;type:text" json:"models"`
	Spend              Milli      `gorm:"column:spend" json:"spend"`
	MaxBudget          *Milli     `gorm:"column:max_budget" json:"maxBudget"`
	BudgetId           string     `gorm:"column:budget_id" json:"budgetId"`
	TpmLimit           *int64     `gorm:"column:tpm_limit" json:"tpmLimit"`
	RpmLimit           *int64     `gorm:"column:rpm_limit" json:"rpmLimit"`
	MaxParallelRequests *int      `gorm:"column:max_parallel_requests" json:"maxParallelRequests"`
	AllowedModels      string     `gorm:"column:allowed_models;type:text" json:"allowedModels"`
	AllowedRoutes      string     `gorm:"column:allowed_routes;type:text" json:"allowedRoutes"`
	Expires            *time.Time `gorm:"column:expires" json:"expires"`
	Blocked            int        `gorm:"column:blocked" json:"blocked"`
	BudgetDuration     string     `gorm:"column:budget_duration" json:"budgetDuration"`
	BudgetResetAt      *time.Time `gorm:"column:budget_reset_at" json:"budgetResetAt"`
	DeprecatedTokenHash string    `gorm:"column:deprecated_token_hash" json:"-"`
	DeprecateExpiresAt *time.Time `gorm:"column:deprecate_expires_at" json:"-"`
}

func (VirtualKey) TableName() string { return "aa_llm_virtual_key" }

type Model struct {
	Id                  uint    `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ModelId             string  `gorm:"column:model_id" json:"modelId"`
	ModelName           string  `gorm:"column:model_name" json:"modelName"`
	ModelGroup          string  `gorm:"column:model_group" json:"modelGroup"`
	Provider            string  `gorm:"column:provider" json:"provider"`
	ProviderModel       string  `gorm:"column:provider_model" json:"providerModel"`
	CredentialId        string  `gorm:"column:credential_id" json:"credentialId"`
	ExtraParams       string  `gorm:"column:extra_params;type:text" json:"extraParams"`
	InputPricePerMillion  Milli `gorm:"column:input_price_per_million" json:"inputPricePerMillion"` // 输入单价（毫/百万 tokens）
	OutputPricePerMillion Milli `gorm:"column:output_price_per_million" json:"outputPricePerMillion"` // 输出单价（毫/百万 tokens）
	MaxInputTokens      int     `gorm:"column:max_input_tokens" json:"maxInputTokens"`
	MaxOutputTokens     int     `gorm:"column:max_output_tokens" json:"maxOutputTokens"`
	TpmLimit            int64   `gorm:"column:tpm_limit" json:"tpmLimit"`
	RpmLimit            int64   `gorm:"column:rpm_limit" json:"rpmLimit"`
	Weight              int     `gorm:"column:weight" json:"weight"`
	Blocked             int     `gorm:"column:blocked" json:"blocked"`
	Status              int     `gorm:"column:status" json:"status"`
}

func (Model) TableName() string { return "aa_llm_model" }

type Credential struct {
	Id               uint   `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	CredentialId     string `gorm:"column:credential_id" json:"credentialId"`
	CredentialName   string `gorm:"column:credential_name" json:"credentialName"`
	ProviderType     string `gorm:"column:provider_type" json:"providerType"`
	BaseURL          string `gorm:"column:base_url" json:"baseUrl"`
	CredentialValues string `gorm:"column:credential_values;type:text" json:"-"` // 加密密文
	Status           int    `gorm:"column:status" json:"status"`
}

func (Credential) TableName() string { return "aa_llm_credential" }

// Budget 共享预算：多个 virtual key 通过 budget_id 共用同一额度，
// Spend 是当前窗口内累计（BudgetDuration 到期滚存清零，见 store.RollBudgetWindow）。
type Budget struct {
	Id             uint       `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BudgetId       string     `gorm:"column:budget_id" json:"budgetId"`
	BudgetName     string     `gorm:"column:budget_name" json:"budgetName"`
	MaxBudget      *Milli     `gorm:"column:max_budget" json:"maxBudget"`
	SoftBudget     *Milli     `gorm:"column:soft_budget" json:"softBudget"`
	Spend          Milli      `gorm:"column:spend" json:"spend"`
	BudgetDuration string     `gorm:"column:budget_duration" json:"budgetDuration"`
	BudgetResetAt  *time.Time `gorm:"column:budget_reset_at" json:"budgetResetAt"`
	GrantRules     string     `gorm:"column:grant_rules" json:"grantRules"` // 适用范围条件行 JSON（空 '[]'=不圈人，仅经 key.budget_id 挂靠生效）
}

func (Budget) TableName() string { return "aa_llm_budget" }

// 凭据值结构：credential_values 列在 api 端存的是 AES 加密后的 hex 字符串，
// 解密后得到本结构（apiKey 等）。router 用自己的密钥解密——为避免两服务共享密钥的复杂性，
// 这里约定 credential_values 直接存 {"api_key":"..."} 明文 json（生产可改回加密 + 共享 jwt.encryptKey）。
type CredentialValues struct {
	APIKey string `json:"api_key"`
}
