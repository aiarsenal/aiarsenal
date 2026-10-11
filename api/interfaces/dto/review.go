package dto

import (
	"encoding/json"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// ===== 统一审核工单（/api/v1/review/order；审核系统「我的提交/我的审核」） =====

// ReviewOrderCreateReq 提交申请：type 区分审核类型（service.ReviewTypeHandler 注册表），
// params 为类型专属原始参数（llm_temp_budget：baseBudgetId?/amount/reason/duration?），handler 内校验。
type ReviewOrderCreateReq struct {
	Type   string         `json:"type" form:"type" binding:"required"`
	Params map[string]any `json:"params" form:"params"`
}
type ReviewOrderReviewReq struct {
	OrderId string `json:"orderId" form:"orderId" binding:"required"`
	Note    string `json:"note" form:"note" binding:"max=500"`
}
type ReviewOrderListReq struct {
	Status string `json:"status" form:"status"` // mine：空=全部；review：空=默认 pending
	PageReq
}

// ReviewOrderItem 工单视图：payload 透出为 JSON 对象（前端按 orderType 渲染详情块）。
type ReviewOrderItem struct {
	OrderId       string          `json:"orderId"`
	OrderType     string          `json:"orderType"`
	Title         string          `json:"title"`
	ApplicantId   string          `json:"applicantId"`
	ApplicantName string          `json:"applicantName"`
	DeptId        string          `json:"deptId"`
	Amount        types.Milli     `json:"amount"` // JSON 边界为主币浮点（毫存储）
	Payload       json.RawMessage `json:"payload"`
	Status        string          `json:"status"` // pending/approved/rejected
	ReviewerId    string          `json:"reviewerId"`
	ReviewerName  string          `json:"reviewerName"`
	ReviewNote    string          `json:"reviewNote"`
	ResultRef     string          `json:"resultRef"` // 审批产物引用（llm=granted budget_id）
	ReviewedAt    *types.Time     `json:"reviewedAt"`
	CreatedAt     *types.Time     `json:"createdAt"`
}

func NewReviewOrderItem(e *entity.ReviewOrder) ReviewOrderItem {
	payload := json.RawMessage(e.Payload)
	if !json.Valid(payload) {
		payload = json.RawMessage("{}")
	}
	return ReviewOrderItem{
		OrderId: e.OrderId, OrderType: e.OrderType, Title: e.Title,
		ApplicantId: e.ApplicantId, ApplicantName: e.ApplicantName, DeptId: e.DeptId,
		Amount: e.Amount, Payload: payload, Status: e.Status,
		ReviewerId: e.ReviewerId, ReviewerName: e.ReviewerName, ReviewNote: e.ReviewNote,
		ResultRef: e.ResultRef, ReviewedAt: e.ReviewedAt, CreatedAt: e.CreatedAt,
	}
}

func NewReviewOrderItems(list []*entity.ReviewOrder) []ReviewOrderItem {
	items := make([]ReviewOrderItem, 0, len(list))
	for _, e := range list {
		items = append(items, NewReviewOrderItem(e))
	}
	return items
}

type ReviewOrderListRes struct {
	ListRes
	List []ReviewOrderItem `json:"list"`
}
