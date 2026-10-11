// mcp-ops 入口（api module 第二 main）：平台内部能力 MCP server（真实数据），
// 业务逻辑与 Web UI 同一套（api 的 service 层），只是入口不同——常驻 http :43220，
// 经网关（mcp/，:43213）分组聚合 + Bearer 鉴权后供 agent 调用（自身不做鉴权）。
// 端点：/mcp 平台数据只读、/mcp/doc(-write) 文档读写、/mcp/kb(-write) 知识库读写、
// /mcp/skill(-write) 技能读写、/mcp/project(-write) 项目读写、/mcp/notice(-write) 通知读写
// （读写分端点，网关侧分组绑哪个 server 即授哪个权）、/mcp/user 员工信息查询（只读）。
// 网关注册：platform-ops / platform-{doc,kb,skill,project,notice}(-write) / platform-user。
// --transport stdio 供本地冒烟。
package main

import (
	"flag"
	"log"
	"log/slog"

	"gorm.io/gorm"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/market"
	xfyun "aiarsenal/infrastructure/market/xfyun"
	"aiarsenal/infrastructure/persistence"
	"aiarsenal/interfaces/ops"
	"aiarsenal/logx"
)

func main() {
	transport := flag.String("transport", "http", "transport: http | stdio")
	host := flag.String("host", "127.0.0.1", "http 模式监听地址")
	port := flag.Int("port", 43220, "http 模式监听端口")
	flag.Parse()

	// http 模式日志进 /var/log/aiarsenal/mcp（stdout + 按日文件）；stdio 模式 stdout 是
	// 协议通道，保持默认输出（stderr）不动，避免污染协议流
	if *transport == "http" {
		logx.InitEnv("/var/log/aiarsenal/mcp", "mcp-ops", slog.LevelInfo)
	}

	// 配置 + 本入口用到的八个库（与 api 主入口同一套 db 装配，DSN 同源）
	if _, err := config.Load(); err != nil {
		log.Fatalf("load config: %v", err)
	}
	for _, initDB := range []func() (*gorm.DB, error){
		db.InitDB, // 主库（aiarsenal）：通知 aa_notice + actor 身份解析 aa_user
		db.InitAgentDB, db.InitClientDB, db.InitSkillDB, db.InitMcpDB, db.InitDocDB, db.InitKbDB,
		db.InitProjectDB, // 项目域（aa_project）
		db.InitScheduleDB, // 日程域（aa_schedule：事件/参会人/会议室）
		db.InitLLMDB,     // 预算域（aa_llm：我的预算 + 临时预算申请）
	} {
		if _, err := initDB(); err != nil {
			log.Fatalf("init db: %v", err)
		}
	}

	// 文档域装配（与 cmd/main.go 同款：repo + 上传人回填 + 离线嵌入）
	docRepo := persistence.NewDocRepository()
	docDirRepo := persistence.NewDocDirRepository()
	docAccessRepo := persistence.NewDocAccessLogRepository()
	docGrantRepo := persistence.NewDocGrantRepository()
	userRepo := persistence.NewUserRepository()
	// 与 api 主服务同一嵌入口径：配置 llm_embedding 走网关真实语义嵌入，否则回落离线 LocalEmbedding。
	embed := service.NewEmbeddingProvider(service.LoadLLMEmbeddingConfig())
	docSvc := service.NewDocService(docRepo, docDirRepo, docAccessRepo, docGrantRepo, userRepo, embed, "data/doc")

	// 知识库域装配（与 cmd/main.go 同款；与文档域同构的独立实现，共用离线嵌入实例）
	kbRepo := persistence.NewKbRepository()
	kbDirRepo := persistence.NewKbDirRepository()
	kbAccessRepo := persistence.NewKbAccessLogRepository()
	kbGrantRepo := persistence.NewKbGrantRepository()
	kbSvc := service.NewKbService(kbRepo, kbDirRepo, kbAccessRepo, kbGrantRepo, userRepo, embed, "data/kb")

	// 通知域装配（主库 aa_notice；收件人解析 notice_user_lookup 直查主库 aa_user）
	noticeSvc := service.NewNoticeService(persistence.NewNoticeRepository())

	// 技能域装配（aa_skill 库，与 cmd/main.go 同款；市场同步/链接导入从本进程出网）
	skillNSRepo := persistence.NewSkillNamespaceRepository()
	skillRepo := persistence.NewSkillRepository()
	skillSecRepo := persistence.NewSkillSecurityRepository()
	skillStorage, err := service.NewSkillStorageFromConfig()
	if err != nil {
		log.Fatalf("init skill storage: %v", err)
	}
	skillScanner := service.NewScannerService(skillRepo, skillSecRepo, skillStorage)
	skillNSSvc := service.NewSkillNamespaceService(skillNSRepo)
	skillSvc := service.NewSkillService(skillRepo, userRepo, skillSecRepo)
	skillSecSvc := service.NewSkillSecurityService(skillSecRepo, skillScanner)
	skillPkgSvc := service.NewSkillPackageService(skillRepo, skillSecRepo, skillStorage, skillScanner, embed)
	skillReg := market.NewRegistry()
	skillReg.Register(xfyun.New())
	skillMarketSiteSvc := service.NewSkillMarketSiteService(persistence.NewSkillMarketSiteRepository())
	skillMarketSvc := service.NewSkillMarketService(skillPkgSvc, skillReg)

	// 实例技能装卸（agent 库，与 cmd/main.go 同款）
	agentCmdRepo := persistence.NewAgentInstanceCommandRepository()
	agentStatusRepo := persistence.NewAgentInstanceRuntimeStatusRepository()
	agentStatusHistoryRepo := persistence.NewAgentInstanceRuntimeStatusHistoryRepository()
	agentConfigChangeRepo := persistence.NewAgentInstanceConfigChangeRepository()
	agentInstanceRepo := persistence.NewAgentInstanceRepository()
	agentInstanceAgentRepo := persistence.NewAgentInstanceAgentRepository()
	agentInstanceSkillRepo := persistence.NewAgentInstanceSkillRepository()
	agentRuntimeSvc := service.NewAgentRuntimeService(agentStatusRepo, agentStatusHistoryRepo,
		agentConfigChangeRepo, agentCmdRepo, agentInstanceRepo, agentInstanceAgentRepo, userRepo,
		agentInstanceSkillRepo, skillRepo, skillSecRepo)
	// 扫描结论双通道通知（站内+数字人同一消息）：scanner 构造早于 runtime/notice，装配尾期补注
	skillScanner.SetChannelNotice(service.NewChannelNoticeService(noticeSvc, agentRuntimeSvc, agentInstanceRepo))

	// 预算域装配（aa_llm 库，与 cmd/main.go 同款；通知走 noticeSvc，渠道推送复用 agentRuntimeSvc）。
	// 临时预算申请=统一审核工单（主库 aa_review_order）；ops 进程不跑历史搬运（api 进程负责，避免双进程竞争）。
	llmBudgetRepo := persistence.NewLLMBudgetRepository()
	llmDailyRepo := persistence.NewLLMDailySpendRepository()
	llmCfgSvc := service.NewLLMConfigService(persistence.NewLLMCredentialRepository(), persistence.NewLLMModelRepository(),
		llmBudgetRepo, persistence.NewLLMModelGroupRepository(), noticeSvc, llmDailyRepo)
	budgetReqSvc := service.NewReviewOrderService(persistence.NewReviewOrderRepository(), userRepo,
		persistence.NewDeptRepository(), noticeSvc, agentRuntimeSvc, agentInstanceRepo,
		service.NewLLMTempBudgetHandler(llmBudgetRepo, llmDailyRepo, persistence.NewPostRepository()))

	// 看板/监控（agent 库，与 cmd/main.go 同款；instanceRepo 复用上面的）
	agentTeamRepo := persistence.NewAgentTeamRepository()
	agentModelRepo := persistence.NewAgentModelRepository()
	agentScanJobRepo := persistence.NewAgentSecurityScanJobRepository()
	agentInvRepo := persistence.NewAgentModelInvocationRepository()
	agentEventRepo := persistence.NewAgentAuditEventRepository()
	agentCostRepo := persistence.NewAgentCostRecordRepository()
	agentHitRepo := persistence.NewAgentRiskHitRepository()
	agentAuditRepo := persistence.NewAgentAuditLogRepository()
	agentDashboardSvc := service.NewAgentDashboardService(agentInstanceRepo, agentTeamRepo, agentModelRepo, agentScanJobRepo)
	agentMonitorSvc := service.NewAgentMonitorService(agentInvRepo, agentEventRepo, agentCostRepo, agentHitRepo, agentAuditRepo)

	// 项目域装配（aa_project 库，与 cmd/main.go 同款）
	projectRepo := persistence.NewProjectRepository()
	projectMemberRepo := persistence.NewProjectMemberRepository()
	projectTaskRepo := persistence.NewProjectTaskRepository()
	projectCommentRepo := persistence.NewProjectTaskCommentRepository()
	projectActivityRepo := persistence.NewProjectTaskActivityRepository()
	projectMilestoneRepo := persistence.NewProjectMilestoneRepository()
	projectStatusRepo := persistence.NewProjectStatusRepository()
	projectSvc := service.NewProjectService(projectRepo, projectMemberRepo, projectStatusRepo)
	projectTaskSvc := service.NewProjectTaskService(projectTaskRepo, projectCommentRepo, projectActivityRepo, projectStatusRepo, userRepo)
	projectBoardSvc := service.NewProjectBoardService(projectTaskRepo, projectStatusRepo, projectActivityRepo, userRepo)
	projectMsSvc := service.NewProjectMilestoneService(projectMilestoneRepo, projectTaskRepo, projectStatusRepo)
	projectActSvc := service.NewProjectActivityService(projectActivityRepo)
	projectRptSvc := service.NewProjectReportService(projectTaskRepo, projectMilestoneRepo, projectMsSvc)
	projectTLSvc := service.NewProjectTimelineService(projectTaskRepo)
	projectMemberSvc := service.NewProjectMemberService(projectMemberRepo, userRepo)

	// 文档空间同步（项目空间）：MCP project_create/update/member 等工具经 ProjectService
	// 触发钩子；不跑 Bootstrap（对账只在 api 进程），竞态靠唯一索引 + 下次 api 重启自愈。
	docSpaceSvc := service.NewDocSpaceService(docDirRepo, docGrantRepo, projectRepo, projectMemberRepo, userRepo)
	projectSvc.SetDocSpaceSync(docSpaceSvc)
	projectMemberSvc.SetDocSpaceSync(docSpaceSvc)

	// 日程域装配（aa_schedule 库）：MCP schedule_* 工具的落点；
	// 邀请/变更/取消通知走双通道（站内+数字人），复用 noticeSvc + agentRuntimeSvc。
	scheduleSvc := service.NewScheduleService(
		persistence.NewScheduleEventRepository(), persistence.NewScheduleAttendeeRepository(),
		persistence.NewScheduleExceptionRepository(), persistence.NewMeetingRoomRepository(), userRepo)
	scheduleSvc.SetChannelNotice(service.NewChannelNoticeService(noticeSvc, agentRuntimeSvc, agentInstanceRepo))

	if err := ops.Run(*transport, ops.Deps{Host: *host, Port: *port, Doc: docSvc, Kb: kbSvc,
		Notice:  noticeSvc,
		Project: projectSvc, Task: projectTaskSvc, Milestone: projectMsSvc, Board: projectBoardSvc,
		Activity: projectActSvc, Report: projectRptSvc, Timeline: projectTLSvc, Member: projectMemberSvc,
		Skill: skillSvc, SkillNS: skillNSSvc, SkillSec: skillSecSvc, Market: skillMarketSvc, MarketSite: skillMarketSiteSvc,
		Runtime: agentRuntimeSvc, Embed: embed,
		Dashboard: agentDashboardSvc, Monitor: agentMonitorSvc,
		LLMCfg: llmCfgSvc, BudgetReq: budgetReqSvc,
		Schedule: scheduleSvc,
	}); err != nil {
		log.Fatalf("run: %v", err)
	}
}
