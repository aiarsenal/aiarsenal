package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// ===================== ReviewOrder（统一审核工单，主库） =====================

// ErrOrderClosed 工单已被处理（通过/驳回）——审批并发双击/双通道同时提交的防护哨兵
// （承接原 ErrRequestClosed 语义，mapBudgetReqErr 同款文案）。
var ErrOrderClosed = errors.New("该申请已被处理")

type ReviewOrderRepository struct{ db *gorm.DB }

func NewReviewOrderRepository() *ReviewOrderRepository {
	return &ReviewOrderRepository{db: db.DB()}
}

func (r *ReviewOrderRepository) Add(ctx context.Context, e *entity.ReviewOrder) (string, error) {
	e.OrderId = idgen.New()
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增审核工单失败", "orderId", e.OrderId, "type", e.OrderType, "applicant", e.ApplicantId, "err", err)
		return "", err
	}
	slog.Info("新增审核工单", "orderId", e.OrderId, "type", e.OrderType, "applicant", e.ApplicantId, "amount", e.Amount)
	return e.OrderId, nil
}

func (r *ReviewOrderRepository) Get(ctx context.Context, orderId string) (*entity.ReviewOrder, error) {
	var e entity.ReviewOrder
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ReviewOrderRepository) ListMine(ctx context.Context, applicantId, status string, pageNum, pageSize int) (list []*entity.ReviewOrder, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ReviewOrder{}).Where("applicant_id = ?", applicantId)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ReviewOrderRepository) ListReview(ctx context.Context, requesterId, deptId string, isAdmin bool, status string, pageNum, pageSize int) (list []*entity.ReviewOrder, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ReviewOrder{}).Where("applicant_id <> ?", requesterId)
	if !isAdmin {
		q = q.Where("dept_id = ?", deptId)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// Close 关单事务：行锁取工单→校验 pending→回写审批结果。approve/reject 共用（status 区分）。
// 并发双批防护：第二个事务等行锁后看到非 pending，返回 ErrOrderClosed。
// 工单状态与类型业务动作（如建临时预算）跨库无法同事务：先关单防重复审批，业务动作失败由
// Reopen 补偿回 pending（见 ReviewOrderService.Approve）。
func (r *ReviewOrderRepository) Close(ctx context.Context, orderId, status, reviewerId, reviewerName, note string) (*entity.ReviewOrder, error) {
	var out *entity.ReviewOrder
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o entity.ReviewOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_id = ?", orderId).First(&o).Error; err != nil {
			return err
		}
		if o.Status != "pending" {
			return ErrOrderClosed
		}
		if err := tx.Model(&entity.ReviewOrder{}).Where("order_id = ?", orderId).Updates(map[string]interface{}{
			"status": status, "reviewer_id": reviewerId, "reviewer_name": reviewerName,
			"review_note": note, "reviewed_at": time.Now(), "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		out = &o
		out.Status = status
		out.ReviewerId, out.ReviewerName, out.ReviewNote = reviewerId, reviewerName, note
		return nil
	})
	if err == nil {
		slog.Info("审核工单关单", "orderId", orderId, "type", out.OrderType, "status", status, "reviewer", reviewerId)
	}
	return out, err
}

// SetResultRef 回写审批产物引用（best-effort：失败仅留空，不回滚已执行的业务动作）。
func (r *ReviewOrderRepository) SetResultRef(ctx context.Context, orderId, ref string) error {
	return r.db.WithContext(ctx).Model(&entity.ReviewOrder{}).Where("order_id = ?", orderId).
		Updates(map[string]interface{}{"result_ref": ref, "updated_at": time.Now()}).Error
}

// Reopen 补偿回 pending：关单成功但类型 Execute（跨库业务动作）失败时回滚工单状态，
// 让申请人/审批人可重试（reviewer 信息一并清空）。
func (r *ReviewOrderRepository) Reopen(ctx context.Context, orderId string) error {
	return r.db.WithContext(ctx).Model(&entity.ReviewOrder{}).Where("order_id = ?", orderId).
		Updates(map[string]interface{}{
			"status": "pending", "reviewer_id": "", "reviewer_name": "",
			"review_note": "", "result_ref": "", "reviewed_at": nil, "updated_at": time.Now(),
		}).Error
}

// MigrateLegacyBudgetRequests 历史临时预算申请一次性搬运：aa_llm 归档表 → 主库统一审核工单。
// 幂等：order_id=原 request_id + ON CONFLICT DO NOTHING，api 重启重跑安全；
// 归档表不存在（全新安装库）或为空直接跳过。只在 api 进程调用（mcp-ops 不跑，避免双进程竞争）。
func MigrateLegacyBudgetRequests() (int, error) {
	// 归档表不存在 → 全新安装，无历史可搬
	var exists bool
	if err := db.LLMDB().Raw(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = current_schema() AND table_name = 'aa_llm_budget_request_archive')`,
	).Scan(&exists).Error; err != nil || !exists {
		return 0, err
	}
	rows, err := db.LLMDB().Raw(
		`SELECT request_id, base_budget_id, base_budget_name, applicant_id, applicant_name, dept_id,
		        amount, reason, budget_duration, status, reviewer_id, reviewer_name, review_note,
		        granted_budget_id, reviewed_at, created_by, updated_by, created_at, updated_at
		 FROM aa_llm_budget_request_archive ORDER BY id`).Rows()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	main := db.DB()
	moved := 0
	for rows.Next() {
		// 本地行结构（旧实体已删，搬运自包含）
		var (
			requestId, baseId, baseName, applicantId, applicantName, deptId string
			amount                                                          int64
			reason, duration, status, reviewerId, reviewerName, note, granted string
			reviewedAt, createdAt, updAt                                    sql.NullTime
			createdBy, updatedBy                                            string
		)
		if err := rows.Scan(&requestId, &baseId, &baseName, &applicantId, &applicantName, &deptId,
			&amount, &reason, &duration, &status, &reviewerId, &reviewerName, &note, &granted,
			&reviewedAt, &createdBy, &updatedBy, &createdAt, &updAt); err != nil {
			return moved, fmt.Errorf("scan legacy budget request: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{
			"baseBudgetId": baseId, "baseBudgetName": baseName, "duration": duration, "reason": reason,
		})
		// NULL 时间保空（reviewed_at 仅已审单有值；created/updated 兜底 now）
		var reviewedAtArg, createdAtArg, updAtArg any
		if reviewedAt.Valid {
			reviewedAtArg = reviewedAt.Time
		}
		if !createdAt.Valid {
			createdAt.Time, createdAt.Valid = time.Now(), true
		}
		if !updAt.Valid {
			updAt.Time, updAt.Valid = createdAt.Time, true
		}
		createdAtArg, updAtArg = createdAt.Time, updAt.Time
		res := main.Exec(
			`INSERT INTO aa_review_order (order_id, order_type, title, applicant_id, applicant_name, dept_id,
			       amount, payload, status, reviewer_id, reviewer_name, review_note, result_ref,
			       reviewed_at, created_by, updated_by, created_at, updated_at)
			 VALUES (?, 'llm_temp_budget', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (order_id) DO NOTHING`,
			requestId, "临时预算申请·基于「"+baseName+"」", applicantId, applicantName, deptId,
			amount, string(payload), status, reviewerId, reviewerName, note, granted,
			reviewedAtArg, createdBy, updatedBy, createdAtArg, updAtArg)
		if res.Error != nil {
			return moved, fmt.Errorf("migrate legacy request %s: %w", requestId, res.Error)
		}
		if res.RowsAffected > 0 {
			moved++
		}
	}
	if moved > 0 {
		slog.Info("历史临时预算申请已搬运至统一审核工单", "count", moved)
	}
	return moved, rows.Err()
}
