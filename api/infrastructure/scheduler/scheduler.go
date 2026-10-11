// Package scheduler 轻量进程内 cron 调度器。
//
// 供需要定时触发的模块共用：Agent 备份计划（aa_agent_backup_schedule.cron_expression）、
// KB 数据源定时同步（aa_kb_datasource.schedule）等。基于 robfig/cron/v3（5 字段标准 cron），
// 以「业务名」为键注册——同名重复注册即替换（Upsert），业务侧无需自己维护 EntryID。
//
// 用法：
//
//	sch := scheduler.New()
//	sch.Start()
//	defer sch.Stop()
//	_ = sch.Register("kb-ds-<bizId>", "0 3 * * *", func(ctx context.Context) { ... })
//	sch.Unregister("kb-ds-<bizId>") // 停用同步
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"log/slog"
)

// Scheduler 进程内 cron 调度器。并发安全。
type Scheduler struct {
	c   *cron.Cron
	mu  sync.Mutex
	ids map[string]cron.EntryID
}

// New 创建未启动的调度器。秒级不需要，标准 5 字段（分 时 日 月 周）。
func New() *Scheduler {
	return &Scheduler{
		c:   cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger))),
		ids: make(map[string]cron.EntryID),
	}
}

// Start 启动调度循环。重复调用无副作用。
func (s *Scheduler) Start() {
	s.c.Start()
}

// Stop 停止并等待正在执行的任务结束（最多等 10s）。
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	}
}

// Register 注册（或替换同名）定时任务。spec 为空返回错误（表示该任务不需要调度）；
// cron 表达式非法返回解析错误。任务 panic 已由 cron.Recover 兜底，不会带崩调度循环。
func (s *Scheduler) Register(name, spec string, fn func(ctx context.Context)) error {
	if spec == "" {
		return fmt.Errorf("scheduler: 空 cron 表达式（%s）", name)
	}
	id, err := s.c.AddFunc(spec, func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scheduler: 任务 panic", "name", name, "err", r)
			}
		}()
		fn(context.Background())
	})
	if err != nil {
		return fmt.Errorf("scheduler: cron 解析失败（%s, %q）: %w", name, spec, err)
	}
	s.mu.Lock()
	// 同名替换：先摘旧的再记新的，保证「停用再启用」场景不留双份。
	if old, ok := s.ids[name]; ok {
		s.c.Remove(old)
	}
	s.ids[name] = id
	s.mu.Unlock()
	slog.Info("scheduler: 已注册任务", "name", name, "spec", spec)
	return nil
}

// Unregister 摘除任务；不存在时静默。
func (s *Scheduler) Unregister(name string) {
	s.mu.Lock()
	id, ok := s.ids[name]
	if ok {
		delete(s.ids, name)
	}
	s.mu.Unlock()
	if ok {
		s.c.Remove(id)
	}
}

// Has 查询任务是否在册。
func (s *Scheduler) Has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ids[name]
	return ok
}
