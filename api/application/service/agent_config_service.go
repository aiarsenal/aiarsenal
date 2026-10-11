package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// AgentConfigService 配置中心：resource / bundle / bundle-item / bundle-skill。
type AgentConfigService struct {
	resourceRepo repository.IAgentConfigResourceRepository
	bundleRepo   repository.IAgentConfigBundleRepository
	itemRepo     repository.IAgentConfigBundleItemRepository
	skillRepo    repository.IAgentConfigBundleSkillRepository
}

func NewAgentConfigService(r repository.IAgentConfigResourceRepository, b repository.IAgentConfigBundleRepository, i repository.IAgentConfigBundleItemRepository, s repository.IAgentConfigBundleSkillRepository) *AgentConfigService {
	return &AgentConfigService{resourceRepo: r, bundleRepo: b, itemRepo: i, skillRepo: s}
}

// ---- ConfigResource ----
func (s *AgentConfigService) ListResources(ctx context.Context, name, resourceType string, pageNum, pageSize int) ([]*entity.AgentConfigResource, int64, error) {
	return s.resourceRepo.List(ctx, name, resourceType, pageNum, pageSize)
}
func (s *AgentConfigService) GetResource(ctx context.Context, id string) (*entity.AgentConfigResource, error) {
	return s.resourceRepo.Get(ctx, id)
}
func (s *AgentConfigService) AddResource(ctx context.Context, e *entity.AgentConfigResource) (string, error) {
	return s.resourceRepo.Add(ctx, e)
}
func (s *AgentConfigService) EditResource(ctx context.Context, e *entity.AgentConfigResource) error {
	return s.resourceRepo.Edit(ctx, e)
}
func (s *AgentConfigService) DeleteResources(ctx context.Context, ids []string) error {
	return s.resourceRepo.Delete(ctx, ids)
}

// ---- ConfigBundle ----
func (s *AgentConfigService) ListBundles(ctx context.Context, name string, pageNum, pageSize int) ([]*entity.AgentConfigBundle, int64, error) {
	return s.bundleRepo.List(ctx, name, pageNum, pageSize)
}
func (s *AgentConfigService) GetBundle(ctx context.Context, id string) (*entity.AgentConfigBundle, error) {
	return s.bundleRepo.Get(ctx, id)
}
func (s *AgentConfigService) AddBundle(ctx context.Context, e *entity.AgentConfigBundle) (string, error) {
	return s.bundleRepo.Add(ctx, e)
}
func (s *AgentConfigService) EditBundle(ctx context.Context, e *entity.AgentConfigBundle) error {
	return s.bundleRepo.Edit(ctx, e)
}
func (s *AgentConfigService) DeleteBundles(ctx context.Context, ids []string) error {
	return s.bundleRepo.Delete(ctx, ids)
}

// ---- BundleItem ----
func (s *AgentConfigService) ListBundleItems(ctx context.Context, bundleId string, pageNum, pageSize int) ([]*entity.AgentConfigBundleItem, int64, error) {
	return s.itemRepo.List(ctx, bundleId, pageNum, pageSize)
}
func (s *AgentConfigService) GetBundleItem(ctx context.Context, id string) (*entity.AgentConfigBundleItem, error) {
	return s.itemRepo.Get(ctx, id)
}
func (s *AgentConfigService) AddBundleItem(ctx context.Context, e *entity.AgentConfigBundleItem) (string, error) {
	return s.itemRepo.Add(ctx, e)
}
func (s *AgentConfigService) EditBundleItem(ctx context.Context, e *entity.AgentConfigBundleItem) error {
	return s.itemRepo.Edit(ctx, e)
}
func (s *AgentConfigService) DeleteBundleItems(ctx context.Context, ids []string) error {
	return s.itemRepo.Delete(ctx, ids)
}

// ---- BundleSkill ----
func (s *AgentConfigService) ListBundleSkills(ctx context.Context, bundleId string, pageNum, pageSize int) ([]*entity.AgentConfigBundleSkill, int64, error) {
	return s.skillRepo.List(ctx, bundleId, pageNum, pageSize)
}
func (s *AgentConfigService) GetBundleSkill(ctx context.Context, id string) (*entity.AgentConfigBundleSkill, error) {
	return s.skillRepo.Get(ctx, id)
}
func (s *AgentConfigService) AddBundleSkill(ctx context.Context, e *entity.AgentConfigBundleSkill) (string, error) {
	return s.skillRepo.Add(ctx, e)
}
func (s *AgentConfigService) EditBundleSkill(ctx context.Context, e *entity.AgentConfigBundleSkill) error {
	return s.skillRepo.Edit(ctx, e)
}
func (s *AgentConfigService) DeleteBundleSkills(ctx context.Context, ids []string) error {
	return s.skillRepo.Delete(ctx, ids)
}
