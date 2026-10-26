package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	rt "github.com/aiarsenal/runtime-go"
	_ "github.com/aiarsenal/runtime-go/localdocker"
	_ "github.com/aiarsenal/runtime-go/tencent"
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/infrastructure/agentruntime"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/cache"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/market"
	xfyun "aiarsenal/infrastructure/market/xfyun"
	"aiarsenal/infrastructure/persistence"
	"aiarsenal/infrastructure/runtimenop"
	"aiarsenal/infrastructure/scheduler"
	"aiarsenal/interfaces/controller"
	"aiarsenal/interfaces/middleware"
	"aiarsenal/interfaces/router"
	"aiarsenal/logx"
)

func main() {
	logx.InitEnv("/var/log/aiarsenal/api", "api", slog.LevelInfo)

	// 1. 配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 2. 基础设施：DB / Redis / casbin
	if _, err := db.InitDB(); err != nil {
		log.Fatalf("init db: %v", err)
	}
	if _, err := db.InitSkillDB(); err != nil {
		log.Fatalf("init skill db: %v", err)
	}
	if _, err := db.InitLLMDB(); err != nil {
		log.Fatalf("init llm db: %v", err)
	}
	if _, err := db.InitMcpDB(); err != nil {
		log.Fatalf("init mcp db: %v", err)
	}
	if _, err := db.InitAgentDB(); err != nil {
		log.Fatalf("init agent db: %v", err)
	}
	if _, err := db.InitClientDB(); err != nil {
		log.Fatalf("init client db: %v", err)
	}
	if _, err := db.InitProjectDB(); err != nil {
		log.Fatalf("init project db: %v", err)
	}
	if _, err := db.InitDocDB(); err != nil {
		log.Fatalf("init doc db: %v", err)
	}
	if _, err := db.InitKbDB(); err != nil {
		log.Fatalf("init kb db: %v", err)
	}
	if _, err := db.InitScheduleDB(); err != nil {
		log.Fatalf("init schedule db: %v", err)
	}
	// 2.1 数据库迁移（golang-migrate，迁移 SQL 编进二进制）。migrations/<db>/ 下有 .sql 即自动
	//     执行（给存量库追平 base SQL 增量），无 .sql 的库静默跳过。失败则启动中止（fail-fast）。
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	// 2.2 历史临时预算申请搬运：aa_llm_budget_request_archive → 主库 aa_review_order
	//     （幂等，ON CONFLICT(order_id) DO NOTHING；只在 api 进程跑，mcp-ops 不跑避免双进程竞争）
	if n, err := persistence.MigrateLegacyBudgetRequests(); err != nil {
		log.Printf("warn: migrate legacy budget requests: %v", err)
	} else if n > 0 {
		log.Printf("migrated %d legacy budget requests into aa_review_order", n)
	}
	if _, err := db.InitRedis(); err != nil {
		log.Fatalf("init redis: %v", err)
	}
	if _, err := auth.Enforcer(); err != nil {
		log.Printf("warn: init casbin: %v (权限校验将不可用)", err)
	}

	// 3. 仓储
	c := cache.New()
	userRepo := persistence.NewUserRepository()
	menuRepo := persistence.NewMenuRepository()
	postRepo := persistence.NewPostRepository()
	deptRepo := persistence.NewDeptRepository()
	loginLogRepo := persistence.NewLoginLogRepository()
	operLogRepo := persistence.NewOperLogRepository()
	onlineRepo := persistence.NewUserOnlineRepository()
	roleRepo := persistence.NewRoleRepository()
	permGroupRepo := persistence.NewPermGroupRepository()
	empSetRepo := persistence.NewEmpSetRepository()

	// Skill 模块仓储（aa_skill 库）
	skillNSRepo := persistence.NewSkillNamespaceRepository()
	skillRepo := persistence.NewSkillRepository()
	skillLabelRepo := persistence.NewSkillLabelRepository()
	skillSecRepo := persistence.NewSkillSecurityRepository()

	// Skill 模块基础设施：对象存储(local/s3) + 嵌入提供者 + 扫描器
	skillStorage, err := service.NewSkillStorageFromConfig()
	if err != nil {
		log.Fatalf("init skill storage: %v", err)
	}
	// 嵌入提供者：配置了 llm_embedding（config.yaml llm_embedding 段 / AIARSENAL_LLM_EMBEDDING_* 环境变量）
	// 时走 llm 网关真实语义嵌入，否则自动回落离线 LocalEmbedding（同维度哈希），检索不停摆。
	skillEmbed := service.NewEmbeddingProvider(service.LoadLLMEmbeddingConfig())
	skillScanner := service.NewScannerService(skillRepo, skillSecRepo, skillStorage)

	// LLM 网关模块仓储（llm 库；dept 配置走 aiarsenal 库 db.DB()）
	llmCredRepo := persistence.NewLLMCredentialRepository()
	llmModelRepo := persistence.NewLLMModelRepository()
	llmBudgetRepo := persistence.NewLLMBudgetRepository()
	reviewOrderRepo := persistence.NewReviewOrderRepository()
	llmGroupRepo := persistence.NewLLMModelGroupRepository()
	llmKeyRepo := persistence.NewLLMVirtualKeyRepository()
	llmDeptRepo := persistence.NewLLMDeptProfileRepository()
	llmProjectRepo := persistence.NewLLMProjectRepository()
	llmEndUserRepo := persistence.NewLLMEndUserRepository()
	llmTagRepo := persistence.NewLLMTagRepository()
	llmSpendRepo := persistence.NewLLMSpendLogRepository()
	llmDailyRepo := persistence.NewLLMDailySpendRepository()
	llmErrorRepo := persistence.NewLLMErrorLogRepository()
	llmAuditRepo := persistence.NewLLMAuditLogRepository()
	llmHealthRepo := persistence.NewLLMHealthCheckRepository()
	llmPromptRepo := persistence.NewLLMPromptRepository()

	// MCP 聚合模块仓储（mcp 库；跨库软引用 aa_user）
	mcpServerRepo := persistence.NewMcpServerRepository()
	mcpGroupRepo := persistence.NewMcpGroupRepository()
	mcpKeyRepo := persistence.NewMcpBearerKeyRepository()
	mcpActivityRepo := persistence.NewMcpActivityRepository()
	mcpPromptRepo := persistence.NewMcpBuiltinPromptRepository()
	mcpResourceRepo := persistence.NewMcpBuiltinResourceRepository()
	mcpCfgRepo := persistence.NewMcpConfigRepository()
	mcpUserCfgRepo := persistence.NewMcpUserConfigRepository()

	// Agent 模块仓储（agent 库；跨库软引用 aa_user）
	agentCfgResRepo := persistence.NewAgentConfigResourceRepository()
	agentCfgBdlRepo := persistence.NewAgentConfigBundleRepository()
	agentCfgBdlItemRepo := persistence.NewAgentConfigBundleItemRepository()
	agentCfgBdlSkRepo := persistence.NewAgentConfigBundleSkillRepository()
	agentScanCfgRepo := persistence.NewAgentSecurityScanConfigRepository()
	agentScanJobRepo := persistence.NewAgentSecurityScanJobRepository()
	agentScanItemRepo := persistence.NewAgentSecurityScanJobItemRepository()
	agentScanRptRepo := persistence.NewAgentSecurityScanReportRepository()
	agentModelRepo := persistence.NewAgentModelRepository()
	agentRuleRepo := persistence.NewAgentRiskRuleRepository()
	agentInvRepo := persistence.NewAgentModelInvocationRepository()
	agentEventRepo := persistence.NewAgentAuditEventRepository()
	agentCostRepo := persistence.NewAgentCostRecordRepository()
	agentHitRepo := persistence.NewAgentRiskHitRepository()
	agentAuditRepo := persistence.NewAgentAuditLogRepository()
	agentQuotaRepo := persistence.NewAgentUserQuotaRepository()

	// Agent v2 运行时/团队层仓储（Phase 1 落数据层；Phase 2 接 Docker 后补齐其余）
	agentInstanceRepo := persistence.NewAgentInstanceRepository()
	agentTeamRepo := persistence.NewAgentTeamRepository()
	agentTeamMemberRepo := persistence.NewAgentTeamMemberRepository()
	agentTeamTaskRepo := persistence.NewAgentTeamTaskRepository()
	agentTeamEventRepo := persistence.NewAgentTeamEventRepository()
	agentTeamWorkItemRepo := persistence.NewAgentTeamWorkItemRepository()
	agentSystemImageRepo := persistence.NewAgentSystemImageSettingRepository()
	// Phase3 控制面仓储（命令队列 + 运行状态 + 实例内 agent 注册）
	agentCmdRepo := persistence.NewAgentInstanceCommandRepository()
	agentRuntimeStatusRepo := persistence.NewAgentInstanceRuntimeStatusRepository()
	agentRuntimeStatusHistoryRepo := persistence.NewAgentInstanceRuntimeStatusHistoryRepository()
	agentConfigChangeRepo := persistence.NewAgentInstanceConfigChangeRepository()
	agentInstanceAgentRepo := persistence.NewAgentInstanceAgentRepository()
	agentInstanceSkillRepo := persistence.NewAgentInstanceSkillRepository()

	// runtime-go Adapter：按 config/tencent.yaml 决定 provider。
	//   enabled=true → tencent（实例创建到腾讯云 CVM）
	//   否则        → localdocker（本机 Docker 真起容器）
	agentRuntimeAdapter, err := buildRuntimeAdapter()
	if err != nil {
		log.Fatalf("init runtime adapter: %v", err)
	}
	log.Printf("runtime adapter: %s", agentRuntimeAdapter.Provider())

	// 4. 应用服务
	captchaSvc := service.NewCaptchaService()
	authSvc := service.NewAuthService(userRepo, menuRepo, captchaSvc)
	postSvc := service.NewPostService(postRepo)
	deptSvc := service.NewDeptService(deptRepo)
	dictSvc := service.NewDictService()
	menuSvc := service.NewMenuService(menuRepo, c)
	roleSvc := service.NewRoleService(roleRepo, menuRepo, c)
	permGroupSvc := service.NewPermGroupService(permGroupRepo, empSetRepo, menuRepo, userRepo)
	userSvc := service.NewUserService(userRepo, roleRepo, postRepo)
	loginLogSvc := service.NewLoginLogService(loginLogRepo)
	operLogSvc := service.NewOperLogService(operLogRepo)
	onlineSvc := service.NewUserOnlineService(onlineRepo, userRepo)
	monitorSvc := service.NewMonitorService()
	cacheSvc := service.NewCacheService(c)
	personalSvc := service.NewPersonalService(userRepo, deptRepo, roleRepo)
	noticeRepo := persistence.NewNoticeRepository()
	noticeSvc := service.NewNoticeService(noticeRepo)

	// Skill 模块服务
	skillNSSvc := service.NewSkillNamespaceService(skillNSRepo)
	skillSvc := service.NewSkillService(skillRepo, userRepo, skillSecRepo)
	skillLabelSvc := service.NewSkillLabelService(skillLabelRepo)
	skillSecSvc := service.NewSkillSecurityService(skillSecRepo, skillScanner)
	skillPkgSvc := service.NewSkillPackageService(skillRepo, skillSecRepo, skillStorage, skillScanner, skillEmbed)
	// 第三方 skill 市场：可同步市场(讯飞)走适配器站内搜索/同步；其余外部市场不代搜不代抓，
	// 收录到 aa_skill_market_site 目录（种子见 skills.sql，页面可增删改查），引导用户去原站搜索后回来导入。
	skillReg := market.NewRegistry()
	skillReg.Register(xfyun.New())
	skillMarketSiteSvc := service.NewSkillMarketSiteService(persistence.NewSkillMarketSiteRepository())
	skillMarketSvc := service.NewSkillMarketService(skillPkgSvc, skillReg)

	// 平台内置配对技能：首启幂等导入（platform 命名空间；best-effort，失败仅 warn 不阻塞启动）
	service.NewSkillBuiltinService(skillNSRepo, skillRepo, skillPkgSvc).Bootstrap(context.Background())

	// LLM 网关模块服务
	llmConfigSvc := service.NewLLMConfigService(llmCredRepo, llmModelRepo, llmBudgetRepo, llmGroupRepo, noticeSvc, llmDailyRepo)
	llmKeySvc := service.NewLLMKeyService(llmKeyRepo)
	llmOrgSvc := service.NewLLMOrgService(llmDeptRepo, llmDailyRepo, llmProjectRepo, llmEndUserRepo, llmTagRepo)
	llmMonitorSvc := service.NewLLMMonitorService(llmSpendRepo, llmDailyRepo, llmErrorRepo, llmAuditRepo, llmHealthRepo, llmPromptRepo)

	// MCP 聚合模块服务
	mcpServerSvc := service.NewMcpServerService(mcpServerRepo)
	mcpProbeSvc := service.NewMcpProbeService(mcpServerRepo, mcpActivityRepo)
	mcpGroupSvc := service.NewMcpGroupService(mcpGroupRepo, mcpServerRepo, userRepo, persistence.NewMcpGroupGrantRepository())
	mcpKeySvc := service.NewMcpKeyService(mcpKeyRepo)
	mcpActivitySvc := service.NewMcpActivityService(mcpActivityRepo)
	mcpBuiltinSvc := service.NewMcpBuiltinService(mcpPromptRepo, mcpResourceRepo)
	mcpCfgSvc := service.NewMcpConfigService(mcpCfgRepo, mcpUserCfgRepo)
	// MCP OAuth 管理端服务（与 cmd/mcp 网关各自一份实例，同库同表；keys 复用 mcpKeySvc）
	mcpOAuthSvc := service.NewMcpOAuthService(
		persistence.NewMcpOAuthClientRepository(),
		persistence.NewMcpOAuthCodeRepository(),
		persistence.NewMcpOAuthTokenRepository(),
		mcpKeySvc)

	// Agent 模块服务
	agentCfgSvc := service.NewAgentConfigService(agentCfgResRepo, agentCfgBdlRepo, agentCfgBdlItemRepo, agentCfgBdlSkRepo)
	agentScanSvc := service.NewAgentScanService(agentScanCfgRepo, agentScanJobRepo, agentScanItemRepo, agentScanRptRepo)
	agentModelSvc := service.NewAgentModelService(agentModelRepo, agentRuleRepo)
	agentMonitorSvc := service.NewAgentMonitorService(agentInvRepo, agentEventRepo, agentCostRepo, agentHitRepo, agentAuditRepo)
	agentTeamSvc := service.NewAgentTeamService(agentTeamRepo, agentTeamMemberRepo, agentTeamTaskRepo, agentTeamEventRepo, agentTeamWorkItemRepo)
	agentSettingsSvc := service.NewAgentSettingsService(agentSystemImageRepo)
	agentDashboardSvc := service.NewAgentDashboardService(agentInstanceRepo, agentTeamRepo, agentModelRepo, agentScanJobRepo)
	agentInstanceSvc := service.NewAgentInstanceService(agentInstanceRepo, agentSystemImageRepo, agentCmdRepo, llmKeySvc, llmModelRepo, agentRuntimeAdapter, agentConfigChangeRepo)
	agentRuntimeSvc := service.NewAgentRuntimeService(agentRuntimeStatusRepo, agentRuntimeStatusHistoryRepo, agentConfigChangeRepo, agentCmdRepo, agentInstanceRepo, agentInstanceAgentRepo, userRepo, agentInstanceSkillRepo, skillRepo, skillSecRepo)
	// hermes mcp_servers 段漂移自愈复用实例服务（SyncHermesMcpSection）；ops 进程不装配（nil 安全）
	agentRuntimeSvc.SetInstanceService(agentInstanceSvc)
	// 双通道通知出口（站内铃铛 + 名下 hermes 数字人转微信，同一消息）单例收口，多处共用：
	// 扫描结论/技能漂移巡检/数字员工创建/MCP server 增删/技能装卸/分组名单变化。
	// scanner 等构造早于 runtime/notice，一律装配尾期经 SetChannelNotice 补注（nil 安全）。
	channelNotice := service.NewChannelNoticeService(noticeSvc, agentRuntimeSvc, agentInstanceRepo)
	skillScanner.SetChannelNotice(channelNotice)
	agentRuntimeSvc.SetChannelNotice(channelNotice)
	agentInstanceSvc.SetChannelNotice(channelNotice)
	mcpServerSvc.SetChannelNotice(channelNotice)

	// 日程模块（aa_schedule 库：事件/参会人/例外/会议室；邀请等通知走双通道）
	scheduleEventRepo := persistence.NewScheduleEventRepository()
	scheduleAttendeeRepo := persistence.NewScheduleAttendeeRepository()
	scheduleExceptionRepo := persistence.NewScheduleExceptionRepository()
	scheduleRoomRepo := persistence.NewMeetingRoomRepository()
	scheduleSvc := service.NewScheduleService(scheduleEventRepo, scheduleAttendeeRepo, scheduleExceptionRepo, scheduleRoomRepo, userRepo)
	scheduleSvc.SetChannelNotice(channelNotice)
	scheduleSvc.Bootstrap(context.Background()) // 会议室全空时灌默认房（幂等 best-effort）
	// 统一审核工单：依赖 agentRuntimeSvc（channel_notify 命令下发）与 noticeSvc（站内铃铛兜底）；
	// llm_temp_budget 为首个类型 handler，后续类型（skill 等）照此注册
	llmTempHandler := service.NewLLMTempBudgetHandler(llmBudgetRepo, llmDailyRepo, postRepo)
	reviewOrderSvc := service.NewReviewOrderService(reviewOrderRepo, userRepo, deptRepo, noticeSvc, agentRuntimeSvc, agentInstanceRepo, llmTempHandler)
	_ = agentQuotaRepo // 配额仓储：v1 暂不接 controller（预留 v2 接 quota 管理页）

	// Client 模块仓储（client 库；独立库，不依赖 agent）
	clientAgentRepo := persistence.NewClientAgentRepository()
	clientVersionRepo := persistence.NewClientAgentVersionRepository()
	clientStatusRepo := persistence.NewClientAgentStatusRepository()
	clientDeviceRepo := persistence.NewClientDeviceRepository()
	clientPackageRepo := persistence.NewClientPackageRepository()
	clientCommandRepo := persistence.NewClientCommandRepository()
	clientLogRepo := persistence.NewClientLogRepository()
	clientConfigChangeRepo := persistence.NewClientConfigChangeRepository()
	clientAppRepo := persistence.NewClientAppRepository()
	clientAppPermRepo := persistence.NewClientAppPermRepository()

	// 项目管理模块仓储（aa_project 库；跨库软引用 aa_user）
	projectRepo := persistence.NewProjectRepository()
	projectMemberRepo := persistence.NewProjectMemberRepository()
	projectTaskRepo := persistence.NewProjectTaskRepository()
	projectCommentRepo := persistence.NewProjectTaskCommentRepository()
	projectActivityRepo := persistence.NewProjectTaskActivityRepository()
	projectMilestoneRepo := persistence.NewProjectMilestoneRepository()
	projectTagRepo := persistence.NewProjectTagRepository()
	projectStatusRepo := persistence.NewProjectStatusRepository()
	projectFieldRepo := persistence.NewProjectCustomFieldRepository()
	projectBindingRepo := persistence.NewProjectBindingRepository()
	// 安装包缓存目录：env 覆盖，默认仓库内 data/client/packages
	packageDir := os.Getenv("CLIENT_PACKAGE_DIR")
	if packageDir == "" {
		packageDir = "/home/ubuntu/aiarsenal/data/client/packages"
	}
	_ = os.MkdirAll(packageDir, 0o755)
	clientSvc := service.NewClientService(clientAgentRepo, clientVersionRepo, clientStatusRepo, clientDeviceRepo, userRepo, clientPackageRepo, clientCommandRepo, clientLogRepo, clientConfigChangeRepo, clientAppRepo, clientAppPermRepo, skillRepo, skillSecRepo, llmKeySvc, llmModelRepo, packageDir)

	// 项目管理模块服务
	projectSvc := service.NewProjectService(projectRepo, projectMemberRepo, projectStatusRepo)
	projectMemberSvc := service.NewProjectMemberService(projectMemberRepo, userRepo)
	projectTaskSvc := service.NewProjectTaskService(projectTaskRepo, projectCommentRepo, projectActivityRepo, projectStatusRepo, userRepo)
	projectBoardSvc := service.NewProjectBoardService(projectTaskRepo, projectStatusRepo, projectActivityRepo, userRepo)
	projectMsSvc := service.NewProjectMilestoneService(projectMilestoneRepo, projectTaskRepo, projectStatusRepo)
	projectActSvc := service.NewProjectActivityService(projectActivityRepo)
	projectCfgSvc := service.NewProjectConfigService(projectTagRepo, projectStatusRepo, projectFieldRepo)
	projectBindSvc := service.NewProjectBindingService(projectBindingRepo)
	projectRptSvc := service.NewProjectReportService(projectTaskRepo, projectMilestoneRepo, projectMsSvc)
	projectTLSvc := service.NewProjectTimelineService(projectTaskRepo)

	// 文档管理模块（aa_doc 库）：嵌入复用技能库的离线 LocalEmbedding（1536 维，与 aa_skill 一致）
	docRepo := persistence.NewDocRepository()
	docDirRepo := persistence.NewDocDirRepository()
	docAccessRepo := persistence.NewDocAccessLogRepository()
	docGrantRepo := persistence.NewDocGrantRepository()
	docSvc := service.NewDocService(docRepo, docDirRepo, docAccessRepo, docGrantRepo, userRepo, skillEmbed, "data/doc")
	docDirSvc := service.NewDocDirService(docDirRepo, docRepo, docGrantRepo, userRepo)

	// 文档空间同步（项目空间/个人空间）：目录随项目/员工账号自动创建与对齐。
	// 启动全量对账 + project/member/user 三服务钩子增量同步（仅 api 进程跑 Bootstrap）。
	docSpaceSvc := service.NewDocSpaceService(docDirRepo, docGrantRepo, projectRepo, projectMemberRepo, userRepo)
	projectSvc.SetDocSpaceSync(docSpaceSvc)
	projectMemberSvc.SetDocSpaceSync(docSpaceSvc)
	userSvc.SetDocSpaceSync(docSpaceSvc)
	docSpaceSvc.Bootstrap(context.Background())

	// 知识库管理模块（aa_kb 库）：与文档管理同构的独立实现；嵌入同样复用技能库的
	// 离线 LocalEmbedding（1536 维，与 aa_skill 一致——向量空间一致，互不干扰）
	kbRepo := persistence.NewKbRepository()
	kbDirRepo := persistence.NewKbDirRepository()
	kbAccessRepo := persistence.NewKbAccessLogRepository()
	kbGrantRepo := persistence.NewKbGrantRepository()
	kbSvc := service.NewKbService(kbRepo, kbDirRepo, kbAccessRepo, kbGrantRepo, userRepo, skillEmbed, "data/kb")
	kbDirSvc := service.NewKbDirService(kbDirRepo, kbRepo, kbGrantRepo, userRepo)

	// KB 数据源连接器（web/数据库/API/对象存储/外部知识库）：进程内 cron 调度 + 执行引擎；
	// 外部检索透传回指 kbSvc（Search 合并外部数据源结果）。连接器凭据在各数据源 config jsonb 内。
	kbDsRepo := persistence.NewKbDatasourceRepository()
	kbRunRepo := persistence.NewKbDatasourceRunRepository()
	kbDsSvc := service.NewKbDatasourceService(kbDsRepo, kbRunRepo, kbDirRepo, kbGrantRepo, kbSvc)
	kbSch := scheduler.New()
	kbDsSvc.SetScheduler(kbSch)
	kbSch.Start()
	kbDsSvc.RegisterSchedules(context.Background())
	kbSvc.SetExternalSearch(kbDsSvc)

	// KB 项目空间/个人空间：目录随项目/员工账号自动创建与对齐
	// （启动全量对账 + project/member/user 三服务钩子增量同步，与文档空间同构）。
	kbSpaceSvc := service.NewKbSpaceService(kbDirRepo, kbGrantRepo, projectRepo, projectMemberRepo, userRepo)
	projectSvc.SetKbSpaceSync(kbSpaceSvc)
	projectMemberSvc.SetKbSpaceSync(kbSpaceSvc)
	userSvc.SetKbSpaceSync(kbSpaceSvc)
	kbSpaceSvc.Bootstrap(context.Background())

	// MCP 分组绑定服务（实例=agent 库直绑∪项目继承+读改写重推；设备=client 库直绑+restart 生效）。
	// 横跨 agent/client/project 三模块的仓储与服务，统一在此装配并回指互连（防构造循环）。
	agentInstanceMcpRepo := persistence.NewAgentInstanceMcpGroupRepository()
	agentMcpSvc := service.NewAgentMcpService(agentInstanceRepo, agentInstanceMcpRepo, mcpKeySvc, mcpGroupSvc, projectBindingRepo, projectRepo, agentCmdRepo, agentInstanceSvc, cfg)
	agentInstanceSvc.SetMcpService(agentMcpSvc)
	projectBindSvc.SetMcpService(agentMcpSvc)
	agentMcpSvc.SetRuntimeService(agentRuntimeSvc) // 绑定即配手册：SyncInstance 尾部自动装配对技能
	agentMcpSvc.SetChannelNotice(channelNotice)    // 分组 server 名单变化：受影响实例归属人双通道通知
	mcpGroupSvc.SetAgentMcpService(agentMcpSvc)    // 分组 server 名单变化：fan-out 重算绑定实例的配对手册

	// ==== Agent v2：docker 桥接 + 服务装配（tty/workspace/持久卷/备份/滚动发布/用量/配置页/用户） ====
	agentBridge, err := agentruntime.New() // docker client 直连桥接（惰性连接；仅 localdocker 分支会真正用到）
	if err != nil {
		log.Fatalf("init agent runtime bridge: %v", err)
	}
	defer agentBridge.Close()

	agentVolumeSvc := service.NewAgentVolumeService(persistence.NewAgentPersistentVolumeRepository(), agentInstanceRepo, agentRuntimeAdapter, agentBridge)
	agentTtySvc := service.NewAgentTtyService(agentBridge, agentRuntimeAdapter, agentInstanceRepo)
	agentWorkspaceSvc := service.NewAgentWorkspaceService(agentBridge, agentRuntimeAdapter, agentInstanceRepo, persistence.NewAgentWorkspaceFileAuditRepository())
	agentBackupSvc := service.NewAgentBackupService(persistence.NewAgentBackupRepository(), persistence.NewAgentBackupScheduleRepository(), agentInstanceRepo, agentRuntimeAdapter, agentBridge, kbSch) // 复用上方 kbSch（Register 按任务名隔离 agent-backup-*），Start 幂等
	agentRolloutSvc := service.NewAgentRolloutService(persistence.NewAgentRuntimeRolloutRepository(), persistence.NewAgentRuntimePodRepository(), agentInstanceRepo, agentRuntimeStatusRepo, agentRuntimeAdapter)
	agentUsageSvc := service.NewAgentUsageService(persistence.NewAgentInstanceUsageRepository())
	agentOpenclawCfgSvc := service.NewAgentOpenclawConfigService(agentCmdRepo, agentConfigChangeRepo, agentInstanceRepo)
	agentUserSvc := service.NewAgentUserService(persistence.NewAgentUserRepository())

	agentInstanceSvc.SetVolumeService(agentVolumeSvc)        // 创建实例 volumeIds 绑定
	agentBackupSvc.SetInstanceService(agentInstanceSvc)      // 恢复时停/启实例
	agentRolloutSvc.SetInstanceService(agentInstanceSvc)     // 批次重启走 Restart
	agentOpenclawCfgSvc.SetInstanceService(agentInstanceSvc) // 配置下发走 pushOpenclawConfigRaw

	clientDeviceMcpRepo := persistence.NewClientDeviceMcpGroupRepository()
	clientMcpSvc := service.NewClientMcpService(clientDeviceRepo, clientStatusRepo, clientDeviceMcpRepo, mcpKeySvc, mcpGroupSvc, clientSvc, cfg)
	clientSvc.SetMcpService(clientMcpSvc)

	// 5. 控制器
	captchaCtl := controller.NewCaptchaController(captchaSvc)
	loginCtl := controller.NewLoginController(authSvc)
	postCtl := controller.NewPostController(postSvc)
	deptCtl := controller.NewDeptController(deptSvc)
	dictCtl := controller.NewDictController(dictSvc)
	menuCtl := controller.NewMenuController(menuSvc, roleSvc)
	roleCtl := controller.NewRoleController(roleSvc)
	permGroupCtl := controller.NewPermGroupController(permGroupSvc)
	userCtl := controller.NewUserController(userSvc)
	loginLogCtl := controller.NewLoginLogController(loginLogSvc)
	operLogCtl := controller.NewOperLogController(operLogSvc)
	onlineCtl := controller.NewUserOnlineController(onlineSvc)
	monitorCtl := controller.NewMonitorController(monitorSvc)
	cacheCtl := controller.NewCacheController(cacheSvc)
	personalCtl := controller.NewPersonalController(personalSvc)
	noticeCtl := controller.NewNoticeController(noticeSvc)

	// Skill 模块控制器
	skillNSCtl := controller.NewSkillNamespaceController(skillNSSvc)
	skillCtl := controller.NewSkillController(skillSvc)
	skillLabelCtl := controller.NewSkillLabelController(skillLabelSvc)
	skillSecCtl := controller.NewSkillSecurityController(skillSecSvc)
	skillPkgCtl := controller.NewSkillPackageController(skillPkgSvc, skillSvc, skillEmbed)
	skillMarketCtl := controller.NewSkillMarketController(skillMarketSvc, skillMarketSiteSvc)

	// LLM 网关模块控制器
	llmConfigCtl := controller.NewLLMConfigController(llmConfigSvc, agentInstanceSvc)
	llmKeyCtl := controller.NewLLMKeyController(llmKeySvc, agentInstanceSvc, noticeSvc)
	reviewOrderCtl := controller.NewReviewOrderController(reviewOrderSvc)
	llmOrgCtl := controller.NewLLMOrgController(llmOrgSvc)
	llmMonitorCtl := controller.NewLLMMonitorController(llmMonitorSvc)

	// MCP 聚合模块控制器
	mcpServerCtl := controller.NewMcpServerController(mcpServerSvc)
	mcpProbeCtl := controller.NewMcpProbeController(mcpProbeSvc)
	mcpGroupCtl := controller.NewMcpGroupController(mcpGroupSvc)
	mcpKeyCtl := controller.NewMcpKeyController(mcpKeySvc)
	mcpActivityCtl := controller.NewMcpActivityController(mcpActivitySvc)
	mcpBuiltinCtl := controller.NewMcpBuiltinController(mcpBuiltinSvc)
	mcpCfgCtl := controller.NewMcpConfigController(mcpCfgSvc)

	// Agent 模块控制器
	agentCfgCtl := controller.NewAgentConfigController(agentCfgSvc)
	agentScanCtl := controller.NewAgentScanController(agentScanSvc)
	agentModelCtl := controller.NewAgentModelController(agentModelSvc)
	agentMonitorCtl := controller.NewAgentMonitorController(agentMonitorSvc)
	agentTeamCtl := controller.NewAgentTeamController(agentTeamSvc)
	agentSettingsCtl := controller.NewAgentSettingsController(agentSettingsSvc)
	agentDashboardCtl := controller.NewAgentDashboardController(agentDashboardSvc)
	agentInstanceCtl := controller.NewAgentInstanceController(agentInstanceSvc)
	agentRuntimeCtl := controller.NewAgentRuntimeController(agentRuntimeSvc)
	agentMcpCtl := controller.NewAgentMcpController(agentMcpSvc)

	// Agent v2 控制器
	agentTtyCtl := controller.NewAgentTtyController(agentTtySvc, agentInstanceSvc)
	agentWorkspaceCtl := controller.NewAgentWorkspaceController(agentWorkspaceSvc, agentInstanceSvc)
	agentVolumeCtl := controller.NewAgentVolumeController(agentVolumeSvc)
	agentBackupCtl := controller.NewAgentBackupController(agentBackupSvc, agentInstanceSvc)
	agentRolloutCtl := controller.NewAgentRolloutController(agentRolloutSvc)
	agentUsageCtl := controller.NewAgentUsageController(agentUsageSvc)
	agentOpenclawCfgCtl := controller.NewAgentOpenclawConfigController(agentOpenclawCfgSvc, agentInstanceSvc)
	agentUserCtl := controller.NewAgentUserController(agentUserSvc, agentInstanceSvc)
	mcpOAuthAdminCtl := controller.NewMcpOAuthAdminController(mcpOAuthSvc)

	// 数据看板（只读聚合；连接自取，无需仓储注入）
	statsCtl := controller.NewStatsController(service.NewStatsLlmService(), service.NewStatsSkillService())

	// Client 模块控制器
	clientCtl := controller.NewClientController(clientSvc)
	clientMcpCtl := controller.NewClientMcpController(clientMcpSvc)

	// 项目管理模块控制器
	projectCtl := controller.NewProjectController(projectSvc)
	projectMemberCtl := controller.NewProjectMemberController(projectMemberSvc)
	projectTaskCtl := controller.NewProjectTaskController(projectTaskSvc)
	projectBoardCtl := controller.NewProjectBoardController(projectBoardSvc)
	projectMsCtl := controller.NewProjectMilestoneController(projectMsSvc)
	projectCfgCtl := controller.NewProjectConfigController(projectCfgSvc)
	projectBindCtl := controller.NewProjectBindingController(projectBindSvc)
	projectActCtl := controller.NewProjectActivityController(projectActSvc)
	projectRptCtl := controller.NewProjectReportController(projectRptSvc)
	projectTLCtl := controller.NewProjectTimelineController(projectTLSvc)
	docCtl := controller.NewDocController(docSvc)
	docDirCtl := controller.NewDocDirController(docDirSvc, docSpaceSvc)
	kbCtl := controller.NewKbController(kbSvc)
	kbDirCtl := controller.NewKbDirController(kbDirSvc, kbSpaceSvc)
	kbDsCtl := controller.NewKbDatasourceController(kbDsSvc)
	scheduleCtl := controller.NewScheduleController(scheduleSvc)

	// 6. gin 装配
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	auditMeta := middleware.NewAuditMeta(menuRepo, deptRepo)
	warmCtx, warmCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := auditMeta.Warm(warmCtx); err != nil {
		slog.Warn("审计展示字段预热失败，首个写请求会补加载", "err", err)
	}
	warmCancel()

	router.Register(r, router.Deps{
		Captcha:        captchaCtl,
		Login:          loginCtl,
		Post:           postCtl,
		Dept:           deptCtl,
		Dict:           dictCtl,
		Menu:           menuCtl,
		Role:           roleCtl,
		PermGroup:      permGroupCtl,
		User:           userCtl,
		LoginLog:       loginLogCtl,
		OperLog:        operLogCtl,
		Online:         onlineCtl,
		Monitor:        monitorCtl,
		Cache:          cacheCtl,
		Personal:       personalCtl,
		Notice:         noticeCtl,
		SkillNS:        skillNSCtl,
		Skill:          skillCtl,
		SkillLabel:     skillLabelCtl,
		SkillSec:       skillSecCtl,
		SkillPkg:       skillPkgCtl,
		SkillMarket:    skillMarketCtl,
		MenuRepo:       menuRepo,
		DeptRepo:       deptRepo,
		UserRepo:       userRepo,
		AuditMeta:      auditMeta,
		OperLogRepo:    operLogRepo,
		LLMConfig:      llmConfigCtl,
		LLMKey:         llmKeyCtl,
		ReviewOrder:    reviewOrderCtl,
		LLMOrg:         llmOrgCtl,
		LLMMonitor:     llmMonitorCtl,
		Stats:          statsCtl,
		McpServer:      mcpServerCtl,
		McpProbe:       mcpProbeCtl,
		McpGroup:       mcpGroupCtl,
		McpKey:         mcpKeyCtl,
		McpActivity:    mcpActivityCtl,
		McpBuiltin:     mcpBuiltinCtl,
		McpConfig:      mcpCfgCtl,
		McpOAuthAdmin:  mcpOAuthAdminCtl,
		AgentConfig:    agentCfgCtl,
		AgentScan:      agentScanCtl,
		AgentModel:     agentModelCtl,
		AgentMonitor:   agentMonitorCtl,
		AgentTeam:      agentTeamCtl,
		AgentSettings:  agentSettingsCtl,
		AgentDashboard: agentDashboardCtl,
		AgentInstance:  agentInstanceCtl,
		AgentRuntime:   agentRuntimeCtl,
		AgentMcp:       agentMcpCtl,
		// Agent v2（tty/workspace/持久卷/备份/滚动发布/用量/OpenClaw 配置页/Agent 用户）
		AgentTty:            agentTtyCtl,
		AgentWorkspace:      agentWorkspaceCtl,
		AgentVolume:         agentVolumeCtl,
		AgentBackup:         agentBackupCtl,
		AgentRollout:        agentRolloutCtl,
		AgentUsage:          agentUsageCtl,
		AgentOpenclawConfig: agentOpenclawCfgCtl,
		AgentUser:           agentUserCtl,
		Client:              clientCtl,
		ClientSvc:           clientSvc,
		ClientMcp:           clientMcpCtl,
		Project:             projectCtl,
		ProjectMember:       projectMemberCtl,
		ProjectTask:         projectTaskCtl,
		ProjectBoard:        projectBoardCtl,
		ProjectMs:           projectMsCtl,
		ProjectCfg:          projectCfgCtl,
		ProjectBind:         projectBindCtl,
		ProjectAct:          projectActCtl,
		ProjectRpt:          projectRptCtl,
		ProjectTL:           projectTLCtl,
		Doc:                 docCtl,
		DocDir:              docDirCtl,
		Kb:                  kbCtl,
		KbDir:               kbDirCtl,
		KbDatasource:        kbDsCtl,
		Schedule:            scheduleCtl,
	})

	// 7. 启动（PORT 环境变量可覆盖，便于开发期避开 :43211）
	addr := cfg.Server.Address
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("AI Arsenal API (gin) listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// buildRuntimeAdapter 按 config/tencent.yaml 选择 runtime provider。
//
//	AIARSENAL_RUNTIME=disabled → runtimenop 空实现（无 Docker 的环境如 Windows 测试机；
//	                             api 正常启动，仅 Agent 实例操作报错）
//	enabled=true → tencent CVM（实例创建到腾讯云）
//	否则        → localdocker（本机 Docker）
func buildRuntimeAdapter() (rt.Adapter, error) {
	if os.Getenv("AIARSENAL_RUNTIME") == "disabled" {
		return runtimenop.Adapter{}, nil
	}
	tc, err := config.LoadTencent()
	if err != nil {
		return nil, err
	}
	if tc != nil && tc.Enabled {
		params := map[string]any{
			"mode":            "cvm",
			"region":          tc.Region,
			"zone":            tc.Zone,
			"vpcId":           tc.VpcID,
			"subnetId":        tc.SubnetID,
			"securityGroupId": tc.SecurityGroupID,
			"imageId":         tc.ImageID,
			"instanceType":    tc.InstanceType,
			"bandwidthMbps":   tc.BandwidthMbps,
			"loginKey":        tc.LoginKey,
			"loginUser":       tc.LoginUser,
		}
		if tc.SecretID != "" {
			params["secretId"] = tc.SecretID
		}
		if tc.SecretKey != "" {
			params["secretKey"] = tc.SecretKey
		}
		return rt.New(rt.Config{Type: "tencent", Params: params})
	}
	params := map[string]any{}
	// 跨 OS：Linux 默认 /var/run/docker.sock 即可；macOS（colima / Docker Desktop）的 socket 不在标准路径，
	// 由 DOCKER_HOST 指定。runtime-go localdocker 不读 env，故这里显式把 DOCKER_HOST 透传成 dockerHost 参数。
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		params["dockerHost"] = host
	}
	return rt.New(rt.Config{Type: "localdocker", Params: params})
}
