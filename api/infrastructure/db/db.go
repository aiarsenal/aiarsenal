package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"aiarsenal/config"
)

var (
	gormDB    *gorm.DB
	skillDB   *gorm.DB
	llmDB     *gorm.DB
	mcpDB     *gorm.DB
	agentDB   *gorm.DB
	clientDB  *gorm.DB
	projectDB *gorm.DB
	docDB     *gorm.DB
	kbDB      *gorm.DB
	scheduleDB *gorm.DB
	redisCl   *redis.Client
)

// InitDB 初始化 GORM（postgres），DSN 由 database.default.link 解析而来
func InitDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSN()
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	lifetime := 30 * time.Second
	sqlDB.SetConnMaxLifetime(lifetime)
	gormDB = g
	return g, nil
}

// DB 已初始化的 GORM 实例（aiarsenal 库）
func DB() *gorm.DB {
	if gormDB == nil {
		if _, err := InitDB(); err != nil {
			panic(err)
		}
	}
	return gormDB
}

// InitSkillDB 初始化 aa_skill 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
func InitSkillDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_skill")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_skill: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	skillDB = g
	return g, nil
}

// SkillDB 已初始化的 aa_skill 库 GORM 实例
func SkillDB() *gorm.DB {
	if skillDB == nil {
		if _, err := InitSkillDB(); err != nil {
			panic(err)
		}
	}
	return skillDB
}

// InitLLMDB 初始化 aa_llm 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
func InitLLMDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_llm")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_llm: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	llmDB = g
	return g, nil
}

// LLMDB 已初始化的 aa_llm 库 GORM 实例
func LLMDB() *gorm.DB {
	if llmDB == nil {
		if _, err := InitLLMDB(); err != nil {
			panic(err)
		}
	}
	return llmDB
}

// InitMcpDB 初始化 aa_mcp 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
func InitMcpDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_mcp")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_mcp: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	mcpDB = g
	return g, nil
}

// McpDB 已初始化的 aa_mcp 库 GORM 实例
func McpDB() *gorm.DB {
	if mcpDB == nil {
		if _, err := InitMcpDB(); err != nil {
			panic(err)
		}
	}
	return mcpDB
}

// InitAgentDB 初始化 aa_agent 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_agent 库存 Agent 工作空间数据（配置中心/Skill/安全扫描/AI Gateway 治理记录等）
func InitAgentDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_agent")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_agent: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	agentDB = g
	return g, nil
}

// AgentDB 已初始化的 aa_agent 库 GORM 实例
func AgentDB() *gorm.DB {
	if agentDB == nil {
		if _, err := InitAgentDB(); err != nil {
			panic(err)
		}
	}
	return agentDB
}

// InitClientDB 初始化 aa_client 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_client 库存桌面客户端（Win/Mac App）数据：三客户端安装/配置规格 + 运行情况上报
func InitClientDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_client")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_client: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	clientDB = g
	return g, nil
}

// ClientDB 已初始化的 aa_client 库 GORM 实例
func ClientDB() *gorm.DB {
	if clientDB == nil {
		if _, err := InitClientDB(); err != nil {
			panic(err)
		}
	}
	return clientDB
}

// InitProjectDB 初始化 aa_project 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_project 库存项目管理模块数据（项目/成员/任务/里程碑/标签/状态流/自定义字段/资源绑定）
func InitProjectDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_project")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_project: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	projectDB = g
	return g, nil
}

// ProjectDB 已初始化的 aa_project 库 GORM 实例
func ProjectDB() *gorm.DB {
	if projectDB == nil {
		if _, err := InitProjectDB(); err != nil {
			panic(err)
		}
	}
	return projectDB
}

// InitDocDB 初始化 aa_doc 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_doc 库存文档管理模块数据（文档元数据/抽取全文/分块语义向量）
// logger 收 Silent：文档正文是 SQL 绑定参数，错误/慢查询日志会把内容打出去——正文绝不落日志
func InitDocDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_doc")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_doc: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	docDB = g
	return g, nil
}

// DocDB 已初始化的 aa_doc 库 GORM 实例
func DocDB() *gorm.DB {
	if docDB == nil {
		if _, err := InitDocDB(); err != nil {
			panic(err)
		}
	}
	return docDB
}

// InitKbDB 初始化 aa_kb 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_kb 库存知识库管理模块数据（文档元数据/抽取全文/分块语义向量）
// logger 收 Silent：文档正文是 SQL 绑定参数，错误/慢查询日志会把内容打出去——正文绝不落日志
func InitKbDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_kb")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_kb: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	kbDB = g
	return g, nil
}

// KbDB 已初始化的 aa_kb 库 GORM 实例
func KbDB() *gorm.DB {
	if kbDB == nil {
		if _, err := InitKbDB(); err != nil {
			panic(err)
		}
	}
	return kbDB
}

// InitScheduleDB 初始化 aa_schedule 库的 GORM 实例（同 PG 实例、异库；DSN 复用 default 仅换库名）
// aa_schedule 库存日程模块数据（事件 series+例外/参会人/会议室）
func InitScheduleDB() (*gorm.DB, error) {
	c := config.Get()
	dsn := c.Database.Default.GormDSNFor("aa_schedule")
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open aa_schedule: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.Database.Default.MaxIdle)
	sqlDB.SetMaxOpenConns(c.Database.Default.MaxOpen)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	scheduleDB = g
	return g, nil
}

// ScheduleDB 已初始化的 aa_schedule 库 GORM 实例
func ScheduleDB() *gorm.DB {
	if scheduleDB == nil {
		if _, err := InitScheduleDB(); err != nil {
			panic(err)
		}
	}
	return scheduleDB
}

// InitRedis 初始化 Redis 客户端
func InitRedis() (*redis.Client, error) {
	c := config.Get()
	cl := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Default.Address,
		DB:       c.Redis.Default.DB,
		Password: c.Redis.Default.Password,
	})
	if err := cl.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	redisCl = cl
	return cl, nil
}

// Redis 已初始化的 Redis 客户端
func Redis() *redis.Client {
	return redisCl
}
