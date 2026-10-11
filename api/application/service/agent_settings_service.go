package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// AgentSettingsService 系统设置：admin 镜像卡片（system_image_setting）。
type AgentSettingsService struct {
	imageRepo repository.IAgentSystemImageSettingRepository
}

func NewAgentSettingsService(r repository.IAgentSystemImageSettingRepository) *AgentSettingsService {
	return &AgentSettingsService{imageRepo: r}
}

func (s *AgentSettingsService) ListImages(ctx context.Context, agent, version string) ([]*entity.AgentSystemImageSetting, error) {
	return s.imageRepo.List(ctx, agent, version)
}
func (s *AgentSettingsService) UpsertImage(ctx context.Context, e *entity.AgentSystemImageSetting) error {
	return s.imageRepo.Upsert(ctx, e)
}
func (s *AgentSettingsService) DeleteImage(ctx context.Context, agent, version string) error {
	return s.imageRepo.Delete(ctx, agent, version)
}
