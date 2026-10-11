package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// AgentScanService 安全扫描：config / job / job-item / report。
type AgentScanService struct {
	configRepo repository.IAgentSecurityScanConfigRepository
	jobRepo    repository.IAgentSecurityScanJobRepository
	itemRepo   repository.IAgentSecurityScanJobItemRepository
	reportRepo repository.IAgentSecurityScanReportRepository
}

func NewAgentScanService(c repository.IAgentSecurityScanConfigRepository, j repository.IAgentSecurityScanJobRepository, i repository.IAgentSecurityScanJobItemRepository, r repository.IAgentSecurityScanReportRepository) *AgentScanService {
	return &AgentScanService{configRepo: c, jobRepo: j, itemRepo: i, reportRepo: r}
}

// ---- ScanConfig ----
func (s *AgentScanService) ListConfigs(ctx context.Context, pageNum, pageSize int) ([]*entity.AgentSecurityScanConfig, int64, error) {
	return s.configRepo.List(ctx, pageNum, pageSize)
}
func (s *AgentScanService) GetConfig(ctx context.Context, id string) (*entity.AgentSecurityScanConfig, error) {
	return s.configRepo.Get(ctx, id)
}
func (s *AgentScanService) AddConfig(ctx context.Context, e *entity.AgentSecurityScanConfig) (string, error) {
	return s.configRepo.Add(ctx, e)
}
func (s *AgentScanService) EditConfig(ctx context.Context, e *entity.AgentSecurityScanConfig) error {
	return s.configRepo.Edit(ctx, e)
}
func (s *AgentScanService) DeleteConfigs(ctx context.Context, ids []string) error {
	return s.configRepo.Delete(ctx, ids)
}

// ---- ScanJob ----
func (s *AgentScanService) ListJobs(ctx context.Context, assetType, status string, pageNum, pageSize int) ([]*entity.AgentSecurityScanJob, int64, error) {
	return s.jobRepo.List(ctx, assetType, status, pageNum, pageSize)
}
func (s *AgentScanService) GetJob(ctx context.Context, id string) (*entity.AgentSecurityScanJob, error) {
	return s.jobRepo.Get(ctx, id)
}
func (s *AgentScanService) AddJob(ctx context.Context, e *entity.AgentSecurityScanJob) (string, error) {
	return s.jobRepo.Add(ctx, e)
}
func (s *AgentScanService) EditJob(ctx context.Context, e *entity.AgentSecurityScanJob) error {
	return s.jobRepo.Edit(ctx, e)
}
func (s *AgentScanService) DeleteJobs(ctx context.Context, ids []string) error {
	return s.jobRepo.Delete(ctx, ids)
}

// ---- ScanJobItem ----
func (s *AgentScanService) ListJobItems(ctx context.Context, jobId string, pageNum, pageSize int) ([]*entity.AgentSecurityScanJobItem, int64, error) {
	return s.itemRepo.List(ctx, jobId, pageNum, pageSize)
}
func (s *AgentScanService) GetJobItem(ctx context.Context, id string) (*entity.AgentSecurityScanJobItem, error) {
	return s.itemRepo.Get(ctx, id)
}
func (s *AgentScanService) AddJobItem(ctx context.Context, e *entity.AgentSecurityScanJobItem) (string, error) {
	return s.itemRepo.Add(ctx, e)
}
func (s *AgentScanService) EditJobItem(ctx context.Context, e *entity.AgentSecurityScanJobItem) error {
	return s.itemRepo.Edit(ctx, e)
}
func (s *AgentScanService) DeleteJobItems(ctx context.Context, ids []string) error {
	return s.itemRepo.Delete(ctx, ids)
}

// ---- ScanReport（只读） ----
func (s *AgentScanService) ListReports(ctx context.Context, jobId string, pageNum, pageSize int) ([]*entity.AgentSecurityScanReport, int64, error) {
	return s.reportRepo.List(ctx, jobId, pageNum, pageSize)
}
func (s *AgentScanService) GetReport(ctx context.Context, id string) (*entity.AgentSecurityScanReport, error) {
	return s.reportRepo.Get(ctx, id)
}
