package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// IReviewOrderRepository 统一审核工单（主库；「我的提交/我的审核」唯一数据源）。
type IReviewOrderRepository interface {
	Add(ctx context.Context, e *entity.ReviewOrder) (string, error)
	Get(ctx context.Context, orderId string) (*entity.ReviewOrder, error)
	ListMine(ctx context.Context, applicantId, status string, pageNum, pageSize int) (list []*entity.ReviewOrder, total int64, err error)
	// ListReview 审批视角：isAdmin=全部；否则=申请人部门快照 dept_id。均排除 requester 自己的申请。
	ListReview(ctx context.Context, requesterId, deptId string, isAdmin bool, status string, pageNum, pageSize int) (list []*entity.ReviewOrder, total int64, err error)
	// Close 关单事务：行锁取工单→校验 pending→回写 status/reviewer/note/reviewed_at。
	// approve/reject 共用（status 入参区分），并发双批防护靠 pending 哨兵（ErrOrderClosed）。
	Close(ctx context.Context, orderId, status, reviewerId, reviewerName, note string) (*entity.ReviewOrder, error)
	// SetResultRef 回写审批产物引用（Execute 成功后 best-effort，失败仅留空不影响流转）。
	SetResultRef(ctx context.Context, orderId, ref string) error
	// Reopen 补偿回 pending：关单成功但类型 Execute（跨库业务动作）失败时回滚工单状态。
	Reopen(ctx context.Context, orderId string) error
}
