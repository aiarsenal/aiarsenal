package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type LoginLogController struct{ svc *service.LoginLogService }

func NewLoginLogController(s *service.LoginLogService) *LoginLogController {
	return &LoginLogController{svc: s}
}

func (h *LoginLogController) List(c *gin.Context) {
	var req dto.LoginLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.LoginName, req.Status, req.Ipaddr, req.LoginLocation, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LoginLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LoginLogController) Delete(c *gin.Context) {
	var req dto.LoginLogDelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *LoginLogController) Clear(c *gin.Context) {
	if err := h.svc.Clear(c); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

type OperLogController struct{ svc *service.OperLogService }

func NewOperLogController(s *service.OperLogService) *OperLogController {
	return &OperLogController{svc: s}
}

func (h *OperLogController) List(c *gin.Context) {
	var req dto.OperLogSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Title, req.RequestMethod, req.OperName, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.OperLogSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *OperLogController) Get(c *gin.Context) {
	var req dto.OperLogGetReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	info, err := h.svc.Get(c, req.OperId)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, info)
}

func (h *OperLogController) Delete(c *gin.Context) {
	var req dto.OperLogDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.OperIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *OperLogController) Clear(c *gin.Context) {
	if err := h.svc.Clear(c); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

type UserOnlineController struct{ svc *service.UserOnlineService }

func NewUserOnlineController(s *service.UserOnlineService) *UserOnlineController {
	return &UserOnlineController{svc: s}
}

func (h *UserOnlineController) List(c *gin.Context) {
	var req dto.UserOnlineSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Username, req.Ip, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.UserOnlineSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *UserOnlineController) Delete(c *gin.Context) {
	var req dto.UserOnlineDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Uuid); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Kick 吊销某用户的全部会话  DELETE /api/v1/system/online/kick?userId=
// 与 Delete 的区别：Delete 只删在线表展示行，令牌仍有效；Kick 让令牌立即失效。
func (h *UserOnlineController) Kick(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	if err := h.svc.Kick(c, middleware.GetUserId(c), userId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
