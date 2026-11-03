// Package alert 安全事件告警（PR② 等保三级 8.1.5.4f「应对安全事件进行识别、报警」）。
//
// 架构原则（PLAN 拍板）：**显式调用点，不挂全局 logger**——防告警风暴、防循环告警，
// 每处埋点在 code review 可见。四类事件：login_lockout / insecure_defaults /
// log_tamper_attempt / 服务异常 ERROR 白名单（redis_unavailable、migration_failure）。
//
// 语义分层：
//   - 落库永远执行（不可配置关闭，等保 8.1.4.3c 审计记录保护）；落库失败仅 ERROR 日志
//     不阻断业务——告警链路不得反向拖垮主流程（与登录锁定层 fail-open 同哲学）。
//   - 外送（钉钉/shoutrrr 通道）受 config alert.enabled 控制，同 eventType+bizKey 在
//     节流窗内去重（防爆破风暴刷爆通道；落库不受节流影响，记录完整保留）。
//   - 投递异步：先落库再入队，worker 带指数退避重试；最终失败置 delivered=false +
//     last_error，启动扫描重投（进程重启丢队列不丢记录）。
//
// 存储收口在 recordStore 接口：生产用 gormStore（gorm 直连 entity.AlertRecord），
// 单测用内存 fake——本包单测零库依赖。
package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
)

// Config 告警配置（config.yaml alert: 段；归一在 config.applyAlertDefaults 完成）。
type Config struct {
	Enabled         bool          // 外送开关（默认 false=只落库不外送；落库不受此开关控制）
	Channels        []string      // shoutrrr 通道 URL（lark:// wecom:// smtp:// generic://）
	DingTalkToken   string        // 钉钉机器人 access_token（空=不启用钉钉）
	DingTalkSecret  string        // 钉钉加签密钥（HMAC-SHA256；生产走 env 覆盖）
	QueueSize       int           // 投递队列容量；<=0 → 64
	RetryMax        int           // 单条投递最大重试次数；<=0 → 5
	ThrottleMinutes int           // 同 eventType+bizKey 外送节流窗（分钟）；<0 归一为 0
	RetryBaseDelay  time.Duration // 重试基础退避（测试注入；零值=1s）
}

// recordStore 告警记录存储（gorm 实现见文件尾；单测注入内存 fake）。
type recordStore interface {
	Create(rec *entity.AlertRecord) error
	UpdateResult(id uint, delivered bool, retries int, lastErr string) error
	Undelivered(limit, retryMax int) ([]entity.AlertRecord, error)
}

// gormStore recordStore 的 gorm 实现（直连 entity.AlertRecord，不经 persistence 层——
// 避免 db→alert→persistence→db 的 import 环；告警是基础设施级旁路，不需要仓储抽象全套）。
type gormStore struct{ db *gorm.DB }

func (s gormStore) Create(rec *entity.AlertRecord) error {
	return s.db.Create(rec).Error
}

func (s gormStore) UpdateResult(id uint, delivered bool, retries int, lastErr string) error {
	return s.db.Model(&entity.AlertRecord{}).Where("id = ?", id).
		Updates(map[string]any{"delivered": delivered, "retries": retries, "last_error": lastErr}).Error
}

func (s gormStore) Undelivered(limit, retryMax int) ([]entity.AlertRecord, error) {
	var recs []entity.AlertRecord
	err := s.db.Where("delivered = ? AND retries < ?", false, retryMax).
		Order("id asc").Limit(limit).Find(&recs).Error
	return recs, err
}

type dispatchTask struct {
	recordId uint
	title    string
	body     string
}

// sendFunc 向全部通道投递一条消息，返回合并错误（nil=全部通道成功或未配置通道）。
type sendFunc func(title, body string) error

var (
	defaultMu sync.RWMutex
	def       *Dispatcher
)

// Init 构造进程级 Dispatcher 并注册为埋点入口（main 在 InitDB 后、RunMigrations 前
// 调用——迁移失败告警在迁移执行点发出）。worker 另经 StartWorker 启动。
func Init(db *gorm.DB, cfg Config) *Dispatcher {
	d := New(gormStore{db: db}, cfg)
	defaultMu.Lock()
	def = d
	defaultMu.Unlock()
	return d
}

// StartWorker 启动投递 worker 与启动重投扫描（main 在迁移与 redis 就绪后调用一次）。
func StartWorker(ctx context.Context) {
	d := Default()
	if d == nil {
		return
	}
	d.Start(ctx)
}

// Default 当前进程 Dispatcher（未装配时 nil）。
func Default() *Dispatcher {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return def
}

// Dispatch 进程级埋点入口：未装配时静默 no-op（单测/未启用场景）。
func Dispatch(eventType, severity, title, body, bizKey string) {
	d := Default()
	if d == nil {
		return
	}
	d.Dispatch(eventType, severity, title, body, bizKey)
}

// Dispatcher 告警调度器：落库（同步必做）+ 异步外送（受开关与节流控制）。
type Dispatcher struct {
	store       recordStore
	cfg         Config
	taskCh      chan dispatchTask
	mu          sync.Mutex
	lastSent    map[string]time.Time // 节流键 eventType|bizKey → 上次外送时刻
	send        sendFunc             // 外送函数（测试注入）
	now         func() time.Time     // 时钟（测试注入）
	sleep       func(time.Duration)  // 退避睡眠（测试注入断言序列；默认 time.Sleep）
	persistSent map[string]time.Time // 基建噪声事件落库节流键（与外送节流分簿）
	wg          sync.WaitGroup
}

// New 构造 Dispatcher（不启动 worker；测试与 Init 共用）。
func New(store recordStore, cfg Config) *Dispatcher {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 64
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = 5
	}
	if cfg.ThrottleMinutes < 0 {
		cfg.ThrottleMinutes = 0
	}
	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = time.Second
	}
	return &Dispatcher{
		store:       store,
		cfg:         cfg,
		taskCh:      make(chan dispatchTask, cfg.QueueSize),
		lastSent:    map[string]time.Time{},
		persistSent: map[string]time.Time{},
		send:        realSend(cfg),
		now:         time.Now,
		sleep:       time.Sleep,
	}
}

// Start 启动 worker 与重投扫描（幂等调用无害但只需一次）。
func (d *Dispatcher) Start(ctx context.Context) {
	d.sweepUndelivered(ctx)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-d.taskCh:
				d.deliver(t)
			}
		}
	}()
}

// Dispatch 落库（同步、不可关闭）+ 入队外送（异步、受开关与节流控制）。
// 任何错误只记 ERROR 日志，绝不向上返回——调用方是业务埋点，告警失败不能拖垮业务。
func (d *Dispatcher) Dispatch(eventType, severity, title, body, bizKey string) {
	// 基建噪声事件（redis/迁移故障）落库侧同键窗口去重：故障风暴期间登录路径
	// 每请求都走到埋点，无鉴权可驱动，按请求速率写表=结构性无界（审计 B2）。
	if persistThrottledEvents[eventType] && d.throttledIn(&d.persistSent, eventType, bizKey) {
		slog.Warn("基建噪声告警窗内抑制（明细在业务日志表）", "eventType", eventType, "bizKey", bizKey)
		return
	}
	rec := &entity.AlertRecord{
		EventType: eventType,
		Severity:  severity,
		Title:     clip(title, 200),
		Body:      body,
		BizKey:    clip(bizKey, 200),
		Delivered: false,
		CreatedAt: d.now(),
	}
	if err := d.store.Create(rec); err != nil {
		slog.Error("告警落库失败（业务不受影响）", "err", err, "eventType", eventType, "title", title)
		return
	}
	if !d.cfg.Enabled {
		return // 外送关闭：只落库（合规基线），delivered 保持 false
	}
	if d.throttled(eventType, bizKey) {
		slog.Warn("告警外送被节流（落库已保留）", "eventType", eventType, "bizKey", bizKey,
			"recordId", rec.Id, "throttleMinutes", d.cfg.ThrottleMinutes)
		return
	}
	select {
	case d.taskCh <- dispatchTask{recordId: rec.Id, title: rec.Title, body: body}:
	default:
		// 队列满：本次外送降级为待重投（落库已在，启动扫描会拾取）
		slog.Error("告警投递队列满，外送降级为待重投", "recordId", rec.Id, "eventType", eventType)
		d.markResult(rec.Id, false, 0, "queue full")
	}
}

// throttleSentMapSize 节流键 map 的容量上限（超出即修剪过期键，防攻击者用可控
// bizKey 无限撑大进程内存——审计 B4 实测 5 万 distinct 用户名=5 万条目不回收）。
const throttleSentMapSize = 4096

// throttled 同 eventType+bizKey 在节流窗内已外送过则 true；bizKey 为空不节流。
// 注意：节流只影响外送，不拦落库（等保口径：记录完整、通道防刷）——例外见
// persistThrottledEvents（基建噪声事件落库侧同键去重）。
func (d *Dispatcher) throttled(eventType, bizKey string) bool {
	return d.throttledIn(&d.lastSent, eventType, bizKey)
}

// throttledIn 同键窗口判重的通用实现（外送 map 与落库 map 共用；键截断防长 bizKey）。
func (d *Dispatcher) throttledIn(m *map[string]time.Time, eventType, bizKey string) bool {
	if bizKey == "" || d.cfg.ThrottleMinutes <= 0 {
		return false
	}
	key := clip(eventType+"|"+bizKey, 220)
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	if len(*m) >= throttleSentMapSize {
		d.pruneLocked(m, now)
	}
	if last, ok := (*m)[key]; ok && now.Sub(last) < time.Duration(d.cfg.ThrottleMinutes)*time.Minute {
		return true
	}
	(*m)[key] = now
	return false
}

// pruneLocked 淘汰窗口外的键；窗口全未过期时淘汰时间最早的一条（map 无序——
// 时间全等时任取一条，保证上限收敛；固定时钟下的等值场景被测试钉住）。调用方持锁。
func (d *Dispatcher) pruneLocked(m *map[string]time.Time, now time.Time) {
	window := time.Duration(d.cfg.ThrottleMinutes) * time.Minute
	oldestKey := ""
	var oldestAt time.Time
	for k, at := range *m {
		if now.Sub(at) >= window {
			delete(*m, k)
			continue
		}
		if oldestKey == "" || at.Before(oldestAt) {
			oldestKey, oldestAt = k, at
		}
	}
	if oldestKey != "" && len(*m) >= throttleSentMapSize {
		delete(*m, oldestKey)
	}
}

// persistThrottledEvents 基建噪声事件：落库侧也按同键窗口去重（审计 B2 实证 redis
// 故障期间无鉴权请求可按请求速率无限写表）。报警通道语义：首条全量、窗内抑制——
// 明细留痕在 aa_login_log/aa_oper_log，不因抑制丢失；安全事件（lockout/tamper）
// 不在此列，落库恒全量。
var persistThrottledEvents = map[string]bool{
	"redis_unavailable": true,
	"migration_failure": true,
}

// deliver 单条投递：指数退避重试（base*2^attempt），终态回写 delivered/last_error。
func (d *Dispatcher) deliver(t dispatchTask) {
	if d.send == nil {
		return
	}
	var lastErr error
	for attempt := 0; attempt <= d.cfg.RetryMax; attempt++ {
		if attempt > 0 {
			// 指数退避：base*2^(attempt-1)——通道故障风暴下的让路语义。
			d.sleep(d.cfg.RetryBaseDelay << (attempt - 1))
		}
		lastErr = d.send(t.title, t.body)
		if lastErr == nil {
			d.markResult(t.recordId, true, attempt, "")
			return
		}
		slog.Warn("告警外送失败", "err", lastErr, "recordId", t.recordId, "attempt", attempt)
	}
	d.markResult(t.recordId, false, d.cfg.RetryMax, lastErr.Error())
}

// markResult 回写投递结果；失败仅日志（记录侧自愈靠启动重投扫描兜底）。
func (d *Dispatcher) markResult(id uint, delivered bool, retries int, lastErr string) {
	if err := d.store.UpdateResult(id, delivered, retries, lastErr); err != nil {
		slog.Error("告警投递结果回写失败", "err", err, "recordId", id)
	}
}

// sweepUndelivered 启动扫描：未投递且重试未耗尽的历史记录重新入队
// （进程重启丢队列的场景；重试耗尽的留档不再投，避免死循环）。
func (d *Dispatcher) sweepUndelivered(ctx context.Context) {
	recs, err := d.store.Undelivered(100, d.cfg.RetryMax)
	if err != nil {
		slog.Warn("告警未投递记录扫描失败", "err", err)
		return
	}
	seenKey := map[string]bool{}
	for _, r := range recs {
		// 重放防突发（审计 B3）：同节流键只重投最早一条，其余留档待下次启动——
		// 否则关闭期/故障期积累的 delivered=false 会在启用或重启时一次性倾泻通道。
		key := clip(r.EventType+"|"+r.BizKey, 220)
		if seenKey[key] {
			continue
		}
		seenKey[key] = true
		select {
		case d.taskCh <- dispatchTask{recordId: r.Id, title: r.Title, body: r.Body}:
		default:
			return // 队列满：剩余留给下次启动
		}
	}
	if len(recs) > 0 {
		slog.Info("告警启动重投", "count", len(recs))
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	// 字节截断可能劈开 UTF-8 序列（审计 B8①）：回退到合法边界。
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
