package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type NoticeController struct{ svc *service.NoticeService }

func NewNoticeController(s *service.NoticeService) *NoticeController {
	return &NoticeController{svc: s}
}

// List 当前用户的个人消息（分页）
func (h *NoticeController) List(c *gin.Context) {
	var req dto.NoticeListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	list, total, err := h.svc.List(c, userId, req.IsRead, req.Type, req.StartTime, req.EndTime, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.NoticeListRes{NoticeList: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// UnreadCount 未读数（供顶栏红点）
func (h *NoticeController) UnreadCount(c *gin.Context) {
	userId := middleware.GetUserId(c)
	count, err := h.svc.UnreadCount(c, userId)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, map[string]int64{"unreadCount": count})
}

// MarkRead 标记单条已读
func (h *NoticeController) MarkRead(c *gin.Context) {
	var req dto.NoticeReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	if err := h.svc.MarkRead(c, req.NoticeId, userId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// MarkAllRead 全部已读
func (h *NoticeController) MarkAllRead(c *gin.Context) {
	userId := middleware.GetUserId(c)
	if err := h.svc.MarkAllRead(c, userId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Delete 删除（仅本人消息）
func (h *NoticeController) Delete(c *gin.Context) {
	var req dto.NoticeDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	userId := middleware.GetUserId(c)
	if err := h.svc.Delete(c, req.Ids, userId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
