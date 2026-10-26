package auth

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/config"
	"aiarsenal/infrastructure/db"
)

// TestLockoutRedisInteraction 锁定模块与 redis 的真实交互（INCR/TTL/阈值满额重置/
// 成功清零）。依赖本机 redis：testing.Short() 或不可达时跳过（仓库自检清单约定的
// 外部依赖测试声明方式）；键用 prverify- 前缀，用例自清理。
func TestLockoutRedisInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过外部依赖测试")
	}
	if _, err := db.InitRedis(); err != nil {
		t.Skipf("redis 不可达，跳过：%v", err)
	}
	cl := db.Redis()
	const user = "prverify-unittest-lockout"
	key := "pwd:fail:" + user
	t.Cleanup(func() { cl.Del(context.Background(), key) })
	cl.Del(context.Background(), key) // 起点干净

	max := config.Get().Password.Lockout.MaxFailures

	// 未失败时未锁定
	if LoginLocked(user) {
		t.Fatal("零失败不应判定锁定")
	}
	// max-1 次：计数累计但未锁
	for i := 1; i < max; i++ {
		RecordLoginFailure(user)
		if n, _ := cl.Get(context.Background(), key).Result(); n != strconv.Itoa(i) {
			t.Fatalf("第 %d 次失败后计数=%q，期望 %d", i, n, i)
		}
		if LoginLocked(user) {
			t.Fatalf("第 %d 次失败不应锁定", i)
		}
	}
	// 达到阈值：锁定成立，且 TTL 满额重置（锁 N 分钟的承诺从这里起算）
	RecordLoginFailure(user)
	if !LoginLocked(user) {
		t.Fatal("达到阈值后应判定锁定")
	}
	ttl, err := cl.TTL(context.Background(), key).Result()
	if err != nil || ttl <= 0 {
		t.Fatalf("阈值处 TTL 异常: %v %v", ttl, err)
	}
	if ttl < lockTTL()-time.Minute || ttl > lockTTL() {
		t.Fatalf("阈值处 TTL 应满额重置为 %v，实际 %v", lockTTL(), ttl)
	}
	// 锁定期内阈值后的失败不再拉长窗口（n>max 不进任何续期分支）
	RecordLoginFailure(user)
	ttl2, _ := cl.TTL(context.Background(), key).Result()
	if ttl2 > ttl {
		t.Fatalf("阈值后失败不应续期窗口：前 %v 后 %v", ttl, ttl2)
	}
	// 成功清零
	ClearLoginFailures(user)
	if LoginLocked(user) {
		t.Fatal("清零后不应判定锁定")
	}
	if _, err := cl.Get(context.Background(), key).Result(); err != redis.Nil {
		t.Fatalf("清零后键应不存在，err=%v", err)
	}
}
