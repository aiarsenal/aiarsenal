package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"aiarsenal/domain/entity"
	"github.com/redis/go-redis/v9"

	"aiarsenal/config"
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
	if err := s.repo.Edit(ctx, e); err != nil {
		return err
	}
	s.bustKeyCache(ctx, e.TokenHash)
	return nil
}

func (s *LLMKeyService) Delete(ctx context.Context, hashes []string) error {
	if err := s.repo.Delete(ctx, hashes); err != nil {
		return err
	}
	s.bustKeyCache(ctx, hashes...)
	return nil
}

func (s *LLMKeyService) Block(ctx context.Context, tokenHash string, blocked bool, by string) error {
	v := uint(0)
	if blocked {
		v = 1
	}
	// 用 UpdateBlocked（map 更新）而非 Edit（结构体 Updates）：解封 v=0 时 gorm 会当零值跳过、不生效。
	if err := s.repo.UpdateBlocked(ctx, tokenHash, v, by); err != nil {
		return err
	}
	s.bustKeyCache(ctx, tokenHash)
	return nil
}

// vkeyBustChannel 缓存失效通道，与 llm 网关 internal/auth/middleware.go 两处须同步。
const vkeyBustChannel = "vkey-bust"

// llmRedisDB llm 网关 redis 逻辑库：api 与 llm 共用实例但默认分库（api db1 / llm db2），
// DEL 跨库不可见，失效走 llm 库上的 pub/sub。库号对齐走两个不同名的变量——本侧
// AIARSENAL_LLM_REDIS_DB（默认 2），llm 侧 REDIS_DB（默认 2，llm/internal/config）：
// 改其一必须同步改另一个，否则通道静默断裂（兜底 5s TTL）。
func llmRedisDB() int {
	if v := os.Getenv("AIARSENAL_LLM_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 2
}

// bustKeyCache 立即失效 llm 网关的 5s 虚拟密钥缓存（F-027）：封禁/删除/编辑后网关侧
// 拒绝生效延迟从最长 5s 降到 0。封禁是低频管理动作，按调用新建短命连接可接受；
// 网关不在线时消息即弃，兜底是网关侧 5s 缓存 TTL 自然过期。
func (s *LLMKeyService) bustKeyCache(ctx context.Context, hashes ...string) {
	if len(hashes) == 0 {
		return
	}
	cl := redis.NewClient(&redis.Options{
		Addr:     config.Get().Redis.Default.Address,
		Password: config.Get().Redis.Default.Password,
		DB:       llmRedisDB(),
	})
	defer cl.Close()
	for _, h := range hashes {
		_ = cl.Publish(ctx, vkeyBustChannel, h).Err()
	}
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
