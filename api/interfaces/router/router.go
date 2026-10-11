package router

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/repository"
	"aiarsenal/interfaces/controller"
	"aiarsenal/interfaces/middleware"
)

// Deps 路由装配依赖（由 cmd/main 注入）
type Deps struct {
	Captcha  *controller.CaptchaController
	Login    *controller.LoginController
	Post     *controller.PostController
	Dept     *controller.DeptController
	Dict     *controller.DictController
	Menu     *controller.MenuController
	Role     *controller.RoleController
	User     *controller.UserController
	LoginLog *controller.LoginLogController
	OperLog  *controller.OperLogController
	Online   *controller.UserOnlineController
	Monitor  *controller.MonitorController
	Cache    *controller.CacheController
	Personal *controller.PersonalController
	Notice   *controller.NoticeController
	// Skill 模块
	SkillNS     *controller.SkillNamespaceController
	Skill       *controller.SkillController
	SkillLabel  *controller.SkillLabelController
	SkillSec    *controller.SkillSecurityController
	SkillPkg    *controller.SkillPackageController
	SkillMarket *controller.SkillMarketController
	MenuRepo    repository.IMenuRepository
	// LLM 网关模块（llm 库，跨库软引用 aa_user）
	LLMConfig       *controller.LLMConfigController
	LLMKey          *controller.LLMKeyController
	LLMOrg     *controller.LLMOrgController
	LLMMonitor *controller.LLMMonitorController
	// 数据看板（LLM 用量 / 技能资产，只读聚合）
	Stats *controller.StatsController
	// 统一审核工单（主库 aa_review_order，审核系统「我的提交/我的审核」）
	ReviewOrder *controller.ReviewOrderController
	// MCP 聚合模块（mcp 库，跨库软引用 aa_user）— 仅管理后台 CRUD；协议端点在独立 mcp/ 服务
	McpServer   *controller.McpServerController
	McpProbe    *controller.McpProbeController
	McpGroup    *controller.McpGroupController
	McpKey      *controller.McpKeyController
	McpActivity *controller.McpActivityController
	McpBuiltin  *controller.McpBuiltinController
	McpConfig   *controller.McpConfigController
	// MCP OAuth 管理端（客户端/令牌 CRUD；协议端点在独立 mcp/ 服务）
	McpOAuthAdmin *controller.McpOAuthAdminController
	// Agent 模块（agent 库；跨库软引用 aa_user）
	AgentConfig    *controller.AgentConfigController
	AgentScan      *controller.AgentScanController
	AgentModel     *controller.AgentModelController
	AgentMonitor   *controller.AgentMonitorController
	AgentTeam      *controller.AgentTeamController
	AgentSettings  *controller.AgentSettingsController
	AgentDashboard *controller.AgentDashboardController
	AgentInstance  *controller.AgentInstanceController
	AgentRuntime   *controller.AgentRuntimeController
	AgentMcp       *controller.AgentMcpController
	// Agent v2（tty/workspace/持久卷/备份/滚动发布/用量/OpenClaw 配置页/Agent 用户）
	AgentTty            *controller.AgentTtyController
	AgentWorkspace      *controller.AgentWorkspaceController
	AgentVolume         *controller.AgentVolumeController
	AgentBackup         *controller.AgentBackupController
	AgentRollout        *controller.AgentRolloutController
	AgentUsage          *controller.AgentUsageController
	AgentOpenclawConfig *controller.AgentOpenclawConfigController
	AgentUser           *controller.AgentUserController
	// Client 模块（桌面客户端 Win/Mac：device-code 登录中转 + 客户端规格 + 运行情况上报 + 设备管理）
	Client    *controller.ClientController
	ClientSvc *service.ClientService
	ClientMcp *controller.ClientMcpController
	// 项目管理模块（aa_project 库，跨库软引用 aa_user）
	Project       *controller.ProjectController
	ProjectMember *controller.ProjectMemberController
	ProjectTask   *controller.ProjectTaskController
	ProjectBoard  *controller.ProjectBoardController
	ProjectMs     *controller.ProjectMilestoneController
	ProjectCfg    *controller.ProjectConfigController
	ProjectBind   *controller.ProjectBindingController
	ProjectAct    *controller.ProjectActivityController
	ProjectRpt    *controller.ProjectReportController
	ProjectTL     *controller.ProjectTimelineController
	// 文档管理模块（aa_doc 库）
	Doc    *controller.DocController
	DocDir *controller.DocDirController
	// 知识库管理模块（aa_kb 库；与文档管理同构的独立实现）
	Kb           *controller.KbController
	KbDir        *controller.KbDirController
	KbDatasource *controller.KbDatasourceController
	// 权限分组管理（分组/员工集合两字典）
	PermGroup *controller.PermGroupController
	// 日程模块（aa_schedule 库：我的日程/会议室预订/周期会议/忙闲）
	Schedule *controller.ScheduleController
}

// Register 注册全部路由
func Register(r *gin.Engine, d Deps) {
	r.Use(middleware.Recovery())
	// CORS 预检兜底：命中路由但方法不匹配时（如 OPTIONS 预检 GET 路由），
	// 走 NoMethod 返回 204 + CORS 头，避免浏览器预检 404。
	r.HandleMethodNotAllowed = true
	r.NoMethod(middleware.NoMethodHandler())
	// 路由不存在兜底：补 CORS 头（NoRoute 不走分组中间件链），旧前端页面调已下线
	// 接口时能拿到明确 404 而不是跨域失败式的「网络异常」。
	r.NoRoute(middleware.NoRouteHandler())

	v1 := r.Group("/api/v1")
	v1.Use(middleware.CORS())

	// 公开：验证码
	pub := v1.Group("/pub")
	pub.Group("/captcha").GET("/get", d.Captcha.Get)

	// system 分组
	sys := v1.Group("/system")
	// 公开：登录 / 退出
	sys.POST("/login", d.Login.Login)
	sys.GET("/logout", d.Login.Logout)

	// Client 模块：device-code 登录中转 + 可安装 agent 列表 + 运行情况上报 + 设备管理
	// exchange 公开（App 用 deviceCode 换 JWT）；其余 JWTAuth + ClientDeviceAuth
	// （web token deviceId 空→跳过设备校验；App token deviceId 非空→校验 active）
	clientPub := sys.Group("/client")
	clientPub.POST("/exchange", d.Client.Exchange)
	// 安装包下载（公开，无鉴权——安装包公开分发，client/管理员/浏览器凭 agent+version+platform 直下）
	clientPub.GET("/packages/:agent/:version/:platform", d.Client.DownloadPackage)
	clientAuth := sys.Group("/client", middleware.JWTAuth(), middleware.ClientDeviceAuth(d.ClientSvc))
	clientAuth.POST("/bind", d.Client.Bind)
	clientAuth.GET("/agents", d.Client.GetAgents)
	// 心跳合一：状态+日志+配置变更+命令进度+命令下发（App 5s 调一次）
	clientAuth.POST("/heartbeat", d.Client.Heartbeat)
	// 技能整包下载（设备 token）：skill_install 命令 payload.packagePath 指向此处，App 凭自身
	// device token 拉取 zip 就地解压安装。仅设备 token 可访问（web 用户 JWT 无 deviceId 会被拒）。
	clientAuth.GET("/skill/package", d.SkillPkg.DownloadDevicePackage)
	// 设备自助 MCP 分组管理（桌面客户端 MCP 管理模块）：deviceId 取 JWT，web token 控制器侧 403 拒绝
	clientAuth.GET("/mcp", d.ClientMcp.ListSelf)
	clientAuth.POST("/mcp/bind", d.ClientMcp.BindSelf)
	clientAuth.POST("/mcp/unbind", d.ClientMcp.UnbindSelf)
	// 安装包下载已移至 clientPub（公开，无鉴权）

	// Client 管理后台（JWTAuth + CasbinAuth；菜单 id 125~137，见 client.sql Part B）
	clientAdmin := sys.Group("/client/admin", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	clientAdmin.GET("/agent/list", d.Client.AdminListAgents)
	clientAdmin.PUT("/agent/save", d.Client.AdminSaveAgent)
	clientAdmin.DELETE("/agent/delete", d.Client.AdminDeleteAgent)
	clientAdmin.GET("/app/list", d.Client.AdminListApps)
	clientAdmin.GET("/app/suggest", d.Client.AdminSuggestApps)
	clientAdmin.PUT("/app/save", d.Client.AdminSaveApp)
	clientAdmin.DELETE("/app/delete", d.Client.AdminDeleteApp)
	clientAdmin.GET("/appperm/get", d.Client.AdminGetDeviceAppPerms)
	clientAdmin.PUT("/appperm/save", d.Client.AdminSetDeviceAppPerms)
	clientAdmin.POST("/appperm/reset", d.Client.AdminResetDeviceAppPerm)
	clientAdmin.PUT("/version/save", d.Client.AdminSaveVersion)
	clientAdmin.DELETE("/version/delete", d.Client.AdminDeleteVersion)
	clientAdmin.GET("/status/list", d.Client.AdminListStatus)
	clientAdmin.GET("/device/list", d.Client.AdminListDevices)
	clientAdmin.GET("/device/get", d.Client.AdminGetDevice)
	clientAdmin.GET("/device/detail", d.Client.AdminGetDeviceDetail)
	clientAdmin.PUT("/device/selected-agents", d.Client.AdminSetSelectedAgents)
	// 渠道绑定（微信等）：列表/启用/扫码/登录/登出/装插件
	clientAdmin.GET("/device/channels", d.Client.AdminListChannels)
	clientAdmin.PUT("/device/channel", d.Client.AdminSetChannel)
	clientAdmin.GET("/device/channel/qr", d.Client.AdminGetChannelQr)
	clientAdmin.POST("/device/channel/login", d.Client.AdminChannelLogin)
	clientAdmin.POST("/device/channel/logout", d.Client.AdminChannelLogout)
	clientAdmin.POST("/device/channel/unbind", d.Client.AdminChannelUnbind)
	clientAdmin.POST("/device/channel/install-plugin", d.Client.AdminChannelInstallPlugin)
	// 技能管理（装/卸走远程指令队列，列表读心跳上报的 skills 快照）
	clientAdmin.GET("/device/skills", d.Client.AdminListDeviceSkills)
	clientAdmin.POST("/device/skill/install", d.Client.AdminInstallDeviceSkill)
	clientAdmin.POST("/device/skill/uninstall", d.Client.AdminUninstallDeviceSkill)
	// MCP 分组绑定（设备详情）：绑/解绑自动签 key + 尝试 restart 生效
	clientAdmin.GET("/device/mcp", d.ClientMcp.List)
	clientAdmin.POST("/device/mcp/bind", d.ClientMcp.Bind)
	clientAdmin.POST("/device/mcp/unbind", d.ClientMcp.Unbind)
	clientAdmin.PUT("/device/revoke", d.Client.AdminRevokeDevice)
	clientAdmin.PUT("/device/block", d.Client.AdminBlockDevice)
	clientAdmin.DELETE("/device/delete", d.Client.AdminDeleteDevice)
	clientAdmin.POST("/device/command", d.Client.AdminEnqueueCommand)
	clientAdmin.GET("/device/commands/:deviceId", d.Client.AdminListCommands)
	clientAdmin.GET("/device/logs", d.Client.AdminListLogs)
	clientAdmin.GET("/device/status-history", d.Client.AdminListStatusByDevice)
	clientAdmin.GET("/device/config-changes", d.Client.AdminListConfigChanges)
	clientAdmin.GET("/device/openclaw-config", d.Client.AdminGetOpenclawConfig)
	clientAdmin.GET("/package/list", d.Client.AdminListPackages)
	clientAdmin.PUT("/package/fetch", d.Client.AdminFetchPackage)
	clientAdmin.DELETE("/package/delete", d.Client.AdminDeletePackage)

	// 受保护：JWT + casbin
	protected := sys.Group("", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	// 用户菜单（当前登录用户）
	protected.GET("/user/getUserMenus", d.Login.GetUserMenus)

	// 岗位
	post := protected.Group("/post")
	post.GET("/list", d.Post.List)
	post.POST("/add", d.Post.Add)
	post.PUT("/edit", d.Post.Edit)
	post.DELETE("/delete", d.Post.Delete)

	// 权限分组管理（分组/员工集合两字典；写后快照刷新即时生效）
	// options 为 GrantEditor 选择器瘦投影：不加菜单行走 casbin 隐式放行（同 user/search 惯例），
	// 登录即可拉到 {id,name}，不吐词条/工号明细。
	pg := protected.Group("/permgroup")
	pgGroup := pg.Group("/group")
	pgGroup.GET("/list", d.PermGroup.GroupList)
	pgGroup.POST("/add", d.PermGroup.GroupSave)
	pgGroup.PUT("/edit", d.PermGroup.GroupSave)
	pgGroup.DELETE("/delete", d.PermGroup.GroupDelete)
	pgSet := pg.Group("/set")
	pgSet.GET("/list", d.PermGroup.SetList)
	pgSet.POST("/add", d.PermGroup.SetSave)
	pgSet.PUT("/edit", d.PermGroup.SetSave)
	pgSet.DELETE("/delete", d.PermGroup.SetDelete)
	pgSet.POST("/resolve", d.PermGroup.SetResolve) // 成员工号→姓名/部门明细
	pg.GET("/options", d.PermGroup.Options)

	// 部门
	dept := protected.Group("/dept")
	dept.GET("/list", d.Dept.List)
	dept.GET("/treeSelect", d.Dept.TreeSelect)
	dept.POST("/add", d.Dept.Add)
	dept.PUT("/edit", d.Dept.Edit)
	dept.DELETE("/delete", d.Dept.Delete)

	// 字典数据（仅查询，数据内置硬编码，无 CRUD）
	dictData := protected.Group("/dict/data")
	dictData.GET("/getDictData", d.Dict.GetDictData)

	// 菜单
	menu := protected.Group("/menu")
	menu.GET("/list", d.Menu.List)
	menu.POST("/add", d.Menu.Add)
	menu.GET("/getParams", d.Menu.GetParams)
	menu.GET("/get", d.Menu.Get)
	menu.PUT("/update", d.Menu.Update)
	menu.DELETE("/delete", d.Menu.Delete)

	// 角色
	role := protected.Group("/role")
	role.GET("/list", d.Role.List)
	role.GET("/getParams", d.Role.GetParams)
	role.GET("/get", d.Role.Get)
	role.POST("/add", d.Role.Add)
	role.PUT("/edit", d.Role.Edit)
	role.DELETE("/delete", d.Role.Delete)

	// 用户
	user := protected.Group("/user")
	user.GET("/list", d.User.List)
	user.GET("/search", d.User.SearchOptions)
	// 按 id 批量解析用户姓名/工号/部门（GrantEditor 已选用户回显；同 search 隐式放行）
	user.GET("/resolve", d.User.ResolveOptions)
	user.GET("/params", d.User.GetParams)
	user.POST("/add", d.User.Add)
	user.PUT("/edit", d.User.Edit)
	user.GET("/getEdit", d.User.GetEdit)
	user.PUT("/resetPwd", d.User.ResetPwd)
	user.PUT("/setStatus", d.User.SetStatus)
	user.DELETE("/delete", d.User.Delete)
	user.GET("/getUsers", d.User.GetUsers)

	// 登录日志
	loginLog := protected.Group("/loginLog")
	loginLog.GET("/list", d.LoginLog.List)
	loginLog.DELETE("/delete", d.LoginLog.Delete)
	loginLog.DELETE("/clear", d.LoginLog.Clear)

	// 操作日志
	operLog := protected.Group("/operLog")
	operLog.GET("/list", d.OperLog.List)
	operLog.GET("/get", d.OperLog.Get)
	operLog.DELETE("/delete", d.OperLog.Delete)
	operLog.DELETE("/clear", d.OperLog.Clear)

	// 在线用户
	online := protected.Group("/online")
	online.GET("/list", d.Online.List)
	online.DELETE("/delete", d.Online.Delete)

	// 服务监控 / 缓存
	protected.GET("/monitor/server", d.Monitor.Server)
	protected.DELETE("/cache/remove", d.Cache.Remove)

	// 个人中心
	personal := protected.Group("/personal")
	personal.GET("/getPersonalInfo", d.Personal.GetPersonalInfo)
	personal.PUT("/edit", d.Personal.Edit)
	personal.PUT("/resetPwd", d.Personal.ResetPwd)

	// 个人消息通知
	notice := protected.Group("/notice")
	notice.GET("/list", d.Notice.List)
	notice.GET("/unreadCount", d.Notice.UnreadCount)
	notice.PUT("/read", d.Notice.MarkRead)
	notice.PUT("/readAll", d.Notice.MarkAllRead)
	notice.DELETE("/delete", d.Notice.Delete)

	// ===== Skill 模块（aa_skill 库，跨库软引用 aa_user） =====
	skillGrp := v1.Group("/skill", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	// 命名空间管理
	skillNS := skillGrp.Group("/namespace")
	skillNS.GET("/list", d.SkillNS.List)
	skillNS.GET("/all", d.SkillNS.All)
	skillNS.GET("/get", d.SkillNS.Get)
	skillNS.POST("/add", d.SkillNS.Add)
	skillNS.PUT("/edit", d.SkillNS.Edit)
	skillNS.DELETE("/delete", d.SkillNS.Delete)
	skillNS.GET("/member/list", d.SkillNS.ListMembers)
	skillNS.POST("/member/add", d.SkillNS.AddMember)
	skillNS.DELETE("/member/remove", d.SkillNS.RemoveMember)

	// 技能管理
	sk := skillGrp.Group("/skill")
	sk.GET("/list", d.Skill.List)
	sk.GET("/get", d.Skill.Get)
	sk.POST("/add", d.Skill.Add)
	sk.PUT("/edit", d.Skill.Edit)
	sk.DELETE("/delete", d.Skill.Delete)
	sk.PUT("/setHidden", d.Skill.SetHidden)
	sk.GET("/version/list", d.Skill.ListVersions)
	sk.GET("/version/get", d.Skill.GetVersion)
	sk.POST("/version/add", d.Skill.AddVersion)
	sk.POST("/version/upload", d.SkillPkg.Upload)
	sk.POST("/version/publish", d.Skill.PublishVersion)
	sk.PUT("/version/yank", d.Skill.YankVersion)
	sk.GET("/tag/list", d.Skill.ListTags)
	sk.POST("/tag/add", d.Skill.AddTag)
	sk.DELETE("/tag/remove", d.Skill.RemoveTag)
	sk.POST("/package/parse", d.SkillPkg.Parse)
	sk.POST("/package/create", d.SkillPkg.Create)
	sk.GET("/version/file/list", d.SkillPkg.ListFiles)
	sk.GET("/file/download", d.SkillPkg.DownloadFile)
	sk.GET("/version/download", d.SkillPkg.DownloadVersion)
	sk.GET("/search", d.SkillPkg.Search)

	// 第三方市场（站内搜索/一键同步 + 链接导入 + 外部市场目录 CRUD）
	mk := skillGrp.Group("/market")
	mk.GET("/sources", d.SkillMarket.Sources)
	mk.GET("/search", d.SkillMarket.Search)
	mk.POST("/sync", d.SkillMarket.Sync)
	mk.POST("/import", d.SkillMarket.Import)
	mk.GET("/site/list", d.SkillMarket.SiteList)
	mk.GET("/site/all", d.SkillMarket.SiteAll)
	mk.POST("/site/add", d.SkillMarket.SiteAdd)
	mk.POST("/site/edit", d.SkillMarket.SiteEdit)
	mk.POST("/site/delete", d.SkillMarket.SiteDelete)

	// 标签管理
	lb := skillGrp.Group("/label")
	lb.GET("/list", d.SkillLabel.List)
	lb.GET("/all", d.SkillLabel.All)
	lb.GET("/get", d.SkillLabel.Get)
	lb.POST("/add", d.SkillLabel.Add)
	lb.PUT("/edit", d.SkillLabel.Edit)
	lb.DELETE("/delete", d.SkillLabel.Delete)
	lb.GET("/trans/list", d.SkillLabel.ListTranslations)
	lb.POST("/trans/upsert", d.SkillLabel.UpsertTranslation)

	// 安全审计
	sec := skillGrp.Group("/security")
	sec.GET("/list", d.SkillSec.ListAudits)
	sec.POST("/rescan", d.SkillSec.Rescan)

	// ===== LLM 网关模块（llm 库，跨库软引用 aa_user） =====
	llmGrp := v1.Group("/llm", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	// 凭据
	cred := llmGrp.Group("/credential")
	cred.GET("/list", d.LLMConfig.ListCredentials)
	cred.GET("/get", d.LLMConfig.GetCredential)
	cred.POST("/add", d.LLMConfig.AddCredential)
	cred.PUT("/edit", d.LLMConfig.EditCredential)
	cred.DELETE("/delete", d.LLMConfig.DeleteCredentials)
	cred.POST("/discover-models", d.LLMConfig.DiscoverModels)

	// 模型部署
	mdl := llmGrp.Group("/model")
	mdl.GET("/list", d.LLMConfig.ListModels)
	mdl.GET("/all", d.LLMConfig.AllModels)
	mdl.GET("/get", d.LLMConfig.GetModel)
	mdl.POST("/add", d.LLMConfig.AddModel)
	mdl.PUT("/edit", d.LLMConfig.EditModel)
	mdl.DELETE("/delete", d.LLMConfig.DeleteModels)

	// 预算
	bdg := llmGrp.Group("/budget")
	bdg.GET("/list", d.LLMConfig.ListBudgets)
	bdg.GET("/get", d.LLMConfig.GetBudget)
	bdg.GET("/mine", d.LLMConfig.MyBudgets)
	bdg.GET("/spend/history", d.LLMConfig.MySpendHistory)
	bdg.POST("/add", d.LLMConfig.AddBudget)
	bdg.PUT("/edit", d.LLMConfig.EditBudget)
	bdg.DELETE("/delete", d.LLMConfig.DeleteBudgets)

	// 模型分组（字典）
	gp := llmGrp.Group("/group")
	gp.GET("/list", d.LLMConfig.ListGroups)
	gp.GET("/get", d.LLMConfig.GetGroup)
	gp.POST("/add", d.LLMConfig.AddGroup)
	gp.PUT("/edit", d.LLMConfig.EditGroup)
	gp.DELETE("/delete", d.LLMConfig.DeleteGroups)

	// 虚拟密钥
	key := llmGrp.Group("/key")
	key.GET("/list", d.LLMKey.List)
	key.GET("/get", d.LLMKey.Get)
	key.POST("/add", d.LLMKey.Add)
	key.PUT("/edit", d.LLMKey.Edit)
	key.DELETE("/delete", d.LLMKey.Delete)
	key.PUT("/block", d.LLMKey.Block)
	key.PUT("/rotate", d.LLMKey.Rotate)
	key.GET("/check-use", d.LLMKey.CheckUse) // 删除前查是否被实例使用（前端二次确认用）

	// 部门 LLM 配置（复用 aa_dept）
	dt := llmGrp.Group("/dept")
	dt.GET("/list", d.LLMOrg.ListDepts)
	dt.GET("/get", d.LLMOrg.GetDept)
	dt.PUT("/edit", d.LLMOrg.EditDeptConfig)

	// 项目
	pj := llmGrp.Group("/project")
	pj.GET("/list", d.LLMOrg.ListProjects)
	pj.GET("/get", d.LLMOrg.GetProject)
	pj.POST("/add", d.LLMOrg.AddProject)
	pj.PUT("/edit", d.LLMOrg.EditProject)
	pj.DELETE("/delete", d.LLMOrg.DeleteProjects)

	// 终端用户
	eu := llmGrp.Group("/endUser")
	eu.GET("/list", d.LLMOrg.ListEndUsers)
	eu.POST("/add", d.LLMOrg.AddEndUser)
	eu.PUT("/edit", d.LLMOrg.EditEndUser)
	eu.DELETE("/delete", d.LLMOrg.DeleteEndUsers)

	// 标签
	tg := llmGrp.Group("/tag")
	tg.GET("/list", d.LLMOrg.ListTags)
	tg.POST("/add", d.LLMOrg.AddTag)
	tg.PUT("/edit", d.LLMOrg.EditTag)
	tg.DELETE("/delete", d.LLMOrg.DeleteTags)

	// 监控：用量 / 日聚合 / 错误 / 审计 / 健康检查 / 提示词
	llmGrp.GET("/spend/list", d.LLMMonitor.ListSpendLogs)
	llmGrp.GET("/spend/get", d.LLMMonitor.GetSpendLog)
	llmGrp.GET("/spend/aggregate", d.LLMMonitor.AggregateSpend)
	llmGrp.GET("/daily/list", d.LLMMonitor.ListDailySpend)
	llmGrp.GET("/error/list", d.LLMMonitor.ListErrorLogs)
	llmGrp.GET("/audit/list", d.LLMMonitor.ListAuditLogs)
	llmGrp.GET("/health/list", d.LLMMonitor.ListHealthChecks)
	llmGrp.GET("/prompt/list", d.LLMMonitor.ListPrompts)
	llmGrp.GET("/prompt/get", d.LLMMonitor.GetPromptVersions)
	llmGrp.POST("/prompt/add", d.LLMMonitor.AddPrompt)

	// ===== 数据看板（LLM 用量 / 技能资产；只读聚合） =====
	statsGrp := v1.Group("/stats", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	{
		llmStats := statsGrp.Group("/llm")
		llmStats.GET("/overview", d.Stats.LlmOverview)
		llmStats.GET("/trend", d.Stats.LlmTrend)
		llmStats.GET("/top", d.Stats.LlmTop)
		skillStats := statsGrp.Group("/skill")
		skillStats.GET("/overview", d.Stats.SkillOverview)
		skillStats.GET("/install-trend", d.Stats.SkillInstallTrend)
	}

	// ===== 统一审核工单（主库 aa_review_order；审核系统「我的提交/我的审核」） =====
	// 自助端点：菜单行只是导航（/review/mine、/review/todo），本组路径无 aa_rule 行时 casbin 放行，
	// 真实授权在 service（提交=本人，审批=部门负责人或超管）。
	reviewGrp := v1.Group("/review", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	ro := reviewGrp.Group("/order")
	ro.POST("/create", d.ReviewOrder.Create)
	ro.GET("/mine", d.ReviewOrder.Mine)
	ro.GET("/review", d.ReviewOrder.ReviewList)
	ro.POST("/approve", d.ReviewOrder.Approve)
	ro.POST("/reject", d.ReviewOrder.Reject)

	// ===== MCP 聚合模块（mcp 库，跨库软引用 aa_user） =====
	// 管理后台：JWT + casbin（协议端点在独立 mcp/ 服务）
	mcpGrp := v1.Group("/mcp", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	srv := mcpGrp.Group("/server")
	srv.GET("/list", d.McpServer.List)
	srv.GET("/all", d.McpServer.All)
	srv.GET("/get", d.McpServer.Get)
	srv.POST("/add", d.McpServer.Add)
	srv.PUT("/edit", d.McpServer.Edit)
	srv.DELETE("/delete", d.McpServer.Delete)
	srv.POST("/test", d.McpProbe.Test)
	srv.POST("/callTool", d.McpProbe.CallTool)

	grp := mcpGrp.Group("/group")
	grp.GET("/list", d.McpGroup.List)
	grp.GET("/all", d.McpGroup.All)
	grp.GET("/get", d.McpGroup.Get)
	grp.POST("/add", d.McpGroup.Add)
	grp.PUT("/edit", d.McpGroup.Edit)
	grp.DELETE("/delete", d.McpGroup.Delete)
	// 组合授权配置（菜单表无此路径 → casbin fail-open，真实门槛在 service 层 canManage）
	grp.GET("/grant/get", d.McpGroup.GrantGet)
	grp.POST("/grant/save", d.McpGroup.GrantSave)

	mcpKey := mcpGrp.Group("/key")
	mcpKey.GET("/list", d.McpKey.List)
	mcpKey.GET("/get", d.McpKey.Get)
	mcpKey.POST("/add", d.McpKey.Add)
	mcpKey.PUT("/edit", d.McpKey.Edit)
	mcpKey.DELETE("/delete", d.McpKey.Delete)
	mcpKey.PUT("/block", d.McpKey.Block)
	mcpKey.PUT("/rotate", d.McpKey.Rotate)

	// OAuth 客户端/令牌管理（管理台 CRUD；协议端点在独立 mcp/ 服务的 /oauth/*）
	oauthCli := mcpGrp.Group("/oauth/client")
	oauthCli.GET("/list", d.McpOAuthAdmin.List)
	oauthCli.POST("/add", d.McpOAuthAdmin.Add)
	oauthCli.PUT("/edit", d.McpOAuthAdmin.Edit)
	oauthCli.DELETE("/delete", d.McpOAuthAdmin.Delete)
	oauthCli.POST("/rotate-secret", d.McpOAuthAdmin.RotateSecret)
	oauthTok := mcpGrp.Group("/oauth/token")
	oauthTok.GET("/list", d.McpOAuthAdmin.TokenList)
	oauthTok.POST("/revoke", d.McpOAuthAdmin.TokenRevoke)

	mcpGrp.GET("/activity/list", d.McpActivity.List)

	pr := mcpGrp.Group("/prompt")
	pr.GET("/list", d.McpBuiltin.ListPrompts)
	pr.GET("/get", d.McpBuiltin.GetPrompt)
	pr.POST("/add", d.McpBuiltin.AddPrompt)
	pr.PUT("/edit", d.McpBuiltin.EditPrompt)
	pr.DELETE("/delete", d.McpBuiltin.DeletePrompts)

	rs := mcpGrp.Group("/resource")
	rs.GET("/list", d.McpBuiltin.ListResources)
	rs.GET("/get", d.McpBuiltin.GetResource)
	rs.POST("/add", d.McpBuiltin.AddResource)
	rs.PUT("/edit", d.McpBuiltin.EditResource)
	rs.DELETE("/delete", d.McpBuiltin.DeleteResources)

	mcpCfg := mcpGrp.Group("/config")
	mcpCfg.GET("/get", d.McpConfig.Get)
	mcpCfg.PUT("/edit", d.McpConfig.Save)
	mcpCfg.GET("/user/get", d.McpConfig.GetUserConfig)
	mcpCfg.PUT("/user/edit", d.McpConfig.SaveUserConfig)

	// ===== Agent 模块（agent 库；跨库软引用 aa_user） =====
	agentGrp := v1.Group("/agent", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	// 配置中心：resource / bundle / bundle-item / bundle-skill
	agCfg := agentGrp.Group("/config")
	agCfg.GET("/resource/list", d.AgentConfig.ListResources)
	agCfg.GET("/resource/get", d.AgentConfig.GetResource)
	agCfg.POST("/resource/add", d.AgentConfig.AddResource)
	agCfg.PUT("/resource/edit", d.AgentConfig.EditResource)
	agCfg.DELETE("/resource/delete", d.AgentConfig.DeleteResources)
	agCfg.GET("/bundle/list", d.AgentConfig.ListBundles)
	agCfg.GET("/bundle/get", d.AgentConfig.GetBundle)
	agCfg.POST("/bundle/add", d.AgentConfig.AddBundle)
	agCfg.PUT("/bundle/edit", d.AgentConfig.EditBundle)
	agCfg.DELETE("/bundle/delete", d.AgentConfig.DeleteBundles)
	agCfg.GET("/bundle-item/list", d.AgentConfig.ListBundleItems)
	agCfg.GET("/bundle-item/get", d.AgentConfig.GetBundleItem)
	agCfg.POST("/bundle-item/add", d.AgentConfig.AddBundleItem)
	agCfg.PUT("/bundle-item/edit", d.AgentConfig.EditBundleItem)
	agCfg.DELETE("/bundle-item/delete", d.AgentConfig.DeleteBundleItems)
	agCfg.GET("/bundle-skill/list", d.AgentConfig.ListBundleSkills)
	agCfg.GET("/bundle-skill/get", d.AgentConfig.GetBundleSkill)
	agCfg.POST("/bundle-skill/add", d.AgentConfig.AddBundleSkill)
	agCfg.PUT("/bundle-skill/edit", d.AgentConfig.EditBundleSkill)
	agCfg.DELETE("/bundle-skill/delete", d.AgentConfig.DeleteBundleSkills)

	// 安全扫描：config / job / job-item / report
	agScan := agentGrp.Group("/scan")
	agScan.GET("/config/list", d.AgentScan.ListConfigs)
	agScan.GET("/config/get", d.AgentScan.GetConfig)
	agScan.POST("/config/add", d.AgentScan.AddConfig)
	agScan.PUT("/config/edit", d.AgentScan.EditConfig)
	agScan.DELETE("/config/delete", d.AgentScan.DeleteConfigs)
	agScan.GET("/job/list", d.AgentScan.ListJobs)
	agScan.GET("/job/get", d.AgentScan.GetJob)
	agScan.POST("/job/add", d.AgentScan.AddJob)
	agScan.PUT("/job/edit", d.AgentScan.EditJob)
	agScan.DELETE("/job/delete", d.AgentScan.DeleteJobs)
	agScan.GET("/job-item/list", d.AgentScan.ListJobItems)
	agScan.GET("/job-item/get", d.AgentScan.GetJobItem)
	agScan.GET("/report/list", d.AgentScan.ListReports)
	agScan.GET("/report/get", d.AgentScan.GetReport)

	// 模型与规则：model（api_key 加密）/ risk-rule
	agMdl := agentGrp.Group("/model")
	agMdl.GET("/list", d.AgentModel.ListModels)
	agMdl.GET("/get", d.AgentModel.GetModel)
	agMdl.POST("/add", d.AgentModel.AddModel)
	agMdl.PUT("/edit", d.AgentModel.EditModel)
	agMdl.DELETE("/delete", d.AgentModel.DeleteModels)
	agMdl.GET("/rule/list", d.AgentModel.ListRules)
	agMdl.GET("/rule/get", d.AgentModel.GetRule)
	agMdl.POST("/rule/add", d.AgentModel.AddRule)
	agMdl.PUT("/rule/edit", d.AgentModel.EditRule)
	agMdl.DELETE("/rule/delete", d.AgentModel.DeleteRules)

	// 治理监控（只读）：invocation / audit-event / cost / risk-hit / audit-log
	agentGrp.GET("/monitor/invocation/list", d.AgentMonitor.ListInvocations)
	agentGrp.GET("/monitor/invocation/get", d.AgentMonitor.GetInvocation)
	agentGrp.GET("/monitor/audit-event/list", d.AgentMonitor.ListAuditEvents)
	agentGrp.GET("/monitor/audit-event/get", d.AgentMonitor.GetAuditEvent)
	agentGrp.GET("/monitor/cost/list", d.AgentMonitor.ListCostRecords)
	agentGrp.GET("/monitor/cost/get", d.AgentMonitor.GetCostRecord)
	agentGrp.GET("/monitor/risk-hit/list", d.AgentMonitor.ListRiskHits)
	agentGrp.GET("/monitor/risk-hit/get", d.AgentMonitor.GetRiskHit)
	agentGrp.GET("/monitor/audit-log/list", d.AgentMonitor.ListAuditLogs)
	agentGrp.GET("/monitor/audit-log/get", d.AgentMonitor.GetAuditLog)
	agentGrp.DELETE("/monitor/audit-log/delete", d.AgentMonitor.DeleteAuditLogs)

	// v2 功能：Team 协作（team/member/task/event/work-item）
	agTeam := agentGrp.Group("/team")
	agTeam.GET("/list", d.AgentTeam.ListTeams)
	agTeam.GET("/get", d.AgentTeam.GetTeam)
	agTeam.POST("/add", d.AgentTeam.AddTeam)
	agTeam.PUT("/edit", d.AgentTeam.EditTeam)
	agTeam.DELETE("/delete", d.AgentTeam.DeleteTeams)
	agTeam.GET("/member/list", d.AgentTeam.ListMembers)
	agTeam.POST("/member/add", d.AgentTeam.AddMember)
	agTeam.PUT("/member/edit", d.AgentTeam.EditMember)
	agTeam.DELETE("/member/delete", d.AgentTeam.DeleteMembers)
	agTeam.GET("/task/list", d.AgentTeam.ListTasks)
	agTeam.POST("/task/dispatch", d.AgentTeam.DispatchTask)
	agTeam.GET("/event/list", d.AgentTeam.ListEvents)
	agTeam.GET("/work-item/list", d.AgentTeam.ListWorkItems)
	agTeam.POST("/work-item/add", d.AgentTeam.AddWorkItem)
	agTeam.PUT("/work-item/edit", d.AgentTeam.EditWorkItem)
	agTeam.DELETE("/work-item/delete", d.AgentTeam.DeleteWorkItems)

	// v2 功能：系统设置（admin 镜像卡片）
	agSet := agentGrp.Group("/settings")
	agSet.GET("/image/list", d.AgentSettings.ListImages)
	agSet.PUT("/image/upsert", d.AgentSettings.UpsertImage)
	agSet.DELETE("/image/delete", d.AgentSettings.DeleteImage)

	// v2 功能：概览
	agentGrp.GET("/dashboard/overview", d.AgentDashboard.Overview)

	// 「打开桌面」反代：公开（无 JWTAuth），路径里的短期 JWT 自证身份。
	// 必须挂在 v1（非 agentGrp）：iframe 的资产/WS 子请求不带 Authorization 头。
	desktopProxy := v1.Group("/agent/instance")
	desktopProxy.GET("/:id/desktop/:token/*any", d.AgentInstance.DesktopProxy)

	// v2 功能：实例（localdocker 真跑容器）
	agInst := agentGrp.Group("/instance")
	agInst.GET("/list", d.AgentInstance.List)
	agInst.GET("/get", d.AgentInstance.Get)
	agInst.POST("/add", d.AgentInstance.Create)
	agInst.POST("/:id/start", d.AgentInstance.Start)
	agInst.POST("/:id/stop", d.AgentInstance.Stop)
	agInst.POST("/:id/restart", d.AgentInstance.Restart)
	agInst.DELETE("/delete", d.AgentInstance.Delete)
	agInst.GET("/:id/status", d.AgentInstance.Status)
	agInst.POST("/:id/exec", d.AgentInstance.Exec)
	agInst.GET("/:id/logs", d.AgentInstance.Logs)
	// 轮换实例专属 LLM 密钥并重发 agent 运行配置（零停机）
	agInst.POST("/:id/refresh-llm-key", d.AgentInstance.RefreshLLMKey)
	// 签发桌面预览的短期路径 token（受 JWTAuth+CasbinAuth 保护，仅 owner 可签）
	agInst.POST("/:id/desktop/access", d.AgentInstance.DesktopAccess)
	// 管理员：全部实例
	agentGrp.GET("/instance/all", d.AgentInstance.ListAll)

	// ===== Phase3 控制面：容器内 agent 心跳+认领（InstanceAuth，instance token 鉴权） =====
	agRuntime := v1.Group("/agent/runtime", middleware.InstanceAuth())
	agRuntime.POST("/heartbeat", d.AgentRuntime.Heartbeat)
	// 实例技能取包：manager 用 instance token 拉取 skill 整包 zip（install_skill 命令的 https 链接指向此处）
	agRuntime.GET("/skill/package", d.SkillPkg.DownloadPackage)

	// Phase3 控制面：管理端命令下发 + 状态查询（JWTAuth + CasbinAuth，复用 agentGrp）
	agInst.POST("/:id/command/enqueue", d.AgentRuntime.EnqueueCommand)
	agInst.GET("/:id/command/list", d.AgentRuntime.ListCommands)
	agInst.GET("/:id/runtime-status", d.AgentRuntime.GetRuntimeStatus)
	agInst.GET("/:id/runtime-status/history", d.AgentRuntime.ListStatusHistory)
	agInst.GET("/:id/config-changes", d.AgentRuntime.ListConfigChanges)
	// 实例技能管理（管理端）：列出已安装 / 下发 install_skill / 下发 uninstall_skill
	agInst.GET("/:id/skill/list", d.AgentRuntime.ListInstanceSkills)
	agInst.POST("/:id/skill/install", d.AgentRuntime.InstallSkill)
	agInst.POST("/:id/skill/uninstall", d.AgentRuntime.UninstallSkill)
	// 实例 MCP 分组绑定（详情抽屉）：绑/解绑自动签 key + 重推 agent 运行配置
	agInst.GET("/:id/mcp/list", d.AgentMcp.List)
	agInst.POST("/:id/mcp/bind", d.AgentMcp.Bind)
	agInst.POST("/:id/mcp/unbind", d.AgentMcp.Unbind)

	// ==== Agent v2：交互式终端（access 在 JWT 组；WS 在公开组，路径 token 自证身份，与桌面同构） ====
	agInst.POST("/:id/tty/access", d.AgentTty.TtyAccess)
	desktopProxy.GET("/:id/tty/:token", d.AgentTty.TtyServeWS)

	// ==== Agent v2：工作区文件管理（owner 校验在 controller；仅 localdocker） ====
	agInst.GET("/:id/workspace/list", d.AgentWorkspace.List)
	agInst.GET("/:id/workspace/download", d.AgentWorkspace.Download)
	agInst.GET("/:id/workspace/audit", d.AgentWorkspace.Audit)
	agInst.POST("/:id/workspace/upload", d.AgentWorkspace.Upload)
	agInst.POST("/:id/workspace/mkdir", d.AgentWorkspace.Mkdir)
	agInst.POST("/:id/workspace/delete", d.AgentWorkspace.Delete)
	agInst.POST("/:id/workspace/rename", d.AgentWorkspace.Rename)

	// ==== Agent v2：持久卷 ====
	agVol := agentGrp.Group("/volume")
	agVol.GET("/list", d.AgentVolume.List)
	agVol.GET("/:id", d.AgentVolume.Get)
	agVol.POST("/create", d.AgentVolume.Create)
	agVol.POST("/delete", d.AgentVolume.Delete)
	agVol.POST("/:id/unbind", d.AgentVolume.Unbind)

	// ==== Agent v2：备份/恢复 + 备份计划（静态段与 :id 同层，gin 静态优先） ====
	agBak := agentGrp.Group("/backup")
	agBak.GET("/list", d.AgentBackup.List)
	agBak.GET("/:id", d.AgentBackup.Get)
	agBak.POST("/create", d.AgentBackup.Create)
	agBak.POST("/delete", d.AgentBackup.Delete)
	agBak.POST("/:id/restore", d.AgentBackup.Restore)
	agBak.GET("/schedule/list", d.AgentBackup.ListSchedules)
	agBak.POST("/schedule/create", d.AgentBackup.AddSchedule)
	agBak.POST("/schedule/:id/edit", d.AgentBackup.EditSchedule)
	agBak.POST("/schedule/delete", d.AgentBackup.DeleteSchedule)

	// ==== Agent v2：滚动发布（进度查询入口惰性推进） ====
	agRol := agentGrp.Group("/runtime/rollout")
	agRol.GET("/list", d.AgentRollout.List)
	agRol.POST("/create", d.AgentRollout.Create)
	agRol.GET("/:id/progress", d.AgentRollout.Progress)
	agRol.POST("/:id/pause", d.AgentRollout.Pause)
	agRol.POST("/:id/resume", d.AgentRollout.Resume)
	agRol.POST("/:id/cancel", d.AgentRollout.Cancel)

	// ==== Agent v2：用量查询（管理端）====
	agentGrp.GET("/usage/list", d.AgentUsage.List)

	// ==== Agent v2：OpenClaw 配置页（契约见 ui api/system/agent/openclaw.ts） ====
	agOc := agentGrp.Group("/openclaw")
	agOc.GET("/configs", d.AgentOpenclawConfig.List)
	agOc.GET("/configs/:instanceId", d.AgentOpenclawConfig.Get)
	agOc.PUT("/configs/:instanceId", d.AgentOpenclawConfig.Save)
	agOc.POST("/configs/validate", d.AgentOpenclawConfig.Validate)

	// ==== Agent v2：Agent 用户管理（契约见 ui api/system/agent/user.ts） ====
	agUser := agentGrp.Group("/user")
	agUser.GET("/list", d.AgentUser.List)
	agUser.POST("/add", d.AgentUser.Add)
	agUser.PUT("/edit", d.AgentUser.Edit)
	agUser.DELETE("/delete", d.AgentUser.Delete)
	agUser.PUT("/reset-password", d.AgentUser.ResetPassword)

	// ==== Agent v2：运行时计量上报（InstanceAuth，与心跳同组；实例身份取 token 不接受自报）====
	agRuntime.POST("/report", d.AgentUsage.Report)

	// ===== 文档管理模块（aa_doc 库，跨库软引用 aa_user） =====
	// 上传即抽取→分块→嵌入；更新原件/改标题摘要自动重嵌；MCP 读写入口在 mcp-ops 服务
	docGrp := v1.Group("/doc", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	dc := docGrp.Group("/document")
	dc.GET("/list", d.Doc.List)
	dc.GET("/get", d.Doc.Get)
	dc.GET("/content", d.Doc.Content)
	dc.GET("/raw", d.Doc.Raw)
	dc.GET("/search", d.Doc.Search)
	dc.GET("/access", d.Doc.Access)
	dc.POST("/upload", d.Doc.Upload)
	dc.POST("/update", d.Doc.Update)
	dc.POST("/move", d.Doc.Move)
	dc.POST("/embed", d.Doc.Embed)
	dc.PUT("/edit", d.Doc.Edit)
	dc.DELETE("/delete", d.Doc.Delete)
	docGrp.GET("/grant/get", d.Doc.GrantGet)
	docGrp.POST("/grant/save", d.Doc.GrantSave)
	// 目录管理（树/新建/重命名/移动/删除/按引用反查——项目空间跳转用）
	dd := docGrp.Group("/dir")
	dd.GET("/list", d.DocDir.List)
	dd.GET("/byref", d.DocDir.ByRef)
	dd.POST("/create", d.DocDir.Create)
	dd.POST("/move", d.DocDir.Move)
	dd.PUT("/rename", d.DocDir.Rename)
	dd.DELETE("/delete", d.DocDir.Delete)

	// ===== 知识库管理模块（aa_kb 库，跨库软引用 aa_user） =====
	// 与文档管理同构的独立实现：上传即抽取→分块→嵌入；MCP 读写入口在 mcp-ops 服务
	kbGrp := v1.Group("/kb", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	kc := kbGrp.Group("/document")
	kc.GET("/list", d.Kb.List)
	kc.GET("/get", d.Kb.Get)
	kc.GET("/content", d.Kb.Content)
	kc.GET("/raw", d.Kb.Raw)
	kc.GET("/search", d.Kb.Search)
	kc.GET("/access", d.Kb.Access)
	kc.POST("/upload", d.Kb.Upload)
	kc.POST("/update", d.Kb.Update)
	kc.POST("/move", d.Kb.Move)
	kc.POST("/embed", d.Kb.Embed)
	kc.PUT("/edit", d.Kb.Edit)
	kc.DELETE("/delete", d.Kb.Delete)
	kbGrp.GET("/grant/get", d.Kb.GrantGet)
	kbGrp.POST("/grant/save", d.Kb.GrantSave)
	// 目录管理（树/新建/重命名/移动/删除/按引用反查——项目空间跳转用）
	kd := kbGrp.Group("/dir")
	kd.GET("/list", d.KbDir.List)
	kd.GET("/byref", d.KbDir.ByRef)
	kd.POST("/create", d.KbDir.Create)
	kd.POST("/move", d.KbDir.Move)
	kd.PUT("/rename", d.KbDir.Rename)
	kd.DELETE("/delete", d.KbDir.Delete)
	// KB 数据源连接器（目录可写者可管；web/数据库/API/对象存储/外部知识库五种接入）
	kds := kbGrp.Group("/datasource")
	kds.GET("/list", d.KbDatasource.List)
	kds.GET("/get", d.KbDatasource.Get)
	kds.POST("/add", d.KbDatasource.Add)
	kds.PUT("/edit", d.KbDatasource.Edit)
	kds.DELETE("/delete", d.KbDatasource.Delete)
	kds.POST("/runs/cancel", d.KbDatasource.CancelRun)
	kds.POST("/:datasourceId/trigger", d.KbDatasource.Trigger)
	kds.GET("/:datasourceId/runs", d.KbDatasource.Runs)

	// ===== 项目管理模块（aa_project 库，跨库软引用 aa_user） =====
	// 注意：/api/v1/llm/project/* 是 LLM 预算「项目」，与这里的业务项目无关。
	pmGrp := v1.Group("/project", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))

	// 项目（顶层资源容器）
	pm := pmGrp.Group("")
	pm.GET("/list", d.Project.List)
	pm.GET("/all", d.Project.All)
	pm.GET("/get", d.Project.Get)
	pm.GET("/overview", d.Project.Overview)
	pm.POST("/add", d.Project.Add)
	pm.PUT("/edit", d.Project.Edit)
	pm.DELETE("/delete", d.Project.Delete)

	// 成员
	pmm := pmGrp.Group("/member")
	pmm.GET("/list", d.ProjectMember.List)
	pmm.POST("/add", d.ProjectMember.Add)
	pmm.PUT("/edit", d.ProjectMember.Edit)
	pmm.DELETE("/delete", d.ProjectMember.Delete)

	// 任务（含子任务 + 评论）
	pmt := pmGrp.Group("/task")
	pmt.GET("/list", d.ProjectTask.List)
	pmt.GET("/get", d.ProjectTask.Get)
	pmt.POST("/add", d.ProjectTask.Add)
	pmt.PUT("/edit", d.ProjectTask.Edit)
	pmt.DELETE("/delete", d.ProjectTask.Delete)
	pmt.PUT("/move", d.ProjectBoard.Move)
	pmt.GET("/comment/list", d.ProjectTask.ListComments)
	pmt.POST("/comment/add", d.ProjectTask.AddComment)
	pmt.DELETE("/comment/delete", d.ProjectTask.DeleteComment)

	// 看板（列 + 任务聚合）
	pmGrp.GET("/board", d.ProjectBoard.Board)

	// 里程碑
	pmMs := pmGrp.Group("/milestone")
	pmMs.GET("/list", d.ProjectMs.List)
	pmMs.GET("/get", d.ProjectMs.Get)
	pmMs.POST("/add", d.ProjectMs.Add)
	pmMs.PUT("/edit", d.ProjectMs.Edit)
	pmMs.DELETE("/delete", d.ProjectMs.Delete)

	// 动态（活动流，只读）
	pmGrp.GET("/activity/list", d.ProjectAct.List)

	// 资源绑定（项目 ↔ Agent/Skill/KB/LLM/MCP）
	pmBd := pmGrp.Group("/binding")
	pmBd.GET("/list", d.ProjectBind.List)
	pmBd.GET("/get", d.ProjectBind.Get)
	pmBd.POST("/add", d.ProjectBind.Add)
	pmBd.PUT("/edit", d.ProjectBind.Edit)
	pmBd.DELETE("/delete", d.ProjectBind.Delete)

	// 设置（标签 / 状态流 / 自定义字段）
	pmCfg := pmGrp.Group("/config")
	pmCfg.GET("/tag/list", d.ProjectCfg.TagList)
	pmCfg.POST("/tag/add", d.ProjectCfg.TagAdd)
	pmCfg.PUT("/tag/edit", d.ProjectCfg.TagEdit)
	pmCfg.DELETE("/tag/delete", d.ProjectCfg.TagDelete)
	pmCfg.GET("/status/list", d.ProjectCfg.StatusList)
	pmCfg.POST("/status/add", d.ProjectCfg.StatusAdd)
	pmCfg.PUT("/status/edit", d.ProjectCfg.StatusEdit)
	pmCfg.DELETE("/status/delete", d.ProjectCfg.StatusDelete)
	pmCfg.GET("/field/list", d.ProjectCfg.FieldList)
	pmCfg.POST("/field/add", d.ProjectCfg.FieldAdd)
	pmCfg.PUT("/field/edit", d.ProjectCfg.FieldEdit)
	pmCfg.DELETE("/field/delete", d.ProjectCfg.FieldDelete)

	// 报表 / 时间线（只读聚合）
	pmGrp.GET("/report/overview", d.ProjectRpt.Overview)
	pmGrp.GET("/timeline/list", d.ProjectTL.List)

	// 日程模块（aa_schedule 库，跨库软引用 aa_user）
	scGrp := v1.Group("/schedule", middleware.JWTAuth(), middleware.CasbinAuth(d.MenuRepo))
	sc := scGrp.Group("")
	sc.GET("/event/list", d.Schedule.List)
	sc.GET("/event/detail", d.Schedule.Detail)
	sc.POST("/event/add", d.Schedule.Add)
	sc.PUT("/event/edit", d.Schedule.Edit)
	sc.DELETE("/event/delete", d.Schedule.Delete)
	sc.POST("/event/exception", d.Schedule.Exception)
	sc.POST("/attendee/respond", d.Schedule.Respond)
	sc.GET("/freebusy", d.Schedule.FreeBusy)
	sc.GET("/freeslots", d.Schedule.FindSlots)
	scRoom := scGrp.Group("/room")
	scRoom.GET("/list", d.Schedule.RoomList)
	scRoom.GET("/overview", d.Schedule.RoomOverview)
	scRoom.POST("/add", d.Schedule.RoomAdd)
	scRoom.PUT("/edit", d.Schedule.RoomEdit)
	scRoom.DELETE("/delete", d.Schedule.RoomDelete)
}
