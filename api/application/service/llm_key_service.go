package service

import (
	"context"
	"errors"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/crypto"
)

// LLM 虚拟密钥服务。
// 签发：RandomKey 生成明文 → sha256 哈希落库 → 明文仅返回一次。
// VerificationToken 语义：以 token_hash 为查询主键，spend/budget 走预算表/列。

type LLMKeyService struct {
	repo repository.ILLMVirtualKeyRepository
}

func NewLLMKeyService(r repository.ILLMVirtualKeyRepository) *LLMKeyService {
	return &LLMKeyService{repo: r}
}

func (s *LLMKeyService) List(ctx context.Context, alias, userId, teamId string, pn, ps int) ([]*entity.LLMVirtualKey, int64, error) {
	return s.repo.List(ctx, alias, userId, teamId, pn, ps)
}

func (s *LLMKeyService) Get(ctx context.Context, tokenHash string) (*entity.LLMVirtualKey, error) {
	return s.repo.Get(ctx, tokenHash)
}

// GetByAlias 按 key_alias 查询（同 alias 唯一）。desktop 模块用：替换旧 key 前先查。
func (s *LLMKeyService) GetByAlias(ctx context.Context, alias string) (*entity.LLMVirtualKey, error) {
	return s.repo.GetByAlias(ctx, alias)
}

// ListAffectedKeys 列出 allowed_models 会让该模型可见的 key（模型禁用/删除后定位需刷新配置的实例用）。
func (s *LLMKeyService) ListAffectedKeys(ctx context.Context, group, modelName string) ([]*entity.LLMVirtualKey, error) {
	return s.repo.ListAffectedByModel(ctx, group, modelName)
}

// Create 生成新 key，返回 (tokenHash, 明文 key)。
func (s *LLMKeyService) Create(ctx context.Context, e *entity.LLMVirtualKey) (string, string, error) {
	plain, err := crypto.RandomKey("sk-aa")
	if err != nil {
		return "", "", err
	}
	e.TokenHash = crypto.HashToken(plain)
	if _, err := s.repo.Add(ctx, e); err != nil {
		return "", "", err
	}
	return e.TokenHash, plain, nil
}

func (s *LLMKeyService) Edit(ctx context.Context, e *entity.LLMVirtualKey) error {
	if e.TokenHash == "" {
		return errors.New("tokenHash 不能为空")
	}
	return s.repo.Edit(ctx, e)
}

func (s *LLMKeyService) Delete(ctx context.Context, hashes []string) error {
	return s.repo.Delete(ctx, hashes)
}

func (s *LLMKeyService) Block(ctx context.Context, tokenHash string, blocked bool, by string) error {
	v := uint(0)
	if blocked {
		v = 1
	}
	// 用 UpdateBlocked（map 更新）而非 Edit（结构体 Updates）：解封 v=0 时 gorm 会当零值跳过、不生效。
	return s.repo.UpdateBlocked(ctx, tokenHash, v, by)
}

// Rotate 轮换：生成新明文，token_hash 原地替换为新哈希；旧 hash 保留 5 分钟宽限期
// （存入 deprecated_token_hash + deprecate_expires_at），宽限内新旧明文均可鉴权，
// 到期后旧明文自动失效。其余配置（别名/预算/限速/允许模型/归属）全部保留。
func (s *LLMKeyService) Rotate(ctx context.Context, tokenHash, by string) (string, error) {
	if _, err := s.repo.Get(ctx, tokenHash); err != nil {
		return "", err
	}
	plain, err := crypto.RandomKey("sk-aa")
	if err != nil {
		return "", err
	}
	newHash := crypto.HashToken(plain)
	graceExpires := time.Now().Add(5 * time.Minute)
	if err := s.repo.RotateHash(ctx, tokenHash, newHash, graceExpires, by); err != nil {
		return "", err
	}
	return plain, nil
}

// IncrementSpend / TouchActive 供 router 计费回调调用。
func (s *LLMKeyService) IncrementSpend(ctx context.Context, tokenHash string, delta types.Milli) error {
	return s.repo.UpdateSpend(ctx, tokenHash, delta)
}

func (s *LLMKeyService) TouchActive(ctx context.Context, tokenHash string) error {
	return s.repo.UpdateLastActive(ctx, tokenHash)
}
