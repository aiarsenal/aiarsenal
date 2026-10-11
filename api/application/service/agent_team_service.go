package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// AgentTeamService Team 协作（多 agent 编排）：team / member / task / event / work-item。
// Phase 1：纯数据 CRUD + 任务派发落库（DB-only，不含 redis stream 投递，留 Phase 2/3）。
type AgentTeamService struct {
	teamRepo     repository.IAgentTeamRepository
	memberRepo   repository.IAgentTeamMemberRepository
	taskRepo     repository.IAgentTeamTaskRepository
	eventRepo    repository.IAgentTeamEventRepository
	workItemRepo repository.IAgentTeamWorkItemRepository
}

func NewAgentTeamService(t repository.IAgentTeamRepository, m repository.IAgentTeamMemberRepository, tk repository.IAgentTeamTaskRepository, e repository.IAgentTeamEventRepository, w repository.IAgentTeamWorkItemRepository) *AgentTeamService {
	return &AgentTeamService{teamRepo: t, memberRepo: m, taskRepo: tk, eventRepo: e, workItemRepo: w}
}

// ---- Team ----
func (s *AgentTeamService) ListTeams(ctx context.Context, userId, name string, pageNum, pageSize int) ([]*entity.AgentTeam, int64, error) {
	return s.teamRepo.List(ctx, userId, name, pageNum, pageSize)
}
func (s *AgentTeamService) GetTeam(ctx context.Context, id string) (*entity.AgentTeam, error) {
	return s.teamRepo.Get(ctx, id)
}
func (s *AgentTeamService) AddTeam(ctx context.Context, e *entity.AgentTeam) (string, error) {
	return s.teamRepo.Add(ctx, e)
}
func (s *AgentTeamService) EditTeam(ctx context.Context, e *entity.AgentTeam) error {
	return s.teamRepo.Edit(ctx, e)
}
func (s *AgentTeamService) DeleteTeams(ctx context.Context, ids []string) error {
	return s.teamRepo.Delete(ctx, ids)
}

// ---- Member ----
func (s *AgentTeamService) ListMembers(ctx context.Context, teamId string, pageNum, pageSize int) ([]*entity.AgentTeamMember, int64, error) {
	return s.memberRepo.List(ctx, teamId, pageNum, pageSize)
}
func (s *AgentTeamService) GetMember(ctx context.Context, id string) (*entity.AgentTeamMember, error) {
	return s.memberRepo.Get(ctx, id)
}
func (s *AgentTeamService) AddMember(ctx context.Context, e *entity.AgentTeamMember) (string, error) {
	return s.memberRepo.Add(ctx, e)
}
func (s *AgentTeamService) EditMember(ctx context.Context, e *entity.AgentTeamMember) error {
	return s.memberRepo.Edit(ctx, e)
}
func (s *AgentTeamService) DeleteMembers(ctx context.Context, ids []string) error {
	return s.memberRepo.Delete(ctx, ids)
}

// ---- Task（派发） ----
func (s *AgentTeamService) ListTasks(ctx context.Context, teamId string, status int, pageNum, pageSize int) ([]*entity.AgentTeamTask, int64, error) {
	return s.taskRepo.List(ctx, teamId, status, pageNum, pageSize)
}
func (s *AgentTeamService) GetTask(ctx context.Context, id string) (*entity.AgentTeamTask, error) {
	return s.taskRepo.Get(ctx, id)
}
func (s *AgentTeamService) DispatchTask(ctx context.Context, e *entity.AgentTeamTask) (string, error) {
	// Phase 1：仅落库为 pending；redis stream 投递留 Phase 2/3。
	return s.taskRepo.Add(ctx, e)
}

// ---- Event ----
func (s *AgentTeamService) ListEvents(ctx context.Context, teamId string, pageNum, pageSize int) ([]*entity.AgentTeamEvent, int64, error) {
	return s.eventRepo.List(ctx, teamId, pageNum, pageSize)
}

// ---- WorkItem ----
func (s *AgentTeamService) ListWorkItems(ctx context.Context, rootTaskId string, status int, pageNum, pageSize int) ([]*entity.AgentTeamWorkItem, int64, error) {
	return s.workItemRepo.List(ctx, rootTaskId, status, pageNum, pageSize)
}
func (s *AgentTeamService) GetWorkItem(ctx context.Context, id string) (*entity.AgentTeamWorkItem, error) {
	return s.workItemRepo.Get(ctx, id)
}
func (s *AgentTeamService) AddWorkItem(ctx context.Context, e *entity.AgentTeamWorkItem) (string, error) {
	return s.workItemRepo.Add(ctx, e)
}
func (s *AgentTeamService) EditWorkItem(ctx context.Context, e *entity.AgentTeamWorkItem) error {
	return s.workItemRepo.Edit(ctx, e)
}
func (s *AgentTeamService) DeleteWorkItems(ctx context.Context, ids []string) error {
	return s.workItemRepo.Delete(ctx, ids)
}
