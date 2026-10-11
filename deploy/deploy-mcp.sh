#!/usr/bin/env bash
# ============================================================================
# MCP 网关部署脚本 —— 编译 mcp 二进制并重启 systemd 服务
#   用法:  sudo bash deploy/deploy-mcp.sh <test|prod> [--force]
#
# 说明：
#   - 网关是 api module 的第三 main（api/cmd/mcp，与 api 主服务/ops 同一套数据层），
#     二进制与环境无关（运行时由 systemd 注入 MCP_PORT=43213）；<env> 仅用于
#     跨环境守卫与日志。
#   - 配置读 api/config/config.yaml 的 database + actor 节（systemd 单元
#     WorkingDirectory=api/ 目录），不再有独立 mcp/ 配置副本。
#   - 编译以 ubuntu 跑（root 下自动 sudo -u ubuntu），二进制落 api/bin/mcp。
#   - 端口 :43213（systemd 单元 MCP_PORT 可改）。
# ============================================================================
set -euo pipefail
source "$(dirname "$0")/_common.sh"

parse_env "$@"
require_root
guard_cross_env

API_DIR="${REPO}/api"
SVC="aiarsenal-mcp"

echo "==> 环境: ${ENV}（mcp 二进制环境无关，env 仅守卫/日志用）"

if ! grep -q "^actor:" "${API_DIR}/config/config.yaml"; then
  echo "!! ${API_DIR}/config/config.yaml 缺 actor.signKey 节——网关令牌交换签名依赖它（与 ops 同值），补上再部署"
  exit 1
fi

echo "==> 安装 systemd unit: ${SVC}"
install_unit "${SYSTEMD_SRC}/${SVC}.service"

build_go "$API_DIR" /tmp/mcp-gw-new ./cmd/mcp || { echo "MCP 构建失败"; exit 1; }
rotate_binary "$SVC" /tmp/mcp-gw-new "${API_DIR}/bin/mcp" "$API_DIR"

# 演示 server：网关按需 spawn 的子进程，放好二进制即可，无需重启服务
build_go "$API_DIR" /tmp/mcp-demo-new ./cmd/mcp-demo || { echo "mcp-demo 构建失败"; exit 1; }
install -o ubuntu -g ubuntu -m 0755 /tmp/mcp-demo-new "${API_DIR}/bin/mcp-demo"
rm -f /tmp/mcp-demo-new

echo
echo "==> 完成：[${ENV}] ${SVC} 已重启"
echo "    端口 :43213  状态: $(systemctl is-active "$SVC")"
