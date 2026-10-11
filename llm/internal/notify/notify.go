// Package notify 预算低额/耗尽站内通知：经 IAMDB 直写 aiarsenal 库 aa_notice
// （UI 右上角【通知】铃铛读取，link 走内部路由跳转）。
// 网关与 api 是两个独立 Go module，api 侧发通知走 notice_service.Push——两处文案/档位
// 判定保持一致（api 侧见 application/service/llm_config_service.go notifyBudgetLevel，改任一侧须同步）。
package notify

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aiarsenal/llm/internal/db"
	"aiarsenal/llm/internal/store"
)

// 通知档位（aa_llm_budget_notice.level 取值，与 api 侧同步）。
const (
	LevelLow       = "low"       // 剩余 <10%（spend/max ≥ 90%）
	LevelExhausted = "exhausted" // 已用尽（≥100%）
)

// noticeID 32 位业务 id（首尾字母，语义同 api 侧 infrastructure/idgen）。
func noticeID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	const alnum = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 32)
	if _, err := rand.Read(b); err == nil {
		b[0] = letters[int(b[0])%len(letters)]
		for i := 1; i < 31; i++ {
			b[i] = alnum[int(b[i])%len(alnum)]
		}
		b[31] = letters[int(b[31])%len(letters)]
		return string(b)
	}
	// rand 失败极少见：时间戳兜底（aa_notice.notice_id 唯一索引，冲突仅丢本条通知）
	return fmt.Sprintf("n%d", time.Now().UnixNano())
}

// PushUserNotice 给 userId 发一条个人通知（aa_notice，created_by='0' 系统）。
// IAMDB 未配置/不可用时返回错误，调用方记日志即可——通知丢失不影响主链路（fail-open）。
func PushUserNotice(ctx context.Context, userId, title, content, link, ntype string) error {
	if userId == "" {
		return nil
	}
	iam := db.IAMDB()
	if iam == nil {
		return errors.New("notify: iam db not ready")
	}
	if ntype == "" {
		ntype = "info"
	}
	return iam.WithContext(ctx).Exec(`
INSERT INTO aa_notice (notice_id, user_id, title, content, link, type, is_read, created_by, created_at, updated_at)
VALUES (?,?,?,?,?,?,0,'0',now(),now())`,
		noticeID(), userId, title, content, link, ntype).Error
}

// BudgetAlerts 预算低额/耗尽提醒主入口（计费路径与 403 拒流路径共用）：
// 按 id 重查预算当前值（增量计费后内存值已过期），spend/max ≥90% 发低额 warning、
// ≥100% 发耗尽 error；aa_llm_budget_notice 按 user+budget+window+level 去重，
// 每窗口每人每预算每档至多一条（窗口 key：有周期=reset_at，无周期=当月）。
// 全程 best-effort：任一步失败只记日志。在 goroutine 里调用，自带 recover。
func BudgetAlerts(ctx context.Context, st *store.Store, userId string, budgetIDs []string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("预算低额通知 panic", "user", userId, "panic", r)
		}
	}()
	if userId == "" || len(budgetIDs) == 0 {
		return
	}
	budgets, err := st.GetBudgetsByIds(ctx, budgetIDs)
	if err != nil {
		slog.Warn("预算低额通知：预算重查失败", "user", userId, "err", err)
		return
	}
	for _, b := range budgets {
		if b.MaxBudget == nil || *b.MaxBudget <= 0 {
			continue // 不限额预算永不触警
		}
		pct := float64(b.Spend) / float64(*b.MaxBudget)
		level := ""
		switch {
		case pct >= 1:
			level = LevelExhausted
		case pct >= 0.9:
			level = LevelLow
		default:
			continue
		}
		windowKey := time.Now().Format("200601")
		if b.BudgetResetAt != nil {
			windowKey = b.BudgetResetAt.Format(time.RFC3339)
		}
		first, derr := st.TryBudgetNoticeDedup(ctx, userId, b.BudgetId, windowKey, level)
		if derr != nil {
			slog.Warn("预算低额通知：去重失败", "budget", b.BudgetId, "user", userId, "err", derr)
			continue
		}
		if !first {
			continue
		}
		remain := *b.MaxBudget - b.Spend
		if remain < 0 {
			remain = 0
		}
		var title, content, ntype string
		if level == LevelExhausted {
			title, ntype = "预算已用尽", "error"
			content = fmt.Sprintf("预算「%s」本周期额度已用尽（$%.2f / $%.2f），命中此预算的 AI 调用将被拒绝。", b.BudgetName, b.Spend.Float(), b.MaxBudget.Float())
		} else {
			title, ntype = "预算余额不足", "warning"
			content = fmt.Sprintf("预算「%s」本周期仅剩 %.1f%%（剩余 $%.2f / 上限 $%.2f），用尽后命中此预算的 AI 调用将被拒绝。", b.BudgetName, (1-pct)*100, remain.Float(), b.MaxBudget.Float())
		}
		if perr := PushUserNotice(ctx, userId, title, content, "/llm/budget/my", ntype); perr != nil {
			slog.Warn("预算低额通知：写入失败", "budget", b.BudgetId, "user", userId, "err", perr)
		}
	}
}
