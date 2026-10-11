package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// MCP 模块仓储实现（mcp 库）。统一走 db.McpDB()。
// 写入时 idgen.New() 生成业务 id；软删表 Where deleted_at is null；删除=置 deleted_at。

// ===================== Server =====================

type McpServerRepository struct{ db *gorm.DB }

func NewMcpServerRepository() *McpServerRepository { return &McpServerRepository{db: db.McpDB()} }

func (r *McpServerRepository) List(ctx context.Context, name, typ string, pageNum, pageSize int) (list []*entity.McpServer, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpServer{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ? or description like ?", "%"+name+"%", "%"+name+"%")
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpServerRepository) All(ctx context.Context) (list []*entity.McpServer, err error) {
	err = r.db.WithContext(ctx).Where("deleted_at is null and enabled = 1").Order("id desc").Find(&list).Error
	return
}

func (r *McpServerRepository) Get(ctx context.Context, id string) (*entity.McpServer, error) {
	var e entity.McpServer
	if err := r.db.WithContext(ctx).Where("server_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpServerRepository) GetByName(ctx context.Context, name string) (*entity.McpServer, error) {
	var e entity.McpServer
	if err := r.db.WithContext(ctx).Where("name = ? and deleted_at is null", name).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpServerRepository) Add(ctx context.Context, e *entity.McpServer) (string, error) {
	e.ServerId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ServerId, nil
}

func (r *McpServerRepository) Edit(ctx context.Context, e *entity.McpServer) error {
	return r.db.WithContext(ctx).Model(&entity.McpServer{}).Where("server_id = ?", e.ServerId).Updates(e).Error
}

func (r *McpServerRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpServer{}).
		Where("server_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Group =====================

type McpGroupRepository struct{ db *gorm.DB }

func NewMcpGroupRepository() *McpGroupRepository { return &McpGroupRepository{db: db.McpDB()} }

func (r *McpGroupRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpGroup, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpGroup{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ? or description like ?", "%"+name+"%", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpGroupRepository) All(ctx context.Context) (list []*entity.McpGroup, err error) {
	err = r.db.WithContext(ctx).Where("deleted_at is null").Order("id desc").Find(&list).Error
	return
}

func (r *McpGroupRepository) Get(ctx context.Context, id string) (*entity.McpGroup, error) {
	var e entity.McpGroup
	if err := r.db.WithContext(ctx).Where("group_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpGroupRepository) GetByName(ctx context.Context, name string) (*entity.McpGroup, error) {
	var e entity.McpGroup
	if err := r.db.WithContext(ctx).Where("name = ? and deleted_at is null", name).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpGroupRepository) Add(ctx context.Context, e *entity.McpGroup) (string, error) {
	e.GroupId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.GroupId, nil
}

func (r *McpGroupRepository) Edit(ctx context.Context, e *entity.McpGroup) error {
	return r.db.WithContext(ctx).Model(&entity.McpGroup{}).Where("group_id = ?", e.GroupId).Updates(e).Error
}

func (r *McpGroupRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpGroup{}).
		Where("group_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// SetVisibility 分组可见性档位（0私有 1按条件 2全员）——map 更新绕开 Edit 零值跳过。
func (r *McpGroupRepository) SetVisibility(ctx context.Context, groupId string, visibility uint) error {
	return r.db.WithContext(ctx).Model(&entity.McpGroup{}).
		Where("group_id = ?", groupId).
		Updates(map[string]interface{}{"visibility": visibility, "updated_at": gorm.Expr("now()")}).Error
}

// mcpGroupReadPredicate 分组「可使用」读谓词（SQL 形态；照 kbReadPredicate，与 service/mcp_group_access.go 的 canUseGroup 逐条对齐）：
//
//	(owner=uid ∨ created_by=uid ∨ visibility=2 ∨ ∃条件行命中)
//
// 行命中 = 各维度「空=不限 ∨ 命中」全 AND（含权限分组/员工集合引用门：t.Groups/t.Sets 已由
// grantmeta 实时解析，引用行下推为精确判定，SQL 即权威、无 Go 复核），再与 exclude 异或（<>）。
// 与 aa_kb_grant 的差异：无 target_type/action 维度（单动作），目标列 = g.target_id → 分组 group_id。
// jsonb 存在性判定用函数形 jsonb_exists/jsonb_exists_any——操作符形 ? 会被 gorm 当占位符吃掉。
// 身份维度为空（无部门/角色/岗位/用户）时对应维度只允许「不限」行命中，与 Go Hit 一致。
// SQL 无超管臂（与 KB 一致，超管短路在 service 层）。
func mcpGroupReadPredicate(alias string, t model.Identity) (string, []interface{}) {
	args := make([]interface{}, 0, 10)
	userArm := "g.user_ids = '[]'"
	if t.UserId != "" {
		userArm = "(g.user_ids = '[]' or jsonb_exists(g.user_ids, ?))"
		args = append(args, t.UserId)
	}
	deptArm := "g.dept_id = ''"
	if t.DeptId != "" {
		deptArm = "(g.dept_id = '' or g.dept_id = ?)"
		args = append(args, t.DeptId)
	}
	roleArm := "g.role_ids = '[]'"
	if len(t.RoleIds) > 0 {
		ph := make([]string, 0, len(t.RoleIds))
		for _, id := range t.RoleIds {
			ph = append(ph, "?")
			args = append(args, id)
		}
		roleArm = "(g.role_ids = '[]' or jsonb_exists_any(g.role_ids, array[" + strings.Join(ph, ",") + "]))"
	}
	postArm := "g.post_ids = '[]'"
	if len(t.PostIds) > 0 {
		ph := make([]string, 0, len(t.PostIds))
		for _, id := range t.PostIds {
			ph = append(ph, "?")
			args = append(args, id)
		}
		postArm = "(g.post_ids = '[]' or jsonb_exists_any(g.post_ids, array[" + strings.Join(ph, ",") + "]))"
	}
	groupGate := "g.group_id = ''"
	if len(t.Groups) > 0 {
		ph := make([]string, 0, len(t.Groups))
		for _, id := range t.Groups {
			ph = append(ph, "?")
			args = append(args, id)
		}
		groupGate = "(g.group_id = '' or g.group_id in (" + strings.Join(ph, ",") + "))"
	}
	setGate := "g.set_id = ''"
	if len(t.Sets) > 0 {
		ph := make([]string, 0, len(t.Sets))
		for _, id := range t.Sets {
			ph = append(ph, "?")
			args = append(args, id)
		}
		setGate = "(g.set_id = '' or g.set_id in (" + strings.Join(ph, ",") + "))"
	}
	hit := "(" + userArm + " and " + deptArm + " and " + roleArm + " and " + postArm +
		" and " + groupGate + " and " + setGate + ")"
	exists := fmt.Sprintf(
		"exists (select 1 from aa_mcp_group_grant g where g.target_id = %s.group_id and %s <> g.exclude)",
		alias, hit)
	cond := fmt.Sprintf("(%s.owner = ? or %s.created_by = ? or %s.visibility = 2 or %s)",
		alias, alias, alias, exists)
	args = append([]interface{}{t.UserId, t.UserId}, args...) // owner/created_by 参数在最前
	return cond, args
}

// ListVisible 带身份过滤的分页列表（t nil 时等同 List）。
func (r *McpGroupRepository) ListVisible(ctx context.Context, name string, pageNum, pageSize int, t *model.Identity) (list []*entity.McpGroup, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpGroup{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ? or description like ?", "%"+name+"%", "%"+name+"%")
	}
	if t != nil {
		cond, args := mcpGroupReadPredicate("aa_mcp_group", *t)
		q = q.Where(cond, args...)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// AllFor 带身份过滤的全量（t nil 时等同 All）。
func (r *McpGroupRepository) AllFor(ctx context.Context, t *model.Identity) (list []*entity.McpGroup, err error) {
	q := r.db.WithContext(ctx).Where("deleted_at is null")
	if t != nil {
		cond, args := mcpGroupReadPredicate("aa_mcp_group", *t)
		q = q.Where(cond, args...)
	}
	err = q.Order("id desc").Find(&list).Error
	return
}

// ===================== BearerKey =====================

type McpBearerKeyRepository struct{ db *gorm.DB }

func NewMcpBearerKeyRepository() *McpBearerKeyRepository {
	return &McpBearerKeyRepository{db: db.McpDB()}
}

func (r *McpBearerKeyRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBearerKey, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpBearerKey{})
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpBearerKeyRepository) Get(ctx context.Context, id string) (*entity.McpBearerKey, error) {
	var e entity.McpBearerKey
	if err := r.db.WithContext(ctx).Where("key_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpBearerKeyRepository) GetByHash(ctx context.Context, tokenHash string) (*entity.McpBearerKey, error) {
	var e entity.McpBearerKey
	if err := r.db.WithContext(ctx).Where("token_hash = ? and enabled = 1", tokenHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpBearerKeyRepository) Add(ctx context.Context, e *entity.McpBearerKey) (string, error) {
	e.KeyId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.KeyId, nil
}

func (r *McpBearerKeyRepository) Edit(ctx context.Context, e *entity.McpBearerKey) error {
	return r.db.WithContext(ctx).Model(&entity.McpBearerKey{}).Where("key_id = ?", e.KeyId).Updates(e).Error
}

func (r *McpBearerKeyRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpBearerKey{}).Where("key_id in ?", ids).Delete(nil).Error
}

// ===================== Activity =====================

type McpActivityRepository struct{ db *gorm.DB }

func NewMcpActivityRepository() *McpActivityRepository { return &McpActivityRepository{db: db.McpDB()} }

func (r *McpActivityRepository) List(ctx context.Context, serverName, tool, status, userId string, dateRange []string, pageNum, pageSize int) (list []*entity.McpActivity, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpActivity{})
	if serverName != "" {
		q = q.Where("server_name = ?", serverName)
	}
	if tool != "" {
		q = q.Where("tool like ?", "%"+tool+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if len(dateRange) == 2 {
		q = q.Where("ts >= ? and ts <= ?", dateRange[0], dateRange[1])
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpActivityRepository) Add(ctx context.Context, e *entity.McpActivity) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// ===================== BuiltinPrompt =====================

type McpBuiltinPromptRepository struct{ db *gorm.DB }

func NewMcpBuiltinPromptRepository() *McpBuiltinPromptRepository {
	return &McpBuiltinPromptRepository{db: db.McpDB()}
}

func (r *McpBuiltinPromptRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBuiltinPrompt, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpBuiltinPrompt{})
	if name != "" {
		q = q.Where("name like ? or title like ?", "%"+name+"%", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpBuiltinPromptRepository) All(ctx context.Context) (list []*entity.McpBuiltinPrompt, err error) {
	err = r.db.WithContext(ctx).Where("enabled = 1").Order("id desc").Find(&list).Error
	return
}

func (r *McpBuiltinPromptRepository) Get(ctx context.Context, id string) (*entity.McpBuiltinPrompt, error) {
	var e entity.McpBuiltinPrompt
	if err := r.db.WithContext(ctx).Where("prompt_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpBuiltinPromptRepository) Add(ctx context.Context, e *entity.McpBuiltinPrompt) (string, error) {
	e.PromptId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.PromptId, nil
}

func (r *McpBuiltinPromptRepository) Edit(ctx context.Context, e *entity.McpBuiltinPrompt) error {
	return r.db.WithContext(ctx).Model(&entity.McpBuiltinPrompt{}).Where("prompt_id = ?", e.PromptId).Updates(e).Error
}

func (r *McpBuiltinPromptRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpBuiltinPrompt{}).Where("prompt_id in ?", ids).Delete(nil).Error
}

// ===================== BuiltinResource =====================

type McpBuiltinResourceRepository struct{ db *gorm.DB }

func NewMcpBuiltinResourceRepository() *McpBuiltinResourceRepository {
	return &McpBuiltinResourceRepository{db: db.McpDB()}
}

func (r *McpBuiltinResourceRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.McpBuiltinResource, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.McpBuiltinResource{})
	if name != "" {
		q = q.Where("name like ? or uri like ?", "%"+name+"%", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *McpBuiltinResourceRepository) All(ctx context.Context) (list []*entity.McpBuiltinResource, err error) {
	err = r.db.WithContext(ctx).Where("enabled = 1").Order("id desc").Find(&list).Error
	return
}

func (r *McpBuiltinResourceRepository) Get(ctx context.Context, id string) (*entity.McpBuiltinResource, error) {
	var e entity.McpBuiltinResource
	if err := r.db.WithContext(ctx).Where("resource_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpBuiltinResourceRepository) Add(ctx context.Context, e *entity.McpBuiltinResource) (string, error) {
	e.ResourceId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ResourceId, nil
}

func (r *McpBuiltinResourceRepository) Edit(ctx context.Context, e *entity.McpBuiltinResource) error {
	return r.db.WithContext(ctx).Model(&entity.McpBuiltinResource{}).Where("resource_id = ?", e.ResourceId).Updates(e).Error
}

func (r *McpBuiltinResourceRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.McpBuiltinResource{}).Where("resource_id in ?", ids).Delete(nil).Error
}

// ===================== Config（单例） =====================

type McpConfigRepository struct{ db *gorm.DB }

func NewMcpConfigRepository() *McpConfigRepository { return &McpConfigRepository{db: db.McpDB()} }

func (r *McpConfigRepository) Get(ctx context.Context) (*entity.McpConfig, error) {
	var e entity.McpConfig
	if err := r.db.WithContext(ctx).Where("id = 'default'").First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpConfigRepository) Save(ctx context.Context, e *entity.McpConfig) error {
	e.Id = "default"
	// UPSERT：存在则更新，不存在则插入
	return r.db.WithContext(ctx).Save(e).Error
}

// ===================== UserConfig =====================

type McpUserConfigRepository struct{ db *gorm.DB }

func NewMcpUserConfigRepository() *McpUserConfigRepository {
	return &McpUserConfigRepository{db: db.McpDB()}
}

func (r *McpUserConfigRepository) Get(ctx context.Context, userId string) (*entity.McpUserConfig, error) {
	var e entity.McpUserConfig
	if err := r.db.WithContext(ctx).Where("user_id = ?", userId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *McpUserConfigRepository) Save(ctx context.Context, e *entity.McpUserConfig) error {
	// 按 user_id UPSERT
	var exist entity.McpUserConfig
	err := r.db.WithContext(ctx).Where("user_id = ?", e.UserId).First(&exist).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r.db.WithContext(ctx).Create(e).Error
		}
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.McpUserConfig{}).Where("user_id = ?", e.UserId).
		Updates(map[string]interface{}{
			"routing":           e.Routing,
			"additional_config": e.AdditionalConfig,
			"updated_at":        time.Now(),
		}).Error
}
