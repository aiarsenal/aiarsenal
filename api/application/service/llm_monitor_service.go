package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// LLM 监控服务：用量日志 / 日聚合 / 错误日志 / 审计日志 / 健康检查 / 提示词。
// 前端只读查询；写入口供 router 回调与后台 job 调用。

type LLMMonitorService struct {
	spendRepo  repository.ILLMSpendLogRepository
	dailyRepo  repository.ILLMDailySpendRepository
	errorRepo  repository.ILLMErrorLogRepository
	auditRepo  repository.ILLMAuditLogRepository
	healthRepo repository.ILLMHealthCheckRepository
	promptRepo repository.ILLMPromptRepository
}

func NewLLMMonitorService(sp repository.ILLMSpendLogRepository, d repository.ILLMDailySpendRepository, e repository.ILLMErrorLogRepository, a repository.ILLMAuditLogRepository, h repository.ILLMHealthCheckRepository, p repository.ILLMPromptRepository) *LLMMonitorService {
	return &LLMMonitorService{spendRepo: sp, dailyRepo: d, errorRepo: e, auditRepo: a, healthRepo: h, promptRepo: p}
}

// ===== SpendLog =====
func (s *LLMMonitorService) ListSpendLogs(ctx context.Context, userId, teamId, model, apiKey string, dateRange []string, pn, ps int) ([]*entity.LLMSpendLog, int64, error) {
	return s.spendRepo.List(ctx, userId, teamId, model, apiKey, dateRange, pn, ps)
}
func (s *LLMMonitorService) GetSpendLog(ctx context.Context, requestId string) (*entity.LLMSpendLog, error) {
	return s.spendRepo.Get(ctx, requestId)
}
func (s *LLMMonitorService) AddSpendLog(ctx context.Context, e *entity.LLMSpendLog) error {
	return s.spendRepo.Add(ctx, e)
}

// ===== DailySpend =====
func (s *LLMMonitorService) ListDailySpend(ctx context.Context, scopeType, scopeId string, dateRange []string, pn, ps int) ([]*entity.LLMDailySpend, int64, error) {
	return s.dailyRepo.List(ctx, []string{scopeType}, []string{scopeId}, dateRange, pn, ps)
}
func (s *LLMMonitorService) UpsertDailySpend(ctx context.Context, e *entity.LLMDailySpend) error {
	return s.dailyRepo.Upsert(ctx, e)
}

// AggregateSpend 花费归集报表：dimension ∈ date/key/user/model，返回维度聚合行 + 窗口合计。
func (s *LLMMonitorService) AggregateSpend(ctx context.Context, dimension string, dateRange []string) ([]*entity.LLMSpendAggregate, *entity.LLMSpendTotals, error) {
	switch dimension {
	case "date", "key", "user", "model":
	default:
		return nil, nil, errors.New("dimension 须为 date/key/user/model")
	}
	list, err := s.dailyRepo.Aggregate(ctx, dimension, dateRange, 50)
	if err != nil {
		return nil, nil, err
	}
	totals, err := s.dailyRepo.AggregateTotals(ctx, dateRange)
	if err != nil {
		return nil, nil, err
	}
	return list, totals, nil
}

// ===== ErrorLog =====
func (s *LLMMonitorService) ListErrorLogs(ctx context.Context, model, apiKey string, dateRange []string, pn, ps int) ([]*entity.LLMErrorLog, int64, error) {
	return s.errorRepo.List(ctx, model, apiKey, dateRange, pn, ps)
}
func (s *LLMMonitorService) AddErrorLog(ctx context.Context, e *entity.LLMErrorLog) error {
	return s.errorRepo.Add(ctx, e)
}

// ===== AuditLog =====
func (s *LLMMonitorService) ListAuditLogs(ctx context.Context, tableName, action string, pn, ps int) ([]*entity.LLMAuditLog, int64, error) {
	return s.auditRepo.List(ctx, tableName, action, pn, ps)
}
func (s *LLMMonitorService) AddAuditLog(ctx context.Context, e *entity.LLMAuditLog) error {
	return s.auditRepo.Add(ctx, e)
}

// ===== HealthCheck =====
func (s *LLMMonitorService) ListHealthChecks(ctx context.Context, modelName, status string, pn, ps int) ([]*entity.LLMHealthCheck, int64, error) {
	return s.healthRepo.List(ctx, modelName, status, pn, ps)
}
func (s *LLMMonitorService) AddHealthCheck(ctx context.Context, e *entity.LLMHealthCheck) (string, error) {
	return s.healthRepo.Add(ctx, e)
}

// ===== Prompt =====
func (s *LLMMonitorService) ListPrompts(ctx context.Context, name, env string, pn, ps int) ([]*entity.LLMPrompt, int64, error) {
	return s.promptRepo.List(ctx, name, env, pn, ps)
}
func (s *LLMMonitorService) GetPromptVersions(ctx context.Context, promptId string) ([]*entity.LLMPrompt, error) {
	return s.promptRepo.Get(ctx, promptId)
}
func (s *LLMMonitorService) AddPrompt(ctx context.Context, e *entity.LLMPrompt) (string, error) {
	return s.promptRepo.Add(ctx, e)
}
func (s *LLMMonitorService) EditPrompt(ctx context.Context, e *entity.LLMPrompt) error {
	return s.promptRepo.Edit(ctx, e)
}
func (s *LLMMonitorService) DeletePrompts(ctx context.Context, ids []string) error {
	return s.promptRepo.Delete(ctx, ids)
}
