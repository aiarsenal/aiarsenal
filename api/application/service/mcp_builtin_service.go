package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// MCP 内置 Prompt / Resource 服务。
// 这些是网关自身向 AI 客户端暴露的 prompt 与 resource（与上游 server 无关），
// 由 SSE 端点在 list/get 时合并进返回结果。

type McpBuiltinService struct {
	promptRepo   repository.IMcpBuiltinPromptRepository
	resourceRepo repository.IMcpBuiltinResourceRepository
}

func NewMcpBuiltinService(p repository.IMcpBuiltinPromptRepository, r repository.IMcpBuiltinResourceRepository) *McpBuiltinService {
	return &McpBuiltinService{promptRepo: p, resourceRepo: r}
}

// ===== Prompt =====
func (s *McpBuiltinService) ListPrompts(ctx context.Context, name string, pn, ps int) ([]*entity.McpBuiltinPrompt, int64, error) {
	return s.promptRepo.List(ctx, name, pn, ps)
}
func (s *McpBuiltinService) AllPrompts(ctx context.Context) ([]*entity.McpBuiltinPrompt, error) {
	return s.promptRepo.All(ctx)
}
func (s *McpBuiltinService) GetPrompt(ctx context.Context, id string) (*entity.McpBuiltinPrompt, error) {
	return s.promptRepo.Get(ctx, id)
}
func (s *McpBuiltinService) AddPrompt(ctx context.Context, e *entity.McpBuiltinPrompt) (string, error) {
	if e.Name == "" {
		return "", errors.New("name 不能为空")
	}
	if e.Template == "" {
		return "", errors.New("template 不能为空")
	}
	if e.Arguments == "" {
		e.Arguments = "[]"
	}
	if e.Enabled == 0 {
		e.Enabled = 1
	}
	return s.promptRepo.Add(ctx, e)
}
func (s *McpBuiltinService) EditPrompt(ctx context.Context, e *entity.McpBuiltinPrompt) error {
	if e.PromptId == "" {
		return errors.New("promptId 不能为空")
	}
	return s.promptRepo.Edit(ctx, e)
}
func (s *McpBuiltinService) DeletePrompts(ctx context.Context, ids []string) error {
	return s.promptRepo.Delete(ctx, ids)
}

// ===== Resource =====
func (s *McpBuiltinService) ListResources(ctx context.Context, name string, pn, ps int) ([]*entity.McpBuiltinResource, int64, error) {
	return s.resourceRepo.List(ctx, name, pn, ps)
}
func (s *McpBuiltinService) AllResources(ctx context.Context) ([]*entity.McpBuiltinResource, error) {
	return s.resourceRepo.All(ctx)
}
func (s *McpBuiltinService) GetResource(ctx context.Context, id string) (*entity.McpBuiltinResource, error) {
	return s.resourceRepo.Get(ctx, id)
}
func (s *McpBuiltinService) AddResource(ctx context.Context, e *entity.McpBuiltinResource) (string, error) {
	if e.Uri == "" {
		return "", errors.New("uri 不能为空")
	}
	if e.Content == "" {
		return "", errors.New("content 不能为空")
	}
	if e.Enabled == 0 {
		e.Enabled = 1
	}
	return s.resourceRepo.Add(ctx, e)
}
func (s *McpBuiltinService) EditResource(ctx context.Context, e *entity.McpBuiltinResource) error {
	if e.ResourceId == "" {
		return errors.New("resourceId 不能为空")
	}
	return s.resourceRepo.Edit(ctx, e)
}
func (s *McpBuiltinService) DeleteResources(ctx context.Context, ids []string) error {
	return s.resourceRepo.Delete(ctx, ids)
}
