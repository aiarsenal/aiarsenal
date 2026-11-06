package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type CalendarSyncController struct {
	svc *service.CalendarSyncService
}

func NewCalendarSyncController(svc *service.CalendarSyncService) *CalendarSyncController {
	return &CalendarSyncController{svc: svc}
}

func (h *CalendarSyncController) List(c *gin.Context) {
	list, err := h.svc.ListAccounts(c, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, "获取日历同步账号失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *CalendarSyncController) Discover(c *gin.Context) {
	var req dto.CalendarSyncDiscoverReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.Discover(c, req.ServerUrl, req.Username, req.Password)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *CalendarSyncController) Add(c *gin.Context) {
	var req dto.CalendarSyncAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id, err := h.svc.AddAccount(c, middleware.GetUserId(c), service.CalendarSyncAccountInput{
		Name: req.Name, ServerUrl: req.ServerUrl, CalendarUrl: req.CalendarUrl,
		Username: req.Username, Password: req.Password, Enabled: req.Enabled,
		SyncIntervalMinutes: req.SyncIntervalMinutes, ConflictPolicy: req.ConflictPolicy,
	})
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"accountId": id})
}

func (h *CalendarSyncController) Edit(c *gin.Context) {
	var req dto.CalendarSyncEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	err := h.svc.EditAccount(c, middleware.GetUserId(c), req.AccountId, service.CalendarSyncAccountInput{
		Name: req.Name, ServerUrl: req.ServerUrl, CalendarUrl: req.CalendarUrl,
		Username: req.Username, Password: req.Password, Enabled: req.Enabled,
		SyncIntervalMinutes: req.SyncIntervalMinutes, ConflictPolicy: req.ConflictPolicy,
	})
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *CalendarSyncController) Delete(c *gin.Context) {
	var req dto.CalendarSyncDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteAccount(c, middleware.GetUserId(c), req.AccountId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *CalendarSyncController) Run(c *gin.Context) {
	var req dto.CalendarSyncRunReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	result, err := h.svc.SyncAccount(c, middleware.GetUserId(c), req.AccountId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, result)
}
