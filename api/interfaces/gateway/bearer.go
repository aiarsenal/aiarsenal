package gateway

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/crypto"
)

// MCP Bearer 鉴权中间件。
// 从 Authorization: Bearer <token> 取明文 → sha256 哈希 → 查 aa_mcp_bearer_key(enabled=1)；
// 直连未命中再回落 OAuth access token（sha256 命中 aa_mcp_oauth_token 且未过期/未吊销
// → 等效其关联 Bearer Key 鉴权，主体 key 删除/停用即失效）。校验 access_type 范围是否
// 覆盖请求路径的 scope（$smart / 空路径仍按原口径：仅 AccessType=all 放行，工具面在
// sse 层收窄为 meta-tool）。通过后写 ctx。

const (
	ctxKey  = "mcpBearerKey"
	ctxScp  = "mcpScope"
	ctxKind = "mcpScopeKind"
)

func BearerAuth(keySvc *service.McpKeyService, groupSvc *service.McpGroupService, serverSvc *service.McpServerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "missing bearer token"})
			return
		}
		hash := crypto.HashToken(token)
		k, err := keySvc.GetByHash(c.Request.Context(), hash)
		if err != nil || k == nil {
			// OAuth access token 回落：等效其关联 Bearer Key（scope/access_type/工具面按 key 实时判定）。
			k, err = OAuthService(keySvc).ResolveAccessToken(c.Request.Context(), hash)
			if err != nil || k == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid token"})
				return
			}
		}
		scope, kind := resolveScope(c, groupSvc, serverSvc)
		if !keySvc.Authorize(k, scope, kind) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "key 无权访问该 scope: " + scope})
			return
		}
		c.Set(ctxKey, k)
		c.Set(ctxScp, scope)
		c.Set(ctxKind, kind)
		c.Next()
	}
}

func bearerToken(h string) string {
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// resolveScope 从 /mcp/:scope 提取 scope 与分类（group 优先，与 sse.resolveScope 同序）——
// Authorize 的 groups/servers 分支按分类匹配：分组端点必须归 "group"，否则 groups 型 key
// （实例/设备绑定签发的 AccessType=groups）连自己的分组端点也 403（端点内并无二次判定）。
// 空或 $smart → 全部；两个名单都未命中也按 server 报（路由层稍后以「scope 未找到」拒绝）。
func resolveScope(c *gin.Context, groupSvc *service.McpGroupService, serverSvc *service.McpServerService) (string, string) {
	scope := c.Param("scope")
	if scope == "" || scope == "$smart" {
		return "", ""
	}
	ctx := c.Request.Context()
	if g, e := groupSvc.GetByName(ctx, scope); e == nil && g != nil {
		return scope, "group"
	}
	if sv, e := serverSvc.GetByName(ctx, scope); e == nil && sv != nil {
		return scope, "server"
	}
	return scope, "server"
}

func GetKey(c *gin.Context) *entity.McpBearerKey {
	v, ok := c.Get(ctxKey)
	if !ok {
		return nil
	}
	k, _ := v.(*entity.McpBearerKey)
	return k
}

func GetScope(c *gin.Context) (string, string) {
	s, _ := c.Get(ctxScp)
	k, _ := c.Get(ctxKind)
	ss, _ := s.(string)
	kk, _ := k.(string)
	return ss, kk
}
