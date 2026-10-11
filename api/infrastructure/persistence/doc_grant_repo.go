package persistence

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/db"
)

type DocGrantRepository struct{ db *gorm.DB }

func NewDocGrantRepository() *DocGrantRepository { return &DocGrantRepository{db: db.DocDB()} }

func (r *DocGrantRepository) ListAll(ctx context.Context) ([]*entity.DocGrant, error) {
	var list []*entity.DocGrant
	err := r.db.WithContext(ctx).Order("id").Find(&list).Error
	return list, err
}

func (r *DocGrantRepository) ListByTarget(ctx context.Context, targetType, targetId string) ([]*entity.DocGrant, error) {
	var list []*entity.DocGrant
	err := r.db.WithContext(ctx).
		Where("target_type = ? and target_id = ?", targetType, targetId).
		Order("id").Find(&list).Error
	return list, err
}

// Replace 重写（目标,动作）条件行集：单事务先删后插。
// jsonb 列以 JSON 文本入参（PG 隐式转 jsonb）；行 id 重新生成。
func (r *DocGrantRepository) Replace(ctx context.Context, targetType, targetId, action string, rows []model.GrantRow, actor string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_type = ? and target_id = ? and action = ?", targetType, targetId, action).
			Delete(&entity.DocGrant{}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			g := &entity.DocGrant{
				TargetType: targetType,
				TargetId:   targetId,
				Action:     action,
				UserIds:    idsToJSON(row.UserIds),
				DeptId:     row.DeptId,
				RoleIds:    idsToJSON(row.RoleIds),
				PostIds:    idsToJSON(row.PostIds),
				Exclude:    row.Exclude,
				GroupId:    row.GroupId,
				SetId:      row.SetId,
				CreatedBy:  actor,
			}
			if err := tx.Create(g).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DocGrantRepository) DeleteByTargets(ctx context.Context, targetType string, targetIds []string) error {
	if len(targetIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("target_type = ? and target_id in ?", targetType, targetIds).
		Delete(&entity.DocGrant{}).Error
}

// idsToJSON []string → jsonb 文本（nil/空 → '[]'，与列默认一致）
func idsToJSON(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}
