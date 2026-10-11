package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 菜单管理 =====
type MenuSearchReq struct {
	Title     string `json:"menuName" form:"menuName"`
	Component string `json:"component" form:"component"`
}
type MenuListRes struct {
	Rules []*model.MenuInfoRes `json:"rules"`
}
type MenuAddReq struct {
	MenuType  uint   `json:"menuType" form:"menuType"`
	Pid       string `json:"parentId" form:"parentId"`
	Name      string `json:"name" form:"name" binding:"required"`
	Title     string `json:"menuName" form:"menuName" binding:"required"`
	EnName    string `json:"enName" form:"enName"`
	Icon      string `json:"icon" form:"icon"`
	Weigh     int    `json:"menuSort" form:"menuSort"`
	Condition string `json:"condition" form:"condition"`
	Remark    string `json:"remark" form:"remark"`
	IsHide    uint   `json:"isHide" form:"isHide"`
	Path      string `json:"path" form:"path"`
	Redirect  string `json:"redirect" form:"redirect"`
	Component string `json:"component" form:"component"`
	IsLink    uint   `json:"isLink" form:"isLink"`
	IsIframe  uint   `json:"isIframe" form:"isIframe"`
	IsCached  uint   `json:"isKeepAlive" form:"isKeepAlive"`
	IsAffix   uint   `json:"isAffix" form:"isAffix"`
	LinkUrl   string `json:"linkUrl" form:"linkUrl"`
	// GrantRules 组合授权条件行 JSON（[]{userIds,deptId,roleIds,postIds,exclude}），可空。
	// 角色直授不经菜单接口（统一走角色管理页），故无 roles 字段。
	GrantRules string `json:"grantRules" form:"grantRules"`
}
type MenuGetParamsRes struct {
	Roles []*entity.Role       `json:"roles"`
	Menus []*model.MenuInfoRes `json:"menus"`
}
type MenuInfoReq struct {
	Id string `json:"id" form:"id" binding:"required"`
}
type MenuInfoRes struct {
	Rule    *entity.Menu `json:"rule"`
	RoleIds []string     `json:"roleIds"`
}
type MenuUpdateReq struct {
	Id        string `json:"id" form:"id" binding:"required"`
	MenuType  uint   `json:"menuType" form:"menuType"`
	Pid       string `json:"parentId" form:"parentId"`
	Name      string `json:"name" form:"name" binding:"required"`
	Title     string `json:"menuName" form:"menuName" binding:"required"`
	EnName    string `json:"enName" form:"enName"`
	Icon      string `json:"icon" form:"icon"`
	Weigh     int    `json:"menuSort" form:"menuSort"`
	Condition string `json:"condition" form:"condition"`
	Remark    string `json:"remark" form:"remark"`
	IsHide    uint   `json:"isHide" form:"isHide"`
	Path      string `json:"path" form:"path"`
	Redirect  string `json:"redirect" form:"redirect"`
	Component string `json:"component" form:"component"`
	IsLink    uint   `json:"isLink" form:"isLink"`
	IsIframe  uint   `json:"isIframe" form:"isIframe"`
	IsCached  uint   `json:"isKeepAlive" form:"isKeepAlive"`
	IsAffix   uint   `json:"isAffix" form:"isAffix"`
	LinkUrl   string `json:"linkUrl" form:"linkUrl"`
	// GrantRules 组合授权条件行 JSON（[]{userIds,deptId,roleIds,postIds,exclude}），可空。
	// 角色直授不经菜单接口（统一走角色管理页），故无 roles 字段。
	GrantRules string `json:"grantRules" form:"grantRules"`
}
type MenuDeleteReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
