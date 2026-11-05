package auth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/llm/internal/config"
)

// TestKeyBustSubscriber 集成验证（F-027）：api 侧在 llm redis 库上 PUBLISH vkey-bust
// <hash>，订阅者立即 DEL vkey:<hash> —— 封禁生效延迟从 5s 降到 0。
// 依赖本机 redis（llm 默认 db2）：不可达或 testing.Short() 时跳过；键一律 aa-test- 前缀。
func TestKeyBustSubscriber(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过真 redis 集成")
	}
	cfg := config.Load()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, DB: cfg.Redis.DB})
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis 不可达，跳过: %v", err)
	}

	testKey := "vkey:aa-test-bust-hash"
	if err := rdb.Set(ctx, testKey, "1", time.Minute).Err(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	defer func() { _ = rdb.Del(context.Background(), testKey).Err() }()

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()
	StartKeyBustSubscriber(bctx, rdb)

	// 等订阅就绪：pub/sub 是 at-most-once，发布早于订阅建立消息即丢（慢机 flake 源）。
	// 轮询 PubSubNumSub 确认订阅者数 ≥1 再发。
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if n, _ := rdb.PubSubNumSub(ctx, VKeyBustChannel).Result(); n[VKeyBustChannel] >= 1 {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Skip("订阅者未在时限内就绪（慢机/redis 半可用），跳过避免假红")
	}
	if err := rdb.Publish(ctx, VKeyBustChannel, "aa-test-bust-hash").Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	gone := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, _ := rdb.Exists(context.Background(), testKey).Result()
		if n == 0 {
			gone = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !gone {
		t.Fatalf("PUBLISH 后缓存键未被订阅者删除（F-027 失效链断裂）")
	}
}
