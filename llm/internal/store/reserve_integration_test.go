package store

import (
	"context"
	"os"
	"sync"
	"testing"

	"aiarsenal/llm/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// F-024 预留/退款语义的集成测试（变异 A/B 的杀手）：预算原子闸门与退款幂等是
// 本修复的安全核心，纯函数表测覆盖不到（需真库行级原子性）。
// 运行方式与 subscriber_test 同约定：LLM_TEST_DSN 缺省跳过；本地
//
//	LLM_TEST_DSN='host=127.0.0.1 port=5434 user=aiarsenal password=aiarsenal123
//	dbname=aa_llm sslmode=disable' go test ./internal/store/ -run TestReserve
func TestReserveRefundSemantics(t *testing.T) {
	dsn := os.Getenv("LLM_TEST_DSN")
	if dsn == "" || testing.Short() {
		t.Skip("LLM_TEST_DSN 未设置或 short 模式：跳过真库集成")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("DSN 不可达，跳过: %v", err)
	}
	st := New(gdb, &config.Config{SecretKey: "test"})
	ctx := context.Background()
	hash := "aa-test-reserve-hash"
	bid := "aa-test-reserve-budget"

	cleanup := func() {
		gdb.Exec(`DELETE FROM aa_llm_virtual_key WHERE token_hash = ?`, hash)
		gdb.Exec(`DELETE FROM aa_llm_budget WHERE budget_id = ?`, bid)
	}
	cleanup()
	defer cleanup()

	// 种子：预算 500 的 key + 预算 400 的共享预算
	if err := gdb.Exec(`INSERT INTO aa_llm_virtual_key (token_hash, key_alias, spend, max_budget)
		VALUES (?, 't', 0, 500)`, hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`INSERT INTO aa_llm_budget (budget_id, budget_name, spend, max_budget)
		VALUES (?, 't', 0, 400)`, bid).Error; err != nil {
		t.Fatal(err)
	}

	// T1 原子闸门：300 预留成功 → 再 300 超限失败 → 退款后再次成功
	ok, err := st.ReserveKeySpend(ctx, hash, 300)
	if err != nil || !ok {
		t.Fatalf("T1a 首次预留应成功: ok=%v err=%v", ok, err)
	}
	ok, err = st.ReserveKeySpend(ctx, hash, 300)
	if err != nil || ok {
		t.Fatalf("T1b 200+300>500 应失败: ok=%v err=%v", ok, err)
	}
	if err := st.RefundKeySpend(ctx, hash, 300); err != nil {
		t.Fatal(err)
	}
	var spend int64
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if spend != 0 {
		t.Fatalf("T1c 退款后 spend 应=0, 得 %d", spend)
	}

	// T2 退款下限（GREATEST(0,…))：spend=0 时退款不得打负
	if err := st.RefundKeySpend(ctx, hash, 300); err != nil {
		t.Fatal(err)
	}
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if spend < 0 {
		t.Fatalf("T2 退款下限失效：spend=%d 为负（跨窗口缝隙）", spend)
	}

	// T3 重复退款下限：重复/并发退款（bill 与 defer settleAll 竞争的极端形态）
	// 不得把 spend 打负；once-guard 的「只退一次」语义在 proxy 层
	// （spendReservation）由 TestSpendReservationRefundOnce 行为级验证。
	reserved, _ := st.ReserveKeySpend(ctx, hash, 300)
	if !reserved {
		t.Fatal("T3 前置预留失败")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := st.RefundKeySpend(ctx, hash, 300); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if spend != 0 {
		t.Fatalf("T3 并发退款后 spend 应=0（GREATEST 下限）, 得 %d", spend)
	}

	// T4 预算闸门对称性：共享预算同语义 + soft-deleted 行不预留
	ok, _ = st.ReserveBudgetSpend(ctx, bid, 400)
	if !ok {
		t.Fatal("T4a 共享预算首次预留应成功")
	}
	ok, _ = st.ReserveBudgetSpend(ctx, bid, 400)
	if ok {
		t.Fatal("T4b 400+400>400 应失败")
	}
	st.RefundBudgetSpend(ctx, bid, 400)
	gdb.Exec(`UPDATE aa_llm_budget SET deleted_at = now() WHERE budget_id = ?`, bid)
	ok, _ = st.ReserveBudgetSpend(ctx, bid, 100)
	if ok {
		t.Fatal("T4c 软删预算行不应预留")
	}

	// T5 并发预留的原子性：预算 500，8 并发各预留 100 → 恰 5 成功 3 失败（无丢失更新）
	gdb.Exec(`UPDATE aa_llm_virtual_key SET spend = 0 WHERE token_hash = ?`, hash)
	results := make([]bool, 8)
	var wg2 sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			ok, _ := st.ReserveKeySpend(ctx, hash, 100)
			results[i] = ok
		}(i)
	}
	wg2.Wait()
	wins := 0
	for _, ok := range results {
		if ok {
			wins++
		}
	}
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if wins != 5 || spend != 500 {
		t.Fatalf("T5 并发预留应恰 5 成功（wins=%d spend=%d）——原子性破坏", wins, spend)
	}
}
