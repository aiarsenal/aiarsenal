package entity

import "aiarsenal/domain/types"

// 项目管理模块实体（aa_project 库，表前缀 aa_project_）。
// 设计见 docs/feature-overview.md 第三节；按 MVP 范围裁剪（时间线/报表为简化视图，跳过 Wiki/Worklog）。
// 约定与 mcp/llm 一致：id bigserial PK + <实体>_id varchar(32) UNIQUE（idgen 生成）；
//   时间 *types.Time（GORM 不自动回填，repo 层显式盖 created_at/updated_at；软删手动 deleted_at）；
//   跨库软引用 owner/assignee_id/reporter_id/user_id/dept_id → aiarsenal.aa_user.user_id（varchar(32) biz_id，无 FK）；
//   jsonb 列用 string + type:jsonb（自行解析）。

// ============ 项目（顶层资源容器）============
type Project struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ProjectId   string      `gorm:"column:project_id;uniqueIndex" json:"projectId"`
	Name        string      `gorm:"column:name" json:"name"`
	Code        string      `gorm:"column:code" json:"code"` // 项目代号/Key，项目内唯一可选
	Description string      `gorm:"column:description" json:"description"`
	Status      string      `gorm:"column:status" json:"status"`         // planning/active/paused/closed/archived
	Priority    string      `gorm:"column:priority" json:"priority"`     // low/medium/high/urgent
	Progress    int         `gorm:"column:progress" json:"progress"`     // 0-100
	Owner       string      `gorm:"column:owner" json:"owner"`           // 创建人/负责人 user_id
	Visibility  string      `gorm:"column:visibility" json:"visibility"` // private/dept/team/public
	DeptId      string      `gorm:"column:dept_id" json:"deptId"`
	StartDate   *types.Time `gorm:"column:start_date" json:"startDate"`
	DueDate     *types.Time `gorm:"column:due_date" json:"dueDate"`
	Color       string      `gorm:"column:color" json:"color"`
	Tags        string      `gorm:"column:tags;type:jsonb" json:"tags"`         // ["tag1","tag2"]
	Settings    string      `gorm:"column:settings;type:jsonb" json:"settings"` // 预留项目级设置
	Archived    uint        `gorm:"column:archived" json:"archived"`            // 0/1
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (Project) TableName() string { return "aa_project" }

// ============ 成员 ============
type ProjectMember struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	MemberId  string      `gorm:"column:member_id;uniqueIndex" json:"memberId"`
	ProjectId string      `gorm:"column:project_id" json:"projectId"`
	UserId    string      `gorm:"column:user_id" json:"userId"` // 软引用 aa_user.user_id
	Role      string      `gorm:"column:role" json:"role"`      // owner/maintainer/member/observer
	UserName  string      `gorm:"-" json:"userName"`            // 成员姓名（跨库回填 aa_user.name，展示用）
	JoinedAt  *types.Time `gorm:"column:joined_at" json:"joinedAt"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectMember) TableName() string { return "aa_project_member" }

// ============ 任务（含子任务：parent_task_id 非空即子任务）============
type ProjectTask struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TaskId        string      `gorm:"column:task_id;uniqueIndex" json:"taskId"`
	ProjectId     string      `gorm:"column:project_id" json:"projectId"`
	ParentTaskId  string      `gorm:"column:parent_task_id" json:"parentTaskId"` // 空=顶级任务
	MilestoneId   string      `gorm:"column:milestone_id" json:"milestoneId"`
	Title         string      `gorm:"column:title" json:"title"`
	Description   string      `gorm:"column:description" json:"description"`
	Status        string      `gorm:"column:status" json:"status"`     // 对应 aa_project_status.key
	Priority      string      `gorm:"column:priority" json:"priority"` // low/medium/high/urgent
	AssigneeId    string      `gorm:"column:assignee_id" json:"assigneeId"`
	AssigneeName  string      `gorm:"-" json:"assigneeName"` // 负责人姓名（跨库回填 aa_user.name，展示用）
	ReporterId    string      `gorm:"column:reporter_id" json:"reporterId"`
	StartDate     *types.Time `gorm:"column:start_date" json:"startDate"`
	DueDate       *types.Time `gorm:"column:due_date" json:"dueDate"`
	DoneAt        *types.Time `gorm:"column:done_at" json:"doneAt"`
	SortOrder     int         `gorm:"column:sort_order" json:"sortOrder"` // 看板列内排序
	Tags          string      `gorm:"column:tags;type:jsonb" json:"tags"`
	CustomFields  string      `gorm:"column:custom_fields;type:jsonb" json:"customFields"` // {fieldId:value}
	EstimateHours float64     `gorm:"column:estimate_hours;type:decimal(10,2)" json:"estimateHours"`
	SpentHours    float64     `gorm:"column:spent_hours;type:decimal(10,2)" json:"spentHours"`
	CreatedBy     string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy     string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectTask) TableName() string { return "aa_project_task" }

// ============ 任务评论 ============
type ProjectTaskComment struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	CommentId       string      `gorm:"column:comment_id;uniqueIndex" json:"commentId"`
	TaskId          string      `gorm:"column:task_id" json:"taskId"`
	ProjectId       string      `gorm:"column:project_id" json:"projectId"`
	UserId          string      `gorm:"column:user_id" json:"userId"`
	UserName        string      `gorm:"-" json:"userName"` // 评论人姓名（跨库回填，展示用）
	Content         string      `gorm:"column:content" json:"content"`
	Mentioned       string      `gorm:"column:mentioned;type:jsonb" json:"mentioned"`    // ["userId",...]
	ParentCommentId string      `gorm:"column:parent_comment_id" json:"parentCommentId"` // 回复
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectTaskComment) TableName() string { return "aa_project_task_comment" }

// ============ 动态（活动流，append-only）============
type ProjectTaskActivity struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ActivityId string      `gorm:"column:activity_id;uniqueIndex" json:"activityId"`
	ProjectId  string      `gorm:"column:project_id" json:"projectId"`
	TaskId     string      `gorm:"column:task_id" json:"taskId"`
	UserId     string      `gorm:"column:user_id" json:"userId"`
	Action     string      `gorm:"column:action" json:"action"`            // created/updated/commented/status_changed/assigned/archived
	Target     string      `gorm:"column:target" json:"target"`            // 受体标题，便于活动流直接展示
	Detail     string      `gorm:"column:detail;type:jsonb" json:"detail"` // 变更明细 {from,to,...}
	CreatedAt  *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (ProjectTaskActivity) TableName() string { return "aa_project_task_activity" }

// ============ 里程碑 ============
type ProjectMilestone struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	MilestoneId string      `gorm:"column:milestone_id;uniqueIndex" json:"milestoneId"`
	ProjectId   string      `gorm:"column:project_id" json:"projectId"`
	Name        string      `gorm:"column:name" json:"name"`
	Description string      `gorm:"column:description" json:"description"`
	Status      string      `gorm:"column:status" json:"status"` // open/in_progress/closed
	StartDate   *types.Time `gorm:"column:start_date" json:"startDate"`
	DueDate     *types.Time `gorm:"column:due_date" json:"dueDate"`
	SortOrder   int         `gorm:"column:sort_order" json:"sortOrder"`
	TaskTotal   int         `gorm:"-" json:"taskTotal"` // 里程碑下任务总数（汇总回填）
	TaskDone    int         `gorm:"-" json:"taskDone"`  // 已完成任务数
	Progress    int         `gorm:"-" json:"progress"`  // 完成率 0-100
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectMilestone) TableName() string { return "aa_project_milestone" }

// ============ 标签 ============
type ProjectTag struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TagId     string      `gorm:"column:tag_id;uniqueIndex" json:"tagId"`
	ProjectId string      `gorm:"column:project_id" json:"projectId"`
	Name      string      `gorm:"column:name" json:"name"`
	Color     string      `gorm:"column:color" json:"color"`
	SortOrder int         `gorm:"column:sort_order" json:"sortOrder"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectTag) TableName() string { return "aa_project_tag" }

// ============ 状态流（同时驱动看板列）============
type ProjectStatus struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	StatusId  string      `gorm:"column:status_id;uniqueIndex" json:"statusId"`
	ProjectId string      `gorm:"column:project_id" json:"projectId"`
	Key       string      `gorm:"column:key" json:"key"` // 稳定键，如 todo/doing/done（任务 status 存此）
	Name      string      `gorm:"column:name" json:"name"`
	Color     string      `gorm:"column:color" json:"color"`
	Category  string      `gorm:"column:category" json:"category"` // todo/doing/done
	SortOrder int         `gorm:"column:sort_order" json:"sortOrder"`
	IsDefault uint        `gorm:"column:is_default" json:"isDefault"` // 新建任务默认状态
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectStatus) TableName() string { return "aa_project_status" }

// ============ 自定义字段 ============
type ProjectCustomField struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	FieldId   string      `gorm:"column:field_id;uniqueIndex" json:"fieldId"`
	ProjectId string      `gorm:"column:project_id" json:"projectId"`
	Name      string      `gorm:"column:name" json:"name"`
	Type      string      `gorm:"column:type" json:"type"`                  // text/number/date/select/multiselect/user/url
	Options   string      `gorm:"column:options;type:jsonb" json:"options"` // select/multiselect 选项
	SortOrder int         `gorm:"column:sort_order" json:"sortOrder"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectCustomField) TableName() string { return "aa_project_custom_field" }

// ============ 资源绑定（项目=顶层 AI 资源容器）============
type ProjectBinding struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BindingId  string      `gorm:"column:binding_id;uniqueIndex" json:"bindingId"`
	ProjectId  string      `gorm:"column:project_id" json:"projectId"`
	Kind       string      `gorm:"column:kind" json:"kind"`                // agent/skill/kb/llm_budget/llm_key/mcp_group
	TargetId   string      `gorm:"column:target_id" json:"targetId"`       // 被绑定资源的 biz-id
	TargetName string      `gorm:"column:target_name" json:"targetName"`   // 缓存展示名
	Config     string      `gorm:"column:config;type:jsonb" json:"config"` // 绑定级参数
	Enabled    uint        `gorm:"column:enabled" json:"enabled"`
	CreatedBy  string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt  *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt  *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ProjectBinding) TableName() string { return "aa_project_binding" }
