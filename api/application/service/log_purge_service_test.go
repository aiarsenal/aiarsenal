package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ---- 档位闸门判定矩阵（变异锚点：删任何分支必有格红）----

func TestAuditProtectionCheck(t *testing.T) {
	cases := []struct {
		name       string
		protection string
		isAdmin    bool
		want       error // nil=放行
	}{
		{"strict+超管 也拒（等保合规态无一例外）", "strict", true, ErrAuditProtection},
		{"strict+非超管 拒", "strict", false, ErrAuditProtection},
		{"flexible+超管 放行", "flexible", true, nil},
		{"flexible+非超管 拒", "flexible", false, ErrAuditNotAdmin},
		{"未知档位+超管 按 strict 拒（配错=收紧）", "weird", true, ErrAuditProtection},
		{"未知档位+非超管 拒", "weird", false, ErrAuditProtection},
		{"空档位+超管 按 strict 拒", "", true, ErrAuditProtection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := auditProtectionCheck(tc.protection, tc.isAdmin)
			if !errors.Is(got, tc.want) {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

// ---- 留存清理服务（fake repo；边界 181/179 由真实运行矩阵覆盖，这里钉调用形状）----

type fakePurgeLoginRepo struct {
	rows    []int64 // 每行的 login_time 相对 now 的天数（负=过去）
	deleted []int64
}

func (f *fakePurgeLoginRepo) List(ctx context.Context, loginName, status, ip, location string, pageNum, pageSize int) ([]*entity.LoginLog, int64, error) {
	return nil, 0, nil
}
func (f *fakePurgeLoginRepo) Delete(ctx context.Context, infoIds []string) error { return nil }
func (f *fakePurgeLoginRepo) Clear(ctx context.Context) error                    { return nil }

// PurgeBefore 按传入 cutoff 用绝对时间判定（变异 M5 补网：cutoff 被偷改时
// 本 fake 必须跟着红——此前硬编码 d>180 与 cutoff 解耦，变异存活）。
func (f *fakePurgeLoginRepo) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	for _, d := range f.rows {
		at := baseTime().AddDate(0, 0, -int(d))
		if at.Before(cutoff) {
			f.deleted = append(f.deleted, d)
			n++
		}
	}
	return n, nil
}

type fakePurgeOperRepo struct {
	rows    []int64
	deleted []int64
	inserts []*entity.OperLog
}

func (f *fakePurgeOperRepo) List(ctx context.Context, title, method, operName string, pageNum, pageSize int) ([]*model.OperLogListRes, int64, error) {
	return nil, 0, nil
}
func (f *fakePurgeOperRepo) Get(ctx context.Context, operId string) (*model.OperLogInfoRes, error) {
	return nil, nil
}
func (f *fakePurgeOperRepo) Insert(ctx context.Context, log *entity.OperLog) error {
	f.inserts = append(f.inserts, log)
	return nil
}
func (f *fakePurgeOperRepo) Delete(ctx context.Context, operIds []string) error { return nil }
func (f *fakePurgeOperRepo) Clear(ctx context.Context) error                    { return nil }
func (f *fakePurgeOperRepo) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var n int64
	for _, d := range f.rows {
		at := baseTime().AddDate(0, 0, -int(d))
		if at.Before(cutoff) {
			f.deleted = append(f.deleted, d)
			n++
		}
	}
	return n, nil
}

func TestPurgeOnceBoundaryAndTrace(t *testing.T) {
	lg := &fakePurgeLoginRepo{rows: []int64{181, 200, 180, 90}} // 180=恰在界内不清（超过 180 天才清）
	op := &fakePurgeOperRepo{rows: []int64{365, 10}}
	svc := NewLogPurgeService(lg, op, 180)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	lr, or, err := svc.PurgeOnce(context.Background())
	if err != nil {
		t.Fatalf("PurgeOnce: %v", err)
	}
	if lr != 2 || or != 1 {
		t.Fatalf("删除行数 login=%d oper=%d want 2/1（180 天整不算超期）", lr, or)
	}
	if len(op.inserts) != 1 {
		t.Fatalf("清理动作必须写 aa_oper_log：inserts=%d", len(op.inserts))
	}
	if op.inserts[0].Title != "日志留存清理" {
		t.Fatalf("留痕 title=%q", op.inserts[0].Title)
	}
	if lg.deleted[0] != 181 || lg.deleted[1] != 200 {
		t.Fatalf("被删行不符: %v", lg.deleted)
	}
}

// baseTime 测试统一时钟基准（fake 行的绝对时间据此生成）。
func baseTime() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// resolveCaller 表测（变异 M3 补网）：查库出错/目标不存在一律非超管（fail-closed）。
func TestResolveCaller(t *testing.T) {
	admin := &entity.User{UserName: "boss", IsAdmin: 1}
	plain := &entity.User{UserName: "emp", IsAdmin: 0}
	cases := []struct {
		name      string
		caller    *entity.User
		err       error
		wantName  string
		wantAdmin bool
	}{
		{"正常超管", admin, nil, "boss", true},
		{"普通用户", plain, nil, "emp", false},
		{"查库出错 fail-closed", nil, errors.New("db down"), "", false},
		{"目标不存在 fail-closed", nil, nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, isAdmin := resolveCaller(tc.caller, tc.err)
			if name != tc.wantName || isAdmin != tc.wantAdmin {
				t.Fatalf("got (%q,%v) want (%q,%v)", name, isAdmin, tc.wantName, tc.wantAdmin)
			}
		})
	}
}
