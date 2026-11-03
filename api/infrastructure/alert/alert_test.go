package alert

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aiarsenal/domain/entity"
)

// ---- 钉钉签名向量（国标式锁实现：固定 secret+时间戳 → 固定签名，改算法必红）----

func TestSignDingTalkVector(t *testing.T) {
	const secret = "SEC68f2a1b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7"
	const ts = int64(1730700000000)
	const want = "%2FisvP8CBs2ZK2XDUbDCNP7Qxw1Vn5E9LHRFQ%2FxpgOGc%3D"
	if got := SignDingTalk(secret, ts); got != want {
		t.Fatalf("SignDingTalk 向量不符\n got=%s\nwant=%s", got, want)
	}
	// 时间戳或密钥任一变化，签名必须变化（防「常数签名」实现）
	if SignDingTalk(secret, ts+1) == want || SignDingTalk(secret+"x", ts) == want {
		t.Fatal("签名对时间戳/密钥不敏感，加签形同虚设")
	}
}

// ---- 测试用 fake store ----

type fakeStore struct {
	mu        sync.Mutex
	recs      []*entity.AlertRecord
	nextID    uint
	createErr error
	results   []string // "id:delivered:retries:lastErr" 调用序
}

func (f *fakeStore) Create(rec *entity.AlertRecord) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	rec.Id = f.nextID
	cp := *rec
	f.recs = append(f.recs, &cp)
	return nil
}

func (f *fakeStore) UpdateResult(id uint, delivered bool, retries int, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, strconv.FormatUint(uint64(id), 10)+":"+
		strconv.FormatBool(delivered)+":"+strconv.Itoa(retries)+":"+lastErr)
	for _, r := range f.recs {
		if r.Id == id {
			r.Delivered = delivered
			r.Retries = retries
			r.LastError = lastErr
		}
	}
	return nil
}

func (f *fakeStore) Undelivered(limit, retryMax int) ([]entity.AlertRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []entity.AlertRecord
	for _, r := range f.recs {
		if !r.Delivered && r.Retries < retryMax {
			out = append(out, *r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// newTestDispatcher 注入 fake store/固定时钟/计数 send，并启动 worker。
// 第三返回值是 send 调用次数的加锁读函数（worker 与测试并发读写，禁裸指针）。
func newTestDispatcher(t *testing.T, cfg Config) (*Dispatcher, *fakeStore, func() int) {
	t.Helper()
	fs := &fakeStore{}
	calls := 0
	var mu sync.Mutex
	d := New(fs, cfg)
	d.store = fs // New 内部已包同一 store；显式回填保证测试引用一致
	d.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	d.send = func(title, body string) error {
		mu.Lock()
		calls++
		mu.Unlock()
		return nil
	}
	d.Start(context.Background())
	return d, fs, func() int {
		mu.Lock()
		defer mu.Unlock()
		return calls
	}
}

// waitFor 等待条件成立（worker 异步语义的同步点；超时 fail）。
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(msg)
}

// ---- Dispatch 语义 ----

// 外送关闭（默认态）：落库发生、外送不发生、delivered=false。
func TestDispatchPersistOnlyWhenDisabled(t *testing.T) {
	d, fs, calls := newTestDispatcher(t, Config{Enabled: false})
	d.Dispatch("login_lockout", "warning", "t", "b", "user1")
	if len(fs.recs) != 1 {
		t.Fatalf("落库条数=%d want 1", len(fs.recs))
	}
	if fs.recs[0].Delivered {
		t.Fatal("外送关闭时 delivered 必须为 false")
	}
	time.Sleep(20 * time.Millisecond) // 若实现错误地入队，给 worker 机会暴露
	if n := calls(); n != 0 {
		t.Fatalf("外送关闭时 send 被调用 %d 次", n)
	}
}

// 外送开启：落库+入队投递，成功后 delivered=true。
func TestDispatchEnablesDelivery(t *testing.T) {
	d, fs, calls := newTestDispatcher(t, Config{Enabled: true, RetryBaseDelay: time.Millisecond})
	d.Dispatch("login_lockout", "warning", "t", "b", "user1")
	waitFor(t, func() bool {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		return calls() == 1 && len(fs.recs) == 1 && fs.recs[0].Delivered
	}, "投递未收敛（send 或 markResult 未完成）")
}

// 节流：同 eventType+bizKey 窗口内第二次只落库不外送；bizKey 为空不节流。
func TestThrottleDedupsDeliveryButKeepsRecords(t *testing.T) {
	d, fs, calls := newTestDispatcher(t, Config{Enabled: true, ThrottleMinutes: 10})
	d.Dispatch("login_lockout", "warning", "t1", "b", "user1")
	d.Dispatch("login_lockout", "warning", "t2", "b", "user1")   // 被节流
	d.Dispatch("login_lockout", "warning", "t3", "b", "user2")   // 不同 bizKey 不节流
	d.Dispatch("migration_failure", "error", "t4", "b", "user1") // 不同 eventType 不节流
	waitFor(t, func() bool { return calls() == 3 }, "send 调用次数不为 3")
	if len(fs.recs) != 4 {
		t.Fatalf("节流不得拦落库：落库 %d 条 want 4", len(fs.recs))
	}
}

// 落库失败：不 panic、不外送、不影响业务（调用方无返回值即无阻断面）。
func TestDispatchStoreFailureIsSwallowed(t *testing.T) {
	fs := &fakeStore{createErr: errors.New("db down")}
	d, _, calls := newTestDispatcher(t, Config{Enabled: true})
	d.store = fs
	d.Dispatch("migration_failure", "error", "t", "b", "")
	if n := calls(); n != 0 {
		t.Fatalf("落库失败时不得外送（send 被调 %d 次）", n)
	}
}

// ---- 钉钉 HTTP 语义（httptest 回显）----

func TestSendDingTalkHTTP(t *testing.T) {
	orig := dingtalkEndpoint
	defer func() { dingtalkEndpoint = orig }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sign") == "" || r.URL.Query().Get("timestamp") == "" {
			t.Error("缺少加签参数")
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()
	dingtalkEndpoint = srv.URL
	if err := sendDingTalk("tok", "sec", "标题", "正文", srv.Client()); err != nil {
		t.Fatalf("errcode=0 应成功: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"sign not match"}`))
	}))
	defer bad.Close()
	dingtalkEndpoint = bad.URL
	if err := sendDingTalk("tok", "sec", "t", "b", bad.Client()); err == nil {
		t.Fatal("errcode!=0 必须报错以触发重试")
	} else if !strings.Contains(err.Error(), "310000") {
		t.Fatalf("错误应含 errcode: %v", err)
	}
}

// ---- 重试与启动重投 ----

// 重试退避：前 2 次失败第 3 次成功 → delivered=true 且 retries=2。
func TestDeliverRetriesThenSucceeds(t *testing.T) {
	d, fs, _ := newTestDispatcher(t, Config{Enabled: true, RetryMax: 5, RetryBaseDelay: time.Millisecond})
	attempt := 0
	d.send = func(title, body string) error {
		attempt++
		if attempt < 3 {
			return errors.New("boom")
		}
		return nil
	}
	_ = d.store.Create(&entity.AlertRecord{EventType: "e", Title: "t", CreatedAt: d.now()})
	d.deliver(dispatchTask{recordId: fs.recs[0].Id, title: "t", body: "b"})
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if !fs.recs[0].Delivered || fs.recs[0].Retries != 2 {
		t.Fatalf("delivered=%v retries=%d want true/2", fs.recs[0].Delivered, fs.recs[0].Retries)
	}
}

// 重试耗尽：delivered=false + last_error；sweep 不重投耗尽记录（防死循环），
// 只重投「未耗尽未投递」的记录（如重启时队列中丢失的）。
func TestDeliverExhaustedAndSweepSemantics(t *testing.T) {
	d, fs, _ := newTestDispatcher(t, Config{Enabled: true, RetryMax: 2, RetryBaseDelay: time.Millisecond})
	fail := true
	d.send = func(title, body string) error {
		if fail {
			return errors.New("down")
		}
		return nil
	}
	// 记录 A：重试耗尽（retries=2=RetryMax）
	_ = d.store.Create(&entity.AlertRecord{EventType: "e", Title: "A", CreatedAt: d.now()})
	d.deliver(dispatchTask{recordId: fs.recs[0].Id, title: "A", body: "b"})
	// 记录 B：部分重试未耗尽（模拟重启丢队列：delivered=false、retries=1）
	_ = d.store.Create(&entity.AlertRecord{EventType: "e", Title: "B", Retries: 1, CreatedAt: d.now()})
	fs.mu.Lock()
	if fs.recs[0].Delivered || fs.recs[0].Retries != 2 || !strings.Contains(fs.recs[0].LastError, "down") {
		t.Fatalf("A 耗尽后应 delivered=false/retries=2/last_error=down: %+v", fs.recs[0])
	}
	fs.mu.Unlock()

	fail = false
	d.sweepUndelivered(context.Background())
	waitFor(t, func() bool {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		return fs.recs[1].Delivered && !fs.recs[0].Delivered
	}, "sweep 应只重投 B（A 耗尽留档不重投）")
}

// ---- 白名单 scheme ----

func TestSenderBySchemeWhitelist(t *testing.T) {
	for _, s := range []string{"lark", "wecom", "smtp", "generic"} {
		if _, err := senderByScheme(s); err != nil {
			t.Fatalf("scheme %q 应在白名单: %v", s, err)
		}
	}
	if _, err := senderByScheme("slack"); err == nil {
		t.Fatal("白名单外 scheme 必须拒绝（不引 pkg/router 全表）")
	}
}

// ---- 审计处置补网 ----

// B1：钉钉失败错误串不得含 access_token/sign（凭据经 *url.Error 进日志+last_error 实锤过）。
func TestSendDingTalkErrorHasNoCredential(t *testing.T) {
	const token = "SECRET_TOKEN_abc123"
	const secret = "SEC_secret_xyz789"
	orig := dingtalkEndpoint
	defer func() { dingtalkEndpoint = orig }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dingtalkEndpoint = srv.URL
	srv.Close() // 立即关闭制造连接拒绝（通道断开场景）

	err := sendDingTalk(token, secret, "t", "b", http.DefaultClient)
	if err == nil {
		t.Fatal("连接拒绝应返回错误")
	}
	msg := err.Error()
	if strings.Contains(msg, token) || strings.Contains(msg, "sign=") || strings.Contains(msg, "access_token=") {
		t.Fatalf("错误串泄漏凭据: %s", msg)
	}
}

// B2：基建噪声事件（redis_unavailable/migration_failure）落库侧同键窗内抑制；
// 安全事件（login_lockout）不受影响——落库恒全量。
func TestPersistThrottleInfraNoiseOnly(t *testing.T) {
	d, fs, _ := newTestDispatcher(t, Config{Enabled: false, ThrottleMinutes: 10})
	for i := 0; i < 5; i++ {
		d.Dispatch("redis_unavailable", "error", "t", "b", "redis")
	}
	for i := 0; i < 5; i++ {
		d.Dispatch("login_lockout", "warning", "t", "b", "user1")
	}
	if n := len(fs.recs); n != 6 {
		t.Fatalf("redis 噪声 5 抑 1 + lockout 5 全量 = 6 行，got %d", n)
	}
	// migration_failure 同键同窗也抑制
	d.Dispatch("migration_failure", "error", "t", "b", "aiarsenal")
	d.Dispatch("migration_failure", "error", "t", "b", "aiarsenal")
	if n := len(fs.recs); n != 7 {
		t.Fatalf("migration_failure 5 抑 1 = 7 行，got %d", n)
	}
}

// B3：sweep 同节流键只重投最早一条，其余留档（防重启/启用时重放突发）。
func TestSweepDedupsByThrottleKey(t *testing.T) {
	d, fs, calls := newTestDispatcher(t, Config{Enabled: true, RetryMax: 5, RetryBaseDelay: time.Millisecond})
	// 三条同键未投递 + 一条异键未投递
	for i, key := range []string{"redis", "redis", "redis", "other"} {
		_ = d.store.Create(&entity.AlertRecord{EventType: "e", Title: strconv.Itoa(i), BizKey: key, CreatedAt: d.now()})
	}
	d.sweepUndelivered(context.Background())
	waitFor(t, func() bool { return calls() == 2 }, "sweep 应只重投 2 条（同键取最早）")
	if n := len(fs.recs); n != 4 {
		t.Fatalf("落库不受 sweep 影响：4 行，got %d", n)
	}
}

// B4：节流 map 容量上限收敛（攻击者用可控 bizKey 撑 map 实证过 5 万条目）。
func TestThrottleMapBounded(t *testing.T) {
	d, _, _ := newTestDispatcher(t, Config{Enabled: true, ThrottleMinutes: 10})
	for i := 0; i < throttleSentMapSize+500; i++ {
		d.Dispatch("login_lockout", "warning", "t", "b", "user-"+strconv.Itoa(i))
	}
	d.mu.Lock()
	n := len(d.lastSent)
	d.mu.Unlock()
	if n > throttleSentMapSize {
		t.Fatalf("节流 map 超上限：%d > %d", n, throttleSentMapSize)
	}
}
