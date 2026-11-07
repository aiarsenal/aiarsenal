package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// ---- insecureDefaults 凭据项（F-003/F-004）----
// 变异锚点：删 redis.password 项 → 空口令格红；删 database.default.link 项 →
// 默认/同源口令格红；判定条件放宽（空配置也报）→ 零值格红。

func TestInsecureDefaultsCredentialItems(t *testing.T) {
	has := func(items []string, want string) bool {
		for _, it := range items {
			if it == want {
				return true
			}
		}
		return false
	}

	// 已配置 Redis（address 非空）但未设口令 → F-003 告警项
	cfg := &Config{}
	cfg.Redis.Default.Address = "127.0.0.1:6379"
	if got := cfg.insecureDefaults(); !has(got, "redis.password") {
		t.Fatalf("已配置无口令的 Redis 应触发 redis.password 告警项，got %v", got)
	}

	// 数据库连接串为仓库默认口令 → F-004 告警项
	cfg.Database.Default.Link = "pgsql:aiarsenal:aiarsenal123@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable"
	got := cfg.insecureDefaults()
	if !has(got, "redis.password") || !has(got, "database.default.link") {
		t.Fatalf("默认口令连接串+空 Redis 口令应同时触发两项，got %v", got)
	}

	// 账号与口令同源（报告 F-004 的实锤形态）同样触发
	cfg.Database.Default.Link = "pgsql:aa:aa@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable"
	if got = cfg.insecureDefaults(); !has(got, "database.default.link") {
		t.Fatalf("账号口令同源的连接串应触发 database.default.link 告警项，got %v", got)
	}

	// 净配置（Redis 设了口令 + 数据库随机口令）→ 两项都不在
	clean := &Config{}
	clean.Redis.Default.Address = "127.0.0.1:6379"
	clean.Redis.Default.Password = "9f86d081884c7d65"
	clean.Database.Default.Link = "pgsql:aiarsenal:0f4b1e4d9c2a@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable"
	if got = clean.insecureDefaults(); has(got, "redis.password") || has(got, "database.default.link") {
		t.Fatalf("净配置不应触发凭据告警项，got %v", got)
	}

	// 零值配置（未配置 Redis / 无连接串）→ 不报（保持既有空配置零告警钉子）
	if got := (&Config{}).insecureDefaults(); has(got, "redis.password") || has(got, "database.default.link") {
		t.Fatalf("零值配置不应触发凭据告警项，got %v", got)
	}
}

func TestInsecureDefaultNoticeCredentialCopy(t *testing.T) {
	for _, item := range []string{"redis.password", "database.default.link"} {
		title, body := insecureDefaultNotice(item)
		if title == "" || body == "" || title == item {
			t.Fatalf("%s 的告警文案缺失（title=%q body 长度 %d）", item, title, len(body))
		}
	}
	if _, body := insecureDefaultNotice("redis.password"); !strings.Contains(body, "AIARSENAL_REDIS_PASSWORD") {
		t.Fatalf("redis.password 文案应给出环境变量处置入口")
	}
	if _, body := insecureDefaultNotice("database.default.link"); !strings.Contains(body, "AIARSENAL_DATABASE_DEFAULT_LINK") {
		t.Fatalf("database.default.link 文案应给出环境变量处置入口")
	}
}

// ---- Load 凭据环境变量覆盖接线（F-003/F-004）----
// 变异锚点：删任一 v.Set 分支 → 对应格红。

func TestApplyCredentialEnvOverrides(t *testing.T) {
	t.Setenv("AIARSENAL_DATABASE_DEFAULT_LINK", "pgsql:aiarsenal:rand0m-p4ss@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable")
	t.Setenv("AIARSENAL_REDIS_PASSWORD", "r3dis-s3cret")
	v := viper.New()
	v.Set("database.default.link", "pgsql:aiarsenal:aiarsenal123@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable")
	v.Set("redis.default.password", "")
	applyCredentialEnvOverrides(v)
	if got := v.GetString("database.default.link"); !strings.Contains(got, "rand0m-p4ss") {
		t.Fatalf("数据库连接串未被环境变量覆盖: %q", got)
	}
	if got := v.GetString("redis.default.password"); got != "r3dis-s3cret" {
		t.Fatalf("Redis 口令未被环境变量覆盖: %q", got)
	}

	// 无环境变量时保持 yaml 原值
	os.Unsetenv("AIARSENAL_DATABASE_DEFAULT_LINK")
	os.Unsetenv("AIARSENAL_REDIS_PASSWORD")
	v2 := viper.New()
	v2.Set("database.default.link", "pgsql:aiarsenal:aiarsenal123@tcp(127.0.0.1:5432)/aiarsenal?sslmode=disable")
	applyCredentialEnvOverrides(v2)
	if got := v2.GetString("database.default.link"); !strings.Contains(got, "aiarsenal123") {
		t.Fatalf("无环境变量时应保持 yaml 原值: %q", got)
	}
}
