package service

import (
	"context"
	"encoding/json"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/crypto"
)

// MCP Bearer 端点密钥服务。
// 签发：crypto.RandomKey("mcp") 生成明文 → sha256 哈希落库 → 明文仅返回一次。
// 鉴权时中间件按 token_hash 查表 + 校验 access_type 范围（all/groups/servers/custom）。
// BearerKey 语义，token 列改存哈希（与 aa_llm_virtual_key 一致，不存明文）。

type McpKeyService struct {
	repo repository.IMcpBearerKeyRepository
}

func NewMcpKeyService(r repository.IMcpBearerKeyRepository) *McpKeyService {
	return &McpKeyService{repo: r}
}

func (s *McpKeyService) List(ctx context.Context, name string, pn, ps int) ([]*entity.McpBearerKey, int64, error) {
	return s.repo.List(ctx, name, pn, ps)
}

func (s *McpKeyService) Get(ctx context.Context, id string) (*entity.McpBearerKey, error) {
	return s.repo.Get(ctx, id)
}

// GetByHash 供 BearerAuth 中间件按哈希查活跃 key。
func (s *McpKeyService) GetByHash(ctx context.Context, tokenHash string) (*entity.McpBearerKey, error) {
	return s.repo.GetByHash(ctx, tokenHash)
}

// Create 生成新 key，返回 (keyId, 明文 key)。
func (s *McpKeyService) Create(ctx context.Context, e *entity.McpBearerKey) (string, string, error) {
	if e.Name == "" {
		return "", "", errors.New("name 不能为空")
	}
	if e.AccessType == "" {
		e.AccessType = "all"
	}
	if e.Kind == "" {
		e.Kind = "user"
	}
	if e.AllowedGroups == "" {
		e.AllowedGroups = "[]"
	}
	if e.AllowedServers == "" {
		e.AllowedServers = "[]"
	}
	plain, err := crypto.RandomKey("mcp")
	if err != nil {
		return "", "", err
	}
	e.TokenHash = crypto.HashToken(plain)
	if e.Enabled == 0 {
		e.Enabled = 1
	}
	id, err := s.repo.Add(ctx, e)
	if err != nil {
		return "", "", err
	}
	return id, plain, nil
}

func (s *McpKeyService) Edit(ctx context.Context, e *entity.McpBearerKey) error {
	if e.KeyId == "" {
		return errors.New("keyId 不能为空")
	}
	return s.repo.Edit(ctx, e)
}

func (s *McpKeyService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}

func (s *McpKeyService) Block(ctx context.Context, id string, blocked bool, by string) error {
	e := &entity.McpBearerKey{KeyId: id, Owner: by}
	if blocked {
		e.Enabled = 0
	} else {
		e.Enabled = 1
	}
	return s.repo.Edit(ctx, e)
}

// Rotate 轮换：生成新明文，替换哈希，保留其余字段。
func (s *McpKeyService) Rotate(ctx context.Context, id, by string) (string, error) {
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	plain, err := crypto.RandomKey("mcp")
	if err != nil {
		return "", err
	}
	cur.TokenHash = crypto.HashToken(plain)
	cur.Owner = by
	if err := s.repo.Delete(ctx, []string{id}); err != nil {
		return "", err
	}
	if _, err := s.repo.Add(ctx, cur); err != nil {
		return "", err
	}
	return plain, nil
}

// Authorize 校验 key 是否可访问某 scope（server name 或 group name）。
// scope 为空表示访问 /mcp 全部，只有 all/custom(空) 放行。
func (s *McpKeyService) Authorize(k *entity.McpBearerKey, scope, scopeKind string) bool {
	if k == nil || k.Enabled == 0 {
		return false
	}
	switch k.AccessType {
	case "all":
		return true
	case "groups":
		if scopeKind != "group" {
			return false
		}
		return containsJSON(k.AllowedGroups, scope)
	case "servers":
		if scopeKind != "server" {
			return false
		}
		return containsJSON(k.AllowedServers, scope)
	case "custom":
		// custom：groups ∪ servers 命中即放行
		if scopeKind == "group" {
			return containsJSON(k.AllowedGroups, scope)
		}
		if scopeKind == "server" {
			return containsJSON(k.AllowedServers, scope)
		}
	}
	return false
}

// containsJSON 判断 jsonb 字符串数组列是否包含 v。
func containsJSON(jsonArr, v string) bool {
	if jsonArr == "" {
		return false
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonArr), &arr); err != nil {
		return false
	}
	for _, s := range arr {
		if s == v {
			return true
		}
	}
	return false
}
