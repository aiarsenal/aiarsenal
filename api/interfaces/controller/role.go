package controller

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type RoleController struct{ svc *service.RoleService }

func NewRoleController(s *service.RoleService) *RoleController { return &RoleController{svc: s} }

func (h *RoleController) List(c *gin.Context) {
	var req dto.RoleListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	slog.Debug("role 列表请求", "roleName", req.RoleName, "status", req.Status, "pageNum", req.PageNum, "pageSize", req.PageSize)
	list, total, err := h.svc.List(c, req.RoleName, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		slog.Error("role 列表失败", "roleName", req.RoleName, "status", req.Status, "err", err)
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.RoleListRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *RoleController) GetParams(c *gin.Context) {
	menu, err := h.svc.GetParams(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.RoleGetParamsRes{Menu: menu})
}

func (h *RoleController) Get(c *gin.Context) {
	var req dto.RoleGetReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	role, menuIds, err := h.svc.Get(c, req.Id)
	if err != nil {
		middleware.Fail(c, "获取角色信息失败")
		return
	}
	middleware.Success(c, dto.RoleGetRes{Role: role, MenuIds: menuIds})
}

func (h *RoleController) Add(c *gin.Context) {
	var req dto.RoleAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Add(c, &entity.Role{
		Name: req.Name, Status: req.Status, ListOrder: req.ListOrder, Remark: req.Remark,
	}, req.MenuIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *RoleController) Edit(c *gin.Context) {
	var req dto.RoleEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Edit(c, &entity.Role{
		RoleId: req.Id, Name: req.Name, Status: req.Status, ListOrder: req.ListOrder, Remark: req.Remark,
	}, req.MenuIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *RoleController) Delete(c *gin.Context) {
	var req dto.RoleDeleteReq
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
