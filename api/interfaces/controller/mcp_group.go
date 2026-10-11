package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// MCP 分组控制器。

type McpGroupController struct{ svc *service.McpGroupService }

func NewMcpGroupController(s *service.McpGroupService) *McpGroupController {
	return &McpGroupController{svc: s}
}

func (h *McpGroupController) List(c *gin.Context) {
	var req dto.McpGroupSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.VisibleList(c, req.Name, req.PageNum, req.PageSize, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpGroupSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *McpGroupController) All(c *gin.Context) {
	list, err := h.svc.VisibleAll(c, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, list)
}

func (h *McpGroupController) Get(c *gin.Context) {
	id := c.Query("groupId")
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

func (h *McpGroupController) Add(c *gin.Context) {
	var req dto.McpGroupAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpGroup{Name: req.Name, Description: req.Description, Servers: req.Servers}
	e.CreatedBy = middleware.GetUserId(c)
	e.UpdatedBy = middleware.GetUserId(c)
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpGroupController) Edit(c *gin.Context) {
	var req dto.McpGroupEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpGroup{GroupId: req.GroupId, Name: req.Name, Description: req.Description, Servers: req.Servers}
	e.UpdatedBy = middleware.GetUserId(c)
	if err := h.svc.Edit(c, e, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpGroupController) Delete(c *gin.Context) {
	var req dto.McpIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// GrantGet GET /mcp/group/grant/get?groupId=... 授权配置回显（owner/超管）。
func (h *McpGroupController) GrantGet(c *gin.Context) {
	var req dto.McpGroupGrantGetReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	info, err := h.svc.GetGrants(c, req.GroupId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, info)
}

// GrantSave POST /mcp/group/grant/save 保存可见性档位 + 条件行（整体重写，owner/超管）。
func (h *McpGroupController) GrantSave(c *gin.Context) {
	var req dto.McpGroupGrantSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.SaveGrants(c, req.GroupId, req.Visibility, req.Rules, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
