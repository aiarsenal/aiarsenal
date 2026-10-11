package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// MCP OAuth 授权仓储实现（mcp 库）。统一走 db.McpDB()。
// 仓储只收发哈希：client_secret / code / access_token / refresh_token 的明文
// 在 service 层生成并即抛（仅返回一次），落库前均经 crypto.HashToken。
// 授权码一次性消费用条件 UPDATE（used_at is null）原子置位，防并发重放。

// ===================== OAuth Client =====================

type McpOAuthClientRepository struct{ db *gorm.DB }

func NewMcpOAuthClientRepository() *McpOAuthClientRepository {
	return &McpOAuthClientRepository{db: db.McpDB()}
}

func (r *McpOAuthClientRepository) GetByClientId(ctx context.Context, clientId string) (*entity.McpOAuthClient, error) {
	var e entity.McpOAuthClient
	if err := r.db.WithContext(ctx).Where("client_id = ? and is_enabled = 1", clientId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpOAuthClientRepository) Add(ctx context.Context, e *entity.McpOAuthClient) (string, error) {
	e.BizId = idgen.New()
	if e.ClientId == "" {
		e.ClientId = idgen.New()
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ClientId, nil
}

func (r *McpOAuthClientRepository) Edit(ctx context.Context, e *entity.McpOAuthClient) error {
	return r.db.WithContext(ctx).Model(&entity.McpOAuthClient{}).Where("biz_id = ?", e.BizId).Updates(e).Error
}

// ===== 管理端 CRUD 增补（主服务 interfaces/controller/mcp_oauth_admin.go） =====

// GetByClientIdAll 按 client_id 取客户端，不过滤启用状态（管理端编辑/轮换需取到停用行）。
func (r *McpOAuthClientRepository) GetByClientIdAll(ctx context.Context, clientId string) (*entity.McpOAuthClient, error) {
	var e entity.McpOAuthClient
	if err := r.db.WithContext(ctx).Where("client_id = ?", clientId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// List 管理端分页列表：keyword 模糊匹配 client_name / client_id，id 倒序。
func (r *McpOAuthClientRepository) List(ctx context.Context, keyword string, pageNum, pageSize int) (list []*entity.McpOAuthClient, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpOAuthClient{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("client_name like ? or client_id like ?", kw, kw)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// ListByClientIds 按协议 client_id 集合批量取客户端（令牌列表富化 client_name 用）。
func (r *McpOAuthClientRepository) ListByClientIds(ctx context.Context, clientIds []string) ([]*entity.McpOAuthClient, error) {
	if len(clientIds) == 0 {
		return nil, nil
	}
	var list []*entity.McpOAuthClient
	err := r.db.WithContext(ctx).Where("client_id in ?", clientIds).Find(&list).Error
	return list, err
}

// SaveAll 管理端整行保存：显式列清单，零值也落库——结构体 Updates 缺省跳零值，
// 管理端停用（is_enabled=0）/ 转 public 清空 secret 哈希（空串）必须显式覆盖。
func (r *McpOAuthClientRepository) SaveAll(ctx context.Context, e *entity.McpOAuthClient) error {
	return r.db.WithContext(ctx).Model(&entity.McpOAuthClient{}).
		Where("biz_id = ?", e.BizId).
		Select("client_name", "client_secret_hash", "redirect_uris", "grant_types", "scope", "is_enabled", "updated_at").
		Updates(e).Error
}

// DeleteByClientIds 按协议 client_id 集合删除客户端（调用方须先级联吊销其令牌）。
func (r *McpOAuthClientRepository) DeleteByClientIds(ctx context.Context, clientIds []string) error {
	if len(clientIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpOAuthClient{}).
		Where("client_id in ?", clientIds).Delete(nil).Error
}

// ===================== OAuth Code =====================

type McpOAuthCodeRepository struct{ db *gorm.DB }

func NewMcpOAuthCodeRepository() *McpOAuthCodeRepository {
	return &McpOAuthCodeRepository{db: db.McpDB()}
}

func (r *McpOAuthCodeRepository) Add(ctx context.Context, e *entity.McpOAuthCode) error {
	e.BizId = idgen.New()
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *McpOAuthCodeRepository) GetByHash(ctx context.Context, codeHash string) (*entity.McpOAuthCode, error) {
	var e entity.McpOAuthCode
	if err := r.db.WithContext(ctx).Where("code_hash = ?", codeHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// MarkUsed 一次性消费：条件 UPDATE 原子置位，返回受影响行数（0=已被消费或不存在）。
func (r *McpOAuthCodeRepository) MarkUsed(ctx context.Context, codeHash string) (int64, error) {
	res := r.db.WithContext(ctx).Model(&entity.McpOAuthCode{}).
		Where("code_hash = ? and used_at is null", codeHash).
		Update("used_at", gorm.Expr("now()"))
	return res.RowsAffected, res.Error
}

// ===================== OAuth Token =====================

type McpOAuthTokenRepository struct{ db *gorm.DB }

func NewMcpOAuthTokenRepository() *McpOAuthTokenRepository {
	return &McpOAuthTokenRepository{db: db.McpDB()}
}

func (r *McpOAuthTokenRepository) Add(ctx context.Context, e *entity.McpOAuthToken) error {
	e.BizId = idgen.New()
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *McpOAuthTokenRepository) GetByAccessHash(ctx context.Context, accessTokenHash string) (*entity.McpOAuthToken, error) {
	var e entity.McpOAuthToken
	if err := r.db.WithContext(ctx).Where("access_token_hash = ?", accessTokenHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpOAuthTokenRepository) GetByRefreshHash(ctx context.Context, refreshTokenHash string) (*entity.McpOAuthToken, error) {
	var e entity.McpOAuthToken
	if err := r.db.WithContext(ctx).Where("refresh_token_hash = ?", refreshTokenHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpOAuthTokenRepository) Revoke(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&entity.McpOAuthToken{}).
		Where("id = ? and revoked_at is null", id).
		Update("revoked_at", gorm.Expr("now()")).Error
}

// ===== 管理端 CRUD 增补（主服务 interfaces/controller/mcp_oauth_admin.go） =====

// GetByBizId 按对外令牌 id（biz_id）取行（管理端吊销入参即它）。
func (r *McpOAuthTokenRepository) GetByBizId(ctx context.Context, bizId string) (*entity.McpOAuthToken, error) {
	var e entity.McpOAuthToken
	if err := r.db.WithContext(ctx).Where("biz_id = ?", bizId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// List 管理端分页列表：clientId 空=全部，id 倒序（后签发的在前）。
func (r *McpOAuthTokenRepository) List(ctx context.Context, clientId string, pageNum, pageSize int) (list []*entity.McpOAuthToken, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpOAuthToken{})
	if clientId != "" {
		q = q.Where("client_id = ?", clientId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// CountActiveByClientIds 各 client 未吊销令牌数（group by 一次查回，缺省 0 不进 map）。
func (r *McpOAuthTokenRepository) CountActiveByClientIds(ctx context.Context, clientIds []string) (map[string]int64, error) {
	out := make(map[string]int64)
	if len(clientIds) == 0 {
		return out, nil
	}
	var rows []struct {
		ClientId string `gorm:"column:client_id"`
		N        int64  `gorm:"column:n"`
	}
	err := r.db.WithContext(ctx).Model(&entity.McpOAuthToken{}).
		Select("client_id, count(*) as n").
		Where("client_id in ? and revoked_at is null", clientIds).
		Group("client_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ClientId] = row.N
	}
	return out, nil
}

// RevokeByClientIds 按客户端集合批量吊销其全部未吊销令牌（删除客户端时级联吊销）。
func (r *McpOAuthTokenRepository) RevokeByClientIds(ctx context.Context, clientIds []string) error {
	if len(clientIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpOAuthToken{}).
		Where("client_id in ? and revoked_at is null", clientIds).
		Update("revoked_at", gorm.Expr("now()")).Error
}
