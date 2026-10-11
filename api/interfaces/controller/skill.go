package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type SkillController struct{ svc *service.SkillService }

func NewSkillController(s *service.SkillService) *SkillController { return &SkillController{svc: s} }

func (h *SkillController) List(c *gin.Context) {
	var req dto.SkillSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.NamespaceId, req.Slug, req.Name, req.Status, req.KeyWords, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.SkillSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *SkillController) Get(c *gin.Context) {
	id := c.Query("skillId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	sk, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, sk)
}

func (h *SkillController) Add(c *gin.Context) {
	var req dto.SkillAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillReq
	uid := middleware.GetUserId(c)
	if _, err := h.svc.Add(c, &entity.Skill{
		NamespaceId: r.NamespaceId, Slug: r.Slug, DisplayName: r.DisplayName, Summary: r.Summary,
		OwnerId: uid, Visibility: r.Visibility, Status: r.Status, CreatedBy: uid,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillController) Edit(c *gin.Context) {
	var req dto.SkillEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillReq
	if err := h.svc.Edit(c, &entity.Skill{
		SkillId: req.SkillId, NamespaceId: r.NamespaceId, Slug: r.Slug, DisplayName: r.DisplayName,
		Summary: r.Summary, Visibility: r.Visibility, Status: r.Status,
		UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillController) Delete(c *gin.Context) {
	var req dto.SkillDeleteReq
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

func (h *SkillController) SetHidden(c *gin.Context) {
	var req dto.SkillHiddenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.SetHidden(c, req.SkillId, req.Hidden, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 版本 =====

func (h *SkillController) ListVersions(c *gin.Context) {
	id := c.Query("skillId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.ListVersions(c, id)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillController) GetVersion(c *gin.Context) {
	id := c.Query("versionId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetVersion(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *SkillController) AddVersion(c *gin.Context) {
	var req dto.SkillVersionAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if _, err := h.svc.AddVersion(c, &entity.SkillVersion{
		SkillId: req.SkillId, Version: req.Version, Changelog: req.Changelog,
	}, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillController) PublishVersion(c *gin.Context) {
	var req dto.SkillVersionYankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// 发布只需 versionId，复用结构；容错
	}
	if req.VersionId == "" {
		middleware.Fail(c, "versionId 不能为空")
		return
	}
	if err := h.svc.PublishVersion(c, req.VersionId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillController) YankVersion(c *gin.Context) {
	var req dto.SkillVersionYankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.YankVersion(c, req.VersionId, middleware.GetUserId(c), req.Reason); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 标签 =====

func (h *SkillController) ListTags(c *gin.Context) {
	id := c.Query("skillId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.ListTags(c, id)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillController) AddTag(c *gin.Context) {
	var req dto.SkillTagAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AddTag(c, &entity.SkillTag{
		SkillId: req.SkillId, TagName: req.TagName, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillController) RemoveTag(c *gin.Context) {
	var req dto.SkillTagDelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.RemoveTag(c, req.TagId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
