package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// LLM 模块仓储实现（llm 库）。统一走 db.LLMDB()。
// 写入时 idgen.New() 生成业务 id；软删表 Where deleted_at is null；删除=置 deleted_at。

func paginate(pageNum, pageSize int) (int, int) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return (pageNum - 1) * pageSize, pageSize
}

// ===================== Credential =====================

type LLMCredentialRepository struct{ db *gorm.DB }

func NewLLMCredentialRepository() *LLMCredentialRepository {
	return &LLMCredentialRepository{db: db.LLMDB()}
}

func (r *LLMCredentialRepository) List(ctx context.Context, name, providerType string, pageNum, pageSize int) (list []*entity.LLMCredential, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMCredential{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("credential_name like ?", "%"+name+"%")
	}
	if providerType != "" {
		q = q.Where("provider_type = ?", providerType)
	}
	if err = q.Count(&total).Error; err != nil {
		slog.Error("凭据列表计数失败", "name", name, "provider", providerType, "err", err)
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	// 诊断「创建时间」空值：统计返回行里 created_at 为 NULL 的条数。
	emptyCreated := 0
	for _, c := range list {
		if c.CreatedAt == nil {
			emptyCreated++
		}
	}
	slog.Debug("凭据列表查询", "name", name, "provider", providerType, "pageNum", pageNum, "pageSize", pageSize, "total", total, "returned", len(list), "created_at_nil", emptyCreated, "err", err)
	return
}

func (r *LLMCredentialRepository) Get(ctx context.Context, id string) (*entity.LLMCredential, error) {
	var e entity.LLMCredential
	if err := r.db.WithContext(ctx).Where("credential_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMCredentialRepository) Add(ctx context.Context, e *entity.LLMCredential) (string, error) {
	e.CredentialId = idgen.New()
	// gorm 对自定义指针类型 *types.Time 不会自动回填 created_at/updated_at（实测全为 NULL），
	// 显式写入，否则列表「创建时间」列恒空。对齐 agent_runtime_repo 的写法。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增凭据失败", "credentialId", e.CredentialId, "name", e.CredentialName, "provider", e.ProviderType, "err", err)
		return "", err
	}
	slog.Info("新增凭据", "credentialId", e.CredentialId, "name", e.CredentialName, "provider", e.ProviderType, "createdAt", e.CreatedAt)
	return e.CredentialId, nil
}

func (r *LLMCredentialRepository) Edit(ctx context.Context, e *entity.LLMCredential) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now // 同 Add：显式写 updated_at
	// 用 map 显式写出列：GORM 的 Updates(struct) 会丢弃零值字段，导致 status=0（停用）、
	// base_url/credential_info 清空等无法落库。credential_values 仅在有新密文时覆盖，避免把已存密钥清空。
	fields := map[string]any{
		"credential_name": e.CredentialName,
		"provider_type":   e.ProviderType,
		"base_url":        e.BaseURL,
		"credential_info": e.CredentialInfo,
		"status":          e.Status,
		"updated_by":      e.UpdatedBy,
		"updated_at":      e.UpdatedAt,
	}
	if e.CredentialValues != "" {
		fields["credential_values"] = e.CredentialValues
	}
	if err := r.db.WithContext(ctx).Model(&entity.LLMCredential{}).
		Where("credential_id = ?", e.CredentialId).Updates(fields).Error; err != nil {
		slog.Error("编辑凭据失败", "credentialId", e.CredentialId, "err", err)
		return err
	}
	slog.Info("编辑凭据", "credentialId", e.CredentialId, "updatedAt", e.UpdatedAt)
	return nil
}

func (r *LLMCredentialRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMCredential{}).
		Where("credential_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Model =====================

type LLMModelRepository struct{ db *gorm.DB }

func NewLLMModelRepository() *LLMModelRepository { return &LLMModelRepository{db: db.LLMDB()} }

func (r *LLMModelRepository) List(ctx context.Context, name, provider, group string, status *uint, pageNum, pageSize int) (list []*entity.LLMModel, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMModel{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("model_name like ? or model_group like ?", "%"+name+"%", "%"+name+"%")
	}
	if provider != "" {
		q = q.Where("provider = ?", provider)
	}
	if group != "" {
		q = q.Where("model_group = ?", group)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMModelRepository) All(ctx context.Context) (list []*entity.LLMModel, err error) {
	err = r.db.WithContext(ctx).Where("deleted_at is null and blocked = 0").Order("id desc").Find(&list).Error
	return
}

func (r *LLMModelRepository) Get(ctx context.Context, id string) (*entity.LLMModel, error) {
	var e entity.LLMModel
	if err := r.db.WithContext(ctx).Where("model_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMModelRepository) Add(ctx context.Context, e *entity.LLMModel) (string, error) {
	// 凭据必填：网关把上游 key 唯一来源绑在 credential_id，空则调用必 503。
	if e.CredentialId == "" {
		return "", errors.New("请选择凭据")
	}
	// 部署查重：(model_group, provider_model, credential_id) 全同视为冗余部署。
	// 多凭据负载均衡请改用不同凭据——那是允许的、有意义的“重复”。
	if cnt, err := r.countDupDeploy(ctx, e.ModelGroup, e.ProviderModel, e.CredentialId, ""); err != nil {
		return "", err
	} else if cnt > 0 {
		return "", errors.New("该分组下已存在相同上游模型+凭据的部署（多凭据负载均衡请选用不同凭据）")
	}
	e.ModelId = idgen.New()
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增模型失败", "modelId", e.ModelId, "name", e.ModelName, "err", err)
		return "", err
	}
	slog.Info("新增模型", "modelId", e.ModelId, "name", e.ModelName)
	return e.ModelId, nil
}

func (r *LLMModelRepository) Edit(ctx context.Context, e *entity.LLMModel) error {
	// 改凭据/模型时查重（排除自身）；多凭据 LB 仍允许同 group+model+不同 credential。
	// 不强制非空：避免老数据（空 credential）切换 status 时被卡死，空值由 Add 拦截 + 网关路由过滤兜底。
	if cnt, err := r.countDupDeploy(ctx, e.ModelGroup, e.ProviderModel, e.CredentialId, e.ModelId); err != nil {
		return err
	} else if cnt > 0 {
		return errors.New("该分组下已存在相同上游模型+凭据的部署（多凭据负载均衡请选用不同凭据）")
	}
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	// 用 map 显式写出可编辑列：GORM 的 Updates(struct) 会丢弃零值字段，导致 status=0（停用）无法落库。
	// 仅列 UI 编辑面涉及的列；tpm_limit/rpm_limit/extra_params/model_info/cache 单价 等表单未管的字段保持原值。
	fields := map[string]any{
		"model_name":             e.ModelName,
		"model_group":            e.ModelGroup,
		"provider":               e.Provider,
		"provider_model":         e.ProviderModel,
		"credential_id":          e.CredentialId,
		"input_price_per_million":  e.InputPricePerMillion,
		"output_price_per_million": e.OutputPricePerMillion,
		"max_input_tokens":       e.MaxInputTokens,
		"max_output_tokens":      e.MaxOutputTokens,
		"weight":                 e.Weight,
		"status":                 e.Status,
		"updated_by":             e.UpdatedBy,
		"updated_at":             e.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Model(&entity.LLMModel{}).
		Where("model_id = ?", e.ModelId).Updates(fields).Error; err != nil {
		slog.Error("编辑模型失败", "modelId", e.ModelId, "err", err)
		return err
	}
	slog.Info("编辑模型", "modelId", e.ModelId, "updatedAt", e.UpdatedAt, "status", e.Status)
	return nil
}

func (r *LLMModelRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMModel{}).
		Where("model_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// BatchSyncModelTokenLimits 把同 (model_group, model_name) 下所有存活部署的 max_input/output_tokens
// 同步成同一值。这俩是模型固有属性（同模型本该一致），编辑一个部署时批量对齐，避免 FetchLLMModels
// 富化 group/model 时取到参差值（多个部署 max_output 不一致 → 下发不确定）。
func (r *LLMModelRepository) BatchSyncModelTokenLimits(ctx context.Context, group, modelName string, maxInput, maxOutput int, updatedBy string) error {
	now := types.Time(time.Now())
	return r.db.WithContext(ctx).Model(&entity.LLMModel{}).
		Where("model_group = ? AND model_name = ? AND deleted_at IS NULL", group, modelName).
		Updates(map[string]any{
			"max_input_tokens":  maxInput,
			"max_output_tokens": maxOutput,
			"updated_by":        updatedBy,
			"updated_at":        now,
		}).Error
}

// countDupDeploy 统计存活的同 (model_group, provider_model, credential_id) 部署数。
// excludeID 非空时排除自身（编辑场景）。credID 为空不参与查重（已被 Add 必填校验拦截）。
func (r *LLMModelRepository) countDupDeploy(ctx context.Context, group, providerModel, credID, excludeID string) (int64, error) {
	if credID == "" {
		return 0, nil
	}
	q := r.db.WithContext(ctx).Model(&entity.LLMModel{}).
		Where("model_group = ? AND provider_model = ? AND credential_id = ? AND deleted_at IS NULL",
			group, providerModel, credID)
	if excludeID != "" {
		q = q.Where("model_id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		slog.Error("模型部署查重失败", "group", group, "providerModel", providerModel, "err", err)
		return 0, err
	}
	return cnt, nil
}

// ExistsDeploy 同 (group, providerModel, credID) 是否已存在存活部署（discover 导入查重用）。
func (r *LLMModelRepository) ExistsDeploy(ctx context.Context, group, providerModel, credID string) (bool, error) {
	cnt, err := r.countDupDeploy(ctx, group, providerModel, credID, "")
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ===================== Budget =====================

type LLMBudgetRepository struct{ db *gorm.DB }

func NewLLMBudgetRepository() *LLMBudgetRepository { return &LLMBudgetRepository{db: db.LLMDB()} }

func (r *LLMBudgetRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMBudget, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMBudget{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("budget_name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMBudgetRepository) Get(ctx context.Context, id string) (*entity.LLMBudget, error) {
	var e entity.LLMBudget
	if err := r.db.WithContext(ctx).Where("budget_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMBudgetRepository) Add(ctx context.Context, e *entity.LLMBudget) (string, error) {
	e.BudgetId = idgen.New()
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增预算失败", "budgetId", e.BudgetId, "name", e.BudgetName, "err", err)
		return "", err
	}
	slog.Info("新增预算", "budgetId", e.BudgetId, "name", e.BudgetName)
	return e.BudgetId, nil
}

func (r *LLMBudgetRepository) Edit(ctx context.Context, e *entity.LLMBudget) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMBudget{}).Where("budget_id = ?", e.BudgetId).Updates(e).Error; err != nil {
		slog.Error("编辑预算失败", "budgetId", e.BudgetId, "err", err)
		return err
	}
	slog.Info("编辑预算", "budgetId", e.BudgetId)
	return nil
}

func (r *LLMBudgetRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMBudget{}).
		Where("budget_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ListGrantBudgets 取全部配了适用范围（grant_rules 非空）的预算（「我的预算」圈人判定用）。
// 注意：jsonb 列不能与 '' 比较（invalid input syntax for type json），列 NOT NULL DEFAULT '[]'，
// 空态即 '[]'。语义与网关侧 llm/internal/store.ListGrantBudgets 一致（改任一侧须同步）。
func (r *LLMBudgetRepository) ListGrantBudgets(ctx context.Context) ([]*entity.LLMBudget, error) {
	var list []*entity.LLMBudget
	err := r.db.WithContext(ctx).
		Where("deleted_at is null and grant_rules is not null and grant_rules <> '[]'").
		Find(&list).Error
	return list, err
}

// budgetWindow 把 budget_duration 解析成窗口时长（与网关侧 store.budgetWindow 同步维护）。
func budgetWindow(d string) (time.Duration, bool) {
	switch strings.TrimSpace(d) {
	case "1d":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d", "1mo":
		return 30 * 24 * time.Hour, true
	}
	return 0, false
}

func budgetNextReset(from time.Time, dur time.Duration) time.Time {
	next := from.Add(dur)
	for !next.After(time.Now()) {
		next = next.Add(dur)
	}
	return next
}

// RollBudgetWindow 预算窗口到期懒滚存（行锁下到期则 spend 清零、reset_at 推进），返回滚存后的最新值。
// 语义与网关侧 llm/internal/store.RollBudgetWindow 一致（改任一侧须同步）：「我的预算」读取前调用，
// 保证展示的「本周期已消费」不是过期窗口的旧值。
func (r *LLMBudgetRepository) RollBudgetWindow(ctx context.Context, budgetID string) (*entity.LLMBudget, error) {
	var out *entity.LLMBudget
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var b entity.LLMBudget
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("budget_id = ? and deleted_at is null", budgetID).First(&b).Error; err != nil {
			return err
		}
		out = &b
		dur, ok := budgetWindow(b.BudgetDuration)
		if !ok {
			return nil
		}
		var next time.Time
		if b.BudgetResetAt == nil {
			next = budgetNextReset(time.Now(), dur)
		} else {
			resetAt := time.Time(*b.BudgetResetAt)
			if !resetAt.Before(time.Now()) {
				return nil
			}
			next = budgetNextReset(resetAt, dur)
		}
		if err := tx.Model(&entity.LLMBudget{}).Where("budget_id = ?", budgetID).Updates(map[string]interface{}{
			"spend": 0, "budget_reset_at": next, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		b.Spend = 0
		reset := types.Time(next)
		b.BudgetResetAt = &reset
		out = &b
		return nil
	})
	return out, err
}

// TryBudgetNoticeDedup 预算低额/耗尽通知去重：INSERT ON CONFLICT DO NOTHING，
// RowsAffected==1 表示本窗口本档位首次（应发通知），==0 表示已发过。
// 表 aa_llm_budget_notice 由网关侧计费路径共用（两 module 各一份实现，改任一侧须同步）。
func (r *LLMBudgetRepository) TryBudgetNoticeDedup(ctx context.Context, userId, budgetId, windowKey, level string) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_budget_notice (user_id, budget_id, window_key, level, created_at)
VALUES (?,?,?,?,now())
ON CONFLICT (user_id, budget_id, window_key, level) DO NOTHING`,
		userId, budgetId, windowKey, level)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ===================== BudgetRequest（临时预算申请，已上收统一审核中心） =====================
// 原仓储实现删除：申请单统一走主库 aa_review_order（ReviewOrderRepository，
// persistence/review_order_repo.go），历史表归档搬运见 MigrateLegacyBudgetRequests。

// ===================== ModelGroup（分组字典，软引用 aa_llm_model.model_group）=====================

type LLMModelGroupRepository struct{ db *gorm.DB }

func NewLLMModelGroupRepository() *LLMModelGroupRepository {
	return &LLMModelGroupRepository{db: db.LLMDB()}
}

func (r *LLMModelGroupRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMModelGroup, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMModelGroup{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("group_name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMModelGroupRepository) Get(ctx context.Context, id string) (*entity.LLMModelGroup, error) {
	var e entity.LLMModelGroup
	if err := r.db.WithContext(ctx).Where("group_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// GetByName 按 group_name 取分组（含已删校验用，唯一性查重时调）。
func (r *LLMModelGroupRepository) GetByName(ctx context.Context, name string) (*entity.LLMModelGroup, error) {
	var e entity.LLMModelGroup
	if err := r.db.WithContext(ctx).Where("group_name = ? and deleted_at is null", name).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMModelGroupRepository) Add(ctx context.Context, e *entity.LLMModelGroup) (string, error) {
	// group_name 唯一性查重（软引用字典，重名会破坏模型分组语义）
	var cnt int64
	if err := r.db.WithContext(ctx).Model(&entity.LLMModelGroup{}).
		Where("group_name = ? and deleted_at is null", e.GroupName).Count(&cnt).Error; err != nil {
		slog.Error("新增分组查重失败", "groupName", e.GroupName, "err", err)
		return "", err
	}
	if cnt > 0 {
		return "", errors.New("分组名已存在")
	}
	e.GroupId = idgen.New()
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增分组失败", "groupId", e.GroupId, "groupName", e.GroupName, "err", err)
		return "", err
	}
	slog.Info("新增模型分组", "groupId", e.GroupId, "groupName", e.GroupName)
	return e.GroupId, nil
}

func (r *LLMModelGroupRepository) Edit(ctx context.Context, e *entity.LLMModelGroup) error {
	// 改名时查重（排除自身）
	if e.GroupName != "" {
		var cnt int64
		if err := r.db.WithContext(ctx).Model(&entity.LLMModelGroup{}).
			Where("group_name = ? and group_id <> ? and deleted_at is null", e.GroupName, e.GroupId).Count(&cnt).Error; err != nil {
			slog.Error("编辑分组查重失败", "groupId", e.GroupId, "err", err)
			return err
		}
		if cnt > 0 {
			return errors.New("分组名已存在")
		}
	}
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMModelGroup{}).Where("group_id = ?", e.GroupId).Updates(e).Error; err != nil {
		slog.Error("编辑分组失败", "groupId", e.GroupId, "err", err)
		return err
	}
	slog.Info("编辑模型分组", "groupId", e.GroupId, "groupName", e.GroupName)
	return nil
}

func (r *LLMModelGroupRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&entity.LLMModelGroup{}).
		Where("group_id in ?", ids).Update("deleted_at", time.Now()).Error; err != nil {
		slog.Error("删除分组失败", "ids", ids, "err", err)
		return err
	}
	slog.Info("删除模型分组", "ids", ids)
	return nil
}

// ===================== VirtualKey =====================

type LLMVirtualKeyRepository struct{ db *gorm.DB }

func NewLLMVirtualKeyRepository() *LLMVirtualKeyRepository {
	return &LLMVirtualKeyRepository{db: db.LLMDB()}
}

func (r *LLMVirtualKeyRepository) List(ctx context.Context, alias, userId, teamId string, pageNum, pageSize int) (list []*entity.LLMVirtualKey, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).Where("deleted_at is null")
	if alias != "" {
		q = q.Where("key_alias like ?", "%"+alias+"%")
	}
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if teamId != "" {
		q = q.Where("team_id = ?", teamId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMVirtualKeyRepository) Get(ctx context.Context, tokenHash string) (*entity.LLMVirtualKey, error) {
	var e entity.LLMVirtualKey
	if err := r.db.WithContext(ctx).Where("token_hash = ? and deleted_at is null", tokenHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMVirtualKeyRepository) GetByAlias(ctx context.Context, alias string) (*entity.LLMVirtualKey, error) {
	var e entity.LLMVirtualKey
	if err := r.db.WithContext(ctx).Where("key_alias = ? and deleted_at is null", alias).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListAffectedByModel 列出 allowed_models 会让该模型可见的 key：空/[]/["all"]（不限制），
// 或含具体 "group/modelName"，或含裸组 "group"（覆盖成员）。模型禁用/删除后定位需刷新配置的实例用。
// LIKE 用带引号 token（"value"）精确匹配 JSON 串里的字符串元素，避开 jsonb 空串坑。
func (r *LLMVirtualKeyRepository) ListAffectedByModel(ctx context.Context, group, modelName string) (list []*entity.LLMVirtualKey, err error) {
	if group == "" {
		return nil, nil
	}
	composite := group
	if modelName != "" {
		composite = group + "/" + modelName
	}
	err = r.db.WithContext(ctx).
		Where("deleted_at is null").
		Where("allowed_models = '' OR allowed_models = '[]' OR lower(allowed_models) LIKE ? OR allowed_models LIKE ? OR allowed_models LIKE ?",
			`%"all"%`, `%"`+composite+`"%`, `%"`+group+`"%`).
		Find(&list).Error
	return
}

func (r *LLMVirtualKeyRepository) Add(ctx context.Context, e *entity.LLMVirtualKey) (string, error) {
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增虚拟密钥失败", "keyAlias", e.KeyAlias, "err", err)
		return "", err
	}
	slog.Info("新增虚拟密钥", "keyAlias", e.KeyAlias)
	return e.TokenHash, nil
}

func (r *LLMVirtualKeyRepository) Edit(ctx context.Context, e *entity.LLMVirtualKey) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).Where("token_hash = ?", e.TokenHash).Updates(e).Error; err != nil {
		slog.Error("编辑虚拟密钥失败", "keyAlias", e.KeyAlias, "err", err)
		return err
	}
	slog.Info("编辑虚拟密钥", "keyAlias", e.KeyAlias)
	return nil
}

// UpdateBlocked 单独更新 blocked（map 更新，不跳零值）。
// 解封 blocked=0 时若走结构体 Updates(e)，gorm 会把 0 当零值跳过 → 解封不生效；故封禁/解封专用此方法。
func (r *LLMVirtualKeyRepository) UpdateBlocked(ctx context.Context, tokenHash string, blocked uint, by string) error {
	now := types.Time(time.Now())
	return r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).
		Where("token_hash = ?", tokenHash).
		Updates(map[string]any{"blocked": blocked, "updated_by": by, "updated_at": &now}).Error
}

func (r *LLMVirtualKeyRepository) Delete(ctx context.Context, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).
		Where("token_hash in ?", hashes).Update("deleted_at", time.Now()).Error
}

// RotateHash 原地轮换 token_hash：旧 hash 进入 5min 宽限期（deprecated_token_hash +
// deprecate_expires_at），宽限内新旧 hash 均可鉴权并落到同一行（配置/花费连续）。
func (r *LLMVirtualKeyRepository) RotateHash(ctx context.Context, oldHash, newHash string, graceExpires time.Time, by string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).
		Where("token_hash = ? AND deleted_at IS NULL", oldHash).
		Updates(map[string]any{
			"token_hash":            newHash,
			"deprecated_token_hash": oldHash,
			"deprecate_expires_at":  graceExpires,
			"rotation_count":        gorm.Expr("rotation_count + 1"),
			"last_rotation_at":      now,
			"key_rotation_at":       nil,
			"updated_by":            by,
			"updated_at":            now,
		}).Error
}

func (r *LLMVirtualKeyRepository) UpdateSpend(ctx context.Context, tokenHash string, delta types.Milli) error {
	return r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).
		Where("token_hash = ?", tokenHash).UpdateColumn("spend", gorm.Expr("spend + ?", delta)).Error
}

func (r *LLMVirtualKeyRepository) UpdateLastActive(ctx context.Context, tokenHash string) error {
	return r.db.WithContext(ctx).Model(&entity.LLMVirtualKey{}).
		Where("token_hash = ?", tokenHash).UpdateColumn("last_active", time.Now()).Error
}

// ===================== DeptProfile（aa_dept，aiarsenal 库） =====================
// 注意：aa_dept 在 aiarsenal 库，故用 db.DB()（非 LLMDB）。仅更新 LLM 扩展列。

type LLMDeptProfileRepository struct{ db *gorm.DB }

func NewLLMDeptProfileRepository() *LLMDeptProfileRepository {
	return &LLMDeptProfileRepository{db: db.DB()}
}

func (r *LLMDeptProfileRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMDeptProfile, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMDeptProfile{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("dept_name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	// 不 join：先取部门行，负责人姓名/工号由 service 用 LeaderMapByIds 单独批量读 aa_user 回填。
	err = q.Order("order_num asc, id asc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMDeptProfileRepository) Get(ctx context.Context, deptId string) (*entity.LLMDeptProfile, error) {
	var e entity.LLMDeptProfile
	if err := r.db.WithContext(ctx).Where("dept_id = ? and deleted_at is null", deptId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// LeaderMapByIds 批量取 user_id→{name, employee_no}（不 join，单独读 aa_user）。
func (r *LLMDeptProfileRepository) LeaderMapByIds(ctx context.Context, userIds []string) (map[string]entity.LeaderInfo, error) {
	out := make(map[string]entity.LeaderInfo, len(userIds))
	if len(userIds) == 0 {
		return out, nil
	}
	var rows []struct {
		UserId     string `gorm:"column:user_id"`
		Name       string `gorm:"column:name"`
		EmployeeNo string `gorm:"column:employee_no"`
	}
	if err := r.db.WithContext(ctx).Table("aa_user").
		Select("user_id, name, employee_no").
		Where("user_id in ? and deleted_at is null", userIds).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.UserId] = entity.LeaderInfo{Name: r.Name, EmployeeNo: r.EmployeeNo}
	}
	return out, nil
}

// NameMapByIds 批量取 dept_id→dept_name，仅取未删除部门。
func (r *LLMDeptProfileRepository) NameMapByIds(ctx context.Context, deptIds []string) (map[string]string, error) {
	out := make(map[string]string, len(deptIds))
	if len(deptIds) == 0 {
		return out, nil
	}
	var rows []struct {
		DeptId   string `gorm:"column:dept_id"`
		DeptName string `gorm:"column:dept_name"`
	}
	if err := r.db.WithContext(ctx).Table("aa_dept").
		Select("dept_id, dept_name").
		Where("dept_id in ? and deleted_at is null", deptIds).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.DeptId] = r.DeptName
	}
	return out, nil
}

// UpdateLLMConfig 只覆盖 LLM 扩展列（不碰 dept_name/parent_id 等系统字段）。
func (r *LLMDeptProfileRepository) UpdateLLMConfig(ctx context.Context, e *entity.LLMDeptProfile) error {
	return r.db.WithContext(ctx).Model(&entity.LLMDeptProfile{}).
		Where("dept_id = ?", e.DeptId).
		Updates(map[string]any{
			"llm_models":                e.LlmModels,
			"llm_tpm_limit":             e.LlmTpmLimit,
			"llm_rpm_limit":             e.LlmRpmLimit,
			"llm_max_parallel_requests": e.LlmMaxParallelReq,
			"llm_budget_duration":       e.LlmBudgetDuration,
			"llm_budget_reset_at":       e.LlmBudgetResetAt,
			"llm_router_settings":       e.LlmRouterSettings,
			"llm_blocked":               e.LlmBlocked,
		}).Error
}

// ===================== Project =====================

type LLMProjectRepository struct{ db *gorm.DB }

func NewLLMProjectRepository() *LLMProjectRepository { return &LLMProjectRepository{db: db.LLMDB()} }

func (r *LLMProjectRepository) List(ctx context.Context, alias, teamId string, pageNum, pageSize int) (list []*entity.LLMProject, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMProject{}).Where("deleted_at is null")
	if alias != "" {
		q = q.Where("project_alias like ?", "%"+alias+"%")
	}
	if teamId != "" {
		q = q.Where("team_id = ?", teamId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMProjectRepository) Get(ctx context.Context, id string) (*entity.LLMProject, error) {
	var e entity.LLMProject
	if err := r.db.WithContext(ctx).Where("project_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMProjectRepository) Add(ctx context.Context, e *entity.LLMProject) (string, error) {
	e.ProjectId = idgen.New()
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增项目失败", "projectId", e.ProjectId, "alias", e.ProjectAlias, "err", err)
		return "", err
	}
	slog.Info("新增项目", "projectId", e.ProjectId, "alias", e.ProjectAlias)
	return e.ProjectId, nil
}

func (r *LLMProjectRepository) Edit(ctx context.Context, e *entity.LLMProject) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMProject{}).Where("project_id = ?", e.ProjectId).Updates(e).Error; err != nil {
		slog.Error("编辑项目失败", "projectId", e.ProjectId, "err", err)
		return err
	}
	slog.Info("编辑项目", "projectId", e.ProjectId)
	return nil
}

func (r *LLMProjectRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMProject{}).
		Where("project_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== EndUser =====================

type LLMEndUserRepository struct{ db *gorm.DB }

func NewLLMEndUserRepository() *LLMEndUserRepository { return &LLMEndUserRepository{db: db.LLMDB()} }

func (r *LLMEndUserRepository) List(ctx context.Context, alias string, pageNum, pageSize int) (list []*entity.LLMEndUser, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMEndUser{})
	if alias != "" {
		q = q.Where("alias like ?", "%"+alias+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMEndUserRepository) Get(ctx context.Context, id string) (*entity.LLMEndUser, error) {
	var e entity.LLMEndUser
	if err := r.db.WithContext(ctx).Where("end_user_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMEndUserRepository) Add(ctx context.Context, e *entity.LLMEndUser) (string, error) {
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增终端用户失败", "endUserId", e.EndUserId, "alias", e.Alias, "err", err)
		return "", err
	}
	slog.Info("新增终端用户", "endUserId", e.EndUserId, "alias", e.Alias)
	return e.EndUserId, nil
}

func (r *LLMEndUserRepository) Edit(ctx context.Context, e *entity.LLMEndUser) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMEndUser{}).Where("end_user_id = ?", e.EndUserId).Updates(e).Error; err != nil {
		slog.Error("编辑终端用户失败", "endUserId", e.EndUserId, "err", err)
		return err
	}
	slog.Info("编辑终端用户", "endUserId", e.EndUserId)
	return nil
}

func (r *LLMEndUserRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("end_user_id in ?", ids).Delete(&entity.LLMEndUser{}).Error
}

// ===================== Tag =====================

type LLMTagRepository struct{ db *gorm.DB }

func NewLLMTagRepository() *LLMTagRepository { return &LLMTagRepository{db: db.LLMDB()} }

func (r *LLMTagRepository) List(ctx context.Context, name string, pageNum, pageSize int) (list []*entity.LLMTag, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMTag{})
	if name != "" {
		q = q.Where("tag_name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMTagRepository) Get(ctx context.Context, name string) (*entity.LLMTag, error) {
	var e entity.LLMTag
	if err := r.db.WithContext(ctx).Where("tag_name = ?", name).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMTagRepository) Add(ctx context.Context, e *entity.LLMTag) error {
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增标签失败", "name", e.TagName, "err", err)
		return err
	}
	slog.Info("新增标签", "name", e.TagName)
	return nil
}

func (r *LLMTagRepository) Edit(ctx context.Context, e *entity.LLMTag) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMTag{}).Where("tag_name = ?", e.TagName).Updates(e).Error; err != nil {
		slog.Error("编辑标签失败", "name", e.TagName, "err", err)
		return err
	}
	slog.Info("编辑标签", "name", e.TagName)
	return nil
}

func (r *LLMTagRepository) Delete(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("tag_name in ?", names).Delete(&entity.LLMTag{}).Error
}

// ===================== SpendLog =====================

type LLMSpendLogRepository struct{ db *gorm.DB }

func NewLLMSpendLogRepository() *LLMSpendLogRepository { return &LLMSpendLogRepository{db: db.LLMDB()} }

func (r *LLMSpendLogRepository) List(ctx context.Context, userId, teamId, model, apiKey string, dateRange []string, pageNum, pageSize int) (list []*entity.LLMSpendLog, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMSpendLog{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if teamId != "" {
		q = q.Where("team_id = ?", teamId)
	}
	if model != "" {
		q = q.Where("model like ?", "%"+model+"%")
	}
	if apiKey != "" {
		q = q.Where("api_key = ?", apiKey)
	}
	if len(dateRange) == 2 {
		q = q.Where("start_time >= ? and start_time <= ?", dateRange[0], dateRange[1])
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMSpendLogRepository) Get(ctx context.Context, requestId string) (*entity.LLMSpendLog, error) {
	var e entity.LLMSpendLog
	if err := r.db.WithContext(ctx).Where("request_id = ?", requestId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LLMSpendLogRepository) Add(ctx context.Context, e *entity.LLMSpendLog) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// ===================== DailySpend =====================

type LLMDailySpendRepository struct{ db *gorm.DB }

func NewLLMDailySpendRepository() *LLMDailySpendRepository {
	return &LLMDailySpendRepository{db: db.LLMDB()}
}

func (r *LLMDailySpendRepository) List(ctx context.Context, scopeType, scopeId, dateRange []string, pageNum, pageSize int) (list []*entity.LLMDailySpend, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMDailySpend{})
	if len(scopeType) > 0 && scopeType[0] != "" {
		q = q.Where("scope_type = ?", scopeType[0])
	}
	if len(scopeId) > 0 && scopeId[0] != "" {
		q = q.Where("scope_id = ?", scopeId[0])
	}
	if len(dateRange) == 2 {
		q = q.Where("date >= ? and date <= ?", dateRange[0], dateRange[1])
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// Upsert 日聚合语义：命中唯一约束则累加，否则插入。
func (r *LLMDailySpendRepository) Upsert(ctx context.Context, e *entity.LLMDailySpend) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_daily_spend (scope_type, scope_id, date, api_key, model, model_group, custom_llm_provider, endpoint, mcp_namespaced_tool_name, prompt_tokens, completion_tokens, cache_read_input_tokens, cache_creation_input_tokens, spend, api_requests, successful_requests, failed_requests, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,now(),now())
ON CONFLICT (scope_type, scope_id, date, api_key, model, custom_llm_provider, mcp_namespaced_tool_name, endpoint) DO UPDATE SET
  prompt_tokens = aa_llm_daily_spend.prompt_tokens + EXCLUDED.prompt_tokens,
  completion_tokens = aa_llm_daily_spend.completion_tokens + EXCLUDED.completion_tokens,
  cache_read_input_tokens = aa_llm_daily_spend.cache_read_input_tokens + EXCLUDED.cache_read_input_tokens,
  cache_creation_input_tokens = aa_llm_daily_spend.cache_creation_input_tokens + EXCLUDED.cache_creation_input_tokens,
  spend = aa_llm_daily_spend.spend + EXCLUDED.spend,
  api_requests = aa_llm_daily_spend.api_requests + EXCLUDED.api_requests,
  successful_requests = aa_llm_daily_spend.successful_requests + EXCLUDED.successful_requests,
  failed_requests = aa_llm_daily_spend.failed_requests + EXCLUDED.failed_requests,
  updated_at = now()`,
		e.ScopeType, e.ScopeId, e.Date, e.ApiKey, e.Model, e.ModelGroup, e.CustomLlmProvider, e.Endpoint, e.McpNamespacedToolName,
		e.PromptTokens, e.CompletionTokens, e.CacheReadInputTokens, e.CacheCreationInputTokens, e.Spend, e.ApiRequests, e.SuccessfulRequests, e.FailedRequests,
	).Error
}

// Aggregate 花费归集：按维度 GROUP BY 窗口内花费/请求/Token。
// 计数口径：一律取 scope_type='key' 的行——每笔请求固定落一条 key scope（user/team scope 是
// 按持有关系补落的副本行），跨 scope 求和会重复计数；user 维度取 scope_type='user'（仅有
// user 归属的 key 的请求计入，语义即"按用户归集"）。
// dimension：date（按日趋势）/ key（按密钥，TOP N 花费降序）/ user（按用户）/ model（按模型）。
func (r *LLMDailySpendRepository) Aggregate(ctx context.Context, dimension string, dateRange []string, limit int) (list []*entity.LLMSpendAggregate, err error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := "scope_type = 'key'"
	args := []interface{}{}
	if len(dateRange) == 2 {
		where += " and date >= ? and date <= ?"
		args = append(args, dateRange[0], dateRange[1])
	}
	switch dimension {
	case "date":
		return aggregateRows(ctx, r.db, `
SELECT date AS date, '' AS scope_id, '' AS key_alias, '' AS model,
  SUM(spend)::bigint AS spend, SUM(api_requests) AS api_requests,
  SUM(prompt_tokens) AS prompt_tokens, SUM(completion_tokens) AS completion_tokens
FROM aa_llm_daily_spend WHERE `+where+` GROUP BY date ORDER BY date`, args)
	case "key":
		return aggregateRows(ctx, r.db, `
SELECT '' AS date, d.scope_id AS scope_id, COALESCE(v.key_alias, '') AS key_alias, '' AS model,
  SUM(d.spend)::bigint AS spend, SUM(d.api_requests) AS api_requests,
  SUM(d.prompt_tokens) AS prompt_tokens, SUM(d.completion_tokens) AS completion_tokens
FROM aa_llm_daily_spend d
LEFT JOIN aa_llm_virtual_key v ON v.token_hash = d.scope_id
WHERE d.`+where+` GROUP BY d.scope_id, v.key_alias ORDER BY spend DESC LIMIT ?`, append(args, limit))
	case "user":
		uw := strings.Replace(where, "scope_type = 'key'", "scope_type = 'user'", 1)
		return aggregateRows(ctx, r.db, `
SELECT '' AS date, scope_id AS scope_id, '' AS key_alias, '' AS model,
  SUM(spend)::bigint AS spend, SUM(api_requests) AS api_requests,
  SUM(prompt_tokens) AS prompt_tokens, SUM(completion_tokens) AS completion_tokens
FROM aa_llm_daily_spend WHERE `+uw+` GROUP BY scope_id ORDER BY spend DESC LIMIT ?`, append(args, limit))
	case "model":
		return aggregateRows(ctx, r.db, `
SELECT '' AS date, '' AS scope_id, '' AS key_alias, model AS model,
  SUM(spend)::bigint AS spend, SUM(api_requests) AS api_requests,
  SUM(prompt_tokens) AS prompt_tokens, SUM(completion_tokens) AS completion_tokens
FROM aa_llm_daily_spend WHERE `+where+` GROUP BY model ORDER BY spend DESC LIMIT ?`, append(args, limit))
	}
	return nil, fmt.Errorf("invalid dimension: %s", dimension)
}

// AggregateTotals 窗口内总花费/请求/Token（key scope 口径，同 Aggregate）。
func (r *LLMDailySpendRepository) AggregateTotals(ctx context.Context, dateRange []string) (*entity.LLMSpendTotals, error) {
	where := "scope_type = 'key'"
	args := []interface{}{}
	if len(dateRange) == 2 {
		where += " and date >= ? and date <= ?"
		args = append(args, dateRange[0], dateRange[1])
	}
	var t entity.LLMSpendTotals
	err := r.db.WithContext(ctx).Raw(`
SELECT COALESCE(SUM(spend),0)::bigint AS spend, COALESCE(SUM(api_requests),0) AS api_requests,
  COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens
FROM aa_llm_daily_spend WHERE `+where, args...).Scan(&t).Error
	return &t, err
}

// SumScopeDaily 某 scope 日粒度消费聚合（我的预算·历史周期消费）：日行按 date 聚合升序返回，
// service 层再按周（ISO）/月分桶。
func (r *LLMDailySpendRepository) SumScopeDaily(ctx context.Context, scopeType, scopeId string, dateRange []string) ([]*entity.LLMSpendPoint, error) {
	where := "scope_type = ? and scope_id = ?"
	args := []interface{}{scopeType, scopeId}
	if len(dateRange) == 2 {
		where += " and date >= ? and date <= ?"
		args = append(args, dateRange[0], dateRange[1])
	}
	var list []*entity.LLMSpendPoint
	err := r.db.WithContext(ctx).Raw(`
SELECT date, COALESCE(SUM(spend),0)::bigint AS spend, COALESCE(SUM(api_requests),0) AS requests,
  COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens
FROM aa_llm_daily_spend WHERE `+where+`
GROUP BY date ORDER BY date`, args...).Scan(&list).Error
	return list, err
}

// aggregateRows 执行聚合 SQL 并扫到 LLMSpendAggregate 行。
func aggregateRows(ctx context.Context, gdb *gorm.DB, sql string, args []interface{}) ([]*entity.LLMSpendAggregate, error) {
	var list []*entity.LLMSpendAggregate
	err := gdb.WithContext(ctx).Raw(sql, args...).Scan(&list).Error
	return list, err
}

// ===================== ErrorLog =====================

type LLMErrorLogRepository struct{ db *gorm.DB }

func NewLLMErrorLogRepository() *LLMErrorLogRepository { return &LLMErrorLogRepository{db: db.LLMDB()} }

func (r *LLMErrorLogRepository) List(ctx context.Context, model, apiKey string, dateRange []string, pageNum, pageSize int) (list []*entity.LLMErrorLog, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMErrorLog{})
	if model != "" {
		q = q.Where("model_group like ? or model_name like ?", "%"+model+"%", "%"+model+"%")
	}
	if apiKey != "" {
		q = q.Where("api_key = ?", apiKey)
	}
	if len(dateRange) == 2 {
		q = q.Where("start_time >= ? and start_time <= ?", dateRange[0], dateRange[1])
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMErrorLogRepository) Add(ctx context.Context, e *entity.LLMErrorLog) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// ===================== AuditLog =====================

type LLMAuditLogRepository struct{ db *gorm.DB }

func NewLLMAuditLogRepository() *LLMAuditLogRepository { return &LLMAuditLogRepository{db: db.LLMDB()} }

func (r *LLMAuditLogRepository) List(ctx context.Context, tableName, action string, pageNum, pageSize int) (list []*entity.LLMAuditLog, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMAuditLog{})
	if tableName != "" {
		q = q.Where("table_name = ?", tableName)
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

func (r *LLMAuditLogRepository) Add(ctx context.Context, e *entity.LLMAuditLog) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// ===================== HealthCheck =====================

type LLMHealthCheckRepository struct{ db *gorm.DB }

func NewLLMHealthCheckRepository() *LLMHealthCheckRepository {
	return &LLMHealthCheckRepository{db: db.LLMDB()}
}

func (r *LLMHealthCheckRepository) List(ctx context.Context, modelName, status string, pageNum, pageSize int) (list []*entity.LLMHealthCheck, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMHealthCheck{})
	if modelName != "" {
		q = q.Where("model_name like ?", "%"+modelName+"%")
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

func (r *LLMHealthCheckRepository) Add(ctx context.Context, e *entity.LLMHealthCheck) (string, error) {
	e.HealthCheckId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.HealthCheckId, nil
}

// ===================== Prompt =====================

type LLMPromptRepository struct{ db *gorm.DB }

func NewLLMPromptRepository() *LLMPromptRepository { return &LLMPromptRepository{db: db.LLMDB()} }

func (r *LLMPromptRepository) List(ctx context.Context, name, environment string, pageNum, pageSize int) (list []*entity.LLMPrompt, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.LLMPrompt{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if environment != "" {
		q = q.Where("environment = ?", environment)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *LLMPromptRepository) Get(ctx context.Context, promptId string) (list []*entity.LLMPrompt, err error) {
	err = r.db.WithContext(ctx).Where("prompt_id = ? and deleted_at is null", promptId).Order("version desc").Find(&list).Error
	return
}

func (r *LLMPromptRepository) Add(ctx context.Context, e *entity.LLMPrompt) (string, error) {
	e.PromptId = idgen.New()
	// gorm 不会自动回填 *types.Time，显式写 created_at/updated_at，否则列表「创建时间」恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		slog.Error("新增提示词失败", "promptId", e.PromptId, "name", e.Name, "err", err)
		return "", err
	}
	slog.Info("新增提示词", "promptId", e.PromptId, "name", e.Name)
	return e.PromptId, nil
}

func (r *LLMPromptRepository) Edit(ctx context.Context, e *entity.LLMPrompt) error {
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := r.db.WithContext(ctx).Model(&entity.LLMPrompt{}).Where("prompt_id = ? and version = ?", e.PromptId, e.Version).Updates(e).Error; err != nil {
		slog.Error("编辑提示词失败", "promptId", e.PromptId, "version", e.Version, "err", err)
		return err
	}
	slog.Info("编辑提示词", "promptId", e.PromptId, "version", e.Version)
	return nil
}

func (r *LLMPromptRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.LLMPrompt{}).
		Where("prompt_id in ?", ids).Update("deleted_at", time.Now()).Error
}
