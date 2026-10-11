package model

import "aiarsenal/domain/entity"

// UserRoleDeptRes 带部门/角色/岗位的用户数据（用户列表返回项）
type UserRoleDeptRes struct {
	*entity.User
	Dept     *UserRoleInfoRes   `json:"dept,omitempty"`
	RoleInfo []*UserRoleInfoRes `json:"roleInfo"`
	Post     []*UserPostInfoRes `json:"post"`
}
