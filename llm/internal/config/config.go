package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config LLM 网关配置。默认值对齐本机测试环境（api.aiarsenal.cc 同实例），
// 所有字段可用环境变量覆盖。

type Config struct {
	Env       string // test | prod（APP_ENV，默认 test）
	Domain    string // 公开域名：test/prod 均为 llm.aiarsenal.cc（ROUTER_DOMAIN 可覆盖）
	Scheme    string // 对外协议 http|https（ROUTER_SCHEME，默认 https）
	DB        DBConfig
	IAMDB     DBConfig // aiarsenal 库只读连接：预算适用范围判定要解析用户身份（aa_user/aa_rule/aa_perm_group/aa_emp_set）
	Redis     RedisConfig
	Server    ServerConfig
	SecretKey string // 与 api 的 jwt.encryptKey 一致，用于解密 provider 凭据
}

// IsProd 是否生产环境。
func (c *Config) IsProd() bool { return c.Env == "prod" }

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	MaxIdle  int
	MaxOpen  int
}

type RedisConfig struct {
	Addr     string
	DB       int
	Password string
}

type ServerConfig struct {
	Port string
}

func Load() *Config {
	envName := env("APP_ENV", "test")
	return &Config{
		Env:    envName,
		Domain: env("ROUTER_DOMAIN", defaultDomain(envName)),
		Scheme: env("ROUTER_SCHEME", "https"),
		DB: DBConfig{
			Host: env("DB_HOST", "127.0.0.1"), Port: envInt("DB_PORT", 5432),
			User: env("DB_USER", "aiarsenal"), Password: env("DB_PASSWORD", "aiarsenal123"),
			DBName: env("DB_NAME", "aa_llm"), MaxIdle: envInt("DB_MAX_IDLE", 10), MaxOpen: envInt("DB_MAX_OPEN", 10),
		},
		IAMDB: DBConfig{
			// 默认与主库同实例同账套，仅库名不同（aiarsenal 与 aa_llm 同 PG 实例，跨库软引用）
			Host: env("IAM_DB_HOST", env("DB_HOST", "127.0.0.1")), Port: envInt("IAM_DB_PORT", envInt("DB_PORT", 5432)),
			User: env("IAM_DB_USER", env("DB_USER", "aiarsenal")), Password: env("IAM_DB_PASSWORD", env("DB_PASSWORD", "aiarsenal123")),
			DBName: env("IAM_DB_NAME", "aiarsenal"), MaxIdle: envInt("IAM_DB_MAX_IDLE", 2), MaxOpen: envInt("IAM_DB_MAX_OPEN", 5),
		},
		Redis: RedisConfig{
			Addr: env("REDIS_ADDR", "127.0.0.1:6379"), DB: envInt("REDIS_DB", 2),
			Password: env("REDIS_PASSWORD", ""),
		},
		Server:    ServerConfig{Port: env("PORT", "43214")},
		SecretKey: env("SECRET_KEY", "0606c02e01b634ee079ef01fb8ac09ef"), // 开发占位值；生产经 systemd 注入 SECRET_KEY（/etc/aiarsenal/env）
	}
}

// defaultDomain 默认公开域名：测试/生产统一 llm.aiarsenal.cc（测试靠 /etc/hosts 本机解析）。
func defaultDomain(envName string) string {
	return "llm.aiarsenal.cc"
}

// Origin 返回 <scheme>://<domain> 形式，供 CORS 与 URL 生成用（scheme 由 ROUTER_SCHEME 控制）。
func (c *Config) Origin() string {
	return c.Scheme + "://" + c.Domain
}

// DSN Postgres gorm DSN。
func (d DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		d.Host, d.Port, d.User, d.Password, d.DBName)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
