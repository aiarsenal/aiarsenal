#!/usr/bin/env bash
# ============================================================================
# AI Arsenal 一键部署脚本（环境驱动域名）
#   用法:  sudo bash /home/ubuntu/aiarsenal/deploy.sh <test|prod|rebuild-db>
#   示例:  sudo bash /home/ubuntu/aiarsenal/deploy.sh test     # 测试环境 aiarsenal.cc（HTTP + /etc/hosts）
#          sudo bash /home/ubuntu/aiarsenal/deploy.sh prod     # 生产环境 aiarsenal.cc（HTTPS + 真实证书）
#          sudo bash /home/ubuntu/aiarsenal/deploy.sh rebuild-db  # 重建全部 PG 库（危险）
#
# 域名由 UI 的环境配置文件决定（test/prod 统一 aiarsenal.cc，仅 HTTP/HTTPS 之别）：
#   test → ui/.env.test         → http://api.aiarsenal.cc   → 纯 HTTP + /etc/hosts 本机解析（无证书）
#   prod → ui/.env.production   → https://api.aiarsenal.cc  → 通配符证书 *.aiarsenal.cc + HTTPS
#
# 完成：按环境构建 UI + 后端服务；prod 额外签发通配符证书并启用 HTTPS，test 走纯 HTTP。
#
# 证书：Let's Encrypt / ZeroSSL + DNS-01 手动验证（无需 DNS API 密钥）
#   第 1 次运行：输出需添加的 TXT 记录(_acme-challenge 值)
#   到域名 DNS 服务商(腾讯云/DNSPod)添加 TXT 后，再次运行本脚本
#   第 2 次运行：验证通过 → 安装证书 → 写 nginx HTTPS 配置 → reload
#
# 数据库重建（可选、显式触发，不在默认部署流里）：
#   sudo bash deploy.sh rebuild-db
#   重建全部 PG 库（aiarsenal / aa_skill / aa_llm / aa_mcp / aa_agent / aa_client / aa_project）：
#   按依赖顺序加载 resource/data/*.sql（每表 DROP IF EXISTS，可重复跑），⚠️ 会清空现有表。
#
# 前提：已装 acme.sh(/root/.acme.sh/acme.sh)、nginx、node；
#       A 记录 <domain> 与 api.<domain> 已指向本机。
# ============================================================================
set -euo pipefail

# ---------- rebuild-db 子命令：重建数据库（危险，显式触发） ----------
if [ "${1:-}" = "rebuild-db" ]; then
  REPO="/home/ubuntu/aiarsenal"
  SQL_DIR="${REPO}/api/resource/data"
  [ "$(id -u)" = "0" ] || { echo "请用 root 运行: sudo bash $0 rebuild-db"; exit 1; }
  command -v psql >/dev/null || { echo "需要 psql (postgresql-client)"; exit 1; }
  id postgres >/dev/null 2>&1 || { echo "需要 postgres 系统用户"; exit 1; }

  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  echo "!! 即将重建全部数据库：DROP 现有表 + 重新建表 + 种子。"
  echo "!! 涉及库：aiarsenal / aa_skill / aa_llm / aa_mcp / aa_agent / aa_client / aa_project / aa_doc / aa_kb / aa_schedule"
  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  read -r -p "确认清空重建？输入 yes 继续： " ANS
  [ "$ANS" = "yes" ] || { echo "已取消"; exit 1; }

  PSQL=(sudo -u postgres psql -v ON_ERROR_STOP=1)
  # aiarsenal.sql 不自建库，需先确保 aiarsenal 库存在
  "${PSQL[@]}" -tc "SELECT 1 FROM pg_database WHERE datname='aiarsenal'" | grep -q 1 \
    || "${PSQL[@]}" -c "CREATE DATABASE aiarsenal"

  # 按依赖顺序加载：aiarsenal 先（aa_user/aa_menu 等，agent.sql / project.sql / schedule.sql 的 \connect aiarsenal 依赖它）；
  # skills/doc/kb 建表用 vector 类型，正文自带 CREATE EXTENSION（超管执行，OK）
  for f in aiarsenal.sql skills.sql llm.sql mcp.sql agent.sql client.sql project.sql doc.sql kb.sql schedule.sql; do
    echo "==> 加载 ${f}"
    "${PSQL[@]}" -d aiarsenal -f "${SQL_DIR}/${f}"
  done
  # GRANT + OWNER 收口：表/序列由超管(postgres)建成，api 以 aiarsenal 连接，须授权（否则 api
  # 查询全报 permission denied）；OWNER 必须归 aiarsenal——迁移系统 ALTER TABLE 要求所有者，GRANT 不够
  for db in aiarsenal aa_skill aa_llm aa_mcp aa_agent aa_client aa_project aa_doc aa_kb aa_schedule; do
    echo "==> GRANT ${db}"
    "${PSQL[@]}" -d "$db" -c "GRANT ALL ON SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL TABLES IN SCHEMA public TO aiarsenal; \
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO aiarsenal;" >/dev/null
    "${PSQL[@]}" -d "$db" -tAc "SELECT 'ALTER TABLE ' || quote_ident(tablename) || ' OWNER TO aiarsenal;' FROM pg_tables WHERE schemaname='public' AND tableowner <> 'aiarsenal'" | "${PSQL[@]}" -d "$db" >/dev/null
    "${PSQL[@]}" -d "$db" -tAc "SELECT 'ALTER SEQUENCE ' || quote_ident(sequencename) || ' OWNER TO aiarsenal;' FROM pg_sequences WHERE schemaname='public' AND sequenceowner <> 'aiarsenal'" | "${PSQL[@]}" -d "$db" >/dev/null
  done
  echo "==> 数据库重建完成（10 个库 + GRANT）"
  exit 0
fi

# ---------- 参数 → 环境（必须显式选择，不传参则交互询问，避免误部署）----------
ENV="${1:-}"
if [ -z "$ENV" ]; then
  echo "未指定部署环境，请选择："
  echo "  test → aiarsenal.cc（测试环境，纯 HTTP + /etc/hosts 本机解析）"
  echo "  prod → aiarsenal.cc（生产环境，HTTPS + 真实证书）"
  read -r -p "请输入 test 或 prod: " ENV
fi
case "$ENV" in
  test) MODE="test";       ENV_FILE=".env.test" ;;
  prod) MODE="production"; ENV_FILE=".env.production" ;;
  *) echo "用法: sudo bash $0 <test|prod|rebuild-db> [--force]"; exit 1 ;;
esac

REPO="/home/ubuntu/aiarsenal"
ACME="/root/.acme.sh/acme.sh"
SSL_DIR="/etc/nginx/ssl"
UI_DIR="${REPO}/ui"
ENV_PATH="${UI_DIR}/${ENV_FILE}"

# ---------- 0. 前置检查 ----------
[ "$(id -u)" = "0" ] || { echo "请用 root 运行: sudo bash $0 $ENV"; exit 1; }
[ -x "$ACME" ] || { echo "acme.sh 未安装: $ACME"; exit 1; }
command -v nginx >/dev/null || { echo "nginx 未安装"; exit 1; }
[ -f "$ENV_PATH" ] || { echo "缺少环境配置: $ENV_PATH"; exit 1; }

# ---------- 1. 从配置读取域名 ----------
API_URL="$(grep -E '^VITE_API_URL=' "$ENV_PATH" | head -1 | cut -d= -f2- | tr -d '"\r')"
[ -n "$API_URL" ] || { echo "未在 $ENV_PATH 找到 VITE_API_URL"; exit 1; }
# http(s)://api.aiarsenal.cc -> aiarsenal.cc
DOMAIN="$(printf '%s' "$API_URL" | sed -E 's#^https?://api\.##; s#/.*$##; s#:.*$##')"
API_DOMAIN="api.${DOMAIN}"
NGINX_CONF="/etc/nginx/conf.d/${DOMAIN}.conf"
CONF_DIR="/root/.acme.sh/${DOMAIN}_ecc"

# ---------- 环境隔离：本机已部署则提示覆盖 ----------
# test 与 prod 现统一使用 aiarsenal.cc 域名（仅 test=HTTP / prod=HTTPS 之别），
# nginx conf 均落到 /etc/nginx/conf.d/aiarsenal.cc.conf，无法按文件名区分环境，
# 故仅做「已部署则提示覆盖」的通用提醒，--force 跳过。
if [ -f /etc/nginx/conf.d/aiarsenal.cc.conf ]; then
  echo "!! 检测到本机已部署 aiarsenal.cc（/etc/nginx/conf.d/aiarsenal.cc.conf）。"
  echo "!! 重新部署将覆盖现有 ui/dist 与后端服务（test=HTTP / prod=HTTPS）。"
  echo "!! 如确需覆盖：sudo bash $0 $ENV --force"
  [ "${2:-}" = "--force" ] || exit 1
fi

echo "==> 环境: ${ENV}  模式: ${MODE}"
echo "==> 配置: ${ENV_PATH}  →  VITE_API_URL=${API_URL}"
echo "==> 域名: ${DOMAIN}  (API: ${API_DOMAIN})"

# ---------- 2. UI: 按环境构建（vue-vben-admin，pnpm monorepo）----------
echo "==> 构建 UI (vben, mode=${MODE})"
sudo -u ubuntu bash -c "cd '${REPO}/ui' && export PATH=\"\$HOME/.local/bin:\$PATH\" && pnpm --filter @vben/web-antd build:${MODE}" 2>&1 | tail -5 || { echo "UI 构建失败"; exit 1; }
# nginx root 指向 ui/dist，构建产物在 ui/apps/web-antd/dist，同步过去
rsync -a --delete "${REPO}/ui/apps/web-antd/dist/" "${UI_DIR}/dist/"
echo "==> UI dist 已同步到 ${UI_DIR}/dist"

# ---------- 2.5 Go 后端：安装 systemd unit + 重建二进制 + 重启 ----------
API_DIR="${REPO}/api"
LLM_DIR="${REPO}/llm"
SYSTEMD_SRC="${REPO}/deploy/systemd"

# 密钥文件（api/mcp/llm 三个 unit 的 EnvironmentFile）：首次部署生成随机密钥，绝不进 git。
# ⚠️ 存量部署升级到密钥外置版本前：先手工把【当前线上正在用的】encryptKey/signKey 值写入
#    /etc/aiarsenal/env 再重启（AIARSENAL_JWT_ENCRYPT_KEY 同时是 LLM 凭据 AES 密钥，换值会导致
#    库内既有凭据无法解密）；待后台重录凭据后再轮换成随机值。
if [ ! -f /etc/aiarsenal/env ]; then
  mkdir -p /etc/aiarsenal
  AA_ENC_KEY=$(openssl rand -hex 16)
  cat > /etc/aiarsenal/env <<SECRETS
# AI Arsenal 服务密钥（deploy.sh 首次部署生成；轮换后 systemctl restart aiarsenal-api aiarsenal-mcp aiarsenal-llm）
# AIARSENAL_JWT_ENCRYPT_KEY：JWT 签名 + LLM 凭据/Agent api_key 的 AES-256-GCM 密钥（32 位 hex）
AIARSENAL_JWT_ENCRYPT_KEY=${AA_ENC_KEY}
# AIARSENAL_ACTOR_SIGN_KEY：网关→ops 令牌交换 HMAC-SHA256 签名（api 与 mcp 同值）
AIARSENAL_ACTOR_SIGN_KEY=aa-actor-$(openssl rand -hex 16)
# SECRET_KEY：llm 网关解密 provider 凭据——必须与 AIARSENAL_JWT_ENCRYPT_KEY 同值（api 加密、llm 解密）
SECRET_KEY=${AA_ENC_KEY}
SECRETS
  unset AA_ENC_KEY
  chown root:root /etc/aiarsenal/env && chmod 600 /etc/aiarsenal/env
  echo "==> 已生成 /etc/aiarsenal/env（随机密钥，权限 600）"
else
  echo "==> /etc/aiarsenal/env 已存在，沿用现有密钥"
fi

echo "==> 安装 systemd unit (aiarsenal-api / aiarsenal-mcp / aiarsenal-llm)"
# api unit 含 __PUBLIC_SCHEME__ 占位（对外协议 test→http / prod→https），按当前环境替换后安装
case "$ENV" in test) PUB_SCHEME="http";; *) PUB_SCHEME="https";; esac
sed "s/__PUBLIC_SCHEME__/${PUB_SCHEME}/g" "${SYSTEMD_SRC}/aiarsenal-api.service" > /etc/systemd/system/aiarsenal-api.service
chmod 644 /etc/systemd/system/aiarsenal-api.service
install -m644 "${SYSTEMD_SRC}/aiarsenal-mcp.service" /etc/systemd/system/aiarsenal-mcp.service
# llm unit 含 __ENV__ 占位，按当前环境替换后安装
sed "s/__ENV__/${ENV}/g" "${SYSTEMD_SRC}/aiarsenal-llm.service" > /etc/systemd/system/aiarsenal-llm.service
chmod 644 /etc/systemd/system/aiarsenal-llm.service
systemctl daemon-reload
systemctl enable aiarsenal-api aiarsenal-mcp aiarsenal-llm >/dev/null 2>&1 || true

echo "==> 构建 API 二进制 (api)"
sudo -u ubuntu bash -c "cd '${API_DIR}' && go build -o /tmp/api-main-new ./cmd" 2>&1 | tail -3 \
  || { echo "API 构建失败"; exit 1; }
systemctl stop aiarsenal-api 2>/dev/null || true
mkdir -p "${API_DIR}/bin" "${API_DIR}/resource/log"
mv /tmp/api-main-new "${API_DIR}/bin/main"
chown ubuntu:ubuntu "${API_DIR}/bin/main"
systemctl start aiarsenal-api

echo "==> 构建 MCP 网关二进制 (mcp, api module 第三 main)"
# 与 aiarsenal-mcp.service 一致：二进制落 api/bin/mcp，cwd=api/ 读 api/config/config.yaml
sudo -u ubuntu bash -c "cd '${API_DIR}' && go build -o /tmp/mcp-gw-new ./cmd/mcp" 2>&1 | tail -3 \
  || { echo "MCP 构建失败"; exit 1; }
systemctl stop aiarsenal-mcp 2>/dev/null || true
mv /tmp/mcp-gw-new "${API_DIR}/bin/mcp"
chown ubuntu:ubuntu "${API_DIR}/bin/mcp"
systemctl start aiarsenal-mcp

echo "==> 构建 LLM 网关二进制 (llm)"
mkdir -p "${LLM_DIR}/bin" "${LLM_DIR}/resource/log"
sudo -u ubuntu bash -c "cd '${LLM_DIR}' && go build -o /tmp/llm-gw-new ./cmd" 2>&1 | tail -3 \
  || { echo "LLM 构建失败"; exit 1; }
systemctl stop aiarsenal-llm 2>/dev/null || true
mv /tmp/llm-gw-new "${LLM_DIR}/bin/llm"
chown ubuntu:ubuntu "${LLM_DIR}/bin/llm"
systemctl start aiarsenal-llm

# ---------- 3/4. 通配符证书：仅生产环境（test 走纯 HTTP + /etc/hosts，跳过证书）----------
if [ "$ENV" != "prod" ]; then
  echo "==> 测试环境走纯 HTTP（/etc/hosts 本机解析），跳过证书签发/安装"
else
mkdir -p "$SSL_DIR"

issue_cert()  { sudo -E "$ACME" --renew -d "$DOMAIN" -d "*.$DOMAIN" \
                  --yes-I-know-dns-manual-mode-enough-go-ahead-please --dnssleep 90; }
create_order(){ sudo -E "$ACME" --issue -d "$DOMAIN" -d "*.$DOMAIN" \
                  --dns --yes-I-know-dns-manual-mode-enough-go-ahead-please --dnssleep 90 || true; }

if [ -f "$SSL_DIR/${DOMAIN}.fullchain.pem" ] && [ -f "$SSL_DIR/${DOMAIN}.key" ]; then
  echo "==> 证书已安装于 $SSL_DIR，跳过签发"
else
  if [ -d "$CONF_DIR" ] && [ -f "$CONF_DIR/${DOMAIN}.conf" ]; then
    echo "==> 检测到已创建的证书订单，尝试完成 DNS 验证(renew)..."
    if ! issue_cert; then
      echo
      echo "==> 验证仍未通过。请确认 TXT 记录已生效："
      echo "    dig +short TXT _acme-challenge.${DOMAIN}"
      exit 1
    fi
  else
    echo "==> 首次运行：提交通配符证书订单并生成 DNS 验证 TXT..."
    create_order
    echo
    echo "================ DNS-01 验证说明 ================"
    echo "请在 ${DOMAIN} 的 DNS 服务商(腾讯云 DNSPod)添加 TXT 记录："
    echo "  主机记录: _acme-challenge   记录类型: TXT"
    # acme.sh 可能给出多条 TXT 值（覆盖 *.<domain> 与 <domain>）
    grep -Eho '"value":"[^"]+"' "${CONF_DIR}/${DOMAIN}.conf" 2>/dev/null \
      | sed 's/"value":"//;s/"$//' | sort -u | while read -r v; do
          echo "  记录值:   ${v}"
        done
    echo "（同名多条 TXT，DNSPod 允许；或一条记录填多个值）"
    echo "等待 1-2 分钟 DNS 生效后，再次运行本脚本完成签发："
    echo "  sudo bash $0 $ENV"
    echo "================================================"
    exit 0
  fi
fi

# ---------- 4. 安装证书 ----------
echo "==> 安装证书到 $SSL_DIR"
sudo -E "$ACME" --install-cert -d "$DOMAIN" -d "*.$DOMAIN" \
  --key-file       "$SSL_DIR/${DOMAIN}.key" \
  --fullchain-file "$SSL_DIR/${DOMAIN}.fullchain.pem" \
  --reloadcmd      "systemctl reload nginx || true"
chmod 644 "$SSL_DIR/${DOMAIN}.fullchain.pem"
chmod 600 "$SSL_DIR/${DOMAIN}.key"
fi

# ---------- 5. nginx 配置（使用仓库内已保存的配置，区分环境）----------
NGINX_SRC="${REPO}/deploy/nginx/${ENV}.conf"
[ -f "$NGINX_SRC" ] || { echo "缺少 nginx 配置: $NGINX_SRC"; exit 1; }
echo "==> 安装 nginx 配置: $NGINX_SRC → $NGINX_CONF"
# 先备份旧配置（若有）
[ -f "$NGINX_CONF" ] && cp "$NGINX_CONF" "${NGINX_CONF}.bak.$(date +%s)" 2>/dev/null || true
cp "$NGINX_SRC" "$NGINX_CONF"

# ---------- 6. 校验并生效 ----------
nginx -t && systemctl reload nginx
echo
SCHEME="https"; [ "$ENV" = "test" ] && SCHEME="http"
echo "==> 完成：[${ENV}] ${SCHEME}://${DOMAIN} 与 ${SCHEME}://${API_DOMAIN} 已启用（$([ "$ENV" = "prod" ] && echo HTTPS || echo "纯 HTTP + /etc/hosts")）"
echo "    UI  : ${SCHEME}://${DOMAIN}"
echo "    API : ${SCHEME}://${API_DOMAIN}  →  127.0.0.1:43211"
echo "    nginx 配置仓库路径: ${NGINX_SRC}（修改后重跑本脚本即生效）"
