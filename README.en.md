<div align="center">
  <a href="https://aiarsenal.cc"><img src="docs/logo.png" alt="AI Arsenal Logo" width="320"></a>

  # AI Arsenal

  **Unified Management Platform for Enterprise AI Applications**

  [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)
  [![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
  [![Vue](https://img.shields.io/badge/Vue-3-4FC08D.svg)](https://vuejs.org)
  [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16_·_pgvector-336791.svg)](https://www.postgresql.org)
  [![Tauri](https://img.shields.io/badge/Desktop-Tauri_2-FFC131.svg)](https://tauri.app)
  [![AI](https://img.shields.io/badge/AI-GLM-8B5CF6.svg)](https://aiarsenal.cc)

  🌐 Website [aiarsenal.cc](https://aiarsenal.cc) · Gitee [aiarsenal/aiarsenal](https://gitee.com/aiarsenal/aiarsenal) · GitHub [aiarsenal/aiarsenal](https://github.com/aiarsenal/aiarsenal)

  [简体中文](README.md) · English
</div>

---

AI Arsenal brings **Skills, LLM, MCP, and Agents** together with enterprise knowledge (documents / knowledge bases) under one governance platform: the admin console governs assets, the LLM gateway unifies model access and billing, the MCP gateway opens platform capabilities to any AI client, and the Agent runtime carries the full instance lifecycle — with companion Mac / Windows desktop clients. The platform was built end-to-end with **GLM**, orchestrating and governing all kinds of AI capabilities in one consistent way.

Tech foundation: backend [Gin](https://github.com/gin-gonic/gin) + GORM (DDD four layers), frontend [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin) (Ant Design Vue) — **Gin + Vben**.

## ✨ Highlights

### 🔐 Fine-grained permission control — down to buttons and data rows

- **Three-layer permission model**: directory → page → button, with 13 top-level directories · 68 menu pages · **200 button-level permissions**, all policy-managed via Casbin RBAC;
- **Row-level read / write grants**: on top of three visibility tiers, documents and knowledge bases manage **separate read-grant and write-grant rows** — visible & readable ≠ writable;
- **Per-key model control**: LLM virtual keys restrict available models per key, with three-level budget scoping (key / shared pool / user group) and one-click blocking & rotation;
- **Instance-level credential isolation**: every Agent instance binds its own LLM key with zero-downtime rotation.

### ✂️ Read / write separation across the stack — management and data planes fully decoupled

- **Process level**: the stateless management plane (api) and the stateful data planes (mcp / llm gateways) run as separate processes and ports — restarting the management plane never drops SSE long connections nor stops stdio child processes;
- **Tool level**: platform MCP tools are **registered in pairs** (`platform-doc` / `platform-doc-write`, `platform-kb` / `platform-kb-write` …); whether a caller gets read-only or writable tools is decided by the Bearer key's `access_type` — **agents are read-only by default, write access is granted separately**;
- **Authorization level**: read grants and write grants for documents / knowledge bases are separate rows with separate approval;
- **Database level**: 10 business databases modeled per module (`aa_skill` / `aa_llm` / `aa_mcp` / `aa_agent` …) — data never crosses module boundaries.

### 🌐 Plug and play for any scenario

- **Any model**: the OpenAI-compatible family (deepseek / moonshot / groq / openrouter / ollama …) direct + **native Anthropic `/v1/messages` bidirectional translation** + **native Gemini `/v1beta` bidirectional translation** — enter credentials (AES-256-GCM encrypted) and go;
- **Any client**: the MCP gateway serves Claude Desktop / Cursor and any AI client speaking SSE or streamable-http, with connection testing / trial invocation / onboarding guides;
- **Any runtime**: Agent instances run on local Docker by default, one-line switch to Tencent Cloud CVM via `config/tencent.yaml`;
- **Any platform**: one-command deployment scripts for macOS / Windows / Ubuntu, ready out of the box.

## 🏗️ Architecture Overview

```
┌─────────────┐   ┌─────────────┐   ┌───────────────────────┐
│ ui (Vue 3)  │   │ client      │   │ AI clients            │
│ Admin UI    │   │ (Tauri 2)   │   │ Claude Desktop/Cursor │
└──────┬──────┘   └──────┬──────┘   └───────────┬───────────┘
       │ REST            │ heartbeat / cmds     │ MCP protocol
       ▼                 ▼                      ▼
┌───────────────────────────────────────────────────────┐
│ Management plane  api (:43211) · stateless REST       │
│ JWT + Casbin · DDD · multi-DB GORM · Agent ctrl-plane │
├───────────────────────────────────────────────────────┤
│ Data plane  mcp gateway (:43213)  llm gateway (:43214)│
│ SSE / streamable-http / stdio   OpenAI + Anthropic    │
│ Bearer auth · tool compression  vKey·budget·failover  │
└──────┬────────────────────────────────────┬───────────┘
       ▼                                    ▼
┌──────────────────────┐        ┌────────────────────────┐
│ PostgreSQL × 10 DBs  │        │ Agent runtime          │
│ pgvector + Redis     │        │ Local Docker / CVM     │
└──────────────────────┘        └────────────────────────┘
```

| Service | Location | Default port | Role |
| --- | --- | --- | --- |
| **api** | `api/cmd` | 43211 | Management plane REST: CRUD for all modules, auth, Agent runtime control plane (provisioning / heartbeat / command queue) |
| **mcp gateway** | `api/cmd/mcp` | 43213 | Data plane: MCP protocol endpoint aggregating upstream servers and platform tools, for AI clients |
| **mcp-ops** | `api/cmd/ops` | 43220 | Internal platform capability MCP server: platform-doc / kb / skill / project / ops … (read/write pair registration) |
| **llm gateway** | `llm/cmd` | 43214 | Data plane: OpenAI-compatible + native Anthropic endpoints, virtual-key auth, weighted routing, async billing |
| **ui** | `ui/` | — | Admin console + public landing page (Vue 3 + Ant Design Vue) |
| **client** | `client/` | — | Mac / Windows desktop client (Tauri 2 + Rust) |

> **Why separate gateways?** SSE long connections, resident stdio child processes and session state conflict with the stateless REST model of api — restarting api would drop every session. Hence the data planes run as independent processes, sharing the same data layer and auth code with api (multi-main within the api module).
>
> **Agent runtime SDK**: [aiarsenal/runtime-go](https://github.com/aiarsenal/runtime-go) (separate repository) abstracts Docker / Tencent Cloud CVM adapters; api orchestrates instance lifecycles through it. It also hosts the knowledge-ecosystem integration (the `cmd/docsource` CLI: unified Markdown pulls from Feishu / DingTalk / Tencent Docs / Notion).

## 📦 Core Modules (All 8 Shipped)

> Status: ✅ Shipped. The "Progress" column reflects what the code actually implements, listing **completed → pending** capabilities.

| Module | Status | Progress (Completed → Pending) |
| --- | --- | --- |
| ⚙️ System | ✅ Shipped | Users / roles / menus / departments / posts / permission groups / log monitoring (login + operation + online users) · JWT + Casbin RBAC · button-level permissions — admin foundation complete |
| 🧩 Skill | ✅ Shipped | zip upload & parsing (frontmatter + manifest) · security scanning (banned extensions / secret fingerprints / eval rules; failing review blocks publish & install) · pgvector vector + full-text search · iFlytek marketplace (detail pages / direct-link import + catalog guide; search & sync via MCP tools) |
| 🧠 LLM | ✅ Shipped | Credential / model / virtual-key / budget / usage / health back office + `llm/` gateway · OpenAI-compatible `/v1/chat/completions` + native Anthropic `/v1/messages` (bidirectional translation + passthrough) + native Gemini `/v1beta/models/{model}:generateContent` (bidirectional translation, SSE streaming) · `/v1/embeddings` upstream embeddings (optionally LLM-powered, shared by Skill / Docs / KB retrieval with offline-hash fallback) · cross-credential failover · weighted routing + cooldown + health probing · three-level budget async billing · AES-256-GCM credentials |
| 🔌 MCP | ✅ Shipped | `api/cmd/mcp` gateway (SSE / streamable-http) · Bearer auth · stdio process-group management · tool-result compression · full OAuth client / code / token lifecycle (PKCE S256; secret / code / token stored as sha256 only) · `$smart` vector smart routing (tool-description embeddings + pgvector ivfflat semantic tool selection) · upstream probing / trial calls / client onboarding · internal platform toolset (platform-doc / kb / skill / project / ops … via `api/cmd/ops`) |
| 📑 Docs & Knowledge | ✅ Shipped | doc / kb dual modules: upload (multi-file + zip unpack) / inline preview / text extraction & chunking / embeddings / pgvector semantic search · 3-tier visibility + row-level grants · project / personal space sync (doc + kb) · KB datasource connectors (web / database / API / object storage / external KB, cron-scheduled sync + run history) · agent retrieval via MCP (platform-doc / platform-kb) |
| 📋 Project | ✅ Shipped | Projects / tasks / kanban / milestones / members / reports / timeline / activity · custom fields & status flows · project ↔ Agent / Skill / KB / LLM / MCP bindings · 22 MCP tools |
| 🤖 Agent | ✅ Shipped | Instance lifecycle (local Docker default, optional Tencent CVM provider) · remote desktop portal (noVNC reverse proxy) · interactive terminal (websocket tty) · workspace file management · persistent volumes / backup / rolling release · instance usage reporting & queries · OpenClaw config page & agent-user management · skill-install command queue · heartbeat + config-drift self-healing · MCP group binding · per-instance LLM key rotation · config center / security scanning / model rules / governance monitoring / team collaboration |
| 💻 Desktop Client | ✅ Shipped | Tauri 2 (Mac / Win) · device-code login relay · install / start / stop OpenClaw / Hermes / Obsidian · WeChat QR channel binding (dual-platform dual-path) · device management & banning · agent package cache · Mac desktop node (screen capture / mouse & keyboard control / permission engine / exec approval, ~7,700 lines of Rust) + Windows desktop node (Mac parity: screen capture / mouse & keyboard control / foreground-app gate / exec approval) · CI (api+llm build & test / UI build / cargo check on both ends) and automated dmg / msi release packaging |

> Thirteen top-level directories (System / Monitoring / Dashboard / Audit / Schedule / Docs / Knowledge Base / Skill / LLM / MCP / Agent / Client / Project), 68 menu pages and 200 button-level permissions — 60+ admin pages and 529 backend routes in total.

## 🧰 Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.26 · Gin · GORM · DDD four layers (interfaces / application / domain / infrastructure) |
| Frontend | Vue 3 · [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin) 5.7 (Ant Design Vue) · Vite · vue-i18n (zh + en) |
| Database | PostgreSQL 16 (10 databases, one instance; pgvector on `aa_skill` / `aa_doc` / `aa_kb`) · Redis |
| Desktop | Tauri 2 (Rust + Vue 3, Mac / Windows) |
| Agent runtime | [runtime-go](https://github.com/aiarsenal/runtime-go): local Docker adapter (default) + Tencent Cloud CVM provider (switch via `config/tencent.yaml`) |
| Auth | JWT (golang-jwt/jwt v5) · Casbin RBAC |
| AI | Built end-to-end with GLM |

## 🚀 Quick Start

Three ways to run: **local dev (macOS)** / **local dev (Windows)** / **server deploy (Ubuntu)**. Default ports: api `:43211` · mcp `:43213` · llm `:43214`.

### Prerequisites

| Component | Version | Notes |
| --- | --- | --- |
| Go | ≥ 1.26 | api / mcp / llm services |
| Node + pnpm | Node ≥ 20 (v22 recommended) · pnpm ≥ 10 | ui (vue-vben-admin monorepo) |
| PostgreSQL | 15 / 16 | 10 databases, one instance |
| pgvector | — | vector semantic search (`aa_skill` / `aa_doc` / `aa_kb`) |
| Redis | — | sessions / cache |
| nginx | — | static assets + reverse proxy |
| Docker | — | Agent local runtime (macOS: colima / Docker Desktop; server: docker engine) |
| acme.sh | — | prod server only: wildcard cert via DNS-01 |

### Option 1: Local Development (macOS)

`scripts/local-test.sh` replicates `deploy.sh test` on a Mac: plain HTTP + `/etc/hosts` resolution + three services + nginx — open **http://aiarsenal.cc** in the browser and go.

```bash
# Prerequisites (install once manually): pgvector needs source `make install` into pg15; install Node v22 via nvm
brew install go nginx postgresql@15 redis colima
npm i -g pnpm

bash scripts/local-test.sh prereq     # environment self-check
bash scripts/local-test.sh deploy     # = db-init && build && config && start
```

Step-by-step subcommands are also available: `db-init` / `build` / `config` / `start` / `stop` / `status` / `logs`.

<details>
<summary><b>Three known local pitfalls (auto-handled by the script; expand for troubleshooting)</b></summary>

1. **colima's docker socket is not at the standard path `/var/run/docker.sock`** → the script forwards the current docker context endpoint to api via `DOCKER_HOST` (runtime-go's localdocker defaults to the standard path; it must be set explicitly across OSes).
2. **colima's first start downloads a ~340MB VM image from GitHub**, which easily times out in mainland China → download it via a proxy, then start offline with `colima start --disk-image <file>.raw --force-disk-image`.
3. **nginx runs as nobody by default and cannot traverse macOS home directories** → `config` opens other-traverse permissions on each segment of the repo path, otherwise the landing page 403s.

</details>

### Option 2: Local Development (Windows, Git Bash)

`scripts/local-test-win.sh` mirrors the mac version with the same subcommands (`prereq / pg-init / db-init / build / config / start / stop / status / logs / deploy`). **No Docker required** — api starts with `AIARSENAL_RUNTIME=disabled` (the Agent container runtime is disabled; all other modules unaffected).

```bash
# Prerequisites (install once; unpack/install all under %USERPROFILE%\tools\)
winget install GoLang.Go OpenJS.NodeJS.LTS Rustlang.Rustup
winget install Microsoft.VisualStudio.2022.BuildTools --override "quiet wait norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"
# Go modules: China proxy (all dependencies are public, incl. github.com/aiarsenal/runtime-go; no GOPRIVATE needed)
go env -w GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn
# PostgreSQL 16 (EDB binaries zip) → tools/pgsql; nginx official zip → tools/nginx
# Redis (tporadowski Windows build zip) → tools/redis
# pgvector: in a VS dev shell, nmake /F Makefile.win && nmake /F Makefile.win install (PGROOT → tools/pgsql)
npm i -g pnpm

bash scripts/local-test-win.sh pg-init   # first run: initdb (idempotent skip afterwards)
bash scripts/local-test-win.sh deploy    # = db-init && build && config && start (config prompts UAC once to edit hosts)
```

<details>
<summary><b>Known Windows pitfalls (auto-handled by the script; expand for troubleshooting)</b></summary>

1. **PostgreSQL cannot be started directly from Git Bash** — the MSYS environment crashes backend processes with 0xC0000142; the script launches them via PowerShell `Start-Process` in a clean environment.
2. **ARM64 machines**: winget's arm64 Node crashes rollup's native modules → UI builds must use x64 Node (`tools/node-x64`, downloaded from npmmirror; after switching arch delete `node_modules` and reinstall); low-memory machines (≤4GB) also need `NODE_OPTIONS=--max-old-space-size=2800` (built into the script).
3. **psql defaults to GBK on Chinese Windows** — the script sets `PGCLIENTENCODING=UTF8` to load UTF-8 seeds.
4. **Fresh databases do not replay migrations** — the base SQL is already the latest schema (migration history squashed into the baseline); a brand-new database needs no preset `schema_migrations`; future incremental migrations number from `000001` and take effect naturally.
5. **No Docker**: the hermes client (container runtime) is unavailable and Agent instance operations report "runtime disabled" — expected behavior.

</details>

### Option 3: Server Deployment (Ubuntu)

**Prerequisites**: Ubuntu + the table above + `acme.sh` (installed at `/root/.acme.sh/`); DNS A records for `<domain>` and `api.<domain>` pointing to this machine.

```bash
sudo bash deploy.sh rebuild-db   # first-time DB creation (⚠️ DROPs existing tables — first run or when wiping is OK)
sudo bash deploy.sh test         # staging: plain HTTP + /etc/hosts
sudo bash deploy.sh prod         # production: HTTPS + wildcard certificate
```

Script flow: build UI + api/mcp/llm per environment → install systemd units and restart → prod issues/installs the wildcard certificate → install nginx config and reload.

> **The prod certificate uses manual DNS-01 validation, so the first run needs two passes**: the first run prints the `_acme-challenge` TXT record → add it at your DNS provider → wait 1-2 minutes for propagation → run `sudo bash deploy.sh prod` again to finish issuing and installing.

## 🔑 Secrets & Credentials (Required Reading for Deployment)

The repository **ships no real secrets**: `jwt.encryptKey` / `actor.signKey` in `api/config/config.yaml` and the llm gateway `SECRET_KEY` are dev placeholders; production overrides them via environment variables — `deploy.sh` generates `/etc/aiarsenal/env` (random keys, mode 600) on first deploy, injected into the three systemd units via `EnvironmentFile`:

| Environment variable | Purpose |
| --- | --- |
| `AIARSENAL_JWT_ENCRYPT_KEY` | JWT signing + **AES-256-GCM key for LLM credentials / Agent api_keys** (changing it invalidates existing ciphertext in the DB) |
| `AIARSENAL_ACTOR_SIGN_KEY` | HMAC signing for gateway → ops token exchange (same value for api and mcp) |
| `SECRET_KEY` | llm gateway's own JWT key |

Provider API keys (Alibaba Cloud / OpenAI etc.) never land in git nor in seed SQL — enter them in the admin console after deployment ("LLM Management → Credentials", stored AES-256-GCM encrypted). To rotate: edit `/etc/aiarsenal/env`, then `systemctl restart aiarsenal-api aiarsenal-mcp aiarsenal-llm`.

On startup, api / mcp / ops each check whether `jwt.encryptKey` is still taken from `config.yaml`. If so, each emits one `level=WARN` line carrying an `insecureDefaults` field (currently `[jwt.encryptKey]`) and **do not block startup**. That line means the instance's key comes from the `config.yaml` committed to the repo (the public placeholder by default): when it is reachable from the internet, that key is effectively public and can be abused to sign tokens. Mind the order of operations — changing the key makes existing LLM credentials / Agent api_keys in the DB undecryptable, so an existing deployment must first copy the **currently active** key into `/etc/aiarsenal/env` and restart, re-enter the credentials in the console, and only then rotate to a random value. `deploy.sh` repeats the same warning. Running all three processes on one host produces three identical WARN lines, which is expected.

## 🗃️ Databases & Migrations

- **Multi-database modeling**: 10 business databases on one instance (`aiarsenal` / `aa_skill` / `aa_llm` / `aa_mcp` / `aa_agent` / `aa_client` / `aa_project` / `aa_doc` / `aa_kb` / `aa_schedule`), isolated per module.
- **Migration runner wired in** ([api/infrastructure/db/migrate.go](api/infrastructure/db/migrate.go), golang-migrate + `go:embed`), executed database-by-database on api startup. **No migration files are committed yet** — startup silently skips them with zero side effects; follow the conventions below and files take effect automatically:

| Convention | Description |
| --- | --- |
| Location | [api/infrastructure/db/migrations/](api/infrastructure/db/migrations/)`<db>/NNNNNN_<name>.{up,down}.sql`, `<db>` one of the 10 databases |
| Idempotency | Hard requirement: every migration must guard with `IF EXISTS` / `IF NOT EXISTS` (replays are possible) |
| Current stage | Schema changes directly edit [api/resource/data/](api/resource/data/) DDL + `deploy.sh rebuild-db` (⚠️ DROPs existing tables) |
| Ownership | Migrations run as the non-superuser role `aiarsenal`; before enabling, DB objects must be owned by `aiarsenal` (settled on DB rebuild) |

## 🖥️ Desktop Client

[GitHub Releases](https://github.com/aiarsenal/aiarsenal/releases) ships prebuilt desktop clients (macOS `.dmg` / Windows `.msi`). **Prebuilt packages may only connect to http services**: self-deploy the server (local dev or your own server via `deploy.sh test`) and point the `aiarsenal.cc` domains to that machine via hosts — no official online service is contacted. Full steps and self-building (incl. macOS signing / TCC permissions) in [client/README.md](client/README.md).

## 📁 Repository Structure

```
aiarsenal/
├── api/        Backend (Gin + GORM + DDD) — three mains: api / mcp gateway / mcp-ops
├── ui/         Admin frontend (vue-vben-admin + Ant Design Vue) + public landing page
├── llm/        LLM gateway (separate Go module, OpenAI-compatible + native Anthropic)
├── client/     Desktop client (Tauri 2, Mac / Windows)
├── homepage/   Official static site (GitHub Pages, aiarsenal.cc)
├── deploy/     nginx / systemd templates
├── scripts/    One-command local deploy scripts for macOS / Windows
├── deploy.sh   Server one-command deploy entry
└── docs/       Documentation (MCP onboarding guide etc.)
```

> The Agent runtime SDK lives in a separate repository: [github.com/aiarsenal/runtime-go](https://github.com/aiarsenal/runtime-go). Several external reference repos sit at the top level — read-only references, gitignored, **not part of this project**.

## 🔁 Release Workflow

Local commit → `git push origin master` (origin dual-pushes [Gitee](https://gitee.com/aiarsenal/aiarsenal) and [GitHub](https://github.com/aiarsenal/aiarsenal)) → server `git pull` → re-run `deploy.sh`:

```bash
# local
git push origin master
# server
cd /home/ubuntu/aiarsenal && git pull && sudo bash deploy.sh prod
```

## 🗺️ Roadmap

- **Desktop client**: channels extended to DingTalk / WeCom / Feishu / Telegram (WeChat channel shipped)
- **Desktop node**: complete screen recording / camera capture (screen.record / camera.snap-clip, both platforms)
- **Knowledge ecosystem**: a unified retrieval surface across docsource-pulled Feishu / DingTalk / Tencent Docs / Notion documents and the in-platform Docs / KB

## 🛡️ Auth & Security

- Sessions are JWT-based (HS256), 30-day default validity, configurable. Every login token carries a `jti` registered in a server-side session store: logout and admin-forced sign-off (online users → kick) invalidate the token immediately instead of relying on the client discarding it.
- Every request re-reads the user's current identity (`is_admin` / department / posts / status) from the database: permission changes and account freezes take effect on the next request. The token proves who you are; it no longer carries a 30-day authorization snapshot.
- Permissions are Casbin RBAC: super admin (`is_admin`) bypasses all; regular users are authorized per menu policy, down to button level. Three role-independent server-side gates apply on top:
  - **The super-admin flag can only be granted or revoked by a super admin** (user create/edit included);
  - **No one can change their own admin flag, roles or posts through the user-edit API**; accounts whose target is a super admin can only be edited / disabled / deleted by a super admin;
  - **Agent instance APIs are restricted to the instance owner and super admins** (exec / logs / start / stop / restart / status / detail / key rotation / delete).
- Admin-side password reset only allows a super admin to reset others and anyone to reset themselves; every change is recorded by the operation-log middleware. Self-service password change lives in the personal center.
- LLM credentials and Agent `api_key`s are AES-256-GCM encrypted at rest.
- Business IDs are 32-char strings (`biz_id`) — stable across environments, non-enumerable.

## 👤 Default Account

The baseline seed ships an admin and demo accounts (all with password `aiarsenal123`): **`hi` / `aiarsenal123`** (super admin), `user001` ~ `user100` (demo users). **Change the default password immediately after the first deployment.** Demo data (department tree / users / logs) is entirely fictional and unrelated to any real organization.

The landing page is a pure project introduction (no console entry); the login page is at `<domain>/#/auth/login`.

## 📄 License

This project is open-sourced under the [Apache License 2.0](./LICENSE) (third-party components and trademark notices in [NOTICE](./NOTICE); the `ui/` subtree keeps vue-vben-admin's MIT license).

The copyright holder reserves the right to offer paid secondary development and commercial support on top of its held copyright. For business cooperation, feel free to reach out.

---

<div align="center">

AI Arsenal · Built end-to-end with GLM

</div>
