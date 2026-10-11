package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq" // 注册 database/sql 的 "postgres" 驱动，供迁移专用连接使用

	"aiarsenal/config"
)

// migrations 嵌入各库的迁移 SQL：migrations/<db>/NNNNNN_<name>.{up,down}.sql。
// 某库目录为空（或无 .sql）则运行时静默跳过；base SQL（resource/data/*.sql）仍是建表真相源，
// 迁移只负责给存量库追平 base SQL 中的增量变更（须与 base SQL 保持一致且幂等）。
//
//go:embed migrations
var migrationsFS embed.FS

// dbs 迁移目标库（与 db.go 的 Init*DB 一一对应，顺序无关——各库独立版本）。
var dbs = []string{"aiarsenal", "aa_skill", "aa_llm", "aa_mcp", "aa_agent", "aa_client", "aa_project", "aa_doc", "aa_kb", "aa_schedule"}

// RunMigrations 在 api 启动时对每个库跑 golang-migrate 的 Up()：
//   - 某库在 migrations/<db>/ 下无 .sql 迁移文件 → 静默跳过；
//   - 已是最新 → ErrNoChange（忽略）；
//   - 有新迁移 → 每文件单事务执行（postgres DDL 可事务，失败整体回滚）。
//
// 迁移以非 owner 角色 aiarsenal 执行（DSN 来自 database.default.link），故启用迁移前须
// 把库表所有权收口给 aiarsenal（ALTER TABLE 须所有者），否则报 "must be owner of table"。
func RunMigrations() error {
	for _, name := range dbs {
		if err := migrateDB(name); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
	}
	return nil
}

func migrateDB(name string) error {
	// 该库无迁移目录、或目录内无 .sql 迁移文件 → 静默跳过（不建连接、不打日志）。
	entries, err := migrationsFS.ReadDir("migrations/" + name)
	if err != nil {
		return nil
	}
	hasSQL := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			hasSQL = true
			break
		}
	}
	if !hasSQL {
		return nil
	}

	// 用专用 lib/pq 连接跑迁移（DSN 经 MigrateDSNFor 去掉 TimeZone，lib/pq 不认该关键字；
	// 迁移只跑 DDL、不读写 timestamp，时区无影响）。不复用 gorm 连接池，跑完即关。
	dsn := config.Get().Database.Default.MigrateDSNFor(name)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer sqlDB.Close()

	dbInst, err := postgres.WithInstance(sqlDB, &postgres.Config{MigrationsTable: "schema_migrations"})
	if err != nil {
		return fmt.Errorf("db instance: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations/"+name)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, name, dbInst)
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("up: %w", err)
	}

	switch v, _, verr := m.Version(); {
	case errors.Is(verr, migrate.ErrNilVersion):
		log.Printf("[migrate] %s: 无版本（初始状态）", name)
	case verr != nil:
		log.Printf("[migrate] %s: 完成（读版本失败: %v）", name, verr)
	default:
		log.Printf("[migrate] %s → v%d", name, v)
	}
	return nil
}
