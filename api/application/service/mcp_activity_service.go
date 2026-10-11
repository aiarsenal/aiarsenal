package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"time"
)

// MCP 调用审计服务。
// 每次工具调用（成功/失败）落一条 aa_mcp_activity；status=success|error。

type McpActivityService struct {
	repo repository.IMcpActivityRepository
}

func NewMcpActivityService(r repository.IMcpActivityRepository) *McpActivityService {
	return &McpActivityService{repo: r}
}

func (s *McpActivityService) List(ctx context.Context, serverName, tool, status, userId string, dateRange []string, pn, ps int) ([]*entity.McpActivity, int64, error) {
	return s.repo.List(ctx, serverName, tool, status, userId, dateRange, pn, ps)
}

// Record 异步写一条审计（调用方持 ctx）。input/output 由压缩服务截断后传入。
func (s *McpActivityService) Record(ctx context.Context, serverName, tool, status, groupName, userId, keyId, keyName, sourceIp, errMsg string, durationMs int, input, output string) {
	now := types.Time(time.Now())
	_ = s.repo.Add(ctx, &entity.McpActivity{
		ServerName:   serverName,
		Tool:         tool,
		DurationMs:   durationMs,
		Status:       status,
		Input:        input,
		Output:       output,
		GroupName:    groupName,
		UserId:       userId,
		KeyId:        keyId,
		KeyName:      keyName,
		SourceIp:     sourceIp,
		ErrorMessage: errMsg,
		Ts:           &now,
	})
}
