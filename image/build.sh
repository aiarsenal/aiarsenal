#!/usr/bin/env bash
# image/build.sh — 构建 aiarsenal 桌面镜像（webtop:ubuntu-kde + Selkies 底座，按 agent+version 维度）。
#
# Dockerfile 按版本分目录：image/Dockerfile/<agent>/<version>/Dockerfile（image/gen.sh 从模板生成）。
# 本脚本缺版本目录时先自动 gen，再构建。
#
# 用法：
#   bash image/build.sh openclaw 2026.8.2   # → aiarsenal-openclaw-desktop:2026.8.2
#   bash image/build.sh hermes 0.21.0       # → aiarsenal-hermes-desktop:0.21.0
#
# hermes 需仓库根 hermes-agent-main/ 源码（gen.sh 会从 zip 自动解压并校验版本）。
# 发版（git tag + 注册 DB + 清旧镜像）用 image/release.sh，别手动 build+入库。
set -euo pipefail

# docker 在本机装于 /usr/local 下（不在最小 PATH；后台 shell 尤其容易缺），显式补上。
export PATH="/usr/local/bin:$PATH"

CONTEXT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"   # image/
REPO="$(cd "${CONTEXT}/.." && pwd)"
cd "${REPO}"

AGENT="${1:?用法: bash image/build.sh <openclaw|hermes> <version>}"
VERSION="${2:?用法: bash image/build.sh <openclaw|hermes> <version>}"

case "${AGENT}" in
  openclaw|hermes) ;;
  *) echo "未知 agent: ${AGENT}（应为 openclaw|hermes）"; exit 1 ;;
esac

DF="image/Dockerfile/${AGENT}/${VERSION}/Dockerfile"
if [ ! -f "${DF}" ]; then
  echo "==> 缺 ${DF}，先从模板生成"
  bash image/gen.sh "${AGENT}" "${VERSION}"
fi

# webtop 底座 digest pin：本地已有就用 RepoDigest 钉住 ARG，防 BuildKit 按 registry 最新
# digest 重拉 6.48GB（记忆：webtop digest pin 走缓存）。本地没有则不 pin（触发正常拉取）。
BUILD_ARGS=()
WEBTOP_DIGEST="$(docker image inspect lscr.io/linuxserver/webtop:ubuntu-kde --format '{{index .RepoDigests 0}}' 2>/dev/null || true)"
if [ -n "${WEBTOP_DIGEST}" ]; then
  BUILD_ARGS+=(--build-arg "WEBTOP_BASE_IMAGE=${WEBTOP_DIGEST}")
  echo "    webtop digest pin: ${WEBTOP_DIGEST}"
fi

IMAGE_TAG="aiarsenal-${AGENT}-desktop:${VERSION}"
echo "==> 构建 ${IMAGE_TAG}（${DF}）"
docker build "${BUILD_ARGS[@]}" -f "${DF}" -t "${IMAGE_TAG}" "${REPO}"

echo "==> 完成: ${IMAGE_TAG}"
