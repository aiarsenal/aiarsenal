package controller

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// agent_workspace.go — 实例工作区文件管理（localdocker 直连桥接）。
// 请求/响应结构体内联在本文件（dto 目录不归本任务改）。
// 全部端点挂 agInst（JWT+Casbin），owner 校验与 tty 同款。

// AgentWorkspaceController 工作区文件控制器。
type AgentWorkspaceController struct {
	svc     *service.AgentWorkspaceService
	instSvc *service.AgentInstanceService // 实例归属校验
}

// NewAgentWorkspaceController 构造。
func NewAgentWorkspaceController(svc *service.AgentWorkspaceService, instSvc *service.AgentInstanceService) *AgentWorkspaceController {
	return &AgentWorkspaceController{svc: svc, instSvc: instSvc}
}

// checkOwner 实例存在 + 归属校验（返回gin处理结果：true=通过）。
func (h *AgentWorkspaceController) checkOwner(c *gin.Context, id string) bool {
	uid := middleware.GetUserId(c)
	ins, err := h.instSvc.Get(c, id)
	if err != nil || ins == nil {
		middleware.Fail(c, "未找到实例")
		return false
	}
	if ins.UserId != uid {
		middleware.Fail(c, "无权访问该实例")
		return false
	}
	return true
}

// WorkspaceListReq 目录列表请求（GET query）。
type WorkspaceListReq struct {
	Dir string `form:"dir"` // 工作区相对路径，空=根 /aa
}

// List 列目录（GET /agent/instance/:id/workspace/list?dir=）。
func (h *AgentWorkspaceController) List(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	var req WorkspaceListReq
	_ = c.ShouldBindQuery(&req)
	files, err := h.svc.List(c, id, req.Dir)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": files, "dir": req.Dir})
}

// WorkspacePathReq 相对路径类操作通用请求体。
type WorkspacePathReq struct {
	Path string `json:"path" binding:"required"`
}

// Upload 上传/覆盖文件（POST /agent/instance/:id/workspace/upload，multipart：
// 字段 file=文件内容，path=目标相对路径）。
func (h *AgentWorkspaceController) Upload(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	relPath := c.PostForm("path")
	if relPath == "" {
		middleware.Fail(c, "缺少目标路径 path")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		middleware.Fail(c, "缺少上传文件 file")
		return
	}
	src, err := fh.Open()
	if err != nil {
		middleware.Fail(c, "读取上传文件失败")
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		middleware.Fail(c, "读取上传文件失败")
		return
	}
	if err := h.svc.Upload(c, id, middleware.GetUserId(c), relPath, content); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"path": relPath, "size": len(content)})
}

// Download 下载文件（GET /agent/instance/:id/workspace/download?path=，流式）。
func (h *AgentWorkspaceController) Download(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	relPath := c.Query("path")
	if relPath == "" {
		middleware.Fail(c, "缺少 path")
		return
	}
	rc, size, err := h.svc.Download(c, id, middleware.GetUserId(c), relPath)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	defer rc.Close()
	// attachment 头带原始文件名（basename 由前端传或用 path 段）。
	filename := lastPathSegment(relPath)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.Status(200)
	_, _ = io.Copy(c.Writer, rc)
}

// Mkdir 建目录（POST /agent/instance/:id/workspace/mkdir）。
func (h *AgentWorkspaceController) Mkdir(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	var req WorkspacePathReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Mkdir(c, id, middleware.GetUserId(c), req.Path); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"path": req.Path})
}

// Delete 删除文件/目录（POST /agent/instance/:id/workspace/delete）。
func (h *AgentWorkspaceController) Delete(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	var req WorkspacePathReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, id, middleware.GetUserId(c), req.Path); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"path": req.Path})
}

// WorkspaceRenameReq 改名请求体。
type WorkspaceRenameReq struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}

// Rename 改名/移动（POST /agent/instance/:id/workspace/rename）。
func (h *AgentWorkspaceController) Rename(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	var req WorkspaceRenameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Rename(c, id, middleware.GetUserId(c), req.From, req.To); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"from": req.From, "to": req.To})
}

// Audit 操作审计分页（GET /agent/instance/:id/workspace/audit?pageNum=&pageSize=）。
func (h *AgentWorkspaceController) Audit(c *gin.Context) {
	id := c.Param("id")
	if !h.checkOwner(c, id) {
		return
	}
	list, total, err := h.svc.AuditList(c, id, queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": total})
}

// lastPathSegment 取相对路径最后一段作下载文件名。
func lastPathSegment(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// queryInt query 整型取值（缺省/非法走 def）。本任务新增 controller 共用的分页小工具。
func queryInt(c *gin.Context, name string, def int) int {
	if s := c.Query(name); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return def
}
