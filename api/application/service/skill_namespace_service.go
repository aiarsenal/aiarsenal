package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

type SkillNamespaceService struct {
	repo repository.ISkillNamespaceRepository
}

func NewSkillNamespaceService(r repository.ISkillNamespaceRepository) *SkillNamespaceService {
	return &SkillNamespaceService{repo: r}
}

func (s *SkillNamespaceService) List(ctx context.Context, slug, name, nsType, status string, pageNum, pageSize int) ([]*entity.SkillNamespace, int64, error) {
	return s.repo.List(ctx, slug, name, nsType, status, pageNum, pageSize)
}

func (s *SkillNamespaceService) All(ctx context.Context) ([]*entity.SkillNamespace, error) {
	return s.repo.All(ctx)
}

// Get 命名空间详情（F-017 横向越权修复）：创建者 / 命名空间成员 / 超管可读，
// 其余一律「未找到」（不泄露存在性）。userId 为空（系统路径）按不可读处理——
// 唯一调用方是鉴权后的 controller。
func (s *SkillNamespaceService) Get(ctx context.Context, namespaceId, callerId string, callerIsAdmin bool) (*entity.SkillNamespace, error) {
	ns, err := s.repo.Get(ctx, namespaceId)
	if err != nil {
		return nil, err
	}
	isMember := false
	if callerId != "" {
		for _, m := range func() []*entity.SkillNamespaceMember {
			ms, _ := s.repo.ListMembers(ctx, namespaceId)
			return ms
		}() {
			if m.UserId == callerId {
				isMember = true
				break
			}
		}
	}
	if !canReadNamespace(ns.CreatedBy, callerId, callerIsAdmin, isMember) {
		return nil, errDetailNotFound
	}
	return ns, nil
}

func (s *SkillNamespaceService) Add(ctx context.Context, n *entity.SkillNamespace) (string, error) {
	if n.Slug == "" {
		return "", errors.New("命名空间标识不能为空")
	}
	if existed, _ := s.repo.GetBySlug(ctx, n.Slug); existed != nil {
		return "", errors.New("命名空间标识已存在")
	}
	if n.Type == "" {
		n.Type = "TEAM"
	}
	return s.repo.Add(ctx, n)
}

func (s *SkillNamespaceService) Edit(ctx context.Context, n *entity.SkillNamespace) error {
	return s.repo.Edit(ctx, n)
}

func (s *SkillNamespaceService) Delete(ctx context.Context, namespaceId string) error {
	ns, err := s.repo.Get(ctx, namespaceId)
	if err != nil {
		return err
	}
	if ns.Type == "GLOBAL" {
		return errors.New("全局命名空间不可删除")
	}
	return s.repo.Delete(ctx, namespaceId)
}

func (s *SkillNamespaceService) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	return s.repo.ListMembers(ctx, namespaceId)
}

func (s *SkillNamespaceService) AddMember(ctx context.Context, m *entity.SkillNamespaceMember) error {
	if m.Role == "" {
		m.Role = "MEMBER"
	}
	return s.repo.AddMember(ctx, m)
}

func (s *SkillNamespaceService) RemoveMember(ctx context.Context, memberId string) error {
	return s.repo.RemoveMember(ctx, memberId)
}
