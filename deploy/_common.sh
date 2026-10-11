#!/usr/bin/env bash
# ============================================================================
# deploy/_common.sh —— 各 per-service 部署脚本共享的公共函数（被 source，不单独执行）
#
# 用法：在 deploy/deploy-<svc>.sh 顶部：
#   source "$(dirname "$0")/_common.sh"
#
# 约定（与仓库根 deploy.sh 的「一机一环境」模型一致）：
#   - 一律以构建用户 ubuntu 跑编译（root 时自动 sudo -u ubuntu），避免 root 写出
#     root 属主的文件污染 node_modules / bin。
#   - Go 服务部署需 root（systemctl / 写 /etc/systemd）。
#   - 同机只装一个环境（test/prod 均为 aiarsenal.cc，仅 HTTP/HTTPS 之别），跨环境部署会互相
#     覆盖 ui/dist、后端服务、库 → guard_cross_env 拦截。
# ============================================================================
set -euo pipefail

REPO="/home/ubuntu/aiarsenal"
UBUNTU_USER="ubuntu"
SYSTEMD_SRC="${REPO}/deploy/systemd"

# 解析环境参数：<test|prod> [--force]  →  全局变量 ENV / MODE / FORCE
# 不传参则交互询问；--force 跳过跨环境守卫。
parse_env() {
  ENV="${1:-}"
  FORCE=0
  local a
  for a in "$@"; do [ "$a" = "--force" ] && FORCE=1; done
  if [ -z "$ENV" ] || [ "$ENV" = "--force" ]; then
    echo "未指定部署环境，请选择："
    echo "  test → aiarsenal.cc（测试环境，纯 HTTP + /etc/hosts）"
    echo "  prod → aiarsenal.cc（生产环境，须部署到独立机器）"
    read -r -p "请输入 test 或 prod: " ENV
  fi
  case "$ENV" in
    test) MODE="test" ;;
    prod) MODE="production" ;;
    *) echo "用法: sudo bash $0 <test|prod> [--force]"; exit 1 ;;
  esac
}

# 必须 root（Go 服务脚本用）
require_root() {
  [ "$(id -u)" = "0" ] || { echo "请用 root 运行: sudo bash $0 ${ENV:-<test|prod>}"; exit 1; }
}

# 探测本机是否已部署 aiarsenal.cc（test/prod 同域名，无法按文件名区分，仅返回 deployed/unknown）
machine_env() {
  if [ -f /etc/nginx/conf.d/aiarsenal.cc.conf ]; then echo deployed
  else echo unknown; fi
}

# 跨环境互斥守卫：test/prod 现统一 aiarsenal.cc 域名（仅 HTTP/HTTPS 之别），
# nginx conf 均落 aiarsenal.cc.conf，无法按文件名区分 → 仅做「已部署则提示覆盖」。
guard_cross_env() {
  local me; me="$(machine_env)"
  if [ "$me" != "unknown" ]; then
    echo "!! 本机已部署 aiarsenal.cc，重新部署将覆盖现有 dist / 后端服务（test=HTTP / prod=HTTPS）。"
    echo "!! 如确需覆盖：$0 $ENV --force"
    [ "$FORCE" = "1" ] || exit 1
    echo "!! --force 已指定，继续（将覆盖现有环境）"
  fi
}

# 以构建用户 ubuntu 执行命令串：root 时 sudo -u ubuntu，否则直接执行
# （UI 部署可由 ubuntu 直接跑；Go 部署在 root 下经此切到 ubuntu 跑编译）
run_build() {
  if [ "$(id -u)" = "0" ]; then
    sudo -u "$UBUNTU_USER" bash -c "$1"
  else
    bash -c "$1"
  fi
}

# 编译 Go 服务：ubuntu 跑 go build → 临时二进制（调用方负责 mv/chown/重启）
# 用法: build_go <服务目录> <输出临时路径> [构建包路径，默认 ./cmd]
# 返回: 依赖 pipefail，构建失败时函数返回非零
build_go() {
  local dir="$1" out="$2" pkg="${3:-./cmd}"
  echo "==> go build: ${dir} (${pkg})"
  run_build "cd '${dir}' && export PATH=\"\$HOME/.local/bin:\$PATH\" && go build -o '${out}' '${pkg}'" 2>&1 | tail -3
}

# 安装 systemd unit（原样拷贝）+ daemon-reload + enable
# 用法: install_unit <源 unit 文件路径>
install_unit() {
  local src="$1" dst="/etc/systemd/system/$(basename "$1")"
  install -m644 "$src" "$dst"
  systemctl daemon-reload
  systemctl enable "$(basename "$1" .service)" >/dev/null 2>&1 || true
}

# 平滑替换 Go 服务二进制并重启：停服务 → mv 新二进制 → chown ubuntu → 启动 → 打印状态
# 用法: rotate_binary <service 名> <新二进制临时路径> <目标二进制路径> <服务目录>
rotate_binary() {
  local svc="$1" tmp="$2" bin="$3" dir="$4"
  mkdir -p "$dir/bin" "$dir/resource/log"
  systemctl stop "$svc" 2>/dev/null || true
  mv "$tmp" "$bin"
  chown "$UBUNTU_USER:$UBUNTU_USER" "$bin"
  systemctl start "$svc"
  sleep 1
  echo "==> ${svc}: $(systemctl is-active "$svc")"
}
