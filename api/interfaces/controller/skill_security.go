package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type SkillSecurityController struct{ svc *service.SkillSecurityService }

func NewSkillSecurityController(s *service.SkillSecurityService) *SkillSecurityController {
	return &SkillSecurityController{svc: s}
}

func (h *SkillSecurityController) ListAudits(c *gin.Context) {
	var req dto.SkillAuditSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAudits(c, req.VersionId, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.SkillAuditSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *SkillSecurityController) Rescan(c *gin.Context) {
	var req dto.SkillAuditRescanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id, err := h.svc.Rescan(c, req.VersionId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"auditId": id})
}
