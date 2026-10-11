package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// MCP 模块仓储接口（mcp 库）。软删表统一 Where deleted_at is null。

type IMcpServerRepository interface {
	List(ctx context.Context, name, typ string, pageNum, pageSize int) (list []*entity.McpServer, total int64, err error)
	All(ctx context.Context) (list []*entity.McpServer, err error)
	Get(ctx context.Context, id string) (*entity.McpServer, error)
	GetByName(ctx context.Context, name string) (*entity.McpServer, error)
	Add(ctx context.Context, e *entity.McpServer) (string, error)
	Edit(ctx context.Context, e *entity.McpServer) error
	Delete(ctx context.Context, ids []string) error
}

type IMcpGroupRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpGroup, total int64, err error)
	All(ctx context.Context) (list []*entity.McpGroup, err error)
	// ListVisible/AllFor 带身份过滤（t nil 等同 List/All；超管短路在 service 层）。
	ListVisible(ctx context.Context, name string, pageNum, pageSize int, t *model.Identity) (list []*entity.McpGroup, total int64, err error)
	AllFor(ctx context.Context, t *model.Identity) (list []*entity.McpGroup, err error)
	Get(ctx context.Context, id string) (*entity.McpGroup, error)
	GetByName(ctx context.Context, name string) (*entity.McpGroup, error)
	Add(ctx context.Context, e *entity.McpGroup) (string, error)
	Edit(ctx context.Context, e *entity.McpGroup) error
	SetVisibility(ctx context.Context, groupId string, visibility uint) error
	Delete(ctx context.Context, ids []string) error
}

type IMcpGroupGrantRepository interface {
	ListAll(ctx context.Context) (list []*entity.McpGroupGrant, err error)
	ListByGroup(ctx context.Context, groupId string) (list []*entity.McpGroupGrant, err error)
	// Replace 重写分组条件行集：单事务先删后插。
	Replace(ctx context.Context, groupId string, rows []model.GrantRow, actor string) error
	DeleteByGroups(ctx context.Context, groupIds []string) error
}

type IMcpBearerKeyRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBearerKey, total int64, err error)
	Get(ctx context.Context, id string) (*entity.McpBearerKey, error)
	GetByHash(ctx context.Context, tokenHash string) (*entity.McpBearerKey, error)
	Add(ctx context.Context, e *entity.McpBearerKey) (string, error)
	Edit(ctx context.Context, e *entity.McpBearerKey) error
	Delete(ctx context.Context, ids []string) error
}

type IMcpActivityRepository interface {
	List(ctx context.Context, serverName, tool, status, userId string, dateRange []string, pageNum, pageSize int) (list []*entity.McpActivity, total int64, err error)
	Add(ctx context.Context, e *entity.McpActivity) error
}

type IMcpBuiltinPromptRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBuiltinPrompt, total int64, err error)
	All(ctx context.Context) (list []*entity.McpBuiltinPrompt, err error)
	Get(ctx context.Context, id string) (*entity.McpBuiltinPrompt, error)
	Add(ctx context.Context, e *entity.McpBuiltinPrompt) (string, error)
	Edit(ctx context.Context, e *entity.McpBuiltinPrompt) error
	Delete(ctx context.Context, ids []string) error
}

type IMcpBuiltinResourceRepository interface {
	List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBuiltinResource, total int64, err error)
	All(ctx context.Context) (list []*entity.McpBuiltinResource, err error)
	Get(ctx context.Context, id string) (*entity.McpBuiltinResource, error)
	Add(ctx context.Context, e *entity.McpBuiltinResource) (string, error)
	Edit(ctx context.Context, e *entity.McpBuiltinResource) error
	Delete(ctx context.Context, ids []string) error
}

type IMcpConfigRepository interface {
	Get(ctx context.Context) (*entity.McpConfig, error)
	Save(ctx context.Context, e *entity.McpConfig) error
}

type IMcpUserConfigRepository interface {
	Get(ctx context.Context, userId string) (*entity.McpUserConfig, error)
	Save(ctx context.Context, e *entity.McpUserConfig) error
}
