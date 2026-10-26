package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 用户 =====
type UserSearchReq struct {
	DeptId   string `json:"deptId" form:"deptId"`
	Mobile   string `json:"mobile" form:"mobile"`
	Status   string `json:"status" form:"status"`
	KeyWords string `json:"keyWords" form:"keyWords"`
	PageReq
}
type UserSearchRes struct {
	UserList []*model.UserRoleDeptRes `json:"userList"`
	ListRes
}
type UserGetParamsRes struct {
	RoleList []*entity.Role `json:"roleList"`
	Posts    []*entity.Post `json:"posts"`
}
type SetUserReq struct {
	DeptId     string   `json:"deptId" form:"deptId" binding:"required"`
	Email      string   `json:"email" form:"email" binding:"omitempty,email"`
	NickName   string   `json:"nickName" form:"nickName" binding:"required"`
	Name       string   `json:"name" form:"name" binding:"required"`
	EmployeeNo string   `json:"employeeNo" form:"employeeNo" binding:"required"`
	Mobile     string   `json:"mobile" form:"mobile" binding:"required"`
	PostIds    []string `json:"postIds" form:"postIds"`
	Remark     string   `json:"remark" form:"remark"`
	RoleIds    []string `json:"roleIds" form:"roleIds"`
	Sex        int      `json:"sex" form:"sex"`
	Status     uint     `json:"status" form:"status"`
	IsAdmin    int      `json:"isAdmin" form:"isAdmin"`
}
type UserAddReq struct {
	*SetUserReq
	UserName string `json:"userName" form:"userName" binding:"required"`
	Password string `json:"password" form:"password" binding:"required"`
}
type UserEditReq struct {
	*SetUserReq
	UserId string `json:"userId" form:"userId" binding:"required"`
	// IsAdmin 影子字段（深度 0，JSON 绑定时覆盖 SetUserReq 的值类型）：
	// 指针语义区分「请求体没带」和「显式传值」。UI 编辑表单的回填白名单不含
	// isAdmin，请求体永远缺省——仓储层对 nil 绝不写该列，否则超管在界面上
	// 改个手机号就会把管理员静默降级为 0。显式传值才落库（超管收权必须传 0）。
	IsAdmin *int `json:"isAdmin" form:"isAdmin"`
}
type UserGetEditReq struct {
	Id string `json:"id" form:"id"`
}
type UserGetEditRes struct {
	User           *entity.User `json:"user"`
	CheckedRoleIds []string     `json:"checkedRoleIds"`
	CheckedPosts   []string     `json:"checkedPosts"`
}
type UserResetPwdReq struct {
	UserId   string `json:"userId" form:"userId" binding:"required"`
	Password string `json:"password" form:"password" binding:"required"`
}
type UserStatusReq struct {
	UserId     string `json:"userId" form:"userId" binding:"required"`
	UserStatus uint   `json:"status" form:"status" binding:"required"`
}
type UserDeleteReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
type UserGetByIdsReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
type UserGetByIdsRes struct {
	List []*model.UserSimpleRes `json:"list"`
}

// UserSearchOptionsReq 人员选择器搜索（姓名 / 工号 / 手机号 / 邮箱 四字段 OR）
type UserSearchOptionsReq struct {
	KeyWords string `json:"keyWords" form:"keyWords"`
	PageReq
}
type UserSearchOptionsRes struct {
	List []*model.UserOptionRes `json:"list"`
	ListRes
}

// UserResolveReq 按 id 批量解析用户（授权条件行已选用户回显；ids 为逗号分隔）
type UserResolveReq struct {
	Ids string `json:"ids" form:"ids"`
}
type UserResolveRes struct {
	List []*model.UserOptionRes `json:"list"`
}
