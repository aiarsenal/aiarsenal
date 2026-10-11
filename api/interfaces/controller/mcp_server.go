package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// MCP 服务器配置控制器。

type McpServerController struct{ svc *service.McpServerService }

func NewMcpServerController(s *service.McpServerService) *McpServerController {
	return &McpServerController{svc: s}
}

func (h *McpServerController) List(c *gin.Context) {
	var req dto.McpServerSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Name, req.Type, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpServerSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *McpServerController) All(c *gin.Context) {
	list, err := h.svc.All(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, list)
}

func (h *McpServerController) Get(c *gin.Context) {
	id := c.Query("serverId")
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

func (h *McpServerController) Add(c *gin.Context) {
	var req dto.McpServerAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := mcpServerFromReq(req.McpServerReq)
	e.CreatedBy = middleware.GetUserId(c)
	e.UpdatedBy = middleware.GetUserId(c)
	if _, err := h.svc.Add(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpServerController) Edit(c *gin.Context) {
	var req dto.McpServerEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := mcpServerFromReq(req.McpServerReq)
	e.ServerId = req.ServerId
	e.UpdatedBy = middleware.GetUserId(c)
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpServerController) Delete(c *gin.Context) {
	var req dto.McpIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func mcpServerFromReq(r *dto.McpServerReq) *entity.McpServer {
	return &entity.McpServer{
		Name: r.Name, Type: r.Type, Description: r.Description, Url: r.Url, Command: r.Command,
		Args: r.Args, Env: r.Env, Headers: r.Headers, Options: r.Options, Oauth: r.Oauth, Proxy: r.Proxy,
		Openapi: r.Openapi, PassthroughHeaders: r.PassthroughHeaders, Enabled: r.Enabled, Visibility: r.Visibility,
		EnableKeepAlive: r.EnableKeepAlive, KeepAliveInterval: r.KeepAliveInterval,
		Tools: r.Tools, Prompts: r.Prompts, Resources: r.Resources,
	}
}
