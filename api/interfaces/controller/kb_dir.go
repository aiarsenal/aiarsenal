package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// KbDirController 文档目录管理（树/新建/重命名/移动/删除/按引用反查）
type KbDirController struct {
	svc   *service.KbDirService
	space *service.KbSpaceService
}

func NewKbDirController(s *service.KbDirService, space *service.KbSpaceService) *KbDirController {
	return &KbDirController{svc: s, space: space}
}

// List 目录树（含每目录直属/子树文档数）
// GET /api/v1/doc/dir/list
func (h *KbDirController) List(c *gin.Context) {
	tree, err := h.svc.Tree(c, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "获取目录失败")
		return
	}
	middleware.Success(c, tree)
}

// Create 新建目录（parentId ”=根）
// POST /api/v1/doc/dir/create
func (h *KbDirController) Create(c *gin.Context) {
	var req dto.KbDirCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	d, err := h.svc.Create(c, req.ParentId, req.Name, middleware.GetUserId(c),
		req.Visibility, req.ReadRules, req.WriteRules, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, d)
}

// Rename 重命名目录
// PUT /api/v1/doc/dir/rename
func (h *KbDirController) Rename(c *gin.Context) {
	var req dto.KbDirRenameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Rename(c, req.DirId, req.Name, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Move 移动目录（拖拽）
// POST /api/v1/doc/dir/move
func (h *KbDirController) Move(c *gin.Context) {
	var req dto.KbDirMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Move(c, req.DirId, req.ParentId, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Delete 删除目录（须空目录）
// DELETE /api/v1/doc/dir/delete
func (h *KbDirController) Delete(c *gin.Context) {
	var req dto.KbDirDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.DirId, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ByRef 按引用查托管目录（项目空间跳转：kind=project|personal + refId=projectId/userId，me=本人）
// GET /api/v1/kb/dir/byref
func (h *KbDirController) ByRef(c *gin.Context) {
	d, err := h.space.GetByRef(c, c.Query("kind"), c.Query("refId"), middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"dirId": d.DirId, "name": d.Name, "kind": d.Kind, "refId": d.RefId})
}
