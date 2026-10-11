#!/usr/bin/env bash
# ============================================================================
# AI Arsenal 桌面客户端后端端点验证脚本
#
# 验证 /api/v1/system/client/* 四个端点的完整链路：
#   exchange(pending) → login → bind → exchange(ok) → get clients → report
#
# 前提：
#   1. 后端已起，且 APP_DEBUG=1（登录跳过图形验证码；否则需手工处理验证码）
#   2. 已装 curl + jq
#   3. desktop 库已建表并种入三客户端规格（见 resource/data/desktop.sql）
#
# 用法:
#   bash desktop-verify.sh                         # 默认 http://api.aiarsenal.cc
#   bash desktop-verify.sh http://api.aiarsenal.cc # 指定 base
#   USERNAME=admin PASSWORD=123456 bash desktop-verify.sh
# ============================================================================
set -euo pipefail

BASE="${1:-http://api.aiarsenal.cc}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-123456}"
API="${BASE}/api/v1/system"

command -v jq >/dev/null || { echo "需要 jq"; exit 1; }

DC="verify-$(date +%s)-$$"
pass=0; fail=0
ok() { echo "  ✅ $1"; pass=$((pass+1)); }
ko() { echo "  ❌ $1"; fail=$((fail+1)); }

echo "目标: ${API}  deviceCode=${DC}"

# ---- 1. exchange（未绑定，应 pending）----
echo "[1] exchange(未绑定) → 期望 pending"
r=$(curl -s -X POST "${API}/client/exchange" -H 'Content-Type: application/json' -d "{\"deviceCode\":\"${DC}\"}")
st=$(echo "$r" | jq -r '.data.status // empty' 2>/dev/null || echo "")
if [ "$st" = "pending" ]; then ok "exchange pending"; else ko "exchange pending 失败: $r"; fi

# ---- 2. 登录（debug 模式跳过验证码）----
echo "[2] login"
r=$(curl -s -X POST "${API}/login" -H 'Content-Type: application/json' \
    -d "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD}\",\"verifyKey\":\"\",\"verifyCode\":\"\"}")
TOKEN=$(echo "$r" | jq -r '.data.token // empty' 2>/dev/null || echo "")
if [ -n "$TOKEN" ]; then ok "登录拿到 token"; else ko "登录失败: $r"; exit 1; fi

# ---- 3. bind（带 token 绑定 deviceCode）----
echo "[3] bind"
r=$(curl -s -X POST "${API}/client/bind" -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" -d "{\"deviceCode\":\"${DC}\"}")
b=$(echo "$r" | jq -r '.data.bound // empty' 2>/dev/null || echo "")
if [ "$b" = "true" ]; then ok "bind 成功"; else ko "bind 失败: $r"; fi

# ---- 4. exchange（已绑定，应 ok + 返回 token）----
echo "[4] exchange(已绑定) → 期望 ok"
r=$(curl -s -X POST "${API}/client/exchange" -H 'Content-Type: application/json' -d "{\"deviceCode\":\"${DC}\"}")
st=$(echo "$r" | jq -r '.data.status // empty' 2>/dev/null || echo "")
t2=$(echo "$r" | jq -r '.data.token // empty' 2>/dev/null || echo "")
if [ "$st" = "ok" ] && [ -n "$t2" ]; then ok "exchange ok + token"; else ko "exchange ok 失败: $r"; fi

# ---- 5. exchange 再来一次（一次性，应 pending）----
echo "[5] exchange(重复消费) → 期望 pending"
r=$(curl -s -X POST "${API}/client/exchange" -H 'Content-Type: application/json' -d "{\"deviceCode\":\"${DC}\"}")
st=$(echo "$r" | jq -r '.data.status // empty' 2>/dev/null || echo "")
if [ "$st" = "pending" ]; then ok "deviceCode 一次性消费"; else ko "一次性失败: $r"; fi

# ---- 6. get clients ----
echo "[6] get clients"
r=$(curl -s -X GET "${API}/client/clients" -H "Authorization: Bearer ${TOKEN}")
n=$(echo "$r" | jq -r '[.data.openclaw,.data.hermes,.data.obsidian] | length' 2>/dev/null || echo 0)
keys=$(echo "$r" | jq -r '.data | [keys[]] | join(",")' 2>/dev/null || echo "")
if [ -n "$keys" ]; then ok "get clients → ${keys}"; else ko "get clients 失败: $r"; fi

# ---- 7. report ----
echo "[7] report"
r=$(curl -s -X POST "${API}/client/clients/report" -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d '{"client":"openclaw","status":"stopped","version":"v2026.5.4","pidOrContainerId":"","uptimeSeconds":0,"lastError":"","extra":""}')
rp=$(echo "$r" | jq -r '.data.reported // empty' 2>/dev/null || echo "")
if [ "$rp" = "true" ]; then ok "report 成功"; else ko "report 失败: $r"; fi

echo "----------------------------------"
echo "结果: ${pass} 通过 / ${fail} 失败"
[ "$fail" -eq 0 ]
