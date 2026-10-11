package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// 项目管理模块仓储实现（aa_project 库）。统一走 db.ProjectDB()。
// 写入时 idgen.New() 生成业务 id；*types.Time 不自动回填，显式盖 created_at/updated_at；
// 软删表 Where deleted_at is null；删除=置 deleted_at；Edit 用显式 map 避免 GORM 丢零值（progress=0/enabled=0 等）。

func nowPtr() *types.Time { t := types.Time(time.Now()); return &t }

// ===================== Project =====================

type ProjectRepository struct{ db *gorm.DB }

func NewProjectRepository() *ProjectRepository { return &ProjectRepository{db: db.ProjectDB()} }

func (r *ProjectRepository) List(ctx context.Context, name, status string, pageNum, pageSize int) (list []*entity.Project, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.Project{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ? or code like ? or description like ?", "%"+name+"%", "%"+name+"%", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectRepository) All(ctx context.Context) (list []*entity.Project, err error) {
	// 选择器用：未归档项目，按 id desc
	err = r.db.WithContext(ctx).Where("deleted_at is null and archived = 0").Order("id desc").Find(&list).Error
	return
}

func (r *ProjectRepository) Get(ctx context.Context, id string) (*entity.Project, error) {
	var e entity.Project
	if err := r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectRepository) Add(ctx context.Context, e *entity.Project) (string, error) {
	e.ProjectId = idgen.New()
	now := nowPtr()
	if e.CreatedAt == nil {
		e.CreatedAt = now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ProjectId, nil
}

func (r *ProjectRepository) Edit(ctx context.Context, e *entity.Project) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.Project{}).Where("project_id = ?", e.ProjectId).Updates(map[string]any{
		"name":        e.Name,
		"code":        e.Code,
		"description": e.Description,
		"status":      e.Status,
		"priority":    e.Priority,
		"progress":    e.Progress,
		"owner":       e.Owner,
		"visibility":  e.Visibility,
		"dept_id":     e.DeptId,
		"start_date":  e.StartDate,
		"due_date":    e.DueDate,
		"color":       e.Color,
		"tags":        e.Tags,
		"settings":    e.Settings,
		"archived":    e.Archived,
		"updated_by":  e.UpdatedBy,
		"updated_at":  e.UpdatedAt,
	}).Error
}

func (r *ProjectRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.Project{}).
		Where("project_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Member =====================

type ProjectMemberRepository struct{ db *gorm.DB }

func NewProjectMemberRepository() *ProjectMemberRepository {
	return &ProjectMemberRepository{db: db.ProjectDB()}
}

func (r *ProjectMemberRepository) List(ctx context.Context, projectID, role string, pageNum, pageSize int) (list []*entity.ProjectMember, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectMember{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectMemberRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectMember, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).Order("id asc").Find(&list).Error
	return
}

func (r *ProjectMemberRepository) Get(ctx context.Context, id string) (*entity.ProjectMember, error) {
	var e entity.ProjectMember
	if err := r.db.WithContext(ctx).Where("member_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectMemberRepository) GetByProjectUser(ctx context.Context, projectID, userID string) (*entity.ProjectMember, error) {
	var e entity.ProjectMember
	if err := r.db.WithContext(ctx).Where("project_id = ? and user_id = ? and deleted_at is null", projectID, userID).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectMemberRepository) Add(ctx context.Context, e *entity.ProjectMember) (string, error) {
	e.MemberId = idgen.New()
	now := nowPtr()
	if e.JoinedAt == nil {
		e.JoinedAt = now
	}
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.MemberId, nil
}

func (r *ProjectMemberRepository) Edit(ctx context.Context, e *entity.ProjectMember) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectMember{}).Where("member_id = ?", e.MemberId).Updates(map[string]any{
		"role":       e.Role,
		"updated_at": e.UpdatedAt,
	}).Error
}

func (r *ProjectMemberRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectMember{}).
		Where("member_id in ?", ids).Update("deleted_at", time.Now()).Error
}

func (r *ProjectMemberRepository) DeleteByProject(ctx context.Context, projectID string) error {
	return r.db.WithContext(ctx).Model(&entity.ProjectMember{}).
		Where("project_id = ?", projectID).Update("deleted_at", time.Now()).Error
}

// ===================== Task =====================

type ProjectTaskRepository struct{ db *gorm.DB }

func NewProjectTaskRepository() *ProjectTaskRepository {
	return &ProjectTaskRepository{db: db.ProjectDB()}
}

func (r *ProjectTaskRepository) List(ctx context.Context, projectID, status, assignee, priority string, pageNum, pageSize int) (list []*entity.ProjectTask, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectTask{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if assignee != "" {
		q = q.Where("assignee_id = ?", assignee)
	}
	if priority != "" {
		q = q.Where("priority = ?", priority)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectTaskRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectTask, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).
		Order("sort_order asc, id desc").Find(&list).Error
	return
}

func (r *ProjectTaskRepository) ListByMilestone(ctx context.Context, projectID, milestoneID string) (list []*entity.ProjectTask, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and milestone_id = ? and deleted_at is null", projectID, milestoneID).
		Order("id desc").Find(&list).Error
	return
}

func (r *ProjectTaskRepository) Get(ctx context.Context, id string) (*entity.ProjectTask, error) {
	var e entity.ProjectTask
	if err := r.db.WithContext(ctx).Where("task_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectTaskRepository) Add(ctx context.Context, e *entity.ProjectTask) (string, error) {
	e.TaskId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TaskId, nil
}

func (r *ProjectTaskRepository) Edit(ctx context.Context, e *entity.ProjectTask) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectTask{}).Where("task_id = ?", e.TaskId).Updates(map[string]any{
		"parent_task_id": e.ParentTaskId,
		"milestone_id":   e.MilestoneId,
		"title":          e.Title,
		"description":    e.Description,
		"status":         e.Status,
		"priority":       e.Priority,
		"assignee_id":    e.AssigneeId,
		"reporter_id":    e.ReporterId,
		"start_date":     e.StartDate,
		"due_date":       e.DueDate,
		"done_at":        e.DoneAt,
		"sort_order":     e.SortOrder,
		"tags":           e.Tags,
		"custom_fields":  e.CustomFields,
		"estimate_hours": e.EstimateHours,
		"spent_hours":    e.SpentHours,
		"updated_by":     e.UpdatedBy,
		"updated_at":     e.UpdatedAt,
	}).Error
}

func (r *ProjectTaskRepository) UpdateStatus(ctx context.Context, taskID, status string, sortOrder int) error {
	return r.db.WithContext(ctx).Model(&entity.ProjectTask{}).Where("task_id = ?", taskID).
		Updates(map[string]any{
			"status":     status,
			"sort_order": sortOrder,
			"updated_at": time.Now(),
			"done_at":    statusDoneAt(status),
		}).Error
}

// statusDoneAt 进入 done 类状态时回填完成时间，否则清空（nil → NULL）。
func statusDoneAt(status string) *time.Time {
	if status == "done" || status == "closed" {
		t := time.Now()
		return &t
	}
	return nil
}

func (r *ProjectTaskRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectTask{}).
		Where("task_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== TaskComment =====================

type ProjectTaskCommentRepository struct{ db *gorm.DB }

func NewProjectTaskCommentRepository() *ProjectTaskCommentRepository {
	return &ProjectTaskCommentRepository{db: db.ProjectDB()}
}

func (r *ProjectTaskCommentRepository) List(ctx context.Context, taskID string) (list []*entity.ProjectTaskComment, err error) {
	err = r.db.WithContext(ctx).Where("task_id = ? and deleted_at is null", taskID).Order("id asc").Find(&list).Error
	return
}

func (r *ProjectTaskCommentRepository) Add(ctx context.Context, e *entity.ProjectTaskComment) (string, error) {
	e.CommentId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.CommentId, nil
}

func (r *ProjectTaskCommentRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectTaskComment{}).
		Where("comment_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== TaskActivity（append-only）=====================

type ProjectTaskActivityRepository struct{ db *gorm.DB }

func NewProjectTaskActivityRepository() *ProjectTaskActivityRepository {
	return &ProjectTaskActivityRepository{db: db.ProjectDB()}
}

func (r *ProjectTaskActivityRepository) List(ctx context.Context, projectID, taskID, action string, pageNum, pageSize int) (list []*entity.ProjectTaskActivity, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectTaskActivity{})
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectTaskActivityRepository) Add(ctx context.Context, e *entity.ProjectTaskActivity) (string, error) {
	e.ActivityId = idgen.New()
	e.CreatedAt = nowPtr()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ActivityId, nil
}

// ===================== Milestone =====================

type ProjectMilestoneRepository struct{ db *gorm.DB }

func NewProjectMilestoneRepository() *ProjectMilestoneRepository {
	return &ProjectMilestoneRepository{db: db.ProjectDB()}
}

func (r *ProjectMilestoneRepository) List(ctx context.Context, projectID, status string, pageNum, pageSize int) (list []*entity.ProjectMilestone, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectMilestone{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectMilestoneRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectMilestone, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).
		Order("sort_order asc, id desc").Find(&list).Error
	return
}

func (r *ProjectMilestoneRepository) Get(ctx context.Context, id string) (*entity.ProjectMilestone, error) {
	var e entity.ProjectMilestone
	if err := r.db.WithContext(ctx).Where("milestone_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectMilestoneRepository) Add(ctx context.Context, e *entity.ProjectMilestone) (string, error) {
	e.MilestoneId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.MilestoneId, nil
}

func (r *ProjectMilestoneRepository) Edit(ctx context.Context, e *entity.ProjectMilestone) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectMilestone{}).Where("milestone_id = ?", e.MilestoneId).Updates(map[string]any{
		"name":        e.Name,
		"description": e.Description,
		"status":      e.Status,
		"start_date":  e.StartDate,
		"due_date":    e.DueDate,
		"sort_order":  e.SortOrder,
		"updated_at":  e.UpdatedAt,
	}).Error
}

func (r *ProjectMilestoneRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectMilestone{}).
		Where("milestone_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Tag =====================

type ProjectTagRepository struct{ db *gorm.DB }

func NewProjectTagRepository() *ProjectTagRepository {
	return &ProjectTagRepository{db: db.ProjectDB()}
}

func (r *ProjectTagRepository) List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectTag, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectTag{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectTagRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectTag, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).
		Order("sort_order asc, id desc").Find(&list).Error
	return
}

func (r *ProjectTagRepository) Get(ctx context.Context, id string) (*entity.ProjectTag, error) {
	var e entity.ProjectTag
	if err := r.db.WithContext(ctx).Where("tag_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectTagRepository) Add(ctx context.Context, e *entity.ProjectTag) (string, error) {
	e.TagId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TagId, nil
}

func (r *ProjectTagRepository) Edit(ctx context.Context, e *entity.ProjectTag) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectTag{}).Where("tag_id = ?", e.TagId).Updates(map[string]any{
		"name":       e.Name,
		"color":      e.Color,
		"sort_order": e.SortOrder,
		"updated_at": e.UpdatedAt,
	}).Error
}

func (r *ProjectTagRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectTag{}).
		Where("tag_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Status（状态流 + 看板列）=====================

type ProjectStatusRepository struct{ db *gorm.DB }

func NewProjectStatusRepository() *ProjectStatusRepository {
	return &ProjectStatusRepository{db: db.ProjectDB()}
}

func (r *ProjectStatusRepository) List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectStatus, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectStatus{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectStatusRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectStatus, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).
		Order("sort_order asc, id asc").Find(&list).Error
	return
}

func (r *ProjectStatusRepository) GetDefault(ctx context.Context, projectID string) (*entity.ProjectStatus, error) {
	var e entity.ProjectStatus
	// 优先 is_default=1，否则取该 project 第一个
	err := r.db.WithContext(ctx).Where("project_id = ? and is_default = 1 and deleted_at is null", projectID).First(&e).Error
	if err == nil {
		return &e, nil
	}
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).Order("sort_order asc, id asc").First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectStatusRepository) Get(ctx context.Context, id string) (*entity.ProjectStatus, error) {
	var e entity.ProjectStatus
	if err := r.db.WithContext(ctx).Where("status_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectStatusRepository) GetByKey(ctx context.Context, projectID, key string) (*entity.ProjectStatus, error) {
	var e entity.ProjectStatus
	if err := r.db.WithContext(ctx).Where("project_id = ? and key = ? and deleted_at is null", projectID, key).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectStatusRepository) Add(ctx context.Context, e *entity.ProjectStatus) (string, error) {
	e.StatusId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.StatusId, nil
}

func (r *ProjectStatusRepository) Edit(ctx context.Context, e *entity.ProjectStatus) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectStatus{}).Where("status_id = ?", e.StatusId).Updates(map[string]any{
		"key":        e.Key,
		"name":       e.Name,
		"color":      e.Color,
		"category":   e.Category,
		"sort_order": e.SortOrder,
		"is_default": e.IsDefault,
		"updated_at": e.UpdatedAt,
	}).Error
}

func (r *ProjectStatusRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectStatus{}).
		Where("status_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== CustomField =====================

type ProjectCustomFieldRepository struct{ db *gorm.DB }

func NewProjectCustomFieldRepository() *ProjectCustomFieldRepository {
	return &ProjectCustomFieldRepository{db: db.ProjectDB()}
}

func (r *ProjectCustomFieldRepository) List(ctx context.Context, projectID string, pageNum, pageSize int) (list []*entity.ProjectCustomField, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectCustomField{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectCustomFieldRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectCustomField, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).
		Order("sort_order asc, id asc").Find(&list).Error
	return
}

func (r *ProjectCustomFieldRepository) Get(ctx context.Context, id string) (*entity.ProjectCustomField, error) {
	var e entity.ProjectCustomField
	if err := r.db.WithContext(ctx).Where("field_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectCustomFieldRepository) Add(ctx context.Context, e *entity.ProjectCustomField) (string, error) {
	e.FieldId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.FieldId, nil
}

func (r *ProjectCustomFieldRepository) Edit(ctx context.Context, e *entity.ProjectCustomField) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectCustomField{}).Where("field_id = ?", e.FieldId).Updates(map[string]any{
		"name":       e.Name,
		"type":       e.Type,
		"options":    e.Options,
		"sort_order": e.SortOrder,
		"updated_at": e.UpdatedAt,
	}).Error
}

func (r *ProjectCustomFieldRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectCustomField{}).
		Where("field_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Binding =====================

type ProjectBindingRepository struct{ db *gorm.DB }

func NewProjectBindingRepository() *ProjectBindingRepository {
	return &ProjectBindingRepository{db: db.ProjectDB()}
}

func (r *ProjectBindingRepository) List(ctx context.Context, projectID, kind string, pageNum, pageSize int) (list []*entity.ProjectBinding, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ProjectBinding{}).Where("deleted_at is null")
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ProjectBindingRepository) ListByProject(ctx context.Context, projectID string) (list []*entity.ProjectBinding, err error) {
	err = r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", projectID).Order("id desc").Find(&list).Error
	return
}

func (r *ProjectBindingRepository) ListByTarget(ctx context.Context, kind, targetId string) (list []*entity.ProjectBinding, err error) {
	err = r.db.WithContext(ctx).Where("kind = ? and target_id = ? and deleted_at is null", kind, targetId).Order("id desc").Find(&list).Error
	return
}

func (r *ProjectBindingRepository) Get(ctx context.Context, id string) (*entity.ProjectBinding, error) {
	var e entity.ProjectBinding
	if err := r.db.WithContext(ctx).Where("binding_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ProjectBindingRepository) Add(ctx context.Context, e *entity.ProjectBinding) (string, error) {
	e.BindingId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.BindingId, nil
}

func (r *ProjectBindingRepository) Edit(ctx context.Context, e *entity.ProjectBinding) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ProjectBinding{}).Where("binding_id = ?", e.BindingId).Updates(map[string]any{
		"kind":        e.Kind,
		"target_id":   e.TargetId,
		"target_name": e.TargetName,
		"config":      e.Config,
		"enabled":     e.Enabled,
		"updated_at":  e.UpdatedAt,
	}).Error
}

func (r *ProjectBindingRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ProjectBinding{}).
		Where("binding_id in ?", ids).Update("deleted_at", time.Now()).Error
}
