package config

import (
	"fmt"
	"log/slog"
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
	Password PasswordConfig `mapstructure:"password"`
	Totp     TotpConfig     `mapstructure:"totp"`

	// jwtKeyFromFile 标记 jwt.encryptKey 是否直接取自 config.yaml（未被环境变量覆盖）。
	// 不导出、不参与反序列化，仅由 Load 填入，供 insecureDefaults 上报。
	jwtKeyFromFile bool
	// pepperFromFile 标记 password.hash.pepper 是否直接取自 config.yaml（真值提交进
	// git 等于把口令密钥公开）。同上，仅供 insecureDefaults 上报。
	pepperFromFile bool
}

// PasswordConfig 口令加固配置（部署时定档，无管理界面）。三段在 Load 时经
// applyPasswordDefaults 归一，归一后字段即生效值，消费方直读无需再兜底。
type PasswordConfig struct {
	Hash    PasswordHashConfig    `mapstructure:"hash"`
	Policy  PasswordPolicyConfig  `mapstructure:"policy"`
	Lockout PasswordLockoutConfig `mapstructure:"lockout"`
}

type PasswordHashConfig struct {
	// Algorithm 哈希档位：pbkdf2-sm3（默认，国密 SM3）| pbkdf2-sha512；
	// 与口令存储串的算法 id 同名（见 infrastructure/auth/password.go）。
	// 换档只影响之后新产生的口令，存量串按串内参数校验。
	Algorithm string `mapstructure:"algorithm"`
	// Iterations PBKDF2 迭代次数；<=0 取算法默认（defaultHashIterations）。
	// 定档原则：目标部署机上单次派生 50~100ms（README 记录实测值）。
	Iterations int `mapstructure:"iterations"`
	// Pepper 系统侧口令密钥（HMAC 混合进派生前的口令）：非空时新口令落
	// *-peppered 档，拖库者缺这把密钥就无法离线枚举。生产经环境变量
	// AIARSENAL_PWD_PEPPER 注入，≥32 字节随机值，多实例必须同值；
	// 移除/换值会使全部 peppered 口令不可校验（等同强制全员重置）。
	Pepper string `mapstructure:"pepper"`
}

type PasswordPolicyConfig struct {
	// Level 强度档位：loose | standard（默认，等保基线）| strict。
	Level string `mapstructure:"level"`
	// RotationDays 口令定期更换天数（等保 a 项「定期更换」的可选落地）：
	// 0=不启用（默认，现有部署零变化）；>0 时过期口令登录被拒（管理员重置解除），
	// 等保三级口径建议 90。
	RotationDays int `mapstructure:"rotationDays"`
}

type PasswordLockoutConfig struct {
	// MaxFailures 连续登录失败锁定阈值；<=0 → 5。
	MaxFailures int `mapstructure:"maxFailures"`
	// LockMinutes 锁定时长（分钟，TTL 到期自动解锁）；<=0 → 30。
	LockMinutes int `mapstructure:"lockMinutes"`
}

// TotpConfig 双因子（TOTP 动态码）配置。
type TotpConfig struct {
	// Enforced 开启后未绑定双因子的账号登录被拒（等保三级 d 项口径）；
	// 默认 false=自愿绑定。**启用前先让全员完成绑定**（未绑定者登录后自助 setup）；
	// 启用后的恢复/入职通道是管理员重置：生成新密钥并一次性返回，下发录入后即可登录。
	Enforced bool `mapstructure:"enforced"`
	// Issuer otpauth 绑定 URI 中的发行方名（验证器 App 里展示）。
	Issuer string `mapstructure:"issuer"`
}

// defaultHashIterations 各算法默认迭代，以「单次派生 50~100ms 登录预算」定档
// （pbkdf2-sm3@60000 本机 Apple arm64 实测 ≈93ms，纯 Go 实现；部署机应按 README
// 复测后显式覆盖）。注意这是登录延迟与离线爆破成本的显式权衡：爆破者用的是自己的
// 优化实现（约快 3.5 倍），60000 迭代的每猜测成本约为 210000 的三成；如需对齐
// OWASP 迭代量级，上调到 210000（本机 ≈300ms/次）即可，调参即时生效且不影响存量
// 串校验（参数自描述）。pbkdf2-sha512 走标准库汇编，220000 对齐 OWASP 口令存储
// 速查表（2026-10 实抓）且实测 ≈60ms。
var defaultHashIterations = map[string]int{
	"pbkdf2-sm3":    60000,
	"pbkdf2-sha512": 220000,
}

// applyPasswordDefaults 在 Load 时一次性归一口令配置：非法值回落默认并 WARN，
// 之后消费方直读字段即是生效值（不打热路径、不重复告警）。
func (c *Config) applyPasswordDefaults() {
	h := &c.Password.Hash
	switch h.Algorithm {
	case "":
		h.Algorithm = "pbkdf2-sm3"
	case "pbkdf2-sm3", "pbkdf2-sha512":
	default:
		slog.Warn("password.hash.algorithm 非法，回落 pbkdf2-sm3", "given", h.Algorithm)
		h.Algorithm = "pbkdf2-sm3"
	}
	if h.Iterations <= 0 {
		h.Iterations = defaultHashIterations[h.Algorithm]
	}
	p := &c.Password.Policy
	switch p.Level {
	case "":
		p.Level = "standard"
	case "loose", "standard", "strict":
	default:
		slog.Warn("password.policy.level 非法，回落 standard", "given", p.Level)
		p.Level = "standard"
	}
	if p.RotationDays < 0 {
		p.RotationDays = 0
	}
	if c.Totp.Issuer == "" {
		c.Totp.Issuer = "AI Arsenal"
	}
	l := &c.Password.Lockout
	if l.MaxFailures <= 0 {
		l.MaxFailures = 5
	}
	if l.LockMinutes <= 0 {
		l.LockMinutes = 30
	}
	if p := c.Password.Hash.Pepper; p != "" && len(p) < 32 {
		// 不阻断：短 pepper 仍优于无 pepper，但要让运维知道该换 ≥32 字节随机值
		slog.Warn("password.hash.pepper 长度不足 32 字节，抗拖库强度打折，建议换成随机长值", "length", len(p))
	}
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
	// 覆盖前先记下 yaml 原值：生效值仍等于它，说明密钥来自仓库而未被环境变量覆盖。
	// 前提是 viper 没启用 AutomaticEnv/BindEnv（全仓无自动绑定，覆盖一律走下方 v.Set）。
	yamlJwtKey := v.GetString("jwt.encryptKey")
	// 密钥类配置支持环境变量覆盖（优先于 yaml）：仓库里的 yaml 只放开发占位值，
	// 生产经 systemd EnvironmentFile（/etc/aiarsenal/env）注入真实密钥，绝不提交进 git。
	// encryptKey 同时是 LLM 凭据的 AES 密钥——存量库换 key 前须先用旧 key 导出/重录凭据。
	if k := os.Getenv("AIARSENAL_JWT_ENCRYPT_KEY"); k != "" {
		v.Set("jwt.encryptKey", k)
	}
	if k := os.Getenv("AIARSENAL_ACTOR_SIGN_KEY"); k != "" {
		v.Set("actor.signKey", k)
	}
	yamlPepper := v.GetString("password.hash.pepper")
	// 口令 pepper 与 jwt 密钥同级敏感：仓库 yaml 只允许留空（功能关），真值走 env。
	if k := os.Getenv("AIARSENAL_PWD_PEPPER"); k != "" {
		v.Set("password.hash.pepper", k)
	}
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	c.applyPasswordDefaults()
	c.jwtKeyFromFile = jwtKeyFromConfigFile(c.Jwt.EncryptKey, yamlJwtKey)
	c.pepperFromFile = pepperFromConfigFile(c.Password.Hash.Pepper, yamlPepper)
	c.warnInsecureDefaults()
	cfg = c
	return c, nil
}

// jwtKeyFromConfigFile 判断生效的 jwt.encryptKey 是否取自随源码提交的 config.yaml。
// 判据是「来源」而不是「强度」：比起比对硬编码占位值，这里只有一份真相，yaml 换值自动
// 跟随，不会出现告警静默失效。空值也算取自该文件——它确实来自仓库里的配置。
func jwtKeyFromConfigFile(effective, fromConfigFile string) bool {
	return effective == fromConfigFile
}

// pepperFromConfigFile 同 jwtKeyFromConfigFile 的判据，但空值不算告警对象：
// pepper 默认留空（功能关）是文档化的正常态，只有非空真值进了仓库 yaml 才值得喊。
func pepperFromConfigFile(effective, fromConfigFile string) bool {
	return effective != "" && effective == fromConfigFile
}

// insecureDefaults 返回仍取自仓库默认值的敏感配置项，目前只覆盖 jwt.encryptKey，
// 新增检测项在这里追加即可。暂不导出：消费者只有本包的启动告警。
func (c *Config) insecureDefaults() []string {
	var out []string
	if c.jwtKeyFromFile {
		out = append(out, "jwt.encryptKey")
	}
	if c.pepperFromFile {
		out = append(out, "password.hash.pepper")
	}
	return out
}

// warnInsecureDefaults 在敏感配置仍取自 config.yaml 时输出 WARN 级告警。
//
// 用 WARN 不用 ERROR：进程照常启动、请求不受影响，不构成失败；ERROR 会被 oncall 分级
// 抓走，稀释真正的事故信号。不阻断启动是因为 api 侧尚无环境标识（APP_ENV 仅 llm 网关
// 读取），拒绝会让存量部署升级后起不来。各条目后果不同，逐项给文案——混成一句会让
// 运维对错对象执行处置。
func (c *Config) warnInsecureDefaults() {
	for _, item := range c.insecureDefaults() {
		switch item {
		case "jwt.encryptKey":
			slog.Warn(
				"敏感配置仍取自随源码提交的 config.yaml：若本实例对公网可达，"+
					"该密钥等同公开，存在被冒用签发 token 的风险；"+
					"更换方式见 README「密钥与凭据（部署必读）」一节"+
					"（该键同时加密库内 LLM 凭据，换值会使既有密文失效）。",
				"insecureDefault", item,
			)
		case "password.hash.pepper":
			slog.Warn(
				"口令 pepper 的真值已随源码提交的 config.yaml 公开：拖库免疫失效，"+
					"等于未启用 pepper；处置：改经环境变量 AIARSENAL_PWD_PEPPER 注入"+
					"（≥32 字节随机值，多实例同值），换值会使既有 peppered 口令失效"+
					"（等同强制全员重置）。",
				"insecureDefault", item,
			)
		}
	}
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
