package proxy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"aiarsenal/llm/internal/auth"
	"aiarsenal/llm/internal/model"
	"aiarsenal/llm/internal/store"
)

// F-024 预算竞态修复：判读旧值放行 + 响应后才记账的链路被 20 并发实测刷出 21 倍。
// 修复=发送上游前按估算成本「原子预留」（判定与扣减同一 UPDATE，无缓存读），请求
// 终局对账（退还预留、真实成本经既有 Increment*Spend 异步入账）。
//   - 预留量：prompt 按 rune 数（CJK≈1 token/字，英文高估——高估方向=多预留后退还），
//     输出按请求 max_tokens（缺省 4096）。
//   - 失败语义 fail-closed：预留 SQL 出错拒绝请求（原「预算查询失败放行」口径废弃）；
//     预算余量不足 403。
//   - 未配价模型（in/out 均为 0）无法估成本，不预留（计费亦为 0，无放大面）。
//   - 用户权限组预算（圈人 OR 语义）暂不预留：OR 放行 + 全量扣减的语义下预留量没有
//     无歧义定义，维持鉴权层缓存预检 + 异步扣减现状，残余竞态如实披露为后续项。

// spendReservation 一次请求的预留状态：refund 各预算维度一次性（sync.Once 语义由
// mutex + bool 实现），供 bill（每 attempt 调用）与入口 defer（无 bill 兜底）竞争收敛。
type spendReservation struct {
	st *store.Store

	keyHash   string
	keyEst    model.Milli
	sharedID  string
	sharedEst model.Milli

	keyOnce, sharedOnce sync.Once
}

// refundKey 退还 key 预算预留（幂等：仅首次生效）。
func (r *spendReservation) refundKey(ctx context.Context) {
	if r == nil || r.keyEst == 0 {
		return
	}
	r.keyOnce.Do(func() {
		if err := r.st.RefundKeySpend(ctx, r.keyHash, r.keyEst); err != nil {
			slog.Error("key 预算预留退还失败（spend 将虚高，等下一窗口滚存）", "hash", r.keyHash, "est", r.keyEst, "err", err)
		}
	})
}

// refundShared 退还共享预算预留（幂等）。
func (r *spendReservation) refundShared(ctx context.Context) {
	if r == nil || r.sharedEst == 0 {
		return
	}
	r.sharedOnce.Do(func() {
		if err := r.st.RefundBudgetSpend(ctx, r.sharedID, r.sharedEst); err != nil {
			slog.Error("共享预算预留退还失败", "budget", r.sharedID, "est", r.sharedEst, "err", err)
		}
	})
}

// settleAll 请求终局兜底：bill 未触达的路径（无候选成功也必有 failed bill，此处为
// 防御性兜底）退还全部预留。
func (r *spendReservation) settleAll(ctx context.Context) {
	r.refundKey(ctx)
	r.refundShared(ctx)
}

// estimateRequestMilli 请求成本预估（milli）。
func estimateRequestMilli(body []byte, inputPerM, outputPerM model.Milli) model.Milli {
	if inputPerM <= 0 && outputPerM <= 0 {
		return 0
	}
	prompt := utf8.RuneCount(body)
	outTok := 4096
	var req struct {
		MaxTokens int `json:"max_tokens"`
	}
	if json.Unmarshal(body, &req) == nil && req.MaxTokens > 0 {
		outTok = req.MaxTokens
	}
	return model.CostMilli(prompt, outTok, inputPerM, outputPerM)
}

// budgetFail 预算类失败响应（OpenAI 错误形态，与 auth 包的 fail 同形——该函数未导出）。
func budgetFail(c *gin.Context, code int, errType, msg string) {
	c.AbortWithStatusJSON(code, gin.H{
		"error": gin.H{"type": errType, "message": msg},
	})
}

// reserveFor 发送上游前的预算预留：key 自身预算 + 挂靠共享预算逐份原子预留。
// 失败（DB 错误）fail-closed 503；余量不足 403。成功返回预留状态（bill 与入口
// defer 依据它做终局对账），并写入 context。
func (h *Handler) reserveFor(c *gin.Context, k *auth.Key, hash string, body []byte, m *model.Model) (*spendReservation, bool) {
	est := estimateRequestMilli(body, m.InputPricePerMillion, m.OutputPricePerMillion)
	rs := &spendReservation{st: h.st, keyHash: hash}
	if k.MaxBudget != nil && est > 0 {
		ok, err := h.st.ReserveKeySpend(c, hash, est)
		if err != nil {
			slog.Error("key 预算预留失败（fail-closed）", "hash", hash, "err", err)
			budgetFail(c, http.StatusServiceUnavailable, "budget_unavailable", "Budget check unavailable, request refused")
			return nil, false
		}
		if !ok {
			budgetFail(c, http.StatusForbidden, "budget_exceeded", "Max budget exceeded")
			return nil, false
		}
		rs.keyEst = est
	}
	if k.BudgetId != "" && est > 0 {
		ok, err := h.st.ReserveBudgetSpend(c, k.BudgetId, est)
		if err != nil {
			slog.Error("共享预算预留失败（fail-closed）", "budget", k.BudgetId, "err", err)
			rs.refundKey(c)
			budgetFail(c, http.StatusServiceUnavailable, "budget_unavailable", "Budget check unavailable, request refused")
			return nil, false
		}
		if !ok {
			// 0 行更新分两种：余量不足（403）与预算行不存在/已删除（鉴权层
			// sharedBudgetOk 对「配置缺失」的既定语义是放行——挂靠失效 budget_id
			// 的 key 不应被预留层打死，两层语义在此对齐）。用存在性检查区分。
			b, berr := h.st.GetBudgetById(c, k.BudgetId)
			if berr != nil || b == nil {
				slog.Warn("挂靠共享预算不存在，预留跳过放行（与鉴权层配置缺失语义对齐）", "budget", k.BudgetId)
				c.Set("spend_reservation", rs)
				return rs, true
			}
			rs.refundKey(c)
			budgetFail(c, http.StatusForbidden, "budget_exceeded", "Shared budget exceeded")
			return nil, false
		}
		rs.sharedID = k.BudgetId
		rs.sharedEst = est
	}
	c.Set("spend_reservation", rs)
	return rs, true
}
