package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// agent_user.go — Agent 平台用户仓储接口（v2 占位菜单：Agent 用户管理）。
// 与主库 aa_user 解耦（username 软引用），实例授权存 instance_ids jsonb。

type IAgentUserRepository interface {
	// List keyword 模糊 username/display_name；status -1=全部 0/1=按 is_enabled 过滤。
	List(ctx context.Context, keyword string, status int, pageNum, pageSize int) (list []*entity.AgentUser, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentUser, error)
	GetByUsername(ctx context.Context, username string) (*entity.AgentUser, error)
	Add(ctx context.Context, e *entity.AgentUser) (string, error)
	Edit(ctx context.Context, e *entity.AgentUser) error
	// EditFields 显式字段更新（gorm 结构体 Updates 跳零值，is_enabled=0 要能生效）。
	EditFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, ids []string) error
}
