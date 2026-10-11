package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 岗位 =====
type PostSearchReq struct {
	PostCode string `json:"postCode" form:"postCode"`
	PostName string `json:"postName" form:"postName"`
	Status   string `json:"status" form:"status"`
	PageReq
}
type PostSearchRes struct {
	ListRes
	PostList []*entity.Post `json:"postList"`
}
type PostReq struct {
	PostCode string `json:"postCode" form:"postCode" binding:"required"`
	PostName string `json:"postName" form:"postName" binding:"required"`
	PostSort int    `json:"postSort" form:"postSort" binding:"required"`
	Status   uint   `json:"status" form:"status" binding:"required"`
	Remark   string `json:"remark" form:"remark"`
}
type PostAddReq struct{ *PostReq }
type PostEditReq struct {
	PostId string `json:"postId" form:"postId" binding:"required"`
	*PostReq
}
type PostDeleteReq struct {
	Ids []string `json:"ids" form:"ids"`
}

// ===== 部门 =====
type DeptSearchReq struct {
	DeptName string `json:"deptName" form:"deptName"`
	Status   string `json:"status" form:"status"`
}
type DeptSearchRes struct {
	DeptList []*model.DeptInfo `json:"deptList"`
}
type DeptAddReq struct {
	ParentID string `json:"parentId" form:"parentId"`
	DeptName string `json:"deptName" form:"deptName" binding:"required"`
	OrderNum int    `json:"orderNum" form:"orderNum"`
	LeaderId string `json:"leaderId" form:"leaderId"`
	Status   uint   `json:"status" form:"status" binding:"required"`
}
type DeptEditReq struct {
	DeptId   string `json:"deptId" form:"deptId" binding:"required"`
	ParentID string `json:"parentId" form:"parentId"`
	DeptName string `json:"deptName" form:"deptName" binding:"required"`
	OrderNum int    `json:"orderNum" form:"orderNum"`
	LeaderId string `json:"leaderId" form:"leaderId"`
	Status   uint   `json:"status" form:"status" binding:"required"`
}
type DeptDeleteReq struct {
	Id string `json:"id" form:"id" binding:"required"`
}
type DeptTreeSelectRes struct {
	Deps []*model.DeptTreeNode `json:"deps"`
}
