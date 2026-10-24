package auth

import (
	"context"
	"log/slog"
	"time"

	"aiarsenal/config"
	"aiarsenal/infrastructure/db"
)

// 会话注册表：登录签发的每个 web/桌面令牌在 redis 记一条（SET of jti，TTL=令牌有效期）。
//
// JWT 本身无状态，"退出/吊销"必须有服务端状态才有意义：
//   - 登录 RegisterSession；
//   - 每个请求 SessionValid 校验（身份回查之后）；
//   - 本人登出 RevokeSession，管理员吊销 RevokeAllSessions（离职/封号的即时回收）。
//
// redis 故障按拒绝处理：会话闸门 fail-closed——存储抖动不能变成一次全量放行，
// 大不了是全员重新登录，而不是裸奔。
func sessionKey(userId string) string { return "jwt:sess:" + userId }

func sessionTTL() time.Duration {
	c := config.Get()
	if c.Jwt.TimeOut <= 0 {
		return 30 * 24 * time.Hour
	}
	return time.Duration(c.Jwt.TimeOut) * time.Second
}

// RegisterSession 登录成功后登记会话。jti 为空（实例令牌等非登录通道）不登记。
func RegisterSession(userId, jti string) {
	if userId == "" || jti == "" {
		return
	}
	cl := db.Redis()
	if cl == nil {
		slog.Warn("redis 未初始化，会话未登记", "userId", userId)
		return
	}
	key := sessionKey(userId)
	cl.SAdd(context.Background(), key, jti)
	cl.Expire(context.Background(), key, sessionTTL())
}

// SessionValid 校验 jti 是否仍是有效会话。任何异常（未登记、redis 不可用）一律拒绝。
func SessionValid(userId, jti string) bool {
	if userId == "" || jti == "" {
		return false
	}
	cl := db.Redis()
	if cl == nil {
		slog.Error("会话校验时 redis 不可用，按拒绝处理", "userId", userId)
		return false
	}
	member, err := cl.SIsMember(context.Background(), sessionKey(userId), jti).Result()
	if err != nil {
		slog.Error("会话校验查询失败，按拒绝处理", "err", err, "userId", userId)
		return false
	}
	return member
}

// RevokeSession 吊销单个会话（本人登出）。
func RevokeSession(userId, jti string) {
	if userId == "" || jti == "" {
		return
	}
	if cl := db.Redis(); cl != nil {
		cl.SRem(context.Background(), sessionKey(userId), jti)
	}
}

// RevokeAllSessions 吊销某用户全部会话（管理员操作：离职/封号/强制下线）。
func RevokeAllSessions(userId string) {
	if userId == "" {
		return
	}
	if cl := db.Redis(); cl != nil {
		cl.Del(context.Background(), sessionKey(userId))
	}
}
