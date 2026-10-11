#!/usr/bin/env bash
# image/gen.sh — 按 agent+version 生成版本化 Dockerfile（image/Dockerfile/<agent>/<version>/Dockerfile）。
#
# 模板在 image/Dockerfile/_template/（__VERSION__ 占位，单一维护点）；本脚本替换占位落版本目录，
# 生成物提交入库——每个版本的 Dockerfile 直接可 review/可追溯。改模板后对已有版本重跑 gen 同步。
#
# 用法：
#   bash image/gen.sh openclaw 2026.8.2   # openclaw 装自 npm（openclaw@2026.8.2）
#   bash image/gen.sh hermes 0.21.0       # hermes 装自本地源码，版本须与源码 pyproject.toml 一致
#
# hermes 源码约定放仓库根 hermes-agent-main/（gitignored 外部源码）；缺失时自动从仓库根最新的
# hermes-agent-*.zip 解压（zip 内有重复条目，用 -o 覆盖）。
set -euo pipefail

CONTEXT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"   # image/
REPO="$(cd "${CONTEXT}/.." && pwd)"
cd "${REPO}"

AGENT="${1:?用法: bash image/gen.sh <openclaw|hermes> <version>}"
VERSION="${2:?用法: bash image/gen.sh <openclaw|hermes> <version>}"

case "${AGENT}" in
  openclaw|hermes) ;;
  *) echo "未知 agent: ${AGENT}（应为 openclaw|hermes）"; exit 1 ;;
esac

if [ "${AGENT}" = "hermes" ]; then
  # hermes 版本事实来源 = 源码 pyproject.toml；缺源码先从 zip 恢复，再校验一致（防目录名/版本漂移）。
  SRC_DIR="${REPO}/hermes-agent-main"
  if [ ! -d "${SRC_DIR}" ]; then
    HERMES_ZIP="$(ls -t "${REPO}"/hermes-agent-*.zip 2>/dev/null | head -n 1 || true)"
    if [ -z "${HERMES_ZIP}" ]; then
      echo "!! 缺 hermes 源码：无 ${SRC_DIR}/，仓库根也没有 hermes-agent-*.zip"; exit 1
    fi
    echo "==> 解压 ${HERMES_ZIP} → hermes-agent-main/（一次性，后续复用；gitignored）"
    unzip -q -o "${HERMES_ZIP}" -d "${REPO}"
    rm -rf "${SRC_DIR}"
    mv "${REPO}"/hermes-agent-*/ "${SRC_DIR}"
  fi
  SRC_VER="$(grep -m1 '^version' "${SRC_DIR}/pyproject.toml" | cut -d'"' -f2)"
  if [ "${SRC_VER}" != "${VERSION}" ]; then
    echo "!! 源码实际版本 ${SRC_VER:-读不到} ≠ ${VERSION}；请换源码（重解压对应 zip）或改版本号"; exit 1
  fi
else
  # openclaw 安装自 npm（openclaw@<version>，构建期还有 --version 断言把关）；仓库根的
  # openclaw-<version>.zip 只是版本参照物——没有匹配 zip 仅提示不阻断（npm 才是事实来源）。
  if ls "${REPO}"/openclaw-*.zip >/dev/null 2>&1; then
    if ! ls "${REPO}"/openclaw-*.zip | grep -F "${VERSION}" >/dev/null; then
      echo "   （提示：仓库根未见 openclaw-${VERSION}.zip；npm 上存在 openclaw@${VERSION} 即可构建）"
    fi
  fi
fi

TMPL="image/Dockerfile/_template/Dockerfile.${AGENT}-desktop.tmpl"
OUT="image/Dockerfile/${AGENT}/${VERSION}/Dockerfile"
[ -f "${TMPL}" ] || { echo "!! 模板缺失: ${TMPL}"; exit 1; }
mkdir -p "$(dirname "${OUT}")"
sed "s/__VERSION__/${VERSION}/g" "${TMPL}" > "${OUT}"
echo "✓ 生成 ${OUT}"
