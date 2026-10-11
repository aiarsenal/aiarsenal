package service

import (
	"context"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/crypto"
)

// AgentModelService AI Gateway 治理配置：model（api_key 加密）/ risk-rule。
// 凭据 api_key 走 AES-256-GCM 加密落库（密钥取 jwt.encryptKey，与 llm 模块一致）。
type AgentModelService struct {
	modelRepo repository.IAgentModelRepository
	ruleRepo  repository.IAgentRiskRuleRepository
}

func NewAgentModelService(m repository.IAgentModelRepository, r repository.IAgentRiskRuleRepository) *AgentModelService {
	return &AgentModelService{modelRepo: m, ruleRepo: r}
}

// GetModelAPIKey 解密返回 provider 明文 key（仅供内部/测试连通调用，不暴露给前端 list）。
func (s *AgentModelService) GetModelAPIKey(ctx context.Context, id string) (string, *entity.AgentModel, error) {
	m, err := s.modelRepo.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	pt, _ := crypto.Decrypt(m.ApiKey, config.Get().Jwt.EncryptKey)
	return pt, m, nil
}

// AddModel apiKey 明文 → 加密落库。
func (s *AgentModelService) AddModel(ctx context.Context, e *entity.AgentModel, apiKey string) (string, error) {
	if apiKey != "" {
		enc, err := crypto.Encrypt(apiKey, config.Get().Jwt.EncryptKey)
		if err != nil {
			return "", err
		}
		e.ApiKey = enc
	}
	if e.IsActive == 0 {
		e.IsActive = 1
	}
	if e.Currency == "" {
		e.Currency = "USD"
	}
	return s.modelRepo.Add(ctx, e)
}

// EditModel 若提供新 apiKey 则加密更新；否则保留原密文（不传 apiKey 字段更新）。
func (s *AgentModelService) EditModel(ctx context.Context, e *entity.AgentModel, apiKey string) error {
	if apiKey != "" {
		enc, err := crypto.Encrypt(apiKey, config.Get().Jwt.EncryptKey)
		if err != nil {
			return err
		}
		e.ApiKey = enc
	}
	return s.modelRepo.Edit(ctx, e)
}

// ---- Model CRUD ----
func (s *AgentModelService) ListModels(ctx context.Context, name, providerType string, pageNum, pageSize int) ([]*entity.AgentModel, int64, error) {
	return s.modelRepo.List(ctx, name, providerType, pageNum, pageSize)
}
func (s *AgentModelService) GetModel(ctx context.Context, id string) (*entity.AgentModel, error) {
	return s.modelRepo.Get(ctx, id)
}
func (s *AgentModelService) DeleteModels(ctx context.Context, ids []string) error {
	return s.modelRepo.Delete(ctx, ids)
}

// ---- RiskRule CRUD ----
func (s *AgentModelService) ListRules(ctx context.Context, name string, pageNum, pageSize int) ([]*entity.AgentRiskRule, int64, error) {
	return s.ruleRepo.List(ctx, name, pageNum, pageSize)
}
func (s *AgentModelService) GetRule(ctx context.Context, id string) (*entity.AgentRiskRule, error) {
	return s.ruleRepo.Get(ctx, id)
}
func (s *AgentModelService) AddRule(ctx context.Context, e *entity.AgentRiskRule) (string, error) {
	if e.IsEnabled == 0 {
		e.IsEnabled = 1
	}
	return s.ruleRepo.Add(ctx, e)
}
func (s *AgentModelService) EditRule(ctx context.Context, e *entity.AgentRiskRule) error {
	return s.ruleRepo.Edit(ctx, e)
}
func (s *AgentModelService) DeleteRules(ctx context.Context, ids []string) error {
	return s.ruleRepo.Delete(ctx, ids)
}
