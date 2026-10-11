package service

import (
	"context"

	"aiarsenal/domain/repository"
	"aiarsenal/interfaces/dto"
)

// AgentDashboardService 概览：聚合实例/团队/模型/扫描/调用 统计。
// Phase 1：用各仓储 List 的 total 计数（page 1 size 1）；调用/成本 24h 留 Phase 2 接入网关后填充。
type AgentDashboardService struct {
	instanceRepo repository.IAgentInstanceRepository
	teamRepo     repository.IAgentTeamRepository
	modelRepo    repository.IAgentModelRepository
	scanJobRepo  repository.IAgentSecurityScanJobRepository
}

func NewAgentDashboardService(i repository.IAgentInstanceRepository, t repository.IAgentTeamRepository, m repository.IAgentModelRepository, sj repository.IAgentSecurityScanJobRepository) *AgentDashboardService {
	return &AgentDashboardService{instanceRepo: i, teamRepo: t, modelRepo: m, scanJobRepo: sj}
}

func (s *AgentDashboardService) Overview(ctx context.Context, userId string) (*dto.AgentDashboardRes, error) {
	res := &dto.AgentDashboardRes{}
	if _, total, err := s.instanceRepo.List(ctx, userId, "", -1, 1, 1); err == nil {
		res.InstanceTotal = total
	}
	if _, total, err := s.instanceRepo.List(ctx, userId, "", 1, 1, 1); err == nil {
		res.InstanceRunning = total
	}
	if _, total, err := s.teamRepo.List(ctx, userId, "", 1, 1); err == nil {
		res.TeamTotal = total
	}
	if _, total, err := s.modelRepo.List(ctx, "", "", 1, 1); err == nil {
		res.ModelTotal = total
	}
	// scan job active：status 非 done/failed（这里取 running 计数近似）
	if _, total, err := s.scanJobRepo.List(ctx, "", "1", 1, 1); err == nil {
		res.ScanJobActive = total
	}
	// invocation24h / cost24h：留 Phase 2 接入 AI 网关上报后填充
	return res, nil
}
