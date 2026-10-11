package entity

import "aiarsenal/domain/types"

// MCP OAuth 授权实体（mcp 库，表前缀 aa_mcp_oauth_）。
// 对齐 MCP 规范 2025-06-18 Authorization：动态客户端注册（RFC 7591）+
// 授权码流程（RFC 6749）+ PKCE（RFC 7636，仅 S256）。
// 授权主体复用既有 Bearer Key：bearer_key_id 软引用 aa_mcp_bearer_key.key_id，
// access token 命中时等效其关联 key 鉴权（scope/access_type/工具面按 key 实时判定）。
// 明文一律不落库：client_secret / code / access_token / refresh_token 均只存 sha256。

// ============ OAuth 客户端 ============
type McpOAuthClient struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BizId            string      `gorm:"column:biz_id;uniqueIndex" json:"bizId"`
	ClientId         string      `gorm:"column:client_id;uniqueIndex" json:"clientId"`
	ClientSecretHash string      `gorm:"column:client_secret_hash" json:"-"` // 哈希不外露
	ClientName       string      `gorm:"column:client_name" json:"clientName"`
	RedirectUris     string      `gorm:"column:redirect_uris;type:jsonb" json:"redirectUris"` // ["https://..."]
	GrantTypes       string      `gorm:"column:grant_types;type:jsonb" json:"grantTypes"`     // ["authorization_code","refresh_token"]
	Scope            string      `gorm:"column:scope" json:"scope"`                           // 注册 scope 白名单（空=不限，key 权限兜底）
	IsEnabled        uint        `gorm:"column:is_enabled" json:"isEnabled"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpOAuthClient) TableName() string { return "aa_mcp_oauth_client" }

// ============ OAuth 授权码（一次性，短时效） ============
type McpOAuthCode struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BizId               string      `gorm:"column:biz_id;uniqueIndex" json:"bizId"`
	CodeHash            string      `gorm:"column:code_hash;uniqueIndex" json:"-"`
	ClientId            string      `gorm:"column:client_id" json:"clientId"`
	RedirectUri         string      `gorm:"column:redirect_uri" json:"redirectUri"`
	BearerKeyId         string      `gorm:"column:bearer_key_id" json:"bearerKeyId"` // 授权主体 = 既有 Bearer Key
	Scope               string      `gorm:"column:scope" json:"scope"`
	AccessType          string      `gorm:"column:access_type" json:"accessType"` // 签发时 key 的 access_type 快照
	CodeChallenge       string      `gorm:"column:code_challenge" json:"-"`       // PKCE S256 挑战（base64url(sha256(verifier))）
	CodeChallengeMethod string      `gorm:"column:code_challenge_method" json:"-"`
	ExpiresAt           *types.Time `gorm:"column:expires_at" json:"expiresAt"`
	UsedAt              *types.Time `gorm:"column:used_at" json:"usedAt"` // 置非空即作废（防重放）
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (McpOAuthCode) TableName() string { return "aa_mcp_oauth_code" }

// ============ OAuth 令牌（access/refresh 同行成对） ============
type McpOAuthToken struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BizId            string      `gorm:"column:biz_id;uniqueIndex" json:"bizId"`
	AccessTokenHash  string      `gorm:"column:access_token_hash;uniqueIndex" json:"-"`
	RefreshTokenHash string      `gorm:"column:refresh_token_hash;uniqueIndex" json:"-"`
	ClientId         string      `gorm:"column:client_id" json:"clientId"`
	BearerKeyId      string      `gorm:"column:bearer_key_id" json:"bearerKeyId"` // 等效鉴权的主体 key
	Scope            string      `gorm:"column:scope" json:"scope"`
	AccessType       string      `gorm:"column:access_type" json:"accessType"` // 签发时 key 的 access_type 快照
	ExpiresAt        *types.Time `gorm:"column:expires_at" json:"expiresAt"`   // access token 过期时间（refresh 不单独限时，吊销即失效）
	RevokedAt        *types.Time `gorm:"column:revoked_at" json:"revokedAt"`   // 吊销/轮换置非空，整行（access+refresh）作废
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpOAuthToken) TableName() string { return "aa_mcp_oauth_token" }
