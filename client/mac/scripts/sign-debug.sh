#!/bin/bash
# debug 包固定证书重签——TCC 权限（屏幕录制/辅助功能）按代码签名认 App，
# debug 构建默认 ad-hoc 签名每次编译都变，权限条目跟着作废（每次重编译都要去
# 系统设置重新勾一次）。用钥匙串里固定的 Apple Development 证书重签后签名恒定，
# 授权一次跨编译保持。
#
# ⚠️ 不能用自签证书：macOS TCC 不认不受信任的自签签名（开关弹回/授权被忽略，
# 实踩 2026-08-15：CN=aiarsenal-dev 自签证书怎么勾都 unauthorized）。
# 必须用真实 Apple Development 证书（Xcode 登录免费 Apple ID 即可创建）。
#
# 证书选择顺序（指纹优先，避开同名歧义——维护者本机有两条同名证书）：
#   1. 环境变量 AARS_SIGN_IDENTITY（指纹或完整 CN 均可）
#   2. 下方 PINNED_HASH 指纹（维护者机器）
#   3. 钥匙串里任意 "Apple Development" 证书
# 都没有 → 提示后跳过（app 仍可用，仅 TCC 权限每次重编译需重新授权）。
set -e
cd "$(dirname "$0")/.."   # 到 client/mac（tauri 项目根）

PINNED_HASH="7883533D87CB5E775FE704E442D4CD624B60554F"   # 本机 Apple Development 证书指纹（security find-identity 可查；换证书须同步更新）
APP="src-tauri/target/debug/bundle/macos/AI Arsenal.app"
[ -d "$APP" ] || { echo "bundle 不存在: $APP（先跑 pnpm build:debug）"; exit 1; }

IDENTITIES=$(security find-identity -v -p codesigning)

IDENTITY=""
if [ -n "$AARS_SIGN_IDENTITY" ]; then
  IDENTITY="$AARS_SIGN_IDENTITY"
elif echo "$IDENTITIES" | grep -q "$PINNED_HASH"; then
  IDENTITY="$PINNED_HASH"
else
  # 取第一条 Apple Development 证书的指纹（行格式：1) HEX "Apple Development: name (team)"）
  IDENTITY=$(echo "$IDENTITIES" | grep '"Apple Development' | head -1 | sed -E 's/^[[:space:]]*[0-9]+\)[[:space:]]+([A-F0-9]+).*/\1/')
fi

if [ -z "$IDENTITY" ]; then
  echo "⚠️  钥匙串里没有 Apple Development 证书，跳过重签（ad-hoc 签名，App 可正常使用，"
  echo "    但每次重编译后屏幕录制/辅助功能权限需在系统设置重新授权一次）。"
  echo "    想固定签名：Xcode → Settings → Accounts → 登录 Apple ID → Manage Certificates →"
  echo "    创建 Apple Development，然后重新跑 pnpm build:debug。"
  exit 0
fi

codesign --force --deep --sign "$IDENTITY" "$APP"
codesign --verify --strict "$APP" 2>&1
codesign -dv "$APP" 2>&1 | grep -E 'TeamIdentifier|^Authority' || true
echo "已用 Apple Development 固定签名（TCC 权限跨编译保持）"
