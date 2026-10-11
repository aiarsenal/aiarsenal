package controller

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 项目管理 — 项目控制器（项目 CRUD + 概览 + 选择器）。

type ProjectController struct {
	svc *service.ProjectService
}

func NewProjectController(s *service.ProjectService) *ProjectController {
	return &ProjectController{svc: s}
}

func (h *ProjectController) List(c *gin.Context) {
	var req dto.ProjectSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Name, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.ProjectSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *ProjectController) All(c *gin.Context) {
	list, err := h.svc.All(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, list)
}

func (h *ProjectController) Get(c *gin.Context) {
	id := c.Query("projectId")
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

func (h *ProjectController) Overview(c *gin.Context) {
	id := c.Query("projectId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Overview(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *ProjectController) Add(c *gin.Context) {
	var req dto.ProjectAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectFromReq(req.ProjectReq)
	e.Owner = middleware.GetUserId(c)
	e.CreatedBy = middleware.GetUserId(c)
	e.UpdatedBy = middleware.GetUserId(c)
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectController) Edit(c *gin.Context) {
	var req dto.ProjectEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := projectFromReq(req.ProjectReq)
	e.ProjectId = req.ProjectId
	e.UpdatedBy = middleware.GetUserId(c)
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *ProjectController) Delete(c *gin.Context) {
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

// projectFromReq DTO→实体；tagList 便捷数组优先于 tags 字符串。
func projectFromReq(r *dto.ProjectReq) *entity.Project {
	tags := r.Tags
	if len(r.TagList) > 0 {
		tags = stringJSONArr(r.TagList)
	}
	return &entity.Project{
		Name: r.Name, Code: r.Code, Description: r.Description, Status: r.Status, Priority: r.Priority,
		Progress: r.Progress, Visibility: r.Visibility, DeptId: r.DeptId, Color: r.Color,
		Tags: tags, Settings: r.Settings, Archived: r.Archived,
		StartDate: parseTimePtr(r.StartDate), DueDate: parseTimePtr(r.DueDate),
	}
}

// parseTimePtr 把 ISO/日期字符串解析为 *types.Time；空串→nil。
func parseTimePtr(s string) *types.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, lay := range []string{"2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(lay, s); err == nil {
			v := types.Time(t)
			return &v
		}
	}
	return nil
}

// stringJSONArr 序化字符串数组为 JSON 文本。
func stringJSONArr(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "[]"
	}
	return string(b)
}
