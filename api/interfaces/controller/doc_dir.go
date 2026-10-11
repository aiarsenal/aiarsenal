package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// DocDirController 文档目录管理（树/新建/重命名/移动/删除/按引用反查）
type DocDirController struct {
	svc   *service.DocDirService
	space *service.DocSpaceService
}

func NewDocDirController(s *service.DocDirService, space *service.DocSpaceService) *DocDirController {
	return &DocDirController{svc: s, space: space}
}

// List 目录树（含每目录直属/子树文档数）
// GET /api/v1/doc/dir/list
func (h *DocDirController) List(c *gin.Context) {
	tree, err := h.svc.Tree(c, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "获取目录失败")
		return
	}
	middleware.Success(c, tree)
}

// Create 新建目录（parentId ”=根）
// POST /api/v1/doc/dir/create
func (h *DocDirController) Create(c *gin.Context) {
	var req dto.DocDirCreateReq
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
func (h *DocDirController) Rename(c *gin.Context) {
	var req dto.DocDirRenameReq
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
func (h *DocDirController) Move(c *gin.Context) {
	var req dto.DocDirMoveReq
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
func (h *DocDirController) Delete(c *gin.Context) {
	var req dto.DocDirDeleteReq
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

// ByRef 按引用查托管目录（项目空间跳转：kind=project|personal + refId=projectId/userId）
// GET /api/v1/doc/dir/byref
func (h *DocDirController) ByRef(c *gin.Context) {
	d, err := h.space.GetByRef(c, c.Query("kind"), c.Query("refId"), middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"dirId": d.DirId, "name": d.Name, "kind": d.Kind, "refId": d.RefId})
}
