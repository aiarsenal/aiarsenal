#!/usr/bin/env bash
# ============================================================================
# AI Arsenal 本机（Windows / Git Bash）测试环境部署脚本
#   把 scripts/local-test.sh 的行为复刻到 Windows：纯 HTTP + hosts 本机解析 +
#   aiarsenal.cc 域名 + 三服务 + nginx，浏览器开 http://aiarsenal.cc 即用。
#
#   与 mac 版的关键差异：
#     - PostgreSQL / Redis / nginx 是本机自解压安装（默认 %USERPROFILE%\tools\），
#       非 brew；postgres 由脚本经「干净的 PowerShell 环境」拉起（从 Git Bash 直接起会导致
#       后端进程 0xC0000142 DLL 初始化崩溃——MSYS 环境/句柄污染，PostgreSQL on Windows 已知坑）
#     - ARM64 机型注意：rollup 的 win32-arm64 原生模块在 vite build 渲染 chunks 阶段稳定崩溃
#       （0xC0000005）→ 本脚本 PATH 优先 %USERPROFILE%\tools\node-x64（x64 Node 走模拟层，
#       node_modules 须由 x64 node 安装过一次，见 build 步骤说明）
#     - 无 Docker：api 以 AIARSENAL_RUNTIME=disabled 启动（runtimenop 空实现），
#       Agent 实例相关操作不可用，其余模块不受影响
#     - 日志统一进仓库 .run/logs（AIARSENAL_LOG_DIR，免 /var/log 需管理员）
#     - 进程树回收用 taskkill（unix 版进程组强杀已按平台拆分进 *_unix.go/*_windows.go）
#
#   前置（手动装，本脚本不管，见 README「Windows 部署」）：
#     - winget：Go ≥1.26（GoLang.Go）/ Node（OpenJS.NodeJS，日常用）
#     - %USERPROFILE%\tools\pgsql    ← EDB postgresql-16.x-windows-x64-binaries.zip 解压
#     - %USERPROFILE%\tools\nginx    ← nginx.org 官方 zip 解压
#     - %USERPROFILE%\tools\redis    ← tporadowski/redis v5.0.14.1 zip 解压（GitHub Releases，
#       直连被墙走 gh-proxy.com 镜像；Memurai 的 MSI 在 ARM64 装不上 1603）
#     - %USERPROFILE%\tools\node-x64 ← npmmirror 的 node-v22-win-x64.zip 解压 + 其 npm i -g pnpm（ARM64 机需要）
#     - pgvector（VS BuildTools nmake 编译进 pgsql；aa_skill/aa_doc/aa_kb 需要，缺了则三库降级）
#     - go env -w GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn（依赖均已公开，无需 GOPRIVATE）
#
#   用法：
#     bash scripts/local-test-win.sh prereq    # 检查前置是否就绪
#     bash scripts/local-test-win.sh pg-init   # 首次：initdb 建数据目录（幂等跳过）
#     bash scripts/local-test-win.sh db-init   # 建 role + 10 库（api/resource/data/*.sql）+ GRANT
#     bash scripts/local-test-win.sh build     # 构建 UI(build:test) + api/mcp/llm 二进制
#     bash scripts/local-test-win.sh config    # hosts（弹 UAC）+ nginx 配置
#     bash scripts/local-test-win.sh start     # 起 postgres/redis 校验 → 5 个服务 → nginx
#     bash scripts/local-test-win.sh stop      # 停 5 个服务 + nginx（postgres/redis 不停）
#     bash scripts/local-test-win.sh status    # 端口/进程状态
#     bash scripts/local-test-win.sh logs      # tail -f 服务日志
#     bash scripts/local-test-win.sh deploy    # = db-init && build && config && start
#
#   访问：浏览器开 http://aiarsenal.cc（hosts 已解析到 127.0.0.1）
# ============================================================================
set -euo pipefail

# ---------- 路径 / 常量 ----------
REPO="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS="${AIARSENAL_TOOLS:-$HOME/tools}"
PG_HOME="$TOOLS/pgsql"
PGDATA="$TOOLS/pgsql-data"
PG_BIN="$PG_HOME/bin"
PSQL="$PG_BIN/psql.exe"
PGCTL="$PG_BIN/pg_ctl.exe"
NGINX_HOME="$TOOLS/nginx"
REDIS_HOME="$TOOLS/redis"
NGINX="$(cygpath -m "$NGINX_HOME")"   # 混合路径 C:/...，nginx/psql 等 Windows 程序用
PID_DIR="$REPO/.run"
LOG_ROOT="$REPO/.run/logs"
DATA_DIR="$REPO/data/client/packages"
SQL_DIR="$REPO/api/resource/data"
HOSTS_FILE="/c/Windows/System32/drivers/etc/hosts"

# 工具链进 PATH。node-x64 优先：ARM64 机的 arm64 Node 会触发 rollup 原生模块崩溃，
# vite 构建一律走 x64 模拟层（node_modules 必须由同一 arch 的 node 安装——换 arch 后先删掉重装）。
export PATH="$PG_BIN:$TOOLS/node-x64:$TOOLS/redis:$HOME/AppData/Roaming/npm:/c/Program Files/nodejs:/c/Program Files/Go:$PATH"
# 国内拉 Go 模块（依赖均已公开，无需 GOPRIVATE）
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"

DBS=(aiarsenal aa_skill aa_llm aa_mcp aa_agent aa_client aa_project aa_doc aa_kb aa_schedule)
API_PORT=43211; MCP_PORT=43213; LLM_PORT=43214; DEMO_PORT=43219; OPS_PORT=43220

# ---------- 辅助 ----------
log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
err()  { printf '\033[31m错误:\033[0m %s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }
wpath() { cygpath -m "$1"; }   # /c/x → C:/x

pg_pw() { cat "$PG_HOME/.superpw" 2>/dev/null; }
export PGPASSWORD="$(pg_pw)"
# SQL 种子是 UTF-8；中文 Windows 的 psql 默认 client_encoding=GBK，会把中文种子转挂
export PGCLIENTENCODING=UTF8

pg_running()  { "$PGCTL" -D "$(wpath "$PGDATA")" status >/dev/null 2>&1; }
redis_ping()  { "$REDIS_HOME/redis-cli.exe" -h 127.0.0.1 -p 6379 ping 2>/dev/null | grep -q PONG; }
port_up()     { timeout 1 bash -c "echo > /dev/tcp/127.0.0.1/$1" 2>/dev/null; }
wait_port()   { local p=$1 i; for i in $(seq 1 40); do port_up "$p" && return 0; sleep 1; done; return 1; }

# PostgreSQL 必须从干净环境拉起：Git Bash 的 MSYS 环境/句柄会让后端进程
# 在首个连接到达时 0xC0000142（DLL 初始化失败）崩溃。经 PowerShell Start-Process 隔离。
ensure_postgres() {
  if pg_running; then log "postgres 已在运行"; else
    log "启动 postgres（经 PowerShell 干净环境——Git Bash 直起会 0xC0000142）"
    # 不用 -Wait：Start-Process -Wait 会连子进程一起等，postgres.exe 是 pg_ctl 的常驻
    # 子进程 → 永不返回（2026-09-29 实测 start 卡死 15min+）。就绪判断交给下方 psql 轮询。
    powershell -NoProfile -Command "Start-Process -FilePath '$(wpath "$PGCTL")' -ArgumentList '-D','$(wpath "$PGDATA")','-l','$(wpath "$PGDATA/server.log")','-w','start' -WindowStyle Hidden" || { err "postgres 启动失败（看 $PGDATA/server.log）"; return 1; }
  fi
  local i; for i in $(seq 1 30); do
    PGPASSWORD="$(pg_pw)" "$PSQL" -h 127.0.0.1 -U postgres -tAc "SELECT 1" >/dev/null 2>&1 && { log "postgres :5432 ✓"; return 0; }
    sleep 1
  done
  err "postgres 连不上（密码文件 $PG_HOME/.superpw 是否还在？）"; return 1
}

ensure_redis() {
  if redis_ping; then log "redis :6379 ✓"; else
    [ -x "$REDIS_HOME/redis-server.exe" ] || { err "redis 未就绪（$REDIS_HOME 缺 redis-server.exe）"; return 1; }
    log "启动 redis（$REDIS_HOME，控制台进程）"
    ( cd "$REDIS_HOME" && nohup ./redis-server.exe --port 6379 >/dev/null 2>&1 & )
    local i; for i in $(seq 1 10); do redis_ping && { log "redis :6379 ✓"; return 0; }; sleep 1; done
    err "redis 启动失败"; return 1
  fi
}

# ---------- 子命令 ----------

cmd_prereq() {
  log "前置检查"
  have go && go version | grep -qE 'go1\.(2[6-9]|[3-9])' && echo "  Go: $(go version)" || err "Go ≥1.26 未就绪（winget install GoLang.Go）"
  have node && echo "  node: $(node --version)" || err "node 未装（winget install OpenJS.NodeJS.LTS）"
  have pnpm && echo "  pnpm: $(pnpm --version)" || err "pnpm 未装（npm i -g pnpm）"
  [ -x "$PSQL" ] && echo "  psql: $("$PSQL" --version)" || err "psql 未就绪（EDB pg16 binaries zip 解压到 $PG_HOME）"
  pg_running && echo "  postgres: 运行中" || echo "  postgres: 未运行（start 时会自动起）"
  redis_ping && echo "  redis: :6379 ✓" || echo "  redis: 未运行（start 时会自动起）"
  [ -x "$NGINX_HOME/nginx.exe" ] && echo "  nginx: $NGINX_HOME" || err "nginx 未就绪（nginx.org zip 解压到 $NGINX_HOME）"
  "$PSQL" -h 127.0.0.1 -U postgres -d aa_skill -tAc "SELECT 1 FROM pg_extension WHERE extname='vector'" 2>/dev/null | grep -q 1 \
    && echo "  pgvector: 已装" || echo "  pgvector: 未见（aa_skill/aa_doc/aa_kb 三库将降级）"
  have cargo && echo "  rust: $(rustc --version)" || echo "  rust: 未装（仅 client 打包需要：winget install Rustlang.Rustup）"
}

cmd_pg_init() {
  if [ -f "$PGDATA/PG_VERSION" ]; then log "数据目录已存在（$PGDATA），跳过 initdb"; return 0; fi
  log "initdb（超管 postgres，密码写入 $PG_HOME/.superpw）"
  local pw; pw="$(openssl rand -hex 8 2>/dev/null || echo aiarsenal$(date +%s))"
  printf '%s' "$pw" > "$PG_HOME/.superpw"
  ( cd "$PG_HOME" && ./bin/initdb.exe -D "$(wpath "$PGDATA")" -U postgres --pwfile="$(wpath "$PG_HOME/.superpw")" -E UTF8 --locale=C ) \
    && log "initdb 完成" || { err "initdb 失败"; return 1; }
}

cmd_db_init() {
  log "数据库初始化（用 $SQL_DIR/*.sql）"
  ensure_postgres
  # pgvector 探测：aa_skill 要 CREATE EXTENSION vector，缺扩展则三库降级（跳过建表，跑通主链路）
  local have_vector=0
  if "$PSQL" -h 127.0.0.1 -U postgres -d postgres -tAc "SELECT 1 FROM pg_available_extensions WHERE name='vector'" 2>/dev/null | grep -q 1; then
    have_vector=1
  fi
  # 1. role（幂等）
  if ! "$PSQL" -h 127.0.0.1 -U postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='aiarsenal'" | grep -q 1; then
    "$PSQL" -h 127.0.0.1 -U postgres -c "CREATE ROLE aiarsenal LOGIN PASSWORD 'aiarsenal123';"
  else log "role aiarsenal 已存在"; fi
  # 2. aiarsenal 库（其余库由各自 SQL 的 \gexec 创建）
  "$PSQL" -h 127.0.0.1 -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aiarsenal'" | grep -q 1 \
    || "$PSQL" -h 127.0.0.1 -U postgres -c "CREATE DATABASE aiarsenal;"
  # 注意：aiarsenal.sql 不含 \connect，必须用 -d aiarsenal 连进去加载（否则表会建到 postgres 库）
  "$PSQL" -h 127.0.0.1 -U postgres -d aiarsenal -v ON_ERROR_STOP=1 -f "$(wpath "$SQL_DIR/aiarsenal.sql")" >/dev/null && log "aiarsenal.sql ✓"
  # 3. aa_skill：先建库 + vector 扩展，再跑 skills.sql（建表用到 vector 类型）
  "$PSQL" -h 127.0.0.1 -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aa_skill'" | grep -q 1 \
    || "$PSQL" -h 127.0.0.1 -U postgres -c "CREATE DATABASE aa_skill;"
  if [ "$have_vector" = 1 ]; then
    "$PSQL" -h 127.0.0.1 -U postgres -d aa_skill -c "CREATE EXTENSION IF NOT EXISTS vector;"
    "$PSQL" -h 127.0.0.1 -U postgres -v ON_ERROR_STOP=1 -f "$(wpath "$SQL_DIR/skills.sql")" >/dev/null && log "skills.sql ✓"
  else
    err "pgvector 未装，跳过 skills.sql（aa_skill 降级；装好 pgvector 后重跑 db-init 即补齐）"
  fi
  # 4. 其余库（schedule.sql 自带 \connect aa_schedule 建库 + 末尾 \connect aiarsenal 灌菜单）
  for f in llm mcp agent client project schedule; do
    "$PSQL" -h 127.0.0.1 -U postgres -v ON_ERROR_STOP=1 -f "$(wpath "$SQL_DIR/$f.sql")" >/dev/null && log "$f.sql ✓"
  done
  # 5. aa_doc / aa_kb（与 aa_skill 同构，需要 vector）
  for pair in "aa_doc:doc" "aa_kb:kb"; do
    db="${pair%%:*}"; f="${pair##*:}"
    "$PSQL" -h 127.0.0.1 -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$db'" | grep -q 1 \
      || "$PSQL" -h 127.0.0.1 -U postgres -c "CREATE DATABASE $db;"
    if [ "$have_vector" = 1 ]; then
      "$PSQL" -h 127.0.0.1 -U postgres -d "$db" -c "CREATE EXTENSION IF NOT EXISTS vector;"
      "$PSQL" -h 127.0.0.1 -U postgres -v ON_ERROR_STOP=1 -f "$(wpath "$SQL_DIR/$f.sql")" >/dev/null && log "$f.sql ✓"
    else
      err "pgvector 未装，跳过 $f.sql（$db 降级）"
    fi
  done
  # 6.（2026-09 squash 后已无迁移文件可预置）base SQL 即最新结构，fresh 库不建 schema_migrations，
  #    后续增量迁移从 000001 起编号、api 启动时自动生效——约定见 api/infrastructure/db/migrations/README.md。
  # 7. GRANT + OWNER 收口（表/序列由超管建成，api 以 aiarsenal 连接；
  #    OWNER 必须归 aiarsenal——迁移系统 ALTER TABLE 要求所有者，GRANT 不够）
  for db in "${DBS[@]}"; do
    "$PSQL" -h 127.0.0.1 -U postgres -d "$db" -c "GRANT ALL ON SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL TABLES IN SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO aiarsenal;" >/dev/null
    "$PSQL" -h 127.0.0.1 -U postgres -d "$db" -tAc "SELECT 'ALTER TABLE ' || quote_ident(tablename) || ' OWNER TO aiarsenal;' FROM pg_tables WHERE schemaname='public' AND tableowner <> 'aiarsenal'" | "$PSQL" -h 127.0.0.1 -U postgres -d "$db" >/dev/null
    "$PSQL" -h 127.0.0.1 -U postgres -d "$db" -tAc "SELECT 'ALTER SEQUENCE ' || quote_ident(sequencename) || ' OWNER TO aiarsenal;' FROM pg_sequences WHERE schemaname='public' AND sequenceowner <> 'aiarsenal'" | "$PSQL" -h 127.0.0.1 -U postgres -d "$db" >/dev/null
  done
  log "数据库初始化完成（10 库 + vector(若有) + GRANT）"
}

cmd_build() {
  log "构建 UI（build:test）"
  ( cd "$REPO/ui" && pnpm install --frozen-lockfile 2>/dev/null || pnpm install )
  ( cd "$REPO/ui" && pnpm --filter @vben/web-antd build:test )
  rm -rf "$REPO/ui/dist" && mkdir -p "$REPO/ui/dist" && cp -r "$REPO/ui/apps/web-antd/dist/." "$REPO/ui/dist/"
  log "构建 Go 服务（api/mcp/llm/mcp-ops/mcp-demo，Windows 出 .exe）"
  ( cd "$REPO/api" && go build -o bin/main.exe ./cmd ) && log "api ✓"
  ( cd "$REPO/llm" && go build -o bin/llm.exe ./cmd ) && log "llm ✓"
  # mcp / mcp-ops / mcp-demo 都是 api module 的 main（同一套数据层与鉴权逻辑，入口不同）
  ( cd "$REPO/api" && go build -o bin/mcp.exe ./cmd/mcp ) && log "mcp ✓"
  ( cd "$REPO/api" && go build -o bin/mcp-ops.exe ./cmd/ops ) && log "mcp-ops ✓"
  ( cd "$REPO/api" && go build -o bin/mcp-demo.exe ./cmd/mcp-demo ) && log "mcp-demo ✓"
  # 种子里 stdio demo 的命令是相对路径 bin/mcp-demo：网关 cwd=api/ 会解析到 ../mcp/bin/，
  # Go 的 LookPath 在 Windows 自动补 .exe → 放到 ../mcp/bin/mcp-demo.exe 即命中，无需改 SQL
  mkdir -p "$REPO/mcp/bin" && cp -f "$REPO/api/bin/mcp-demo.exe" "$REPO/mcp/bin/mcp-demo.exe"
}

cmd_config() {
  log "系统配置"
  # hosts（需管理员：弹一次 UAC 提权执行）。经脚本文件而非内联命令——bash→powershell→powershell
  # 三层转义会把 Add-Content 的参数弄坏（实测静默不写入），-File 方式最稳。
  if ! grep -q 'aiarsenal\.cc' "$HOSTS_FILE" 2>/dev/null; then
    log "hosts 需要管理员权限，弹 UAC 授权框（请点「是」）……"
    local ps1="$TOOLS/add-hosts.ps1"
    mkdir -p "$TOOLS"
    printf '%s\n' \
      '$hosts = "$env:SystemRoot\System32\drivers\etc\hosts"' \
      'if (-not (Select-String -Path $hosts -Pattern "aiarsenal\.cc" -Quiet)) {' \
      '  Add-Content -Path $hosts -Value "127.0.0.1 aiarsenal.cc api.aiarsenal.cc mcp.aiarsenal.cc llm.aiarsenal.cc"' \
      '}' \
      > "$ps1"
    powershell -NoProfile -Command "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','$(wpath "$ps1")'"
    ipconfig //flushdns >/dev/null 2>&1 || true
    grep -q 'aiarsenal\.cc' "$HOSTS_FILE" && log "hosts 已加 aiarsenal.cc 解析" || { err "hosts 追加失败（可手动编辑 $HOSTS_FILE）"; return 1; }
  else log "hosts 已有 aiarsenal.cc"; fi
  # 日志目录
  mkdir -p "$LOG_ROOT"/{api,mcp,llm}
  log "日志目录 $LOG_ROOT 就绪"
  # nginx：conf 由代码内 test.conf sed 路径得到（仅替换 /home/ubuntu/aiarsenal → 本仓）。
  # Windows zip 的 nginx.conf 没有 conf.d include 也没有 mime.types 路径问题 → 生成一份精简主配置。
  sed "s#/home/ubuntu/aiarsenal#$(wpath "$REPO")#g" "$REPO/deploy/nginx/test.conf" \
    > "$NGINX_HOME/conf/aiarsenal.cc.conf"
  cat > "$NGINX_HOME/conf/nginx.conf" <<'EOF'
worker_processes  1;
error_log  logs/error.log;
pid        logs/nginx.pid;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;
    sendfile      on;
    keepalive_timeout  65;
    include aiarsenal.cc.conf;
}
EOF
  ( cd "$NGINX_HOME" && ./nginx.exe -t ) && log "nginx 配置校验通过（→ $NGINX_HOME/conf/aiarsenal.cc.conf）"
}

svc_running() {  # svc_running <name>：按 .run/<name>.winpid 探活
  local wpid
  [ -f "$PID_DIR/$1.winpid" ] || return 1
  wpid=$(cat "$PID_DIR/$1.winpid")
  tasklist //FI "PID eq $wpid" 2>/dev/null | grep -q "$wpid"
}

# winpid_of <exe名>：tasklist CSV 里取第一个匹配的 Windows PID。
# 不用 MSYS ps -W——它看不见 nohup/后台拉起的进程（main.exe/nginx.exe 实测全漏），
# tasklist 是 Windows 原生枚举，无此盲区。
winpid_of() {
  tasklist //FI "IMAGENAME eq $1" //FO CSV //NH 2>/dev/null | awk -F'","' 'NR==1{gsub(/"/,"",$2); print $2; exit}'
}

start_svc() {  # start_svc <name> <cwd> <logfile> <exe> [args...]；env 经当前 shell export 传入
  local name="$1" cwd="$2" lf="$3" exe="$4"; shift 4
  if svc_running "$name"; then log "$name 已在运行(winpid $(cat "$PID_DIR/$name.winpid"))"; return 0; fi
  mkdir -p "$(dirname "$lf")"
  ( cd "$cwd" && nohup "$exe" "$@" >"$lf" 2>&1 & )
  # Windows 上进程出现在 tasklist 有轻微延迟，重试几次；
  # 管道在 pipefail 下 grep 无匹配退出码非零，须 || true 防 set -e 掐死脚本
  local base wpid i; base="$(basename "$exe")"
  for i in 1 2 3 4 5; do
    sleep 1
    wpid="$(winpid_of "$base" || true)"
    [ -n "$wpid" ] && break
  done
  if [ -n "$wpid" ]; then echo "$wpid" > "$PID_DIR/$name.winpid"; log "$name 启动（winpid $wpid，日志 $lf）"
  else err "$name 启动后未找到进程（看 $lf）"; return 1; fi
}

cmd_start() {
  log "启动"
  mkdir -p "$PID_DIR" "$DATA_DIR" "$LOG_ROOT"/{api,mcp,llm}
  ensure_postgres
  ensure_redis
  # 无 Docker 部署：runtimenop 空实现（Agent 实例操作报错，其余模块正常）
  # AIARSENAL_AGENT/MCP/LLM_BASE_URL 是注入容器的回连地址，无 Docker 不需要
  AIARSENAL_PUBLIC_SCHEME=http AIARSENAL_PUBLIC_DOMAIN=aiarsenal.cc CLIENT_PACKAGE_DIR="$DATA_DIR" \
    AIARSENAL_LOG_DIR="$LOG_ROOT" AIARSENAL_RUNTIME=disabled \
    start_svc api "$REPO/api" "$LOG_ROOT/api/stdout.log" "$REPO/api/bin/main.exe"
  # mcp 网关：api module 第三 main（cwd 必须是 api/，config/ 与主服务同份）
  MCP_PORT=$MCP_PORT AIARSENAL_LOG_DIR="$LOG_ROOT" \
    start_svc mcp "$REPO/api" "$LOG_ROOT/mcp/stdout.log" "$REPO/api/bin/mcp.exe"
  APP_ENV=test PORT=$LLM_PORT ROUTER_SCHEME=http ROUTER_DOMAIN=llm.aiarsenal.cc AIARSENAL_LOG_DIR="$LOG_ROOT" \
    start_svc llm "$REPO/llm" "$LOG_ROOT/llm/stdout.log" "$REPO/llm/bin/llm.exe"
  # demo-http：MCP 演示上游常驻监听（streamable-http 模式）。
  # stdio 的 demo 不用常驻——网关/管理台探测按需拉子进程、用完即杀。
  DEMO_PORT=$DEMO_PORT \
    start_svc demo "$REPO/api" "$LOG_ROOT/mcp/demo-http.log" "$REPO/api/bin/mcp-demo.exe" --transport http --port $DEMO_PORT
  # mcp-ops：企业内部能力 MCP server（api module 另一 main，业务逻辑与 api 同源），
  # cwd 必须是 api/（config/ 在那）；网关注册名 platform-ops / platform-{doc,kb,skill,project,notice}(-write) / platform-user。
  AIARSENAL_LOG_DIR="$LOG_ROOT" \
    start_svc ops "$REPO/api" "$LOG_ROOT/mcp/mcp-ops.log" "$REPO/api/bin/mcp-ops.exe" --port $OPS_PORT
  log "等待服务端口就绪"
  wait_port $API_PORT && log "api :$API_PORT ✓" || err "api 未在 :$API_PORT 起来（看 $LOG_ROOT/api/stdout.log）"
  wait_port $MCP_PORT && log "mcp :$MCP_PORT ✓" || err "mcp 未起来（看 $LOG_ROOT/mcp/stdout.log）"
  wait_port $LLM_PORT && log "llm :$LLM_PORT ✓" || err "llm 未起来（看 $LOG_ROOT/llm/stdout.log）"
  wait_port $DEMO_PORT && log "demo-http :$DEMO_PORT ✓" || err "demo-http 未起来（看 $LOG_ROOT/mcp/demo-http.log）"
  wait_port $OPS_PORT && log "mcp-ops :$OPS_PORT ✓" || err "mcp-ops 未起来（看 $LOG_ROOT/mcp/mcp-ops.log）"
  # nginx（Windows 版是控制台程序，nohup 后台拉起；重载/停止走 -s 信号）
  if port_up 80; then log "nginx :80 已在监听"; else
    ( cd "$NGINX_HOME" && nohup ./nginx.exe -p "$(wpath "$NGINX_HOME")" >/dev/null 2>&1 & )
    sleep 1
    port_up 80 && log "nginx :80 ✓" || { err "nginx 未起来（看 $NGINX_HOME/logs/error.log）"; return 1; }
  fi
  log "完成 → http://aiarsenal.cc"
}

cmd_stop() {
  log "停止（postgres / redis 常驻，不停）"
  local s wpid
  for s in api mcp llm demo ops; do
    if [ -f "$PID_DIR/$s.winpid" ]; then wpid=$(cat "$PID_DIR/$s.winpid");
      taskkill //F //T //PID "$wpid" >/dev/null 2>&1 && log "$s (winpid $wpid) 已停" || log "$s 未在运行";
      rm -f "$PID_DIR/$s.winpid"; fi
  done
  if port_up 80; then ( cd "$NGINX_HOME" && ./nginx.exe -p "$(wpath "$NGINX_HOME")" -s stop >/dev/null 2>&1 || true ); log "nginx 已停"; else log "nginx 未运行"; fi
}

cmd_status() {
  log "端口监听"
  for p in 80 5432 6379 $API_PORT $MCP_PORT $LLM_PORT $DEMO_PORT $OPS_PORT; do port_up "$p" && echo "  :$p LISTEN ✓" || echo "  :$p ——"; done
  log "服务进程"
  local s
  for s in api mcp llm demo ops; do svc_running "$s" \
    && echo "  $s winpid $(cat "$PID_DIR/$s.winpid") ✓" || echo "  $s 未运行"; done
}

cmd_logs() { tail -f "$LOG_ROOT"/{api,mcp,llm}/stdout.log "$LOG_ROOT"/mcp/demo-http.log "$LOG_ROOT"/mcp/mcp-ops.log 2>/dev/null; }

cmd_deploy() { cmd_db_init; cmd_build; cmd_config; cmd_start; }

case "${1:-}" in
  prereq) cmd_prereq ;;
  pg-init) cmd_pg_init ;;
  db-init) cmd_db_init ;;
  build) cmd_build ;;
  config) cmd_config ;;
  start) cmd_start ;;
  stop) cmd_stop ;;
  status) cmd_status ;;
  logs) cmd_logs ;;
  deploy) cmd_deploy ;;
  *) echo "用法: bash $0 <prereq|pg-init|db-init|build|config|start|stop|status|logs|deploy>"; exit 1 ;;
esac
