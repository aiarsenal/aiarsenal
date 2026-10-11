package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiarsenal/llm/internal/crypto"
	"aiarsenal/llm/internal/config"
	"aiarsenal/llm/internal/grantx"
	"aiarsenal/llm/internal/model"
)

// Store 封装 router 对 llm 库的只读查询 + 凭据解密。

type Store struct {
	db        *gorm.DB
	secretKey string
}

func New(db *gorm.DB, cfg *config.Config) *Store {
	return &Store{db: db, secretKey: cfg.SecretKey}
}

// DB 暴露底层 gorm 实例（用于写 last_active / spend 等少量更新）。
func (s *Store) DB() *gorm.DB { return s.db }

// SpendRecord 计费日志入参（字段与 aa_llm_spend_log 对齐）。
type SpendRecord struct {
	RequestID        string
	CallType         string
	ApiKey           string
	Spend            model.Milli
	PromptTokens     int
	CompletionTokens int
	CacheReadTokens  int
	CacheCreateTokens int
	StartTime        time.Time
	EndTime          time.Time
	DurationMs       int
	Model            string
	ModelID          string
	ModelGroup       string
	Provider         string
	ApiBase          string
	UserId           string
	TeamId           string
	OrgId            string
	ProjectId        string
	EndUser          string
	SessionID        string
	IP               string
	CacheHit         string
	Status           string
	Messages         string // 请求输入（客户端原始 body，截断到 64KB）
	Response         string // 响应输出（非流式：完整 body；流式：空，截断到 64KB）
}

// WriteSpendLog 写一条请求花费日志。
func (s *Store) WriteSpendLog(ctx context.Context, r SpendRecord) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_spend_log (request_id, call_type, api_key, spend, total_tokens, prompt_tokens, completion_tokens,
  cache_read_input_tokens, cache_creation_input_tokens, start_time, end_time, request_duration_ms, model, model_id,
  model_group, custom_llm_provider, api_base, user_id, team_id, org_id, project_id, end_user, session_id,
  requester_ip_address, cache_hit, status, metadata, messages, response)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'{}',?,?)`,
		r.RequestID, r.CallType, r.ApiKey, r.Spend, r.PromptTokens+r.CompletionTokens, r.PromptTokens, r.CompletionTokens,
		r.CacheReadTokens, r.CacheCreateTokens, r.StartTime, r.EndTime, r.DurationMs, r.Model, r.ModelID,
		r.ModelGroup, r.Provider, r.ApiBase, r.UserId, r.TeamId, r.OrgId, r.ProjectId, r.EndUser, r.SessionID,
		r.IP, r.CacheHit, r.Status, r.Messages, r.Response,
	).Error
}

// UpsertDailySpend 按 scope 累加日聚合（ON CONFLICT 语义）。
func (s *Store) UpsertDailySpend(ctx context.Context, r SpendRecord, scopeType, scopeId, date string) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_daily_spend (scope_type, scope_id, date, api_key, model, model_group, custom_llm_provider,
  endpoint, mcp_namespaced_tool_name, prompt_tokens, completion_tokens, cache_read_input_tokens,
  cache_creation_input_tokens, spend, api_requests, successful_requests, failed_requests, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,0,0,now(),now())
ON CONFLICT (scope_type, scope_id, date, api_key, model, custom_llm_provider, mcp_namespaced_tool_name, endpoint) DO UPDATE SET
  prompt_tokens = aa_llm_daily_spend.prompt_tokens + EXCLUDED.prompt_tokens,
  completion_tokens = aa_llm_daily_spend.completion_tokens + EXCLUDED.completion_tokens,
  spend = aa_llm_daily_spend.spend + EXCLUDED.spend,
  api_requests = aa_llm_daily_spend.api_requests + EXCLUDED.api_requests,
  updated_at = now()`,
		scopeType, scopeId, date, r.ApiKey, r.Model, r.ModelGroup, r.Provider, "", "",
		r.PromptTokens, r.CompletionTokens, r.CacheReadTokens, r.CacheCreateTokens, r.Spend,
	).Error
}

// IncrementKeySpend 给 virtual key 的 spend 增量。
func (s *Store) IncrementKeySpend(ctx context.Context, hash string, delta model.Milli) error {
	return s.db.WithContext(ctx).Exec(`UPDATE aa_llm_virtual_key SET spend = spend + ? WHERE token_hash = ?`, delta, hash).Error
}

// WriteErrorLog 写错误日志。requestBody 落 request_kwargs（详情页"请求参数"），便于复现失败请求。
func (s *Store) WriteErrorLog(ctx context.Context, r SpendRecord, excType, excStr, statusCode, requestBody string) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_error_log (request_id, start_time, end_time, api_base, model_group, model_name,
  model_id, request_kwargs, exception_type, exception_string, status_code, api_key, user_id, team_id)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.RequestID, r.StartTime, r.EndTime, r.ApiBase, r.ModelGroup, r.Model, r.ModelID,
		requestBody, excType, excStr, statusCode, r.ApiKey, r.UserId, r.TeamId,
	).Error
}

// GetKeyByHash 按 token_hash 取 virtual key（未删除）。
// 命中规则：精确匹配 token_hash，或命中宽限期内的 deprecated_token_hash
// （轮换后旧明文保留 5min，deprecate_expires_at > now 期间仍可鉴权并落到同一行）。
func (s *Store) GetKeyByHash(ctx context.Context, hash string) (*model.VirtualKey, error) {
	var k model.VirtualKey
	now := time.Now()
	err := s.db.WithContext(ctx).
		Where(`(token_hash = ? OR (deprecated_token_hash = ? AND deprecate_expires_at IS NOT NULL AND deprecate_expires_at > ?)) AND deleted_at IS NULL`, hash, hash, now).
		First(&k).Error
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListModelsByGroup 按客户端传入的 id + 入口端点可路由的 provider 集合取可用部署。
// id 形如 "group/model"（含 /）或裸 "group"；providers 由入口端点给出（见 proxy.Handler）：
//   - /v1/chat/completions、/v1/embeddings → openai 兼容族（openai/deepseek/moonshot/groq/openrouter/ollama）
//   - /v1/messages                          → anthropic
//
//	"auto/glm-5.2" → model_group='auto' AND model_name='glm-5.2'（同 group+model+provider 族下多凭据负载）
//	"auto"         → model_group='auto'（固定 provider 族下组内所有 model，多模型负载池）
//
// credential_id 为空的部署不参与路由——网关上游 key 唯一来源是 credential_id，空则调用必 503。
func (s *Store) ListModelsByGroup(ctx context.Context, id string, providers []string) ([]*model.Model, error) {
	var list []*model.Model
	q := s.db.WithContext(ctx).
		Where("deleted_at IS NULL AND blocked = 0 AND status = 1 AND credential_id <> ''").
		Where("provider IN ?", providers)
	if idx := strings.Index(id, "/"); idx >= 0 {
		q = q.Where("model_group = ? AND model_name = ?", id[:idx], id[idx+1:])
	} else {
		q = q.Where("model_group = ?", id)
	}
	err := q.Find(&list).Error
	return list, err
}

// GetModelById 取单条部署。
func (s *Store) GetModelById(ctx context.Context, id string) (*model.Model, error) {
	var m model.Model
	err := s.db.WithContext(ctx).Where("model_id = ? and deleted_at is null", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// AllModels 取全部可用部署（/v1/models 与 fallback 用）。
func (s *Store) AllModels(ctx context.Context) ([]*model.Model, error) {
	var list []*model.Model
	err := s.db.WithContext(ctx).
		Where("deleted_at is null and blocked = 0 and status = 1").Find(&list).Error
	return list, err
}

// UpsertHealthCheck 写一条部署健康探测结果（health_check_id = model_id，每部署一行滚动更新）：
// 成功 healthy_count+1、失败 unhealthy_count+1，覆盖 status/error/耗时/时间。
// 唯一索引 idx_aa_llm_health_check_id 兜底并发（探测串行，理论不撞）。
func (s *Store) UpsertHealthCheck(ctx context.Context, modelID, modelName, status, errMsg string, rttMs float64) error {
	healthy, unhealthy := 0, 0
	if status == "healthy" {
		healthy = 1
	} else {
		unhealthy = 1
	}
	return s.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_health_check (health_check_id, model_name, model_id, status, healthy_count, unhealthy_count,
  error_message, response_time_ms, details, checked_by, checked_at, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,'router',now(),now(),now())
ON CONFLICT (health_check_id) DO UPDATE SET
  status = EXCLUDED.status,
  healthy_count = aa_llm_health_check.healthy_count + EXCLUDED.healthy_count,
  unhealthy_count = aa_llm_health_check.unhealthy_count + EXCLUDED.unhealthy_count,
  error_message = EXCLUDED.error_message,
  response_time_ms = EXCLUDED.response_time_ms,
  checked_at = now(),
  updated_at = now()`,
		modelID, modelName, modelID, status, healthy, unhealthy, errMsg, rttMs, "",
	).Error
}

// ---- 共享预算（aa_llm_budget）----

// GetBudgetById 取共享预算（未删除）。
func (s *Store) GetBudgetById(ctx context.Context, id string) (*model.Budget, error) {
	var b model.Budget
	err := s.db.WithContext(ctx).Where("budget_id = ? AND deleted_at IS NULL", id).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// budgetWindow 把 budget_duration 配置解析成窗口时长（"1d"/"7d"/"30d"；"1mo" 按 30 天）。
// 空/未知值返回 ok=false——无周期的预算永不滚存，spend 只增不减。
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

// nextReset 从 from 起把重置点推进到未来（from 在窗口内则 from+dur；已落后多个窗口则连加追平，
// 不丢窗口边界），返回新的 budget_reset_at。
func nextReset(from time.Time, dur time.Duration) time.Time {
	next := from.Add(dur)
	for !next.After(time.Now()) {
		next = next.Add(dur)
	}
	return next
}

// RollBudgetWindow 预算窗口到期懒滚存：行锁下检查 reset_at，到期（或配了周期但未初始化）则
// spend 清零、reset_at 推进到下个窗口；未到期/无周期原样返回。返回滚存后的最新预算。
// 懒执行（读路径触发）而非定时任务：无人使用的预算不滚也没意义，下次访问自然对齐。
func (s *Store) RollBudgetWindow(ctx context.Context, budgetID string) (*model.Budget, error) {
	var out *model.Budget
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var b model.Budget
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("budget_id = ? AND deleted_at IS NULL", budgetID).First(&b).Error; err != nil {
			return err
		}
		out = &b
		dur, ok := budgetWindow(b.BudgetDuration)
		if !ok {
			return nil
		}
		var next time.Time
		if b.BudgetResetAt == nil {
			next = nextReset(time.Now(), dur) // 首次访问初始化窗口，spend 从零起算
		} else {
			if !b.BudgetResetAt.Before(time.Now()) {
				return nil // 未到期
			}
			next = nextReset(*b.BudgetResetAt, dur)
		}
		if err := tx.Model(&model.Budget{}).Where("budget_id = ?", budgetID).Updates(map[string]interface{}{
			"spend": 0, "budget_reset_at": next, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		b.Spend, b.BudgetResetAt = 0, &next
		out = &b
		return nil
	})
	return out, err
}

// RollKeyBudgetWindow key 自身预算周期到期懒滚存（aa_llm_virtual_key.budget_duration）：
// spend 清零 + budget_reset_at 推进。语义同 RollBudgetWindow。
func (s *Store) RollKeyBudgetWindow(ctx context.Context, tokenHash string) (*model.VirtualKey, error) {
	var out *model.VirtualKey
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var k model.VirtualKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND deleted_at IS NULL", tokenHash).First(&k).Error; err != nil {
			return err
		}
		out = &k
		dur, ok := budgetWindow(k.BudgetDuration)
		if !ok {
			return nil
		}
		var next time.Time
		if k.BudgetResetAt == nil {
			next = nextReset(time.Now(), dur)
		} else {
			if !k.BudgetResetAt.Before(time.Now()) {
				return nil
			}
			next = nextReset(*k.BudgetResetAt, dur)
		}
		if err := tx.Model(&model.VirtualKey{}).Where("token_hash = ?", tokenHash).Updates(map[string]interface{}{
			"spend": 0, "budget_reset_at": next, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		k.Spend, k.BudgetResetAt = 0, &next
		out = &k
		return nil
	})
	return out, err
}

// IncrementBudgetSpend 给共享预算的当前窗口花费增量（与 IncrementKeySpend 同步发生在计费路径）。
func (s *Store) IncrementBudgetSpend(ctx context.Context, budgetID string, delta model.Milli) error {
	return s.db.WithContext(ctx).Exec(`UPDATE aa_llm_budget SET spend = spend + ?, updated_at = now() WHERE budget_id = ?`, delta, budgetID).Error
}

// ListGrantBudgets 取全部配了适用范围（grant_rules 非空）的预算。
// 圈人预算数量级很小（个位数~几十），整表载入后由调用方按身份逐份匹配；
// 解析失败/空规则 ('[]') 的行不返回。授权路径每次请求都会调用，由 auth 层做 redis 缓存。
func (s *Store) ListGrantBudgets(ctx context.Context) ([]*model.Budget, error) {
	var rows []*model.Budget
	// 注意：jsonb 列不能与 '' 比较（invalid input syntax for type json），列 NOT NULL DEFAULT '[]'，
	// 空态即 '[]'，用 <> '[]'::jsonb 过滤即可。
	err := s.db.WithContext(ctx).
		Where("deleted_at IS NULL AND grant_rules IS NOT NULL AND grant_rules <> '[]'").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := rows[:0]
	for _, b := range rows {
		if _, err := grantx.ParseGrantRows(b.GrantRules); err == nil {
			out = append(out, b)
		}
	}
	return out, nil
}

// GetBudgetsByIds 按 id 集重查预算（计费增量后内存值已过期，以 DB 为准；低额通知用）。
func (s *Store) GetBudgetsByIds(ctx context.Context, ids []string) ([]*model.Budget, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*model.Budget
	err := s.db.WithContext(ctx).Where("budget_id IN ? AND deleted_at IS NULL", ids).Find(&rows).Error
	return rows, err
}

// TryBudgetNoticeDedup 预算低额/耗尽通知去重：INSERT ON CONFLICT DO NOTHING，
// RowsAffected==1 表示本窗口本档位首次（应发通知）。与 api 侧 llm_repo 同名方法同步维护。
func (s *Store) TryBudgetNoticeDedup(ctx context.Context, userId, budgetId, windowKey, level string) (bool, error) {
	res := s.db.WithContext(ctx).Exec(`
INSERT INTO aa_llm_budget_notice (user_id, budget_id, window_key, level, created_at)
VALUES (?,?,?,?,now())
ON CONFLICT (user_id, budget_id, window_key, level) DO NOTHING`,
		userId, budgetId, windowKey, level)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// GetCredentialAPIKey 取凭据并解密出明文 api_key。
func (s *Store) GetCredentialAPIKey(ctx context.Context, credID string) (string, *model.Credential, error) {
	var c model.Credential
	if err := s.db.WithContext(ctx).Where("credential_id = ? and deleted_at is null", credID).First(&c).Error; err != nil {
		return "", nil, err
	}
	// 先尝试解密（api 端用 AES 加密）；失败则当作明文。
	pt, derr := crypto.Decrypt(c.CredentialValues, s.secretKey)
	if derr != nil || pt == "" {
		pt = c.CredentialValues
	}
	// 兼容两种存储格式：api 端加密的是裸 api_key 字符串；也可能存 {"api_key":"..."} JSON。
	var v model.CredentialValues
	if jerr := json.Unmarshal([]byte(pt), &v); jerr == nil && v.APIKey != "" {
		return v.APIKey, &c, nil
	}
	// 解密结果不是 JSON 对象，就当作裸 api_key。
	if pt != "" {
		return pt, &c, nil
	}
	return "", nil, errors.New("credential api_key empty")
}
