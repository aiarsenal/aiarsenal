package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// fakeBudgetRepo 只实现 resolveBase/Execute 用到的读方法；嵌入接口，未用方法被调即 panic 暴露。
type fakeBudgetRepo struct {
	repository.ILLMBudgetRepository
	budgets []*entity.LLMBudget
	added   []*entity.LLMBudget
}

func (f *fakeBudgetRepo) Get(_ context.Context, id string) (*entity.LLMBudget, error) {
	for _, b := range f.budgets {
		if b.BudgetId == id {
			return b, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeBudgetRepo) ListGrantBudgets(_ context.Context) ([]*entity.LLMBudget, error) {
	return f.budgets, nil
}

func (f *fakeBudgetRepo) Add(_ context.Context, b *entity.LLMBudget) (string, error) {
	b.BudgetId = "granted-new"
	f.added = append(f.added, b)
	return b.BudgetId, nil
}

// fakeDailyRepo 只实现 budgetWindowSpend 用到的 SumScopeDaily（windowSpend 毫值可注）。
type fakeDailyRepo struct {
	repository.ILLMDailySpendRepository
	windowSpend types.Milli
}

func (f *fakeDailyRepo) SumScopeDaily(_ context.Context, _, _ string, _ []string) ([]*entity.LLMSpendPoint, error) {
	return []*entity.LLMSpendPoint{{Spend: f.windowSpend}}, nil
}

// fakePostRepo 只实现 ShouldAutoApprove 用到的 HoldsTopPost（top 可注：申请人是否持最高职级）。
type fakePostRepo struct {
	repository.IPostRepository
	top bool
}

func (f *fakePostRepo) HoldsTopPost(_ context.Context, _ string) bool { return f.top }

func mp(v int64) *types.Milli { m := types.Milli(v); return &m } // 金额指针（毫）

// 命中本人的圈人规则（userIds 精确圈）。
const reqHitRules = `[{"userIds":["u1"],"roleIds":[],"postIds":[],"deptId":"","exclude":false}]`

var reqTestIdentity = model.Identity{UserId: "u1", RoleIds: []string{"r1"}}

func llmHandler(budgets []*entity.LLMBudget, windowSpend types.Milli) *LLMTempBudgetHandler {
	return NewLLMTempBudgetHandler(&fakeBudgetRepo{budgets: budgets}, &fakeDailyRepo{windowSpend: windowSpend}, &fakePostRepo{})
}

func TestResolveBaseAutoPicksLargestMatched(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "b1", BudgetName: "small", MaxBudget: mp(200_0000), GrantRules: reqHitRules},
		{BudgetId: "b2", BudgetName: "big", MaxBudget: mp(400_0000), GrantRules: reqHitRules},
		// 圈别人：上限再大也不参与
		{BudgetId: "bx", BudgetName: "other", MaxBudget: mp(99_990_000),
			GrantRules: `[{"userIds":["someone"]}]`},
	}, 1)
	got, err := h.resolveBase(context.Background(), reqTestIdentity, "")
	if err != nil {
		t.Fatalf("auto pick: %v", err)
	}
	if got.BudgetId != "b2" {
		t.Fatalf("auto pick = %s(%s), want b2", got.BudgetId, got.BudgetName)
	}
}

func TestResolveBaseUnlimitedBeatsFinite(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "b2", BudgetName: "big", MaxBudget: mp(400_0000), GrantRules: reqHitRules},
		{BudgetId: "inf", BudgetName: "unlimited", GrantRules: reqHitRules},
	}, 1)
	got, err := h.resolveBase(context.Background(), reqTestIdentity, "")
	if err != nil || got.BudgetId != "inf" {
		t.Fatalf("unlimited should win, got %s err=%v", got.BudgetId, err)
	}
}

func TestResolveBaseTiePrefersLargerSpend(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "a", BudgetName: "a", MaxBudget: mp(400_0000), Spend: types.Milli(10_000), GrantRules: reqHitRules},
		{BudgetId: "c", BudgetName: "c", MaxBudget: mp(400_0000), Spend: types.Milli(50_000), GrantRules: reqHitRules},
	}, 1)
	got, err := h.resolveBase(context.Background(), reqTestIdentity, "")
	if err != nil || got.BudgetId != "c" {
		t.Fatalf("tie should pick larger spend, got %s err=%v", got.BudgetId, err)
	}
}

func TestResolveBaseExplicitIdValidated(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "b2", BudgetName: "big", MaxBudget: mp(400_0000), GrantRules: reqHitRules},
		{BudgetId: "bx", BudgetName: "other", MaxBudget: mp(99_990_000),
			GrantRules: `[{"userIds":["someone"]}]`},
	}, 1)
	if _, err := h.resolveBase(context.Background(), reqTestIdentity, "bx"); err == nil ||
		!strings.Contains(err.Error(), "未命中该预算") {
		t.Fatalf("explicit non-matched base should fail, got err=%v", err)
	}
	got, err := h.resolveBase(context.Background(), reqTestIdentity, "b2")
	if err != nil || got.BudgetId != "b2" {
		t.Fatalf("explicit matched base, got %s err=%v", got.BudgetId, err)
	}
}

func TestResolveBaseNoMatchAtAll(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "bx", BudgetName: "other", MaxBudget: mp(99_990_000),
			GrantRules: `[{"userIds":["someone"]}]`},
	}, 1)
	_, err := h.resolveBase(context.Background(), reqTestIdentity, "")
	if err == nil || !strings.Contains(err.Error(), "未命中任何预算") {
		t.Fatalf("no matched budget should fail, got err=%v", err)
	}
}

// Prepare 门槛：命中预算但本周期无消耗 → 拒绝（与 /mine 按钮同一口径）。
func TestPrepareRejectsNoWindowSpend(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "b2", BudgetName: "big", MaxBudget: mp(400_0000), GrantRules: reqHitRules},
	}, 0)
	_, _, _, err := h.Prepare(context.Background(), reqTestIdentity,
		map[string]any{"amount": 100.0, "reason": "项目冲刺", "duration": "30d"})
	if err == nil || !strings.Contains(err.Error(), "还没有消费") {
		t.Fatalf("no window spend should fail, got err=%v", err)
	}
}

// Prepare 正常路径：载荷/标题/金额（毫）齐备。
func TestPrepareBuildsOrder(t *testing.T) {
	h := llmHandler([]*entity.LLMBudget{
		{BudgetId: "b2", BudgetName: "big", MaxBudget: mp(400_0000), GrantRules: reqHitRules},
	}, types.Milli(50))
	title, amount, payload, err := h.Prepare(context.Background(), reqTestIdentity,
		map[string]any{"amount": 100.0, "reason": "项目冲刺", "duration": "7d"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if amount != 100_0000 { // $100 → 100 万毫
		t.Fatalf("amount = %d, want 1000000", int64(amount))
	}
	if title != "临时预算申请·基于「big」" {
		t.Fatalf("title = %s", title)
	}
	if payload["baseBudgetId"] != "b2" || payload["duration"] != "7d" || payload["reason"] != "项目冲刺" {
		t.Fatalf("payload = %v", payload)
	}
}

// Execute：建申请人专属临时预算（grant_rules 只圈申请人、上限=申请额、窗口懒初始化）。
func TestExecuteCreatesTempBudget(t *testing.T) {
	repo := &fakeBudgetRepo{budgets: []*entity.LLMBudget{
		{BudgetId: "b2", BudgetName: "big", GrantRules: reqHitRules},
	}}
	h := NewLLMTempBudgetHandler(repo, &fakeDailyRepo{}, &fakePostRepo{})
	o := &entity.ReviewOrder{
		OrderType: "llm_temp_budget", ApplicantId: "u1", ApplicantName: "张三",
		Amount: 100_0000,
		Payload: `{"baseBudgetId":"b2","baseBudgetName":"big","duration":"7d","reason":"冲刺"}`,
	}
	ref, err := h.Execute(context.Background(), model.Identity{UserId: "leader"}, o)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ref != "granted-new" || len(repo.added) != 1 {
		t.Fatalf("ref=%s added=%d", ref, len(repo.added))
	}
	b := repo.added[0]
	if b.MaxBudget == nil || *b.MaxBudget != 100_0000 || b.Spend != 0 ||
		b.BudgetDuration != "7d" || b.BudgetResetAt != nil {
		t.Fatalf("budget = %+v", b)
	}
	if !strings.Contains(b.GrantRules, `"u1"`) || strings.Contains(b.GrantRules, "leader") {
		t.Fatalf("grantRules = %s", b.GrantRules)
	}
	if !strings.HasPrefix(b.BudgetName, "临时预算·big·张三") {
		t.Fatalf("budgetName = %s", b.BudgetName)
	}
	if b.CreatedAt != nil && time.Time(*b.CreatedAt).IsZero() {
		t.Fatal("createdAt zero")
	}
}

// ShouldAutoApprove：最高职级（HoldsTopPost）免审判定直通 postRepo；nil 仓储判 false（宁可多审）。
func TestShouldAutoApprove(t *testing.T) {
	ctx := context.Background()
	if !(NewLLMTempBudgetHandler(&fakeBudgetRepo{}, &fakeDailyRepo{}, &fakePostRepo{top: true})).
		ShouldAutoApprove(ctx, reqTestIdentity) {
		t.Fatal("top post holder should auto-approve")
	}
	if (NewLLMTempBudgetHandler(&fakeBudgetRepo{}, &fakeDailyRepo{}, &fakePostRepo{})).
		ShouldAutoApprove(ctx, reqTestIdentity) {
		t.Fatal("non-top user should not auto-approve")
	}
	if (NewLLMTempBudgetHandler(&fakeBudgetRepo{}, &fakeDailyRepo{}, nil)).
		ShouldAutoApprove(ctx, reqTestIdentity) {
		t.Fatal("nil postRepo should default to manual review")
	}
}
