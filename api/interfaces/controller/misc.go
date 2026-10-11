package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type MonitorController struct{ svc *service.MonitorService }

func NewMonitorController(s *service.MonitorService) *MonitorController {
	return &MonitorController{svc: s}
}

func (h *MonitorController) Server(c *gin.Context) {
	info, err := h.svc.Server()
	if err != nil {
		middleware.Fail(c, "获取监控信息失败")
		return
	}
	middleware.Success(c, info)
}

type CacheController struct{ svc *service.CacheService }

func NewCacheController(s *service.CacheService) *CacheController { return &CacheController{svc: s} }

func (h *CacheController) Remove(c *gin.Context) {
	if err := h.svc.Remove(c); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

type PersonalController struct{ svc *service.PersonalService }

func NewPersonalController(s *service.PersonalService) *PersonalController {
	return &PersonalController{svc: s}
}

func (h *PersonalController) GetPersonalInfo(c *gin.Context) {
	uid := middleware.GetUserId(c)
	user, roles, deptName, err := h.svc.Info(c, uid)
	if err != nil {
		middleware.Fail(c, "获取用户信息失败")
		return
	}
	middleware.Success(c, dto.PersonalInfoRes{User: user, Roles: roles, DeptName: deptName})
}

func (h *PersonalController) Edit(c *gin.Context) {
	var req dto.PersonalEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	uid := middleware.GetUserId(c)
	p := &entity.User{
		UserNickname: req.Nickname, Mobile: req.Mobile, Remark: req.Remark,
		Sex: req.Sex, UserEmail: req.UserEmail, Describe: req.Describe, Avatar: req.Avatar,
	}
	user, token, err := h.svc.Edit(c, uid, p)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.PersonalEditRes{UserInfo: user, Token: token})
}

func (h *PersonalController) ResetPwd(c *gin.Context) {
	var req dto.PersonalResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	uid := middleware.GetUserId(c)
	if err := h.svc.ResetPwd(c, uid, req.Password); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
