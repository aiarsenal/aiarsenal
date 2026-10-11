package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/mcpgateway/compression"
	"aiarsenal/infrastructure/mcpgateway/registry"
	"aiarsenal/infrastructure/persistence"
	"aiarsenal/interfaces/gateway"
	"aiarsenal/logx"
)

// MCP 协议网关（api 模块第三 main，与 api 主服务 / cmd/ops 同一套数据层）：
// 面向 AI 客户端（Claude Desktop/Cursor）暴露 SSE/streamable-http 端点，长连接、
// 有状态、Bearer 鉴权、独立端口（:43213）。数据层/鉴权与 Web UI 共用
// application/service + infrastructure/persistence，只是入口不同。

func main() {
	logx.InitEnv("/var/log/aiarsenal/mcp", "mcp", slog.LevelInfo)

	if _, err := config.Load(); err != nil {
		log.Fatalf("load config: %v", err)
	}
	if _, err := db.InitMcpDB(); err != nil {
		log.Fatalf("init mcp db: %v", err)
	}

	// 仓储（mcp 库；与 Web UI 管理端同一套）
	serverRepo := persistence.NewMcpServerRepository()
	groupRepo := persistence.NewMcpGroupRepository()
	keyRepo := persistence.NewMcpBearerKeyRepository()
	activityRepo := persistence.NewMcpActivityRepository()
	promptRepo := persistence.NewMcpBuiltinPromptRepository()
	resourceRepo := persistence.NewMcpBuiltinResourceRepository()
	cfgRepo := persistence.NewMcpConfigRepository()

	// 管理服务（查询/校验逻辑，与 Web UI 同源单份）
	serverSvc := service.NewMcpServerService(serverRepo)
	groupSvc := service.NewMcpGroupService(groupRepo, serverRepo, nil, persistence.NewMcpGroupGrantRepository()) // 仅网关鉴权用，不查列表；userRepo 无需（IdentityOf 裸身份）
	keySvc := service.NewMcpKeyService(keyRepo)
	activitySvc := service.NewMcpActivityService(activityRepo)
	builtinSvc := service.NewMcpBuiltinService(promptRepo, resourceRepo)
	_ = service.NewMcpConfigService(cfgRepo, nil) // 配置加载压缩参数用

	compressSvc := compression.New()
	reg := registry.New(compressSvc)
	sseSvc := gateway.New(serverSvc, groupSvc, builtinSvc, activitySvc, compressSvc, reg, config.Get().Actor.SignKey)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// OAuth 授权端点（MCP 规范 2025-06-18 Authorization）：/oauth/register、/oauth/authorize、
	// /oauth/token、/.well-known/oauth-authorization-server。自鉴权，无需 Bearer 中间件。
	// 可用 MCP_OAUTH_ISSUER 覆盖 discovery 文档里的对外根地址（反代场景建议显式配置）。
	gateway.RegisterOAuthRoutes(r, keySvc)

	// MCP 协议端点：/mcp / /mcp/:scope（GET=SSE、POST=streamable-http、DELETE=shutdown）
	mcpEp := r.Group("/mcp", gateway.BearerAuth(keySvc, groupSvc, serverSvc))
	mcpEp.GET("", sseSvc.Endpoint)
	mcpEp.GET("/:scope", sseSvc.Endpoint)
	mcpEp.POST("", sseSvc.Endpoint)
	mcpEp.POST("/:scope", sseSvc.Endpoint)
	mcpEp.DELETE("", sseSvc.Endpoint)        // 基端点 DELETE：SDK 按会话头终止会话
	mcpEp.DELETE("/:scope", sseSvc.Shutdown) // scope 端点 DELETE：停子进程 + 失效 handler（热换）

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "mcp-gateway"})
	})

	addr := ":" + envOr("MCP_PORT", "43213")
	log.Printf("AI Arsenal MCP gateway listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
