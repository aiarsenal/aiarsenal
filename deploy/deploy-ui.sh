#!/usr/bin/env bash
# ============================================================================
# 前端（UI）部署脚本 —— 按环境编译 vue-vben-admin 并同步到 ui/dist
#   用法:  bash deploy/deploy-ui.sh <test|prod> [--force]
#   test → pnpm build:test        → ui/dist（aiarsenal.cc，HTTP）
#   prod → pnpm build:production  → ui/dist（aiarsenal.cc，HTTPS）
#
# 说明：
#   - 不需要 root（产物与 nginx root(ui/dist) 都在 ubuntu 主目录下）；以 root 运行
#     时编译会自动切到 ubuntu 用户，避免 root 属主文件污染。
#   - 构建命令对齐仓库根 deploy.sh：pnpm --filter @vben/web-antd build:<MODE>。
# ============================================================================
set -euo pipefail
source "$(dirname "$0")/_common.sh"

parse_env "$@"
guard_cross_env

UI_DIR="${REPO}/ui"
echo "==> 环境: ${ENV}  模式: ${MODE}"

echo "==> 构建 UI (vben, mode=${MODE})"
run_build "cd '${UI_DIR}' && export PATH=\"\$HOME/.local/bin:\$PATH\" && pnpm --filter @vben/web-antd build:${MODE}" 2>&1 | tail -5 \
  || { echo "UI 构建失败"; exit 1; }

echo "==> 同步 dist → ${UI_DIR}/dist"
rsync -a --delete "${UI_DIR}/apps/web-antd/dist/" "${UI_DIR}/dist/"

echo
echo "==> 完成：[${ENV}] UI 已部署到 ${UI_DIR}/dist"
echo "    chunk 带 hash，正常刷新站点即可生效（必要时 Ctrl+Shift+R 强刷）。"
