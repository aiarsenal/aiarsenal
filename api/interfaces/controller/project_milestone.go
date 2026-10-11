package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 里程碑控制器。

type ProjectMilestoneController struct {
	svc *service.ProjectMilestoneService
}

func NewProjectMilestoneController(s *service.ProjectMilestoneService) *ProjectMilestoneController {
	return &ProjectMilestoneController{svc: s}
}

func (h *ProjectMilestoneController) List(c *gin.Context) {
	var req dto.ProjectMilestoneSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ProjectId, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectMilestoneSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *ProjectMilestoneController) Get(c *gin.Context) {
	id := c.Query("milestoneId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *ProjectMilestoneController) Add(c *gin.Context) {
	var req dto.ProjectMilestoneAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectMilestoneFromReq(req.ProjectMilestoneReq)
	e.CreatedBy = middleware.GetUserId(c)
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectMilestoneController) Edit(c *gin.Context) {
	var req dto.ProjectMilestoneEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectMilestoneFromReq(req.ProjectMilestoneReq)
	e.MilestoneId = req.MilestoneId
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectMilestoneController) Delete(c *gin.Context) {
	var req dto.ProjectIdsReq
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

func projectMilestoneFromReq(r *dto.ProjectMilestoneReq) *entity.ProjectMilestone {
	return &entity.ProjectMilestone{
		ProjectId: r.ProjectId, Name: r.Name, Description: r.Description, Status: r.Status, SortOrder: r.SortOrder,
		StartDate: parseTimePtr(r.StartDate), DueDate: parseTimePtr(r.DueDate),
	}
}
