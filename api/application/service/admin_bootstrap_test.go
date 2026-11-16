package service

import (
	"context"
	"strings"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/auth"
)

// F-002 引导纯函数表测。
func TestGenerateAdminPassword(t *testing.T) {
	p1, err := generateAdminPassword()
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := generateAdminPassword()
	if len(p1) != 32 || len(p2) != 32 {
		t.Fatalf("口令长度应 32: %d/%d", len(p1), len(p2))
	}
	if p1 == p2 {
		t.Fatal("两次生成不应相同")
	}
	for _, c := range p1 {
		if strings.ContainsRune("'\"\\`$", c) {
			t.Fatalf("口令含易注入字符: %q", c)
		}
	}
	// 口令必须能被校验链路接受（满足最严档的字符类别）
	if !auth.VerifyPassword(p1, func() string {
		h, _ := auth.HashPassword(p1)
		return h
	}()) {
		t.Fatal("生成的口令哈希往返校验失败")
	}
}

func TestNeedsBootstrapAdmin(t *testing.T) {
	if !needsBootstrapAdmin(0) {
		t.Fatal("零超管应引导")
	}
	for _, n := range []int{1, 2, 100} {
		if needsBootstrapAdmin(n) {
			t.Fatalf("%d 个超管不应引导", n)
		}
	}
	if needsBootstrapAdmin(-1) {
		t.Fatal("负数计数（查询异常路径的防御格）不应引导")
	}
}

func TestShouldSkipSeedScan(t *testing.T) {
	if !shouldSkipSeedScan(true) {
		t.Fatal("demo 声明应跳过")
	}
	if shouldSkipSeedScan(false) {
		t.Fatal("非 demo 不应跳过")
	}
}

func TestIsDemoAccountName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"hi", true},
		{"user001", true},
		{"user100", true},
		{"user0001", false},
		{"user01", false},
		{"userxyz", false},
		{"alice", false},
		{"admin", false},
		{"user001 ", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isDemoAccountName(c.name); got != c.want {
			t.Errorf("isDemoAccountName(%q)=%v, 期望 %v", c.name, got, c.want)
		}
	}
}

// seedPasswordRisk：默认口令仍可登录的演示账号必须被点名；非演示账号/已改密账号不报。
func TestSeedPasswordRisk(t *testing.T) {
	defaultHash, _ := auth.HashPassword("aiarsenal123")
	otherHash, _ := auth.HashPassword("S3cure!Passphrase-2026")
	users := []entity.User{
		{UserName: "user001", UserPassword: defaultHash},  // 仍默认口令 → 报
		{UserName: "hi", UserPassword: defaultHash},       // 仍默认口令 → 报
		{UserName: "user042", UserPassword: otherHash},    // 已改密 → 不报
		{UserName: "alice", UserPassword: defaultHash},    // 非演示形态 → 不报
		{UserName: "user077", UserPassword: "not-a-hash"}, // 畸形串校验失败 → 不报
	}
	got := seedPasswordRisk(users)
	if len(got) != 2 || got[0] != "user001" || got[1] != "hi" {
		t.Fatalf("应只点名 user001/hi, 得 %v", got)
	}
}

// 非演示形态用户整体不进候选（空结果短路，不触发告警）。
func TestSeedPasswordRiskNoCandidates(t *testing.T) {
	h, _ := auth.HashPassword("aiarsenal123")
	if risky := seedPasswordRisk([]entity.User{{UserName: "admin", UserPassword: h}}); len(risky) != 0 {
		t.Fatalf("非演示账号不应被点名: %v", risky)
	}
	if risky := seedPasswordRisk(nil); len(risky) != 0 {
		t.Fatalf("空集应空结果: %v", risky)
	}
}

// ---- F-002 第二轮审计缺口：扫描管线行为级测试（可注入形态）----
// 变异锚点：删 skip 决策（M4 类）→ demo 声明格红；候选过滤放宽 → 非演示格红；
// 告警内容拼装错误 → 文案格红。

func TestRunSeedScanPipeline(t *testing.T) {
	defaultHash, _ := auth.HashPassword("aiarsenal123")
	otherHash, _ := auth.HashPassword("S3cure!Passphrase-2026")
	riskyUsers := []entity.User{
		{UserName: "user001", UserPassword: defaultHash},
		{UserName: "user002", UserPassword: defaultHash},
	}
	quietUsers := []entity.User{{UserName: "user001", UserPassword: otherHash}}

	cases := []struct {
		name         string
		demoDeclared bool
		candidates   []entity.User
		fetchErr     error
		wantDispatch bool
		wantInBody   string
	}{
		{"存量部署默认口令命中→告警", false, riskyUsers, nil, true, "user001"},
		{"demo 声明→跳过不告警", true, riskyUsers, nil, false, ""},
		{"全部已改密→不告警", false, quietUsers, nil, false, ""},
		{"零候选→不告警", false, nil, nil, false, ""},
		{"查询失败→不告警不崩溃", false, nil, context.DeadlineExceeded, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatched := []string{}
			p := scanPipeline{
				fetchCandidates: func(ctx context.Context) ([]entity.User, error) {
					return tc.candidates, tc.fetchErr
				},
				dispatch: func(eventType, severity, title, body, bizKey string) {
					dispatched = append(dispatched, eventType+"|"+body)
				},
			}
			runSeedScan(context.Background(), p, tc.demoDeclared)
			if tc.wantDispatch {
				if len(dispatched) != 1 || !strings.Contains(dispatched[0], tc.wantInBody) {
					t.Fatalf("应告警且点名 %s: %v", tc.wantInBody, dispatched)
				}
				if !strings.Contains(dispatched[0], "seed_default_password") {
					t.Fatalf("事件类型错误: %v", dispatched)
				}
			} else if len(dispatched) != 0 {
				t.Fatalf("不应告警: %v", dispatched)
			}
		})
	}
}
