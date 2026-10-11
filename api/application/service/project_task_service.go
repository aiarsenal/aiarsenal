package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 任务服务：任务/子任务 CRUD + 评论 + 活动流写入。
// 任务 status 存 aa_project_status.key；新建任务缺省状态时取项目默认状态。

type ProjectTaskService struct {
	repo        repository.IProjectTaskRepository
	commentRepo repository.IProjectTaskCommentRepository
	activity    repository.IProjectTaskActivityRepository
	statusRepo  repository.IProjectStatusRepository
	userRepo    repository.IUserRepository // 跨库回填负责人姓名（aiarsenal.aa_user）
}

func NewProjectTaskService(r repository.IProjectTaskRepository, cr repository.IProjectTaskCommentRepository, ar repository.IProjectTaskActivityRepository, sr repository.IProjectStatusRepository, ur repository.IUserRepository) *ProjectTaskService {
	return &ProjectTaskService{repo: r, commentRepo: cr, activity: ar, statusRepo: sr, userRepo: ur}
}

func (s *ProjectTaskService) List(ctx context.Context, projectID, status, assignee, priority string, pn, ps int) ([]*entity.ProjectTask, int64, error) {
	list, total, err := s.repo.List(ctx, projectID, status, assignee, priority, pn, ps)
	if err != nil {
		return nil, 0, err
	}
	enrichTaskAssignees(ctx, s.userRepo, list)
	return list, total, nil
}

func (s *ProjectTaskService) ListByMilestone(ctx context.Context, projectID, milestoneID string) ([]*entity.ProjectTask, error) {
	list, err := s.repo.ListByMilestone(ctx, projectID, milestoneID)
	if err != nil {
		return nil, err
	}
	enrichTaskAssignees(ctx, s.userRepo, list)
	return list, nil
}

func (s *ProjectTaskService) Get(ctx context.Context, id string) (*entity.ProjectTask, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	enrichTaskAssignees(ctx, s.userRepo, []*entity.ProjectTask{t})
	return t, nil
}

func (s *ProjectTaskService) Add(ctx context.Context, e *entity.ProjectTask, userID string) (string, error) {
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return "", errors.New("title 不能为空")
	}
	if e.ProjectId == "" {
		return "", errors.New("projectId 不能为空")
	}
	if e.Priority == "" {
		e.Priority = "medium"
	}
	if e.Status == "" {
		if def, err := s.statusRepo.GetDefault(ctx, e.ProjectId); err == nil && def != nil {
			e.Status = def.Key
		} else {
			e.Status = "todo"
		}
	}
	if e.ReporterId == "" {
		e.ReporterId = userID
	}
	if e.CreatedBy == "" {
		e.CreatedBy = userID
	}
	e.UpdatedBy = userID
	id, err := s.repo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	recordActivity(ctx, s.activity, e.ProjectId, id, userID, "created", e.Title, "")
	if e.AssigneeId != "" {
		recordActivity(ctx, s.activity, e.ProjectId, id, userID, "assigned", e.Title, e.AssigneeId)
	}
	return id, nil
}

func (s *ProjectTaskService) Edit(ctx context.Context, e *entity.ProjectTask, userID string) error {
	if e.TaskId == "" {
		return errors.New("taskId 不能为空")
	}
	old, _ := s.repo.Get(ctx, e.TaskId)
	e.UpdatedBy = userID
	if err := s.repo.Edit(ctx, e); err != nil {
		return err
	}
	recordActivity(ctx, s.activity, e.ProjectId, e.TaskId, userID, "updated", e.Title, "")
	if old != nil && old.Status != e.Status {
		recordActivity(ctx, s.activity, e.ProjectId, e.TaskId, userID, "status_changed", e.Title, detailJSON("from", old.Status, "to", e.Status))
	}
	if old != nil && old.AssigneeId != e.AssigneeId {
		recordActivity(ctx, s.activity, e.ProjectId, e.TaskId, userID, "assigned", e.Title, e.AssigneeId)
	}
	return nil
}

func (s *ProjectTaskService) Delete(ctx context.Context, ids []string, userID, projectID string) error {
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		recordActivity(ctx, s.activity, projectID, id, userID, "archived", "", "")
	}
	return nil
}

// ===== 评论 =====

func (s *ProjectTaskService) ListComments(ctx context.Context, taskID string) ([]*entity.ProjectTaskComment, error) {
	list, err := s.commentRepo.List(ctx, taskID)
	if err != nil {
		return nil, err
	}
	// 评论人姓名回填（失败静默，展示回落 userId）
	ids := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, c := range list {
		if c.UserId != "" && !seen[c.UserId] {
			seen[c.UserId] = true
			ids = append(ids, c.UserId)
		}
	}
	m := userNameMap(ctx, s.userRepo, ids)
	for _, c := range list {
		if n, ok := m[c.UserId]; ok {
			c.UserName = n
		}
	}
	return list, nil
}

func (s *ProjectTaskService) AddComment(ctx context.Context, e *entity.ProjectTaskComment, userID string) (string, error) {
	if e.TaskId == "" {
		return "", errors.New("taskId 不能为空")
	}
	e.Content = strings.TrimSpace(e.Content)
	if e.Content == "" {
		return "", errors.New("content 不能为空")
	}
	e.UserId = userID
	// 回填 projectId（若未带）
	if e.ProjectId == "" {
		if t, err := s.repo.Get(ctx, e.TaskId); err == nil {
			e.ProjectId = t.ProjectId
		}
	}
	id, err := s.commentRepo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	recordActivity(ctx, s.activity, e.ProjectId, e.TaskId, userID, "commented", "", "")
	return id, nil
}

func (s *ProjectTaskService) DeleteComment(ctx context.Context, ids []string) error {
	return s.commentRepo.Delete(ctx, ids)
}

// ===== 活动流（共享写入助手，board/task 共用）=====

// recordActivity 写一条活动记录；detail 形如 {"from":"todo","to":"done"}。
func recordActivity(ctx context.Context, repo repository.IProjectTaskActivityRepository, projectID, taskID, userID, action, target, detail string) {
	if repo == nil {
		return
	}
	_, _ = repo.Add(ctx, &entity.ProjectTaskActivity{
		ProjectId: projectID, TaskId: taskID, UserId: userID, Action: action, Target: target, Detail: detail,
	})
}

// detailJSON 组装简单 from/to 明细；失败返回 ""。
func detailJSON(kv ...string) string {
	if len(kv)%2 != 0 || len(kv) == 0 {
		return ""
	}
	m := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}
