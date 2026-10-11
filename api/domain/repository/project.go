package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// 项目管理模块仓储接口（aa_project 库）。软删表统一 Where deleted_at is null。

// ===== 项目 =====
type IProjectRepository interface {
	List(ctx context.Context, name, status string, pageNum, pageSize int) (list []*entity.Project, total int64, err error)
	All(ctx context.Context) (list []*entity.Project, err error)
	Get(ctx context.Context, id string) (*entity.Project, error)
	Add(ctx context.Context, e *entity.Project) (string, error)
	Edit(ctx context.Context, e *entity.Project) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 成员 =====
type IProjectMemberRepository interface {
	List(ctx context.Context, projectID, role string, pageNum, pageSize int) (list []*entity.ProjectMember, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectMember, err error)
	Get(ctx context.Context, id string) (*entity.ProjectMember, error)
	GetByProjectUser(ctx context.Context, projectID, userID string) (*entity.ProjectMember, error)
	Add(ctx context.Context, e *entity.ProjectMember) (string, error)
	Edit(ctx context.Context, e *entity.ProjectMember) error
	Delete(ctx context.Context, ids []string) error
	DeleteByProject(ctx context.Context, projectID string) error
}

// ===== 任务 =====
type IProjectTaskRepository interface {
	List(ctx context.Context, projectID, status, assignee, priority string, pageNum, pageSize int) (list []*entity.ProjectTask, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectTask, err error)
	ListByMilestone(ctx context.Context, projectID, milestoneID string) (list []*entity.ProjectTask, err error)
	Get(ctx context.Context, id string) (*entity.ProjectTask, error)
	Add(ctx context.Context, e *entity.ProjectTask) (string, error)
	Edit(ctx context.Context, e *entity.ProjectTask) error
	UpdateStatus(ctx context.Context, taskID, status string, sortOrder int) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 任务评论 =====
type IProjectTaskCommentRepository interface {
	List(ctx context.Context, taskID string) (list []*entity.ProjectTaskComment, err error)
	Add(ctx context.Context, e *entity.ProjectTaskComment) (string, error)
	Delete(ctx context.Context, ids []string) error
}

// ===== 动态（append-only）=====
type IProjectTaskActivityRepository interface {
	List(ctx context.Context, projectID, taskID, action string, pageNum, pageSize int) (list []*entity.ProjectTaskActivity, total int64, err error)
	Add(ctx context.Context, e *entity.ProjectTaskActivity) (string, error)
}

// ===== 里程碑 =====
type IProjectMilestoneRepository interface {
	List(ctx context.Context, projectID, status string, pageNum, pageSize int) (list []*entity.ProjectMilestone, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectMilestone, err error)
	Get(ctx context.Context, id string) (*entity.ProjectMilestone, error)
	Add(ctx context.Context, e *entity.ProjectMilestone) (string, error)
	Edit(ctx context.Context, e *entity.ProjectMilestone) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 标签 =====
type IProjectTagRepository interface {
	List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectTag, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectTag, err error)
	Get(ctx context.Context, id string) (*entity.ProjectTag, error)
	Add(ctx context.Context, e *entity.ProjectTag) (string, error)
	Edit(ctx context.Context, e *entity.ProjectTag) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 状态流 =====
type IProjectStatusRepository interface {
	List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectStatus, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectStatus, err error)
	GetDefault(ctx context.Context, projectID string) (*entity.ProjectStatus, error)
	Get(ctx context.Context, id string) (*entity.ProjectStatus, error)
	GetByKey(ctx context.Context, projectID, key string) (*entity.ProjectStatus, error)
	Add(ctx context.Context, e *entity.ProjectStatus) (string, error)
	Edit(ctx context.Context, e *entity.ProjectStatus) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 自定义字段 =====
type IProjectCustomFieldRepository interface {
	List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectCustomField, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectCustomField, err error)
	Get(ctx context.Context, id string) (*entity.ProjectCustomField, error)
	Add(ctx context.Context, e *entity.ProjectCustomField) (string, error)
	Edit(ctx context.Context, e *entity.ProjectCustomField) error
	Delete(ctx context.Context, ids []string) error
}

// ===== 资源绑定 =====
type IProjectBindingRepository interface {
	List(ctx context.Context, projectID, kind string, pageNum, pageSize int) (list []*entity.ProjectBinding, total int64, err error)
	ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectBinding, err error)
	ListByTarget(ctx context.Context, kind, targetId string) (list []*entity.ProjectBinding, err error) // 反查某资源被绑到哪些项目（软删过滤）
	Get(ctx context.Context, id string) (*entity.ProjectBinding, error)
	Add(ctx context.Context, e *entity.ProjectBinding) (string, error)
	Edit(ctx context.Context, e *entity.ProjectBinding) error
	Delete(ctx context.Context, ids []string) error
}
