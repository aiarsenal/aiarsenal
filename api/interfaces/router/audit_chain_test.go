package router

import (
	"time"

	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/interfaces/middleware"
)

type fakeOperLogRepo struct {
	inserted []*entity.OperLog
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
	return nil
}
func (f *fakeOperLogRepo) Delete(context.Context, []string) error { return nil }
func (f *fakeOperLogRepo) Clear(context.Context) error            { return nil }

// PurgeBefore 满足 IOperLogRepository（留存清理接口；路由测试不触达）。
func (f *fakeOperLogRepo) PurgeBefore(context.Context, time.Time) (int64, error) { return 0, nil }

// 这条用例只做一件事：证明 v1 分组真的挂了审计中间件。
// 删掉 auditChain 里的 OperLog（或整条 v1.Use），它必须变红。
func TestAuditChainWiresOperLog(t *testing.T) {
	repo := &fakeOperLogRepo{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(auditChain(Deps{OperLogRepo: repo})...)
	// 认证在审计之后，与真实挂载顺序一致
	v1.Use(func(c *gin.Context) {
		middleware.SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice"})
	})
	v1.POST("/probe/write", func(c *gin.Context) { middleware.Success(c, nil) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/probe/write", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(repo.inserted) != 1 {
		t.Fatalf("审计中间件未生效：落库 %d 条", len(repo.inserted))
	}
	if got := repo.inserted[0].OperUrl; got != "/api/v1/probe/write" {
		t.Fatalf("记录的路由不对：%s", got)
	}
}

// 缺少 OperLogRepo 时不能 panic：Deps{} 在测试与工具里会被大量构造。
func TestAuditChainToleratesNilRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(auditChain(Deps{})...)
	v1.POST("/probe/write", func(c *gin.Context) { middleware.Success(c, nil) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/probe/write", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("响应码 = %d", w.Code)
	}
}
