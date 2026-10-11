package dto

import "aiarsenal/domain/entity"

// MCP 模块请求/响应 DTO。

// ===== 通用 =====
type McpIdsReq struct {
	Ids []string `json:"ids" form:"ids"`
}

// ===== Server =====
type McpServerSearchReq struct {
	Name string `json:"name" form:"name"`
	Type string `json:"type" form:"type"`
	PageReq
}
type McpServerSearchRes struct {
	ListRes
	List []*entity.McpServer `json:"list"`
}
type McpServerReq struct {
	Name               string `json:"name" form:"name" binding:"required"`
	Type               string `json:"type" form:"type" binding:"required"`
	Description        string `json:"description" form:"description"`
	Url                string `json:"url" form:"url"`
	Command            string `json:"command" form:"command"`
	Args               string `json:"args" form:"args"`
	Env                string `json:"env" form:"env"`
	Headers            string `json:"headers" form:"headers"`
	Options            string `json:"options" form:"options"`
	Oauth              string `json:"oauth" form:"oauth"`
	Proxy              string `json:"proxy" form:"proxy"`
	Openapi            string `json:"openapi" form:"openapi"`
	PassthroughHeaders string `json:"passthroughHeaders" form:"passthroughHeaders"`
	Enabled            uint   `json:"enabled" form:"enabled"`
	Visibility         string `json:"visibility" form:"visibility"`
	EnableKeepAlive    uint   `json:"enableKeepAlive" form:"enableKeepAlive"`
	KeepAliveInterval  int    `json:"keepAliveInterval" form:"keepAliveInterval"`
	Tools              string `json:"tools" form:"tools"`
	Prompts            string `json:"prompts" form:"prompts"`
	Resources          string `json:"resources" form:"resources"`
}
type McpServerAddReq struct{ *McpServerReq }
type McpServerEditReq struct {
	ServerId string `json:"serverId" form:"serverId" binding:"required"`
	*McpServerReq
}

// ===== Group =====
type McpGroupSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type McpGroupSearchRes struct {
	ListRes
	List []*entity.McpGroup `json:"list"`
}
type McpGroupReq struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Description string `json:"description" form:"description"`
	Servers     string `json:"servers" form:"servers"` // jsonb 数组字符串
}
type McpGroupAddReq struct{ *McpGroupReq }
type McpGroupEditReq struct {
	GroupId string `json:"groupId" form:"groupId" binding:"required"`
	*McpGroupReq
}

// McpGroupGrantGetReq 分组授权配置回显。
type McpGroupGrantGetReq struct {
	GroupId string `json:"groupId" form:"groupId" binding:"required"`
}

// McpGroupGrantSaveReq 分组授权配置保存（可见性档位 + 条件行整体重写）。
type McpGroupGrantSaveReq struct {
	GroupId    string `json:"groupId" form:"groupId" binding:"required"`
	Visibility uint   `json:"visibility" form:"visibility"`           // 0私有 1按条件 2全员
	Rules      string `json:"rules" form:"rules"`                     // GrantRow 数组 JSON 文本
}

// ===== BearerKey =====
type McpKeySearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type McpKeySearchRes struct {
	ListRes
	List []*entity.McpBearerKey `json:"list"`
}
type McpKeyCreateRes struct {
	KeyId string `json:"keyId"`
	Token string `json:"token"` // 明文，仅签发/轮换时返回一次
}
type McpKeyReq struct {
	Name           string `json:"name" form:"name" binding:"required"`
	Kind           string `json:"kind" form:"kind"`
	AccessType     string `json:"accessType" form:"accessType"`
	AllowedGroups  string `json:"allowedGroups" form:"allowedGroups"`
	AllowedServers string `json:"allowedServers" form:"allowedServers"`
	Enabled        uint   `json:"enabled" form:"enabled"`
}
type McpKeyAddReq struct{ *McpKeyReq }
type McpKeyEditReq struct {
	KeyId string `json:"keyId" form:"keyId" binding:"required"`
	*McpKeyReq
}

// ===== Activity =====
type McpActivitySearchReq struct {
	ServerName string `json:"serverName" form:"serverName"`
	Tool       string `json:"tool" form:"tool"`
	Status     string `json:"status" form:"status"`
	UserId     string `json:"userId" form:"userId"`
	PageReq
}
type McpActivitySearchRes struct {
	ListRes
	List []*entity.McpActivity `json:"list"`
}

// ===== BuiltinPrompt =====
type McpPromptSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type McpPromptSearchRes struct {
	ListRes
	List []*entity.McpBuiltinPrompt `json:"list"`
}
type McpPromptReq struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
	Template    string `json:"template" form:"template" binding:"required"`
	Arguments   string `json:"arguments" form:"arguments"`
	Enabled     uint   `json:"enabled" form:"enabled"`
}
type McpPromptAddReq struct{ *McpPromptReq }
type McpPromptEditReq struct {
	PromptId string `json:"promptId" form:"promptId" binding:"required"`
	*McpPromptReq
}

// ===== BuiltinResource =====
type McpResourceSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type McpResourceSearchRes struct {
	ListRes
	List []*entity.McpBuiltinResource `json:"list"`
}
type McpResourceReq struct {
	Uri         string `json:"uri" form:"uri" binding:"required"`
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	MimeType    string `json:"mimeType" form:"mimeType"`
	Content     string `json:"content" form:"content" binding:"required"`
	Enabled     uint   `json:"enabled" form:"enabled"`
}
type McpResourceAddReq struct{ *McpResourceReq }
type McpResourceEditReq struct {
	ResourceId string `json:"resourceId" form:"resourceId" binding:"required"`
	*McpResourceReq
}

// ===== 上游探测（连接测试 / 工具预览 / 试调用） =====
type McpServerTestReq struct {
	ServerId      string `json:"serverId" form:"serverId"` // 二选一：测已保存的
	*McpServerReq        // 或测未保存的（新增弹窗先测后存）
}
type McpToolPreview struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}
type McpPromptPreview struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type McpResourcePreview struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
	Uri         string `json:"uri,omitempty"`
}
type McpServerTestRes struct {
	Ok         bool                 `json:"ok"`
	Message    string               `json:"message,omitempty"` // 失败原因
	DurationMs int                  `json:"durationMs"`
	Tools      []McpToolPreview     `json:"tools"`
	Prompts    []McpPromptPreview   `json:"prompts"`
	Resources  []McpResourcePreview `json:"resources"`
}
type McpServerCallReq struct {
	ServerId  string         `json:"serverId" form:"serverId" binding:"required"`
	Tool      string         `json:"tool" form:"tool" binding:"required"`
	Arguments map[string]any `json:"arguments"`
}
type McpServerCallRes struct {
	DurationMs int    `json:"durationMs"`
	Text       string `json:"text"`
}
