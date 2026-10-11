package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type DeptController struct{ svc *service.DeptService }

func NewDeptController(s *service.DeptService) *DeptController { return &DeptController{svc: s} }

func (h *DeptController) List(c *gin.Context) {
	var req dto.DeptSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.List(c, req.DeptName, req.Status)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.DeptSearchRes{DeptList: list})
}

func (h *DeptController) TreeSelect(c *gin.Context) {
	nodes, err := h.svc.Tree(c, "", "")
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.DeptTreeSelectRes{Deps: nodes})
}

func (h *DeptController) Add(c *gin.Context) {
	var req dto.DeptAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	d := &entity.Dept{
		ParentId: req.ParentID, DeptName: req.DeptName, OrderNum: req.OrderNum,
		LeaderId: req.LeaderId, Status: req.Status,
		CreatedBy: middleware.GetUserId(c),
	}
	if err := h.svc.Add(c, d); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *DeptController) Edit(c *gin.Context) {
	var req dto.DeptEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	d := &entity.Dept{
		DeptId: req.DeptId, ParentId: req.ParentID, DeptName: req.DeptName,
		OrderNum: req.OrderNum, LeaderId: req.LeaderId, Status: req.Status,
	}
	if err := h.svc.Edit(c, d); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *DeptController) Delete(c *gin.Context) {
	var req dto.DeptDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
