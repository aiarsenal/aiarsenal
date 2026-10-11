package service

import (
	"context"
	"errors"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 项目服务。项目=顶层资源容器；本服务管项目 CRUD 与概览。
// 创建项目时自动写入创建人为 owner + 一个 owner 成员记录 + 默认状态流（见 EnsureDefaults）。

type ProjectService struct {
	repo       repository.IProjectRepository
	memberRepo repository.IProjectMemberRepository
	statusRepo repository.IProjectStatusRepository
	docSpace   *DocSpaceService // 项目空间目录同步（延迟接线；nil=未接进程钩子静默跳过）
	kbSpace    *KbSpaceService  // KB 项目空间目录同步（延迟接线，与文档空间同构）
}

func NewProjectService(r repository.IProjectRepository, mr repository.IProjectMemberRepository, sr repository.IProjectStatusRepository) *ProjectService {
	return &ProjectService{repo: r, memberRepo: mr, statusRepo: sr}
}

// SetDocSpaceSync 接入项目空间同步器（api/ops 装配期；防构造循环用 setter）
func (s *ProjectService) SetDocSpaceSync(d *DocSpaceService) { s.docSpace = d }

// SetKbSpaceSync 接入 KB 项目空间同步器（api 装配期）
func (s *ProjectService) SetKbSpaceSync(x *KbSpaceService) { s.kbSpace = x }

// syncKbSpace KB 项目空间目录同步（nil=未接线静默跳过；失败由同步器内部吸收）
func (s *ProjectService) syncKbSpace(ctx context.Context, projectID string) {
	if s.kbSpace != nil {
		s.kbSpace.SyncProject(ctx, projectID)
	}
}

func (s *ProjectService) List(ctx context.Context, name, status string, pn, ps int) ([]*entity.Project, int64, error) {
	return s.repo.List(ctx, name, status, pn, ps)
}

func (s *ProjectService) All(ctx context.Context) ([]*entity.Project, error) {
	return s.repo.All(ctx)
}

func (s *ProjectService) Get(ctx context.Context, id string) (*entity.Project, error) {
	return s.repo.Get(ctx, id)
}

func (s *ProjectService) Add(ctx context.Context, e *entity.Project) (string, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return "", errors.New("name 不能为空")
	}
	if e.Status == "" {
		e.Status = "planning"
	}
	if e.Priority == "" {
		e.Priority = "medium"
	}
	if e.Visibility == "" {
		e.Visibility = "private"
	}
	id, err := s.repo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	// 创建人即 owner 成员
	if e.Owner != "" {
		_, _ = s.memberRepo.Add(ctx, &entity.ProjectMember{ProjectId: id, UserId: e.Owner, Role: "owner", CreatedBy: e.Owner})
	}
	// 默认状态流：todo / in_progress / done
	s.ensureDefaultStatuses(ctx, id, e.Owner)
	// 项目空间目录（aa_doc / aa_kb 库；best-effort，失败下次对账自愈）
	if s.docSpace != nil {
		s.docSpace.SyncProject(ctx, id)
	}
	s.syncKbSpace(ctx, id)
	return id, nil
}

// ensureDefaultStatuses 为新项目写入三段默认状态（若该项目尚无任何状态）。
func (s *ProjectService) ensureDefaultStatuses(ctx context.Context, projectID, userID string) {
	if list, err := s.statusRepo.ListByProject(ctx, projectID); err == nil && len(list) > 0 {
		return
	}
	defaults := []entity.ProjectStatus{
		{ProjectId: projectID, Key: "todo", Name: "待办", Color: "default", Category: "todo", SortOrder: 1, IsDefault: 1, CreatedBy: userID},
		{ProjectId: projectID, Key: "doing", Name: "进行中", Color: "processing", Category: "doing", SortOrder: 2},
		{ProjectId: projectID, Key: "done", Name: "已完成", Color: "success", Category: "done", SortOrder: 3},
	}
	for i := range defaults {
		if defaults[i].CreatedBy == "" {
			defaults[i].CreatedBy = userID
		}
		_, _ = s.statusRepo.Add(ctx, &defaults[i])
	}
}

func (s *ProjectService) Edit(ctx context.Context, e *entity.Project) error {
	if e.ProjectId == "" {
		return errors.New("projectId 不能为空")
	}
	if err := s.repo.Edit(ctx, e); err != nil {
		return err
	}
	// 改名同步项目空间目录名
	if s.docSpace != nil {
		s.docSpace.SyncProject(ctx, e.ProjectId)
	}
	s.syncKbSpace(ctx, e.ProjectId)
	return nil
}

func (s *ProjectService) Delete(ctx context.Context, ids []string) error {
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	// 软删后目录保留，名字追加（已归档）
	if s.docSpace != nil {
		for _, id := range ids {
			s.docSpace.SyncProjectDeleted(ctx, id)
		}
	}
	if s.kbSpace != nil {
		for _, id := range ids {
			s.kbSpace.SyncProjectDeleted(ctx, id)
		}
	}
	return nil
}

// Overview 项目概览（详情抽屉用）：成员数 + 任务统计 + 里程碑进度。
func (s *ProjectService) Overview(ctx context.Context, id string) (map[string]any, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	members, _ := s.memberRepo.ListByProject(ctx, id)
	return map[string]any{
		"project":     p,
		"memberCount": len(members),
	}, nil
}
