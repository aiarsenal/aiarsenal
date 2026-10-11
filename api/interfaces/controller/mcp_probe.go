package controller

// MCP 上游探测控制器：连接测试（含工具预览）与在线试调用。

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type McpProbeController struct{ svc *service.McpProbeService }

func NewMcpProbeController(s *service.McpProbeService) *McpProbeController {
	return &McpProbeController{svc: s}
}

// Test 连接测试：serverId 非空测已保存配置，否则按表单字段直测。
func (h *McpProbeController) Test(c *gin.Context) {
	var req dto.McpServerTestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	var form *entity.McpServer
	if req.McpServerReq != nil {
		form = mcpServerFromReq(req.McpServerReq)
	}
	res, ms, err := h.svc.Test(c, req.ServerId, form)
	out := dto.McpServerTestRes{Ok: err == nil, DurationMs: ms,
		Tools: []dto.McpToolPreview{}, Prompts: []dto.McpPromptPreview{}, Resources: []dto.McpResourcePreview{}}
	if err != nil {
		out.Message = err.Error()
		middleware.Success(c, out)
		return
	}
	for _, t := range res.Tools {
		out.Tools = append(out.Tools, dto.McpToolPreview{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	for _, p := range res.Prompts {
		out.Prompts = append(out.Prompts, dto.McpPromptPreview{Name: p.Name, Description: p.Description})
	}
	for _, r := range res.Resources {
		out.Resources = append(out.Resources, dto.McpResourcePreview{Name: r.Name, Description: r.Description, MimeType: r.MIMEType, Uri: r.URI})
	}
	middleware.Success(c, out)
}

// CallTool 在线试调用。
func (h *McpProbeController) CallTool(c *gin.Context) {
	var req dto.McpServerCallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	text, ms, err := h.svc.CallTool(c, req.ServerId, req.Tool, req.Arguments)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.McpServerCallRes{DurationMs: ms, Text: text})
}
