package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// MCP 内置 Prompt / Resource 控制器。

type McpBuiltinController struct{ svc *service.McpBuiltinService }

func NewMcpBuiltinController(s *service.McpBuiltinService) *McpBuiltinController {
	return &McpBuiltinController{svc: s}
}

// ===== Prompt =====
func (h *McpBuiltinController) ListPrompts(c *gin.Context) {
	var req dto.McpPromptSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListPrompts(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpPromptSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *McpBuiltinController) GetPrompt(c *gin.Context) {
	id := c.Query("promptId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetPrompt(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *McpBuiltinController) AddPrompt(c *gin.Context) {
	var req dto.McpPromptAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpBuiltinPrompt{Name: req.Name, Title: req.Title, Description: req.Description, Template: req.Template, Arguments: req.Arguments, Enabled: req.Enabled}
	if _, err := h.svc.AddPrompt(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpBuiltinController) EditPrompt(c *gin.Context) {
	var req dto.McpPromptEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpBuiltinPrompt{PromptId: req.PromptId, Name: req.Name, Title: req.Title, Description: req.Description, Template: req.Template, Arguments: req.Arguments, Enabled: req.Enabled}
	if err := h.svc.EditPrompt(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpBuiltinController) DeletePrompts(c *gin.Context) {
	var req dto.McpIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeletePrompts(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Resource =====
func (h *McpBuiltinController) ListResources(c *gin.Context) {
	var req dto.McpResourceSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListResources(c, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpResourceSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *McpBuiltinController) GetResource(c *gin.Context) {
	id := c.Query("resourceId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetResource(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *McpBuiltinController) AddResource(c *gin.Context) {
	var req dto.McpResourceAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpBuiltinResource{Uri: req.Uri, Name: req.Name, Description: req.Description, MimeType: req.MimeType, Content: req.Content, Enabled: req.Enabled}
	if _, err := h.svc.AddResource(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpBuiltinController) EditResource(c *gin.Context) {
	var req dto.McpResourceEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	e := &entity.McpBuiltinResource{ResourceId: req.ResourceId, Uri: req.Uri, Name: req.Name, Description: req.Description, MimeType: req.MimeType, Content: req.Content, Enabled: req.Enabled}
	if err := h.svc.EditResource(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *McpBuiltinController) DeleteResources(c *gin.Context) {
	var req dto.McpIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteResources(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
