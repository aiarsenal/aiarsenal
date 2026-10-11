package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// MCP OAuth 授权仓储接口（mcp 库）。授权主体复用既有 Bearer Key（bearer_key_id），
// 明文 secret/code/token 一律不落库——各仓储只收发哈希。

type IMcpOAuthClientRepository interface {
	// GetByClientId 按 OAuth 协议 client_id 取启用中的客户端（is_enabled=1）。
	GetByClientId(ctx context.Context, clientId string) (*entity.McpOAuthClient, error)
	Add(ctx context.Context, e *entity.McpOAuthClient) (string, error)
	Edit(ctx context.Context, e *entity.McpOAuthClient) error

	// ===== 管理端 CRUD（主服务 interfaces/controller/mcp_oauth_admin.go）增补 =====

	// GetByClientIdAll 按 client_id 取客户端，不过滤启用状态（管理端编辑/轮换需取到停用行）。
	GetByClientIdAll(ctx context.Context, clientId string) (*entity.McpOAuthClient, error)
	// List 管理端分页列表：keyword 模糊匹配 client_name / client_id，pageNum/pageSize 从 1 起，id 倒序。
	List(ctx context.Context, keyword string, pageNum, pageSize int) (list []*entity.McpOAuthClient, total int64, err error)
	// ListByClientIds 按协议 client_id 集合批量取客户端（令牌列表富化 client_name 用）。
	ListByClientIds(ctx context.Context, clientIds []string) ([]*entity.McpOAuthClient, error)
	// SaveAll 管理端整行保存：显式列清单让零值也落库——GORM 结构体 Updates 缺省跳零值，
	// 停用（is_enabled=0）/ 转 public 清空 secret 哈希（空串）都依赖它。
	SaveAll(ctx context.Context, e *entity.McpOAuthClient) error
	// DeleteByClientIds 按协议 client_id 集合删除客户端（调用方须先级联吊销其令牌）。
	DeleteByClientIds(ctx context.Context, clientIds []string) error
}

type IMcpOAuthCodeRepository interface {
	Add(ctx context.Context, e *entity.McpOAuthCode) error
	// GetByHash 按授权码哈希取行（不做时效/已用判定，语义判定在 service 层）。
	GetByHash(ctx context.Context, codeHash string) (*entity.McpOAuthCode, error)
	// MarkUsed 一次性消费：used_at 置非空，返回受影响行数（0=已被消费，防重放）。
	MarkUsed(ctx context.Context, codeHash string) (int64, error)
}

type IMcpOAuthTokenRepository interface {
	Add(ctx context.Context, e *entity.McpOAuthToken) error
	// GetByAccessHash 按访问令牌哈希取行。
	GetByAccessHash(ctx context.Context, accessTokenHash string) (*entity.McpOAuthToken, error)
	// GetByRefreshHash 按刷新令牌哈希取行。
	GetByRefreshHash(ctx context.Context, refreshTokenHash string) (*entity.McpOAuthToken, error)
	// Revoke 吊销（revoked_at 置非空，整行 access+refresh 一并作废）。
	Revoke(ctx context.Context, id uint) error

	// ===== 管理端 CRUD（主服务 interfaces/controller/mcp_oauth_admin.go）增补 =====

	// GetByBizId 按对外令牌 id（biz_id）取行（管理端吊销入参即它）。
	GetByBizId(ctx context.Context, bizId string) (*entity.McpOAuthToken, error)
	// List 管理端分页列表：clientId 空=全部，id 倒序。
	List(ctx context.Context, clientId string, pageNum, pageSize int) (list []*entity.McpOAuthToken, total int64, err error)
	// CountActiveByClientIds 统计各 client 未吊销令牌数（客户端列表富化 tokenCount 用）。
	CountActiveByClientIds(ctx context.Context, clientIds []string) (map[string]int64, error)
	// RevokeByClientIds 按客户端集合批量吊销其全部未吊销令牌（删除客户端时级联吊销）。
	RevokeByClientIds(ctx context.Context, clientIds []string) error
}
