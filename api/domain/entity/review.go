package entity

import (
	"aiarsenal/domain/types"
)

// ============ 统一审核工单 ============
// 全平台申请单汇聚（主库 aa_review_order，审核系统「我的提交/我的审核」唯一数据源）。
// order_type 区分审核类型（注册见 service.ReviewTypeHandler），payload 存类型专属快照 JSON，
// 金额走通用列 amount（毫）；审批通过的产物引用记 ResultRef（如 llm=新建临时预算 budget_id）。
type ReviewOrder struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	OrderId        string      `gorm:"column:order_id" json:"orderId"`
	OrderType      string      `gorm:"column:order_type" json:"orderType"` // llm_temp_budget（后续类型注册追加）
	Title          string      `gorm:"column:title" json:"title"`
	ApplicantId    string      `gorm:"column:applicant_id" json:"applicantId"`
	ApplicantName  string      `gorm:"column:applicant_name" json:"applicantName"`
	DeptId         string      `gorm:"column:dept_id" json:"deptId"` // 申请时部门快照（审批人过滤维度）
	Amount         types.Milli `gorm:"column:amount" json:"amount"`
	Payload        string      `gorm:"column:payload;type:text" json:"payload"` // 类型专属快照 JSON
	Status         string      `gorm:"column:status" json:"status"`             // pending/approved/rejected
	ReviewerId     string      `gorm:"column:reviewer_id" json:"reviewerId"`
	ReviewerName   string      `gorm:"column:reviewer_name" json:"reviewerName"`
	ReviewNote     string      `gorm:"column:review_note" json:"reviewNote"`
	ResultRef      string      `gorm:"column:result_ref" json:"resultRef"` // 审批产物引用（llm=granted budget_id）
	ReviewedAt     *types.Time `gorm:"column:reviewed_at" json:"reviewedAt"`
	CreatedBy      string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy      string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ReviewOrder) TableName() string { return "aa_review_order" }
