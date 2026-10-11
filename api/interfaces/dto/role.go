package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 角色 =====
type RoleListReq struct {
	RoleName string `json:"roleName" form:"roleName"`
	Status   string `json:"roleStatus" form:"roleStatus"`
	PageReq
}
type RoleListRes struct {
	ListRes
	List []*entity.Role `json:"list"`
}
type RoleGetParamsRes struct {
	Menu []*model.MenuInfoRes `json:"menu"`
}
type RoleAddReq struct {
	Name      string   `json:"name" form:"name" binding:"required"`
	Status    uint     `json:"status" form:"status"`
	ListOrder uint     `json:"listOrder" form:"listOrder"`
	Remark    string   `json:"remark" form:"remark"`
	MenuIds   []string `json:"menuIds" form:"menuIds"`
}
type RoleGetReq struct {
	Id string `json:"id" form:"id" binding:"required"`
}
type RoleGetRes struct {
	Role    *entity.Role `json:"role"`
	MenuIds []string     `json:"menuIds"`
}
type RoleEditReq struct {
	Id        string   `json:"id" form:"id" binding:"required"`
	Name      string   `json:"name" form:"name" binding:"required"`
	Status    uint     `json:"status" form:"status"`
	ListOrder uint     `json:"listOrder" form:"listOrder"`
	Remark    string   `json:"remark" form:"remark"`
	MenuIds   []string `json:"menuIds" form:"menuIds"`
}
type RoleDeleteReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
