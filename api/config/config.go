package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置（对应 config.yaml 各段）
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Public   PublicConfig   `mapstructure:"public"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Jwt      JwtConfig      `mapstructure:"jwt"`
	Casbin   CasbinConfig   `mapstructure:"casbin"`
	System   SystemConfig   `mapstructure:"system"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Actor    ActorConfig    `mapstructure:"actor"`
	Agent    AgentConfig    `mapstructure:"agent"`
}

// AgentConfig Agent 模块运行配置（v2：实例备份/持久卷等本机落盘路径）。
type AgentConfig struct {
	// BackupDir 实例工作区备份落盘目录（<dir>/<backupId>.tar.gz）。
	// 空走默认值 AgentBackupDir()；localdocker 单机场景，CVM 运行时不落盘。
	BackupDir string `mapstructure:"backupDir"`
}

// AgentBackupDir 备份落盘目录（带默认值兜底：数据盘惯例路径，启动时 service 会 MkdirAll）。
func (c *Config) AgentBackupDir() string {
	if c != nil && c.Agent.BackupDir != "" {
		return c.Agent.BackupDir
	}
	return "/var/lib/aiarsenal/backup"
}

// ActorConfig 令牌交换（网关→ops 第二跳身份透传）验签配置；
// signKey 与 mcp/config/config.yaml 的 actor.signKey 必须同值（HMAC-SHA256）。
type ActorConfig struct {
	SignKey string `mapstructure:"signKey"`
}

// PublicConfig 对外公开访问配置（开源部署按实际域名/协议修改，或用环境变量覆盖）。
// scheme + domain 决定 api/mcp/llm 子域与对外协议：测试 http / 生产 https。
type PublicConfig struct {
	Scheme string `mapstructure:"scheme"` // http | https
	Domain string `mapstructure:"domain"` // 对外根域名，如 aiarsenal.cc；api./mcp./llm. 子域据此推导
}

type ServerConfig struct {
	Address string `mapstructure:"address"`
}

type DatabaseConfig struct {
	Default DatabaseDefault `mapstructure:"default"`
}

type DatabaseDefault struct {
	Link        string `mapstructure:"link"`
	MaxIdle     int    `mapstructure:"maxIdle"`
	MaxOpen     int    `mapstructure:"maxOpen"`
	MaxLifetime string `mapstructure:"maxLifetime"`
}

type RedisConfig struct {
	Default RedisDefault `mapstructure:"default"`
}

type RedisDefault struct {
	Address  string `mapstructure:"address"`
	DB       int    `mapstructure:"db"`
	Password string `mapstructure:"password"`
}

type JwtConfig struct {
	CacheKey     string   `mapstructure:"cacheKey"`
	TimeOut      int64    `mapstructure:"timeOut"`
	MaxRefresh   int64    `mapstructure:"maxRefresh"`
	MultiLogin   bool     `mapstructure:"multiLogin"`
	EncryptKey   string   `mapstructure:"encryptKey"`
	CacheModel   string   `mapstructure:"cacheModel"`
	ExcludePaths []string `mapstructure:"excludePaths"`
}

type CasbinConfig struct {
	ModelFile  string `mapstructure:"modelFile"`
	PolicyFile string `mapstructure:"policyFile"`
	Cluster    bool   `mapstructure:"cluster"`
}

type SystemConfig struct {
	DataDir string `mapstructure:"dataDir"`
}

// StorageConfig 对象存储配置（对应 runtime-go storage 包）。
// type=local 走本机磁盘；type=s3 走 S3 兼容（阿里云 OSS / 腾讯云 COS / MinIO / AWS S3）。
type StorageConfig struct {
	Type           string `mapstructure:"type"`      // local | s3
	LocalRoot      string `mapstructure:"localRoot"` // local 模式根；空→${system.dataDir}/skill（兼容存量）
	Endpoint       string `mapstructure:"endpoint"`  // s3 端点：OSS oss-cn-beijing.aliyuncs.com / COS cos.ap-beijing.myqcloud.com / MinIO host:9000
	Region         string `mapstructure:"region"`
	AccessKey      string `mapstructure:"accessKey"`
	SecretKey      string `mapstructure:"secretKey"`
	Bucket         string `mapstructure:"bucket"`
	BasePath       string `mapstructure:"basePath"`       // key 前缀；留空（service 层 key 已含 skill/ 前缀）
	UseSSL         bool   `mapstructure:"useSSL"`         // s3 是否走 https
	ForcePathStyle bool   `mapstructure:"forcePathStyle"` // OSS/COS/MinIO 建议 true；AWS S3 一般 false
}

var cfg *Config

// Load 从 config/config.yaml 加载配置
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath("./config")
	if pwd, err := os.Getwd(); err == nil {
		v.AddConfigPath(pwd + "/config")
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	// 密钥类配置支持环境变量覆盖（优先于 yaml）：仓库里的 yaml 只放开发占位值，
	// 生产经 systemd EnvironmentFile（/etc/aiarsenal/env）注入真实密钥，绝不提交进 git。
	// encryptKey 同时是 LLM 凭据的 AES 密钥——存量库换 key 前须先用旧 key 导出/重录凭据。
	if k := os.Getenv("AIARSENAL_JWT_ENCRYPT_KEY"); k != "" {
		v.Set("jwt.encryptKey", k)
	}
	if k := os.Getenv("AIARSENAL_ACTOR_SIGN_KEY"); k != "" {
		v.Set("actor.signKey", k)
	}
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg = c
	return c, nil
}

// Get 已加载的全局配置
func Get() *Config {
	if cfg == nil {
		c, err := Load()
		if err != nil {
			panic(err)
		}
		cfg = c
	}
	return cfg
}

// PublicScheme 对外协议：env(AIARSENAL_PUBLIC_SCHEME) > 配置 public.scheme > 默认 https。
func (c *Config) PublicScheme() string {
	if s := os.Getenv("AIARSENAL_PUBLIC_SCHEME"); s != "" {
		return s
	}
	if c.Public.Scheme != "" {
		return c.Public.Scheme
	}
	return "https"
}

// PublicDomain 对外根域名：env(AIARSENAL_PUBLIC_DOMAIN) > 配置 public.domain。
func (c *Config) PublicDomain() string {
	if d := os.Getenv("AIARSENAL_PUBLIC_DOMAIN"); d != "" {
		return d
	}
	return c.Public.Domain
}

// AgentBaseURL 容器内 agent 回连 api 的公共地址：{scheme}://api.{domain}。
// env(AIARSENAL_AGENT_BASE_URL) 可整体覆盖（需含 scheme）。
func (c *Config) AgentBaseURL() string {
	if u := os.Getenv("AIARSENAL_AGENT_BASE_URL"); u != "" {
		return u
	}
	return c.PublicScheme() + "://api." + c.PublicDomain()
}

// McpBaseURL agent 容器/桌面端访问 MCP 网关的公共地址：{scheme}://mcp.{domain}。
// env(AIARSENAL_MCP_BASE_URL) 可整体覆盖（需含 scheme）。本机部署 hosts 指本地，与 api 域名同机制。
func (c *Config) McpBaseURL() string {
	if u := os.Getenv("AIARSENAL_MCP_BASE_URL"); u != "" {
		return u
	}
	return c.PublicScheme() + "://mcp." + c.PublicDomain()
}

// HostPublicIP 桌面实例 access_url 的宿主地址：env(AIARSENAL_HOST_IP) > public.domain。
func (c *Config) HostPublicIP() string {
	if ip := os.Getenv("AIARSENAL_HOST_IP"); ip != "" {
		return ip
	}
	return c.PublicDomain()
}

// DesktopProxyBaseURL 「打开桌面」反代 iframe 的公共来源：{scheme}://{domain}（与 UI 同源）。
// env(AIARSENAL_DESKTOP_PROXY_BASE_URL) 可整体覆盖。
func (c *Config) DesktopProxyBaseURL() string {
	if u := os.Getenv("AIARSENAL_DESKTOP_PROXY_BASE_URL"); u != "" {
		return u
	}
	return c.PublicScheme() + "://" + c.PublicDomain()
}

// parseLink 解析 pgsql link（pgsql:user:pass@tcp(host:port)/db?params）
// 返回 gorm postgres DSN 各组成部分。
func (d DatabaseDefault) parseLink() (host, port, user, pass, dbname, params string) {
	link := d.Link
	link = strings.TrimPrefix(link, "pgsql:")
	link = strings.TrimPrefix(link, "postgres:")
	link = strings.TrimPrefix(link, "pg:")
	// user:pass@tcp(host:port)/dbname?params
	at := strings.Index(link, "@")
	userPass := link[:at]
	rest := link[at+1:]
	hostPort := rest
	if i := strings.Index(rest, "/"); i >= 0 {
		hostPort = rest[:i]
		rest = rest[i+1:]
		if q := strings.Index(rest, "?"); q >= 0 {
			dbname = rest[:q]
			params = rest[q+1:]
		} else {
			dbname = rest
		}
	}
	user = userPass
	if i := strings.Index(userPass, ":"); i >= 0 {
		user = userPass[:i]
		pass = userPass[i+1:]
	}
	hostPort = strings.TrimPrefix(hostPort, "tcp(")
	hostPort = strings.TrimSuffix(hostPort, ")")
	host = hostPort
	port = "5432"
	if i := strings.Index(hostPort, ":"); i >= 0 {
		host = hostPort[:i]
		port = hostPort[i+1:]
	}
	if params == "" {
		params = "sslmode=disable"
	}
	return
}

// GormDSN 把 pgsql link 转成 gorm postgres DSN（默认库 aiarsenal）
func (d DatabaseDefault) GormDSN() string {
	host, port, user, pass, dbname, params := d.parseLink()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s %s",
		host, port, user, pass, dbname, params, d.tzParam(params))
}

// GormDSNFor 同 GormDSN 但指向指定库名（用于 aa_skill 等同实例异库）
func (d DatabaseDefault) GormDSNFor(dbname string) string {
	host, port, user, pass, _, params := d.parseLink()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s %s",
		host, port, user, pass, dbname, params, d.tzParam(params))
}

// MigrateDSNFor 返回 golang-migrate（lib/pq 驱动）用的 DSN：与 GormDSNFor 同构但
// 不带 TimeZone 参数——lib/pq 不认 TimeZone 关键字。迁移只跑 DDL、不读写 timestamp，
// 时区无影响。host/port/user/pass 复用 default.link，仅换库名。
func (d DatabaseDefault) MigrateDSNFor(dbname string) string {
	host, port, user, pass, _, params := d.parseLink()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s",
		host, port, user, pass, dbname, params)
}

// tzParam 返回 TimeZone=Asia/Shanghai 参数（若 params 已含时区则不重复加）。
// pgx 读 timestamp without time zone 默认按 UTC，与 Go now()(CST) 写入错位 8 小时；
// 设会话 TimeZone 后读写一致，根治时区错位。
func (d DatabaseDefault) tzParam(params string) string {
	if strings.Contains(params, "TimeZone=") || strings.Contains(params, "time_zone=") || strings.Contains(params, "timezone=") {
		return ""
	}
	return "TimeZone=Asia/Shanghai"
}
