package auth

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/config"
	"aiarsenal/infrastructure/db"
)

// login_lockout.go — 登录失败锁定（等保 GB/T 22239-2019 身份鉴别 b 项：限制非法
// 登录次数）。计数键 pwd:fail:<username>：INCR 累计，达阈值进入锁定；TTL 到期自动
// 解锁；登录成功 DEL 清零。
//
// 与 session.go 的关键差异：**本模块 fail-open**——redis 不可用或读写出错一律放行。
// 原因：会话闸门管的是鉴权本体，失效等于裸奔，必须 fail-closed（见 session.go）；
// 锁定只是防爆破加固层，失效时口令校验仍在，登录可用性优先（需求拍板：redis 故障
// 保登录可用）。两处注释互指，改取舍时两边一起想。
//
// 已知权衡：按用户名锁定可被恶意利用做账号 DoS（知道用户名即可连错 N 次锁 T 分钟），
// 而等保 b 项要求的正是账号维度；缓解：TTL 自动解锁、锁窗可配、登录日志留痕可审计。

const loginFailKeyPrefix = "pwd:fail:"

func loginFailKey(username string) string { return loginFailKeyPrefix + username }

// LoginLocked 该账号是否处于锁定期。任何存储异常按未锁定处理（fail-open）。
func LoginLocked(username string) bool {
	if username == "" {
		return false
	}
	cl := db.Redis()
	if cl == nil {
		slog.Warn("redis 未初始化，登录锁定检查放行", "username", username)
		return false
	}
	n, err := cl.Get(context.Background(), loginFailKey(username)).Result()
	if err != nil {
		// 键不存在（redis.Nil）与真实故障同向处理：未锁定（fail-open）。
		// 故障时告警——锁定层失效运维必须可见，否则防爆破只剩口令校验一层。
		if !errors.Is(err, redis.Nil) {
			slog.Warn("登录锁定检查失败，放行（fail-open）", "err", err, "username", username)
		}
		return false
	}
	count, err := strconv.Atoi(n)
	if err != nil {
		return false
	}
	return count >= config.Get().Password.Lockout.MaxFailures
}

// RecordLoginFailure 记一次失败。计数窗口从首次失败起算（首写挂 TTL，后续不续期，
// 防止攻击者用持续失败把计数窗口无限拉长）；达到阈值的**那一次**把 TTL 满额重置——
// 「锁 N 分钟」是用户可见的承诺，不能因计数窗口已消耗而提前失效。非首次失败防御性
// 补挂 TTL——防 INCR 成功而 EXPIRE 未落（或键被意外重建）留下永不过期的计数键，
// 把账号永久锁死。
func RecordLoginFailure(username string) {
	if username == "" {
		return
	}
	cl := db.Redis()
	if cl == nil {
		slog.Warn("redis 未初始化，登录失败未计数", "username", username)
		return
	}
	ctx := context.Background()
	key := loginFailKey(username)
	n, err := cl.Incr(ctx, key).Result()
	if err != nil {
		slog.Warn("登录失败计数失败", "err", err, "username", username)
		return
	}
	switch {
	case n == 1:
		cl.Expire(ctx, key, lockTTL())
	case int(n) == config.Get().Password.Lockout.MaxFailures:
		cl.Expire(ctx, key, lockTTL()) // 达到阈值：锁定时长从现在起满额
	default:
		if ttl, err := cl.TTL(ctx, key).Result(); err != nil || ttl <= 0 {
			cl.Expire(ctx, key, lockTTL())
		}
	}
}

// ClearLoginFailures 登录成功清零计数。
func ClearLoginFailures(username string) {
	if username == "" {
		return
	}
	if cl := db.Redis(); cl != nil {
		cl.Del(context.Background(), loginFailKey(username))
	}
}

func lockTTL() time.Duration {
	return time.Duration(config.Get().Password.Lockout.LockMinutes) * time.Minute
}
