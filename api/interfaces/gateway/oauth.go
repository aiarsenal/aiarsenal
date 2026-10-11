// Package gateway OAuth 授权端点（MCP 规范 2025-06-18 Authorization）。
// 授权主体复用既有 Bearer Key：同意页粘贴 key 明文完成身份验证 → 签发授权码 →
// 换 access/refresh token；此后 access token 经 BearerAuth 等效其关联 key 鉴权。
// 端点（路由注册在 cmd/mcp/main.go，见报告 INTEGRATION 节，本包只提供实现与注册入口）：
//
//	GET  /.well-known/oauth-authorization-server  授权服务器元数据（RFC 8414）
//	POST /oauth/register                          动态客户端注册（RFC 7591）
//	GET  /oauth/authorize                         简易同意页（粘贴已有 Bearer Key）
//	POST /oauth/authorize                         提交同意 → 签发 code（302 回 redirect_uri）
//	POST /oauth/token                             code 换令牌 / refresh 刷新（RFC 6749 §5 JSON 错误）
package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/crypto"
	"aiarsenal/infrastructure/persistence"
)

// oauthShared 网关进程内单份 OAuth 服务：鉴权中间件与 /oauth/* 端点共用一份
// （服务无状态、数据在 mcp 库；懒建兜底——cmd/mcp 未显式装配时首次请求自建）。
var oauthShared struct {
	once sync.Once
	svc  *service.McpOAuthService
}

// OAuthService 取网关单份 OAuth 服务。keySvc 为首次调用时的 Bearer Key 服务
// （cmd/mcp 只有一份，先传先建）。
func OAuthService(keySvc *service.McpKeyService) *service.McpOAuthService {
	oauthShared.once.Do(func() {
		oauthShared.svc = service.NewMcpOAuthService(
			persistence.NewMcpOAuthClientRepository(),
			persistence.NewMcpOAuthCodeRepository(),
			persistence.NewMcpOAuthTokenRepository(),
			keySvc)
	})
	return oauthShared.svc
}

// RegisterOAuthRoutes 注册 OAuth 授权端点（无需 Bearer 鉴权——注册/同意/换令牌自身即鉴权）。
// cmd/mcp/main.go 装配：gateway.RegisterOAuthRoutes(r, keySvc)。
func RegisterOAuthRoutes(r gin.IRouter, keySvc *service.McpKeyService) {
	oauthSvc := OAuthService(keySvc)
	g := r.Group("")
	g.Use(oauthCORS)
	g.GET("/.well-known/oauth-authorization-server", oauthMetadata)
	g.POST("/oauth/register", oauthRegister(oauthSvc))
	g.GET("/oauth/authorize", oauthAuthorizePage(oauthSvc))
	g.POST("/oauth/authorize", oauthAuthorizeSubmit(oauthSvc, keySvc))
	g.POST("/oauth/token", oauthToken(oauthSvc))
	g.OPTIONS("/oauth/*any", func(c *gin.Context) { c.Status(http.StatusNoContent) })
}

// oauthCORS 元数据/端点放行跨域匿名访问（浏览器侧 MCP 客户端发现与探测需要）。
func oauthCORS(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
	c.Next()
}

// oauthIssuer 元数据 issuer（RFC 8414 要求与授权端点同源根）：
// env(MCP_OAUTH_ISSUER) > config.McpBaseURL()（如 https://mcp.example.com）> 请求 Host。
func oauthIssuer(c *gin.Context) string {
	if u := strings.TrimRight(strings.TrimSpace(os.Getenv("MCP_OAUTH_ISSUER")), "/"); u != "" {
		return u
	}
	if u := strings.TrimRight(strings.TrimSpace(config.Get().McpBaseURL()), "/"); u != "" {
		return u
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// oauthMetadata GET /.well-known/oauth-authorization-server。
func oauthMetadata(c *gin.Context) {
	iss := oauthIssuer(c)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                iss,
		"authorization_endpoint":                iss + "/oauth/authorize",
		"token_endpoint":                        iss + "/oauth/token",
		"registration_endpoint":                 iss + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{service.OAuthGrantAuthorizationCode, service.OAuthGrantRefreshToken},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"}, // PKCE 强制，仅 S256
	})
}

// oauthRegister POST /oauth/register（RFC 7591 动态注册，201 返回 client_id/secret）。
func oauthRegister(oauthSvc *service.McpOAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClientName   string   `json:"client_name"`
			RedirectUris []string `json:"redirect_uris"`
			GrantTypes   []string `json:"grant_types"`
			Scope        string   `json:"scope"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": err.Error()})
			return
		}
		clientId, secret, err := oauthSvc.RegisterClient(c.Request.Context(), req.ClientName, req.RedirectUris, req.GrantTypes, req.Scope)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": err.Error()})
			return
		}
		// 回显以落库为准（client_name/grant_types 缺省值由服务层补齐，RFC 7591 §3.2.1）。
		name, uris, grants := req.ClientName, req.RedirectUris, req.GrantTypes
		if c2, e := oauthSvc.GetClient(c.Request.Context(), clientId); e == nil && c2 != nil {
			name = c2.ClientName
			if arr := oauthJSONStrings(c2.RedirectUris); len(arr) > 0 {
				uris = arr
			}
			if arr := oauthJSONStrings(c2.GrantTypes); len(arr) > 0 {
				grants = arr
			}
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusCreated, gin.H{
			"client_id":                  clientId,
			"client_secret":              secret, // 明文仅此一次，落库只存 sha256
			"client_id_issued_at":        time.Now().Unix(),
			"client_name":                name,
			"redirect_uris":              uris,
			"grant_types":                grants,
			"token_endpoint_auth_method": "client_secret_basic",
		})
	}
}

// oauthJSONStrings jsonb 字符串数组列解析（注册响应回显落库值用；与服务层 parseJSONStrings 同源写法）。
func oauthJSONStrings(jsonArr string) []string {
	if jsonArr == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonArr), &arr); err != nil {
		return nil
	}
	return arr
}

// ============ 同意页（简易授权页：粘贴已有 Bearer Key 完成身份验证） ============

// oauthAuthzParams 授权请求参数集（GET 渲染表单 / POST 表单回传两程共用；
// 经 Request.FormValue 读取——GET 取 query、POST 取 body，字段一致）。
type oauthAuthzParams struct {
	responseType        string
	clientId            string
	redirectUri         string
	scope               string
	state               string
	codeChallenge       string
	codeChallengeMethod string
}

func oauthAuthzFrom(c *gin.Context) oauthAuthzParams {
	return oauthAuthzParams{
		responseType:        c.Request.FormValue("response_type"),
		clientId:            c.Request.FormValue("client_id"),
		redirectUri:         c.Request.FormValue("redirect_uri"),
		scope:               c.Request.FormValue("scope"),
		state:               c.Request.FormValue("state"),
		codeChallenge:       c.Request.FormValue("code_challenge"),
		codeChallengeMethod: c.Request.FormValue("code_challenge_method"),
	}
}

// oauthValidateAuthz 授权请求语义校验，返回 (client, 错误文案)（文案空=通过）。
func oauthValidateAuthz(ctx context.Context, oauthSvc *service.McpOAuthService, p oauthAuthzParams) (*entity.McpOAuthClient, string) {
	if p.responseType != "code" {
		return nil, "response_type 仅支持 code"
	}
	if p.clientId == "" || p.redirectUri == "" {
		return nil, "缺少 client_id 或 redirect_uri"
	}
	client, err := oauthSvc.GetClient(ctx, p.clientId)
	if err != nil {
		return nil, "未知 client_id"
	}
	if !service.RedirectAllowed(client, p.redirectUri) {
		// 未注册的回调地址绝不 302（防开放重定向），只渲染错误页。
		return nil, "redirect_uri 未在客户端注册列表中"
	}
	if p.codeChallenge == "" {
		return nil, "PKCE code_challenge 必填（仅支持 S256）"
	}
	if p.codeChallengeMethod != "" && p.codeChallengeMethod != "S256" {
		return nil, "code_challenge_method 仅支持 S256"
	}
	return client, ""
}

// oauthAuthorizePage GET /oauth/authorize：渲染同意页。
// 校验失败（未知 client / 未注册 redirect_uri 等）直接渲染错误页，绝不 302 到来路。
func oauthAuthorizePage(oauthSvc *service.McpOAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := oauthAuthzFrom(c)
		if _, msg := oauthValidateAuthz(c.Request.Context(), oauthSvc, p); msg != "" {
			oauthHTML(c, http.StatusBadRequest, "授权请求无效", msg)
			return
		}
		oauthHTMLForm(c, p, "")
	}
}

// oauthAuthorizeSubmit POST /oauth/authorize：校验 Bearer Key → 签发 code → 302 回 redirect_uri。
// key 无效/停用：重渲染同意页并提示（不跳转，不签发）。
func oauthAuthorizeSubmit(oauthSvc *service.McpOAuthService, keySvc *service.McpKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := oauthAuthzFrom(c)
		client, msg := oauthValidateAuthz(c.Request.Context(), oauthSvc, p)
		if msg != "" {
			oauthHTML(c, http.StatusBadRequest, "授权请求无效", msg)
			return
		}
		keyPlain := c.PostForm("bearer_key")
		if keyPlain == "" {
			oauthHTMLForm(c, p, "请粘贴已有 Bearer Key 完成身份验证")
			return
		}
		key, err := keySvc.GetByHash(c.Request.Context(), crypto.HashToken(keyPlain))
		if err != nil || key == nil || key.Enabled == 0 {
			oauthHTMLForm(c, p, "Bearer Key 无效或已停用")
			return
		}
		code, err := oauthSvc.IssueCode(c.Request.Context(), client, key, p.redirectUri, p.scope, p.codeChallenge)
		if err != nil {
			oauthHTML(c, http.StatusBadRequest, "签发授权码失败", err.Error())
			return
		}
		// 302 回 redirect_uri：code + state（RFC 6749 §4.1.2）。
		target, _ := url.Parse(p.redirectUri)
		q := target.Query()
		q.Set("code", code)
		if p.state != "" {
			q.Set("state", p.state)
		}
		target.RawQuery = q.Encode()
		c.Redirect(http.StatusFound, target.String())
	}
}

// ============ 令牌端点 ============

// oauthToken POST /oauth/token：authorization_code 换令牌 / refresh_token 刷新。
// 错误一律 RFC 6749 §5.2 JSON（error + error_description）。
func oauthToken(oauthSvc *service.McpOAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		clientId, clientSecret := oauthClientCredentials(c)
		client, err := oauthSvc.AuthenticateClient(c.Request.Context(), clientId, clientSecret)
		if err != nil {
			oauthTokenError(c, http.StatusUnauthorized, "invalid_client", err.Error())
			return
		}
		switch c.PostForm("grant_type") {
		case service.OAuthGrantAuthorizationCode:
			row, access, refresh, err := oauthSvc.ExchangeCode(c.Request.Context(), client,
				c.PostForm("code"), c.PostForm("redirect_uri"), c.PostForm("code_verifier"))
			oauthTokenResult(c, row, access, refresh, err)
		case service.OAuthGrantRefreshToken:
			row, access, refresh, err := oauthSvc.Refresh(c.Request.Context(), client, c.PostForm("refresh_token"))
			oauthTokenResult(c, row, access, refresh, err)
		case "":
			oauthTokenError(c, http.StatusBadRequest, "invalid_request", "缺少 grant_type")
		default:
			oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", "不支持的 grant_type: "+c.PostForm("grant_type"))
		}
	}
}

// oauthTokenResult 令牌响应（成功）/ RFC 6749 错误码映射（失败）。
func oauthTokenResult(c *gin.Context, row *entity.McpOAuthToken, access, refresh string, err error) {
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOAuthClientUnknown), errors.Is(err, service.ErrOAuthClientAuth):
			oauthTokenError(c, http.StatusUnauthorized, "invalid_client", err.Error())
		case errors.Is(err, service.ErrOAuthGrantUnsupported):
			oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", err.Error())
		case errors.Is(err, service.ErrOAuthScopeInvalid):
			oauthTokenError(c, http.StatusBadRequest, "invalid_scope", err.Error())
		case errors.Is(err, service.ErrOAuthGrantInvalid), errors.Is(err, service.ErrOAuthRedirectInvalid),
			errors.Is(err, service.ErrOAuthVerifierInvalid):
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", err.Error())
		default:
			oauthTokenError(c, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}
	expiresIn := int64(0)
	if row.ExpiresAt != nil {
		expiresIn = int64(time.Until(time.Time(*row.ExpiresAt)).Seconds())
		if expiresIn < 0 {
			expiresIn = 0
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refresh,
		"scope":         row.Scope,
	})
}

// oauthClientCredentials 取客户端凭据：Authorization: Basic（RFC 6749 §2.3.1）优先，
// 回落表单 client_id/client_secret（client_secret_post）。
func oauthClientCredentials(c *gin.Context) (id, secret string) {
	if h := c.GetHeader("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "basic") {
			if raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1])); err == nil {
				if i := strings.IndexByte(string(raw), ':'); i >= 0 {
					// RFC 要求凭据先 form-urlencoded 再拼接，解码兼容未编码的旧客户端。
					id, _ = url.QueryUnescape(string(raw)[:i])
					secret, _ = url.QueryUnescape(string(raw)[i+1:])
				}
			}
		}
	}
	if id == "" {
		id = c.PostForm("client_id")
		secret = c.PostForm("client_secret")
	}
	return id, secret
}

func oauthTokenError(c *gin.Context, status int, code, desc string) {
	c.Header("Cache-Control", "no-store")
	if code == "invalid_client" {
		c.Header("WWW-Authenticate", `Basic realm="aiarsenal-mcp-gateway"`)
	}
	c.JSON(status, gin.H{"error": code, "error_description": desc})
}

// ============ 简易 HTML 页面 ============

// oauthHTML 错误页（不重定向——来路不可信）。
func oauthHTML(c *gin.Context, status int, title, msg string) {
	c.Data(status, "text/html; charset=utf-8", []byte(fmt.Sprintf(`<!doctype html>
<html lang="zh"><head><meta charset="utf-8"><title>%s</title></head>
<body style="font-family:sans-serif;max-width:32rem;margin:4rem auto;">
<h2>%s</h2><p style="color:#b00;">%s</p>
<p><a href="/">返回</a></p></body></html>`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(msg))))
}

// oauthHTMLForm 同意页：展示授权请求摘要，表单粘贴已有 Bearer Key 完成身份验证。
// 各授权参数随隐藏字段回传 POST /oauth/authorize（FormValue 两程同读）。
func oauthHTMLForm(c *gin.Context, p oauthAuthzParams, errMsg string) {
	hidden := ""
	for k, v := range map[string]string{
		"response_type":         p.responseType,
		"client_id":             p.clientId,
		"redirect_uri":          p.redirectUri,
		"scope":                 p.scope,
		"state":                 p.state,
		"code_challenge":        p.codeChallenge,
		"code_challenge_method": p.codeChallengeMethod,
	} {
		if v == "" {
			continue
		}
		hidden += fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`+"\n", k, html.EscapeString(v))
	}
	errBox := ""
	if errMsg != "" {
		errBox = `<p style="color:#b00;">` + html.EscapeString(errMsg) + `</p>`
	}
	page := fmt.Sprintf(`<!doctype html>
<html lang="zh"><head><meta charset="utf-8"><title>MCP 授权</title></head>
<body style="font-family:sans-serif;max-width:32rem;margin:4rem auto;">
<h2>授权应用访问 MCP 网关</h2>
<p>应用 <b>%s</b> 请求访问 MCP 网关（回调 %s%s）。</p>
<p>请粘贴你的 <b>已有 Bearer Key</b> 完成身份验证。授权后签发的令牌等效该 Key 的权限范围
（该 Key 被删除或停用时，对应令牌立即失效）。</p>
%s
<form method="POST" action="/oauth/authorize">
%s
<label>Bearer Key<br><input type="password" name="bearer_key" style="width:100%%" autocomplete="off" required></label><br><br>
<button type="submit">同意并授权</button>
</form></body></html>`,
		html.EscapeString(p.clientId), html.EscapeString(p.redirectUri), scopeSuffix(p.scope), errBox, hidden)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

// scopeSuffix 同意页 scope 摘要后缀（空则不展示）。
func scopeSuffix(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}
	return "，申请 scope：" + scope
}
