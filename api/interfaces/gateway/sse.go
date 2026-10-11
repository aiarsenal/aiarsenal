// Package gateway MCP 协议网关入口（api 模块第三 main：cmd/mcp）。
// 把聚合后的 tools/prompts/resources 通过 streamable-http 暴露给 AI 客户端，
// 路由 /mcp / /mcp/{group} / /mcp/{server}；$smart 为向量智能路由端点
// （仅暴露 meta-tool tool_search，见 addSmartTool），OAuth 授权端点见 oauth.go。
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/crypto"
	"aiarsenal/infrastructure/mcpgateway/compression"
	"aiarsenal/infrastructure/mcpgateway/registry"
	"aiarsenal/infrastructure/persistence"
)

// ctxAudit 随 request context 传递的调用归属（bearer key + 来源 IP），
// 供工具转发闭包在 streamable-http 会话内取用落审计。
type ctxAudit struct {
	key *entity.McpBearerKey
	ip  string
}

type ctxAuditKey struct{}

type Service struct {
	serverSvc *service.McpServerService
	groupSvc  *service.McpGroupService
	builtin   *service.McpBuiltinService
	activity  *service.McpActivityService
	compress  *compression.Service
	registry  *registry.Service
	signKey   string // 令牌交换签名密钥（空 = 不签名透传）

	// handlers 按 scope 缓存 StreamableHTTPHandler：会话表挂在 handler 实例上，
	// 每请求新建会让 initialize 建的会话在下一个 POST 里 session not found。
	handlers sync.Map // scope string → *mcp.StreamableHTTPHandler
}

func New(serverSvc *service.McpServerService, groupSvc *service.McpGroupService,
	builtin *service.McpBuiltinService, activity *service.McpActivityService,
	compress *compression.Service, reg *registry.Service, signKey string) *Service {
	return &Service{serverSvc: serverSvc, groupSvc: groupSvc, builtin: builtin,
		activity: activity, compress: compress, registry: reg, signKey: signKey}
}

// handlerFor 取/建 scope 级 StreamableHTTPHandler（进程内复用）。
// getServer 在每个新会话建立时被调：现场重新解析 scope 成员——分组/启停变更
// 由新会话自然生效；存量会话保持建立时的工具视图（客户端 DELETE 重连即可刷新）。
func (s *Service) handlerFor(scope string) *mcp.StreamableHTTPHandler {
	if v, ok := s.handlers.Load(scope); ok {
		return v.(*mcp.StreamableHTTPHandler)
	}
	v, _ := s.handlers.LoadOrStore(scope, mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			names, _, err := s.resolveScope(r.Context(), scope)
			if err != nil {
				return nil
			}
			s.ensureConnected(r.Context(), names)
			return s.buildServer(scope, names)
		}, &mcp.StreamableHTTPOptions{DisableLocalhostProtection: true}))
	// DisableLocalhostProtection：go-sdk≥1.4 默认开 DNS 重绑定防护——「loopback 到站 + 非 loopback
	// Host 头」即 403 Forbidden: invalid Host header。本网关是跨主机消费的鉴权服务而非本地服务：
	// 前面有 BearerAuth（token 哈希查库 + scope 授权），浏览器裸打的 DNS 重绑定场景拿不出 Bearer key；
	// 而合法链路恰好全中误报——colima/slirp 下容器连 host.docker.internal:43213 被折成宿主 127.0.0.1
	// 到站（Host 头仍是 host.docker.internal），nginx 同机反代（127.0.0.1 到站 + 域名 Host）同理。
	return v.(*mcp.StreamableHTTPHandler)
}

// Endpoint 处理 /mcp / /mcp/:scope（GET=SSE、POST=streamable-http、DELETE=shutdown）。
func (s *Service) Endpoint(c *gin.Context) {
	scope := c.Param("scope")
	names, _, err := s.resolveScope(c.Request.Context(), scope)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.ensureConnected(c.Request.Context(), names)

	// 把调用归属（key/IP）塞进 request context，工具转发闭包从 ctx 取——
	// streamable-http 会话可跨多个 POST 复用，按“触发调用的那次请求”归属最准。
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(),
		ctxAuditKey{}, ctxAudit{key: GetKey(c), ip: c.ClientIP()}))

	// 令牌交换：凡有归属人的 key（user 自建 / agent 挂实例 / client 挂设备）一律把
	// 调用方身份签名进 ctx，headerRT 出站注入 X-AA-Actor——第二跳（ops）验签后按
	// 归属用户的五元组收紧数据可见性。key 挂在什么载体上不影响权限，权限永远跟人走。
	// 无归属 key / 未配置密钥不签：ops 侧回落 system 身份（loopback 信任）。
	if k := GetKey(c); k != nil && k.Owner != "" && s.signKey != "" {
		if pl, sig, err := crypto.SignActor(s.signKey, k.KeyId, k.Owner, k.Kind, time.Now()); err == nil {
			c.Request = c.Request.WithContext(registry.WithActor(c.Request.Context(), pl, sig))
		}
	}

	s.handlerFor(scope).ServeHTTP(c.Writer, c.Request)
}

func (s *Service) Shutdown(c *gin.Context) {
	scope := c.Param("scope")
	if scope != "" && !strings.Contains(scope, "/") {
		// 失效 scope 级 handler 缓存 + 停 stdio 子进程：下次连接按最新配置重建（热换入口）。
		s.handlers.Delete(scope)
		_ = s.registry.Stop(c.Request.Context(), scope)
	}
	c.Status(http.StatusOK)
}

func (s *Service) ensureConnected(ctx context.Context, names []string) {
	for _, name := range names {
		if _, ok := s.registry.Get(name); !ok {
			if srv, e := s.serverSvc.GetByName(ctx, name); e == nil {
				_ = s.registry.Start(ctx, srv)
			}
		}
	}
}

// buildServer 构造下游 MCP server：聚合各 server 的 tools（命名空间 server__tool）+ 内置 prompt/resource。
// scope=$smart 走向量智能路由（ToolSearch 模式）：工具面不再全量罗列，只暴露一个
// meta-tool「tool_search」（pgvector 余弦检索全部已聚合工具），instructions 指引
// 调用方先检索、再改连 /mcp/{server} 使用原生工具。
func (s *Service) buildServer(scope string, names []string) *mcp.Server {
	opts := &mcp.ServerOptions{}
	if scope == "$smart" {
		opts.Instructions = smartInstructions
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp", Version: "1.0"}, opts)
	if scope == "$smart" {
		s.addSmartTool(srv)
	} else {
		s.addAggregatedTools(srv, names)
	}
	s.addBuiltinPrompts(srv)
	s.addBuiltinResources(srv)
	return srv
}

// smartInstructions $smart 端点的 server instructions（tool_search 用法说明）。
const smartInstructions = "本端点为 $smart 向量智能路由模式：不罗列全部上游工具，只提供 meta-tool「tool_search」。" +
	"用法：先用自然语言调用 tool_search（入参 query，可选 k，缺省 8）按向量相似度检索目标工具，" +
	"命中结果给出 server__tool 全名、描述与所属 server；随后改连 /mcp/{server} 端点（Bearer 鉴权不变）" +
	"直接使用该 server 的原生工具。"

// toolSearchShared 网关进程内单份工具向量检索服务（$smart meta-tool 后端；懒建兜底——
// cmd/mcp 未显式装配时首次调用自建，数据源 aa_mcp_tool_embedding）。
var toolSearchShared struct {
	once sync.Once
	svc  *service.McpToolSearchService
}

func toolSearchSvc() *service.McpToolSearchService {
	toolSearchShared.once.Do(func() {
		toolSearchShared.svc = service.NewMcpToolSearchService(persistence.NewMcpToolEmbeddingRepository())
	})
	return toolSearchShared.svc
}

// addSmartTool $smart 端点的 meta-tool：pgvector 余弦相似度检索全部已聚合工具。
// 入参 query（必填）+ k（缺省 8）；返回 server__tool 全名 + 描述 + 所属 server，
// 并引导改连 /mcp/{server} 直接使用原生工具。
func (s *Service) addSmartTool(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name: service.SmartToolName,
		Description: "按自然语言检索本网关已聚合的全部 MCP 工具（向量相似度 TopK）。" +
			"返回工具全名（server__tool）、描述与所属 server；确定目标后请改连 /mcp/{server} 端点直接调用其原生工具。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "自然语言检索词（工具名/功能描述均可）"},
				"k":     map[string]any{"type": "integer", "description": "返回条数，缺省 8"},
			},
			"required": []string{"query"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Query string `json:"query"`
			K     int    `json:"k"`
		}
		if b, err := json.Marshal(req.Params.Arguments); err == nil {
			_ = json.Unmarshal(b, &in)
		}
		hits, err := toolSearchSvc().Search(ctx, in.Query, in.K)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: service.RenderText(in.Query, hits)}},
		}, nil
	})
}

func (s *Service) addAggregatedTools(srv *mcp.Server, names []string) {
	for _, name := range names {
		e, ok := s.registry.Get(name)
		if !ok {
			continue
		}
		for _, t := range e.Tools() {
			tool := *t // copy
			tool.Name = name + "__" + tool.Name
			serverName, toolName := name, t.Name
			srv.AddTool(&tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				start := time.Now()
				in, _ := json.Marshal(req.Params.Arguments)
				// 令牌交换签名须每次调用现签：会话 ctx 继承自「建连那次请求」（go-sdk
				// server.Connect(req.Context()) 后 jsonrpc2 分离取消、值保留），Endpoint 塞进
				// 去的签名是建连时刻的，60s TTL——常驻客户端（hermes 数小时）第一次调用之后
				// 全带着过期签名到 ops，验签失败回落 system 身份=权限判定全放行（知识库
				// exclude 文档经 MCP 检索仍可见实踩）。key 归属实体不过期，取出来重签即可。
				if a, _ := ctx.Value(ctxAuditKey{}).(ctxAudit); a.key != nil && a.key.Owner != "" && s.signKey != "" {
					if pl, sig, err := crypto.SignActor(s.signKey, a.key.KeyId, a.key.Owner, a.key.Kind, time.Now()); err == nil {
						ctx = registry.WithActor(ctx, pl, sig)
					}
				}
				out, isToolErr, err := s.registry.CallTool(ctx, serverName, toolName, req.Params.Arguments)
				a, _ := ctx.Value(ctxAuditKey{}).(ctxAudit)
				if err != nil {
					s.RecordActivity(ctx, a.key, a.ip, serverName, toolName, "error", err.Error(),
						int(time.Since(start).Milliseconds()), truncStr(string(in), 2000), "")
					return nil, err
				}
				if isToolErr {
					// 工具级失败：内容回传给客户端（isError=true），审计按 error 记
					s.RecordActivity(ctx, a.key, a.ip, serverName, toolName, "error", truncStr(out, 500),
						int(time.Since(start).Milliseconds()), truncStr(string(in), 2000), truncStr(out, 2000))
					return &mcp.CallToolResult{IsError: true,
						Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil
				}
				s.RecordActivity(ctx, a.key, a.ip, serverName, toolName, "success", "",
					int(time.Since(start).Milliseconds()), truncStr(string(in), 2000), truncStr(out, 2000))
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil
			})
		}
	}
}

func (s *Service) addBuiltinPrompts(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{Name: "builtin_prompts", Description: "List all builtin prompts"},
		func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			all, _ := s.builtin.AllPrompts(ctx)
			var b strings.Builder
			for _, p := range all {
				b.WriteString(p.Name + ": " + p.Description + "\n")
			}
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{{Role: "assistant", Content: &mcp.TextContent{Text: b.String()}}},
			}, nil
		})
}

func (s *Service) addBuiltinResources(srv *mcp.Server) {
	srv.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: "aa-mcp://builtin/{name}", Name: "Builtin Resource"},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			all, _ := s.builtin.AllResources(ctx)
			for _, r := range all {
				if strings.HasSuffix(req.Params.URI, r.Name) || req.Params.URI == r.Uri {
					return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: r.Content, MIMEType: r.MimeType}}}, nil
				}
			}
			return nil, errors.New("resource not found")
		})
}

func (s *Service) resolveScope(ctx context.Context, scope string) (names []string, groupName string, err error) {
	if scope == "" || scope == "$smart" {
		// $smart：全部 server 可达不变；差别只在工具面——聚合端点全量罗列，
		// $smart 仅暴露 meta-tool（buildServer 分叉）。ensureConnected 保留：
		// 顺带让 registry.discover 把各 server 工具同步进向量索引（tool_search 数据源）。
		list, e := s.serverSvc.All(ctx)
		if e != nil {
			return nil, "", e
		}
		for _, sv := range list {
			names = append(names, sv.Name)
		}
		return names, "", nil
	}
	if g, e := s.groupSvc.GetByName(ctx, scope); e == nil && g != nil {
		cfgs, _ := s.groupSvc.GetServerConfigs(g)
		for _, sc := range cfgs {
			names = append(names, sc.Name)
		}
		return names, g.Name, nil
	}
	if sv, e := s.serverSvc.GetByName(ctx, scope); e == nil && sv != nil {
		return []string{sv.Name}, "", nil
	}
	return nil, "", errors.New("scope 未找到: " + scope)
}

// RecordActivity 记审计。WithoutCancel：客户端断开不连坐取消 DB 写。
func (s *Service) RecordActivity(ctx context.Context, e *entity.McpBearerKey, sourceIp, serverName, tool, status, errMsg string, durationMs int, input, output string) {
	if s.activity == nil || e == nil {
		return
	}
	s.activity.Record(context.WithoutCancel(ctx), serverName, tool, status, "", e.Owner, e.KeyId, e.Name, sourceIp, errMsg, durationMs, input, output)
}

// 令牌交换的载荷/签名/验签单源在 infrastructure/crypto/actor.go（网关签发、ops 验签，
// 字段序即签名基准，两侧不再各写一份 HMAC）。

// truncStr 截断审计载荷，防 jsonb 膨胀。
func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
