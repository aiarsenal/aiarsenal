package service

import (
	"context"
	"errors"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 里程碑服务：里程碑 CRUD；进度由关联任务汇总（done 类状态任务占比）。
type ProjectMilestoneService struct {
	repo       repository.IProjectMilestoneRepository
	taskRepo   repository.IProjectTaskRepository
	statusRepo repository.IProjectStatusRepository // done 类状态按 category 判定（兜底 key done/closed）
}

func NewProjectMilestoneService(r repository.IProjectMilestoneRepository, tr repository.IProjectTaskRepository, sr repository.IProjectStatusRepository) *ProjectMilestoneService {
	return &ProjectMilestoneService{repo: r, taskRepo: tr, statusRepo: sr}
}

func (s *ProjectMilestoneService) List(ctx context.Context, projectID, status string, pn, ps int) ([]*entity.ProjectMilestone, int64, error) {
	list, total, err := s.repo.List(ctx, projectID, status, pn, ps)
	if err != nil {
		return nil, 0, err
	}
	s.fillProgress(ctx, projectID, list)
	return list, total, nil
}

// doneKeys 汇总项目的「完成类」状态键：category=done 的状态 + 兜底 done/closed。
func (s *ProjectMilestoneService) doneKeys(ctx context.Context, projectID string) map[string]bool {
	keys := map[string]bool{"done": true, "closed": true}
	if s.statusRepo == nil {
		return keys
	}
	if sts, err := s.statusRepo.ListByProject(ctx, projectID); err == nil {
		for _, st := range sts {
			if st.Category == "done" {
				keys[st.Key] = true
			}
		}
	}
	return keys
}

// fillProgress 回填各里程碑任务总数/完成数/完成率（best-effort，失败留零）。
func (s *ProjectMilestoneService) fillProgress(ctx context.Context, projectID string, list []*entity.ProjectMilestone) {
	if len(list) == 0 {
		return
	}
	done := s.doneKeys(ctx, projectID)
	for _, ms := range list {
		tasks, err := s.taskRepo.ListByMilestone(ctx, projectID, ms.MilestoneId)
		if err != nil {
			continue
		}
		ms.TaskTotal = len(tasks)
		for _, t := range tasks {
			if done[t.Status] {
				ms.TaskDone++
			}
		}
		if ms.TaskTotal > 0 {
			ms.Progress = ms.TaskDone * 100 / ms.TaskTotal
		}
	}
}

func (s *ProjectMilestoneService) Get(ctx context.Context, id string) (*entity.ProjectMilestone, error) {
	return s.repo.Get(ctx, id)
}

func (s *ProjectMilestoneService) Add(ctx context.Context, e *entity.ProjectMilestone) (string, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return "", errors.New("name 不能为空")
	}
	if e.Status == "" {
		e.Status = "open"
	}
	return s.repo.Add(ctx, e)
}

func (s *ProjectMilestoneService) Edit(ctx context.Context, e *entity.ProjectMilestone) error {
	if e.MilestoneId == "" {
		return errors.New("milestoneId 不能为空")
	}
	return s.repo.Edit(ctx, e)
}

func (s *ProjectMilestoneService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}

// Progress 里程碑进度：关联任务中 done 类状态占比（0-100）。
func (s *ProjectMilestoneService) Progress(ctx context.Context, projectID, milestoneID string) (total, done, rate int, err error) {
	tasks, err := s.taskRepo.ListByMilestone(ctx, projectID, milestoneID)
	if err != nil {
		return
	}
	doneKeys := s.doneKeys(ctx, projectID)
	total = len(tasks)
	for _, t := range tasks {
		if doneKeys[t.Status] {
			done++
		}
	}
	if total > 0 {
		rate = done * 100 / total
	}
	return
}
