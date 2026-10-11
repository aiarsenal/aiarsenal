package entity

import "aiarsenal/domain/types"

// MCP 聚合模块实体（mcp 库，表前缀 aa_mcp_）。
// MCP 模块实体，按 P1 范围裁剪；jsonb 列用 string + type:jsonb（沿用 llm.go 写法）。
// 跨库软引用：owner / user_id / key_id → aiarsenal.aa_user.user_id（varchar(32) biz_id）。
// 智能路由（vector_embeddings）与 OAuth（oauth_clients/oauth_tokens）留 P2/P3。

// ============ MCP 服务器配置 ============
type McpServer struct {
	Id                 uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ServerId           string      `gorm:"column:server_id;uniqueIndex" json:"serverId"`
	Name               string      `gorm:"column:name" json:"name"`
	Type               string      `gorm:"column:type" json:"type"` // stdio/sse/streamable-http/openapi
	Description        string      `gorm:"column:description" json:"description"`
	Url                string      `gorm:"column:url" json:"url"`
	Command            string      `gorm:"column:command" json:"command"`
	Args               string      `gorm:"column:args;type:jsonb" json:"args"`
	Env                string      `gorm:"column:env;type:jsonb" json:"env"`
	Headers            string      `gorm:"column:headers;type:jsonb" json:"headers"`
	Options            string      `gorm:"column:options;type:jsonb" json:"options"`
	Oauth              string      `gorm:"column:oauth;type:jsonb" json:"oauth"`
	Proxy              string      `gorm:"column:proxy;type:jsonb" json:"proxy"`
	Openapi            string      `gorm:"column:openapi;type:jsonb" json:"openapi"`
	PassthroughHeaders string      `gorm:"column:passthrough_headers;type:jsonb" json:"passthroughHeaders"`
	Enabled            uint        `gorm:"column:enabled" json:"enabled"`
	Owner              string      `gorm:"column:owner" json:"owner"`
	Visibility         string      `gorm:"column:visibility" json:"visibility"` // private/group/public
	EnableKeepAlive    uint        `gorm:"column:enable_keep_alive" json:"enableKeepAlive"`
	KeepAliveInterval  int         `gorm:"column:keep_alive_interval" json:"keepAliveInterval"`
	Tools              string      `gorm:"column:tools;type:jsonb" json:"tools"`
	Prompts            string      `gorm:"column:prompts;type:jsonb" json:"prompts"`
	Resources          string      `gorm:"column:resources;type:jsonb" json:"resources"`
	CreatedBy          string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy          string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt          *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt          *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (McpServer) TableName() string { return "aa_mcp_server" }

// ============ 分组 ============
type McpGroup struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	GroupId     string      `gorm:"column:group_id;uniqueIndex" json:"groupId"`
	Name        string      `gorm:"column:name" json:"name"`
	Description string      `gorm:"column:description" json:"description"`
	Servers     string      `gorm:"column:servers;type:jsonb" json:"servers"` // [{name,alias,tools[],prompts[],resources[]|'all'}]
	Visibility  uint        `gorm:"column:visibility" json:"visibility"`     // 0私有(仅owner/超管) 1按条件(aa_mcp_group_grant) 2全员
	Owner       string      `gorm:"column:owner" json:"owner"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
	OwnerName   string      `gorm:"-" json:"ownerName"` // 回填：归属人姓名（aa_user.name），列表展示用
	CanManage   bool        `gorm:"-" json:"canManage"` // 回填：当前身份可管理（编辑/删除/配授权；超管∨owner）
}

func (McpGroup) TableName() string { return "aa_mcp_group" }

// McpGroupGrant 分组组合授权条件行（表 aa_mcp_group_grant，与 aa_kb_grant 同构、单动作「可使用」裁剪：
// 无 target_type/action；TargetId → aa_mcp_group.group_id）。owner 与超管判定层天然全权，不落行。
type McpGroupGrant struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TargetId  string      `gorm:"column:target_id" json:"targetId"`
	UserIds   string      `gorm:"column:user_ids;type:jsonb" json:"userIds"`
	DeptId    string      `gorm:"column:dept_id" json:"deptId"`
	RoleIds   string      `gorm:"column:role_ids;type:jsonb" json:"roleIds"`
	PostIds   string      `gorm:"column:post_ids;type:jsonb" json:"postIds"`
	Exclude   bool        `gorm:"column:exclude" json:"exclude"`
	GroupId   string      `gorm:"column:group_id" json:"groupId"` // 权限分组引用（GrantRow 维度）
	SetId     string      `gorm:"column:set_id" json:"setId"`     // 员工集合引用（GrantRow 维度）
	CreatedBy string      `gorm:"column:created_by" json:"-"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (McpGroupGrant) TableName() string { return "aa_mcp_group_grant" }

// ============ Bearer 端点密钥 ============
type McpBearerKey struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	KeyId          string      `gorm:"column:key_id;uniqueIndex" json:"keyId"`
	Name           string      `gorm:"column:name" json:"name"`
	TokenHash      string      `gorm:"column:token_hash;uniqueIndex" json:"-"` // 哈希不外露
	Enabled        uint        `gorm:"column:enabled" json:"enabled"`
	Kind           string      `gorm:"column:kind" json:"kind"` // system/user/agent/client（后两者挂实例/设备，Owner 仍为归属用户）
	Owner          string      `gorm:"column:owner" json:"owner"`
	AccessType     string      `gorm:"column:access_type" json:"accessType"` // all/groups/servers/custom
	AllowedGroups  string      `gorm:"column:allowed_groups;type:jsonb" json:"allowedGroups"`
	AllowedServers string      `gorm:"column:allowed_servers;type:jsonb" json:"allowedServers"`
	CreatedBy      string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpBearerKey) TableName() string { return "aa_mcp_bearer_key" }

// ============ 工具调用审计 ============
type McpActivity struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ServerName   string      `gorm:"column:server_name" json:"serverName"`
	Tool         string      `gorm:"column:tool" json:"tool"`
	DurationMs   int         `gorm:"column:duration_ms" json:"durationMs"`
	Status       string      `gorm:"column:status" json:"status"` // success/error
	Input        string      `gorm:"column:input" json:"input,omitempty"`
	Output       string      `gorm:"column:output" json:"output,omitempty"`
	GroupName    string      `gorm:"column:group_name" json:"groupName"`
	UserId       string      `gorm:"column:user_id" json:"userId"`
	KeyId        string      `gorm:"column:key_id" json:"keyId"`
	KeyName      string      `gorm:"column:key_name" json:"keyName"`
	SourceIp     string      `gorm:"column:source_ip" json:"sourceIp"`
	ErrorMessage string      `gorm:"column:error_message" json:"errorMessage,omitempty"`
	Ts           *types.Time `gorm:"column:ts" json:"ts"`
}

func (McpActivity) TableName() string { return "aa_mcp_activity" }

// ============ 内置 Prompt ============
type McpBuiltinPrompt struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PromptId    string      `gorm:"column:prompt_id;uniqueIndex" json:"promptId"`
	Name        string      `gorm:"column:name" json:"name"`
	Title       string      `gorm:"column:title" json:"title"`
	Description string      `gorm:"column:description" json:"description"`
	Template    string      `gorm:"column:template" json:"template"`
	Arguments   string      `gorm:"column:arguments;type:jsonb" json:"arguments"`
	Enabled     uint        `gorm:"column:enabled" json:"enabled"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpBuiltinPrompt) TableName() string { return "aa_mcp_builtin_prompt" }

// ============ 内置 Resource ============
type McpBuiltinResource struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ResourceId  string      `gorm:"column:resource_id;uniqueIndex" json:"resourceId"`
	Uri         string      `gorm:"column:uri" json:"uri"`
	Name        string      `gorm:"column:name" json:"name"`
	Description string      `gorm:"column:description" json:"description"`
	MimeType    string      `gorm:"column:mime_type" json:"mimeType"`
	Content     string      `gorm:"column:content" json:"content"`
	Enabled     uint        `gorm:"column:enabled" json:"enabled"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpBuiltinResource) TableName() string { return "aa_mcp_builtin_resource" }

// ============ 系统配置（单例 id='default'） ============
type McpConfig struct {
	Id                    string      `gorm:"column:id;primaryKey" json:"id"`
	Routing               string      `gorm:"column:routing;type:jsonb" json:"routing"`
	Install               string      `gorm:"column:install;type:jsonb" json:"install"`
	ToolResultCompression string      `gorm:"column:tool_result_compression;type:jsonb" json:"toolResultCompression"`
	McpRouter             string      `gorm:"column:mcp_router;type:jsonb" json:"mcpRouter"`
	NameSeparator         string      `gorm:"column:name_separator" json:"nameSeparator"`
	Auth                  string      `gorm:"column:auth;type:jsonb" json:"auth"`
	Discovery             string      `gorm:"column:discovery;type:jsonb" json:"discovery"`
	ActivityLog           string      `gorm:"column:activity_log;type:jsonb" json:"activityLog"`
	EnableSessionRebuild  uint        `gorm:"column:enable_session_rebuild" json:"enableSessionRebuild"`
	CreatedAt             *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpConfig) TableName() string { return "aa_mcp_config" }

// ============ 用户级配置 ============
type McpUserConfig struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	UserId           string      `gorm:"column:user_id" json:"userId"` // → aa_user.user_id
	Routing          string      `gorm:"column:routing;type:jsonb" json:"routing"`
	AdditionalConfig string      `gorm:"column:additional_config;type:jsonb" json:"additionalConfig"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (McpUserConfig) TableName() string { return "aa_mcp_user_config" }
