package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type MenuController struct {
	svc     *service.MenuService
	roleSvc *service.RoleService
}

func NewMenuController(m *service.MenuService, r *service.RoleService) *MenuController {
	return &MenuController{svc: m, roleSvc: r}
}

func (h *MenuController) List(c *gin.Context) {
	var req dto.MenuSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.Search(c, req.Title, req.Component)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	// 有搜索条件 → flat（每项无 children）；否则 → 树
	if req.Title != "" || req.Component != "" {
		middleware.Success(c, gin.H{"rules": list})
	} else {
		middleware.Success(c, gin.H{"rules": h.svc.BuildTree(list)})
	}
}

func (h *MenuController) GetParams(c *gin.Context) {
	roles, err := h.roleSvc.All(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	menus, err := h.svc.GetIsMenuList(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.MenuGetParamsRes{Roles: roles, Menus: menus})
}

func (h *MenuController) Get(c *gin.Context) {
	var req dto.MenuInfoReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	rule, roleIds, err := h.svc.Get(c, req.Id)
	if err != nil {
		middleware.Fail(c, "获取菜单失败")
		return
	}
	middleware.Success(c, dto.MenuInfoRes{Rule: rule, RoleIds: roleIds})
}

func (h *MenuController) Add(c *gin.Context) {
	var req dto.MenuAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Add(c, menuAddToEntity(&req), req.GrantRules); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *MenuController) Update(c *gin.Context) {
	var req dto.MenuUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Update(c, menuUpdateToEntity(&req), req.GrantRules); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *MenuController) Delete(c *gin.Context) {
	var req dto.MenuDeleteReq
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

func menuAddToEntity(r *dto.MenuAddReq) *entity.Menu {
	return &entity.Menu{
		Pid: r.Pid, Name: r.Name, Title: r.Title, EnName: r.EnName, Icon: r.Icon, Condition: r.Condition,
		Remark: r.Remark, MenuType: r.MenuType, Weigh: r.Weigh, IsHide: r.IsHide, Path: r.Path,
		Redirect: r.Redirect, Component: r.Component, IsLink: r.IsLink, IsIframe: r.IsIframe,
		IsCached: r.IsCached, IsAffix: r.IsAffix, LinkUrl: r.LinkUrl,
	}
}

func menuUpdateToEntity(r *dto.MenuUpdateReq) *entity.Menu {
	e := menuAddToEntity(&dto.MenuAddReq{
		MenuType: r.MenuType, Pid: r.Pid, Name: r.Name, Title: r.Title, EnName: r.EnName, Icon: r.Icon,
		Weigh: r.Weigh, Condition: r.Condition, Remark: r.Remark, IsHide: r.IsHide, Path: r.Path,
		Redirect: r.Redirect, Component: r.Component, IsLink: r.IsLink,
		IsIframe: r.IsIframe, IsCached: r.IsCached, IsAffix: r.IsAffix, LinkUrl: r.LinkUrl,
	})
	e.MenuId = r.Id
	return e
}
