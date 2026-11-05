//go:build integration

package middleware

// 真实 PostgreSQL 上的写入验证。这些结论无法用 fake repo 证明：
// 列宽超限（22001）、非法 UTF-8（22021）都是 PG 侧的行为，只有真库能复现。
//
// 本地跑法（需要一个可写的空库）：
//   AUDIT_TEST_DSN='postgres://postgres@127.0.0.1:55432/aiarsenal?sslmode=disable' \
//     go test -tags=integration ./interfaces/middleware/ -run TestIntegration -v
//
// CI：.github/workflows/ci.yml 的 audit-integration job 起 postgres:16 service 跑它。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/idgen"
)

// pgOperLogRepo 与 persistence.OperLogRepository.Insert 同语义：
// 业务 id 由仓储生成，然后整结构体 Create。差别只在连接怎么注入。
type pgOperLogRepo struct{ db *gorm.DB }

func (r *pgOperLogRepo) List(context.Context, string, string, string, int, int) ([]*model.OperLogListRes, int64, error) {
	return nil, 0, nil
}
func (r *pgOperLogRepo) Get(context.Context, string) (*model.OperLogInfoRes, error) { return nil, nil }
func (r *pgOperLogRepo) Insert(ctx context.Context, log *entity.OperLog) error {
	log.OperId = idgen.New()
	return r.db.WithContext(ctx).Create(log).Error
}
func (r *pgOperLogRepo) Delete(context.Context, []string) error { return nil }
func (r *pgOperLogRepo) Clear(context.Context) error            { return nil }

// PurgeBefore 与 persistence.OperLogRepository.PurgeBefore 同语义（集成测试在真库上
// 验证留存清理的 SQL 边界）。
func (r *pgOperLogRepo) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	q := r.db.WithContext(ctx).Where("oper_time < ?", cutoff).Delete(&entity.OperLog{})
	return q.RowsAffected, q.Error
}

func TestIntegrationOperLogWritesToPostgres(t *testing.T) {
	dsn := os.Getenv("AUDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 AUDIT_TEST_DSN，跳过集成测试")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接数据库失败：%v", err)
	}

	// 与 api/resource/data/aiarsenal.sql 的 aa_oper_log 逐列对齐
	err = db.Exec(`
DROP TABLE IF EXISTS aa_oper_log;
CREATE TABLE aa_oper_log (
  id             bigserial PRIMARY KEY,
  oper_id        varchar(32) NOT NULL DEFAULT '',
  title          varchar(50) DEFAULT '',
  business_type  integer DEFAULT 0,
  method         varchar(100) DEFAULT '',
  request_method varchar(10) DEFAULT '',
  operator_type  integer DEFAULT 0,
  oper_name      varchar(50) DEFAULT '',
  dept_name      varchar(50) DEFAULT '',
  oper_url       varchar(500) DEFAULT '',
  oper_ip        varchar(50) DEFAULT '',
  oper_location  varchar(255) DEFAULT '',
  oper_param     text,
  error_msg      varchar(2000) DEFAULT '',
  oper_time      timestamp without time zone
);
CREATE UNIQUE INDEX ON aa_oper_log (oper_id);`).Error
	if err != nil {
		t.Fatalf("建表失败：%v", err)
	}

	repo := &pgOperLogRepo{db: db}
	meta := NewAuditMeta(
		&fakeMenuSource{menus: []*model.MenuInfoRes{{Name: "api/v1/probe/write", Title: "用户管理"}}},
		&fakeDeptSource{list: []*model.DeptInfo{{DeptId: "d1", DeptName: "研发部"}}},
	)
	if err := meta.Warm(context.Background()); err != nil {
		t.Fatalf("预热失败：%v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(OperLog(repo, meta), func(c *gin.Context) {
		SetUser(c, &model.LoginUserRes{Id: "u1", UserName: "alice", DeptId: "d1"})
	})
	v1.POST("/probe/write", func(c *gin.Context) { Success(c, nil) })
	v1.POST("/probe/long-error", func(c *gin.Context) { Fail(c, strings.Repeat("错", 3000)) })
	v1.POST("/probe/panic", func(c *gin.Context) { panic("boom") })

	post := func(path, body string) {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		func() {
			defer func() { _ = recover() }() // panic 由外层 Recovery 处理，这里只防测试进程退出
			r.ServeHTTP(w, req)
		}()
	}
	// 6KB 中文 + 明文密码：验证截断与掩码在真库上成立
	post("/api/v1/probe/write",
		`{"userName":"张三","remark":"`+strings.Repeat("这是一段很长的中文备注", 200)+`","password":"s3cret"}`)
	post("/api/v1/probe/long-error", `{}`)
	post("/api/v1/probe/panic", `{}`)

	time.Sleep(200 * time.Millisecond)

	var rows []*entity.OperLog
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("回查失败：%v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("落库 %d 条，期望 3 条", len(rows))
	}
	if rows[0].Title != "用户管理" || rows[0].DeptName != "研发部" {
		t.Errorf("展示字段未回填：title=%q dept=%q", rows[0].Title, rows[0].DeptName)
	}
	if strings.Contains(rows[0].OperParam, "s3cret") {
		t.Errorf("明文密码进了库：%s", rows[0].OperParam)
	}
	if n := len([]rune(rows[1].ErrorMsg)); n != 2000 {
		t.Errorf("超长错误未裁到列宽：%d 字符", n)
	}
	if !strings.Contains(rows[2].ErrorMsg, "panic: boom") {
		t.Errorf("panic 未记录：%q", rows[2].ErrorMsg)
	}

	// 反证：没有这两道护栏时，PG 会整条拒收
	long := &entity.OperLog{OperId: idgen.New(), ErrorMsg: strings.Repeat("错", 3000), OperTime: ptrTypesTime()}
	if err := db.Create(long).Error; err == nil || !strings.Contains(err.Error(), "22001") {
		t.Errorf("超长 error_msg 应被 PG 拒绝（22001），实际：%v", err)
	}
	bad := &entity.OperLog{OperId: idgen.New(), OperParam: string([]byte{0x41, 0xe6, 0x87}), OperTime: ptrTypesTime()}
	if err := db.Create(bad).Error; err == nil || !strings.Contains(err.Error(), "22021") {
		t.Errorf("非法 UTF-8 应被 PG 拒绝（22021），实际：%v", err)
	}
}

func ptrTypesTime() *types.Time { t := types.Time(time.Now()); return &t }
