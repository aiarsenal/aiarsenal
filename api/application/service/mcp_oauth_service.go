package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/crypto"
)

// MCP OAuth 授权服务（对齐 MCP 规范 2025-06-18 Authorization）。
// 动态客户端注册（RFC 7591）+ 授权码流程（RFC 6749）+ PKCE（RFC 7636，仅 S256）。
// 授权主体复用既有 Bearer Key：同意页粘贴 key 明文完成身份验证，签发的 code/token
// 均锚定该 key（bearer_key_id）——网关鉴权时 access token 等效 key 本体，
// scope/access_type/工具面按 key 实时判定（吊销/停用 key 即断 OAuth 链路）。
// 明文一律不落库：client_secret / code / access_token / refresh_token 均只存 sha256
// （crypto.HashToken），明文仅在签发响应里出现一次。

const (
	// OAuthCodeTTL 授权码有效期（RFC 6749 §4.1.2 建议 ≤10 分钟）。
	OAuthCodeTTL = 10 * time.Minute
	// OAuthAccessTokenTTL 访问令牌有效期。
	OAuthAccessTokenTTL = time.Hour

	// OAuthGrantAuthorizationCode / OAuthGrantRefreshToken 本网关支持的授权类型。
	OAuthGrantAuthorizationCode = "authorization_code"
	OAuthGrantRefreshToken      = "refresh_token"

	// OAuthScopeSeparator scope 分隔符（RFC 6749 §3.3，空格分隔）。
	OAuthScopeSeparator = " "
)

// OAuth 授权链路语义错误（网关层映射为 RFC 6749 §5.2 错误码）。
var (
	ErrOAuthClientUnknown    = errors.New("client 无效或已停用")
	ErrOAuthClientAuth       = errors.New("client 认证失败")
	ErrOAuthGrantUnsupported = errors.New("不支持的 grant_type")
	ErrOAuthGrantInvalid     = errors.New("授权码/刷新令牌无效、过期或已使用")
	ErrOAuthRedirectInvalid  = errors.New("redirect_uri 未注册")
	ErrOAuthVerifierInvalid  = errors.New("PKCE code_verifier 校验失败")
	ErrOAuthScopeInvalid     = errors.New("scope 超出客户端注册范围")
)

type McpOAuthService struct {
	clients repository.IMcpOAuthClientRepository
	codes   repository.IMcpOAuthCodeRepository
	tokens  repository.IMcpOAuthTokenRepository
	keys    *McpKeyService
}

func NewMcpOAuthService(c repository.IMcpOAuthClientRepository, d repository.IMcpOAuthCodeRepository,
	t repository.IMcpOAuthTokenRepository, k *McpKeyService) *McpOAuthService {
	return &McpOAuthService{clients: c, codes: d, tokens: t, keys: k}
}

// ============ 客户端（RFC 7591 动态注册） ============

// RegisterClient 注册客户端，返回 (client_id, client_secret 明文仅此一次)。
// redirect_uris 必填（授权码流程回调白名单，精确匹配）；grant_types 缺省
// authorization_code + refresh_token；scope 为该客户端可请求的白名单（空=不限）。
func (s *McpOAuthService) RegisterClient(ctx context.Context, clientName string, redirectUris, grantTypes []string, scope string) (string, string, error) {
	if len(redirectUris) == 0 {
		return "", "", errors.New("redirect_uris 不能为空")
	}
	if clientName == "" {
		clientName = "unnamed-client"
	}
	if len(grantTypes) == 0 {
		grantTypes = []string{OAuthGrantAuthorizationCode, OAuthGrantRefreshToken}
	}
	ru, err := json.Marshal(redirectUris)
	if err != nil {
		return "", "", err
	}
	gt, err := json.Marshal(grantTypes)
	if err != nil {
		return "", "", err
	}
	secret, err := crypto.RandomKey("mcp")
	if err != nil {
		return "", "", err
	}
	e := &entity.McpOAuthClient{
		ClientName:       clientName,
		ClientSecretHash: crypto.HashToken(secret),
		RedirectUris:     string(ru),
		GrantTypes:       string(gt),
		Scope:            scope,
		IsEnabled:        1,
	}
	clientId, err := s.clients.Add(ctx, e)
	if err != nil {
		return "", "", err
	}
	return clientId, secret, nil
}

// GetClient 按协议 client_id 取启用中的客户端。
func (s *McpOAuthService) GetClient(ctx context.Context, clientId string) (*entity.McpOAuthClient, error) {
	if clientId == "" {
		return nil, ErrOAuthClientUnknown
	}
	c, err := s.clients.GetByClientId(ctx, clientId)
	if err != nil || c == nil {
		return nil, ErrOAuthClientUnknown
	}
	return c, nil
}

// AuthenticateClient client_id+client_secret 认证（client_secret_basic/post 两式共用）。
func (s *McpOAuthService) AuthenticateClient(ctx context.Context, clientId, clientSecret string) (*entity.McpOAuthClient, error) {
	c, err := s.GetClient(ctx, clientId)
	if err != nil {
		return nil, err
	}
	if clientSecret == "" || c.ClientSecretHash != hashText(clientSecret) {
		return nil, ErrOAuthClientAuth
	}
	return c, nil
}

// RedirectAllowed 回调地址精确匹配注册白名单（防开放重定向）。
func RedirectAllowed(c *entity.McpOAuthClient, redirectUri string) bool {
	if c == nil || redirectUri == "" {
		return false
	}
	for _, u := range parseJSONStrings(c.RedirectUris) {
		if u == redirectUri {
			return true
		}
	}
	return false
}

// GrantAllowed 客户端是否注册了某授权类型。
func GrantAllowed(c *entity.McpOAuthClient, grant string) bool {
	for _, g := range parseJSONStrings(c.GrantTypes) {
		if g == grant {
			return true
		}
	}
	return false
}

// ScopeAllowed 请求 scope 是否都在客户端注册白名单内（未注册白名单=不限）。
func ScopeAllowed(c *entity.McpOAuthClient, requested string) bool {
	registered := scopeSet(c.Scope)
	if len(registered) == 0 {
		return true
	}
	for _, sc := range scopeItems(requested) {
		if _, ok := registered[sc]; !ok {
			return false
		}
	}
	return true
}

// ============ 授权码（同意页验证 Bearer Key 通过后签发） ============

// IssueCode 校验授权请求语义并签发授权码（明文仅此一次）。
// codeChallenge 必填（PKCE 强制，仅 S256）；scope 缺省回落客户端注册白名单。
func (s *McpOAuthService) IssueCode(ctx context.Context, client *entity.McpOAuthClient, key *entity.McpBearerKey,
	redirectUri, scope, codeChallenge string) (string, error) {
	if client == nil || key == nil {
		return "", ErrOAuthClientUnknown
	}
	if !RedirectAllowed(client, redirectUri) {
		return "", ErrOAuthRedirectInvalid
	}
	if codeChallenge == "" {
		return "", errors.New("PKCE code_challenge 必填（仅支持 S256）")
	}
	if scope == "" {
		scope = client.Scope
	}
	if !ScopeAllowed(client, scope) {
		return "", ErrOAuthScopeInvalid
	}
	code, err := crypto.RandomKey("mcp")
	if err != nil {
		return "", err
	}
	exp := types.Time(time.Now().Add(OAuthCodeTTL))
	row := &entity.McpOAuthCode{
		CodeHash:            hashText(code),
		ClientId:            client.ClientId,
		RedirectUri:         redirectUri,
		BearerKeyId:         key.KeyId,
		Scope:               scope,
		AccessType:          key.AccessType,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           &exp,
	}
	if err := s.codes.Add(ctx, row); err != nil {
		return "", err
	}
	return code, nil
}

// ExchangeCode 授权码换令牌：返回 (令牌行, access token 明文, refresh token 明文)。
// 校验链：client 认证（调用方先做）→ 码存在/未过期/未使用 → client/redirect 匹配 →
// PKCE S256 回验 → 原子消费（防并发重放）。
func (s *McpOAuthService) ExchangeCode(ctx context.Context, client *entity.McpOAuthClient,
	code, redirectUri, codeVerifier string) (*entity.McpOAuthToken, string, string, error) {
	if client == nil {
		return nil, "", "", ErrOAuthClientUnknown
	}
	if !GrantAllowed(client, OAuthGrantAuthorizationCode) {
		return nil, "", "", ErrOAuthGrantUnsupported
	}
	if code == "" {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	row, err := s.codes.GetByHash(ctx, hashText(code))
	if err != nil || row == nil {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if row.ExpiresAt != nil && time.Time(*row.ExpiresAt).Before(time.Now()) {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if row.ClientId != client.ClientId || row.RedirectUri != redirectUri || redirectUri == "" {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if pkceS256(codeVerifier) != row.CodeChallenge {
		return nil, "", "", ErrOAuthVerifierInvalid
	}
	// 原子消费：used_at is null 条件更新，受影响 0 行=已被并发消费（重放拒绝）。
	if n, err := s.codes.MarkUsed(ctx, row.CodeHash); err != nil || n == 0 {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	return s.issueTokens(ctx, client.ClientId, row.BearerKeyId, row.Scope, row.AccessType)
}

// ============ 刷新（RFC 6749 §6，轮换式） ============

// Refresh 刷新令牌：旧行整体吊销（access+refresh 一并作废），签发新对（同 key/scope）。
func (s *McpOAuthService) Refresh(ctx context.Context, client *entity.McpOAuthClient, refreshToken string) (*entity.McpOAuthToken, string, string, error) {
	if client == nil {
		return nil, "", "", ErrOAuthClientUnknown
	}
	if !GrantAllowed(client, OAuthGrantRefreshToken) {
		return nil, "", "", ErrOAuthGrantUnsupported
	}
	if refreshToken == "" {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	row, err := s.tokens.GetByRefreshHash(ctx, hashText(refreshToken))
	if err != nil || row == nil {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if row.RevokedAt != nil {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if row.ClientId != client.ClientId {
		return nil, "", "", ErrOAuthGrantInvalid
	}
	if err := s.tokens.Revoke(ctx, row.Id); err != nil {
		return nil, "", "", err
	}
	return s.issueTokens(ctx, client.ClientId, row.BearerKeyId, row.Scope, row.AccessType)
}

// ============ 网关鉴权回落 ============

// ResolveAccessToken access token 哈希 → 未吊销未过期的令牌行 → 关联 Bearer Key
// （须存在且 enabled=1）。鉴权语义与 key 直连完全等效（scope/access_type 按 key 实时判定）。
func (s *McpOAuthService) ResolveAccessToken(ctx context.Context, accessTokenHash string) (*entity.McpBearerKey, error) {
	if accessTokenHash == "" {
		return nil, ErrOAuthGrantInvalid
	}
	row, err := s.tokens.GetByAccessHash(ctx, accessTokenHash)
	if err != nil || row == nil {
		return nil, ErrOAuthGrantInvalid
	}
	if row.RevokedAt != nil {
		return nil, ErrOAuthGrantInvalid
	}
	if row.ExpiresAt != nil && time.Time(*row.ExpiresAt).Before(time.Now()) {
		return nil, ErrOAuthGrantInvalid
	}
	k, err := s.keys.Get(ctx, row.BearerKeyId)
	if err != nil || k == nil || k.Enabled == 0 {
		// 主体 key 已删除/停用：OAuth 链路随之断（等效 key 实时判定）。
		return nil, ErrOAuthGrantInvalid
	}
	return k, nil
}

// ============ 内部 ============

// issueTokens 签发一对令牌（access+refresh 明文即抛，落库均哈希）。
func (s *McpOAuthService) issueTokens(ctx context.Context, clientId, bearerKeyId, scope, accessType string) (*entity.McpOAuthToken, string, string, error) {
	access, err := crypto.RandomKey("mcp")
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := crypto.RandomKey("mcp")
	if err != nil {
		return nil, "", "", err
	}
	exp := types.Time(time.Now().Add(OAuthAccessTokenTTL))
	row := &entity.McpOAuthToken{
		AccessTokenHash:  hashText(access),
		RefreshTokenHash: hashText(refresh),
		ClientId:         clientId,
		BearerKeyId:      bearerKeyId,
		Scope:            scope,
		AccessType:       accessType,
		ExpiresAt:        &exp,
	}
	if err := s.tokens.Add(ctx, row); err != nil {
		return nil, "", "", err
	}
	return row, access, refresh, nil
}

// hashText sha256 hex（与 crypto.HashToken 同义；本地别名让语义调用点更短）。
func hashText(s string) string { return crypto.HashToken(s) }

// pkceS256 RFC 7636：base64url(sha256(code_verifier))，无填充。
func pkceS256(verifier string) string {
	if verifier == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// scopeItems / scopeSet scope 串（空格分隔）拆分与集合化。
func scopeItems(scope string) []string {
	var out []string
	cur := ""
	for _, r := range scope {
		if r == ' ' || r == '\t' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func scopeSet(scope string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, s := range scopeItems(scope) {
		m[s] = struct{}{}
	}
	return m
}

// parseJSONStrings jsonb 字符串数组列解析（与 McpKeyService.containsJSON 同源写法）。
func parseJSONStrings(jsonArr string) []string {
	if jsonArr == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonArr), &arr); err != nil {
		return nil
	}
	return arr
}

// ============ 管理端 CRUD（主服务 interfaces/controller/mcp_oauth_admin.go） ============
// 与网关共用同一份数据面（同库同表），管理台负责客户端增删改查 / 轮换 secret / 令牌查询吊销。
// 口径说明：库表无独立 client_type 列——公开客户端不落 secret（client_secret_hash 为空串），
// 类型由哈希是否为空推导，网关与管理端共用这一单一事实源，杜绝两处口径漂移。
// 安全口径与网关一致：client_secret 明文仅创建/轮换时返回一次，落库只存 sha256。

const (
	// OAuthClientTypeConfidential 机密客户端：持有 client_secret（token 端点 client_secret_basic/post 认证）。
	OAuthClientTypeConfidential = "confidential"
	// OAuthClientTypePublic 公开客户端：无 secret，授权码流程全程 PKCE（S256）。
	OAuthClientTypePublic = "public"
)

// ClientTypeOf 由 secret 哈希推导客户端类型（public 客户端不落 secret）。
func ClientTypeOf(c *entity.McpOAuthClient) string {
	if c == nil || c.ClientSecretHash == "" {
		return OAuthClientTypePublic
	}
	return OAuthClientTypeConfidential
}

// AdminListClients 管理端客户端分页列表（keyword 模糊匹配 client_name / client_id）。
// 返回 (行, 各 client 未吊销令牌数, 总数)——令牌数富化供 UI 展示 tokenCount。
func (s *McpOAuthService) AdminListClients(ctx context.Context, keyword string, pn, ps int) ([]*entity.McpOAuthClient, map[string]int64, int64, error) {
	list, total, err := s.clients.List(ctx, keyword, pn, ps)
	if err != nil {
		return nil, nil, 0, err
	}
	ids := make([]string, 0, len(list))
	for _, e := range list {
		ids = append(ids, e.ClientId)
	}
	counts, err := s.tokens.CountActiveByClientIds(ctx, ids)
	if err != nil {
		return nil, nil, 0, err
	}
	return list, counts, total, nil
}

// AdminCreateClient 管理端注册客户端，返回 (clientId, clientSecret 明文仅此一次)。
// clientType 缺省 confidential；public 类型无 secret（PKCE 全程），secret 返回空串不返回给前端。
func (s *McpOAuthService) AdminCreateClient(ctx context.Context, clientName, clientType string, redirectUris, grantTypes, scopes []string, enabled bool) (string, string, error) {
	if len(redirectUris) == 0 {
		return "", "", errors.New("redirectUris 不能为空")
	}
	if clientType == "" {
		clientType = OAuthClientTypeConfidential
	}
	if clientType != OAuthClientTypeConfidential && clientType != OAuthClientTypePublic {
		return "", "", errors.New("clientType 仅支持 confidential / public")
	}
	if clientName == "" {
		clientName = "unnamed-client"
	}
	if len(grantTypes) == 0 {
		grantTypes = []string{OAuthGrantAuthorizationCode, OAuthGrantRefreshToken}
	}
	ru, err := json.Marshal(redirectUris)
	if err != nil {
		return "", "", err
	}
	gt, err := json.Marshal(grantTypes)
	if err != nil {
		return "", "", err
	}
	secret := ""
	secretHash := "" // public 类型落空串（非 sha256("")，空串即「无 secret」标记）
	if clientType == OAuthClientTypeConfidential {
		if secret, err = crypto.RandomKey("mcp"); err != nil {
			return "", "", err
		}
		secretHash = crypto.HashToken(secret)
	}
	e := &entity.McpOAuthClient{
		ClientName:       clientName,
		ClientSecretHash: secretHash,
		RedirectUris:     string(ru),
		GrantTypes:       string(gt),
		Scope:            strings.Join(scopes, OAuthScopeSeparator), // 空格分隔白名单（空=不限）
		IsEnabled:        boolUint(enabled),
	}
	clientId, err := s.clients.Add(ctx, e)
	if err != nil {
		return "", "", err
	}
	return clientId, secret, nil
}

// AdminEditClient 管理端编辑客户端：数组字段传 nil=未传不动，传了（含空数组）即整组覆盖。
// clientType 变更同步 secret 口径：→ public 清空哈希（旧 secret 立即失效）；→ confidential 且
// 原为 public 时生成新哈希（明文不随编辑返回，经 rotate-secret 获取）。
// enabled 传 nil=不动。返回更新后的行供控制器回显。redirect_uris 是授权码流程回调白名单
// （防开放重定向的安全底线），显式传空数组视为非法，拒绝保存。
func (s *McpOAuthService) AdminEditClient(ctx context.Context, clientId, clientName, clientType string,
	redirectUris, grantTypes, scopes []string, enabled *bool) (*entity.McpOAuthClient, error) {
	if clientId == "" {
		return nil, errors.New("clientId 不能为空")
	}
	e, err := s.clients.GetByClientIdAll(ctx, clientId)
	if err != nil || e == nil {
		return nil, ErrOAuthClientUnknown
	}
	if clientType != "" {
		if clientType != OAuthClientTypeConfidential && clientType != OAuthClientTypePublic {
			return nil, errors.New("clientType 仅支持 confidential / public")
		}
		switch {
		case clientType == OAuthClientTypePublic && e.ClientSecretHash != "":
			e.ClientSecretHash = "" // 转 public：清空哈希，旧 secret 立即失效
		case clientType == OAuthClientTypeConfidential && e.ClientSecretHash == "":
			secret, err := crypto.RandomKey("mcp")
			if err != nil {
				return nil, err
			}
			e.ClientSecretHash = crypto.HashToken(secret) // 转 confidential：先落哈希，明文经 rotate-secret 获取
		}
	}
	if clientName != "" {
		e.ClientName = clientName
	}
	if redirectUris != nil {
		if len(redirectUris) == 0 {
			return nil, errors.New("redirectUris 不能为空（回调白名单不可清空）")
		}
		b, err := json.Marshal(redirectUris)
		if err != nil {
			return nil, err
		}
		e.RedirectUris = string(b)
	}
	if grantTypes != nil {
		b, err := json.Marshal(grantTypes)
		if err != nil {
			return nil, err
		}
		e.GrantTypes = string(b)
	}
	if scopes != nil {
		e.Scope = strings.Join(scopes, OAuthScopeSeparator) // 空数组=不限 scope
	}
	if enabled != nil {
		e.IsEnabled = boolUint(*enabled)
	}
	if err := s.clients.SaveAll(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// AdminRotateSecret 管理端轮换 secret：生成新明文（仅此一次返回），落库只存哈希，
// 旧 secret 立即失效。public 类型无 secret，无从轮换——直接报错。
func (s *McpOAuthService) AdminRotateSecret(ctx context.Context, clientId string) (string, string, error) {
	e, err := s.clients.GetByClientIdAll(ctx, clientId)
	if err != nil || e == nil {
		return "", "", ErrOAuthClientUnknown
	}
	if e.ClientSecretHash == "" {
		return "", "", errors.New("public 类型客户端无 secret，请走 PKCE")
	}
	secret, err := crypto.RandomKey("mcp")
	if err != nil {
		return "", "", err
	}
	e.ClientSecretHash = crypto.HashToken(secret)
	if err := s.clients.SaveAll(ctx, e); err != nil {
		return "", "", err
	}
	return e.ClientId, secret, nil
}

// AdminDeleteClients 管理端删除客户端（按协议 client_id 批量）：
// 先级联吊销其全部未吊销令牌（access+refresh 一并作废），再删客户端行。
func (s *McpOAuthService) AdminDeleteClients(ctx context.Context, clientIds []string) error {
	if len(clientIds) == 0 {
		return nil
	}
	if err := s.tokens.RevokeByClientIds(ctx, clientIds); err != nil {
		return err
	}
	return s.clients.DeleteByClientIds(ctx, clientIds)
}

// AdminListTokens 管理端令牌分页列表（clientId 空=全部，id 倒序）。
func (s *McpOAuthService) AdminListTokens(ctx context.Context, clientId string, pn, ps int) ([]*entity.McpOAuthToken, int64, error) {
	return s.tokens.List(ctx, clientId, pn, ps)
}

// AdminTokenEnrich 令牌列表富化：按 client_id 集合查客户端名、按 bearer_key_id 集合查
// 授权主体（关联 Bearer Key 的名称，已删除的 key 查不到则不进 map，前端展示留空）。
func (s *McpOAuthService) AdminTokenEnrich(ctx context.Context, clientIds, keyIds []string) (map[string]string, map[string]string, error) {
	clientNames := make(map[string]string)
	keyNames := make(map[string]string)
	if len(clientIds) > 0 {
		rows, err := s.clients.ListByClientIds(ctx, clientIds)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			clientNames[r.ClientId] = r.ClientName
		}
	}
	// key 仓储无批量接口，逐个去重查（单页量小，够用）。
	seen := make(map[string]struct{}, len(keyIds))
	for _, id := range keyIds {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if k, err := s.keys.Get(ctx, id); err == nil && k != nil {
			keyNames[id] = k.Name
		}
	}
	return clientNames, keyNames, nil
}

// AdminRevokeToken 管理端吊销令牌（按对外令牌 id = biz_id）：revoked_at 置 now，
// 整行 access+refresh 一并作废。已吊销直接幂等成功。
func (s *McpOAuthService) AdminRevokeToken(ctx context.Context, tokenId string) error {
	if tokenId == "" {
		return errors.New("tokenId 不能为空")
	}
	row, err := s.tokens.GetByBizId(ctx, tokenId)
	if err != nil || row == nil {
		return errors.New("令牌不存在")
	}
	if row.RevokedAt != nil {
		return nil
	}
	return s.tokens.Revoke(ctx, row.Id)
}

// boolUint 布尔转库内 smallint 开关位（1/0）。
func boolUint(b bool) uint {
	if b {
		return 1
	}
	return 0
}
