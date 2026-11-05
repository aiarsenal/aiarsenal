package repository

import (
	"time"

	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 岗位 =====
type IPostRepository interface {
	List(ctx context.Context, code, name, status string, pageNum, pageSize int) (list []*entity.Post, total int64, err error)
	Add(ctx context.Context, p *entity.Post) error
	Edit(ctx context.Context, p *entity.Post) error
	Delete(ctx context.Context, postIds []string) error
	// HoldsTopPost 用户是否持有全公司排序最前（post_sort 最小）的启用岗位（最高职级判定）。
	HoldsTopPost(ctx context.Context, userId string) bool
}

// ===== 部门 =====
type IDeptRepository interface {
	List(ctx context.Context, name, status string) (list []*model.DeptInfo, err error)
	Get(ctx context.Context, deptId string) (*entity.Dept, error)
	Add(ctx context.Context, d *entity.Dept) error
	Edit(ctx context.Context, d *entity.Dept) error
	Delete(ctx context.Context, deptId string) error
	HasChildren(ctx context.Context, parentId string) (bool, error)
}

// ===== 登录日志 =====
type ILoginLogRepository interface {
	List(ctx context.Context, loginName, status, ip, location string, pageNum, pageSize int) (list []*entity.LoginLog, total int64, err error)
	Delete(ctx context.Context, infoIds []string) error
	Clear(ctx context.Context) error
	// PurgeBefore 留存清理专用：删除 login_time 早于 cutoff 的行，返回实际删除数。
	// 仅审计留存清理任务调用（PR③），不做软删——超留存下限的数据按合规口径物理清除。
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// ===== 操作日志 =====
type IOperLogRepository interface {
	List(ctx context.Context, title, method, operName string, pageNum, pageSize int) (list []*model.OperLogListRes, total int64, err error)
	Get(ctx context.Context, operId string) (*model.OperLogInfoRes, error)
	Insert(ctx context.Context, log *entity.OperLog) error
	Delete(ctx context.Context, operIds []string) error
	Clear(ctx context.Context) error
	// PurgeBefore 留存清理专用：删除 oper_time 早于 cutoff 的行，返回实际删除数。
	// 与 aa_oper_log 自身的写入路径同规则（不允许绕开留存下限的其它删除面）。
	PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// ===== 在线用户 =====
type IUserOnlineRepository interface {
	List(ctx context.Context, username, ip string, pageNum, pageSize int) (list []*entity.UserOnline, total int64, err error)
	Delete(ctx context.Context, uuid string) error
}

// ===== 角色 =====
type IRoleRepository interface {
	List(ctx context.Context, name, status string, pageNum, pageSize int) (list []*entity.Role, total int64, err error)
	All(ctx context.Context) (list []*entity.Role, err error)
	Get(ctx context.Context, roleId string) (*entity.Role, error)
	Add(ctx context.Context, r *entity.Role) (string, error)
	Edit(ctx context.Context, r *entity.Role) error
	Delete(ctx context.Context, roleIds []string) error
	// casbin 角色-菜单策略
	BindMenus(ctx context.Context, roleId string, menuIds []string) error
	ClearMenus(ctx context.Context, roleId string) error
	GetMenuIds(ctx context.Context, roleId string) ([]string, error)
}
