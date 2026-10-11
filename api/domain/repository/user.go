package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// IUserRepository 用户数据访问（aa_user / aa_login_log / aa_user_online / aa_user_post）
type IUserRepository interface {
	// 登录相关
	GetUserByUsername(ctx context.Context, userName string) (*model.LoginUserRes, error)
	GetUserById(ctx context.Context, userId string) (*model.LoginUserRes, error)
	UpdateLoginInfo(ctx context.Context, userId string, ip string) error
	InsertLoginLog(ctx context.Context, params *model.LoginLogParams)
	InsertUserOnline(ctx context.Context, params *model.UserOnlineParams)

	// 用户管理
	GetEntityById(ctx context.Context, userId string) (*entity.User, error)
	List(ctx context.Context, deptId, mobile, status, keyWords string, pageNum, pageSize int) (list []*entity.User, total int64, err error)
	EnrichUsers(ctx context.Context, users []*entity.User) ([]*model.UserRoleDeptRes, error)
	Add(ctx context.Context, u *entity.User, roleIds, postIds []string) (string, error)
	Edit(ctx context.Context, u *entity.User, roleIds, postIds []string) error
	Delete(ctx context.Context, userIds []string) error
	ResetPwd(ctx context.Context, userId string, pwd, salt string) error
	ChangeStatus(ctx context.Context, userId string, status uint) error
	GetEditUser(ctx context.Context, userId string) (user *entity.User, roleIds []string, postIds []string, err error)
	GetUsersByIds(ctx context.Context, userIds []string) ([]*model.UserSimpleRes, error)
	// 按工号批量取用户姓名+部门（员工集合成员明细解析）
	GetUsersByEmpNos(ctx context.Context, empNos []string) ([]*model.MemberInfoRes, error)
	// 人员选择器搜索：name / employee_no / mobile / user_email 四字段 OR
	SearchOptions(ctx context.Context, keyWords string, pageNum, pageSize int) (list []*model.UserOptionRes, total int64, err error)
	// 按用户 id 批量取选项投影（授权条件行已选用户回显）
	GetUserOptionsByIds(ctx context.Context, userIds []string) ([]*model.UserOptionRes, error)
	// 用户-角色（casbin grouping）维护
	ReplaceUserRoles(ctx context.Context, userId string, roleIds []string) error
	// 权限身份取数：部门 id + 岗位 ids + 工号（登录签发带身份 claims 用）
	GetDeptAndPosts(ctx context.Context, userId string) (deptId string, postIds []string, employeeNo string, err error)
	// ListAdminIds 全部超管 user_id（部门无负责人时临时预算审批的兜底通知对象）
	ListAdminIds(ctx context.Context) ([]string, error)
}
