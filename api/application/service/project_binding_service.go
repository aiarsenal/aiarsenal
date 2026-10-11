package service

import (
	"context"
	"errors"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 资源绑定服务：项目 ↔ AI 资源（agent/skill/kb/llm_budget/llm_key/mcp_group）。
// target 是各模块 biz-id 字符串软引用；target_name 仅缓存展示名，不做跨库 JOIN。
type ProjectBindingService struct {
	repo   repository.IProjectBindingRepository
	mcpSvc *AgentMcpService // agent/mcp_group 绑定变化 → 受影响实例收敛 MCP key/配置（main.go 装配后回指）
}

func NewProjectBindingService(r repository.IProjectBindingRepository) *ProjectBindingService {
	return &ProjectBindingService{repo: r}
}

// SetMcpService main.go 装配完 AgentMcpService 后回指（其构造依赖 AgentInstanceService，晚于本服务）。
func (s *ProjectBindingService) SetMcpService(m *AgentMcpService) { s.mcpSvc = m }

// syncMcp 绑定变化联动实例 MCP（best-effort：失败只记日志，不影响绑定操作本身）。
func (s *ProjectBindingService) syncMcp(ctx context.Context, affected []*entity.ProjectBinding, by string) {
	if s.mcpSvc == nil {
		return
	}
	s.mcpSvc.OnBindingsChanged(ctx, affected, by)
}

func (s *ProjectBindingService) List(ctx context.Context, projectID, kind string, pn, ps int) ([]*entity.ProjectBinding, int64, error) {
	return s.repo.List(ctx, projectID, kind, pn, ps)
}

func (s *ProjectBindingService) ListByProject(ctx context.Context, projectID string) ([]*entity.ProjectBinding, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *ProjectBindingService) Get(ctx context.Context, id string) (*entity.ProjectBinding, error) {
	return s.repo.Get(ctx, id)
}

var validBindingKinds = map[string]bool{
	"agent": true, "skill": true, "kb": true, "llm_budget": true, "llm_key": true, "mcp_group": true,
}

func (s *ProjectBindingService) Add(ctx context.Context, e *entity.ProjectBinding) (string, error) {
	e.Kind = strings.TrimSpace(e.Kind)
	if e.ProjectId == "" || e.Kind == "" || e.TargetId == "" {
		return "", errors.New("projectId/kind/targetId 不能为空")
	}
	if !validBindingKinds[e.Kind] {
		return "", errors.New("不支持的 kind：" + e.Kind)
	}
	if e.Enabled == 0 {
		e.Enabled = 1
	}
	id, err := s.repo.Add(ctx, e)
	if err == nil {
		s.syncMcp(ctx, []*entity.ProjectBinding{e}, e.CreatedBy)
	}
	return id, err
}

func (s *ProjectBindingService) Edit(ctx context.Context, e *entity.ProjectBinding) error {
	if e.BindingId == "" {
		return errors.New("bindingId 不能为空")
	}
	// 改动可能换 target/project（换绑）：新旧两端都算受影响，交由联动方各自重算。
	old, _ := s.repo.Get(ctx, e.BindingId)
	if err := s.repo.Edit(ctx, e); err != nil {
		return err
	}
	affected := []*entity.ProjectBinding{e}
	if old != nil && (old.TargetId != e.TargetId || old.ProjectId != e.ProjectId) {
		affected = append(affected, old)
	}
	s.syncMcp(ctx, affected, e.CreatedBy)
	return nil
}

func (s *ProjectBindingService) Delete(ctx context.Context, ids []string) error {
	// 软删前取回原行（删后查不到），供联动定位受影响实例。
	var affected []*entity.ProjectBinding
	for _, id := range ids {
		if b, err := s.repo.Get(ctx, id); err == nil && b != nil {
			affected = append(affected, b)
		}
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	s.syncMcp(ctx, affected, "")
	return nil
}
