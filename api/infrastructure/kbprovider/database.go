package kbprovider

// database 连接器：关系库查询结果 → 文本条目。
//
// config:
//
//	driver        string postgres|mysql（必填）
//	dsn           string 连接串（必填）
//	query         string 查询 SQL（必填，只读执行方约束在运维侧；本连接器只 Query 不 Exec）
//	title_column  string 标题列（缺省「行 N」）
//	content_column string 正文列（缺省：整行各列「列名: 值」逐行展平）
//	summary_column string 摘要列（可空）
//	key_column    string 外部键列（缺省 row-<n>；upsert 依据）
//	max_rows      int    最多入库行数（默认 5000，上限 50000）
//
// postgres 用 lib/pq（go.mod 直接依赖），mysql 用 go-sql-driver/mysql（go.mod 既有）。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // mysql 驱动（go.mod 既有间接依赖，此处转直接使用）
	_ "github.com/lib/pq"              // postgres 驱动
)

// init 自注册。
func init() { register("database", &DatabasePull{}) }

// DatabasePull 数据库查询连接器。
type DatabasePull struct{}

// Sync 实现 Provider。
func (p *DatabasePull) Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error {
	driver := cfgString(cfg, "driver")
	dsn := cfgString(cfg, "dsn")
	query := strings.TrimSpace(cfgString(cfg, "query"))
	if dsn == "" || query == "" {
		return errors.New("database：缺少 dsn/query 配置")
	}
	switch driver {
	case "postgres", "postgresql":
		driver = "postgres"
	case "mysql":
	default:
		return fmt.Errorf("database：不支持的 driver %q（postgres|mysql）", cfgString(cfg, "driver"))
	}
	titleCol := cfgString(cfg, "title_column")
	contentCol := cfgString(cfg, "content_column")
	summaryCol := cfgString(cfg, "summary_column")
	keyCol := cfgString(cfg, "key_column")
	maxRows := cfgInt(cfg, "max_rows", 5000)
	if maxRows <= 0 {
		maxRows = 5000
	}
	if maxRows > 50000 {
		maxRows = 50000
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("database：打开连接失败：%w", err)
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("database：连接失败：%w", err)
	}

	qCtx, qCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer qCancel()
	rows, err := db.QueryContext(qCtx, query)
	if err != nil {
		return fmt.Errorf("database：查询失败：%w", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("database：取列名失败：%w", err)
	}
	colIdx := map[string]int{}
	for i, c := range cols {
		colIdx[strings.ToLower(c)] = i
	}

	n := 0
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if n >= maxRows {
			run.Note("达到 max_rows=%d 截断", maxRows)
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			run.Inc("skipped", 1)
			run.Note("行扫描失败（第 %d 行）：%v", n+1, err)
			continue
		}
		n++
		run.Inc("rows", 1)
		get := func(col string) string {
			if i, ok := colIdx[strings.ToLower(col)]; ok {
				return vals[i].String
			}
			return ""
		}
		key := "row-" + fmt.Sprintf("%06d", n)
		if keyCol != "" {
			if k := get(keyCol); k != "" {
				key = k
			}
		}
		title := get(titleCol)
		if title == "" {
			title = "行 " + fmt.Sprintf("%06d", n)
		}
		summary := get(summaryCol)
		content := get(contentCol)
		if content == "" { // 无正文列：整行展平为「列名: 值」行
			var lines []string
			for i, c := range cols {
				if vals[i].String != "" {
					lines = append(lines, c+": "+vals[i].String)
				}
			}
			content = strings.Join(lines, "\n")
		}
		if _, err := emit(ingest, run, key, title, summary, content); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("database：遍历结果失败：%w", err)
	}
	return nil
}
