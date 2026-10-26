<div align="center">
  <a href="https://aiarsenal.cc"><img src="docs/logo.png" alt="AI Arsenal Logo" width="320"></a>

  # AI Arsenal

  **企业级 AI 应用一体化管理平台**

  [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)
  [![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
  [![Vue](https://img.shields.io/badge/Vue-3-4FC08D.svg)](https://vuejs.org)
  [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16_·_pgvector-336791.svg)](https://www.postgresql.org)
  [![Tauri](https://img.shields.io/badge/Desktop-Tauri_2-FFC131.svg)](https://tauri.app)
  [![AI](https://img.shields.io/badge/AI-GLM-8B5CF6.svg)](https://aiarsenal.cc)

  🌐 官网 [aiarsenal.cc](https://aiarsenal.cc) · Gitee [aiarsenal/aiarsenal](https://gitee.com/aiarsenal/aiarsenal) · GitHub [aiarsenal/aiarsenal](https://github.com/aiarsenal/aiarsenal)

  简体中文 · [English](README.en.md)
</div>

---

AI Arsenal 将 **Skill、LLM、MCP、Agent** 与企业知识（文档 / 知识库）统一纳入一个平台治理：管理后台治理资产，LLM 网关统一模型接入与计费，MCP 网关把平台能力开放给任意 AI 客户端，Agent 运行时承载实例全生命周期，并配套 Mac / Windows 桌面客户端。平台由 **GLM** 全程参与编写，以一致的方式编排和治理各类 AI 能力。

技术底座：后端 [Gin](https://github.com/gin-gonic/gin) + GORM（DDD 四层），前端 [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin)（Ant Design Vue）—— **Gin + Vben**。

## ✨ 平台特色

### 🔐 细腻的权限管理 —— 细到按钮、细到数据行

- **三层权限模型**：目录 → 页面 → 按钮逐级管控，13 个功能目录 · 68 个菜单页 · **200 项按钮级权限**，全部经 Casbin RBAC 策略化管理；
- **行级读 / 写授权**：文档、知识库在「三档可见性」之上再按**读授权与写授权分行**管理——可见即可读 ≠ 可写；
- **Key 级模型管控**：LLM 虚拟 Key 按 Key 限定可用模型、三级预算圈人（Key / 共享池 / 用户权限组）、一键封禁与轮换；
- **实例级凭据隔离**：每个 Agent 实例绑定独立 LLM Key，支持零停机轮换。

### ✂️ 全链路读写分离设计 —— 管理面与数据面彻底解耦

- **进程级**：无状态管理面（api）与有状态数据面（mcp / llm 网关）分进程、分端口部署，管理面重启不断 SSE 长连接、不停 stdio 子进程；
- **工具级**：平台能力 MCP 工具**成对注册**（`platform-doc` / `platform-doc-write`、`platform-kb` / `platform-kb-write` …），调用方能拿到只读还是可写工具由 Bearer Key 的 `access_type` 决定——**Agent 默认只读，写操作单独授权**；
- **授权级**：文档 / 知识库的读授权与写授权独立成行、独立审批；
- **库级**：10 个业务库分库建模（`aa_skill` / `aa_llm` / `aa_mcp` / `aa_agent` …），模块间数据互不穿透。

### 🌐 任意场景，即插即用

- **任意模型**：OpenAI 兼容族（deepseek / moonshot / groq / openrouter / ollama …）直连 + **Anthropic 原生 `/v1/messages` 双向翻译** + **Gemini 原生 `/v1beta` 双向翻译**，凭据 AES-256-GCM 加密录入即可用；
- **任意客户端**：MCP 网关同时服务 Claude Desktop / Cursor 及任意支持 SSE 或 streamable-http 的 AI 客户端，另附连接测试 / 试调用 / 接入引导；
- **任意运行时**：Agent 实例默认本机 Docker，`config/tencent.yaml` 一键切换腾讯云 CVM；
- **任意平台**：macOS / Windows / Ubuntu 三端一键部署脚本开箱即用。

## 🏗️ 架构总览

```
┌─────────────┐   ┌─────────────┐   ┌───────────────────────┐
│ ui (Vue 3)  │   │ client      │   │ AI 客户端              │
│ 管理后台+落地页│   │ (Tauri 2)   │   │ Claude Desktop/Cursor │
└──────┬──────┘   └──────┬──────┘   └───────────┬───────────┘
       │ REST            │ 心跳/命令/上报          │ MCP 协议
       ▼                 ▼                      ▼
┌───────────────────────────────────────────────────────┐
│ 管理面  api (:43211)  无状态 REST                        │
│ JWT + Casbin · DDD 四层 · 多库 GORM · Agent 控制面       │
├───────────────────────────────────────────────────────┤
│ 数据面  mcp 网关 (:43213)        llm 网关 (:43214)       │
│ SSE / streamable-http / stdio   OpenAI 兼容 + Anthropic │
│ Bearer 鉴权 · 工具结果压缩        虚拟Key · 预算 · failover│
└──────┬────────────────────────────────────┬───────────┘
       ▼                                    ▼
┌──────────────────────┐        ┌────────────────────────┐
│ PostgreSQL × 10 库    │        │ Agent 运行时            │
│ pgvector + Redis     │        │ 本机 Docker / 腾讯云 CVM │
└──────────────────────┘        └────────────────────────┘
```

| 服务 | 代码位置 | 默认端口 | 职责 |
| --- | --- | --- | --- |
| **api** | `api/cmd` | 43211 | 管理面 REST：全部模块 CRUD、鉴权、Agent 运行时控制面（实例下发 / 心跳 / 命令队列） |
| **mcp 网关** | `api/cmd/mcp` | 43213 | 数据面：MCP 协议端点，聚合上游 server 与平台工具，面向 AI 客户端 |
| **mcp-ops** | `api/cmd/ops` | 43220 | 平台内部能力 MCP server：platform-doc / kb / skill / project / ops …（读写分离注册） |
| **llm 网关** | `llm/cmd` | 43214 | 数据面：OpenAI 兼容 + Anthropic 原生端点、虚拟 Key 鉴权、路由轮询、异步计费 |
| **ui** | `ui/` | — | 管理后台 + 免登录落地页（Vue 3 + Ant Design Vue） |
| **client** | `client/` | — | Mac / Windows 桌面客户端（Tauri 2 + Rust） |

> **为什么拆网关**：SSE 长连接、stdio 子进程常驻、会话状态与 api 的无状态 REST 模型冲突——api 重启会断所有会话。故数据面独立进程部署，与 api 共享同一套数据层与鉴权代码（api module 多 main 形态）。
>
> **Agent 运行时 SDK**：[aiarsenal/runtime-go](https://github.com/aiarsenal/runtime-go)（独立仓库），抽象 Docker / 腾讯云 CVM 适配器，api 经它编排实例生命周期；另含知识生态接入（`cmd/docsource` CLI：飞书 / 钉钉 / 腾讯文档 / Notion 统一拉取为 Markdown）。

## 📦 核心能力（八大模块，全部已上线）

> 状态：✅ 已上线。「进度」列以代码实现度为准，给出**已完成 → 待完善**的关键能力。

| 模块 | 状态 | 进度（已完成 → 待完善） |
| --- | --- | --- |
| ⚙️ 系统管理 | ✅ 已上线 | 用户 / 角色 / 菜单 / 部门 / 岗位 / 权限分组 / 日志监控（登录 + 操作 + 在线用户）· JWT + Casbin RBAC · 按钮级权限 — 后台基座完备 |
| 🧩 Skill 管理 | ✅ 已上线 | zip 上传解析（frontmatter + manifest）· 安全扫描（封禁扩展名 / 密钥指纹 / eval 规则，未过审拦截发布与安装）· pgvector 向量 + 全文检索 · 讯飞第三方市场（详情页 / 直链导入 + 目录引导，搜索与同步经 MCP 工具） |
| 🧠 LLM 管理 | ✅ 已上线 | 凭据 / 模型 / 虚拟 Key / 预算 / 用量 / 健康后台 + `llm/` 网关 · OpenAI 兼容 `/v1/chat/completions` + Anthropic 原生 `/v1/messages`（双向翻译 + 透传）+ Gemini 原生 `/v1beta/models/{model}:generateContent`（双向翻译，含 SSE 流式）· `/v1/embeddings` 上游嵌入（可配 LLM embedding 供 Skill / 文档 / 知识库检索共用，离线哈希兜底）· 跨凭据 failover · 轮询 + cooldown + 健康探测 · 三级预算异步计费 · 凭据 AES-256-GCM 加密 |
| 🔌 MCP 管理 | ✅ 已上线 | `api/cmd/mcp` 网关（SSE / streamable-http 双协议）· Bearer 鉴权 · stdio 子进程组管理 · 工具结果压缩 · OAuth 客户端 / 授权码 / 令牌全生命周期（PKCE S256，secret / code / token 仅存 sha256）· `$smart` 向量智能路由（工具描述嵌入 + pgvector ivfflat 语义选工具）· 上游探测 / 试调用 / 客户端接入引导 · 企业内部能力工具集（platform-doc / kb / skill / project / ops 等，经 `api/cmd/ops`） |
| 📑 文档与知识库 | ✅ 已上线 | doc / kb 双模块：上传（多文件 + zip 解包）/ 在线预览 / 文本抽取分块 / 向量嵌入 / pgvector 语义检索 · 三档可见性 + 行级授权 · 项目 / 个人空间同步（doc + kb）· KB 数据源连接器（web / 数据库 / API / 对象存储 / 外部知识库，cron 定时同步 + 运行记录）· 经 MCP（platform-doc / platform-kb）供 Agent 检索 |
| 📋 项目管理 | ✅ 已上线 | 项目 / 任务 / 看板 / 里程碑 / 成员 / 报表 / 时间线 / 动态 · 自定义字段与状态流 · 项目 ↔ Agent / Skill / KB / LLM / MCP 绑定 · 22 个 MCP 工具 |
| 🤖 Agent 管理 | ✅ 已上线 | 实例生命周期（本机 Docker 默认，腾讯云 CVM provider 可选）· 远程桌面门户（noVNC 反代）· 交互式终端（websocket tty）· 工作区文件管理 · 持久卷 / 备份 / 滚动发布 · 实例用量上报与查询 · OpenClaw 配置页与 Agent 用户管理 · 技能安装命令队列 · 心跳 + 配置漂移自愈 · MCP 分组绑定 · 实例 LLM Key 轮换 · 配置中心 / 安全扫描 / 模型规则 / 治理监控 / Team 协作 |
| 💻 客户端 App | ✅ 已上线 | Tauri 2（Mac / Win）· device-code 登录中转 · 装 / 启停 OpenClaw / Hermes / Obsidian · 微信渠道扫码绑定（双端双路径）· 设备管理与封禁 · Agent 包缓存 · Mac 桌面节点（截屏 / 控鼠键 / 权限引擎 / 执行审批，约 7700 行 Rust）+ Windows 桌面节点（对齐 Mac：截屏 / 控鼠键 / 前台应用闸门 / 执行审批）· CI（api+llm 构建测试 / UI 构建 / 双端 cargo check）与 dmg / msi 自动打包发布 |

> 十三个顶级功能目录（系统 / 监控 / 看板 / 审核 / 日程 / 文档 / 知识库 / Skill / LLM / MCP / Agent / 客户端 / 项目），68 个菜单页、200 项按钮级权限，合计 60+ 个管理页面、529 条后端路由。

## 🧰 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.26 · Gin · GORM · DDD 四层（interfaces / application / domain / infrastructure） |
| 前端 | Vue 3 · [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin) 5.7（Ant Design Vue）· Vite · vue-i18n（zh + en） |
| 数据库 | PostgreSQL 16（10 库同实例，`aa_skill` / `aa_doc` / `aa_kb` 启用 pgvector）· Redis |
| 桌面端 | Tauri 2（Rust + Vue 3，Mac / Windows） |
| Agent 运行时 | [runtime-go](https://github.com/aiarsenal/runtime-go)：本机 Docker 适配器（默认）+ 腾讯云 CVM provider（`config/tencent.yaml` 切换） |
| 鉴权 | JWT（golang-jwt/jwt v5）· Casbin RBAC |
| AI | GLM 全程参与编写 |

## 🚀 快速开始

三种运行方式：**本地开发（macOS）** / **本地开发（Windows）** / **服务器部署（Ubuntu）**。服务默认端口：api `:43211` · mcp `:43213` · llm `:43214`。

### 运行环境

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | ≥ 1.26 | api / mcp / llm 三个服务 |
| Node + pnpm | Node ≥ 20（推荐 v22）· pnpm ≥ 10 | ui（vue-vben-admin monorepo） |
| PostgreSQL | 15 / 16 | 10 库同实例 |
| pgvector | — | 向量语义检索（`aa_skill` / `aa_doc` / `aa_kb`） |
| Redis | — | 会话 / 缓存 |
| nginx | — | 静态资源 + 反代 |
| Docker | — | Agent 本机运行时（macOS 用 colima / Docker Desktop；服务器装 docker engine） |
| acme.sh | — | 仅 prod 服务器：通配符证书 DNS-01 签发 |

### 方式一：本地开发（macOS）

`scripts/local-test.sh` 复刻 `deploy.sh test` 到 Mac：纯 HTTP + `/etc/hosts` 本机解析 + 三个服务 + nginx，浏览器开 **http://aiarsenal.cc** 即用。

```bash
# 前置（手动装一次）：pgvector 需源码 make install 进 pg15；Node 用 nvm 装 v22
brew install go nginx postgresql@15 redis colima
npm i -g pnpm

bash scripts/local-test.sh prereq     # 前置自检
bash scripts/local-test.sh deploy     # = db-init && build && config && start
```

也可分步执行：`db-init` / `build` / `config` / `start` / `stop` / `status` / `logs`。

<details>
<summary><b>本地三个已知坑（脚本已自动处理，展开供排查）</b></summary>

1. **colima 的 docker socket 不在标准路径 `/var/run/docker.sock`** → 脚本把当前 docker context 的端点经 `DOCKER_HOST` 透传给 api（runtime-go 的 localdocker 默认连标准路径，跨 OS 须显式指定）。
2. **colima 首次启动要从 GitHub 下载 ~340MB VM 镜像**，国内易超时 → 可经代理下到本地后 `colima start --disk-image <file>.raw --force-disk-image` 离线启动。
3. **nginx 默认以 nobody 跑，无法穿越 macOS 家目录** → `config` 会放开 repo 路径各段的 other 穿越权限，否则首页 403。

</details>

### 方式二：本地开发（Windows，Git Bash）

`scripts/local-test-win.sh` 与 mac 版同构，子命令一致（`prereq / pg-init / db-init / build / config / start / stop / status / logs / deploy`）。**无需 Docker**——api 以 `AIARSENAL_RUNTIME=disabled` 启动（Agent 容器运行时功能停用，其余模块不受影响）。

```bash
# 前置（手动装一次，均解压/安装到 %USERPROFILE%\tools\）
winget install GoLang.Go OpenJS.NodeJS.LTS Rustlang.Rustup
winget install Microsoft.VisualStudio.2022.BuildTools --override "quiet wait norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"
# Go 模块：国内代理（依赖均已公开，含 github.com/aiarsenal/runtime-go，无需 GOPRIVATE）
go env -w GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn
# PostgreSQL 16（EDB binaries zip）→ tools/pgsql；nginx 官方 zip → tools/nginx
# Redis（tporadowski Windows 版 zip）→ tools/redis
# pgvector：VS 开发环境下 nmake /F Makefile.win && nmake /F Makefile.win install（PGROOT 指向 tools/pgsql）
npm i -g pnpm

bash scripts/local-test-win.sh pg-init   # 首次：initdb（之后幂等跳过）
bash scripts/local-test-win.sh deploy    # = db-init && build && config && start（config 弹一次 UAC 改 hosts）
```

<details>
<summary><b>Windows 已知坑（脚本已自动处理，展开供排查）</b></summary>

1. **PostgreSQL 不能从 Git Bash 直起**——MSYS 环境会让后端进程 0xC0000142 崩溃，脚本经 PowerShell `Start-Process` 干净环境拉起。
2. **ARM64 机型**：winget 装的 arm64 Node 触发 rollup 原生模块崩溃 → UI 构建须走 x64 Node（`tools/node-x64`，npmmirror 下载；换 arch 后删 `node_modules` 重装）；小内存机器（≤4GB）还需 `NODE_OPTIONS=--max-old-space-size=2800`（脚本已内置）。
3. **中文 Windows 的 psql 默认 GBK**——脚本设 `PGCLIENTENCODING=UTF8` 灌 UTF-8 种子。
4. **fresh 库不重放迁移**——base SQL 即最新结构（历史迁移已 squash 固化进基线），全新库无需预置 `schema_migrations`；后续增量迁移从 `000001` 起编号，自然生效。
5. **无 Docker**：hermes 客户端（容器运行时）不可用，Agent 实例操作报「runtime disabled」——预期行为。

</details>

### 方式三：服务器部署（Ubuntu）

**前置**：Ubuntu + 上表运行环境 + `acme.sh`（装到 `/root/.acme.sh/`）；DNS A 记录 `<域名>` 与 `api.<域名>` 指向本机。

```bash
sudo bash deploy.sh rebuild-db   # 首次建库（⚠️ 会 DROP 现有表，仅首次或愿意清空时用）
sudo bash deploy.sh test         # 测试环境：纯 HTTP + /etc/hosts
sudo bash deploy.sh prod         # 生产环境：HTTPS + 通配符证书
```

脚本流程：按环境构建 UI + api/mcp/llm 三服务 → 安装 systemd unit 并重启 → prod 签发/安装通配符证书 → 安装 nginx 配置并 reload。

> **prod 证书为 DNS-01 手动验证，首次需跑两次**：第一次运行输出 `_acme-challenge` TXT 记录 → 到 DNS 服务商添加 → 等 1-2 分钟生效 → 再次 `sudo bash deploy.sh prod` 完成签发与安装。

## 🔑 密钥与凭据（部署必读）

仓库内 **不携带任何真实密钥**：`api/config/config.yaml` 的 `jwt.encryptKey` / `actor.signKey`、llm 网关 `SECRET_KEY` 均为开发占位值，生产用环境变量覆盖——`deploy.sh` 首次部署会生成 `/etc/aiarsenal/env`（随机密钥，600 权限），三个 systemd unit 经 `EnvironmentFile` 注入：

| 环境变量 | 用途 |
| --- | --- |
| `AIARSENAL_JWT_ENCRYPT_KEY` | JWT 签名 + **LLM 凭据 / Agent api_key / 双因子 TOTP 密钥的 AES-256-GCM 密钥**（换值会使库内既有密文失效，已绑定双因子的账号全部无法登录，需超管逐个重置双因子） |
| `AIARSENAL_ACTOR_SIGN_KEY` | 网关 → ops 令牌交换 HMAC 签名（api 与 mcp 同值） |
| `SECRET_KEY` | llm 网关自身 JWT 密钥 |

供应商 API Key（阿里云 / OpenAI 等）不落 git、不入种子 SQL——部署后在管理后台「LLM 管理 → 凭据」录入（AES-256-GCM 加密落库）。轮换密钥：改 `/etc/aiarsenal/env` 后 `systemctl restart aiarsenal-api aiarsenal-mcp aiarsenal-llm`。

启动时 api / mcp / ops 会各自检查 `jwt.encryptKey` 是否仍取自 `config.yaml`：是则各输出一条 `level=WARN` 日志（带 `insecureDefaults` 字段，当前值为 `[jwt.encryptKey]`），**不阻断启动**。出现这条日志说明该实例的密钥写在随源码提交的 `config.yaml` 里（默认值即公开占位值），公网可达时该密钥等同公开，存在被冒用签发 token 的风险。处理顺序要注意：换密钥会使库内既有 LLM 凭据 / Agent api_key / 双因子 TOTP 密钥无法解密（后者意味着已绑定双因子的账号全部无法登录，恢复需超管逐个重置双因子），存量部署应先把**当前正在用的**密钥原值写入 `/etc/aiarsenal/env` 再重启，待后台重录凭据后才轮换成随机值——`deploy.sh` 对这一步有同样的提醒。同一台机同时跑三个进程时会看到三条相同的 WARN，属正常。

## 🗃️ 数据库与迁移

- **多库建模**：10 个业务库同实例（`aiarsenal` / `aa_skill` / `aa_llm` / `aa_mcp` / `aa_agent` / `aa_client` / `aa_project` / `aa_doc` / `aa_kb` / `aa_schedule`），模块间数据隔离。
- **迁移运行器已接入**（[api/infrastructure/db/migrate.go](api/infrastructure/db/migrate.go)，golang-migrate + `go:embed`），api 启动自动逐库执行。**当前仓库未提交迁移文件**——启动时静默跳过、零副作用；按约定补文件即自动生效：

| 约定 | 说明 |
| --- | --- |
| 位置 | [api/infrastructure/db/migrations/](api/infrastructure/db/migrations/)`<db>/NNNNNN_<name>.{up,down}.sql`，`<db>` 为 10 库之一 |
| 幂等 | 硬要求：每条迁移须 `IF EXISTS` / `IF NOT EXISTS` 守卫（可能被重放） |
| 当前阶段 | schema 变更直接改 [api/resource/data/](api/resource/data/) 建表 SQL + `deploy.sh rebuild-db`（⚠️ DROP 现有表） |
| 所有权 | 迁移以非超管角色 `aiarsenal` 执行，启用前须确保库表归 `aiarsenal` 所有（重建库时收口） |

## 🖥️ 桌面客户端

[GitHub Releases](https://github.com/aiarsenal/aiarsenal/releases) 提供编译好的桌面客户端（macOS `.dmg` / Windows `.msi`）。**预编译包只允许连接 http 服务**：需自部署服务端（本地开发或自有服务器 `deploy.sh test` 均可）并配置 hosts 把 `aiarsenal.cc` 系域名解析到该机器，不连接任何官方线上服务。完整步骤与自编译（含 macOS 签名 / TCC 权限说明）见 [client/README.md](client/README.md)。

## 📁 仓库结构

```
aiarsenal/
├── api/        后端（Gin + GORM + DDD）—— api / mcp 网关 / mcp-ops 三个 main
├── ui/         管理后台前端（vue-vben-admin + Ant Design Vue）+ 免登录落地页
├── llm/        LLM 网关（独立 Go module，OpenAI 兼容 + Anthropic 原生）
├── client/     桌面客户端（Tauri 2，Mac / Windows）
├── homepage/   官网静态站（GitHub Pages，aiarsenal.cc）
├── deploy/     nginx / systemd 模板
├── scripts/    macOS / Windows 本地一键部署脚本
├── deploy.sh   服务器一键部署入口
└── docs/       文档（MCP 接入指南等）
```

> Agent 运行时 SDK 为独立仓库：[github.com/aiarsenal/runtime-go](https://github.com/aiarsenal/runtime-go)。顶层另有若干外部参考仓库，仅作只读参考、已 gitignore，**不属于本项目**。

## 🔁 代码上线工作流

本地提交 → `git push origin master`（origin 双推 [Gitee](https://gitee.com/aiarsenal/aiarsenal) 与 [GitHub](https://github.com/aiarsenal/aiarsenal)）→ 服务器 `git pull` → 重跑 `deploy.sh`：

```bash
# 本地
git push origin master
# 服务器
cd /home/ubuntu/aiarsenal && git pull && sudo bash deploy.sh prod
```

## 🗺️ 功能路线图

- **客户端 App**：渠道扩展至钉钉 / 企微 / 飞书 / Telegram（微信渠道已上线）
- **桌面节点**：补齐录屏 / 摄像头采集（screen.record / camera.snap-clip，双端）
- **知识生态**：docsource 拉取的飞书 / 钉钉 / 腾讯文档 / Notion 文档与平台内文档 / 知识库统一检索面

## 🛡️ 鉴权与安全

- 登录态基于 JWT（HS256），默认有效期 30 天，可配置。每个登录令牌带 `jti` 并登记进服务端会话注册表：退出登录、管理员吊销（在线用户 → 强制下线）后令牌立即失效，不再依赖前端丢弃。
- 每个请求按 `userId` 回查库内最新身份（`is_admin` / 部门 / 岗位 / 状态）：改权限、封号在下一跳即生效，token 只证明"你是谁"，不携带 30 天不变的授权。
- 权限基于 Casbin RBAC：超管（`is_admin`）全通，普通用户按菜单策略授权，细至按钮级。服务端另有三条与角色无关的硬闸门：
  - **超管标志只能由超管授予或收回**（含用户编辑接口与建档）；
  - **不得借用户编辑接口修改自己的管理员标志、角色与岗位**；目标是超管的账号仅超管可编辑 / 停用 / 删除；
  - **Agent 实例接口仅归属人与超管可操作**（exec / logs / start / stop / restart / status / 详情 / 密钥轮换 / 删除）。
- 密码重置（管理端入口）仅允许超管重置他人、任何人重置本人；每次口令变更经操作日志中间件留痕。个人自助改密走个人中心。
- **口令存储与强度**：
  - 存储：PBKDF2 框架，串格式 `$pbkdf2-<算法>$<迭代>$<盐>$<派生键>` 自描述——调整 `password.hash.algorithm / iterations` 只影响之后新产生的口令，存量按串内参数照常校验。SM3 为按 GB/T 32905-2016 的本地实现（国标官方测试向量锁定单测，未引入第三方国密库），整个功能零新增依赖；默认档 `pbkdf2-sm3@60000`（单次派生实测 ≈90ms，Apple M1 Pro），`pbkdf2-sha512@220000` 对齐 OWASP 口令存储速查表。这是登录延迟与爆破成本的显式权衡：离线爆破者用自己的优化实现（约快 3.5 倍），60000 迭代的每猜测成本约为 210000 的三成——如需对齐 OWASP 迭代量级，上调到 210000（本机 ≈300ms/次）即可，即时生效且不影响存量校验。
  - 目标部署机复测定档：`go test ./infrastructure/auth/ -bench=HashPasswordSM3 -benchtime=5x`，单次落在 50~100ms 即可；偏低就上调 `password.hash.iterations` 并把实测值回填本节。
  - 强度三档（`password.policy.level`）：`loose` 仅限长度 ≥8；`standard`（默认，等保基线）≥8 位 + 大小写/数字/特殊字符至少三类 + 不含账号名 + 禁弱口令黑名单（内嵌 SecLists Top-1000）；`strict` ≥12 位 + 四类全含。后端强制，拒绝文案逐条给出所缺项。
  - 口令 pepper（可选，防拖库的最后一块板）：配置 `AIARSENAL_PWD_PEPPER`（≥32 字节随机值，多实例必须同值）后，新口令在派生前先与该密钥做 HMAC 混合（串内算法 id 变为 `*-peppered`，存量串不受开关影响）。只拖数据库的攻击者缺这把密钥**无法离线枚举**任何口令——迭代数强弱不再影响这个最常见的场景；只有连应用服务器一起失守才退回拼迭代数。移除/换值会使全部 peppered 口令不可校验（等同强制全员重置），启动时对短值 WARN、对真值误提交进 yaml 也会告警。误配窗口内的登录失败会计入锁定：恢复/换 pepper 后账号若报「已锁定」，清除 redis 键 `pwd:fail:*` 或等锁窗过期即可。
  - 双因子（TOTP 动态码，可选/可强制）：登录后 `/personal/totp/setup` 获取密钥与 otpauth 绑定 URI（验证器 App 手动录入）→ `/personal/totp/activate` 验一次动态码激活；停用需动态码；丢手机由超管 `PUT /system/user/totp/reset` 重置。密钥 AES-256-GCM 加密落库，动态码错误计入登录失败锁定。`totp.enforced: true` 后未绑定账号登录被拒——与口令组合即满足等保三级鉴别 d 项（两种鉴别技术、其一为密码技术）。
  - 口令定期更换（可选）：`password.policy.rotationDays`（0=不启用，默认；等保三级建议 90）。启用后过期口令登录被拒（提示联系管理员重置，管理员重置即解除）；存量库升级与全新安装都会把起算点回填为当前时间——所有账号（含种子演示账号）统一获得完整更换窗口，不自动过期任何账号；种子口令仍是公开值，靠「首次部署立即修改」纪律与强度策略兜底。
  - 登录失败锁定（`password.lockout.*`）：连续 5 次失败锁定 30 分钟（锁定时长自达到阈值时满额起算，到期自动解锁），成功登录清零，阈值/时长可配。Redis **运行中断连**时锁定层放行以保登录可用——与会话注册表的 fail-closed 是两处有意为之的不同取舍（前者是加固层，后者是鉴权本体）；注意 redis 在服务启动时不可达则进程无法启动，属既有启动语义，与本层取舍无关。
  - **硬切换**：md5 旧口令路径已删除。md5 时代的存量账号升级后将无法登录，升级前必须重置全部账号口令：普通账号由超管在管理端逐个重置，超管本人口令丢失时在库内将其 `user_password` 换成新格式串（用新版本代码对期望口令调用 `auth.HashPassword` 生成）。
- LLM 凭据、Agent 模型 `api_key` 走 AES-256-GCM 加密落库。
- 业务 ID 采用 32 位字符串（`biz_id`），跨环境稳定、不可枚举。

## 👤 默认账号

基线种子内置管理员与演示账号（密码均为 `aiarsenal123`）：**`hi` / `aiarsenal123`**（超管）、`user001` ~ `user100`（演示用户）。**首次部署后请立即修改默认密码**。种子口令是公开的夹具值，有意不经强度策略（策略只约束改密接口）——也正因此，改密之后 `aiarsenal123` 无法再被设回。演示数据（部门树 / 用户 / 日志）均为虚构，与任何真实组织无关。

首页落地页为纯项目介绍（无控制台入口），登录地址 `<域名>/#/auth/login`。

## 📄 协议

本项目基于 [Apache License 2.0](./LICENSE) 开源（第三方组件清单与商标声明见 [NOTICE](./NOTICE)；`ui/` 子树沿用底座 vue-vben-admin 的 MIT 协议）。

公司保留在自持版权基础上提供收费二次开发与商业支持服务的权利。如需商业合作，欢迎联系。

---

<div align="center">

AI Arsenal · 全栈基于 GLM 构建

</div>
