package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/mcpgateway/compression"
	"aiarsenal/infrastructure/persistence"
)

// 上游 MCP 客户端注册中心：管理到各 MCP server 的连接（stdio/sse/streamable-http）。
// MCP server 连接生命周期管理（建立/复用/健康检查/回收）。
// 发现并缓存每个 server 的 tools/prompts/resources；按需 call_tool。
// stdio server 用 Setpgid 启动，停止时 Kill(-pgid) 杀整组（见 process.go）。
// 另：每次发现到的工具同步进 aa_mcp_tool_embedding 向量索引（$smart tool_search 数据源，
// 256 维 LocalEmbedding 与 aa_skill 同款），工具下线随发现对账清理。

type Entry struct {
	Name      string
	Type      string
	session   *mcp.ClientSession
	transport mcp.Transport
	cmd       *cmdWrap
	cancel    context.CancelFunc
	mu        sync.Mutex
	tools     []*mcp.Tool
	prompts   []*mcp.Prompt
	resources []*mcp.Resource
	ready     bool
}

type Service struct {
	compress  *compression.Service
	embedRepo repository.IMcpToolEmbeddingRepository // 工具向量索引（懒建，见 toolEmbedRepo）
	mu        sync.RWMutex
	entries   map[string]*Entry
}

func New(compress *compression.Service) *Service {
	return &Service{compress: compress, entries: make(map[string]*Entry)}
}

// SetToolEmbedRepo 显式注入工具向量仓储（不注入则首次发现时懒建默认实现）。
func (s *Service) SetToolEmbedRepo(r repository.IMcpToolEmbeddingRepository) {
	s.mu.Lock()
	s.embedRepo = r
	s.mu.Unlock()
}

// toolEmbedRepo 取工具向量仓储（懒建：cmd/mcp 未显式注入时兜底直连 mcp 库；
// 配置未加载时 panic 由进程启动顺序保证不会发生——main 先 Load 再建 registry）。
func (s *Service) toolEmbedRepo() repository.IMcpToolEmbeddingRepository {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.embedRepo == nil {
		s.embedRepo = persistence.NewMcpToolEmbeddingRepository()
	}
	return s.embedRepo
}

// Start 按 server 配置建立连接（Connect 内部完成 initialize 握手）。
func (s *Service) Start(ctx context.Context, srv *entity.McpServer) error {
	name := srv.Name
	s.mu.Lock()
	if _, ok := s.entries[name]; ok {
		s.mu.Unlock()
		_ = s.Stop(ctx, name)
		s.mu.Lock()
	}
	s.mu.Unlock()

	cctx, cancel := context.WithCancel(context.Background())
	entry := &Entry{Name: name, Type: srv.Type, cancel: cancel}

	args := parseStrArray(srv.Args)
	env := parseStrMap(srv.Env)
	headers := parseStrMap(srv.Headers)

	var transport mcp.Transport
	var cw *cmdWrap
	var err error
	switch srv.Type {
	case "stdio":
		transport, cw, err = newStdioTransport(srv.Command, args, env)
	case "sse":
		transport, err = newSSEClientTransport(srv.Url, headers)
	case "streamable-http":
		transport, err = newStreamableHTTPClientTransport(srv.Url, headers)
	default:
		return fmt.Errorf("unsupported type: %s", srv.Type)
	}
	if err != nil {
		cancel()
		return err
	}
	entry.transport = transport
	entry.cmd = cw

	client := mcp.NewClient(&mcp.Implementation{Name: "aiarsenal-mcp-" + name, Version: "1.0"}, nil)

	initCtx, initCancel := context.WithTimeout(cctx, 30*time.Second)
	defer initCancel()
	session, err := client.Connect(initCtx, transport, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("connect %s: %w", name, err)
	}
	entry.session = session
	s.discover(cctx, entry)

	s.mu.Lock()
	s.entries[name] = entry
	s.mu.Unlock()
	return nil
}

func (s *Service) Stop(ctx context.Context, name string) error {
	s.mu.Lock()
	e, ok := s.entries[name]
	delete(s.entries, name)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if e.cancel != nil {
		e.cancel()
	}
	if e.session != nil {
		_ = e.session.Close()
	}
	if e.cmd != nil {
		_ = e.cmd.kill()
	}
	return nil
}

func (s *Service) Get(name string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[name]
	return e, ok
}

func (e *Entry) Tools() []*mcp.Tool {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*mcp.Tool, len(e.tools))
	copy(out, e.tools)
	return out
}
func (e *Entry) Prompts() []*mcp.Prompt {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*mcp.Prompt, len(e.prompts))
	copy(out, e.prompts)
	return out
}
func (e *Entry) Resources() []*mcp.Resource {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*mcp.Resource, len(e.resources))
	copy(out, e.resources)
	return out
}
func (e *Entry) Session() *mcp.ClientSession { return e.session }

func (s *Service) discover(ctx context.Context, e *Entry) {
	if ts, err := e.session.ListTools(ctx, nil); err == nil && ts != nil {
		e.mu.Lock()
		e.tools = ts.Tools
		e.mu.Unlock()
		s.syncToolEmbeddings(ctx, e.Name, ts.Tools)
	}
	if ps, err := e.session.ListPrompts(ctx, nil); err == nil && ps != nil {
		e.mu.Lock()
		e.prompts = ps.Prompts
		e.mu.Unlock()
	}
	if rs, err := e.session.ListResources(ctx, nil); err == nil && rs != nil {
		e.mu.Lock()
		e.resources = rs.Resources
		e.mu.Unlock()
	}
	e.mu.Lock()
	e.ready = true
	e.mu.Unlock()
}

// syncToolEmbeddings 把本次发现到的工具全量对账进向量索引（$smart tool_search 数据源）：
// 发现到的逐条 upsert（描述/向量随最新一次发现刷新），本次未见的（上游已下线）清理。
// 同步失败不阻断连接——索引尽力而为，下次发现会再对账；对账名单按「本次发现的工具」
// 计（upsert 单条失败不把该工具误判为下线而误删存量向量）。
// 向量用 256 维 LocalEmbedding 现算（server+tool+description 文本，与 aa_skill 同款算法）。
func (s *Service) syncToolEmbeddings(ctx context.Context, serverName string, tools []*mcp.Tool) {
	repo := s.toolEmbedRepo()
	emb := service.NewLocalEmbedding(service.ToolEmbeddingDim)
	keep := make([]string, 0, len(tools))
	for _, t := range tools {
		if t == nil || t.Name == "" {
			continue
		}
		keep = append(keep, t.Name)
		vec, err := emb.Embed(serverName + " " + t.Name + " " + t.Description)
		if err != nil {
			continue
		}
		_ = repo.Upsert(context.WithoutCancel(ctx), serverName, t.Name, t.Description, service.VectorToPGText(vec))
	}
	_ = repo.DeleteStale(context.WithoutCancel(ctx), serverName, keep)
}

// CallTool 调用某 server 的工具，结果经压缩服务处理。args 透传（通常为 json.RawMessage）。
// CallTool 转发调用；第二个返回值为工具级错误（result.IsError）——
// 协议上它仍是正常 JSON-RPC 响应（内容即错误文本），但语义是失败，须与成功区分。
func (s *Service) CallTool(ctx context.Context, name, tool string, args any) (txt string, isToolErr bool, err error) {
	e, ok := s.Get(name)
	if !ok {
		return "", false, errors.New("server 未连接: " + name)
	}
	res, err := e.session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "", false, err
	}
	txt = toolResultText(res.Content)
	if s.compress != nil {
		txt = s.compress.Compress(txt)
	}
	return txt, res.IsError, nil
}

func toolResultText(content []mcp.Content) string {
	var b strings.Builder
	for _, c := range content {
		if t, ok := c.(*mcp.TextContent); ok && t != nil {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}
