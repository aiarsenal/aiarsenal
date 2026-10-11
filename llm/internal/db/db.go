package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"aiarsenal/llm/internal/config"
)

var (
	gormDB  *gorm.DB
	iamDB   *gorm.DB
	redisCl *redis.Client
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	g, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	gormDB = g
	return g, nil
}

func DB() *gorm.DB { return gormDB }

// InitIAMDB 初始化 aiarsenal 库连接（只读用途：预算适用范围判定的身份解析）。
// 失败不阻断网关启动：预算圈人判定届时 fail-open 放行（与共享预算缺失放行同语义）。
func InitIAMDB(cfg *config.Config) (*gorm.DB, error) {
	g, err := gorm.Open(postgres.Open(cfg.IAMDB.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open iam: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.IAMDB.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.IAMDB.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	iamDB = g
	return g, nil
}

// IAMDB aiarsenal 库连接；未初始化返回 nil（调用方判空走 fail-open）。
func IAMDB() *gorm.DB { return iamDB }

func InitRedis(cfg *config.Config) (*redis.Client, error) {
	cl := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr, DB: cfg.Redis.DB, Password: cfg.Redis.Password,
	})
	if err := cl.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	redisCl = cl
	return cl, nil
}

func Redis() *redis.Client { return redisCl }
