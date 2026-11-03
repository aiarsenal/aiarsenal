package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// fakeOperLogRepo 只关心 Insert：其余方法在审计中间件里不被调用。
type fakeOperLogRepo struct {
	inserted   []*entity.OperLog
	err        error
	lastCtxErr error
}

var _ repository.IOperLogRepository = (*fakeOperLogRepo)(nil)

func (f *fakeOperLogRepo) List(context.Context, string, string, string, int, int) ([]*model.OperLogListRes, int64, error) {
	return nil, 0, nil
}
func (f *fakeOperLogRepo) Get(context.Context, string) (*model.OperLogInfoRes, error) {
	return nil, nil
}
func (f *fakeOperLogRepo) Insert(ctx context.Context, log *entity.OperLog) error {
	f.lastCtxErr = ctx.Err()
	f.inserted = append(f.inserted, log)
	return f.err
}
func (f *fakeOperLogRepo) Delete(context.Context, []string) error { return nil }
func (f *fakeOperLogRepo) Clear(context.Context) error            { return nil }

// PurgeBefore 满足 IOperLogRepository（留存清理接口；中间件测试不触达）。
func (f *fakeOperLogRepo) PurgeBefore(context.Context, time.Time) (int64, error) { return 0, nil }

// auditRouter 复刻真实挂载顺序：OperLog 在 v1 根分组，认证中间件在其后，
// 因此入口处 context 里没有用户——这正是最容易写错的地方。
func auditRouter(repo repository.IOperLogRepository, withUser bool, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, nil))
	if withUser {
		v1.Use(func(c *gin.Context) {
			SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
		})
	}
	v1.POST("/system/user/edit", handler)
	v1.POST("/client/heartbeat", handler)
	return r
}

func doPost(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOperLogRecordsWriteWithMaskedParam(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) {
		Success(c, gin.H{"ok": true})
	})

	w := doPost(t, r, "/api/v1/system/user/edit", `{"userId":"victim","password":"s3cret"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("响应码 = %d，期望 200", w.Code)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("落库条数 = %d，期望 1", len(repo.inserted))
	}
	got := repo.inserted[0]
	if got.Title != "system/user" {
		t.Errorf("Title = %q，期望模块级前缀", got.Title)
	}
	if got.OperUrl != "/api/v1/system/user/edit" {
		t.Errorf("OperUrl = %q", got.OperUrl)
	}
	if got.RequestMethod != http.MethodPost || got.BusinessType != businessTypeOther {
		t.Errorf("method/businessType = %s/%d，POST 到 edit 应记「其它」", got.RequestMethod, got.BusinessType)
	}
	if got.OperName != "alice" {
		t.Errorf("OperName = %q", got.OperName)
	}
	// OperId 由仓储 Insert 统一生成，中间件不造第二套 ID 规则。
	if got.OperId != "" {
		t.Errorf("OperId 应由仓储生成，中间件不该赋值：%q", got.OperId)
	}
	if got.OperTime == nil {
		t.Error("OperTime 为空")
	}
	if strings.Contains(got.OperParam, "s3cret") {
		t.Errorf("密码未掩码：%s", got.OperParam)
	}
	if !strings.Contains(got.OperParam, "victim") {
		t.Errorf("非敏感字段被误删：%s", got.OperParam)
	}
}

// 处理器必须读到完整请求体：中间件读过之后要还原，否则业务静默收到空 body。
func TestOperLogRestoresRequestBody(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil {
			Fail(c, err.Error())
			return
		}
		Success(c, string(raw))
	})

	w := doPost(t, r, "/api/v1/system/user/edit", `{"userId":"u9"}`)

	if !strings.Contains(w.Body.String(), `{\"userId\":\"u9\"}`) {
		t.Fatalf("处理器读到的 body 不完整：%s", w.Body.String())
	}
}

func TestOperLogSkipsReadRequests(t *testing.T) {
	repo := &fakeOperLogRepo{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, nil), func(c *gin.Context) {
		SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
	})
	v1.GET("/system/user/list", func(c *gin.Context) { Success(c, nil) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/user/list", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(repo.inserted) != 0 {
		t.Fatalf("GET 被记录了 %d 条", len(repo.inserted))
	}
}

// 心跳按设备每几秒一次，记进审计会把真正的写操作淹没。
func TestOperLogSkipsHeartbeat(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) { Success(c, nil) })

	doPost(t, r, "/api/v1/client/heartbeat", `{}`)

	if len(repo.inserted) != 0 {
		t.Fatalf("心跳被记录了 %d 条", len(repo.inserted))
	}
}

// 未登录请求（登录、验证码等）由各自的登录日志负责，不进操作审计。
func TestOperLogSkipsUnauthenticated(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, false, func(c *gin.Context) { Success(c, nil) })

	doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if len(repo.inserted) != 0 {
		t.Fatalf("未登录请求被记录了 %d 条", len(repo.inserted))
	}
}

// 业务失败走 HTTP 200 + code=-1，只看状态码会漏，必须解析响应体。
func TestOperLogCapturesBusinessError(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) { Fail(c, "没有访问权限") })

	doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if len(repo.inserted) != 1 {
		t.Fatalf("落库条数 = %d，期望 1", len(repo.inserted))
	}
	if repo.inserted[0].ErrorMsg != "没有访问权限" {
		t.Errorf("ErrorMsg = %q", repo.inserted[0].ErrorMsg)
	}
}

// 审计是旁路：写库失败不能改变业务响应。
func TestOperLogInsertFailureKeepsResponse(t *testing.T) {
	repo := &fakeOperLogRepo{err: context.DeadlineExceeded}
	r := auditRouter(repo, true, func(c *gin.Context) { Success(c, gin.H{"ok": true}) })

	w := doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("业务响应被审计失败影响：%d %s", w.Code, w.Body.String())
	}
}

func TestMaskSensitiveFormBody(t *testing.T) {
	got := maskSensitive([]byte("userName=bob&password=p%40ss&token=abc"), "application/x-www-form-urlencoded")
	for _, leaked := range []string{"p%40ss", "abc"} {
		if strings.Contains(got, leaked) {
			t.Errorf("表单敏感值未掩码：%s", got)
		}
	}
	if !strings.Contains(got, "userName=bob") {
		t.Errorf("非敏感字段被误删：%s", got)
	}
}

// 超长错误消息必须先裁剪再落库：error_msg 是 varchar(2000)，
// 直接写会让 INSERT 失败，那条审计静默消失——恰好是最该留痕的长错误场景。
func TestOperLogClipsLongBusinessError(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) {
		Fail(c, strings.Repeat("错", 3000))
	})

	doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if len(repo.inserted) != 1 {
		t.Fatalf("落库条数 = %d，期望 1", len(repo.inserted))
	}
	if n := len([]rune(repo.inserted[0].ErrorMsg)); n != colErrorMsgLen {
		t.Fatalf("ErrorMsg 字符数 = %d，期望裁到 %d", n, colErrorMsgLen)
	}
}

// title 列只有 50 字符，路由变长时必须裁而不是让整条审计写不进去。
func TestOperLogClipsLongTitle(t *testing.T) {
	repo := &fakeOperLogRepo{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, nil), func(c *gin.Context) {
		SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
	})
	long := "/" + strings.Repeat("a", 80)
	v1.POST(long, func(c *gin.Context) { Success(c, nil) })

	doPost(t, r, "/api/v1"+long, `{}`)

	if len(repo.inserted) != 1 {
		t.Fatalf("落库条数 = %d", len(repo.inserted))
	}
	if n := len([]rune(repo.inserted[0].Title)); n != colTitleLen {
		t.Fatalf("Title 字符数 = %d，期望裁到 %d", n, colTitleLen)
	}
}

// 响应体超过缓存上限时会被截断，JSON 不再完整。此时不能把失败记成成功，
// 必须退回文本提取（这条路径只有在错误消息超过 16KB 时才走到）。
func TestOperLogCapturesErrorFromTruncatedBody(t *testing.T) {
	repo := &fakeOperLogRepo{}
	r := auditRouter(repo, true, func(c *gin.Context) {
		Fail(c, strings.Repeat("错", 6000))
	})

	doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if len(repo.inserted) != 1 {
		t.Fatalf("落库条数 = %d", len(repo.inserted))
	}
	msg := repo.inserted[0].ErrorMsg
	if msg == "" {
		t.Fatal("响应体被截断后，失败被记成了成功")
	}
	if n := len([]rune(msg)); n > colErrorMsgLen {
		t.Fatalf("ErrorMsg 字符数 = %d，超过列宽 %d", n, colErrorMsgLen)
	}
}

// 失败计数是"审计在丢"的唯一可观测信号，必须真的加上去。
func TestInsertFailureCounterIncrements(t *testing.T) {
	before := auditInsertFailures.Load()

	insertOperLog(&fakeOperLogRepo{err: errors.New("db down")}, &entity.OperLog{OperUrl: "/x"})

	if got := auditInsertFailures.Load(); got != before+1 {
		t.Fatalf("失败计数 = %d，期望 %d", got, before+1)
	}
}

func TestBusinessTypeMapping(t *testing.T) {
	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/system/user/add", businessTypeAdd},
		{http.MethodPost, "/api/v1/system/role/save", businessTypeAdd},
		{http.MethodPost, "/api/v1/system/user/edit", businessTypeOther},
		{http.MethodPost, "/api/v1/agent/instance/i1/exec", businessTypeOther},
		{http.MethodPost, "/api/v1/system/role/bindMenus", businessTypeOther},
		{http.MethodPut, "/api/v1/system/user/edit", businessTypeEdit},
		{http.MethodDelete, "/api/v1/system/operLog/clear", businessTypeDelete},
	}
	for _, c := range cases {
		if got := businessType(c.method, c.path); got != c.want {
			t.Errorf("%s %s → %d，期望 %d", c.method, c.path, got, c.want)
		}
	}
}

// 展示字段加载慢时只能丢标题，不能吃掉写库的时间预算。
// 展示字段在后台刷新，写请求不能等它：查询还卡着，请求也要照常返回并落库。
func TestOperLogDoesNotWaitForMetaRefresh(t *testing.T) {
	repo := &fakeOperLogRepo{}
	menu := &blockingMenuSource{started: make(chan struct{}), release: make(chan struct{})}
	meta := NewAuditMeta(menu, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, meta), func(c *gin.Context) {
		SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
	})
	v1.POST("/system/user/edit", func(c *gin.Context) { Success(c, nil) })

	done := make(chan struct{})
	go func() {
		doPost(t, r, "/api/v1/system/user/edit", `{}`)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("写请求被展示字段刷新阻塞")
	}

	if len(repo.inserted) != 1 {
		t.Fatalf("审计未落库：%d 条", len(repo.inserted))
	}
	if repo.lastCtxErr != nil {
		t.Fatalf("写库上下文已被展示字段耗尽：%v", repo.lastCtxErr)
	}

	close(menu.release)
	waitAuditMetaIdle(t, meta)
}

func TestClipCountsRunes(t *testing.T) {
	got := clip(strings.Repeat("汉", 2100), 2000)
	if n := len([]rune(got)); n != 2000 {
		t.Fatalf("按字符裁剪失败：得到 %d 个字符", n)
	}
	if out := clip("短", 2000); out != "短" {
		t.Fatalf("未超长时不该改动：%q", out)
	}
}

// 处理器 panic 时也必须留下审计：500 恰恰是最需要留痕的时刻。
func TestOperLogRecordsPanic(t *testing.T) {
	repo := &fakeOperLogRepo{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, nil), func(c *gin.Context) {
		SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
	})
	v1.POST("/system/user/edit", func(c *gin.Context) { panic("boom") })

	w := doPost(t, r, "/api/v1/system/user/edit", `{}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("响应码 = %d，期望 500", w.Code)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("panic 请求未落审计：%d 条", len(repo.inserted))
	}
	if !strings.Contains(repo.inserted[0].ErrorMsg, "panic: boom") {
		t.Fatalf("panic 未记入 ErrorMsg：%q", repo.inserted[0].ErrorMsg)
	}
}

// 中文请求体超长时必须退到字符边界：切出非法 UTF-8 会让 PG 拒收整条记录。
func TestTruncateUTF8KeepsValidEncoding(t *testing.T) {
	long := `{"remark":"` + strings.Repeat("这是一段很长的中文备注", 200) + `"}`
	got := truncateUTF8(long, 2048)

	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8")
	}
	if len(got) > 2048+len("…(truncated)") {
		t.Fatalf("截断后仍超长：%d 字节", len(got))
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(got, "…(truncated)")) {
		t.Fatalf("截断改变了原文前缀")
	}
}

// 路由前缀变化时跳过规则要跟着生效，否则心跳会重新灌满审计表。
// 读请求体中途失败时，已读部分必须还给处理器，否则它拿到半截 body。
type failAfterReader struct {
	data string
	done bool
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		return copy(p, r.data), errors.New("connection reset")
	}
	return 0, errors.New("connection reset")
}
func (r *failAfterReader) Close() error { return nil }

func TestOperLogRestoresPartialBodyOnReadError(t *testing.T) {
	repo := &fakeOperLogRepo{}
	var seen string
	r := auditRouter(repo, true, func(c *gin.Context) {
		raw, _ := c.GetRawData()
		seen = string(raw)
		Success(c, nil)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/user/edit", &failAfterReader{data: `{"a":1}`})
	req.ContentLength = 7
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if seen != `{"a":1}` {
		t.Fatalf("处理器没拿到已读部分：%q", seen)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].OperParam != "[body read interrupted]" {
		t.Fatalf("读取中断的参数记录异常：%+v", repo.inserted)
	}
}

func TestIsAuditSkippedMatchesSuffix(t *testing.T) {
	for _, p := range []string{
		"/api/v1/client/heartbeat",
		"/api/v9/client/heartbeat",
		"/agent/runtime/heartbeat",
	} {
		if !isAuditSkipped(p) {
			t.Errorf("%s 应被跳过", p)
		}
	}
	if isAuditSkipped("/api/v1/system/user/edit") {
		t.Error("普通写接口不该被跳过")
	}
}

// 分块传输不读 body：长度未知时 ReadAll 会把整个流读进内存。
func TestOperLogLeavesChunkedBodyAlone(t *testing.T) {
	repo := &fakeOperLogRepo{}
	var seen string
	r := auditRouter(repo, true, func(c *gin.Context) {
		raw, _ := c.GetRawData()
		seen = string(raw)
		Success(c, nil)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/user/edit", strings.NewReader(`{"a":1}`))
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if seen != `{"a":1}` {
		t.Fatalf("处理器没拿到完整 body：%q", seen)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].OperParam != "[chunked body omitted]" {
		t.Fatalf("分块请求的参数记录 = %q", repo.inserted[0].OperParam)
	}
}

func TestAuditTitleTakesModulePrefix(t *testing.T) {
	cases := map[string]string{
		"/api/v1/agent/instance/:id/exec": "agent/instance",
		"/api/v1/system/user/edit":        "system/user",
		"/api/v1/probe/write":             "probe/write",
		"/api/v1/short":                   "short",
		"/api/v1":                         "",
	}
	for path, want := range cases {
		if got := auditTitle(path); got != want {
			t.Errorf("auditTitle(%q) = %q，期望 %q", path, got, want)
		}
	}
}

// 成功计数是统计日志的心跳，也要能验证。
func TestInsertSuccessCounterIncrements(t *testing.T) {
	before := auditInsertOK.Load()

	insertOperLog(&fakeOperLogRepo{}, &entity.OperLog{OperUrl: "/x"})

	if got := auditInsertOK.Load(); got != before+1 {
		t.Fatalf("成功计数 = %d，期望 %d", got, before+1)
	}
}
