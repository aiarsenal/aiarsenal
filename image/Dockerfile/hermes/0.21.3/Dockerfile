# syntax=docker/dockerfile:1
#
# Dockerfile.hermes-desktop — aiarsenal 控制面 Ubuntu+KDE 桌面镜像（Selkies-GWS 远程桌面）+ hermes。
#
# 与 image/Dockerfile/openclaw/Dockerfile.openclaw-desktop 同构：webtop:ubuntu-kde 底座
# （Selkies 流媒体 + KDE 桌面，/init 为 PID 1）+ aiarsenal-manager（Go 静态二进制，s6 longrun）作为
# 「唯一 manager」fork+守护被管 agent。区别仅在被管 agent：openclaw(Node CLI) → hermes(Python venv CLI)。
#
# 关键（用户要求）：hermes gateway 不另起 s6 服务，由 aiarsenal-manager 以
# `hermes gateway run --external-supervisor` 前台 fork 出来并守护（AIARSENAL_RUNTIME=hermes
# 让 manager 走 HermesManager）。hermes 装在 final 阶段就地构建，venv 的 python 引用路径与
# 运行时一致，避免跨镜像拷 venv 的 ABI/python 路径地雷（同 openclaw-desktop 就地 npm install）。
#
# 【模板】本文件是模板（0.21.3 占位），不要直接 docker build 它——
#   bash image/gen.sh hermes <version> 生成 image/Dockerfile/hermes/<version>/Dockerfile（入库可追溯），
#   再 bash image/build.sh hermes <version> 构建。改了模板就重新 gen 各版本目录同步。
# 构建上下文 = 仓库根（因需 hermes 源码目录 hermes-agent-main/，可用 HERMES_SRC 覆盖）。
# 访问：实例创建时 3000 自动映射宿主端口，UI「远程桌面」走 aiarsenal.cc 同源反代看桌面。

ARG WEBTOP_BASE_IMAGE=lscr.io/linuxserver/webtop:ubuntu-kde
ARG GO_BASE_IMAGE=golang:1.24-alpine
ARG UV_IMAGE=ghcr.io/astral-sh/uv:0.11.6
# hermes 版本（镜像按「agent+version」单一维度管理，版本即镜像 tag）。hermes 是本地源码安装
# （无 registry 可钉），gen.sh 会校验源码 pyproject.toml 的 version 与之一致（末尾 `hermes --version`
# 断言双保险）。aa.json 同样由它生成，杜绝声明漂移。
ARG HERMES_VERSION=0.21.3
# hermes 源码目录（构建上下文=仓库根，相对路径）。约定 hermes-agent-main/（gitignored，
# 由 build.sh 从仓库根的 hermes-agent-*.zip 自动解压）。
ARG HERMES_SRC=hermes-agent-main

# ── Stage 0: uv 二进制源（COPY --from 不支持 ARG 变量展开，需独立 stage） ────
FROM ${UV_IMAGE} AS uv-src

# ── Stage 1: 构建 aiarsenal-manager（静态二进制，泛化版 manager） ──────────────
# 路径相对仓库根 context：image/manager/。
FROM ${GO_BASE_IMAGE} AS agent-builder
WORKDIR /src
COPY image/manager/go.* ./
RUN go mod download
COPY image/manager/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/aiarsenal-manager .

# ── Stage 2: webtop ubuntu-kde + hermes + agent + Selkies 修复 ──
FROM ${WEBTOP_BASE_IMAGE}

# 全局 ARG（FROM 前声明）进不了构建 stage——stage 内使用必须重新声明（无默认值=继承全局/CLI 传入），
# 否则展开为空串：COPY ${HERMES_SRC} 空=拷了个空目录（uv sync 报无 pyproject）、版本断言/aa.json 全空。踩过，别删。
ARG HERMES_VERSION
ARG HERMES_SRC

# webtop 默认 USER=abc；装系统包 + 建用户需切 root。
USER root

# 建 aa:aa 用户，HOME=/aa（aiarsenal 专属持久化目录，挂 PVC）。与 openclaw-desktop 一致。
RUN groupadd -f -g 1001 aa && \
    (useradd -u 1001 -g aa -d /aa -m -s /bin/bash aa || usermod -u 1001 -g aa -d /aa -s /bin/bash aa) && \
    mkdir -p /aa && chown -R aa:aa /aa

# hermes 运行时系统依赖（核心 Python deps 多为 wheel 自带，这里补系统工具/库）。
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ffmpeg git ripgrep ca-certificates curl procps && \
    rm -rf /var/lib/apt/lists/*

# uv 二进制（从官方 uv 镜像拷 /uv，避免 curl 装 uv 的网络依赖）。
COPY --from=uv-src /uv /usr/local/bin/uv

# ── hermes 就地安装（核心依赖 only）──────────────────────────────
# UV_PYTHON_INSTALL_DIR=/opt/uv-python（世界可读），保证 aa 能执行 venv 引用的 python
# （uv 默认装在 /root/.local，aa 无权访问）。UV_INDEX_URL 走国内镜像降网络风险（build-arg 可覆盖）。
ARG UV_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
ENV UV_PYTHON_INSTALL_DIR=/aa/.uv-python \
    UV_LINK_MODE=copy \
    UV_INDEX_URL=${UV_INDEX_URL}
# hermes venv 就地构建在 /aa/hermes/.venv（随 PVC 持久化；venv shebang/绝对路径才正确）。
# 末尾断言 `hermes --version` 含 HERMES_VERSION——源码实际版本与声明不符（aa.json）直接构建红。
COPY ${HERMES_SRC} /aa/hermes
RUN cd /aa/hermes && \
    uv python install 3.13 && \
    uv venv --python 3.13 .venv && \
    uv sync --frozen --no-install-project --extra messaging --extra anthropic --extra mcp && \
    uv pip install --python .venv/bin/python --no-deps -e . && \
    .venv/bin/hermes --version | grep -F "${HERMES_VERSION}"
# --extra messaging：装 aiohttp/qrcode 等，微信渠道（gateway/platforms/weixin.py 的 qr_login）
# 与 weixin 平台启动依赖。uv sync --frozen --no-install-project 默认只装核心依赖，
# 不含 messaging 可选组 → qr_login 会 raise "aiohttp is required"。
# --extra anthropic：api_mode=anthropic_messages（anthropic 族模型，控制面按 provider 自动分流）
# 走原生 Anthropic 协议，agent/anthropic_adapter.py import anthropic 包——缺了每条消息
# ImportError 兜底错误文案（auto/glm-5.3 微信实踩；==0.87.0 为 CVE 修复版，pyproject 锁定）。
# --extra mcp：tools/mcp_tool.py import mcp（+httpx2/starlette）——缺了 MCP 静默降级：
# 注册期 ImportError 只落 DEBUG 日志（gateway/run.py 启动 executor 里），实例绑了 MCP 分组
# 也一个工具不注册，INFO 日志无任何异常（微信知识库检索实踩）。
RUN chown -R aa:aa /aa/hermes /aa/.uv-python

# /aa/.aa/aa.json：声明本镜像支持的 agent + 版本，供控制面/manager 读取（决定可下发哪些 agent 命令）。
# 由 HERMES_VERSION 构建期 printf 生成（与源码实际版本经上方断言一致；不再有静态 aa.json 忘改的漂移）。
# 结构见 image/manager/config.go。
RUN mkdir -p /aa/.aa && \
    printf '{"support":[["hermes","%s"]]}\n' "${HERMES_VERSION}" > /aa/.aa/aa.json && \
    chown aa:aa /aa/.aa/aa.json && cat /aa/.aa/aa.json
# /usr/local/bin/hermes 软链到 venv 二进制：桌面终端里直接敲 hermes 可用。
# 注意：不把 venv 加进 PATH（webtop 自带 VIRTUAL_ENV=/lsiopy + python3.14，Selkies/桌面依赖它；
# 改 python3 解析会破坏远程桌面）。manager 走绝对路径（HERMES_BINARY 指向 /aa），不受影响。
RUN ln -sf /aa/hermes/.venv/bin/hermes /usr/local/bin/hermes

# agent 二进制（aa 可执行）。
COPY --from=agent-builder /out/aiarsenal-manager /usr/local/bin/aiarsenal-manager
RUN chmod +x /usr/local/bin/aiarsenal-manager

# aiarsenal-manager 注册为 webtop s6 服务（复用 openclaw-desktop 的 longrun 脚本：
# runuser -u aa -- /usr/local/bin/aiarsenal-manager）。
COPY image/desktop-root/custom-services.d/aiarsenal-manager/run /custom-services.d/aiarsenal-manager
RUN chmod +x /custom-services.d/aiarsenal-manager

# hermes 配置/状态指向 /aa 下（随 PVC 持久化）。AIARSENAL_RUNTIME=hermes 让 manager 用 HermesManager。
# HERMES_DISABLE_LAZY_INSTALLS=1：核心依赖已烤进 venv；禁掉懒加载避免它 shell 调 pip 时被
# webtop 的 VIRTUAL_ENV=/lsiopy(python3.14) 误导，装错环境。需补 provider 时改 Dockerfile 加 extra 重建。
ENV HERMES_HOME=/aa/.hermes \
    HERMES_WRITE_SAFE_ROOT=/aa/.hermes \
    HERMES_DISABLE_LAZY_INSTALLS=1 \
    AIARSENAL_RUNTIME=hermes \
    HERMES_BINARY=/aa/hermes/.venv/bin/hermes \
    HOME=/aa \
    TITLE="AI Arsenal Hermes" \
    SUBFOLDER="/" \
    AIARSENAL_MANAGER_ENABLED=true

# ── Selkies 流媒体修复（无 GPU/瘦客户端/反代链路友好；逐项同 openclaw-desktop）──
# 1) PIXELFLUX_WAYLAND=false：宿主机无 GPU/Wayland，必须走 Xvfb 软件渲染。
ENV PIXELFLUX_WAYLAND=false
# 2) svc-selkies 的 run 脚本死等一个无人写的 pid 文件 → 流后端 8082 永不启动。脚本顶部 touch
#    audio.lock 跳过该块（idempotent，grep 守卫防重复插入）。代价：无音频，画面/键鼠正常。
RUN grep -q 'PATCH: skip broken pid-wait' /etc/s6-overlay/s6-rc.d/svc-selkies/run || \
    sed -i '/^#!\/usr\/bin\/with-contenv/a touch /dev/shm/audio.lock #PATCH: skip broken pid-wait+audio block' /etc/s6-overlay/s6-rc.d/svc-selkies/run
# 3)+4) 纯 JPEG 编码 + 关 PaintOver + 瘦身（零客户端解码器依赖，防反代背压断流）。
ENV SELKIES_ENCODER=jpeg \
    SELKIES_MANUAL_WIDTH=1280 \
    SELKIES_MANUAL_HEIGHT=720 \
    SELKIES_IS_MANUAL_RESOLUTION_MODE=true \
    SELKIES_FRAMERATE=20 \
    SELKIES_JPEG_QUALITY=55 \
    SELKIES_USE_PAINT_OVER_QUALITY=false
# 5) Selkies 前端 localStorage 固定前缀 selkies（防 token-in-path 经反代每次堆积）。
#    按正则字面量匹配（不依赖压缩变量名），一次性覆盖 web / selkies-dashboard /
#    selkies-dashboard-wish 三个 bundle：把「URL(含 token) 前缀表达式」整体换成常量 selkies。
RUN find /usr/share/selkies -name 'index-*.js' -path '*assets*' \
    -exec sed -i 's|window\.location\.href\.split("#")\[0\]\.replace(/\[\^a-zA-Z0-9.-_\]/g,"_")|"selkies"|g' {} +

# Selkies 桌面 HTTP 3000（主，UI 远程桌面走它）；KasmVNC noVNC 3001（备用）。
EXPOSE 3000 3001

# 用 webtop 原生 entrypoint /init（s6-overlay 当 PID 1，拉起桌面 + agent 服务）。不覆盖。
