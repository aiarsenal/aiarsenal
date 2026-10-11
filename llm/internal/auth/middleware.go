package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"aiarsenal/llm/internal/crypto"
	"aiarsenal/llm/internal/grantx"
	"aiarsenal/llm/internal/identity"
	"aiarsenal/llm/internal/model"
	"aiarsenal/llm/internal/notify"
	"aiarsenal/llm/internal/store"
)

// Key 上下文中携带的鉴权后的 virtual key。
type Key struct {
	*model.VirtualKey
}

// AuthMiddleware 校验 Authorization: Bearer <key>。
// 虚拟 Key 鉴权：sha256(明文) → 查库（带 redis 缓存 5s）→
// 校验 blocked/expires/spend<budget → 注入 context。
func AuthMiddleware(st *store.Store, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		if token == "" {
			fail(c, http.StatusUnauthorized, "missing_api_key", "Missing Authorization Bearer token")
			return
		}
		hash := crypto.HashToken(token)
		k, err := loadKey(c, st, rdb, hash)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid_api_key", "Invalid API key")
			return
		}
		if k.Blocked == 1 {
			fail(c, http.StatusForbidden, "blocked_key", "API key blocked")
			return
		}
		if k.Expires != nil && k.Expires.Before(time.Now()) {
			fail(c, http.StatusForbidden, "expired_key", "API key expired")
			return
		}
		// key 自身预算周期到期 → 懒滚存（spend 清零、reset_at 推进）后再判定额度。
		if k.BudgetDuration != "" && (k.BudgetResetAt == nil || k.BudgetResetAt.Before(time.Now())) {
			if nk, err := st.RollKeyBudgetWindow(c, hash); err == nil && nk != nil {
				bustKeyCache(c, rdb, hash)
				k = nk
			}
		}
		if k.MaxBudget != nil && k.Spend >= *k.MaxBudget {
			fail(c, http.StatusForbidden, "budget_exceeded", "Max budget exceeded")
			return
		}
		// 共享预算：多 key 经 budget_id 共用同一额度，超限同样拒流。
		if k.BudgetId != "" && !sharedBudgetOk(c, st, rdb, k.BudgetId) {
			return
		}
		// 用户权限组预算（OR 语义）：预算 grant_rules 圈人，命中多份时任一有余量即放行；
		// 命中集合经 context 传给计费路径逐份扣减。
		if !userBudgetsOk(c, st, rdb, k.UserId) {
			return
		}
		c.Set("key", &Key{VirtualKey: k})
		// context 存规范 token_hash（行上的 token_hash 列）。轮换宽限期内用旧明文
		// 命中同一行时，k.TokenHash 是新哈希，计费/活跃/日聚合都应打到规范行。
		c.Set("token_hash", k.TokenHash)
		c.Next()
	}
}

func extractBearer(c *gin.Context) string {
	// OpenAI 风格：Authorization: Bearer <key>
	h := c.GetHeader("Authorization")
	if h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && (strings.EqualFold(parts[0], "bearer") || strings.EqualFold(parts[0], "basic")) {
			return strings.TrimSpace(parts[1])
		}
		if strings.TrimSpace(h) != "" {
			return strings.TrimSpace(h)
		}
	}
	// Anthropic 风格：x-api-key: <key>
	if k := strings.TrimSpace(c.GetHeader("x-api-key")); k != "" {
		return k
	}
	// Gemini 风格：x-goog-api-key 头 / ?key= 查询参数（/v1beta 原生端点的客户端习惯，优先级最后）
	if k := strings.TrimSpace(c.GetHeader("x-goog-api-key")); k != "" {
		return k
	}
	if k := strings.TrimSpace(c.Query("key")); k != "" {
		return k
	}
	return ""
}

// vkeyCache 缓存条目：VirtualKey.TokenHash 带 json:"-"（防哈希随任何 JSON 外泄），
// 直接 Marshal 缓存会把 token_hash 丢掉——缓存命中回读恒为空，计费/日聚合/last_active
// 全打到空 key 上。缓存里显式补一份 token_hash（外层浅字段，序列化时优先生效）。
type vkeyCache struct {
	model.VirtualKey
	TokenHash string `json:"token_hash"`
}

// loadKey 带 5s redis 缓存（key:vkey:<hash>）。
func loadKey(ctx context.Context, st *store.Store, rdb *redis.Client, hash string) (*model.VirtualKey, error) {
	if rdb != nil {
		key := "vkey:" + hash
		if v, err := rdb.Get(ctx, key).Result(); err == nil {
			var c vkeyCache
			if json.Unmarshal([]byte(v), &c) == nil {
				k := c.VirtualKey
				k.TokenHash = c.TokenHash
				return &k, nil
			}
		}
	}
	k, err := st.GetKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if rdb != nil {
		if b, err := json.Marshal(vkeyCache{VirtualKey: *k, TokenHash: k.TokenHash}); err == nil {
			_ = rdb.Set(ctx, "vkey:"+hash, b, 5*time.Second).Err()
		}
	}
	return k, nil
}

// sharedBudgetOk 校验共享预算额度：加载预算（带 5s 缓存，与 key 缓存同节奏）→
// 周期到期先懒滚存 → max_budget 内返回 true，超限 fail 并返回 false。
// 预算不存在/查询失败放行（配置缺失不应把所有挂靠 key 打死，错误有 spend_log 可查）。
func sharedBudgetOk(c *gin.Context, st *store.Store, rdb *redis.Client, budgetID string) bool {
	b, err := loadBudget(c, st, rdb, budgetID)
	if err != nil || b == nil {
		return true
	}
	if b.BudgetDuration != "" && (b.BudgetResetAt == nil || b.BudgetResetAt.Before(time.Now())) {
		if nb, err := st.RollBudgetWindow(c, budgetID); err == nil && nb != nil {
			bustBudgetCache(c, rdb, budgetID)
			b = nb
		}
	}
	if b.MaxBudget != nil && b.Spend >= *b.MaxBudget {
		fail(c, http.StatusForbidden, "budget_exceeded", "Shared budget exceeded")
		return false
	}
	return true
}

// userBudgetsOk 用户权限组预算判定（OR 语义）：预算自身 grant_rules 圈人，
// key 归属用户命中多份预算时任一有余量即放行，全部超限才拒流。
//   - userId 空（纯团队/项目 key）：无用户主体，不受圈人预算约束，放行
//   - 命中空集：用户不在任何圈人预算范围内，放行
//   - max_budget NULL=不限额；命中预算周期到期先懒滚存再比额度
// 身份/预算解析失败放行 + 记日志（fail-open，与 sharedBudgetOk「配置缺失不打死」同语义）。
// 命中集合（含未超额与已超额的，只要放行就全额计）经 c.Set 传给计费路径。
func userBudgetsOk(c *gin.Context, st *store.Store, rdb *redis.Client, userId string) bool {
	if userId == "" {
		return true
	}
	t, err := identity.Resolve(c, userId)
	if err != nil {
		slog.Error("预算圈人判定：用户身份解析失败，放行", "user", userId, "err", err)
		return true
	}
	budgets := loadGrantBudgets(c, st, rdb)
	matched := make([]*model.Budget, 0, 4)
	for _, b := range budgets {
		rows, perr := grantx.ParseGrantRows(b.GrantRules)
		if perr != nil {
			continue
		}
		if grantx.MatchAny(rows, t) {
			matched = append(matched, b)
		}
	}
	if len(matched) == 0 {
		return true
	}
	ids := make([]string, 0, len(matched))
	hasRoom := false
	for _, b := range matched {
		cur := b
		if b.BudgetDuration != "" && (b.BudgetResetAt == nil || b.BudgetResetAt.Before(time.Now())) {
			if nb, rerr := st.RollBudgetWindow(c, b.BudgetId); rerr == nil && nb != nil {
				bustBudgetCache(c, rdb, b.BudgetId)
				cur = nb
			}
		}
		if cur.MaxBudget == nil || cur.Spend < *cur.MaxBudget {
			hasRoom = true
		}
		ids = append(ids, b.BudgetId)
	}
	if !hasRoom {
		// 全部超限拒流：给用户发「预算已用尽」站内通知（异步，不阻塞 403 返回；
		// gin.Context 请求返回即回收，goroutine 里不能用 c）。
		alertIds := append([]string(nil), ids...)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			notify.BudgetAlerts(ctx, st, userId, alertIds)
		}()
		fail(c, http.StatusForbidden, "budget_exceeded", "User budgets all exceeded")
		return false
	}
	c.Set("user_budget_ids", ids)
	return true
}

// loadGrantBudgets 全部圈人预算，带 5s redis 缓存（vbudgets:grant，与 vkey/vbudget 同节奏）。
// 查询失败返回 nil（调用方按命中空集=放行处理，fail-open）。
func loadGrantBudgets(ctx context.Context, st *store.Store, rdb *redis.Client) []*model.Budget {
	if rdb != nil {
		if v, err := rdb.Get(ctx, "vbudgets:grant").Result(); err == nil {
			var bs []*model.Budget
			if json.Unmarshal([]byte(v), &bs) == nil {
				return bs
			}
		}
	}
	bs, err := st.ListGrantBudgets(ctx)
	if err != nil {
		slog.Error("预算圈人判定：圈人预算加载失败", "err", err)
		return nil
	}
	if rdb != nil {
		if b, err := json.Marshal(bs); err == nil {
			_ = rdb.Set(ctx, "vbudgets:grant", b, 5*time.Second).Err()
		}
	}
	return bs
}

// loadBudget 带 5s redis 缓存（vbudget:<id>）。
func loadBudget(ctx context.Context, st *store.Store, rdb *redis.Client, id string) (*model.Budget, error) {
	if rdb != nil {
		key := "vbudget:" + id
		if v, err := rdb.Get(ctx, key).Result(); err == nil {
			var b model.Budget
			if json.Unmarshal([]byte(v), &b) == nil {
				return &b, nil
			}
		}
	}
	b, err := st.GetBudgetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if rdb != nil {
		if bs, err := json.Marshal(b); err == nil {
			_ = rdb.Set(ctx, "vbudget:"+id, bs, 5*time.Second).Err()
		}
	}
	return b, nil
}

func bustKeyCache(ctx context.Context, rdb *redis.Client, hash string) {
	if rdb != nil {
		_ = rdb.Del(ctx, "vkey:"+hash).Err()
	}
}

func bustBudgetCache(ctx context.Context, rdb *redis.Client, id string) {
	if rdb != nil {
		_ = rdb.Del(ctx, "vbudget:"+id).Err()
	}
}

// ModelAllowed 检查请求的 model 是否在该 key 允许范围内。
// allowed_models 为空/"All" 表示不限制；否则 id 必须在列表中。
// id 形如 "group/model" 或裸 "group"（provider 由端点定，不入此列表）。
// 规则：精确命中；或 "all"；或分组池放行其成员——allowed 含 "auto" 时 "auto/glm-5.2" 也放行（反向不放行）。
func ModelAllowed(k *model.VirtualKey, reqModel string) bool {
	if reqModel == "" {
		return true
	}
	if k.AllowedModels == "" || k.AllowedModels == "[]" {
		return true
	}
	var allowed []string
	if err := json.Unmarshal([]byte(k.AllowedModels), &allowed); err != nil {
		return true
	}
	for _, m := range allowed {
		if m == reqModel || strings.EqualFold(m, "all") || strings.EqualFold(m, "All") {
			return true
		}
		// 分组池覆盖成员：allowed="auto" 放行 "auto/xxx"
		if strings.HasPrefix(reqModel, m+"/") {
			return true
		}
		// 反向：allowed 含具体 "auto/glm"，调裸 "auto" 也放行（分发阶段 FilterAllowedModels 再按 allowed 收窄）
		if !strings.Contains(reqModel, "/") && strings.HasPrefix(m, reqModel+"/") {
			return true
		}
	}
	return false
}

// FilterAllowedModels 在候选部署中只保留该 key 允许的：
//   - allowed_models 空 / ["all"] → 不限制
//   - bare group 在 allowed 内 → 该组全部部署
//   - 具体 "group/name" 在 allowed 内 → 仅该部署
//
// 用于按 model_group 分发时收窄到当前 ak 实际支持的模型，避免分到 ak 未授权的模型。
func FilterAllowedModels(cands []*model.Model, k *model.VirtualKey) []*model.Model {
	if k.AllowedModels == "" || k.AllowedModels == "[]" {
		return cands
	}
	var allowed []string
	if err := json.Unmarshal([]byte(k.AllowedModels), &allowed); err != nil {
		return cands
	}
	hasAll := false
	groupSet := map[string]bool{}
	nameSet := map[string]bool{}
	for _, m := range allowed {
		switch {
		case strings.EqualFold(m, "all") || strings.EqualFold(m, "All"):
			hasAll = true
		case !strings.Contains(m, "/"):
			groupSet[m] = true
		default:
			nameSet[m] = true
		}
	}
	if hasAll {
		return cands
	}
	out := make([]*model.Model, 0, len(cands))
	for _, c := range cands {
		if groupSet[c.ModelGroup] || nameSet[c.ModelGroup+"/"+c.ModelName] {
			out = append(out, c)
		}
	}
	return out
}

func fail(c *gin.Context, code int, errType, msg string) {
	c.AbortWithStatusJSON(code, gin.H{
		"error": gin.H{"type": errType, "message": msg},
	})
}

// TouchActive 更新 last_active（fire-and-forget）。
func TouchActive(st *store.Store, hash string) {
	if hash == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = st.DB().WithContext(ctx).Model(&model.VirtualKey{}).
			Where("token_hash = ?", hash).UpdateColumn("last_active", time.Now()).Error
	}()
}

func init() { _ = fmt.Sprint }
