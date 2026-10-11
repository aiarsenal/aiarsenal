package routing

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/llm/internal/model"
)

// Strategy 路由策略接口。
// Pick 从候选部署中选一个；若全部不可用返回 error（由上层走 fallback）。
type Strategy interface {
	Pick(ctx context.Context, candidates []*model.Model) (*model.Model, error)
	// PickExcluding 单请求内跨凭据 failover 用：选一个未在 tried 中、且未冷却的候选；
	// 若剩余全部在冷却则放宽到冷却中的（best-effort 试一次，对齐 Pick 全冷却回退语义）；剩余为空（全试过）返回 nil。
	PickExcluding(ctx context.Context, candidates []*model.Model, tried map[string]struct{}) (*model.Model, error)
	// MarkCooldown 失败部署置默认冷却（到期自动恢复路由）。
	MarkCooldown(ctx context.Context, modelID string)
	// MarkCooldownFor 失败部署置指定时长冷却（上游 Retry-After 用），到期自动恢复路由。
	MarkCooldownFor(ctx context.Context, modelID string, dur time.Duration)
	// MarkUnhealthy 健康探测持续失败的部署置不健康：路由跳过，直到探测恢复（ClearUnhealthy）。
	// 与冷却互补：冷却是传输层失败的短期退避（自动过期），不健康是探测确认的持续态（显式清除）。
	MarkUnhealthy(ctx context.Context, modelID string)
	// ClearUnhealthy 健康探测恢复后解除跳过，部署重新参与路由。
	ClearUnhealthy(ctx context.Context, modelID string)
}

// Weighted 按 model.Weight 做加权随机分发（归一化 + 加权随机），
// 叠加部署冷却（cooldown）——失败部署 cool 时间内跳过，实现传输层故障转移。
// weight<=0 按 1 计；候选仅 1 条直接返回。
type Weighted struct {
	rdb  *redis.Client
	cool time.Duration
}

func NewWeighted(rdb *redis.Client) *Weighted {
	return &Weighted{rdb: rdb, cool: defaultCooldown()}
}

// defaultCooldown 部署失败后的默认冷却时长（env LLM_COOLDOWN_SECONDS 覆盖，默认 5s）。
// 冷却到期后部署自动重新参与路由，即"凭据恢复后自动恢复使用"。
// 取短值：transient 错误（限流抖动）能尽快恢复；上游真·长期不可用时会带 Retry-After 走 MarkCooldownFor。
func defaultCooldown() time.Duration {
	if s := os.Getenv("LLM_COOLDOWN_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 5 * time.Second
}

func (r *Weighted) Pick(ctx context.Context, candidates []*model.Model) (*model.Model, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no deployments available")
	}
	// 剔除冷却中/不健康的部署
	usable := make([]*model.Model, 0, len(candidates))
	for _, m := range candidates {
		if !r.inCooldown(ctx, m.ModelId) && !r.isUnhealthy(ctx, m.ModelId) {
			usable = append(usable, m)
		}
	}
	// 全部不可路由：退而取候选第一个（让请求试一次，失败再延长冷却）
	if len(usable) == 0 {
		return candidates[0], nil
	}
	return weightedPick(usable), nil
}

// PickExcluding 单请求内跨凭据 failover：先排除本请求已试过的 tried，再排除冷却中的；
// 剩余全在冷却则放宽（试一次，对齐 Pick 全冷却回退），剩余为空（全试过）返回 nil。
func (r *Weighted) PickExcluding(ctx context.Context, candidates []*model.Model, tried map[string]struct{}) (*model.Model, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no deployments available")
	}
	remaining := make([]*model.Model, 0, len(candidates))
	for _, m := range candidates {
		if _, ok := tried[m.ModelId]; ok {
			continue
		}
		remaining = append(remaining, m)
	}
	if len(remaining) == 0 {
		return nil, nil // 本请求内所有候选都已试过
	}
	usable := make([]*model.Model, 0, len(remaining))
	for _, m := range remaining {
		if !r.inCooldown(ctx, m.ModelId) && !r.isUnhealthy(ctx, m.ModelId) {
			usable = append(usable, m)
		}
	}
	if len(usable) == 0 {
		usable = remaining // 全冷却/不健康：放宽，best-effort 试一次
	}
	return weightedPick(usable), nil
}

// weightedPick 在候选中按 Weight 加权随机选一个（weight<=0 当 1；单条直返）。
func weightedPick(usable []*model.Model) *model.Model {
	if len(usable) == 1 {
		return usable[0]
	}
	total := 0
	for _, m := range usable {
		w := m.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total <= 0 {
		return usable[rand.Intn(len(usable))]
	}
	pick := rand.Intn(total)
	for _, m := range usable {
		w := m.Weight
		if w <= 0 {
			w = 1
		}
		pick -= w
		if pick < 0 {
			return m
		}
	}
	return usable[len(usable)-1]
}

func (r *Weighted) inCooldown(ctx context.Context, modelID string) bool {
	if r.rdb == nil {
		return false
	}
	n, err := r.rdb.Exists(ctx, "cooldown:"+modelID).Result()
	return err == nil && n > 0
}

// MarkCooldown 在某部署失败后置默认冷却。冷却到期自动恢复路由。
func (r *Weighted) MarkCooldown(ctx context.Context, modelID string) {
	if r.rdb == nil {
		return
	}
	_ = r.rdb.Set(ctx, "cooldown:"+modelID, "1", r.cool).Err()
}

// MarkCooldownFor 置指定时长的冷却：上游 429 带 Retry-After 头时按其建议时长冷却，
// 让"过会儿又能用"的凭据按上游节奏恢复，不卡死也不浪试。dur<=0 不生效。
func (r *Weighted) MarkCooldownFor(ctx context.Context, modelID string, dur time.Duration) {
	if r.rdb == nil || dur <= 0 {
		return
	}
	_ = r.rdb.Set(ctx, "cooldown:"+modelID, "1", dur).Err()
}

// MarkUnhealthy 探测确认持续失败的部署置不健康：路由跳过（无 TTL，持续态），
// 直到探测成功显式 ClearUnhealthy。全候选不健康时路由层会放宽 best-effort，不会彻底拒流。
func (r *Weighted) MarkUnhealthy(ctx context.Context, modelID string) {
	if r.rdb == nil {
		return
	}
	_ = r.rdb.Set(ctx, unhealthyKey(modelID), "1", 0).Err()
}

// ClearUnhealthy 探测恢复后解除跳过。
func (r *Weighted) ClearUnhealthy(ctx context.Context, modelID string) {
	if r.rdb == nil {
		return
	}
	_ = r.rdb.Del(ctx, unhealthyKey(modelID)).Err()
}

func unhealthyKey(modelID string) string { return "unhealthy:" + modelID }

func (r *Weighted) isUnhealthy(ctx context.Context, modelID string) bool {
	if r.rdb == nil {
		return false
	}
	n, err := r.rdb.Exists(ctx, unhealthyKey(modelID)).Result()
	return err == nil && n > 0
}
