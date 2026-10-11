package ops

// 预算域（aa_llm，llm 库）：我的预算只读 + 临时预算申请全流程读写。
// 读端点 platform-budget（/mcp/budget）、写端点 platform-budget-write（/mcp/budget-write）。
// 与 Web【我的预算】/【审核系统·我的提交/我的审核】同一套 service
// （LLMConfigService / ReviewOrderService，order_type=llm_temp_budget）。
// 身份锁定同通知域：有调用方身份（网关令牌交换）一律锁定为 actor 本人；系统身份
// （console 探针/stdio 冒烟）才允许显式 user_id——网关路径恒有 actor，安全不降。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/persistence"
)

// budgetSelfIdentity 解析「我的预算」类工具的调用方身份：actor 锁定，系统身份显式传 user_id
// （按该用户组装六维身份，圈人判定与网关路径同构）。
func budgetSelfIdentity(ctx context.Context, a map[string]any) (*model.Identity, error) {
	if t := identityFromCtx(ctx); t != nil {
		return t, nil
	}
	uid := argStr(a, "user_id", "")
	if uid == "" {
		return nil, fmt.Errorf("缺少调用方身份，须显式传 user_id")
	}
	return identityByUserId(uid), nil
}

// budgetDurationName 周期枚举转中文
func budgetDurationName(d string) string {
	switch d {
	case "1d":
		return "日"
	case "7d":
		return "周"
	case "30d", "1mo":
		return "月"
	}
	return "无周期（累计）"
}

// budgetReqStatusName 工单状态转中文
func budgetReqStatusName(s string) string {
	switch s {
	case "pending":
		return "待审批"
	case "approved":
		return "已通过"
	case "rejected":
		return "已驳回"
	}
	return s
}

// llmOrderPayload llm_temp_budget 类型的 payload 快照（只取 MCP 展示关心的字段）。
type llmOrderPayload struct {
	BaseBudgetId   string `json:"baseBudgetId"`
	BaseBudgetName string `json:"baseBudgetName"`
	Duration       string `json:"duration"`
	Reason         string `json:"reason"`
}

// llmPayloadOf 解析 llm 类型工单的 payload；非 llm 类型/解析失败返回零值。
func llmPayloadOf(o *entity.ReviewOrder) (p llmOrderPayload) {
	if o.OrderType == "llm_temp_budget" {
		_ = json.Unmarshal([]byte(o.Payload), &p)
	}
	return
}

// reviewOrderOut 工单行 → 规整 map（状态/周期转中文，金额格式化；llm 类型附基础预算/理由）
func reviewOrderOut(o *entity.ReviewOrder) map[string]any {
	m := map[string]any{
		"orderId": o.OrderId, "orderType": o.OrderType, "title": o.Title,
		"status": budgetReqStatusName(o.Status), "applicant": o.ApplicantName,
		"amount": fmt.Sprintf("$%.2f", o.Amount.Float()),
		"reviewer": o.ReviewerName, "reviewNote": o.ReviewNote,
		"resultRef": o.ResultRef, // llm 类型=审批生成的临时预算 budgetId
		"createdAt": fmtTime(o.CreatedAt, "2006-01-02 15:04"),
		"reviewedAt": fmtTime(o.ReviewedAt, "2006-01-02 15:04"),
	}
	if p := llmPayloadOf(o); p != (llmOrderPayload{}) {
		m["baseBudget"] = p.BaseBudgetName
		m["duration"] = budgetDurationName(p.Duration)
		m["reason"] = p.Reason
	}
	return m
}

// ===== 读 server（platform-budget，/mcp/budget） =====

func buildBudgetServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-budget", Version: "1.0"}, nil)

	opsTool(srv, "budget_my", "查我的预算（真实数据）：当前调用方命中的全部预算规则，hit=true 的一条（生效上限最大）为实际使用额度，已用/剩余按「预算上限 − 本人周期内消费」展示层计算；用户问「我还剩多少额度/预算」用它",
		[]toolArg{{"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := budgetSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			res, err := d.LLMCfg.MyBudgets(ctx, *t)
			if err != nil {
				return "", err
			}
			list := res.List
			if len(list) == 0 {
				return "暂无覆盖你的预算规则（不受预算限制）", nil
			}
			items := make([]map[string]any, 0, len(list))
			for _, b := range list {
				m := map[string]any{
					"budgetId": b.BudgetId, "budgetName": b.BudgetName,
					"hit":      b.BudgetId == res.HitBudgetId,
					"duration": budgetDurationName(b.BudgetDuration),
					"spend":    fmt.Sprintf("$%.2f", b.Spend.Float()),
					"isLow":    b.IsLow, "isExhausted": b.IsExhausted,
				}
				if b.MaxBudget != nil {
					m["maxBudget"] = fmt.Sprintf("$%.2f", b.MaxBudget.Float())
					m["remaining"] = fmt.Sprintf("$%.2f", b.Remaining.Float())
					m["usedPercent"] = fmt.Sprintf("%.1f%%", b.UsedPercent)
				} else {
					m["maxBudget"] = "不限"
					m["remaining"] = "不限"
				}
				if b.BudgetResetAt != nil {
					m["resetAt"] = fmtTime(b.BudgetResetAt, "2006-01-02 15:04")
				}
				items = append(items, m)
			}
			return jsonOutAny(items)
		})

	opsTool(srv, "budget_request_list", "查临时预算申请（真实数据，统一审核工单）：我提交的申请 + 待我审批的申请（部门负责人或管理员）；审批人处理申请前先用它拿 order_id",
		[]toolArg{{"status", "状态过滤：pending|approved|rejected（缺省：mine=全部、review=pending）", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 20", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := budgetSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			status := argStr(a, "status", "")
			pn := clampInt(argInt(a, "page", 1), 1, 10000)
			ps := clampInt(argInt(a, "limit", 20), 1, 50)
			mine, _, err := d.BudgetReq.ListMine(ctx, *t, status, pn, ps)
			if err != nil {
				return "", err
			}
			review, _, err := d.BudgetReq.ListReview(ctx, *t, status, pn, ps)
			if err != nil {
				return "", err
			}
			mineItems := make([]map[string]any, 0, len(mine))
			for _, r := range mine {
				mineItems = append(mineItems, reviewOrderOut(r))
			}
			reviewItems := make([]map[string]any, 0, len(review))
			for _, r := range review {
				reviewItems = append(reviewItems, reviewOrderOut(r))
			}
			return jsonOutAny(map[string]any{"mine": mineItems, "review": reviewItems})
		})

	return srv
}

// ===== 写 server（platform-budget-write，/mcp/budget-write） =====

func buildBudgetWriteServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-budget-write", Version: "1.0"}, nil)

	opsTool(srv, "budget_request_create", "申请临时预算（写真实库，走统一审核工单）：个人功能——为自己申请临时额度，缺省自动以命中的最大预算为基础，提交后通知部门负责人审批（申请人持最高职级岗位时免审直接通过并生成临时预算）；amount 单位美元",
		[]toolArg{{"base_budget_id", "基础预算 budgetId（可选；缺省自动取命中本人且上限最大的预算）", false}, {"amount", "临时额度（美元，>0）", true}, {"reason", "申请理由（必填，≤500 字）", true}, {"duration", "周期：1d|7d|30d，缺省 30d", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := budgetSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			params := map[string]any{
				"amount":   argFloat(a, "amount", 0),
				"reason":   argStr(a, "reason", ""),
				"duration": argStr(a, "duration", "30d"),
			}
			if bb := argStr(a, "base_budget_id", ""); bb != "" {
				params["baseBudgetId"] = bb
			}
			e, err := d.BudgetReq.Create(ctx, *t, "llm_temp_budget", params)
			if err != nil {
				return "", err
			}
			p := llmPayloadOf(e)
			if e.Status == "approved" {
				return fmt.Sprintf("临时预算申请 %s（$%.2f，%s）已按最高职级自动通过，临时预算 %s 已生成并生效",
					e.OrderId, e.Amount.Float(), budgetDurationName(p.Duration), e.ResultRef), nil
			}
			return fmt.Sprintf("已提交临时预算申请 %s（$%.2f，%s），待部门负责人审批（进度见 审核系统·我的提交）",
				e.OrderId, e.Amount.Float(), budgetDurationName(p.Duration)), nil
		})

	opsTool(srv, "budget_request_approve", "审批通过临时预算（写真实库）：生成申请人专属临时预算并叠加在固定预算之上，随后通知申请人；仅部门负责人/管理员可用，order_id 来自 budget_request_list",
		[]toolArg{{"order_id", "工单 id（budget_request_list 查得；兼容旧参数 request_id）", true}, {"note", "审批意见（可选）", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := budgetSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			e, err := d.BudgetReq.Approve(ctx, *t, reviewOrderId(a), argStr(a, "note", ""))
			if err != nil {
				return "", mapBudgetReqErr(err)
			}
			p := llmPayloadOf(e)
			return fmt.Sprintf("已通过申请 %s：生成临时预算 %s（$%.2f，%s），已通知申请人",
				e.OrderId, e.ResultRef, e.Amount.Float(), budgetDurationName(p.Duration)), nil
		})

	opsTool(srv, "budget_request_reject", "驳回临时预算申请（写真实库）：驳回后通知申请人；仅部门负责人/管理员可用，order_id 来自 budget_request_list",
		[]toolArg{{"order_id", "工单 id（budget_request_list 查得；兼容旧参数 request_id）", true}, {"note", "驳回理由（建议填写）", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := budgetSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			e, err := d.BudgetReq.Reject(ctx, *t, reviewOrderId(a), argStr(a, "note", ""))
			if err != nil {
				return "", mapBudgetReqErr(err)
			}
			return fmt.Sprintf("已驳回申请 %s，已通知申请人", e.OrderId), nil
		})

	return srv
}

// reviewOrderId 取工单 id：新参数 order_id，兼容旧 request_id（两者取一，order_id 优先）。
func reviewOrderId(a map[string]any) string {
	if id := argStr(a, "order_id", ""); id != "" {
		return id
	}
	return argStr(a, "request_id", "")
}

// mapBudgetReqErr 业务哨兵转友好文案（并发双批时第二个调用者看到已被处理）
func mapBudgetReqErr(err error) error {
	if errors.Is(err, persistence.ErrOrderClosed) {
		return errors.New("该申请已被处理（通过或驳回），请刷新列表")
	}
	return err
}
