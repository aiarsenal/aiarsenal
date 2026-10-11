# ops —— 企业内部能力 MCP server

平台内部能力以 MCP 工具暴露给 agent，经网关（`mcp/`，:43213）分组聚合 + Bearer 鉴权后调用。
**业务逻辑与 Web UI 同一套**（api 的 service 层），只是入口不同：本包是 MCP 出入口的薄适配层，
由 `cmd/ops`（api module 第二 main）拉起，常驻 http :43220，自身不做鉴权（只监听 127.0.0.1，
鉴权在网关）。

## 端点（读写分开）

| 端点 | 网关注册名 | 说明 |
|---|---|---|
| `/mcp` | platform-ops | 平台数据只读（实例/设备/技能/MCP 审计/看板监控） |
| `/mcp/doc` | platform-doc | 文档库只读（检索/阅读/列表） |
| `/mcp/doc-write` | platform-doc-write | 文档库写入（新建/更新，自动重嵌） |
| `/mcp/kb` | platform-kb | 知识库只读（检索/阅读/列表） |
| `/mcp/kb-write` | platform-kb-write | 知识库写入（新建/更新，自动重嵌） |
| `/mcp/notice` | platform-notice | 通知只读（我的通知/未读数/收件人解析） |
| `/mcp/notice-write` | platform-notice-write | 通知写入（发通知/标记已读） |
| `/mcp/project` | platform-project | 项目只读（项目/任务/里程碑/看板/动态/成员） |
| `/mcp/project-write` | platform-project-write | 项目写入（建改项目任务/看板移动/评论/里程碑/成员） |
| `/mcp/skill` | platform-skill | 技能只读（检索/版本/市场/安全审计/实例已装） |
| `/mcp/skill-write` | platform-skill-write | 技能写入（市场导入/版本发布下架/实例装卸） |
| `/mcp/user` | platform-user | 员工信息查询（找人/档案/部门树/花名册，只读） |
| `/mcp/budget` | platform-budget | 预算只读（我的预算/临时预算申请列表） |
| `/mcp/budget-write` | platform-budget-write | 临时预算申请写入（申请/通过/驳回） |
| `/mcp/schedule` | platform-schedule | 日程只读（我的日程/共同空闲/会议室） |
| `/mcp/schedule-write` | platform-schedule-write | 日程写入（创建/修改/取消，含周期与单场例外；confirmed 预检确认模式） |

读写分端点 → 网关侧分别注册，分组授权即可只给读不给写（platform-user 为只读单端点，
员工档案由管理端维护）。未装配的域（Deps 里为 nil）对应端点不挂载；看板/监控工具在
装配了 Dashboard/Monitor 时才注册。

## 工具一览（~54 个）

| 端点 | 工具 | 说明 |
|---|---|---|
| platform-ops | `platform_overview` | 跨四库运营总览（实例/设备/技能/MCP 计数、今日调用量） |
| platform-ops | `agent_list` / `agent_detail` | agent 实例列表（带 instance_id）/详情（不含 token 列） |
| platform-ops | `device_list` | 桌面客户端设备列表 |
| platform-ops | `skill_search` | 技能库模糊检索（语义检索在 platform-skill 的 `skill_search`） |
| platform-ops | `mcp_audit` | MCP 网关调用审计 |
| platform-ops | `dashboard_overview` | 我的看板总览（实例/运行中/团队/模型/扫描计数） |
| platform-ops | `invocation_list` | 模型调用记录（token/延迟/成败） |
| platform-ops | `audit_event_list` / `audit_log_list` | 治理审计事件 / 用户操作审计日志 |
| platform-ops | `cost_record_list` / `risk_hit_list` | 成本记录 / 风险命中 |
| platform-doc | `doc_search` | 语义向量 top-k 检索（块级打分带回标题） |
| platform-doc | `doc_read` / `doc_list` | 按 id/标题读全文、关键词列表 |
| platform-doc-write | `doc_create` / `doc_update` | 纯文本入库/更新，自动清洗分块重嵌 |
| platform-kb | `kb_search` / `kb_read` / `kb_list` | 知识库检索/读全文/列表（与文档域同构） |
| platform-kb-write | `kb_create` / `kb_update` | 知识库入库/更新 |
| platform-notice | `notice_list` / `notice_unread_count` | 我的站内通知/未读数（actor 锁定） |
| platform-notice | `notice_user_lookup` | 按姓名/工号解析收件人 user_id |
| platform-notice-write | `notice_send` | 发站内通知（type 枚举 + link 约定校验） |
| platform-notice-write | `notice_mark_read` / `notice_mark_all_read` | 标记已读 |
| platform-project | `project_list` / `project_get` / `project_overview` | 项目列表/详情/概览 |
| platform-project | `project_task_list` / `project_task_get` | 任务列表/详情 |
| platform-project | `project_milestone_list` / `project_milestone_progress` | 里程碑列表/进度 |
| platform-project | `project_board` / `project_timeline_list` | 看板（状态列+任务）/时间线 |
| platform-project | `project_activity_list` / `project_report_overview` / `project_member_list` | 动态流/报表/成员 |
| platform-project-write | `project_create` / `project_update` / `project_delete` | 项目建改删 |
| platform-project-write | `project_task_create` / `project_task_update` / `project_task_move` | 任务建改/看板移动 |
| platform-project-write | `project_task_comment_add` | 任务评论 |
| platform-project-write | `project_milestone_create` / `project_milestone_update` / `project_member_add` | 里程碑/成员 |
| platform-skill | `skill_list` / `skill_get` / `skill_search` | 列表/详情（含溯源：贡献人/原作者/原链接/标签）/语义检索 |
| platform-skill | `skill_version_list` / `skill_namespace_list` | 版本列表（含审计状态）/命名空间 |
| platform-skill | `skill_market_sources` / `skill_market_search` / `skill_market_sites` | 可同步市场源/市场搜索/外部市场目录（只存链接，引导用户去原站搜索拿安装引用） |
| platform-skill | `skill_sec_list` / `instance_skill_list` | 安全审计（五态）/实例已装技能 |
| platform-skill | `skill_install_search` | 搜可安装候选（含审计状态/已装剔除），配合用户确认后安装 |
| platform-skill-write | `skill_market_sync` / `skill_market_import_url` | 市场同步/链接导入（出网；可记原作者/标签溯源） |
| platform-skill-write | `skill_version_publish` / `skill_version_yank` | 版本发布/下架 |
| platform-skill-write | `skill_install_external` | 按外部引用装（@作者/标识 或链接）：导入（落档溯源）→安全扫描（终态双通道通知归属人：站内+数字人）→安全才发布+安装 |
| platform-skill-write | `skill_install` / `skill_uninstall` | 实例装卸技能（异步命令，返回 cmd_id）；install 须 confirmed=true（用户确认门禁） |
| platform-skill-write | `skill_update` / `skill_delete` | 改技能元数据（名称/简介/可见性）/软删除技能 |
| platform-skill-write | `skill_market_site_add` / `skill_market_site_edit` / `skill_market_site_delete` | 外部市场目录增删改（与 Web「其他市场」页同链路） |
| platform-user | `user_lookup` / `user_get` | 姓名/工号/手机/邮箱模糊找人 / user_id 取员工档案 |
| platform-user | `dept_list` / `dept_user_list` | 部门树（含在编人数）/ 部门花名册（可含下级） |
| platform-schedule | `schedule_list` / `schedule_find_slots` / `schedule_rooms` | 我的日程（周期展开）/ 共同空闲档期（工作日 08-20 点）/ 会议室列表 |
| platform-schedule-write | `schedule_create` | 创建日程/会议（周期子集 repeat/byday/count/until；confirmed 预检→确认→执行，冲突可 ignore_conflicts） |
| platform-schedule-write | `schedule_update` / `schedule_cancel` | 修改（旧值打底局部覆盖）/ 取消（整系列或传 date 单场取消；均 confirmed 门禁） |

## 架构位置

- 文档/知识库/技能/项目/通知工具都是 service 薄壳——与 UI 同库同算法，单源。
  项目域的 `*_update` 因 repo Edit 是全量 map 更新，本层统一「先读旧实体→覆盖给定字段→写」。
  员工信息域是纯目录读模型（queryRows，显式列——aa_user 有 user_password/user_salt/
  last_login_ip 等敏感列，禁止带出）。
- 收件人/监控类工具的身份口径：有调用方身份（网关令牌交换）一律锁定 actor 本人；
  系统身份（console 探针/stdio 冒烟）允许显式 user_id——网关路径恒有 actor，安全不降。
- 平台数据工具是 agent 专用聚合读模型（状态翻中文、跨库计数合成），UI 没有对应形态，
  SQL 直接写在工具旁、不入 persistence；列一律显式写出（不 `SELECT *`——实例/设备表
  有 token 类秘密列）。
- aa_doc 连接的 gorm logger 已收 Silent：文档正文是 SQL 绑定参数，防错误/慢查询日志带出。
- 技能市场同步/链接导入从本进程出网（service 层已有 SSRF 防护/限跳/限 10MB）。
- 通知类工具/链路（预算审批、技能扫描结论…）统一走双通道：站内 aa_notice + 同一消息
  channel_notify 下发收件人名下 hermes 实例（数字人转达），实现单源在
  `application/service/channel_notice_service.go`，新增通知类型自带内容即可。

## 加新工具三步

1. 写一个 `func(ctx, a map[string]any) (string, error)`——读模型 `queryRows(dbXxx, ...)` 或调 service
2. 在对应 `build*Server` 里 `opsTool(...)` 注册名字/描述/参数
3. `go build` 后网关侧无需改动——网关按注册表自动发现工具（tools/list 实时反映）

## 注册与部署

- 网关注册行：`aa_mcp.aa_mcp_server`（server_id `mcpops*` 前缀），种子在
  `api/resource/data/mcp.sql` Part C（platform-ops / doc / kb / skill / project /
  notice / user / budget / schedule 各域 server，2026-09 迁移 squash 后一并固化在基线里）。
- 本机：`scripts/local-test.sh`（构建 `api/cmd/ops` → `api/bin/mcp-ops`，cwd 必须是 `api/`）。
- 冒烟：`--transport stdio` 本地起平台数据 server；http 各端点按 streamable-http 协议
  （POST initialize 拿 Mcp-Session-Id → notifications/initialized → tools/call）。

历史：本包原是独立 module `mcp-ops/`（pgx 裸 SQL），文档域嵌入/分块曾逐字复刻 api 侧；
2026-08 并入 api module 消除算法双源；2026-09 补齐技能/项目/通知/员工信息/预算/日程域与看板监控工具。
