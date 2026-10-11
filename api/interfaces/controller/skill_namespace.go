package controller

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type SkillNamespaceController struct {
	svc *service.SkillNamespaceService
}

func NewSkillNamespaceController(s *service.SkillNamespaceService) *SkillNamespaceController {
	return &SkillNamespaceController{svc: s}
}

func (h *SkillNamespaceController) List(c *gin.Context) {
	var req dto.SkillNamespaceSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	slog.Debug("skill namespace 列表请求", "slug", req.Slug, "name", req.Name, "type", req.Type, "status", req.Status, "pageNum", req.PageNum, "pageSize", req.PageSize)
	list, total, err := h.svc.List(c, req.Slug, req.Name, req.Type, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		slog.Error("skill namespace 列表失败", "slug", req.Slug, "type", req.Type, "status", req.Status, "err", err)
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.SkillNamespaceSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *SkillNamespaceController) All(c *gin.Context) {
	list, err := h.svc.All(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillNamespaceController) Get(c *gin.Context) {
	id := c.Query("namespaceId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	ns, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, ns)
}

func (h *SkillNamespaceController) Add(c *gin.Context) {
	var req dto.SkillNamespaceAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillNamespaceReq
	if _, err := h.svc.Add(c, &entity.SkillNamespace{
		Slug: r.Slug, DisplayName: r.DisplayName, Type: r.Type, Description: r.Description,
		Avatar: r.Avatar, Status: r.Status, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillNamespaceController) Edit(c *gin.Context) {
	var req dto.SkillNamespaceEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SkillNamespaceReq
	if err := h.svc.Edit(c, &entity.SkillNamespace{
		NamespaceId: req.NamespaceId, Slug: r.Slug, DisplayName: r.DisplayName, Type: r.Type,
		Description: r.Description, Avatar: r.Avatar, Status: r.Status,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillNamespaceController) Delete(c *gin.Context) {
	var req dto.SkillNamespaceDeleteReq
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

func (h *SkillNamespaceController) ListMembers(c *gin.Context) {
	id := c.Query("namespaceId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.svc.ListMembers(c, id)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *SkillNamespaceController) AddMember(c *gin.Context) {
	var req dto.SkillNamespaceMemberAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.AddMember(c, &entity.SkillNamespaceMember{
		NamespaceId: req.NamespaceId, UserId: req.UserId, Role: req.Role,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *SkillNamespaceController) RemoveMember(c *gin.Context) {
	var req dto.SkillNamespaceMemberDelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.RemoveMember(c, req.MemberId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
