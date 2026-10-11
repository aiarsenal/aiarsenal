package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 成员服务：项目级成员与角色（owner/maintainer/member/observer），软引用 aa_user。
type ProjectMemberService struct {
	repo     repository.IProjectMemberRepository
	userRepo repository.IUserRepository // 跨库回填成员姓名（aiarsenal.aa_user）
	docSpace *DocSpaceService           // 项目空间目录授权随成员表联动（延迟接线）
	kbSpace  *KbSpaceService            // KB 项目空间目录授权随成员表联动（延迟接线）
}

func NewProjectMemberService(r repository.IProjectMemberRepository, ur repository.IUserRepository) *ProjectMemberService {
	return &ProjectMemberService{repo: r, userRepo: ur}
}

// SetDocSpaceSync 接入项目空间同步器（api/ops 装配期）
func (s *ProjectMemberService) SetDocSpaceSync(d *DocSpaceService) { s.docSpace = d }

// SetKbSpaceSync 接入 KB 项目空间同步器（api 装配期）
func (s *ProjectMemberService) SetKbSpaceSync(x *KbSpaceService) { s.kbSpace = x }

func (s *ProjectMemberService) List(ctx context.Context, projectID, role string, pn, ps int) ([]*entity.ProjectMember, int64, error) {
	list, total, err := s.repo.List(ctx, projectID, role, pn, ps)
	if err != nil {
		return nil, 0, err
	}
	// 成员姓名回填（失败静默，展示回落 userId）
	ids := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, m := range list {
		if m.UserId != "" && !seen[m.UserId] {
			seen[m.UserId] = true
			ids = append(ids, m.UserId)
		}
	}
	names := userNameMap(ctx, s.userRepo, ids)
	for _, m := range list {
		if n, ok := names[m.UserId]; ok {
			m.UserName = n
		}
	}
	return list, total, nil
}

func (s *ProjectMemberService) ListByProject(ctx context.Context, projectID string) ([]*entity.ProjectMember, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *ProjectMemberService) Get(ctx context.Context, id string) (*entity.ProjectMember, error) {
	return s.repo.Get(ctx, id)
}

func (s *ProjectMemberService) Add(ctx context.Context, e *entity.ProjectMember) (string, error) {
	if e.ProjectId == "" || e.UserId == "" {
		return "", errors.New("projectId/userId 不能为空")
	}
	if e.Role == "" {
		e.Role = "member"
	}
	// 同项目同用户去重：已存在则改角色
	if got, err := s.repo.GetByProjectUser(ctx, e.ProjectId, e.UserId); err == nil && got != nil {
		got.Role = e.Role
		_ = s.repo.Edit(ctx, got)
		s.syncDocSpace(ctx, e.ProjectId)
		return got.MemberId, nil
	}
	mid, err := s.repo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	s.syncDocSpace(ctx, e.ProjectId)
	return mid, nil
}

func (s *ProjectMemberService) Edit(ctx context.Context, memberID, role string) error {
	if memberID == "" {
		return errors.New("memberId 不能为空")
	}
	got, err := s.repo.Get(ctx, memberID)
	if err != nil {
		return err
	}
	got.Role = role
	if err := s.repo.Edit(ctx, got); err != nil {
		return err
	}
	s.syncDocSpace(ctx, got.ProjectId)
	return nil
}

func (s *ProjectMemberService) Delete(ctx context.Context, ids []string) error {
	// 软删后反查不到项目，先收集 projectId 再删
	pids := make([]string, 0, len(ids))
	for _, id := range ids {
		if got, err := s.repo.Get(ctx, id); err == nil {
			pids = append(pids, got.ProjectId)
		}
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	for _, pid := range pids {
		s.syncDocSpace(ctx, pid)
	}
	return nil
}

// syncDocSpace 成员变化联动项目空间目录授权（nil=未接线静默跳过）
func (s *ProjectMemberService) syncDocSpace(ctx context.Context, projectID string) {
	if s.docSpace != nil {
		s.docSpace.SyncProjectMembers(ctx, projectID)
	}
	if s.kbSpace != nil {
		s.kbSpace.SyncProjectMembers(ctx, projectID)
	}
}
