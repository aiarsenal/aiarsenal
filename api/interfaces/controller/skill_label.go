package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type SkillLabelController struct{ svc *service.SkillLabelService }

func NewSkillLabelController(s *service.SkillLabelService) *SkillLabelController {
	return &SkillLabelController{svc: s}
}

func (h *SkillLabelController) List(c *gin.Context) {
	var req dto.SkillLabelSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Slug, req.Type, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.SkillLabelSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *SkillLabelController) All(c *gin.Context) {
	list, err := h.svc.All(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillLabelController) Get(c *gin.Context) {
	id := c.Query("labelId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	l, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, l)
}

func (h *SkillLabelController) Add(c *gin.Context) {
	var req dto.SkillLabelAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillLabelReq
	if _, err := h.svc.Add(c, &entity.SkillLabelDefinition{
		Slug: r.Slug, Type: r.Type, VisibleInFilter: r.VisibleInFilter, SortOrder: r.SortOrder,
		CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillLabelController) Edit(c *gin.Context) {
	var req dto.SkillLabelEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillLabelReq
	if err := h.svc.Edit(c, &entity.SkillLabelDefinition{
		LabelId: req.LabelId, Slug: r.Slug, Type: r.Type, VisibleInFilter: r.VisibleInFilter, SortOrder: r.SortOrder,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillLabelController) Delete(c *gin.Context) {
	var req dto.SkillLabelDeleteReq
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

func (h *SkillLabelController) ListTranslations(c *gin.Context) {
	id := c.Query("labelId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.ListTranslations(c, id)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillLabelController) UpsertTranslation(c *gin.Context) {
	var req dto.SkillLabelTransReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.UpsertTranslation(c, &entity.SkillLabelTranslation{
		LabelId: req.LabelId, Locale: req.Locale, DisplayName: req.DisplayName,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
