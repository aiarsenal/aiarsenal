package dto

import "aiarsenal/domain/entity"

// ===== 缓存 =====
type CacheRemoveReq struct{}

// ===== 个人中心 =====
type PersonalInfoRes struct {
	User     *entity.User `json:"user"`
	Roles    []string     `json:"roles"`
	DeptName string       `json:"deptName"`
}
type PersonalEditReq struct {
	Nickname  string `json:"nickname" form:"nickname" binding:"required"`
	Mobile    string `json:"mobile" form:"mobile" binding:"required"`
	Remark    string `json:"remark" form:"remark"`
	Sex       int    `json:"sex" form:"sex"`
	UserEmail string `json:"userEmail" form:"userEmail" binding:"required,email"`
	Describe  string `json:"describe" form:"describe"`
	Avatar    string `json:"avatar" form:"avatar"`
}
type PersonalEditRes struct {
	UserInfo *entity.User `json:"userInfo"`
	Token    string       `json:"token"`
}
type PersonalResetPwdReq struct {
	Password string `json:"password" form:"password" binding:"required"`
}
