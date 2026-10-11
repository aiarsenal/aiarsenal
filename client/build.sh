#!/usr/bin/env bash
# client 编译期构建变体入口：选环境（烘焙 API 域名）+ 按需打包 agent + 钉死 openclaw 版本。
#
# 用法:
#   bash client/build.sh <mac|win> [--env test|prod|...] [--agents openclaw[,hermes,...]] \
#       [--openclaw-version 2026.7.1] [--base-url https://api.example.com] [--dev|--build]
#
# 示例:
#   # 生产环境 + 只打 openclaw + 钉死 2026.7.1，出 dmg
#   bash client/build.sh mac --env prod --agents openclaw --openclaw-version 2026.7.1 --build
#   # 测试环境，全 agent，dev 跑
#   bash client/build.sh mac --env test --dev
#   # 自定义域名（绕过 client-envs.json）
#   bash client/build.sh mac --base-url https://api.mycompany.com --build
#
# 不传 --agents = 打包全部 agent（Cargo default）；不传 --openclaw-version = 沿用后端 spec（不钉死）。
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
PLATFORM=""
ENV_NAME=""
AGENTS=""
OC_VERSION=""
BASE_URL_OVERRIDE=""
MODE="build"   # 默认 build；--dev 切 dev
DRY_RUN=0      # --dry-run 只打印将执行的命令（env 解析 + features），不跑 tauri

usage() {
  echo "用法: client/build.sh <mac|win> [--env test|prod|...] [--agents openclaw[,hermes,...]] [--openclaw-version 2026.7.1] [--base-url URL] [--dev|--build]"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    mac|win) PLATFORM="$1"; shift;;
    --env) ENV_NAME="$2"; shift 2;;
    --agents) AGENTS="$2"; shift 2;;
    --openclaw-version) OC_VERSION="$2"; shift 2;;
    --base-url) BASE_URL_OVERRIDE="$2"; shift 2;;
    --dev) MODE="dev"; shift;;
    --build) MODE="build"; shift;;
    --dry-run) DRY_RUN=1; shift;;
    -h|--help) usage; exit 0;;
    *) echo "未知参数: $1" >&2; usage; exit 1;;
  esac
done

[[ -n "$PLATFORM" ]] || { usage; exit 1; }

# 1) 环境 → AA_BASE_URL / AA_ENV_NAME（烘焙 API 域名进 shared/rust 的 option_env!）
#    client-envs.json 按 --env 取 scheme+domain；--base-url 直传绕过配置文件。
if [[ -n "$BASE_URL_OVERRIDE" ]]; then
  export AA_BASE_URL="$BASE_URL_OVERRIDE"
  export AA_ENV_NAME="custom"
elif [[ -n "$ENV_NAME" ]]; then
  ENVS_JSON="$REPO/api/config/client-envs.json"
  [[ -f "$ENVS_JSON" ]] || { echo "缺 $ENVS_JSON" >&2; exit 1; }
  LINE=$(node -e "
    const j=require('$ENVS_JSON');
    const e=(j.envs||[]).find(x=>x.name==='$ENV_NAME');
    if(!e){process.stderr.write('环境 $ENV_NAME 不在 client-envs.json\\n');process.exit(1);}
    process.stdout.write((e.scheme||'https')+' '+(e.domain||''));
  ") || exit 1
  SCHEME="${LINE% *}"; DOMAIN="${LINE#* }"
  export AA_BASE_URL="$SCHEME://api.$DOMAIN"
  export AA_ENV_NAME="$ENV_NAME"
fi
echo "[build] env=${AA_ENV_NAME:-（未设→编译期默认 test）} base_url=${AA_BASE_URL:-（编译期默认 http://api.aiarsenal.cc）}"

# 2) openclaw 版本钉死（option_env! AA_OPENCLAW_VERSION；不传则沿用后端 spec）
if [[ -n "$OC_VERSION" ]]; then
  export AA_OPENCLAW_VERSION="$OC_VERSION"
fi
echo "[build] platform=$PLATFORM mode=$MODE agents=${AGENTS:-（全部）} openclaw=${OC_VERSION:-（沿用 spec）}"

cd "$REPO/client/$PLATFORM"
if [[ -n "$AGENTS" ]]; then
  # 按需打包：--no-default-features + 显式列要的（custom-protocol + mac 的 openclaw-desktop + 选的 agent-*）
  FEATS="custom-protocol"
  [[ "$PLATFORM" == "mac" ]] && FEATS="$FEATS,openclaw-desktop"
  IFS=',' read -ra A <<< "$AGENTS"
  for a in "${A[@]}"; do FEATS="$FEATS,agent-$a"; done
  echo "[build] features=$FEATS（按需，排除未选 agent）"
  [[ "$DRY_RUN" == "1" ]] && { echo "[dry-run] 将执行: npx tauri $MODE --no-default-features --features $FEATS"; exit 0; }
  npx tauri "$MODE" --no-default-features --features "$FEATS"
else
  # 沿用 package.json scripts（mac 已带 --features openclaw-desktop；default 含全部 agent-*）
  echo "[build] features=default（全部 agent）+ openclaw-desktop(mac)"
  [[ "$DRY_RUN" == "1" ]] && { echo "[dry-run] 将执行: npm run $MODE（package.json scripts，default features）"; exit 0; }
  npm run "$MODE"
fi
