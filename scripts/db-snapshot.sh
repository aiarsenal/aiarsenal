#!/usr/bin/env bash
# ============================================================================
# scripts/db-snapshot.sh — 用户自配数据快照/回灌（防 db-init 清库丢数据）
#
# 背景：local-test.sh db-init / deploy 按 base SQL 每表 DROP 重建（「全新部署」语义），
# 已配置的用户数据（MCP 分组/LLM 凭证/实例/账号/日程…）会一并冲掉——2026-09-12、
# 09-13 连踩两次。本脚本补安全网：把用户数据持续快照，清库后一条命令找回。
#
# 快照格式：pg_dump --data-only --column-inserts（按列名 INSERT，schema 加新列仍兼容，
# 新列走默认值）+ gzip。含 schema_migrations 行，回灌后迁移状态与快照时点一致。
# 恢复 = db-init 重建 schema（以最新 base SQL 为准）→ TRUNCATE 全库表（清掉 db-init
# 灌的种子，防主键冲突）→ 灌快照数据。净效果：新 schema + 快照时点的完整数据。
#
# 用法：
#   bash scripts/db-snapshot.sh backup          # 立即快照 → .run/db-snapshots/<时间戳>/
#   bash scripts/db-snapshot.sh restore latest  # 回灌最近一份快照（也可指定目录名）
#   bash scripts/db-snapshot.sh list            # 列出快照
#   bash scripts/db-snapshot.sh install-hourly  # 装 launchd：每小时自动 backup，保留 72 份（3 天）
#
# ⚠️ 快照含 LLM API key 明文：.run/db-snapshots 已 gitignore，绝不入库/外传。
# ============================================================================
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
PG_BIN="/usr/local/opt/postgresql@15/bin"
PSQL="$PG_BIN/psql"
PGDUMP="$PG_BIN/pg_dump"
PGCTL="$PG_BIN/pg_ctl"
PGDATA="/usr/local/var/postgresql@15"
SNAP_ROOT="$REPO/.run/db-snapshots"
LOG="$REPO/.run/logs/db-snapshot.log"
KEEP=72          # 每小时一份 × 72 = 保留 3 天
# 库列表以实际存在的为准（新库自动带上，不硬编码遗漏）
ALL_DBS="aiarsenal aa_agent aa_client aa_doc aa_kb aa_llm aa_mcp aa_project aa_schedule aa_skill"

export LC_ALL="${LC_ALL:-en_US.UTF-8}"   # 本机 postgres 启动必须带（否则 FATAL: multithreaded）

log() { echo "[$(date '+%F %T')] $*"; }

ensure_postgres() {
  "$PGCTL" -D "$PGDATA" status >/dev/null 2>&1 && return 0
  log "postgres 未运行：bash scripts/local-test.sh start 会一并拉起；本次跳过"
  return 1
}

existing_dbs() {
  local out=()
  for db in $ALL_DBS; do
    "$PSQL" postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$db'" 2>/dev/null | grep -q 1 && out+=("$db")
  done
  echo "${out[*]:-}"
}

cmd_backup() {
  ensure_postgres || return 0
  local ts dir db failed=0
  ts="$(date +%Y%m%d-%H%M%S)"
  dir="$SNAP_ROOT/$ts"
  mkdir -p "$dir"
  log "==> 快照 → $dir"
  for db in $(existing_dbs); do
    if "$PGDUMP" --data-only --column-inserts "$db" 2>>"$LOG" | gzip > "$dir/$db.sql.gz"; then
      log "    $db ✓ ($(du -h "$dir/$db.sql.gz" | cut -f1))"
    else
      log "    ${db} ✗ dump 失败（详见 ${LOG}）"; failed=1
    fi
  done
  # gunzip -t 完整性校验：任何一份损坏整体作废（半套快照恢复出半套数据更糟）
  for f in "$dir"/*.sql.gz; do
    gunzip -t "$f" 2>/dev/null || { log "    $f 损坏，作废本快照"; failed=1; }
  done
  if [ "$failed" = 1 ]; then rm -rf "$dir"; log "!! 快照失败已清理"; exit 1; fi
  date +%s > "$dir/.ts"
  # 只保留最近 KEEP 份（BSD head 无负行数，按总数算截断点）
  local total
  total="$(ls -1d "$SNAP_ROOT"/20* 2>/dev/null | wc -l | tr -d ' ')"
  if [ -n "$total" ] && [ "$total" -gt "$KEEP" ]; then
    ls -1d "$SNAP_ROOT"/20* | head -n $((total - KEEP)) | while read -r old; do
      rm -rf "$old"; log "    清理旧快照 $(basename "$old")"
    done
  fi
  log "✓ 快照完成 $(basename "$dir")（共 $(ls "$dir" | grep -c sql.gz) 库）"
}

cmd_restore() {
  local target="${1:-latest}" dir
  if [ "$target" = "latest" ]; then
    dir="$(ls -1d "$SNAP_ROOT"/20* 2>/dev/null | tail -1)" || true
    [ -n "$dir" ] || { log "!! 无快照可恢复（$SNAP_ROOT 为空）"; exit 1; }
  else
    dir="$SNAP_ROOT/$target"
  fi
  [ -d "$dir" ] || { log "!! 快照目录不存在: $dir"; exit 1; }
  ensure_postgres
  log "==> 回灌 ${dir}（schema 走最新 base SQL，数据回到快照时点）"
  echo "    将 TRUNCATE 全部业务表并灌入快照数据，当前库内未入快照的改动会丢。"
  read -r -p "    确认恢复？[y/N] " ans
  [ "$ans" = "y" ] || { log "已取消"; exit 0; }
  # 1) 重建 schema（全新部署语义；db-init 自己会 ensure postgres）
  bash "$REPO/scripts/local-test.sh" db-init >>"$LOG" 2>&1
  # 2) 逐库：TRUNCATE 全表（清 db-init 灌的种子，防主键冲突）→ 灌快照
  for db in $(existing_dbs); do
    f="$dir/$db.sql.gz"
    [ -f "$f" ] || { log "    $db 无快照文件，跳过"; continue; }
    "$PSQL" -d "$db" -tAc "SELECT 'TRUNCATE TABLE ' || string_agg(format('%I.%I', schemaname, tablename), ', ') || ' CASCADE;'
      FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')" | "$PSQL" -d "$db" >>"$LOG" 2>&1
    # ON_ERROR_STOP 不开：schema 已删列等不兼容时尽量灌，错误行留日志供人工核对
    gunzip -c "$f" | "$PSQL" -d "$db" -v ON_ERROR_STOP=0 -q >>"$LOG" 2>&1
    log "    $db ✓"
  done
  # 3) 汇总灌入报错（正常应为 0 行；有错说明 schema 与快照有不兼容，人工看日志）
  local errs; errs=$(grep -c "^ERROR" "$LOG" 2>/dev/null || echo 0)
  log "✓ 回灌完成（日志 ${LOG}；本次日志内 ERROR $errs 处，非 0 需人工核对）"
}

cmd_list() {
  log "快照目录 ${SNAP_ROOT}："
  ls -1d "$SNAP_ROOT"/20* 2>/dev/null | while read -r d; do
    echo "    $(basename "$d")  $(ls "$d" | grep -c sql.gz) 库  $(du -sh "$d" | cut -f1)"
  done || log "（空）"
}

cmd_install_hourly() {
  local plist="$HOME/Library/LaunchAgents/aiarsenal.db-snapshot.plist"
  mkdir -p "$HOME/Library/LaunchAgents" "$REPO/.run/logs"
  cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>aiarsenal.db-snapshot</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>$REPO/scripts/db-snapshot.sh</string>
    <string>backup</string>
  </array>
  <key>StartInterval</key><integer>3600</integer>
  <key>StandardOutPath</key><string>$LOG</string>
  <key>StandardErrorPath</key><string>$LOG</string>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
EOF
  launchctl bootstrap "gui/$(id -u)" "$plist" 2>/dev/null || launchctl load -w "$plist"
  log "✓ 已装每小时快照（launchd: aiarsenal.db-snapshot，装后立即跑一次）"
  log "    停用：launchctl bootout gui/$(id -u)/aiarsenal.db-snapshot"
}

case "${1:-}" in
  backup)        cmd_backup ;;
  restore)       shift; cmd_restore "${1:-latest}" ;;
  list)          cmd_list ;;
  install-hourly) cmd_install_hourly ;;
  *) sed -n '2,25p' "$0"; exit 1 ;;
esac
