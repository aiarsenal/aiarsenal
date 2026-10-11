package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// MCP Bearer 密钥控制器。签发返回明文一次；轮换返回新明文。

type McpKeyController struct{ svc *service.McpKeyService }

func NewMcpKeyController(s *service.McpKeyService) *McpKeyController {
	return &McpKeyController{svc: s}
}

func (h *McpKeyController) List(c *gin.Context) {
	var req dto.McpKeySearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpKeySearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *McpKeyController) Get(c *gin.Context) {
	id := c.Query("keyId")
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

func (h *McpKeyController) Add(c *gin.Context) {
	var req dto.McpKeyAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := mcpKeyFromReq(req.McpKeyReq)
	e.CreatedBy = middleware.GetUserId(c)
	id, plain, err := h.svc.Create(c, e)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.McpKeyCreateRes{KeyId: id, Token: plain})
}

func (h *McpKeyController) Edit(c *gin.Context) {
	var req dto.McpKeyEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := mcpKeyFromReq(req.McpKeyReq)
	e.KeyId = req.KeyId
	e.Owner = middleware.GetUserId(c)
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpKeyController) Delete(c *gin.Context) {
	var req dto.McpIdsReq
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

func (h *McpKeyController) Block(c *gin.Context) {
	id := c.Query("keyId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	blocked := c.Query("blocked") == "1"
	if err := h.svc.Block(c, id, blocked, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpKeyController) Rotate(c *gin.Context) {
	id := c.Query("keyId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	plain, err := h.svc.Rotate(c, id, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.McpKeyCreateRes{Token: plain})
}

func mcpKeyFromReq(r *dto.McpKeyReq) *entity.McpBearerKey {
	return &entity.McpBearerKey{
		Name: r.Name, Kind: r.Kind, AccessType: r.AccessType,
		AllowedGroups: r.AllowedGroups, AllowedServers: r.AllowedServers, Enabled: r.Enabled,
	}
}
