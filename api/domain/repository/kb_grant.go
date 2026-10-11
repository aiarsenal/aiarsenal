package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// IKbGrantRepository 文档/目录组合授权条件行仓储（aa_kb 库 aa_kb_grant 表）。
// 行语义同 model.GrantRow：非空维度全 AND，多行 OR，exclude 取反；
// action read|write（写授予蕴含读由判定层负责，本仓储只存取）。
type IKbGrantRepository interface {
	// ListAll 全量条件行（表小；权限视图一次载入内存判定）
	ListAll(ctx context.Context) ([]*entity.KbGrant, error)
	// ListByTarget 某目标的全部条件行（两种 action 混合，按 Action 字段区分）
	ListByTarget(ctx context.Context, targetType, targetId string) ([]*entity.KbGrant, error)
	// Replace 重写某目标某动作的条件行集（先删后插；rows 为空 = 清空该动作授权）
	Replace(ctx context.Context, targetType, targetId, action string, rows []model.GrantRow, actor string) error
	// DeleteByTargets 批量清理目标的条件行（文档/目录删除时随清）
	DeleteByTargets(ctx context.Context, targetType string, targetIds []string) error
}
