// Package logx 统一日志：slog + 按日切割文件 + stdout。
// 各服务复制一份（4 个独立 go module），main 开头调 Init。
//
// 输出同时去两处：stdout（manager→docker logs，宿主服务→journalctl 实时看）
// 和按日切割文件（每天一个 <dir>/<prefix>-YYYY-MM-DD.log）。
// slog 默认 logger + 标准 log 都重定向到同一目的地——旧代码 log.Printf 自动进文件，
// 新代码用 slog.Info/Error 等结构化输出。
package logx

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyRotateWriter 按日切割的 io.Writer：每天一个文件 <dir>/<prefix>-YYYY-MM-DD.log。
// 跨日首次 Write 时切换到新文件、关闭旧的。并发安全。
type DailyRotateWriter struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	curDate string // "2006-01-02"
	curFile *os.File
}

func NewDailyRotateWriter(dir, prefix string) *DailyRotateWriter {
	return &DailyRotateWriter{dir: dir, prefix: prefix}
}

func (w *DailyRotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if today != w.curDate {
		if w.curFile != nil {
			_ = w.curFile.Close()
		}
		if err := os.MkdirAll(w.dir, 0755); err != nil {
			return 0, fmt.Errorf("创建日志目录 %s: %w", w.dir, err)
		}
		path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, today))
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, fmt.Errorf("打开日志文件 %s: %w", path, err)
		}
		w.curFile = f
		w.curDate = today
	}
	return w.curFile.Write(p)
}

// Init 配置全局日志：slog 默认 logger + 标准 log 都输出到「stdout + 按日切割文件」。
//
//	dir    日志目录（manager=/aa/.aa/logs；宿主服务=/var/log/aiarsenal/<service>）
//	prefix 文件名前缀（manager/api/mcp/llm）
//	level  slog 级别（slog.LevelInfo / LevelDebug ...）
func Init(dir, prefix string, level slog.Level) {
	multi := io.MultiWriter(os.Stdout, NewDailyRotateWriter(dir, prefix))
	logger := slog.New(slog.NewTextHandler(multi, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	log.SetOutput(multi) // 桥接标准 log：旧 log.Printf 进同一文件
}

// InitEnv 同 Init，但日志目录可用环境变量 AIARSENAL_LOG_DIR 覆盖
// （Windows 等无 /var/log 的环境指到仓库 .run/logs；不设置时行为与 Init 完全一致）。
func InitEnv(dir, prefix string, level slog.Level) {
	if v := os.Getenv("AIARSENAL_LOG_DIR"); v != "" {
		dir = v
	}
	Init(dir, prefix, level)
}
