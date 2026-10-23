package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// fakeOperLogRepo 只关心 Insert：其余方法在审计中间件里不被调用。
type fakeOperLogRepo struct {
	inserted []*entity.OperLog
	err      error
}

var _ repository.IOperLogRepository = (*fakeOperLogRepo)(nil)

func (f *fakeOperLogRepo) List(context.Context, string, string, string, int, int) ([]*model.OperLogListRes, int64, error) {
	return nil, 0, nil
}
func (f *fakeOperLogRepo) Get(context.Context, string) (*model.OperLogInfoRes, error) {
	return nil, nil
}
func (f *fakeOperLogRepo) Insert(_ context.Context, log *entity.OperLog) error {
	f.inserted = append(f.inserted, log)
	return f.err
}
func (f *fakeOperLogRepo) Delete(context.Context, []string) error { return nil }
func (f *fakeOperLogRepo) Clear(context.Context) error            { return nil }

// auditRouter 复刻真实挂载顺序：OperLog 在 v1 根分组，认证中间件在其后，
// 因此入口处 context 里没有用户——这正是最容易写错的地方。
func auditRouter(repo repository.IOperLogRepository, withUser bool, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo))
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
	if got.Title != "system/user/edit" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.OperUrl != "/api/v1/system/user/edit" {
		t.Errorf("OperUrl = %q", got.OperUrl)
	}
	if got.RequestMethod != http.MethodPost || got.BusinessType != businessTypeAdd {
		t.Errorf("method/businessType = %s/%d", got.RequestMethod, got.BusinessType)
	}
	if got.OperName != "alice" {
		t.Errorf("OperName = %q", got.OperName)
	}
	if len(got.OperId) != 32 {
		t.Errorf("OperId 长度 = %d，期望 32", len(got.OperId))
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
	v1.Use(OperLog(repo), func(c *gin.Context) {
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
