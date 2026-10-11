package service

import (
	"context"
	"errors"

	"aiarsenal/domain/repository"
)

// 报表服务（简化 v1）：任务状态/优先级分布、完成率、里程碑进度、成员负载（由任务估算/已耗工时汇总）。
type ProjectReportService struct {
	taskRepo      repository.IProjectTaskRepository
	milestoneRepo repository.IProjectMilestoneRepository
	msSvc         *ProjectMilestoneService
}

func NewProjectReportService(tr repository.IProjectTaskRepository, mr repository.IProjectMilestoneRepository, msSvc *ProjectMilestoneService) *ProjectReportService {
	return &ProjectReportService{taskRepo: tr, milestoneRepo: mr, msSvc: msSvc}
}

// Overview 聚合项目报表数据。
func (s *ProjectReportService) Overview(ctx context.Context, projectID string) (map[string]any, error) {
	if projectID == "" {
		return nil, errors.New("projectId 不能为空")
	}
	tasks, err := s.taskRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byStatus := map[string]int{}
	byPriority := map[string]int{}
	load := map[string]*loadAcc{}
	done := 0
	for _, t := range tasks {
		byStatus[t.Status]++
		byPriority[t.Priority]++
		if t.Status == "done" || t.Status == "closed" {
			done++
		}
		uid := t.AssigneeId
		if uid == "" {
			uid = "__unassigned__"
		}
		a := load[uid]
		if a == nil {
			a = &loadAcc{userId: uid}
			load[uid] = a
		}
		a.count++
		a.estimate += t.EstimateHours
		a.spent += t.SpentHours
	}
	rate := 0
	if len(tasks) > 0 {
		rate = done * 100 / len(tasks)
	}

	// 里程碑进度
	milestones, _ := s.milestoneRepo.ListByProject(ctx, projectID)
	msRate := []map[string]any{}
	for _, m := range milestones {
		total, d, r, _ := s.msSvc.Progress(ctx, projectID, m.MilestoneId)
		msRate = append(msRate, map[string]any{
			"milestoneId": m.MilestoneId, "name": m.Name,
			"total": total, "done": d, "progress": r,
		})
	}

	memberLoad := []map[string]any{}
	for _, a := range load {
		memberLoad = append(memberLoad, map[string]any{
			"userId": a.userId, "taskCount": a.count,
			"estimateHours": a.estimate, "spentHours": a.spent,
		})
	}

	return map[string]any{
		"totalTasks":    len(tasks),
		"byStatus":      byStatus,
		"byPriority":    byPriority,
		"doneRate":      rate,
		"milestoneRate": msRate,
		"memberLoad":    memberLoad,
	}, nil
}

type loadAcc struct {
	userId   string
	count    int
	estimate float64
	spent    float64
}
