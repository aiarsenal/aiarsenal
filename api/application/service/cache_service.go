package service

import (
	"context"

	"aiarsenal/consts"
	"aiarsenal/infrastructure/cache"
	"aiarsenal/infrastructure/db"
)

type CacheService struct{ cache *cache.Cache }

func NewCacheService(c *cache.Cache) *CacheService { return &CacheService{cache: c} }

// Remove 清除全部业务缓存（按 tag + 按 prefix 扫描 redis）
func (s *CacheService) Remove(ctx context.Context) error {
	s.cache.RemoveByTag(ctx, "aiArsenalV3Cache:sysAuthTag")
	s.cache.RemoveByTag(ctx, "aiArsenalV3Cache:sysRole")
	s.cache.RemoveByTag(ctx, "aiArsenalV3Cache:sysAuthMenu")

	// 扫描删除所有带前缀的 key
	rdb := db.Redis()
	if rdb == nil {
		return nil
	}
	prefix := consts.CachePrefix
	var cursor uint64
	for {
		keys, c, err := rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			_ = rdb.Del(ctx, keys...).Err()
		}
		cursor = c
		if cursor == 0 {
			break
		}
	}
	return nil
}
