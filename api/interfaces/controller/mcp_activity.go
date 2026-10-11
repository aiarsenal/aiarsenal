package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// MCP 调用审计控制器（只读）。

type McpActivityController struct{ svc *service.McpActivityService }

func NewMcpActivityController(s *service.McpActivityService) *McpActivityController {
	return &McpActivityController{svc: s}
}

func (h *McpActivityController) List(c *gin.Context) {
	var req dto.McpActivitySearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.ServerName, req.Tool, req.Status, req.UserId, req.DateRange, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.McpActivitySearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}
