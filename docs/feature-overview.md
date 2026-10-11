# AI Arsenal 功能清单

> 来源:菜单种子 `aa_menu`(`api/resource/data/*.sql`)+ UI 视图 `ui/apps/web-antd/src/views/` + API 路由 `api/interfaces/router/router.go`,三者交叉核对。
> 层级:**一级模块**(menu_type=0,侧边栏目录)→ **二级功能**(menu_type=1,页面,组件统一映射到 `system/<模块>/<功能>/index.vue`)。

---

## 一、二级功能总表

| 一级模块 | 二级功能 | 说明 |
|---|---|---|
| 📁 **项目管理** | 📋 项目 | 项目列表 / 概览 / CRUD(进度、成员、状态、起止) |
| | 📝 任务 | 任务 / 子任务 / 评论 / 优先级 / 状态 / 截止 / 标签 / 负责人 |
| | 🔲 看板 | 按状态或自定义列拖拽的看板视图 |
| | 🏁 里程碑 | 里程碑 / 阶段,关联任务并汇总进度 |
| | 🗓️ 时间线 | 甘特图 / 排期视图 |
| | 👥 成员 | 项目成员与角色(负责人/成员/观察者)、邀请 |
| | 📁 文档 | 项目 Wiki / 文档(可关联知识库) |
| | 📊 报表 | 进度 / 燃尽 / 工时 / 成员负载 |
| | 💬 动态 | 活动流 / 讨论通知 |
| | ⚙️ 设置 | 工作流 / 自定义字段 / 标签 / 归档删除 |
| 🤖 **Agent 管理** | 🧩 配置中心 | 资源 / Bundle / Bundle 项 / Bundle 技能 四子资源 CRUD |
| | 🧠 Skill 管理 | 技能 CRUD + blob / 版本 / 扫描结果 |
| | 🛡️ 安全扫描 | 扫描配置 / 任务 / 任务项 / 报告 |
| | ⚙️ 模型与规则 | 模型(api_key 加密)+ 模型规则 CRUD |
| | 📊 治理监控 | 调用 / 审计事件 / 成本 / 风险命中 / 审计日志(只读) |
| | 👥 Team 协作 | 团队 / 成员 / 任务派发 / 事件 / 工作项(v2) |
| | 📈 概览 | Dashboard 总览 |
| | ⚙️ 系统设置 | 管理端镜像卡片增删改 |
| | 📦 运行实例 | localdocker 真实容器:增删 / 启停重启 / 状态 / exec / 日志 / 远程桌面 |
| 🔍 **知识库管理**(新增) | 📊 总览 | KB 数量 / 文档数 / 切片向量数 / 存储 / 检索 QPS 与命中率 / 跨模块引用 |
| | 📚 知识库 | KB 实例 CRUD:类型(文档/网页/FAQ/结构化/外部接入)、嵌入模型、向量库后端、可见性 |
| | 📄 文档 | 文档采集:上传(PDF/DOCX/MD/TXT/HTML/CSV/PPTX/XLSX)、解析切片状态、预览、版本、重解析 |
| | 🔌 数据源 | 连接器:文件上传 / 网页爬取 / 数据库 / API / 对象存储同步 / **外部 KB 服务代理** / 定时同步 |
| | 🧮 切片与向量 | 切片策略(定长/递归/段落/代码感知/Markdown)、重叠、嵌入模型、向量库后端、重排模型 |
| | 🔍 检索测试 | 查询→Top-K(带分数)、混合检索(向量+关键词)、重排、元数据过滤、配置对比 |
| | 🧩 引用与装配 | KB 绑定到 Agent / Skill / Bundle,反向查看"谁在用此 KB" |
| | 🔐 权限与共享 | 可见性(私有/部门/团队/公开)、成员、检索 API Key、分享链接、限流 |
| | 📈 审计与统计 | 检索日志(谁/何时/查询/命中/延迟/成本)、采集日志、按 KB/用户/Agent 统计 |
| | ⚙️ 设置 | 嵌入/重排模型注册(取自 LLM 网关)、向量库连接器、采集并发、保留策略 |
| 💻 **桌面客户端** | 🧩 Agent 配置 | Agent 及版本管理 |
| | 📊 运行状态 | 客户端运行状态 |
| | 🖥️ 设备管理 | 设备列表 + 渠道绑定(微信扫码) + 命令下发/历史 |
| | 📦 安装包管理 | 多平台安装包拉取/删除 |
| 🧩 **Skill 管理** | 🧠 技能管理 | 技能 + 版本/标签 CRUD、发布 / 下架 |
| | 📦 命名空间 | 命名空间 + 成员管理 |
| | 🔍 审核中心 | 审核 + 推广决策 |
| | 🏷️ 标签管理 | 标签 + 多语言翻译 |
| | 🔒 安全审计 | 安全扫描列表 / 重扫 |
| 🤖 **LLM 管理** | 🔑 凭据管理 | 厂商凭据 CRUD |
| | 🧠 模型管理 | 模型部署 CRUD |
| | 🗝️ 密钥管理 | 虚拟密钥 + 封禁 / 轮换 |
| | 💰 预算管理 | 预算 CRUD |
| | 👥 部门配置 | 部门 + 项目 / 终端用户 / 标签 |
| | 📈 用量日志 | 花费记录(只读) |
| | ⚠️ 错误日志 | 调用错误(只读) |
| | 💚 健康检查 | 模型健康(只读) |
| 🔌 **MCP 聚合** | 🖥️ 服务器 | MCP 服务器 CRUD |
| | 🗂️ 分组 | 服务器分组 CRUD |
| | 🗝️ 密钥 | Bearer Key + 封禁 / 轮换 |
| | 📊 调用审计 | 调用活动流水 |
| | 💬 提示词 | 内置 Prompt CRUD |
| | 📦 资源 | 内置 Resource CRUD |
| | ⚙️ 设置 | 网关配置(含按用户) |
| ⚙️ **系统管理** | 📋 菜单管理 | 菜单树 + 权限点 |
| | 👥 角色管理 | 角色 + Casbin 权限分配 |
| | 🏢 部门管理 | 组织部门树 |
| | 💼 岗位管理 | 岗位 CRUD |
| | 👤 用户管理 | 用户 CRUD / 重置密码 / 启停 |
| 📊 **系统监控** | 🖥️ 服务监控 | 服务器 CPU / 内存 / 磁盘 |
| | 📝 登录日志 | 登录记录查询 / 清除 |
| | 📜 操作日志 | 操作记录查询 / 清除 |
| | 🟢 在线用户 | 在线会话列表 / 强退 |

> 共 **9 个一级模块、约 62 个二级页面**。Agent 下另有 OpenClaw 配置 / 用户管理 两个菜单种子已埋但 `is_hide=1`、组件为空(v2 占位,UI 未实现),未计入。LLM / MCP / Agent / Skill / KB 各有独立数据库,软引用系统库 `aa_user`;鉴权分 公共 / JWT / JWT+Casbin / 设备 / 实例 token 五级。

---

## 二、新增模块:知识库管理(Knowledge Base)

### 1. 设计目标

「接入各种各样的知识库」意味着该模块需同时承担两种角色(参考 MCP 模块的「外部聚合 + 内置资源」双层模式):

- **内置 RAG 引擎**:对自有内容(文档 / 网页 / FAQ / 结构化数据)做 采集 → 解析 → 切片 → 嵌入 → 入库 → 检索 的完整管线。
- **外部 KB 聚合**:以「连接器」形式接入第三方 RAG / 搜索服务、已有向量库、企业知识库,不落本地,只做统一检索代理与权限/审计。

并与其他模块协同:嵌入与重排模型取自 **LLM 网关**;KB 可被 **Agent / Skill / Bundle** 引用做 RAG;全程沿用「后端控制路由 + `aa_menu` + Casbin」约定。

### 2. 二级功能表

| 二级功能 | 前端组件 | 核心能力 |
|---|---|---|
| 📊 总览 | `system/knowledgebase/overview` | KB 数 / 文档数 / 切片向量总数 / 存储占用 / 检索 QPS 与命中率 / 最近采集任务 / 跨模块引用关系 |
| 📚 知识库 | `system/knowledgebase/base` | KB 实例 CRUD;类型(文档库/网页库/FAQ 库/结构化库/**外部接入库**);默认嵌入模型、向量库后端、切片策略、可见性、归属部门 |
| 📄 文档 | `system/knowledgebase/document` | 多格式上传(PDF/DOCX/MD/TXT/HTML/CSV/PPTX/XLSX)、解析与切片状态、预览、版本、重解析、批量导入、文档级元数据与过滤标签 |
| 🔌 数据源 | `system/knowledgebase/datasource` | 连接器:文件上传 / 网页爬取(URL·sitemap·增量)/ 数据库(SQL→行切片)/ API 拉取 / 对象存储同步(S3·OSS)/ **外部 KB 服务代理** / 定时同步计划 |
| 🧮 切片与向量 | `system/knowledgebase/chunk` | 切片策略(定长/递归/段落/代码感知/Markdown 感知)、重叠、嵌入模型选择、向量库后端(pgvector/Milvus/Qdrant/Chroma/Weaviate/ES)、重排模型、重建索引、索引统计 |
| 🔍 检索测试 | `system/knowledgebase/retrieval` | 查询→Top-K(带分数)、混合检索(向量+BM25/关键词)、重排、元数据过滤、多配置 A/B 对比、跨内置+外部 KB 联合检索 |
| 🧩 引用与装配 | `system/knowledgebase/binding` | KB 绑定到 Agent / Skill / Bundle(含每绑定检索参数)、反向查看引用方、解绑 |
| 🔐 权限与共享 | `system/knowledgebase/permission` | 可见性(私有/部门/团队/公开)、成员管理、检索 API Key(供 Agent/外部调用)、分享链接、限流 |
| 📈 审计与统计 | `system/knowledgebase/audit` | 检索日志(谁/何时/查询/命中数/延迟/Token 成本)、采集日志、按 KB/用户/Agent 维度统计、错误日志 |
| ⚙️ 设置 | `system/knowledgebase/config` | 嵌入/重排模型注册(从 LLM 网关拉取)、向量库连接器配置、采集并发与重试、数据保留与清理策略 |

### 3. 关键设计点

1. **双层架构**:KB 的「类型」字段决定走内置管线还是外部代理。外部接入库只存连接配置(endpoint / 鉴权 / 检索参数),检索时透传,统一在「检索测试」与对外 API 返回标准化结果。
2. **数据源即连接器**:每类数据源一个 provider 实现,新增类型只需加 provider;支持定时增量同步与失败重试,采集过程异步、可观测(采集日志)。
3. **切片与向量可独立演进**:同一 KB 可换嵌入模型 / 向量库后端,触发「重建索引」而非删库重来;向量存储与元数据存储分离。
4. **与 LLM 网关衔接**:嵌入模型、重排模型一律从 LLM 模块已注册的模型中选择,密钥复用网关凭据,用量计入 LLM 成本统计。
5. **与 Agent / Skill 衔接**:通过「引用与装配」生成绑定关系,Agent 运行时按绑定做 RAG 检索;KB 是一等公民,可被多个 Agent / Skill 共享。
6. **遵循现有约定**:`aa_menu` 种子 + Casbin 授权;独立数据库(建议名 `knowledgebase` 或 `aa_kb`),软引用系统库 `aa_user`;API 前缀 `/api/v1/kb/*`,JWT + Casbin 鉴权(检索对外 API 用 KB API Key)。

### 4. 建议(落地参考,非定稿)

- **数据表(沿用 `aa_` 前缀、id + 业务 id 双主键)**:`aa_kb`(知识库)、`aa_kb_document`(文档)、`aa_kb_chunk`(切片 + 向量引用)、`aa_kb_datasource`(数据源/连接器)、`aa_kb_binding`(引用装配)、`aa_kb_member`(成员)、`aa_kb_apikey`(检索 API Key)、`aa_kb_retrieval_log`(检索审计)。
- **路由前缀**:`/api/v1/kb/{base,document,datasource,chunk,retrieval,binding,permission,audit,config}`。
- **菜单种子**:新增到 `api/resource/data/knowledgebase.sql`,一级目录 `🔍 知识库管理` + 10 个二级页面 + 各页按钮权限点(menu_type=2),并授予超管角色。

---

## 三、新增模块:项目管理(Project Management)

### 1. 设计目标

在本平台中,「项目」既是常规的项目协作空间(任务/看板/里程碑/甘特),更是**顶层资源容器**:一个项目可归属或绑定一组 AI 资源(Agent / Skill / 知识库 / LLM 预算与密钥 / MCP 分组),实现按项目隔离、配额、计费与可见性。项目管理因此不只是通用看板,而是把平台的 AI 能力按业务项目组织起来的一层。

### 2. 二级功能表(基础功能)

| 二级功能 | 前端组件 | 核心能力 |
|---|---|---|
| 📋 项目 | `system/project/list` | 项目列表 / 概览 / CRUD:进度、成员数、状态、起止时间、归属 |
| 📝 任务 | `system/project/task` | 任务 / 子任务 / 评论、优先级、状态、截止、标签、负责人 |
| 🔲 看板 | `system/project/board` | 按状态或自定义列拖拽的看板视图 |
| 🏁 里程碑 | `system/project/milestone` | 里程碑 / 阶段,关联任务,进度汇总 |
| 🗓️ 时间线 | `system/project/timeline` | 甘特图 / 排期视图 |
| 👥 成员 | `system/project/member` | 项目成员与角色(负责人/成员/观察者)、邀请 |
| 📁 文档 | `system/project/wiki` | 项目 Wiki / 文档(可关联知识库) |
| 📊 报表 | `system/project/report` | 进度 / 燃尽 / 工时 / 成员负载 |
| 💬 动态 | `system/project/activity` | 活动流 / 讨论通知 |
| ⚙️ 设置 | `system/project/config` | 工作流 / 自定义字段 / 标签 / 归档删除 |

### 3. 关键设计点

1. **项目 = 顶层资源容器**:项目可绑定 Agent / Skill / 知识库 / LLM 预算与密钥 / MCP 分组,形成按项目的资源隔离、配额与计费边界——这是 PM 在本平台的核心价值,否则只是通用看板。
2. **任务联动 AI**:任务可关联或触发 Agent 运行、引用项目知识库做上下文,产出物挂回任务;任务评论支持 @ Agent。与 Agent / 知识库模块双向打通。
3. **项目级成员与权限**:独立于系统 RBAC 的项目角色(负责人/成员/观察者),软引用系统库 `aa_user`。
4. **多视图同源**:列表 / 看板 / 甘特 / 日历 共享同一份任务数据,只是不同渲染视图。
5. **遵循现有约定**:`aa_menu` 种子 + Casbin;建议独立数据库(如 `project`)或置系统库;API 前缀 `/api/v1/project/*`;一级菜单 weigh=32,排在所有业务模块最前。

### 4. 建议(落地参考,非定稿)

- **数据表(沿用 `aa_` 前缀、id + 业务 id 双主键)**:`aa_project`(项目)、`aa_project_member`(成员)、`aa_project_task`(任务)、`aa_project_task_comment`(评论)、`aa_project_task_activity`(动态)、`aa_project_milestone`(里程碑)、`aa_project_worklog`(工时)、`aa_project_wiki`(文档)、`aa_project_binding`(绑定的 AI 资源:agent/skill/kb/llm/mcp)、`aa_project_tag` / `aa_project_custom_field`(标签/自定义字段)。
- **路由前缀**:`/api/v1/project/{list,task,board,milestone,timeline,member,wiki,report,activity,config}`。
- **菜单种子**:新增到 `api/resource/data/project.sql`,一级目录 `📁 项目管理` + 10 个二级页面 + 各页按钮权限点(menu_type=2),并授予超管角色。
