#!/usr/bin/env bash
# image/release.sh — 发版流水线：本地打 git tag → 构建镜像 → 注册到 aa_agent_system_image_setting
#   （注册后 /#/agent/settings/list 刷新可见；建实例时按 agent/version 解析镜像）。
#
# 镜像按「agent + version」单一维度管理（表 unique(agent, version)，无 os 列），全部 ubuntu 底座。
# Dockerfile 按版本分目录：image/Dockerfile/<agent>/<version>/Dockerfile（缺则先从模板 gen 并要求
# 一并 commit——发版 tag 要能追溯到该版本的 Dockerfile）。
#
# 用法：
#   bash image/release.sh openclaw 2026.8.2   # openclaw 桌面镜像（装自 npm openclaw@2026.8.2）
#   bash image/release.sh hermes  0.21.0      # hermes 桌面镜像（版本须与 hermes-agent-main/ 源码一致）
#
# 流程（任一步失败即中止；构建/入库失败回滚【本地】tag，远端 tag 只在全部成功后才 push）：
#   1. 工作区 image/ 须干净 + push origin master（tag 须指向远端已有 commit，避免悬空）
#   2. 本地打 git tag <agent>-v<version>（已存在则报错，换版本号）
#   3. docker build（webtop 底座用本地镜像 digest pin，防 BuildKit 按 registry 最新 digest 重拉 6.48GB）
#   4. upsert aa_agent_system_image_setting (agent, version, image, display_name)
#   5. 清理旧镜像（保留最新 N 个版本号）
#   6. push git tag（成功才 push，避免给失败版本打远端 tag）
#
# 约定：镜像 tag=<version>，git tag=<agent>-v<version>，两者版本号必须完全一致。
#   （2026-09 前历史 tag 为 openclaw-desktop-v*/hermes-desktop-v*，之后统一收敛为 <agent>-v*。）
set -euo pipefail

# docker/psql 在本机装于 /usr/local 下（不在最小 PATH；后台 shell 尤其容易缺），显式补上。
export PATH="/usr/local/bin:/usr/local/opt/postgresql@15/bin:$PATH"

CONTEXT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"   # image/
REPO="$(cd "${CONTEXT}/.." && pwd)"
cd "${REPO}"

# ---------- DB（本机 aa_agent；可经环境变量覆盖） ----------
: "${DB_HOST:=127.0.0.1}"; : "${DB_PORT:=5432}"
: "${DB_USER:=aiarsenal}"; : "${DB_NAME:=aa_agent}"; : "${DB_PASS:=aiarsenal123}"

AGENT="${1:?用法: bash image/release.sh <openclaw|hermes> <version>}"
VERSION="${2:?用法: bash image/release.sh <openclaw|hermes> <version>}"

case "${AGENT}" in
  openclaw|hermes) ;;
  *) echo "未知 agent: ${AGENT}（应为 openclaw|hermes）"; exit 1 ;;
esac
DF="image/Dockerfile/${AGENT}/${VERSION}/Dockerfile"
IMAGE="aiarsenal-${AGENT}-desktop"
DISPLAY="$([ "${AGENT}" = openclaw ] && echo OpenClaw || echo Hermes) 桌面 ${VERSION} (Ubuntu/KDE + Selkies)"

GIT_TAG="${AGENT}-v${VERSION}"
IMAGE_TAG="${IMAGE}:${VERSION}"
KEEP_VERSIONS=2   # 每次发版后保留的最新版本号 tag 数（旧版本镜像随发版自动清理，防硬盘堆满）

# ---------- 0. 版本 Dockerfile 就位 + 发版相关文件须干净（image/：Dockerfile/manager/build 脚本） ----------
# 缺版本目录先从模板生成（gen.sh 顺带校验 hermes 源码版本）；生成后与 image/ 其它改动一样须 commit。
if [ ! -f "${DF}" ]; then
  echo "==> 缺 ${DF}，先从模板生成"
  bash image/gen.sh "${AGENT}" "${VERSION}"
fi
# 多人并行：全工作区常有别人的未提交改动（settings/ui 等），不管；但 image/ 须干净，
# 否则 tag 落旧 commit、构建产物与代码不符。
if [ -n "$(git status --porcelain -- image/)" ]; then
  echo "!! image/ 有未提交改动，tag 会落在旧 commit、与构建产物不符；请先 commit image/ 再发版："
  git status -s -- image/; exit 1
fi

# ---------- 1. push master（tag 须指向远端已有 commit，避免悬空）----------
# 见记忆 release-push-code-before-tag：原流程只 push tag 不 push master，tag 会指向远端 master
# 还没有的 commit（悬空）——别人 pull master 看不到改动、rebase 易乱。打 tag 前先把 master 推上去。
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "${BRANCH}" != "master" ]; then
  echo "!! 当前分支 ${BRANCH}，发版须在 master 上（切到 master 再发版）"; exit 1
fi
echo "==> [1/6] push origin master（确保 tag 指向远端 master 已有 commit）"
git push origin master || { echo "!! git push origin master 失败（远端可能有新 commit，先 git pull --rebase 再发版）"; exit 1; }

# ---------- 2. 本地 git tag ----------
echo "==> [2/6] 本地打 tag ${GIT_TAG}"
if git rev-parse "${GIT_TAG}" >/dev/null 2>&1; then
  echo "!! tag ${GIT_TAG} 已存在；换个版本号，或先 git tag -d ${GIT_TAG}（远端已 push 还需 git push origin :refs/tags/${GIT_TAG}）"
  exit 1
fi
git tag -a "${GIT_TAG}" -m "${AGENT} v${VERSION}: 镜像 ${IMAGE_TAG}"
cleanup_tag() { echo "!! 步骤失败，回滚本地 tag ${GIT_TAG}"; git tag -d "${GIT_TAG}" >/dev/null 2>&1 || true; }
trap cleanup_tag ERR

# ---------- 3. 构建 ----------
echo "==> [3/6] 构建镜像 ${IMAGE_TAG}"
# webtop 底座：BuildKit 默认按 registry 最新 digest 重拉 6.48GB（实测一个层卡 0.65MB/s）。
# 用本地镜像的 RepoDigest pin 住 ARG → BuildKit 命中本地、不重拉。（记忆：webtop digest pin 走缓存）
WEBTOP_DIGEST="$(docker image inspect lscr.io/linuxserver/webtop:ubuntu-kde --format '{{index .RepoDigests 0}}' 2>/dev/null || true)"
if [ -z "${WEBTOP_DIGEST}" ]; then
  echo "!! 本地无 lscr.io/linuxserver/webtop:ubuntu-kde，会触发慢速拉取；先 docker pull 它再发版"; exit 1
fi
echo "    webtop digest pin: ${WEBTOP_DIGEST}"
# hermes：构建前校验源码版本与发版版本一致（Dockerfile 内还有 --version 断言双保险）。
if [ "${AGENT}" = "hermes" ]; then
  SRC_VER="$(grep -m1 '^version' "${REPO}/hermes-agent-main/pyproject.toml" 2>/dev/null | cut -d'"' -f2 || true)"
  if [ "${SRC_VER}" != "${VERSION}" ]; then
    echo "!! hermes 源码实际版本 ${SRC_VER:-读不到} 与发版版本 ${VERSION} 不一致；换源码或改版本号"; exit 1
  fi
fi
docker build --build-arg "WEBTOP_BASE_IMAGE=${WEBTOP_DIGEST}" -f "${DF}" -t "${IMAGE_TAG}" "${REPO}"

# ---------- 4. 注册到 DB ----------
echo "==> [4/6] 注册 aa_agent_system_image_setting: ${AGENT}/${VERSION} → ${IMAGE_TAG}"
SYSID="a$(tr -dc 'a-z0-9' </dev/urandom | head -c 30 || true)z"   # 32 位首末字母；|| true 防 tr 被 head 截断后 SIGPIPE+pipefail 误判 ERR 回滚 tag
# stdout 重定向到日志文件（非 /dev/null——出错可查），避免经外部管道时 SIGPIPE 误判 ERR 回滚 tag。
# psql 失败（SQL 错误）仍以非 0 退出码被 set -e 捕获。
PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" > /tmp/release-db.log 2>&1 <<SQL
INSERT INTO aa_agent_system_image_setting
  (system_image_id, agent, version, display_name, image, is_enabled, created_at, updated_at)
VALUES ('${SYSID}', '${AGENT}', '${VERSION}', '${DISPLAY}', '${IMAGE_TAG}', 1, now(), now())
ON CONFLICT (agent, version) DO UPDATE
  SET image=EXCLUDED.image, display_name=EXCLUDED.display_name, is_enabled=1, updated_at=now();
SQL

trap - ERR   # 构建+入库均成功，不再回滚

# ---------- 5. 清理旧镜像（保留最新 N 个版本号）----------
# 每次发版后只留最新 KEEP_VERSIONS 个版本号 tag，避免硬盘被历史镜像（每个 ~7GB）堆满。
# best-effort：被容器引用的镜像 rmi 会失败（说明该版本实例还在用），跳过不阻断发版。
# 仅清理 version-like tag（^[0-9]）；test/latest 等命名 tag 不动，避免误删活跃测试标签。
# 注：macOS 自带 bash 3.2——禁 mapfile / head -n -N 这类 bash4/GNU 写法，用可移植语法。
echo "==> [5/6] 清理 ${IMAGE} 旧镜像（保留最新 ${KEEP_VERSIONS} 个版本号）"
_ALL_TAGS="$(docker images "${IMAGE}" --format '{{.Tag}}' | grep -E '^[0-9]' | sort -V || true)"
_COUNT="$(printf '%s\n' "${_ALL_TAGS}" | grep -c . || true)"
if [ "${_COUNT}" -gt "${KEEP_VERSIONS}" ]; then
  _OLD_TAGS="$(printf '%s\n' "${_ALL_TAGS}" | head -n "$((_COUNT - KEEP_VERSIONS))")"
  for _t in ${_OLD_TAGS}; do
    if docker rmi "${IMAGE}:${_t}" >/dev/null 2>&1; then
      echo "    删除 ${IMAGE}:${_t}"
    else
      echo "    跳过 ${IMAGE}:${_t}（被容器引用，rmi 失败）"
    fi
  done
else
  echo "    无多余旧版本"
fi

# ---------- 6. push tag ----------
echo "==> [6/6] push tag ${GIT_TAG}"
git push origin "${GIT_TAG}"

echo ""
echo "✅ 发版完成： tag=${GIT_TAG}   镜像=${IMAGE_TAG}   DB=${AGENT}/${VERSION}"
echo "   /#/agent/settings/list 刷新可见。"
