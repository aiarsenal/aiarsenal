#!/usr/bin/env bash
# ============================================================================
# AI Arsenal 本机（macOS）测试环境部署脚本
#   把 deploy.sh test 的行为复刻到 Mac：纯 HTTP + /etc/hosts 本机解析 + aiarsenal.cc 域名。
#
#   前置（手动装，本脚本不管）：
#     - Go ≥1.26（api 的 go.mod 要求）：brew upgrade go
#     - nginx：brew install nginx
#     - PostgreSQL 15（本机已装）：用 pg_ctl 起（brew services 在本机有 bug）
#     - pgvector（aa_skill 库 vector 扩展）：源码 make install 进 pg15
#     - Docker daemon（api 启动 localdocker runtime adapter 会 Ping daemon）：colima start 或 Docker Desktop
#     - Node v22 + pnpm（nvm 管理）
#     - Redis（本机已在跑）
#
#   用法：
#     bash scripts/local-test.sh prereq    # 检查前置是否就绪
#     bash scripts/local-test.sh db-init   # 建 role + 10 库（调 api/resource/data/*.sql）+ vector 扩展 + GRANT
#     bash scripts/local-test.sh build     # 构建 UI(build:test) + api/mcp/llm 二进制
#     bash scripts/local-test.sh config    # （sudo）/etc/hosts、/var/log 目录、nginx server 配置
#     bash scripts/local-test.sh start     # 起 postgres/docker 校验 → 5 个服务（api/mcp/llm/demo-http/mcp-ops）→ nginx
#     bash scripts/local-test.sh stop      # 停 4 个服务 + nginx
#     bash scripts/local-test.sh status    # 端口/进程状态
#     bash scripts/local-test.sh logs      # tail -f 服务日志
#     bash scripts/local-test.sh deploy    # = db-init && build && config && start（一键全新部署）
#
#   访问：浏览器开 http://aiarsenal.cc（/etc/hosts 已解析到 127.0.0.1）
# ============================================================================
set -euo pipefail

# ---------- 路径 / 常量 ----------
REPO="$(cd "$(dirname "$0")/.." && pwd)"
# brew 前缀自适应：Apple Silicon → /opt/homebrew，Intel → /usr/local
BREW_PREFIX="${HOMEBREW_PREFIX:-$(brew --prefix 2>/dev/null || echo /opt/homebrew)}"
PG_BIN="$BREW_PREFIX/opt/postgresql@15/bin"
PGDATA="$BREW_PREFIX/var/postgresql@15"
PSQL="$PG_BIN/psql"
PGCTL="$PG_BIN/pg_ctl"
PID_DIR="$REPO/.run"
# 日志默认放仓库 .run/logs（免 sudo：/var/log/aiarsenal 需 root mkdir，config 步骤没跑过就全服务起不来）。
# 要回 /var/log/aiarsenal 用 AIARSENAL_LOG_ROOT 覆盖（需先 sudo 建目录）。
LOG_ROOT="${AIARSENAL_LOG_ROOT:-$REPO/.run/logs}"
DATA_DIR="$REPO/data/client/packages"
SQL_DIR="$REPO/api/resource/data"

# 本机 postgres 启动必须带 LC_ALL（否则 FATAL: postmaster became multithreaded）
export LC_ALL="${LC_ALL:-en_US.UTF-8}"
# 确保 brew 工具链在 PATH（非登录 shell 里 nvm/brew 可能不在）
export PATH="/usr/local/go/bin:/usr/local/bin:$BREW_PREFIX/bin:$BREW_PREFIX/opt/node@22/bin:$PG_BIN:$PATH"
# 国内拉 Go 模块：proxy.golang.org 被墙，走 goproxy.cn（依赖均已公开，无需 GOPRIVATE/SSH 改写）
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" 2>/dev/null || true

DBS=(aiarsenal aa_skill aa_llm aa_mcp aa_agent aa_client aa_project aa_doc aa_kb aa_schedule)
API_PORT=43211; MCP_PORT=43213; LLM_PORT=43214; DEMO_PORT=43219; OPS_PORT=43220

# ---------- 辅助 ----------
log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
err()  { printf '\033[31m错误:\033[0m %s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

pg_running() { "$PGCTL" -D "$PGDATA" status >/dev/null 2>&1; }
docker_running() { docker info >/dev/null 2>&1; }
port_up() { nc -z 127.0.0.1 "$1" >/dev/null 2>&1; }
wait_port() { local p=$1 i; for i in $(seq 1 40); do port_up "$p" && return 0; sleep 1; done; return 1; }

ensure_postgres() {
  if pg_running; then log "postgres 已在运行"; else
    log "启动 postgres（LC_ALL=${LC_ALL}）"; "$PGCTL" -D "$PGDATA" -l "$PGDATA/server.log" start
  fi
}
ensure_docker() {
  if docker_running; then log "Docker daemon 已就绪"; return 0; fi
  if have colima; then log "启动 colima（Docker daemon）"; colima start && docker_running \
    && { log "colima 就绪"; return 0; } || { err "colima 启动后 docker 仍不可用"; return 1; }
  fi
  err "Docker daemon 未运行，且未装 colima。请先：brew install colima && colima start  （或打开 Docker Desktop）"; return 1
}

# ---------- 子命令 ----------

cmd_prereq() {
  log "前置检查"
  have go && go version | grep -qE 'go1\.(2[6-9]|[3-9])' && echo "  Go: $(go version)" || err "Go ≥1.26 未就绪（brew upgrade go）"
  have nginx && echo "  nginx: $(nginx -v 2>&1)" || err "nginx 未装（brew install nginx）"
  have psql && echo "  psql: $(psql --version)" || err "psql 未装"
  pg_running && echo "  postgres: 运行中" || echo "  postgres: 未运行（start 时会自动起）"
  docker_running && echo "  docker daemon: 就绪" || echo "  docker daemon: 未就绪（start 时会尝试 colima）"
  redis-cli ping >/dev/null 2>&1 && echo "  redis: PONG" || err "redis 未运行"
  PG_SHARE="$("$PG_BIN/pg_config" --sharedir 2>/dev/null || echo "$BREW_PREFIX/share/postgresql@15")"
  [ -f "$PG_SHARE/extension/vector.control" ] && echo "  pgvector: 已装" || echo "  pgvector: 未见（aa_skill 需要）"
}

cmd_db_init() {
  log "数据库初始化（用 $SQL_DIR/*.sql）"
  ensure_postgres
  # 1. role（幂等）
  if ! "$PSQL" postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='aiarsenal'" | grep -q 1; then
    "$PSQL" postgres -c "CREATE ROLE aiarsenal LOGIN PASSWORD 'aiarsenal123';"
  else log "role aiarsenal 已存在"; fi
  # 2. aiarsenal 库（其余库由各自 SQL 的 \gexec 创建）
  "$PSQL" postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aiarsenal'" | grep -q 1 \
    || "$PSQL" postgres -c "CREATE DATABASE aiarsenal;"
  # 注意：aiarsenal.sql 不含 \connect，必须用 -d aiarsenal 连进去加载（否则表会建到 postgres 库）
  "$PSQL" -d aiarsenal -v ON_ERROR_STOP=1 -f "$SQL_DIR/aiarsenal.sql" >/dev/null && log "aiarsenal.sql ✓"
  # 3. aa_skill：先建库 + vector 扩展，再跑 skills.sql（建表用到 vector 类型）
  "$PSQL" postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aa_skill'" | grep -q 1 \
    || "$PSQL" postgres -c "CREATE DATABASE aa_skill;"
  "$PSQL" -d aa_skill -c "CREATE EXTENSION IF NOT EXISTS vector;"
  "$PSQL" postgres -v ON_ERROR_STOP=1 -f "$SQL_DIR/skills.sql" >/dev/null && log "skills.sql ✓"
  # 4. 其余库（schedule.sql 自带 \connect aa_schedule 建库 + 末尾 \connect aiarsenal 灌菜单）
  for f in llm mcp agent client project schedule; do
    "$PSQL" postgres -v ON_ERROR_STOP=1 -f "$SQL_DIR/$f.sql" >/dev/null && log "$f.sql ✓"
  done
  # 5. aa_doc：先建库 + vector 扩展，再跑 doc.sql（建表用到 vector 类型）
  "$PSQL" postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aa_doc'" | grep -q 1 \
    || "$PSQL" postgres -c "CREATE DATABASE aa_doc;"
  "$PSQL" -d aa_doc -c "CREATE EXTENSION IF NOT EXISTS vector;"
  "$PSQL" postgres -v ON_ERROR_STOP=1 -f "$SQL_DIR/doc.sql" >/dev/null && log "doc.sql ✓"
  # 5b. aa_kb（知识库管理，与 aa_doc 同构）：先建库 + vector 扩展，再跑 kb.sql
  "$PSQL" postgres -tAc "SELECT 1 FROM pg_database WHERE datname='aa_kb'" | grep -q 1 \
    || "$PSQL" postgres -c "CREATE DATABASE aa_kb;"
  "$PSQL" -d aa_kb -c "CREATE EXTENSION IF NOT EXISTS vector;"
  "$PSQL" postgres -v ON_ERROR_STOP=1 -f "$SQL_DIR/kb.sql" >/dev/null && log "kb.sql ✓"
  # 5. GRANT + OWNER 收口（表/序列由超管建成，api 以 aiarsenal 连接；
  #    OWNER 必须归 aiarsenal——迁移系统 ALTER TABLE 要求所有者，GRANT 不够）
  for db in "${DBS[@]}"; do
    "$PSQL" -d "$db" -c "GRANT ALL ON SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL TABLES IN SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO aiarsenal;" >/dev/null
    "$PSQL" -d "$db" -tAc "SELECT 'ALTER TABLE ' || quote_ident(tablename) || ' OWNER TO aiarsenal;' FROM pg_tables WHERE schemaname='public' AND tableowner <> 'aiarsenal'" | "$PSQL" -d "$db" >/dev/null
    "$PSQL" -d "$db" -tAc "SELECT 'ALTER SEQUENCE ' || quote_ident(sequencename) || ' OWNER TO aiarsenal;' FROM pg_sequences WHERE schemaname='public' AND sequenceowner <> 'aiarsenal'" | "$PSQL" -d "$db" >/dev/null
  done
  log "数据库初始化完成（10 库 + vector + GRANT）"
}

cmd_build() {
  log "构建 UI（build:test）"
  ( cd "$REPO/ui" && pnpm install --frozen-lockfile 2>/dev/null || pnpm install )
  ( cd "$REPO/ui" && pnpm --filter @vben/web-antd build:test )
  rsync -a --delete "$REPO/ui/apps/web-antd/dist/" "$REPO/ui/dist/"
  log "构建 Go 服务（api/mcp/llm/mcp-ops）"
  ( cd "$REPO/api" && go build -o bin/main ./cmd ) && log "api ✓"
  ( cd "$REPO/llm" && go build -o bin/llm ./cmd ) && log "llm ✓"
  # mcp / mcp-ops / mcp-demo 都是 api module 的 main（同一套数据层与鉴权逻辑，入口不同）
  ( cd "$REPO/api" && go build -o bin/mcp ./cmd/mcp ) && log "mcp ✓"
  ( cd "$REPO/api" && go build -o bin/mcp-ops ./cmd/ops ) && log "mcp-ops ✓"
  ( cd "$REPO/api" && go build -o bin/mcp-demo ./cmd/mcp-demo ) && log "mcp-demo ✓"
}

cmd_config() {
  log "系统配置（需 sudo）"
  # /etc/hosts
  if ! grep -q 'aiarsenal.cc' /etc/hosts 2>/dev/null; then
    echo '127.0.0.1 aiarsenal.cc api.aiarsenal.cc mcp.aiarsenal.cc llm.aiarsenal.cc' | sudo tee -a /etc/hosts >/dev/null
    log "/etc/hosts 已加 aiarsenal.cc 解析"
  else log "/etc/hosts 已有 aiarsenal.cc"; fi
  # host.docker.internal：API 下发给 agent 的回连地址（AIARSENAL_*_BASE_URL）用它——容器内由
  # Docker DNS 解析到宿主，但桌面客户端的 agent 是原生进程，macOS 不认识该名 → hermes 调 LLM
  # 报 nodename nor servname（httpx.ConnectError Errno 8）。宿主也补 127.0.0.1 解析，容器/原生两代通吃。
  if ! grep -q '127.0.0.1 host.docker.internal' /etc/hosts 2>/dev/null; then
    echo '127.0.0.1 host.docker.internal' | sudo tee -a /etc/hosts >/dev/null
    log "/etc/hosts 已加 host.docker.internal（原生 agent 回连）"
  else log "/etc/hosts 已有 host.docker.internal"; fi
  # 日志目录
  sudo mkdir -p "$LOG_ROOT"/{api,mcp,llm} && sudo chown -R "$(whoami)" "$LOG_ROOT"
  log "日志目录 $LOG_ROOT 就绪"
  # nginx server 配置：由代码内 test.conf sed 路径得到（仅替换 /home/ubuntu/aiarsenal → 本仓）
  NGINX_SRV_DIR="$(nginx -t 2>&1 | grep -oE '/[^ ]*/nginx.conf' | head -1 | xargs dirname)/servers"
  [ -d "$NGINX_SRV_DIR" ] || NGINX_SRV_DIR="/usr/local/etc/nginx/servers"
  mkdir -p "$NGINX_SRV_DIR" 2>/dev/null || sudo mkdir -p "$NGINX_SRV_DIR"
  sed "s#/home/ubuntu/aiarsenal#$REPO#g" "$REPO/deploy/nginx/test.conf" \
    | sudo tee "$NGINX_SRV_DIR/aiarsenal.cc.conf" >/dev/null
  log "nginx server 配置 → $NGINX_SRV_DIR/aiarsenal.cc.conf"
  # nginx 默认以 nobody 跑，需对 repo 静态资源路径有穿越权。macOS 家目录常为 drwxr-x---（other 无权穿越）
  # → nginx 访问 $REPO/ui/dist 报 403。逐段放开 other 的 x（仅穿越、不可列目录），文件本就 o+r。
  for d in "$HOME" "$REPO" "$REPO/ui" "$REPO/ui/dist"; do
    [ -d "$d" ] || continue
    [ "$(stat -f '%Sp' "$d" 2>/dev/null | cut -c9)" = "-" ] && { chmod o+x "$d" && log "放开 $d 的 other 穿越权限（nginx 静态资源）"; }
  done
  # 确认 nginx.conf include servers/*
  NGINX_CONF="$(nginx -t 2>&1 | grep -oE '/[^ ]*/nginx.conf' | head -1)"
  [ -n "$NGINX_CONF" ] || NGINX_CONF="/usr/local/etc/nginx/nginx.conf"
  grep -q 'include servers' "$NGINX_CONF" 2>/dev/null \
    || err "请在 $NGINX_CONF 的 http{} 内加: include servers/*;"
  sudo nginx -t && log "nginx 配置校验通过"
}

start_svc() {  # start_svc <name> <cwd> <pidfile> <logfile> <env...> -- <cmd...>
  local name="$1" cwd="$2" pid="$3" lf="$4"; shift 4
  if [ -f "$pid" ] && kill -0 "$(cat "$pid")" 2>/dev/null; then log "$name 已在运行(pid $(cat "$pid"))"; return 0; fi
  local env=() cmd=() saw=0
  for a in "$@"; do if [ "$a" = "--" ]; then saw=1; elif [ "$saw" = 0 ]; then env+=("$a"); else cmd+=("$a"); fi; done
  # exec 折叠整条 env→nohup→cmd 链：后台子 shell 的 pid 即最终服务进程 pid。
  # （旧写法 `... & echo $!` 在 macOS 上记到的是中间 bash 包装，stop 杀不掉真进程，
  #   旧二进制占着端口 → 新构建 bind 失败悄悄回落旧代码。
  #   env 数组为空时 bash3.2+`set -u` 下 `"${env[@]}"` 会报 unbound，用 `${env[@]+...}` 惯用法兜住。）
  ( cd "$cwd" && exec env ${env[@]+"${env[@]}"} nohup "${cmd[@]}" >"$lf" 2>&1 ) & echo $! >"$pid"
  log "$name 启动（pid $(cat "$pid")，日志 ${lf}）"
}

cmd_start() {
  log "启动"
  mkdir -p "$PID_DIR" "$DATA_DIR" "$LOG_ROOT"/{api,mcp,llm}
  ensure_postgres
  ensure_docker
  # colima/Docker Desktop 的 docker socket 常不在标准 /var/run/docker.sock（colima→~/.colima/default/docker.sock）。
  # api 的 runtime-go localdocker 默认连 /var/run/docker.sock，连不上即 log.Fatalf；把当前 docker context 的端点透传给它。
  if [ -z "${DOCKER_HOST:-}" ]; then
    DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}' 2>/dev/null || true)"
    [ -n "$DOCKER_HOST" ] && export DOCKER_HOST && log "DOCKER_HOST=${DOCKER_HOST}（透传给 api）"
  fi
  # AIARSENAL_AGENT/MCP/LLM_BASE_URL：注入容器的回连地址须走 host.docker.internal 直连端口——
  # 本机 hosts 把 *.aiarsenal.cc 指 127.0.0.1，colima 的 host-resolver 会把该映射透传进容器 DNS，
  # 容器内解析 api.aiarsenal.cc→127.0.0.1（=容器自身）→ 心跳/LLM 调用 connection refused
  #（运行详情「未找到运行状态」、微信只显示「正在输入」收不到回复两个症状同根）。
  # host.docker.internal 由 colima 提供指向 macOS 宿主；api/mcp/llm 均监听 *:4321x，直连绕过 nginx 免 Host 路由/sudo。
  start_svc api  "$REPO/api" "$PID_DIR/api.pid"  "$LOG_ROOT/api/stdout.log" \
    AIARSENAL_PUBLIC_SCHEME=http AIARSENAL_PUBLIC_DOMAIN=aiarsenal.cc CLIENT_PACKAGE_DIR="$DATA_DIR" \
    AIARSENAL_AGENT_BASE_URL=http://host.docker.internal:43211 \
    AIARSENAL_MCP_BASE_URL=http://host.docker.internal:43213 \
    AIARSENAL_LLM_BASE_URL=http://host.docker.internal:43214 \
    -- "$REPO/api/bin/main"
  # mcp 网关：api module 第三 main（cwd 必须是 api/，config/ 与主服务同份）
  start_svc mcp  "$REPO/api" "$PID_DIR/mcp.pid"  "$LOG_ROOT/mcp/stdout.log" \
    MCP_PORT=43213 -- "$REPO/api/bin/mcp"
  start_svc llm  "$REPO/llm" "$PID_DIR/llm.pid"  "$LOG_ROOT/llm/stdout.log" \
    APP_ENV=test PORT=43214 ROUTER_SCHEME=http ROUTER_DOMAIN=llm.aiarsenal.cc -- "$REPO/llm/bin/llm"
  # demo-http：MCP 演示上游常驻监听（streamable-http 模式）。
  # stdio 的 demo 不用常驻——网关/管理台探测按需拉子进程、用完即杀。
  start_svc demo "$REPO/api" "$PID_DIR/demo.pid" "$LOG_ROOT/mcp/demo-http.log" \
    DEMO_PORT=$DEMO_PORT -- "$REPO/api/bin/mcp-demo" --transport http --port $DEMO_PORT
  # mcp-ops：企业内部能力 MCP server（api module 另一 main，业务逻辑与 api 同源），
  # cwd 必须是 api/（config/ 在那）；网关注册名 platform-ops / platform-{doc,kb,skill,project,notice}(-write) / platform-user。
  start_svc ops "$REPO/api" "$PID_DIR/ops.pid" "$LOG_ROOT/mcp/mcp-ops.log" \
    -- "$REPO/api/bin/mcp-ops" --port $OPS_PORT
  log "等待服务端口就绪"
  wait_port $API_PORT && log "api :$API_PORT ✓" || err "api 未在 :$API_PORT 起来（看 $LOG_ROOT/api/stdout.log）"
  wait_port $MCP_PORT && log "mcp :$MCP_PORT ✓" || err "mcp 未起来"
  wait_port $LLM_PORT && log "llm :$LLM_PORT ✓" || err "llm 未起来"
  wait_port $DEMO_PORT && log "demo-http :$DEMO_PORT ✓" || err "demo-http 未起来（看 $LOG_ROOT/mcp/demo-http.log）"
  wait_port $OPS_PORT && log "mcp-ops :$OPS_PORT ✓" || err "mcp-ops 未起来（看 $LOG_ROOT/mcp/mcp-ops.log）"
  # nginx（端口 80 需 root）
  # </dev/null：sudo 无 tty 时快速失败而非挂死等 stdin（交互终端仍正常走 tty 弹密码）
  if pgrep -x nginx >/dev/null 2>&1; then sudo nginx -s reload </dev/null 2>/dev/null && log "nginx reload" || err "nginx reload 需手动: sudo nginx -s reload"; else sudo nginx </dev/null 2>/dev/null && log "nginx 启动" || err "nginx 启动需手动: sudo nginx（或用已可用的 /usr/local/bin/nginx）"; fi
  log "完成 → http://aiarsenal.cc"
}

cmd_stop() {
  log "停止"
  for s in api mcp llm demo ops; do
    if [ -f "$PID_DIR/$s.pid" ]; then pid=$(cat "$PID_DIR/$s.pid"); kill "$pid" 2>/dev/null && log "$s (pid $pid) 已停"; rm -f "$PID_DIR/$s.pid"; fi
  done
  pgrep -x nginx >/dev/null 2>&1 && { sudo nginx -s stop </dev/null 2>/dev/null && log "nginx 已停"; } || log "nginx 未运行（或 sudo 不可用，未停成——port 80 仍被占）"
}

cmd_status() {
  log "端口监听"
  for p in 80 $API_PORT $MCP_PORT $LLM_PORT $DEMO_PORT $OPS_PORT; do port_up "$p" && echo "  :$p LISTEN ✓" || echo "  :$p ——"; done
  log "服务进程"
  for s in api mcp llm demo ops; do [ -f "$PID_DIR/$s.pid" ] && kill -0 "$(cat "$PID_DIR/$s.pid")" 2>/dev/null \
    && echo "  $s pid $(cat "$PID_DIR/$s.pid") ✓" || echo "  $s 未运行"; done
}

cmd_logs() { tail -f "$LOG_ROOT"/{api,mcp,llm}/stdout.log "$LOG_ROOT"/mcp/demo-http.log "$LOG_ROOT"/mcp/mcp-ops.log 2>/dev/null; }

cmd_deploy() { cmd_db_init; cmd_build; cmd_config; cmd_start; }

case "${1:-}" in
  prereq) cmd_prereq ;;
  db-init) cmd_db_init ;;
  build) cmd_build ;;
  config) cmd_config ;;
  start) cmd_start ;;
  stop) cmd_stop ;;
  status) cmd_status ;;
  logs) cmd_logs ;;
  deploy) cmd_deploy ;;
  *) echo "用法: bash $0 <prereq|db-init|build|config|start|stop|status|logs|deploy>"; exit 1 ;;
esac
