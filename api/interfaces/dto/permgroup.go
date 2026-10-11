package dto

import "aiarsenal/domain/entity"

// ===== 权限分组管理 =====
type PermGroupListReq struct {
	Name string `json:"name" form:"name"`
}
type PermGroupListRes struct {
	List []*entity.PermGroup `json:"list"`
}
type PermGroupSaveReq struct {
	Id     string `json:"id" form:"id"` // 空 = 新增
	Name   string `json:"name" form:"name" binding:"required"`
	Remark string `json:"remark" form:"remark"`
	// Terms 词条行 JSON（[]{userIds,deptId,roleIds,postIds,exclude,groupId,setId}，行内非空维度 AND）
	Terms string `json:"terms" form:"terms"`
}
type PermGroupDeleteReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}

type EmpSetListReq struct {
	Name string `json:"name" form:"name"`
}
type EmpSetListRes struct {
	List []*entity.EmpSet `json:"list"`
}
type EmpSetSaveReq struct {
	Id     string `json:"id" form:"id"` // 空 = 新增
	Name   string `json:"name" form:"name" binding:"required"`
	Remark string `json:"remark" form:"remark"`
	// Members 工号 JSON 数组文本（选人 + 粘贴两入口合并后的终态）
	Members string `json:"members" form:"members"`
}
type EmpSetDeleteReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
type EmpSetResolveReq struct {
	// EmpNos 待解析的工号数组（集合弹窗成员明细；上限 500）
	EmpNos []string `json:"empNos" form:"empNos"`
}

// PermGroupOptionsRes 瘦投影（GrantEditor 选择器）
type PermGroupOptionsRes struct {
	Groups []map[string]string `json:"groups"`
	Sets   []map[string]string `json:"sets"`
}
