package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// llm_temp_budget 审核类型：临时预算申请（原 aa_llm_budget_request 流程上收统一审核中心）。
// 提交校验/执行/通知文案全部迁自 llm_budget_request_service.go，语义不变：
// 基于命中的固定预算申请临时额度 → 部门负责人审批 → 通过生成申请人专属临时预算行
// （grant_rules 只圈申请人，靠网关 OR 语义叠加在固定预算之上，网关零改动）。

const budgetReqMaxAmount = types.Milli(1_000_000_000) // 单次临时预算金额上限（$100000，防误输入天价）

type LLMTempBudgetHandler struct {
	budgetRepo repository.ILLMBudgetRepository
	dailyRepo  repository.ILLMDailySpendRepository
	postRepo   repository.IPostRepository
}

func NewLLMTempBudgetHandler(b repository.ILLMBudgetRepository, d repository.ILLMDailySpendRepository,
	p repository.IPostRepository) *LLMTempBudgetHandler {
	return &LLMTempBudgetHandler{budgetRepo: b, dailyRepo: d, postRepo: p}
}

// ShouldAutoApprove 免审规则：申请人持全公司最高职级岗位（自己就是审批链顶端，
// 无人可审也无需审）——提交即通过，临时预算当场生成。
func (h *LLMTempBudgetHandler) ShouldAutoApprove(ctx context.Context, t model.Identity) bool {
	if h.postRepo == nil {
		return false
	}
	return h.postRepo.HoldsTopPost(ctx, t.UserId)
}

// llmTempPayload 工单 payload 快照形状（金额走通用列 amount，载荷内不放）。
type llmTempPayload struct {
	BaseBudgetId   string `json:"baseBudgetId"`
	BaseBudgetName string `json:"baseBudgetName"`
	Duration       string `json:"duration"`
	Reason         string `json:"reason"`
}

func llmTempParse(o *entity.ReviewOrder) llmTempPayload {
	var p llmTempPayload
	_ = json.Unmarshal([]byte(o.Payload), &p)
	return p
}

func (h *LLMTempBudgetHandler) Type() string { return "llm_temp_budget" }

// Prepare 提交校验：周期枚举（缺省 30d）、理由必填 ≤500 字、金额 >0 且 ≤上限、
// 申请人当前命中基础预算（显式指定校验命中；缺省取生效上限最大）、命中预算本周期本人有真实消耗。
func (h *LLMTempBudgetHandler) Prepare(ctx context.Context, t model.Identity, params map[string]any) (string, types.Milli, map[string]any, error) {
	duration := paramStr(params, "duration", "")
	switch duration {
	case "1d", "7d", "30d":
	default:
		duration = "30d"
	}
	reason := strings.TrimSpace(paramStr(params, "reason", ""))
	if reason == "" {
		return "", 0, nil, errors.New("请填写申请理由")
	}
	if len([]rune(reason)) > 500 {
		return "", 0, nil, errors.New("申请理由过长（≤500 字）")
	}
	amount := types.MilliFromFloat(paramFloat(params, "amount", 0))
	if amount <= 0 {
		return "", 0, nil, errors.New("申请金额必须大于 0")
	}
	if amount > budgetReqMaxAmount {
		return "", 0, nil, fmt.Errorf("申请金额超过上限 $%.2f", budgetReqMaxAmount.Float())
	}
	base, err := h.resolveBase(ctx, t, paramStr(params, "baseBudgetId", ""))
	if err != nil {
		return "", 0, nil, err
	}
	// 申请前提：命中预算本周期内本人有真实消耗（与 /mine「申请临时预算」按钮同一口径）。
	if budgetWindowSpend(ctx, h.dailyRepo, t.UserId, base) <= 0 {
		return "", 0, nil, errors.New("你在命中预算的当前周期内还没有消费，无需申请临时预算")
	}
	return "临时预算申请·基于「" + base.BudgetName + "」", amount, map[string]any{
		"baseBudgetId": base.BudgetId, "baseBudgetName": base.BudgetName,
		"duration": duration, "reason": reason,
	}, nil
}

// Execute 审批通过执行：建申请人专属临时预算行（OR 叠加在固定预算之上，网关零改动）。
func (h *LLMTempBudgetHandler) Execute(ctx context.Context, t model.Identity, o *entity.ReviewOrder) (string, error) {
	p := llmTempParse(o)
	amount := o.Amount
	b := &entity.LLMBudget{
		BudgetName:     truncateRunes(fmt.Sprintf("临时预算·%s·%s", p.BaseBudgetName, o.ApplicantName), 100), // 复用 agent_runtime_service 的同名 helper
		MaxBudget:      &amount,
		Spend:          0,
		BudgetDuration: p.Duration,
		// BudgetResetAt 留 nil：api 与网关都按"窗口从现在起算"懒初始化（两侧 RollBudgetWindow 同构）。
		GrantRules: fmt.Sprintf(`[{"userIds":[%q]}]`, o.ApplicantId),
		CreatedBy:  t.UserId,
		UpdatedBy:  t.UserId,
	}
	return h.budgetRepo.Add(ctx, b)
}

// 通知文案（迁自旧 service 原文，落款指到审核中心新入口）。

func (h *LLMTempBudgetHandler) PendingNotice(o *entity.ReviewOrder) (string, string) {
	p := llmTempParse(o)
	return "临时预算申请待审批",
		fmt.Sprintf("「%s」基于预算「%s」申请临时预算 $%.2f（%s）。理由：%s。请在审核系统-我的审核处理。",
			o.ApplicantName, p.BaseBudgetName, o.Amount.Float(), durationText(p.Duration), p.Reason)
}

func (h *LLMTempBudgetHandler) ApprovedNotice(o *entity.ReviewOrder) (string, string, string) {
	p := llmTempParse(o)
	name := truncateRunes(fmt.Sprintf("临时预算·%s·%s", p.BaseBudgetName, o.ApplicantName), 100)
	return "临时预算已通过",
		fmt.Sprintf("你基于预算「%s」申请的临时预算 $%.2f（%s）已通过，临时预算「%s」已生效并叠加在固定预算之上（任一预算有余量即放行）。可在审核系统-我的提交查看进度。",
			p.BaseBudgetName, o.Amount.Float(), durationText(p.Duration), name),
		"/llm/budget/mine"
}

func (h *LLMTempBudgetHandler) RejectedNotice(o *entity.ReviewOrder, note string) (string, string, string) {
	p := llmTempParse(o)
	return "临时预算被驳回",
		fmt.Sprintf("你基于预算「%s」申请的临时预算 $%.2f（%s）被驳回。理由：%s",
			p.BaseBudgetName, o.Amount.Float(), durationText(p.Duration), orDefault(note, "无")),
		"/llm/budget/mine"
}

// resolveBase 定基础预算：显式指定则校验申请人当前命中（防空投他人 budget_id）；
// 缺省自动取命中预算中生效上限最大者（不限额视为 ∞，并列取已用多的），与 /mine 的
// 「命中」标记同一口径。一条都没命中说明当前无人对其限额，无从叠加，直接报错。
func (h *LLMTempBudgetHandler) resolveBase(ctx context.Context, t model.Identity, baseBudgetId string) (*entity.LLMBudget, error) {
	if baseBudgetId != "" {
		base, err := h.budgetRepo.Get(ctx, baseBudgetId)
		if err != nil {
			return nil, errors.New("基础预算不存在")
		}
		rows, perr := model.ParseGrantRows(base.GrantRules)
		if perr != nil || !model.MatchAny(rows, t) {
			return nil, errors.New("你未命中该预算，不能基于它申请临时预算")
		}
		return base, nil
	}
	budgets, err := h.budgetRepo.ListGrantBudgets(ctx)
	if err != nil {
		return nil, err
	}
	var best *entity.LLMBudget
	for _, b := range budgets {
		rows, perr := model.ParseGrantRows(b.GrantRules)
		if perr != nil || !model.MatchAny(rows, t) {
			continue
		}
		if best == nil || budgetCap(b) > budgetCap(best) ||
			(budgetCap(b) == budgetCap(best) && b.Spend > best.Spend) {
			best = b
		}
	}
	if best == nil {
		return nil, errors.New("你当前未命中任何预算，无需申请临时预算")
	}
	return best, nil
}

// budgetCap 生效硬上限：nil/0 视为不限额（∞ 用 MaxInt64 表示，仅参与比较不做算术），
// 与 MyBudgets/前端口径一致。
func budgetCap(b *entity.LLMBudget) types.Milli {
	if b.MaxBudget != nil && *b.MaxBudget > 0 {
		return *b.MaxBudget
	}
	return types.Milli(math.MaxInt64)
}

func durationText(d string) string {
	switch d {
	case "1d":
		return "1 天"
	case "7d":
		return "7 天"
	case "30d":
		return "30 天"
	}
	return d
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
