package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 统一审核工单控制器：提交 / 我的提交 / 待我审列表 / 审批（通过|驳回）。
// 自助端点不建 aa_menu/aa_rule 行（casbin 对无菜单路径放行，先例 /system/notice/*），
// 真实授权在 service：提交=本人，审批=部门负责人或超管。

type ReviewOrderController struct {
	svc *service.ReviewOrderService
}

func NewReviewOrderController(s *service.ReviewOrderService) *ReviewOrderController {
	return &ReviewOrderController{svc: s}
}

func (h *ReviewOrderController) Create(c *gin.Context) {
	var req dto.ReviewOrderCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e, err := h.svc.Create(c, middleware.GetIdentity(c), req.Type, req.Params)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.NewReviewOrderItem(e))
}

// Mine 我的提交（全类型；status 空=全部）。
func (h *ReviewOrderController) Mine(c *gin.Context) {
	var req dto.ReviewOrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListMine(c, middleware.GetIdentity(c), req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.ReviewOrderListRes{
		ListRes: dto.ListRes{CurrentPage: req.PageNum, Total: total},
		List:    dto.NewReviewOrderItems(list),
	})
}

// ReviewList 待我审批（超管=全部；否则=本部门；status 空默认 pending）。
func (h *ReviewOrderController) ReviewList(c *gin.Context) {
	var req dto.ReviewOrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListReview(c, middleware.GetIdentity(c), req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.ReviewOrderListRes{
		ListRes: dto.ListRes{CurrentPage: req.PageNum, Total: total},
		List:    dto.NewReviewOrderItems(list),
	})
}

func (h *ReviewOrderController) Approve(c *gin.Context) {
	h.review(c, true)
}

func (h *ReviewOrderController) Reject(c *gin.Context) {
	h.review(c, false)
}

func (h *ReviewOrderController) review(c *gin.Context, approve bool) {
	var req dto.ReviewOrderReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	var (
		o   *entity.ReviewOrder
		err error
	)
	if approve {
		o, err = h.svc.Approve(c, middleware.GetIdentity(c), req.OrderId, req.Note)
	} else {
		o, err = h.svc.Reject(c, middleware.GetIdentity(c), req.OrderId, req.Note)
	}
	if err != nil {
		// ErrOrderClosed（已被他人处理）等业务错误原样透出给前端提示。
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.NewReviewOrderItem(o))
}
