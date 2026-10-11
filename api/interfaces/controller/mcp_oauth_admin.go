package controller

// MCP OAuth 客户端/令牌管理端控制器（主 api 服务，管理后台 CRUD）。
// 协议端点在独立 mcp 服务（interfaces/gateway/oauth.go：注册/同意页/换令牌），
// 本控制器只做管理台：客户端增删改查、轮换 secret、令牌查询与吊销，
// 复用 McpOAuthService（与网关同一份数据面，库表在 mcp 库 aa_mcp_oauth_*）。
// 安全口径：client_secret 明文仅创建/轮换时返回一次，库内只存 sha256；
// 令牌哈希不可逆，列表只给前缀预览（oauth: + access_token_hash 前 8 位）供排查对账，
// 绝不还原明文。client_type 无独立列：public 客户端不落 secret（哈希为空串），由其推导。

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// ===== 请求 =====

// McpOAuthClientListReq 客户端列表（keyword 模糊匹配名称/client_id）。
type McpOAuthClientListReq struct {
	Keyword string `json:"keyword" form:"keyword"`
	dto.PageReq
}

// McpOAuthClientAddReq 新增客户端。
type McpOAuthClientAddReq struct {
	ClientName   string   `json:"clientName"`
	ClientType   string   `json:"clientType"`   // confidential|public，缺省 confidential
	RedirectUris []string `json:"redirectUris"` // 授权码回调白名单，必填
	GrantTypes   []string `json:"grantTypes"`   // 缺省 authorization_code+refresh_token
	Scopes       []string `json:"scopes"`       // scope 白名单，空=不限
	Enabled      *bool    `json:"enabled"`      // 缺省 true
}

// McpOAuthClientEditReq 编辑客户端（数组字段传了即整组覆盖，null/缺省=不动）。
type McpOAuthClientEditReq struct {
	ClientId     string   `json:"clientId" binding:"required"`
	ClientName   string   `json:"clientName"`
	ClientType   string   `json:"clientType"`
	RedirectUris []string `json:"redirectUris"`
	GrantTypes   []string `json:"grantTypes"`
	Scopes       []string `json:"scopes"`
	Enabled      *bool    `json:"enabled"`
}

// McpOAuthClientDeleteReq 删除客户端（ids=协议 client_id 列表，级联吊销其全部令牌；空列表由服务层兜底）。
type McpOAuthClientDeleteReq struct {
	Ids []string `json:"ids"`
}

// McpOAuthRotateSecretReq 轮换客户端 secret。
type McpOAuthRotateSecretReq struct {
	ClientId string `json:"clientId" binding:"required"`
}

// McpOAuthTokenListReq 令牌列表（clientId 空=全部）。
type McpOAuthTokenListReq struct {
	ClientId string `json:"clientId" form:"clientId"`
	dto.PageReq
}

// McpOAuthTokenRevokeReq 吊销令牌（tokenId=对外令牌 id，库内 biz_id；空值由服务层兜底）。
type McpOAuthTokenRevokeReq struct {
	TokenId string `json:"tokenId"`
}

// ===== 响应 =====

// McpOAuthClientCreateRes 创建/轮换 secret 返回（明文仅此一次；public 类型无 secret 不返回）。
type McpOAuthClientCreateRes struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// McpOAuthClientItem 客户端列表行。
type McpOAuthClientItem struct {
	ClientId     string      `json:"clientId"`
	ClientName   string      `json:"clientName"`
	ClientType   string      `json:"clientType"` // confidential|public
	RedirectUris []string    `json:"redirectUris"`
	GrantTypes   []string    `json:"grantTypes"`
	Scopes       []string    `json:"scopes"`
	Enabled      bool        `json:"enabled"`
	TokenCount   int64       `json:"tokenCount"` // 未吊销令牌数（富化）
	CreatedAt    *types.Time `json:"createdAt,omitempty"`
	UpdatedAt    *types.Time `json:"updatedAt,omitempty"`
}

// McpOAuthClientListRes 客户端列表返回。
type McpOAuthClientListRes struct {
	dto.ListRes
	List []*McpOAuthClientItem `json:"list"`
}

// McpOAuthTokenItem 令牌列表行。
type McpOAuthTokenItem struct {
	TokenId            string      `json:"tokenId"` // 对外令牌 id（库内 biz_id）
	ClientId           string      `json:"clientId"`
	ClientName         string      `json:"clientName,omitempty"` // 富化：客户端已删则为空
	Subject            string      `json:"subject,omitempty"`    // 富化：授权主体=关联 Bearer Key 名称（aa_mcp_bearer_key）
	Scope              string      `json:"scope,omitempty"`
	AccessTokenPreview string      `json:"accessTokenPreview"` // 哈希前缀安全展示，非明文（哈希不可逆）
	IssuedAt           *types.Time `json:"issuedAt,omitempty"`
	ExpiresAt          *types.Time `json:"expiresAt,omitempty"`
	Revoked            bool        `json:"revoked"` // revoked_at 非空即已吊销
}

// McpOAuthTokenListRes 令牌列表返回。
type McpOAuthTokenListRes struct {
	dto.ListRes
	List []*McpOAuthTokenItem `json:"list"`
}

// ===== 控制器 =====

type McpOAuthAdminController struct{ svc *service.McpOAuthService }

func NewMcpOAuthAdminController(s *service.McpOAuthService) *McpOAuthAdminController {
	return &McpOAuthAdminController{svc: s}
}

// List GET 客户端分页列表。
func (h *McpOAuthAdminController) List(c *gin.Context) {
	var req McpOAuthClientListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	rows, counts, total, err := h.svc.AdminListClients(c.Request.Context(), req.Keyword, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	list := make([]*McpOAuthClientItem, 0, len(rows))
	for _, e := range rows {
		list = append(list, mcpOAuthClientItem(e, counts[e.ClientId]))
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, McpOAuthClientListRes{ListRes: dto.ListRes{CurrentPage: pn, Total: total}, List: list})
}

// Add POST 新增客户端。clientSecret 仅此一次返回（public 类型无 secret，不返回）。
func (h *McpOAuthAdminController) Add(c *gin.Context) {
	var req McpOAuthClientAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	enabled := true // 缺省启用（与网关动态注册口径一致）
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	clientId, secret, err := h.svc.AdminCreateClient(c.Request.Context(),
		req.ClientName, req.ClientType, req.RedirectUris, req.GrantTypes, req.Scopes, enabled)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, McpOAuthClientCreateRes{ClientId: clientId, ClientSecret: secret})
}

// Edit PUT 编辑客户端：传入的数组字段整组覆盖（null=不动），返回 {}。
func (h *McpOAuthAdminController) Edit(c *gin.Context) {
	var req McpOAuthClientEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if _, err := h.svc.AdminEditClient(c.Request.Context(), req.ClientId, req.ClientName, req.ClientType,
		req.RedirectUris, req.GrantTypes, req.Scopes, req.Enabled); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Delete DELETE 删除客户端（ids=client_id），级联吊销其全部令牌，返回 {}。
func (h *McpOAuthAdminController) Delete(c *gin.Context) {
	var req McpOAuthClientDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminDeleteClients(c.Request.Context(), req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// RotateSecret POST 轮换 secret：新 secret 仅此一次返回，旧 secret 立即失效。
func (h *McpOAuthAdminController) RotateSecret(c *gin.Context) {
	var req McpOAuthRotateSecretReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	clientId, secret, err := h.svc.AdminRotateSecret(c.Request.Context(), req.ClientId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, McpOAuthClientCreateRes{ClientId: clientId, ClientSecret: secret})
}

// TokenList GET 令牌分页列表（clientId 空=全部，倒序；富化 clientName/subject）。
func (h *McpOAuthAdminController) TokenList(c *gin.Context) {
	var req McpOAuthTokenListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	rows, total, err := h.svc.AdminListTokens(c.Request.Context(), req.ClientId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	clientIds := make([]string, 0, len(rows))
	keyIds := make([]string, 0, len(rows))
	for _, t := range rows {
		clientIds = append(clientIds, t.ClientId)
		keyIds = append(keyIds, t.BearerKeyId)
	}
	clientNames, keyNames, err := h.svc.AdminTokenEnrich(c.Request.Context(), clientIds, keyIds)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	list := make([]*McpOAuthTokenItem, 0, len(rows))
	for _, t := range rows {
		list = append(list, &McpOAuthTokenItem{
			TokenId:            t.BizId,
			ClientId:           t.ClientId,
			ClientName:         clientNames[t.ClientId],
			Subject:            keyNames[t.BearerKeyId],
			Scope:              t.Scope,
			AccessTokenPreview: oauthTokenPreview(t.AccessTokenHash),
			IssuedAt:           t.CreatedAt,
			ExpiresAt:          t.ExpiresAt,
			Revoked:            t.RevokedAt != nil,
		})
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, McpOAuthTokenListRes{ListRes: dto.ListRes{CurrentPage: pn, Total: total}, List: list})
}

// TokenRevoke POST 吊销令牌（revoked_at=now，access+refresh 一并作废；已吊销幂等成功），返回 {}。
func (h *McpOAuthAdminController) TokenRevoke(c *gin.Context) {
	var req McpOAuthTokenRevokeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AdminRevokeToken(c.Request.Context(), req.TokenId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 内部 =====

// mcpOAuthClientItem 客户端实体 → UI 行（jsonb 数组列拆成数组、smallint 开关位转 bool）。
func mcpOAuthClientItem(e *entity.McpOAuthClient, tokenCount int64) *McpOAuthClientItem {
	return &McpOAuthClientItem{
		ClientId:     e.ClientId,
		ClientName:   e.ClientName,
		ClientType:   service.ClientTypeOf(e),
		RedirectUris: mcpOAuthJSONStrings(e.RedirectUris),
		GrantTypes:   mcpOAuthJSONStrings(e.GrantTypes),
		Scopes:       mcpOAuthScopeStrings(e.Scope),
		Enabled:      e.IsEnabled == 1,
		TokenCount:   tokenCount,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

// mcpOAuthJSONStrings jsonb 字符串数组列 → UI 数组（空/坏值给空数组，避免前端拿到 null）。
func mcpOAuthJSONStrings(jsonArr string) []string {
	if jsonArr == "" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonArr), &arr); err != nil {
		return []string{}
	}
	if arr == nil {
		return []string{}
	}
	return arr
}

// mcpOAuthScopeStrings 注册 scope 白名单（库内空格分隔串 → UI 数组；空=不限）。
func mcpOAuthScopeStrings(scope string) []string {
	if strings.TrimSpace(scope) == "" {
		return []string{}
	}
	return strings.Fields(scope)
}

// oauthTokenPreview 令牌安全展示：access_token_hash 是 sha256，不可逆，
// 取前 8 位作对账前缀不足以还原明文；异常短哈希给固定掩码。
func oauthTokenPreview(hash string) string {
	if len(hash) >= 8 {
		return "oauth:" + hash[:8]
	}
	return "oauth:••••"
}
