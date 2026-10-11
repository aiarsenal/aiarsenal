package dto

import "aiarsenal/domain/entity"

// 项目管理模块请求/响应 DTO。

// ===== 通用 =====
type ProjectIdsReq struct {
	Ids []string `json:"ids" form:"ids"`
}

// ===== 项目 =====
type ProjectSearchReq struct {
	Name   string `json:"name" form:"name"`
	Status string `json:"status" form:"status"`
	PageReq
}
type ProjectSearchRes struct {
	ListRes
	List []*entity.Project `json:"list"`
}
type ProjectReq struct {
	Name        string   `json:"name" form:"name" binding:"required"`
	Code        string   `json:"code" form:"code"`
	Description string   `json:"description" form:"description"`
	Status      string   `json:"status" form:"status"`
	Priority    string   `json:"priority" form:"priority"`
	Progress    int      `json:"progress" form:"progress"`
	Owner       string   `json:"owner" form:"owner"`
	Visibility  string   `json:"visibility" form:"visibility"`
	DeptId      string   `json:"deptId" form:"deptId"`
	StartDate   string   `json:"startDate" form:"startDate"`
	DueDate     string   `json:"dueDate" form:"dueDate"`
	Color       string   `json:"color" form:"color"`
	Tags        string   `json:"tags" form:"tags"`
	Settings    string   `json:"settings" form:"settings"`
	Archived    uint     `json:"archived" form:"archived"`
	TagList     []string `json:"tagList" form:"tagList"` // 便捷：标签数组，service 转 tags jsonb
}
type ProjectAddReq struct{ *ProjectReq }
type ProjectEditReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	*ProjectReq
}

// ===== 成员 =====
type ProjectMemberSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	Role      string `json:"role" form:"role"`
	PageReq
}
type ProjectMemberSearchRes struct {
	ListRes
	List []*entity.ProjectMember `json:"list"`
}
type ProjectMemberReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	UserId    string `json:"userId" form:"userId" binding:"required"`
	Role      string `json:"role" form:"role"`
}
type ProjectMemberAddReq struct{ *ProjectMemberReq }
type ProjectMemberEditReq struct {
	MemberId string `json:"memberId" form:"memberId" binding:"required"`
	Role     string `json:"role" form:"role" binding:"required"`
}

// ===== 任务 =====
type ProjectTaskSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	Status    string `json:"status" form:"status"`
	Assignee  string `json:"assignee" form:"assignee"`
	Priority  string `json:"priority" form:"priority"`
	PageReq
}
type ProjectTaskSearchRes struct {
	ListRes
	List []*entity.ProjectTask `json:"list"`
}
type ProjectTaskReq struct {
	ProjectId     string  `json:"projectId" form:"projectId" binding:"required"`
	ParentTaskId  string  `json:"parentTaskId" form:"parentTaskId"`
	MilestoneId   string  `json:"milestoneId" form:"milestoneId"`
	Title         string  `json:"title" form:"title" binding:"required"`
	Description   string  `json:"description" form:"description"`
	Status        string  `json:"status" form:"status"`
	Priority      string  `json:"priority" form:"priority"`
	AssigneeId    string  `json:"assigneeId" form:"assigneeId"`
	ReporterId    string  `json:"reporterId" form:"reporterId"`
	StartDate     string  `json:"startDate" form:"startDate"`
	DueDate       string  `json:"dueDate" form:"dueDate"`
	Tags          string  `json:"tags" form:"tags"`
	CustomFields  string  `json:"customFields" form:"customFields"`
	EstimateHours float64 `json:"estimateHours" form:"estimateHours"`
	SpentHours    float64 `json:"spentHours" form:"spentHours"`
}
type ProjectTaskAddReq struct{ *ProjectTaskReq }
type ProjectTaskEditReq struct {
	TaskId string `json:"taskId" form:"taskId" binding:"required"`
	*ProjectTaskReq
}
type ProjectTaskMoveReq struct {
	TaskId    string `json:"taskId" form:"taskId" binding:"required"`
	Status    string `json:"status" form:"status" binding:"required"`
	SortOrder int    `json:"sortOrder" form:"sortOrder"`
}

// ===== 任务评论 =====
type ProjectTaskCommentListRes struct {
	List []*entity.ProjectTaskComment `json:"list"`
}
type ProjectTaskCommentAddReq struct {
	TaskId          string `json:"taskId" form:"taskId" binding:"required"`
	ProjectId       string `json:"projectId" form:"projectId"`
	Content         string `json:"content" form:"content" binding:"required"`
	ParentCommentId string `json:"parentCommentId" form:"parentCommentId"`
	Mentioned       string `json:"mentioned" form:"mentioned"`
}

// ===== 动态 =====
type ProjectActivitySearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	TaskId    string `json:"taskId" form:"taskId"`
	Action    string `json:"action" form:"action"`
	PageReq
}
type ProjectActivitySearchRes struct {
	ListRes
	List []*entity.ProjectTaskActivity `json:"list"`
}

// ===== 里程碑 =====
type ProjectMilestoneSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	Status    string `json:"status" form:"status"`
	PageReq
}
type ProjectMilestoneSearchRes struct {
	ListRes
	List []*entity.ProjectMilestone `json:"list"`
}
type ProjectMilestoneReq struct {
	ProjectId   string `json:"projectId" form:"projectId" binding:"required"`
	Name        string `json:"name" form:"name" binding:"required"`
	Description string `json:"description" form:"description"`
	Status      string `json:"status" form:"status"`
	StartDate   string `json:"startDate" form:"startDate"`
	DueDate     string `json:"dueDate" form:"dueDate"`
	SortOrder   int    `json:"sortOrder" form:"sortOrder"`
}
type ProjectMilestoneAddReq struct{ *ProjectMilestoneReq }
type ProjectMilestoneEditReq struct {
	MilestoneId string `json:"milestoneId" form:"milestoneId" binding:"required"`
	*ProjectMilestoneReq
}

// ===== 标签 =====
type ProjectTagSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	PageReq
}
type ProjectTagSearchRes struct {
	ListRes
	List []*entity.ProjectTag `json:"list"`
}
type ProjectTagReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	Name      string `json:"name" form:"name" binding:"required"`
	Color     string `json:"color" form:"color"`
	SortOrder int    `json:"sortOrder" form:"sortOrder"`
}
type ProjectTagAddReq struct{ *ProjectTagReq }
type ProjectTagEditReq struct {
	TagId string `json:"tagId" form:"tagId" binding:"required"`
	*ProjectTagReq
}

// ===== 状态流 =====
type ProjectStatusSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	PageReq
}
type ProjectStatusSearchRes struct {
	ListRes
	List []*entity.ProjectStatus `json:"list"`
}
type ProjectStatusReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	Key       string `json:"key" form:"key" binding:"required"`
	Name      string `json:"name" form:"name" binding:"required"`
	Color     string `json:"color" form:"color"`
	Category  string `json:"category" form:"category"`
	SortOrder int    `json:"sortOrder" form:"sortOrder"`
	IsDefault uint   `json:"isDefault" form:"isDefault"`
}
type ProjectStatusAddReq struct{ *ProjectStatusReq }
type ProjectStatusEditReq struct {
	StatusId string `json:"statusId" form:"statusId" binding:"required"`
	*ProjectStatusReq
}

// ===== 自定义字段 =====
type ProjectCustomFieldSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	PageReq
}
type ProjectCustomFieldSearchRes struct {
	ListRes
	List []*entity.ProjectCustomField `json:"list"`
}
type ProjectCustomFieldReq struct {
	ProjectId string `json:"projectId" form:"projectId" binding:"required"`
	Name      string `json:"name" form:"name" binding:"required"`
	Type      string `json:"type" form:"type" binding:"required"`
	Options   string `json:"options" form:"options"`
	SortOrder int    `json:"sortOrder" form:"sortOrder"`
}
type ProjectCustomFieldAddReq struct{ *ProjectCustomFieldReq }
type ProjectCustomFieldEditReq struct {
	FieldId string `json:"fieldId" form:"fieldId" binding:"required"`
	*ProjectCustomFieldReq
}

// ===== 资源绑定 =====
type ProjectBindingSearchReq struct {
	ProjectId string `json:"projectId" form:"projectId"`
	Kind      string `json:"kind" form:"kind"`
	PageReq
}
type ProjectBindingSearchRes struct {
	ListRes
	List []*entity.ProjectBinding `json:"list"`
}
type ProjectBindingReq struct {
	ProjectId  string `json:"projectId" form:"projectId" binding:"required"`
	Kind       string `json:"kind" form:"kind" binding:"required"`
	TargetId   string `json:"targetId" form:"targetId" binding:"required"`
	TargetName string `json:"targetName" form:"targetName"`
	Config     string `json:"config" form:"config"`
	Enabled    uint   `json:"enabled" form:"enabled"`
}
type ProjectBindingAddReq struct{ *ProjectBindingReq }
type ProjectBindingEditReq struct {
	BindingId string `json:"bindingId" form:"bindingId" binding:"required"`
	*ProjectBindingReq
}

// ===== 看板（聚合视图）=====
type ProjectBoardRes struct {
	Columns []*entity.ProjectStatus `json:"columns"`
	Tasks   []*entity.ProjectTask   `json:"tasks"`
}

// ===== 时间线（简化 v1，任务起止条带）=====
type ProjectTimelineRes struct {
	List []*entity.ProjectTask `json:"list"`
}
