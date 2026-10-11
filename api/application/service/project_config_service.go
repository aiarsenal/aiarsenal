package service

import (
	"context"
	"errors"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// 设置服务：标签 / 状态流 / 自定义字段 三类项目级配置 CRUD。
type ProjectConfigService struct {
	tagRepo    repository.IProjectTagRepository
	statusRepo repository.IProjectStatusRepository
	fieldRepo  repository.IProjectCustomFieldRepository
}

func NewProjectConfigService(tr repository.IProjectTagRepository, sr repository.IProjectStatusRepository, fr repository.IProjectCustomFieldRepository) *ProjectConfigService {
	return &ProjectConfigService{tagRepo: tr, statusRepo: sr, fieldRepo: fr}
}

// ===== 标签 =====
func (s *ProjectConfigService) ListTags(ctx context.Context, projectID string, pn, ps int) ([]*entity.ProjectTag, int64, error) {
	return s.tagRepo.List(ctx, projectID, pn, ps)
}
func (s *ProjectConfigService) ListTagsByProject(ctx context.Context, projectID string) ([]*entity.ProjectTag, error) {
	return s.tagRepo.ListByProject(ctx, projectID)
}
func (s *ProjectConfigService) AddTag(ctx context.Context, e *entity.ProjectTag) (string, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" || e.ProjectId == "" {
		return "", errors.New("projectId/name 不能为空")
	}
	return s.tagRepo.Add(ctx, e)
}
func (s *ProjectConfigService) EditTag(ctx context.Context, e *entity.ProjectTag) error {
	if e.TagId == "" {
		return errors.New("tagId 不能为空")
	}
	return s.tagRepo.Edit(ctx, e)
}
func (s *ProjectConfigService) DeleteTags(ctx context.Context, ids []string) error {
	return s.tagRepo.Delete(ctx, ids)
}

// ===== 状态流 =====
func (s *ProjectConfigService) ListStatus(ctx context.Context, projectID string, pn, ps int) ([]*entity.ProjectStatus, int64, error) {
	return s.statusRepo.List(ctx, projectID, pn, ps)
}
func (s *ProjectConfigService) ListStatusByProject(ctx context.Context, projectID string) ([]*entity.ProjectStatus, error) {
	return s.statusRepo.ListByProject(ctx, projectID)
}
func (s *ProjectConfigService) AddStatus(ctx context.Context, e *entity.ProjectStatus) (string, error) {
	e.Key = strings.TrimSpace(e.Key)
	e.Name = strings.TrimSpace(e.Name)
	if e.Key == "" || e.Name == "" || e.ProjectId == "" {
		return "", errors.New("projectId/key/name 不能为空")
	}
	if e.Category == "" {
		e.Category = "todo"
	}
	return s.statusRepo.Add(ctx, e)
}
func (s *ProjectConfigService) EditStatus(ctx context.Context, e *entity.ProjectStatus) error {
	if e.StatusId == "" {
		return errors.New("statusId 不能为空")
	}
	return s.statusRepo.Edit(ctx, e)
}
func (s *ProjectConfigService) DeleteStatus(ctx context.Context, ids []string) error {
	return s.statusRepo.Delete(ctx, ids)
}

// ===== 自定义字段 =====
func (s *ProjectConfigService) ListFields(ctx context.Context, projectID string, pn, ps int) ([]*entity.ProjectCustomField, int64, error) {
	return s.fieldRepo.List(ctx, projectID, pn, ps)
}
func (s *ProjectConfigService) ListFieldsByProject(ctx context.Context, projectID string) ([]*entity.ProjectCustomField, error) {
	return s.fieldRepo.ListByProject(ctx, projectID)
}
func (s *ProjectConfigService) AddField(ctx context.Context, e *entity.ProjectCustomField) (string, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" || e.ProjectId == "" || e.Type == "" {
		return "", errors.New("projectId/name/type 不能为空")
	}
	return s.fieldRepo.Add(ctx, e)
}
func (s *ProjectConfigService) EditField(ctx context.Context, e *entity.ProjectCustomField) error {
	if e.FieldId == "" {
		return errors.New("fieldId 不能为空")
	}
	return s.fieldRepo.Edit(ctx, e)
}
func (s *ProjectConfigService) DeleteFields(ctx context.Context, ids []string) error {
	return s.fieldRepo.Delete(ctx, ids)
}
