package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/mcpclient"
)

// MCP 上游探测服务：连接测试（发现 tools/prompts/resources）与工具试调用。
// 试调用从管理台直连上游（不经 mcp/ 网关），落审计 keyName=console 以区分网关侧调用。

type McpProbeService struct {
	serverRepo   repository.IMcpServerRepository
	activityRepo repository.IMcpActivityRepository
}

func NewMcpProbeService(r repository.IMcpServerRepository, a repository.IMcpActivityRepository) *McpProbeService {
	return &McpProbeService{serverRepo: r, activityRepo: a}
}

// Test 连接测试：serverId 优先取已保存配置，否则用表单字段（新增弹窗“先测后存”）。
func (s *McpProbeService) Test(ctx context.Context, serverId string, form *entity.McpServer) (*mcpclient.ProbeResult, int, error) {
	if serverId != "" {
		saved, err := s.serverRepo.Get(ctx, serverId)
		if err != nil {
			return nil, 0, err
		}
		form = saved
	}
	if form == nil {
		return nil, 0, errInvalidServer
	}
	if form.Type == "openapi" {
		return nil, 0, errOpenapiUnsupported
	}
	if err := validateMcpProbeServer(form); err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	start := time.Now()
	res, err := mcpclient.Probe(ctx, form)
	return res, int(time.Since(start).Milliseconds()), err
}

// CallTool 试调用（仅对已保存 server；成功/失败都落审计）。
func (s *McpProbeService) CallTool(ctx context.Context, serverId, tool string, args map[string]any) (text string, durationMs int, err error) {
	saved, err := s.serverRepo.Get(ctx, serverId)
	if err != nil {
		return "", 0, err
	}
	if saved.Type == "openapi" {
		return "", 0, errOpenapiUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	in, _ := json.Marshal(args)
	start := time.Now()
	res, err := mcpclient.CallTool(ctx, saved, tool, args)
	durationMs = int(time.Since(start).Milliseconds())

	// 抽取 TextContent 拼接（含 tool error：MCP 把工具级错误放 Content+IsError）
	if res != nil {
		var b strings.Builder
		for _, c := range res.Content {
			if t, ok := c.(*mcp.TextContent); ok && t != nil {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(t.Text)
			}
		}
		text = b.String()
	}

	status, errMsg := "success", ""
	if err != nil {
		status, errMsg = "error", err.Error()
	} else if res != nil && res.IsError {
		status, errMsg = "error", text
	}
	now := types.Time(time.Now())
	_ = s.activityRepo.Add(ctx, &entity.McpActivity{
		ServerName: saved.Name, Tool: tool, DurationMs: durationMs, Status: status,
		Input: truncActivity(string(in)), Output: truncActivity(text), KeyName: "console",
		ErrorMessage: truncActivity(errMsg), Ts: &now,
	})
	return text, durationMs, err
}

func truncActivity(s string) string {
	const max = 2000
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

var (
	errInvalidServer      = &probeErr{"name/type 不能为空"}
	errOpenapiUnsupported = &probeErr{"openapi 类型暂不支持探测"}
)

type probeErr struct{ msg string }

func (e *probeErr) Error() string { return e.msg }

// validateMcpProbeServer 探测前的最小校验（与 McpServerService.validateServer 各写一份，仓内零耦合约定）。
func validateMcpProbeServer(e *entity.McpServer) error {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" || e.Type == "" {
		return errInvalidServer
	}
	switch e.Type {
	case "stdio":
		if e.Command == "" {
			return &probeErr{"stdio 类型必须指定 command"}
		}
	case "sse", "streamable-http":
		if e.Url == "" {
			return &probeErr{e.Type + " 类型必须指定 url"}
		}
	}
	return nil
}
