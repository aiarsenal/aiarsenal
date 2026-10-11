package cache

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/infrastructure/db"
)

// Cache 基于 Redis 的简易缓存，支持按 tag 批量失效（与旧 libcache 行为对齐）
type Cache struct {
	rdb redis.UniversalClient
}

func New() *Cache {
	return &Cache{rdb: db.Redis()}
}

func tagKey(tag string) string { return "tag:" + tag }

// Get 读取字符串缓存
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	if c.rdb == nil {
		return "", nil
	}
	return c.rdb.Get(ctx, key).Result()
}

// Set 写入缓存，可附带 tag（失效时按 tag 批量删除）
func (c *Cache) Set(ctx context.Context, key, val string, ttl time.Duration, tags ...string) error {
	if c.rdb == nil {
		return nil
	}
	if err := c.rdb.Set(ctx, key, val, ttl).Err(); err != nil {
		return err
	}
	for _, t := range tags {
		if t != "" {
			_ = c.rdb.SAdd(ctx, tagKey(t), key).Err()
		}
	}
	return nil
}

// RemoveByTag 按 tag 批量删除缓存
func (c *Cache) RemoveByTag(ctx context.Context, tag string) {
	if c.rdb == nil {
		return
	}
	tk := tagKey(tag)
	keys, _ := c.rdb.SMembers(ctx, tk).Result()
	if len(keys) > 0 {
		_ = c.rdb.Del(ctx, keys...).Err()
	}
	_ = c.rdb.Del(ctx, tk).Err()
}

// Del 删除单个 key
func (c *Cache) Del(ctx context.Context, keys ...string) {
	if c.rdb == nil {
		return
	}
	_ = c.rdb.Del(ctx, keys...).Err()
}

// IsEmpty 字符串是否为空
func IsEmpty(s string) bool { return strings.TrimSpace(s) == "" }
