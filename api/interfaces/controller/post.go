package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type PostController struct{ svc *service.PostService }

func NewPostController(s *service.PostService) *PostController { return &PostController{svc: s} }

func (h *PostController) List(c *gin.Context) {
	var req dto.PostSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.PostCode, req.PostName, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.PostSearchRes{PostList: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *PostController) Add(c *gin.Context) {
	var req dto.PostAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.PostReq
	if err := h.svc.Add(c, &entity.Post{
		PostCode: r.PostCode, PostName: r.PostName, PostSort: r.PostSort,
		Status: r.Status, Remark: r.Remark, CreatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *PostController) Edit(c *gin.Context) {
	var req dto.PostEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.PostReq
	if err := h.svc.Edit(c, &entity.Post{
		PostId: req.PostId, PostCode: r.PostCode, PostName: r.PostName,
		PostSort: r.PostSort, Status: r.Status, Remark: r.Remark, UpdatedBy: middleware.GetUserId(c),
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *PostController) Delete(c *gin.Context) {
	var req dto.PostDeleteReq
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
