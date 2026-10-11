package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 任务控制器（任务/子任务 + 评论）。

type ProjectTaskController struct {
	svc *service.ProjectTaskService
}

func NewProjectTaskController(s *service.ProjectTaskService) *ProjectTaskController {
	return &ProjectTaskController{svc: s}
}

func (h *ProjectTaskController) List(c *gin.Context) {
	var req dto.ProjectTaskSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ProjectId, req.Status, req.Assignee, req.Priority, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectTaskSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *ProjectTaskController) Get(c *gin.Context) {
	id := c.Query("taskId")
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

func (h *ProjectTaskController) Add(c *gin.Context) {
	var req dto.ProjectTaskAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectTaskFromReq(req.ProjectTaskReq)
	if _, err := h.svc.Add(c, e, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectTaskController) Edit(c *gin.Context) {
	var req dto.ProjectTaskEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectTaskFromReq(req.ProjectTaskReq)
	e.TaskId = req.TaskId
	if err := h.svc.Edit(c, e, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectTaskController) Delete(c *gin.Context) {
	var req dto.ProjectIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	pid := c.Query("projectId")
	if err := h.svc.Delete(c, req.Ids, middleware.GetUserId(c), pid); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 评论 =====

func (h *ProjectTaskController) ListComments(c *gin.Context) {
	tid := c.Query("taskId")
	if tid == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.ListComments(c, tid)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.ProjectTaskCommentListRes{List: list})
}

func (h *ProjectTaskController) AddComment(c *gin.Context) {
	var req dto.ProjectTaskCommentAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.ProjectTaskComment{
		TaskId: req.TaskId, ProjectId: req.ProjectId, Content: req.Content,
		ParentCommentId: req.ParentCommentId, Mentioned: req.Mentioned,
	}
	if _, err := h.svc.AddComment(c, e, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectTaskController) DeleteComment(c *gin.Context) {
	var req dto.ProjectIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteComment(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func projectTaskFromReq(r *dto.ProjectTaskReq) *entity.ProjectTask {
	return &entity.ProjectTask{
		ProjectId: r.ProjectId, ParentTaskId: r.ParentTaskId, MilestoneId: r.MilestoneId,
		Title: r.Title, Description: r.Description, Status: r.Status, Priority: r.Priority,
		AssigneeId: r.AssigneeId, ReporterId: r.ReporterId, Tags: r.Tags, CustomFields: r.CustomFields,
		EstimateHours: r.EstimateHours, SpentHours: r.SpentHours,
		StartDate: parseTimePtr(r.StartDate), DueDate: parseTimePtr(r.DueDate),
	}
}
