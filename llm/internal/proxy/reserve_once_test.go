package proxy

import (
	"context"
	"os"
	"sync"
	"testing"

	"aiarsenal/llm/internal/config"
	"aiarsenal/llm/internal/store"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestSpendReservationRefundOnce 预留退还的 once 语义行为级验证（F-024 审计变异 B
// 的杀手）：bill 与入口 defer settleAll 竞争调用 refundKey，无论时序如何只退一次。
// LLM_TEST_DSN 缺省跳过（真库集成，与 subscriber_test 同约定）。
func TestSpendReservationRefundOnce(t *testing.T) {
	dsn := os.Getenv("LLM_TEST_DSN")
	if dsn == "" || testing.Short() {
		t.Skip("LLM_TEST_DSN 未设置或 short 模式：跳过真库集成")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("DSN 不可达，跳过: %v", err)
	}
	st := store.New(gdb, &config.Config{SecretKey: "test"})
	ctx := context.Background()
	hash := "aa-test-refund-once-hash"
	gdb.Exec(`DELETE FROM aa_llm_virtual_key WHERE token_hash = ?`, hash)
	defer gdb.Exec(`DELETE FROM aa_llm_virtual_key WHERE token_hash = ?`, hash)
	// 预置既有余额 1000：GREATEST(0,…) 下限会掩盖「重复退款」（退到 0 为止看不出
	// 多退了几次）——有既有余额时重复退款才会跌破应有时值，once 缺陷可见。
	if err := gdb.Exec(`INSERT INTO aa_llm_virtual_key (token_hash, key_alias, spend, max_budget)
		VALUES (?, 't', 1000, 5000)`, hash).Error; err != nil {
		t.Fatal(err)
	}

	ok, err := st.ReserveKeySpend(ctx, hash, 400)
	if err != nil || !ok {
		t.Fatalf("预留失败: ok=%v err=%v", ok, err)
	}
	rs := &spendReservation{st: st, keyHash: hash, keyEst: 400}

	// bill 与 defer settleAll 竞争：8 个并发 refund 调用，once 语义保证只退一次
	// （spend 应从 1400 精确回到 1000；once 缺失则多退至 0 或更低）
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rs.refundKey(ctx)
		}()
	}
	wg.Wait()

	var spend int64
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if spend != 1000 {
		t.Fatalf("once 语义破坏：8 次竞争 refund 后 spend=%d（应=1000 精确退还一次）", spend)
	}
	rs.refundKey(ctx)
	gdb.Raw(`SELECT spend FROM aa_llm_virtual_key WHERE token_hash = ?`, hash).Scan(&spend)
	if spend != 1000 {
		t.Fatalf("once 后重复 refund 改变了状态: %d", spend)
	}
}
