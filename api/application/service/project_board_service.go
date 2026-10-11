package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 看板服务：返回项目的状态列 + 全部任务（前端按 status 分组渲染），并处理拖拽改状态。
type ProjectBoardService struct {
	taskRepo   repository.IProjectTaskRepository
	statusRepo repository.IProjectStatusRepository
	activity   repository.IProjectTaskActivityRepository
	userRepo   repository.IUserRepository // 跨库回填负责人姓名（aiarsenal.aa_user）
}

func NewProjectBoardService(tr repository.IProjectTaskRepository, sr repository.IProjectStatusRepository, ar repository.IProjectTaskActivityRepository, ur repository.IUserRepository) *ProjectBoardService {
	return &ProjectBoardService{taskRepo: tr, statusRepo: sr, activity: ar, userRepo: ur}
}

// Board 返回看板所需列（项目状态流）与任务。
func (s *ProjectBoardService) Board(ctx context.Context, projectID string) ([]*entity.ProjectStatus, []*entity.ProjectTask, error) {
	if projectID == "" {
		return nil, nil, errors.New("projectId 不能为空")
	}
	cols, err := s.statusRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	tasks, err := s.taskRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	enrichTaskAssignees(ctx, s.userRepo, tasks)
	return cols, tasks, nil
}

// Move 跨列拖拽：更新任务 status + sort_order，并写活动流。
func (s *ProjectBoardService) Move(ctx context.Context, taskID, status string, sortOrder int, userID string) error {
	if taskID == "" || status == "" {
		return errors.New("taskId/status 不能为空")
	}
	t, err := s.taskRepo.Get(ctx, taskID)
	if err != nil {
		return err
	}
	old := t.Status
	if err := s.taskRepo.UpdateStatus(ctx, taskID, status, sortOrder); err != nil {
		return err
	}
	if old != status {
		recordActivity(ctx, s.activity, t.ProjectId, taskID, userID, "status_changed", t.Title, detailJSON("from", old, "to", status))
	}
	return nil
}
