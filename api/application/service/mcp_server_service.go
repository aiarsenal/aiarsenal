package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/persistence"
)

// MCP 服务器配置服务。
// server.type ∈ stdio/sse/streamable-http/openapi；name 为唯一标识，对应 MCP 端点 /mcp/{name}。
// 上游连接生命周期管理见 mcp_registry_service；本服务只管配置 CRUD 与可见性/参数校验。

type McpServerService struct {
	repo          repository.IMcpServerRepository
	channelNotice *ChannelNoticeService // server 登记/删除双通道通知（main.go 回指）
}

func NewMcpServerService(r repository.IMcpServerRepository) *McpServerService {
	return &McpServerService{repo: r}
}

// SetChannelNotice main.go 装配后回指（构造早于通知出口）：server 增删通知用。
func (s *McpServerService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

func (s *McpServerService) List(ctx context.Context, name, typ string, pn, ps int) ([]*entity.McpServer, int64, error) {
	return s.repo.List(ctx, name, typ, pn, ps)
}

func (s *McpServerService) All(ctx context.Context) ([]*entity.McpServer, error) {
	return s.repo.All(ctx)
}

func (s *McpServerService) Get(ctx context.Context, id string) (*entity.McpServer, error) {
	return s.repo.Get(ctx, id)
}

func (s *McpServerService) GetByName(ctx context.Context, name string) (*entity.McpServer, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *McpServerService) Add(ctx context.Context, e *entity.McpServer) (string, error) {
	if err := validateServer(e); err != nil {
		return "", err
	}
	if e.Enabled == 0 {
		e.Enabled = 1
	}
	if e.Visibility == "" {
		e.Visibility = "private"
	}
	if e.Tools == "" {
		e.Tools = "{}"
	}
	if e.Prompts == "" {
		e.Prompts = "{}"
	}
	if e.Resources == "" {
		e.Resources = "{}"
	}
	id, err := s.repo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	// 登记成功即双通道通知操作人（站内铃铛 + 名下 hermes 数字人转微信）。
	if s.channelNotice != nil {
		s.channelNotice.Push(ctx, e.CreatedBy, "MCP server 已新增",
			fmt.Sprintf("MCP server「%s」（%s）已登记，可加入分组使用。", e.Name, e.Type),
			"/mcp/server/list", "info", "mcp-server")
	}
	return id, nil
}

func (s *McpServerService) Edit(ctx context.Context, e *entity.McpServer) error {
	if e.ServerId == "" {
		return errors.New("serverId 不能为空")
	}
	if err := validateServer(e); err != nil {
		return err
	}
	return s.repo.Edit(ctx, e)
}

// Delete 删除 server（批量）。删前取名供通知文案；删除成功后双通道通知操作人——
// 引用它的分组会即时失去对应工具（网关按组名现算 server），管理员需要感知。
// 另：工具向量索引随删（$smart tool_search 数据源；检索侧另有 enabled/未删过滤兜底）。
func (s *McpServerService) Delete(ctx context.Context, ids []string, by string) error {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if sv, err := s.repo.Get(ctx, id); err == nil && sv != nil {
			names = append(names, sv.Name)
		}
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	if len(names) > 0 {
		_ = persistence.NewMcpToolEmbeddingRepository().DeleteByServers(
			context.WithoutCancel(ctx), names)
	}
	if s.channelNotice != nil && len(names) > 0 {
		s.channelNotice.Push(ctx, by, "MCP server 已删除",
			fmt.Sprintf("MCP server 已删除：%s。引用它的分组将失去对应工具。", strings.Join(names, "、")),
			"/mcp/server/list", "info", "mcp-server")
	}
	return nil
}

// validateServer 按 type 校验必要字段；禁止 name 含路径分隔符（避免端点路由冲突）。
func validateServer(e *entity.McpServer) error {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New("name 不能为空")
	}
	if strings.ContainsAny(e.Name, "/ \t") || strings.HasPrefix(e.Name, "$") {
		return errors.New("name 不能含空格/斜杠且不能以 $ 开头")
	}
	switch e.Type {
	case "stdio":
		if e.Command == "" {
			return errors.New("stdio 类型必须指定 command")
		}
	case "sse", "streamable-http", "openapi":
		if e.Url == "" {
			return errors.New(e.Type + " 类型必须指定 url")
		}
	default:
		return errors.New("不支持的 type：" + e.Type)
	}
	if e.Visibility == "" {
		e.Visibility = "private"
	}
	return nil
}
