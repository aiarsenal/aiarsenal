package main

import (
	"context"
	"log"
	"log/slog"

	"github.com/gin-gonic/gin"

	"aiarsenal/llm/internal/auth"
	"aiarsenal/llm/internal/config"
	"aiarsenal/llm/internal/db"
	"aiarsenal/llm/internal/health"
	"aiarsenal/llm/internal/middleware"
	"aiarsenal/llm/internal/provider"
	"aiarsenal/llm/internal/proxy"
	"aiarsenal/llm/internal/routing"
	"aiarsenal/llm/internal/store"
	"aiarsenal/llm/logx"
)

func main() {
	logx.InitEnv("/var/log/aiarsenal/llm", "llm", slog.LevelInfo)

	cfg := config.Load()

	// F-004 运行时可见性（llm 侧）：api 的 insecureDefaults 告警覆盖不到独立配置的
	// llm 网关——口令仍是仓库公开默认值或与账号同源时在此 WARN。只告警不阻断：
	// 与 api 侧同口径（进程照常起，信号进日志由部署侧采集）。
	if cfg.DB.Password == "aiarsenal123" || (cfg.DB.User != "" && cfg.DB.Password == cfg.DB.User) {
		slog.Warn("数据库口令仍为公开默认值（或与账号同源）：拖库/接管数据库零门槛；"+
			"处置：经 DB_PASSWORD 注入随机口令（deploy.sh 自动生成）",
			"insecureDefault", "db.password")
	}
	if cfg.Redis.Password == "" {
		slog.Warn("Redis 未设访问口令：本机任意进程可读写 llm 限流/预算缓存；"+
			"处置：redis requirepass + REDIS_PASSWORD 注入（deploy.sh 自动完成）",
			"insecureDefault", "redis.password")
	}

	gdb, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	rdb, err := db.InitRedis(cfg)
	if err != nil {
		log.Fatalf("init redis: %v", err)
	}
	// aiarsenal 库只读连接（预算适用范围判定的身份解析）。失败不阻断启动：
	// 圈人预算判定届时 fail-open 放行（与共享预算缺失放行同语义）。
	if _, err := db.InitIAMDB(cfg); err != nil {
		log.Printf("init iam db: %v（预算圈人判定将放行）", err)
	}

	st := store.New(gdb, cfg)
	reg := provider.NewRegistry()
	reg.Register(provider.OpenAIProvider{})
	reg.Register(provider.AnthropicProvider{})
	reg.Register(provider.GeminiProvider{})
	strat := routing.NewWeighted(rdb)
	h := proxy.New(st, reg, strat)

	// 后台部署健康探测：结果落 aa_llm_health_check 并联动路由（连续失败跳过、恢复解除）。
	go health.New(st, reg, strat).Run(context.Background())

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// F-022：默认不信任任何代理——用量/计费记录的 ClientIP 取 TCP 对端；
	// 挂反代的部署经 TRUSTED_PROXIES（逗号分隔）显式声明可信网段。
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("set trusted proxies: %v", err)
	}
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.Origin()))

	v1 := r.Group("/v1", auth.AuthMiddleware(st, rdb))
	v1.POST("/chat/completions", h.ChatCompletions)
	v1.POST("/messages", h.Messages)
	v1.POST("/embeddings", h.Embeddings)
	v1.GET("/models", h.Models)

	// Gemini 原生协议入口（v1beta）：Gemini 协议客户端 → 网关内翻成 OpenAI 跑 openai 兼容族上游，
	// 响应/SSE 翻回 Gemini 格式（provider/gemini_client.go）。鉴权与 /v1 同一套虚拟 Key 中间件，
	// 另兼容 Gemini 客户端习惯的 x-goog-api-key 头与 ?key= 查询参数（见 auth.extractBearer）。
	// path 形如 /v1beta/models/{model}:generateContent / :streamGenerateContent(?alt=sse)，
	// 末段整段捕获后按 ":" 拆分（proxy/gemini.go）。
	v1beta := r.Group("/v1beta", auth.AuthMiddleware(st, rdb))
	v1beta.POST("/models/:modelAction", h.GeminiGenerateContent)

	// 健康检查：暴露 env / domain，便于确认当前环境
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "env": cfg.Env, "domain": cfg.Domain, "origin": cfg.Origin()})
	})

	addr := ":" + cfg.Server.Port
	log.Printf("AI Arsenal LLM router listening on %s (env=%s domain=%s)", addr, cfg.Env, cfg.Domain)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}
