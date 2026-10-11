# syntax=docker/dockerfile:1
#
# Dockerfile.openclaw-desktop — aiarsenal 控制面 Ubuntu+KDE 桌面镜像（Selkies-GWS 远程桌面）。
#
# 基础镜像 lscr.io/linuxserver/webtop:ubuntu-kde 自带 Selkies-GWS/pixelflux 流媒体后端（HTTP 3000 +
# selkies WS 8082 + Xvfb/s6-overlay），同时保留 KasmVNC/noVNC（3001）。本 Dockerfile 在其上叠：
#   1. aiarsenal-manager（Go 静态二进制）—— s6 longrun，心跳回连 aiarsenal api + 本地管 openclaw。
#   2. openclaw gateway —— agent fork 子进程守护（不走 s6）。
#   3. Selkies 流媒体修复（见下方 ── Selkies 流媒体修复 ── 块）—— 让无 GPU/瘦客户端环境也能用。
#
# 用户 aa:aa，HOME=/aa（aiarsenal 数据，挂 PVC）；/config=webtop 运行时状态（可丢弃）。
#
# 【模板】本文件是模板（2026.8.2 占位），不要直接 docker build 它——
#   bash image/gen.sh openclaw <version> 生成 image/Dockerfile/openclaw/<version>/Dockerfile（入库可追溯），
#   再 bash image/build.sh openclaw <version> 构建。改了模板就重新 gen 各版本目录同步。
# 访问：实例创建时 3000 自动映射宿主端口，UI「远程桌面」走 aiarsenal.cc 同源反代看桌面。

ARG WEBTOP_BASE_IMAGE=lscr.io/linuxserver/webtop:ubuntu-kde
ARG GO_BASE_IMAGE=golang:1.24-alpine
# openclaw 版本（镜像按「agent+version」单一维度管理，版本即镜像 tag；npm 钉死防漂移）。
# gen.sh 会把 2026.8.2 替换成目标版本（默认值；build 仍可 --build-arg 覆盖）。
ARG OPENCLAW_VERSION=2026.8.2

# ── Stage 1: 构建 aiarsenal-manager（静态二进制；构建上下文=仓库根，路径带 image/ 前缀，同 hermes 模板） ──
FROM ${GO_BASE_IMAGE} AS agent-builder
WORKDIR /src
COPY image/manager/go.* ./
RUN go mod download
COPY image/manager/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/aiarsenal-manager .

# ── Stage 2: webtop ubuntu-kde + openclaw + agent + Selkies 修复 ──
FROM ${WEBTOP_BASE_IMAGE}

# 全局 ARG（FROM 前声明）进不了构建 stage——stage 内使用必须重新声明（无默认值=继承全局/CLI 传入），
# 否则展开为空串：openclaw@空=装 latest（tag 与内容物静默漂移）、aa.json 版本为空。踩过，别删。
ARG OPENCLAW_VERSION

# webtop 默认 USER=abc；装系统包 + 建用户需切 root。
USER root

# 建 aa:aa 用户，HOME=/aa（aiarsenal 专属持久化目录，挂 PVC）。
# /aa 与 webtop 的 /config 分工：/aa=aiarsenal 数据(openclaw配置/工作区/agent状态)随PVC持久化；
# /config=webtop 桌面运行时状态(可丢弃)。gid/uid 1001 可能已被 webtop 占用，故用 -f 容错。
RUN groupadd -f -g 1001 aa && \
    (useradd -u 1001 -g aa -d /aa -m -s /bin/bash aa || usermod -u 1001 -g aa -d /aa -s /bin/bash aa) && \
    mkdir -p /aa && chown -R aa:aa /aa

# 装 openclaw 运行时依赖：Node.js（openclaw 是 Node CLI，nodesource nodejs 自带 npm）+ 工具。
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      nodejs ca-certificates curl procps && \
    rm -rf /var/lib/apt/lists/* && \
    node --version && npm --version

# 装 openclaw 进【镜像层 /opt/openclaw】——不再装 /aa 持久卷。
# 旧方案两个坑：① 装进 /aa + `|| echo` 吞掉失败，构建时 npm 超时/被墙被静默 → ship 出空镜像；
#   ② 装在 /aa 依赖 PVC 首挂 seed 才出现，老卷不 seed。改镜像层后两条都消除。
# 保证三件套：① --registry npmmirror 国内源可靠；② 去 `|| echo` 失败即 fail-fast；
#   ③ 末尾 `openclaw --version` 是构建期断言——装不上 openclaw，docker build 直接红，绝不 ship 空镜像。
# openclaw@${OPENCLAW_VERSION} 钉死版本：镜像 tag=版本号，aa.json 与实际安装版本必须一致，
#   不钉版本 npm 装最新 → tag 与内容物漂移。升级 openclaw = 改 ARG（或 build --build-arg）重建镜像。
# /opt/openclaw 归 root（只读应用码）；状态/配置走 /aa（OPENCLAW_HOME）。
RUN npm install -g --prefix /opt/openclaw --registry https://registry.npmmirror.com "openclaw@${OPENCLAW_VERSION}" && \
    ln -sf /opt/openclaw/bin/openclaw /usr/local/bin/openclaw && \
    openclaw --version | grep -F "${OPENCLAW_VERSION}"

# 不烤 openclaw.json 基线配置——先配置后启动：首挂卷无 openclaw.json，manager 的 ConfigExists()
# 判定 false → 不启动 openclaw，等 api 经 config_push 下发完整配置（含 gateway mode=local/bind=
# loopback/auth + models + agents）写盘后再拉起。旧版烤 stub 反让 ConfigExists 误判 true、用空配置先起。
# 仅预建 workspace 目录（openclaw 启动要用）；config 文件由 config_push 写（PushConfig 会 MkdirAll 父目录）。
RUN mkdir -p /aa/.openclaw/workspace && chown -R aa:aa /aa

# /aa/.aa/aa.json：声明本镜像支持的 agent + 版本，供控制面/manager 读取（决定可下发哪些 agent 命令）。
# 由 OPENCLAW_VERSION 构建期 printf 生成（单一事实来源：装什么版本就声明什么版本，
# 不再有静态 aa.json 忘改版本的漂移）。结构见 image/manager/config.go。
RUN mkdir -p /aa/.aa && \
    printf '{"support":[["openclaw","%s"]]}\n' "${OPENCLAW_VERSION}" > /aa/.aa/aa.json && \
    chown aa:aa /aa/.aa/aa.json && cat /aa/.aa/aa.json

# agent 二进制（aa 可执行）。
COPY --from=agent-builder /out/aiarsenal-manager /usr/local/bin/aiarsenal-manager
RUN chmod +x /usr/local/bin/aiarsenal-manager

# aiarsenal-manager 注册为 webtop s6 服务（/custom-services.d/，s6-overlay v3 longrun）。
# webtop 约定：/custom-services.d/<name> 是一个脚本文件（非目录），/init 启动时复制为 longrun。
COPY image/desktop-root/custom-services.d/aiarsenal-manager/run /custom-services.d/aiarsenal-manager
RUN chmod +x /custom-services.d/aiarsenal-manager

# openclaw 配置路径指向 /aa 下（随 PVC 持久化）。HOME=/aa。
ENV OPENCLAW_CONFIG_PATH=/aa/.openclaw/openclaw.json \
    OPENCLAW_HOME=/aa/.openclaw \
    OPENCLAW_CONFIG_DIR=/aa/.openclaw \
    OPENCLAW_WORKSPACE_DIR=/aa/.openclaw/workspace \
    HOME=/aa \
    TITLE="AI Arsenal OpenClaw" \
    SUBFOLDER="/" \
    AIARSENAL_MANAGER_ENABLED=true

# 预装微信插件 @tencent-weixin/openclaw-weixin（第三方渠道插件，非 stock）。
# 装进 /aa/.openclaw（OPENCLAW_HOME）——新实例首挂卷 seed 即有；老卷需重建实例才拿到。
# 不在运行时装（manager 的 channel_login 不再调 plugins install）：运行时装慢（~5min npm）且需重启
# gateway 才加载，会把 channel_login 命令卡死在第一步。这里构建时装好，gateway 启动即加载。
# 以 aa 身份装（运行时 gateway=aa，保留属主）；npm 走 npmmirror 国内源（与 openclaw 主体一致，加速）。
# 失败即 fail-fast（绝不 ship 没插件的镜像）；末尾断言插件目录已生成。
USER aa
RUN npm_config_registry=https://registry.npmmirror.com openclaw plugins install @tencent-weixin/openclaw-weixin && \
    test -n "$(ls -d /aa/.openclaw/npm/projects/*weixin* 2>/dev/null)"
USER root

# ── Selkies 流媒体修复（无 GPU/瘦客户端/反代链路友好）─────────
# webtop 内置 Selkies 默认配置在本环境跑不起来或体验差，下面逐项修正：
# 1) PIXELFLUX_WAYLAND=false：webtop 内置 true → svc-xorg sleep infinity 不起 X server。
#    宿主机无 GPU/Wayland 合成器，必须走 Xvfb 软件渲染。
ENV PIXELFLUX_WAYLAND=false
# 2) svc-selkies 的 run 脚本 `until [ -f /defaults/pid ]` 死等一个无人写的 pid 文件
#    → 流后端 8082 永不启动 → 浏览器 WS 永远连不上。脚本顶部 touch audio.lock 跳过该块
#    （idempotent，grep 守卫防重复插入）。代价：无音频（PulseAudio 块被跳过），画面/键鼠正常。
RUN grep -q 'PATCH: skip broken pid-wait' /etc/s6-overlay/s6-rc.d/svc-selkies/run || \
    sed -i '/^#!\/usr\/bin\/with-contenv/a touch /dev/shm/audio.lock #PATCH: skip broken pid-wait+audio block' /etc/s6-overlay/s6-rc.d/svc-selkies/run
# 3) 纯 JPEG 编码 + 关 PaintOver：默认 x264enc 出 H.264，客户端需 WebCodecs 软解（要 AVX2），
#    无 AVX2 设备报 "your platform is not supported... avx2"；PaintOver 子层也用 H264 同理。
#    ENCODER=jpeg + USE_PAINT_OVER_QUALITY=false → 全链路纯 JPEG（createImageBitmap 解码），零客户端解码器依赖。
# 4) 流瘦身：1280x720@20fps/jpeg_q55 + 锁 manual 分辨率（阻止浏览器 retina 动态拉高分致反代背压断流）。
ENV SELKIES_ENCODER=jpeg \
    SELKIES_MANUAL_WIDTH=1280 \
    SELKIES_MANUAL_HEIGHT=720 \
    SELKIES_IS_MANUAL_RESOLUTION_MODE=true \
    SELKIES_FRAMERATE=20 \
    SELKIES_JPEG_QUALITY=55 \
    SELKIES_USE_PAINT_OVER_QUALITY=false
# 5) Selkies 前端 localStorage 前缀：默认按 window.location.href（含 token-in-path）作前缀，
#    经同源反代每次开桌面=新 token=新前缀，写一整套设置 key 且不清理→堆积。改成固定前缀 selkies。
#    按正则字面量匹配（不依赖压缩变量名），覆盖 web / selkies-dashboard / selkies-dashboard-wish 三 bundle。
RUN find /usr/share/selkies -name 'index-*.js' -path '*assets*' \
    -exec sed -i 's|window\.location\.href\.split("#")\[0\]\.replace(/\[\^a-zA-Z0-9.-_\]/g,"_")|"selkies"|g' {} +

# Selkies 桌面 HTTP 3000（主，UI 远程桌面走它）；KasmVNC noVNC 3001（备用）。
EXPOSE 3000 3001

# 用 webtop 原生 entrypoint /init（s6-overlay 当 PID 1，拉起桌面 + agent 服务）。不覆盖。
