#!/usr/bin/env bash
# ============================================================================
# 后端（API）部署脚本 —— 编译 api 二进制并重启 systemd 服务
#   用法:  sudo bash deploy/deploy-api.sh <test|prod> [--force]
#
# 说明：
#   - api 二进制与环境无关（运行时读 api/config/config.yaml）；但 systemd unit
#     含 AIARSENAL_PUBLIC_SCHEME=__PUBLIC_SCHEME__ 占位，按 <env> 替换（test→http/prod→https）。
#   - 编译以 ubuntu 跑（root 下自动 sudo -u ubuntu），二进制落 api/bin/main。
#   - 端口 :43211（config/config.yaml 可改）。
# ============================================================================
set -euo pipefail
source "$(dirname "$0")/_common.sh"

parse_env "$@"
require_root
guard_cross_env

API_DIR="${REPO}/api"
SVC="aiarsenal-api"

echo "==> 环境: ${ENV}（api 二进制环境无关，env 仅守卫/日志用）"

# unit 含 __PUBLIC_SCHEME__ 占位（对外协议 test→http / prod→https），按当前环境替换后安装
case "$ENV" in test) PUB_SCHEME="http";; *) PUB_SCHEME="https";; esac
echo "==> 安装 systemd unit: ${SVC}（AIARSENAL_PUBLIC_SCHEME=${PUB_SCHEME}）"
sed "s/__PUBLIC_SCHEME__/${PUB_SCHEME}/g" "${SYSTEMD_SRC}/${SVC}.service" > "/etc/systemd/system/${SVC}.service"
chmod 644 "/etc/systemd/system/${SVC}.service"
systemctl daemon-reload
systemctl enable "${SVC}" >/dev/null 2>&1 || true

build_go "$API_DIR" /tmp/api-main-new ./cmd || { echo "API 构建失败"; exit 1; }
rotate_binary "$SVC" /tmp/api-main-new "${API_DIR}/bin/main" "$API_DIR"

echo
echo "==> 完成：[${ENV}] ${SVC} 已重启"
echo "    端口 :43211  状态: $(systemctl is-active "$SVC")"
