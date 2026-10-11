package service

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 动态服务：项目/任务活动流（只读）。
type ProjectActivityService struct {
	repo repository.IProjectTaskActivityRepository
}

func NewProjectActivityService(r repository.IProjectTaskActivityRepository) *ProjectActivityService {
	return &ProjectActivityService{repo: r}
}

func (s *ProjectActivityService) List(ctx context.Context, projectID, taskID, action string, pn, ps int) ([]*entity.ProjectTaskActivity, int64, error) {
	return s.repo.List(ctx, projectID, taskID, action, pn, ps)
}
