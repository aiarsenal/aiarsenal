package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 时间线服务（简化 v1）：返回项目全部任务（带起止/负责人/里程碑），前端渲染日期条带。
type ProjectTimelineService struct {
	taskRepo repository.IProjectTaskRepository
}

func NewProjectTimelineService(tr repository.IProjectTaskRepository) *ProjectTimelineService {
	return &ProjectTimelineService{taskRepo: tr}
}

func (s *ProjectTimelineService) List(ctx context.Context, projectID string) ([]*entity.ProjectTask, error) {
	if projectID == "" {
		return nil, errors.New("projectId 不能为空")
	}
	return s.taskRepo.ListByProject(ctx, projectID)
}
