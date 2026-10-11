package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// AgentMonitorService 治理监控（只读）：model_invocation / audit_event /
// cost_record / risk_hit / audit_log。由网关写入，admin 端只读；audit_log 支持清理。
type AgentMonitorService struct {
	invRepo   repository.IAgentModelInvocationRepository
	eventRepo repository.IAgentAuditEventRepository
	costRepo  repository.IAgentCostRecordRepository
	hitRepo   repository.IAgentRiskHitRepository
	logRepo   repository.IAgentAuditLogRepository
}

func NewAgentMonitorService(i repository.IAgentModelInvocationRepository, e repository.IAgentAuditEventRepository, c repository.IAgentCostRecordRepository, h repository.IAgentRiskHitRepository, l repository.IAgentAuditLogRepository) *AgentMonitorService {
	return &AgentMonitorService{invRepo: i, eventRepo: e, costRepo: c, hitRepo: h, logRepo: l}
}

func (s *AgentMonitorService) ListInvocations(ctx context.Context, userId, modelId string, pageNum, pageSize int) ([]*entity.AgentModelInvocation, int64, error) {
	return s.invRepo.List(ctx, userId, modelId, pageNum, pageSize)
}
func (s *AgentMonitorService) GetInvocation(ctx context.Context, id string) (*entity.AgentModelInvocation, error) {
	return s.invRepo.Get(ctx, id)
}

func (s *AgentMonitorService) ListAuditEvents(ctx context.Context, userId, eventType string, pageNum, pageSize int) ([]*entity.AgentAuditEvent, int64, error) {
	return s.eventRepo.List(ctx, userId, eventType, pageNum, pageSize)
}
func (s *AgentMonitorService) GetAuditEvent(ctx context.Context, id string) (*entity.AgentAuditEvent, error) {
	return s.eventRepo.Get(ctx, id)
}

func (s *AgentMonitorService) ListCostRecords(ctx context.Context, userId string, pageNum, pageSize int) ([]*entity.AgentCostRecord, int64, error) {
	return s.costRepo.List(ctx, userId, pageNum, pageSize)
}
func (s *AgentMonitorService) GetCostRecord(ctx context.Context, id string) (*entity.AgentCostRecord, error) {
	return s.costRepo.Get(ctx, id)
}

func (s *AgentMonitorService) ListRiskHits(ctx context.Context, ruleId string, pageNum, pageSize int) ([]*entity.AgentRiskHit, int64, error) {
	return s.hitRepo.List(ctx, ruleId, pageNum, pageSize)
}
func (s *AgentMonitorService) GetRiskHit(ctx context.Context, id string) (*entity.AgentRiskHit, error) {
	return s.hitRepo.Get(ctx, id)
}

func (s *AgentMonitorService) ListAuditLogs(ctx context.Context, userId, action string, pageNum, pageSize int) ([]*entity.AgentAuditLog, int64, error) {
	return s.logRepo.List(ctx, userId, action, pageNum, pageSize)
}
func (s *AgentMonitorService) GetAuditLog(ctx context.Context, id string) (*entity.AgentAuditLog, error) {
	return s.logRepo.Get(ctx, id)
}
func (s *AgentMonitorService) DeleteAuditLogs(ctx context.Context, ids []string) error {
	return s.logRepo.Delete(ctx, ids)
}
