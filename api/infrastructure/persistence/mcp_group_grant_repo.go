package persistence

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/db"
)

// MCP 分组组合授权条件行仓储（aa_mcp_group_grant；照 kb_grant_repo.go 镜像，单动作「可使用」裁剪：
// 无 target_type/action 维度，目标列 = target_id → aa_mcp_group.group_id）。

type McpGroupGrantRepository struct{ db *gorm.DB }

func NewMcpGroupGrantRepository() *McpGroupGrantRepository {
	return &McpGroupGrantRepository{db: db.McpDB()}
}

func (r *McpGroupGrantRepository) ListAll(ctx context.Context) (list []*entity.McpGroupGrant, err error) {
	err = r.db.WithContext(ctx).Order("id").Find(&list).Error
	return
}

func (r *McpGroupGrantRepository) ListByGroup(ctx context.Context, groupId string) (list []*entity.McpGroupGrant, err error) {
	err = r.db.WithContext(ctx).Where("target_id = ?", groupId).Order("id").Find(&list).Error
	return
}

// Replace 重写分组条件行集：单事务先删后插。
// jsonb 列以 JSON 文本入参（PG 隐式转 jsonb）；行 id 重新生成，created_at 落 DDL DEFAULT NOW()。
func (r *McpGroupGrantRepository) Replace(ctx context.Context, groupId string, rows []model.GrantRow, actor string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("target_id = ?", groupId).Delete(&entity.McpGroupGrant{}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			g := &entity.McpGroupGrant{
				TargetId:  groupId,
				UserIds:   mcpIDsToJSON(row.UserIds),
				DeptId:    row.DeptId,
				RoleIds:   mcpIDsToJSON(row.RoleIds),
				PostIds:   mcpIDsToJSON(row.PostIds),
				Exclude:   row.Exclude,
				GroupId:   row.GroupId,
				SetId:     row.SetId,
				CreatedBy: actor,
			}
			if err := tx.Create(g).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *McpGroupGrantRepository) DeleteByGroups(ctx context.Context, groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("target_id in ?", groupIds).Delete(&entity.McpGroupGrant{}).Error
}

// mcpIDsToJSON []string → jsonb 文本（nil/空 → '[]'，与列默认一致）
func mcpIDsToJSON(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}
