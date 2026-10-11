package service

import (
	"context"
	"errors"
	"log/slog"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

type SkillLabelService struct {
	repo repository.ISkillLabelRepository
}

func NewSkillLabelService(r repository.ISkillLabelRepository) *SkillLabelService {
	return &SkillLabelService{repo: r}
}

func (s *SkillLabelService) List(ctx context.Context, slug, ltype string, pageNum, pageSize int) ([]*entity.SkillLabelDefinition, int64, error) {
	list, total, err := s.repo.List(ctx, slug, ltype, pageNum, pageSize)
	if err != nil {
		return list, total, err
	}
	// 「创建人」列：created_by 存的是 user_id，跨库回填 aa_user.name（对齐 LLMDeptProfile.LeaderMapByIds 写法）。
	ids := make(map[string]struct{}, len(list))
	for _, l := range list {
		if l.CreatedBy != "" {
			ids[l.CreatedBy] = struct{}{}
		}
	}
	if len(ids) > 0 {
		userIds := make([]string, 0, len(ids))
		for id := range ids {
			userIds = append(userIds, id)
		}
		nameMap, mErr := s.repo.CreatorMapByIds(ctx, userIds)
		if mErr != nil {
			slog.Error("skill label 创建人回填失败", "userIds", userIds, "err", mErr)
		} else {
			for _, l := range list {
				if l.CreatedBy != "" {
					l.CreatorName = nameMap[l.CreatedBy]
				}
			}
		}
	}
	return list, total, nil
}

func (s *SkillLabelService) All(ctx context.Context) ([]*entity.SkillLabelDefinition, error) {
	return s.repo.All(ctx)
}

func (s *SkillLabelService) Get(ctx context.Context, labelId string) (*entity.SkillLabelDefinition, error) {
	return s.repo.Get(ctx, labelId)
}

func (s *SkillLabelService) Add(ctx context.Context, l *entity.SkillLabelDefinition) (string, error) {
	if l.Slug == "" {
		return "", errors.New("标签标识不能为空")
	}
	if l.Type == "" {
		l.Type = "RECOMMENDED"
	}
	return s.repo.Add(ctx, l)
}

func (s *SkillLabelService) Edit(ctx context.Context, l *entity.SkillLabelDefinition) error {
	return s.repo.Edit(ctx, l)
}

func (s *SkillLabelService) Delete(ctx context.Context, labelId string) error {
	return s.repo.Delete(ctx, labelId)
}

func (s *SkillLabelService) ListTranslations(ctx context.Context, labelId string) ([]*entity.SkillLabelTranslation, error) {
	return s.repo.ListTranslations(ctx, labelId)
}

func (s *SkillLabelService) UpsertTranslation(ctx context.Context, t *entity.SkillLabelTranslation) error {
	if t.LabelId == "" || t.Locale == "" || t.DisplayName == "" {
		return errors.New("标签/语言/名称不能为空")
	}
	return s.repo.UpsertTranslation(ctx, t)
}
