#!/usr/bin/env bash
# ============================================================================
# LLM 网关部署脚本 —— 编译 llm 二进制，按环境注入 APP_ENV 并重启 systemd 服务
#   用法:  sudo bash deploy/deploy-llm.sh <test|prod> [--force]
#
# 说明：
#   - llm 单元含 __ENV__ 占位（APP_ENV=test/prod，由其推导网关 domain）→ 部署时
#     按当前环境替换后安装。这是 llm 与 api/mcp 的唯一区别。
#   - 编译以 ubuntu 跑（root 下自动 sudo -u ubuntu），二进制落 llm/bin/llm。
#   - 端口 :43214（systemd 单元 PORT 可改）。
# ============================================================================
set -euo pipefail
source "$(dirname "$0")/_common.sh"

parse_env "$@"
require_root
guard_cross_env

LLM_DIR="${REPO}/llm"
SVC="aiarsenal-llm"

echo "==> 环境: ${ENV}  →  APP_ENV=${ENV}"

echo "==> 安装 systemd unit: ${SVC}（替换 __ENV__ → ${ENV}）"
sed "s/__ENV__/${ENV}/g" "${SYSTEMD_SRC}/${SVC}.service" > "/etc/systemd/system/${SVC}.service"
chmod 644 "/etc/systemd/system/${SVC}.service"
systemctl daemon-reload
systemctl enable "$SVC" >/dev/null 2>&1 || true

build_go "$LLM_DIR" /tmp/llm-gw-new ./cmd || { echo "LLM 构建失败"; exit 1; }
rotate_binary "$SVC" /tmp/llm-gw-new "${LLM_DIR}/bin/llm" "$LLM_DIR"

echo
echo "==> 完成：[${ENV}] ${SVC} 已重启（APP_ENV=${ENV}）"
echo "    端口 :43214  状态: $(systemctl is-active "$SVC")"
