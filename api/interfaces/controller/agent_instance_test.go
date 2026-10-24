package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/interfaces/middleware"
)

// fakeInstanceRepo 只实现归属校验用例会走到的方法，其余走嵌入的 nil 接口。
// 嵌入 nil 接口的方法一旦被调用会 panic——归属闸门若被绕过，用例当场炸出来，
// 而不是让"顺手多做了一步"混过去。
type fakeInstanceRepo struct {
	repository.IAgentInstanceRepository
	instances map[string]*entity.AgentInstance
	deleted   [][]string
}

func (f *fakeInstanceRepo) Get(ctx context.Context, id string) (*entity.AgentInstance, error) {
	if ins, ok := f.instances[id]; ok {
		return ins, nil
	}
	return nil, errors.New("record not found")
}

func (f *fakeInstanceRepo) Delete(ctx context.Context, ids []string) error {
	f.deleted = append(f.deleted, ids)
	return nil
}

// newInstanceController 只接实例仓储：归属闸门必须在触达任何其他依赖之前拦截，
// nil 依赖被触达即 panic，等价于"闸门失效"的断言。
func newInstanceController(repo *fakeInstanceRepo) *AgentInstanceController {
	return NewAgentInstanceController(service.NewAgentInstanceService(repo, nil, nil, nil, nil, nil, nil))
}

// callWithUser 构造带登录态的 gin 测试上下文。
func callWithUser(t *testing.T, method, target, userId string, params gin.Params, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body == "" {
		c.Request = httptest.NewRequest(method, target, nil)
	} else {
		c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	c.Params = params
	middleware.SetUser(c, &model.LoginUserRes{Id: userId})
	return c, w
}

func respBody(t *testing.T, w *httptest.ResponseRecorder) middleware.Body {
	t.Helper()
	var b middleware.Body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("响应不是约定的 {code,data,message} 结构: %v (%s)", err, w.Body.String())
	}
	return b
}

// TestInstanceEndpointsRejectNonOwner F-04/P0-04 的回归用例（手册程序3）：
// 实例级接口必须先过归属校验再触达服务层。用户 A 调用用户 B 的实例应当失败。
func TestInstanceEndpointsRejectNonOwner(t *testing.T) {
	repo := &fakeInstanceRepo{instances: map[string]*entity.AgentInstance{
		"inst-1": {InstanceId: "inst-1", UserId: "user-b"},
	}}
	h := newInstanceController(repo)

	handlers := []struct {
		name   string
		method string
		call   func(c *gin.Context)
	}{
		{"Start", http.MethodPost, h.Start},
		{"Stop", http.MethodPost, h.Stop},
		{"Restart", http.MethodPost, h.Restart},
		{"Status", http.MethodGet, h.Status},
		{"Exec", http.MethodPost, h.Exec},
		{"Logs", http.MethodGet, h.Logs},
		{"RefreshLLMKey", http.MethodPost, h.RefreshLLMKey},
	}
	for _, tc := range handlers {
		t.Run(tc.name, func(t *testing.T) {
			c, w := callWithUser(t, tc.method, "/api/v1/agent/instance/inst-1/x", "user-a",
				gin.Params{{Key: "id", Value: "inst-1"}}, "")
			tc.call(c)
			b := respBody(t, w)
			if b.Code == 0 {
				t.Fatal("非归属人请求应当被拒，实际返回成功")
			}
			if b.Message != "无权访问该实例" {
				t.Fatalf("应因归属被拒，实际 message=%q", b.Message)
			}
		})
	}
}

// TestInstanceGetAllowsSuperadmin 超管代管全量实例：非归属人但 is_admin=1 放行
// （判定口径与 CasbinAuth 的超管短路一致）。
func TestInstanceGetAllowsSuperadmin(t *testing.T) {
	repo := &fakeInstanceRepo{instances: map[string]*entity.AgentInstance{
		"inst-1": {InstanceId: "inst-1", UserId: "user-b"},
	}}
	h := newInstanceController(repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/instance/get?instanceId=inst-1", nil)
	middleware.SetUser(c, &model.LoginUserRes{Id: "user-a", IsAdmin: 1})
	h.Get(c)

	if b := respBody(t, w); b.Code != 0 {
		t.Fatalf("超管读取他人实例被误拦: %s", w.Body.String())
	}
}

// TestInstanceGetRejectsNonOwner 实例详情也是实例级数据，归属外读取同样拒绝。
func TestInstanceGetRejectsNonOwner(t *testing.T) {
	repo := &fakeInstanceRepo{instances: map[string]*entity.AgentInstance{
		"inst-1": {InstanceId: "inst-1", UserId: "user-b"},
	}}
	h := newInstanceController(repo)

	c, w := callWithUser(t, http.MethodGet, "/api/v1/agent/instance/get?instanceId=inst-1", "user-a", nil, "")
	h.Get(c)

	if b := respBody(t, w); b.Code == 0 {
		t.Fatal("非归属人读取实例详情应当被拒，实际返回成功")
	}
}

// TestInstanceGetAllowsOwner 归属人自己的实例不受影响——防的是"改成一律拒绝"的过度修复。
func TestInstanceGetAllowsOwner(t *testing.T) {
	repo := &fakeInstanceRepo{instances: map[string]*entity.AgentInstance{
		"inst-1": {InstanceId: "inst-1", UserId: "user-b"},
	}}
	h := newInstanceController(repo)

	c, w := callWithUser(t, http.MethodGet, "/api/v1/agent/instance/get?instanceId=inst-1", "user-b", nil, "")
	h.Get(c)

	if b := respBody(t, w); b.Code != 0 {
		t.Fatalf("归属人读取实例详情被误拦: %s", w.Body.String())
	}
}

// TestInstanceDeleteRejectsForeignId 批量删除混入他人实例时整批拒绝：
// 要么全部可删，要么都不删——部分成功会让"删了谁的实例"无从对账。
func TestInstanceDeleteRejectsForeignId(t *testing.T) {
	repo := &fakeInstanceRepo{instances: map[string]*entity.AgentInstance{
		"inst-1": {InstanceId: "inst-1", UserId: "user-a"},
		"inst-2": {InstanceId: "inst-2", UserId: "user-b"},
	}}
	h := newInstanceController(repo)

	c, w := callWithUser(t, http.MethodDelete, "/api/v1/agent/instance/delete", "user-a", nil,
		`{"ids":["inst-1","inst-2"]}`)
	h.Delete(c)

	if b := respBody(t, w); b.Code == 0 {
		t.Fatal("批量删除混入他人实例应当被拒")
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("拒绝路径不应触发仓储删除，实际 %v", repo.deleted)
	}
}
