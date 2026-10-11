//! exec 审批策略文件：`~/.openclaw/exec-approvals.json`（`$OPENCLAW_STATE_DIR` 覆盖时用覆盖路径）。
//!
//! 与 openclaw CLI 互操作——文件格式、normalize、CAS 写、glob/argPattern 匹配语义逐条对齐
//! openclaw 2026.7.1 源码（src/infra/exec-approvals.ts / exec-allowlist-pattern.ts /
//! exec-command-resolution.ts matchAllowlist / node-host/invoke.ts get|set handler）。
//!
//! 关键语义（出处见各函数注释）：
//! - 文件缺失 = 策略 (full, off)——装好零行为变化，首启不落盘；get/set 命令侧 ensure 才创建。
//! - set 走 baseHash CAS（exists 时必带 baseHash 且须等于当前文件 sha256，防跨进程并发覆盖）。
//! - Windows 的「总是允许」entry 带 argPattern（buildArgPatternFromArgv 仅 Windows 生成；
//!   mac 版不带——同源 TS 分支语义，client 生成与匹配两侧自洽）。
//! - normalize 会剥 entry.commandText（openclaw stripAllowlistCommandText），信息留在 lastUsedCommand。
//!
//! Windows 平台差异（对 mac exec_approvals.rs）：
//! - PATH 分隔 ';' + PATHEXT（.exe/.cmd/.bat）可执行判定（unix 是 0o111 权限位）。
//! - 路径归一 `\\`→`/`（Windows 反斜杠路径对 glob 语义）；glob 匹配大小写不敏感
//!   （Windows 文件系统大小写不敏感）；posix_normalize 的「绝对」判定含盘符 `X:/`。
//! - 原子写不做 0600 chmod（无 unix 权限位；用户目录内文件默认按 ACL 保护，语义等价）。

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Condvar, Mutex, MutexGuard};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

/// openclaw routing/session-key.ts DEFAULT_AGENT_ID：gateway 主 agent 的策略键。
const DEFAULT_AGENT_ID: &str = "main";
const LEGACY_AGENT_KEY: &str = "default";
const WILDCARD_AGENT_KEY: &str = "*";

/// 进程内串行化文件读写（跨进程靠 baseHash CAS）。
static FILE_LOCK: Mutex<()> = Mutex::new(());

fn file_lock() -> MutexGuard<'static, ()> {
    FILE_LOCK.lock().unwrap_or_else(|p| p.into_inner())
}

// ===== 数据结构（serde 对齐 openclaw exec-approvals.json，camelCase；全字段宽松可缺省）=====

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct ApprovalsFile {
    pub version: i64,
    pub socket: Option<SocketInfo>,
    pub defaults: Option<PolicyFields>,
    pub agents: Option<BTreeMap<String, AgentEntry>>,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct SocketInfo {
    pub path: Option<String>,
    pub token: Option<String>,
}

/// defaults 与 agents.<id> 共用的策略字段（openclaw ExecApprovalsDefaults / ExecApprovalsAgent）。
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct PolicyFields {
    /// deny | allowlist | full
    pub security: Option<String>,
    /// off | on-miss | always
    pub ask: Option<String>,
    /// deny | allowlist | full（审批超时/拒绝回退）
    pub ask_fallback: Option<String>,
    pub auto_allow_skills: Option<bool>,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct AgentEntry {
    #[serde(flatten)]
    pub policy: PolicyFields,
    pub allowlist: Option<Vec<AllowlistEntry>>,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct AllowlistEntry {
    pub id: Option<String>,
    pub pattern: Option<String>,
    pub source: Option<String>,
    /// openclaw normalize 会剥掉（stripAllowlistCommandText）；反序列化容忍存量。
    pub command_text: Option<String>,
    pub arg_pattern: Option<String>,
    /// epoch 毫秒
    pub last_used_at: Option<f64>,
    pub last_used_command: Option<String>,
    pub last_resolved_path: Option<String>,
}

struct Snapshot {
    path: String,
    exists: bool,
    hash: String,
    file: ApprovalsFile,
}

// ===== 路径解析（exec-approvals.ts resolveExecApprovalsStateDir/Path/SocketPath）=====

fn home() -> Option<PathBuf> {
    dirs::home_dir()
}

/// `~` / `~/x` 展开为 home（home-dir.ts expandHomePrefix：仅 `~` 或后跟 / \ 才展开，`~user` 原样）。
fn expand_home(input: &str) -> String {
    if !input.starts_with('~') {
        return input.to_string();
    }
    let rest = &input[1..];
    if !rest.is_empty() && !rest.starts_with('/') && !rest.starts_with('\\') {
        return input.to_string();
    }
    match home() {
        Some(h) => h.to_string_lossy().to_string() + rest,
        None => input.to_string(),
    }
}

fn state_dir() -> Option<PathBuf> {
    if let Ok(o) = std::env::var("OPENCLAW_STATE_DIR") {
        let t = o.trim();
        if !t.is_empty() {
            return Some(PathBuf::from(expand_home(t)));
        }
    }
    home().map(|h| h.join(".openclaw"))
}

fn approvals_path() -> PathBuf {
    state_dir()
        .unwrap_or_else(|| PathBuf::from(".openclaw"))
        .join("exec-approvals.json")
}

fn default_socket_path() -> String {
    state_dir()
        .unwrap_or_else(|| PathBuf::from(".openclaw"))
        .join("exec-approvals.sock")
        .to_string_lossy()
        .to_string()
}

/// exec-approvals.ts hasUnmigratedLegacyExecApprovals：仅当 OPENCLAW_STATE_DIR 覆盖、
/// 目标文件不存在而默认 ~/.openclaw/exec-approvals.json 存在时，视为「旧档未迁移」——
/// 读侧回退到严格策略且不落盘（防覆盖旧档），等 openclaw doctor 迁移。
fn has_unmigrated_legacy(target: &Path) -> bool {
    let overridden = std::env::var("OPENCLAW_STATE_DIR")
        .map(|v| !v.trim().is_empty())
        .unwrap_or(false);
    if !overridden {
        return false;
    }
    let legacy = home().map(|h| h.join(".openclaw").join("exec-approvals.json"));
    !target.exists() && legacy.map(|p| p.exists()).unwrap_or(false)
}

/// 未迁移旧档的读侧回退（createUnmigratedLegacyExecApprovalsFallback）：deny + always，从严。
fn legacy_fallback_file() -> ApprovalsFile {
    ApprovalsFile {
        version: 1,
        socket: None,
        defaults: Some(PolicyFields {
            security: Some("deny".into()),
            ask: Some("always".into()),
            ask_fallback: Some("deny".into()),
            auto_allow_skills: None,
        }),
        agents: Some(BTreeMap::new()),
    }
}

fn empty_file() -> ApprovalsFile {
    ApprovalsFile {
        version: 1,
        socket: None,
        defaults: None,
        agents: Some(BTreeMap::new()),
    }
}

// ===== normalize（exec-approvals.ts normalizeExecApprovals）=====

fn trim_opt(v: &Option<String>) -> Option<String> {
    v.as_deref().map(str::trim).filter(|s| !s.is_empty()).map(String::from)
}

fn valid_security(v: &Option<String>) -> Option<String> {
    v.as_deref()
        .filter(|s| ["deny", "allowlist", "full"].contains(s))
        .map(String::from)
}

fn valid_ask(v: &Option<String>) -> Option<String> {
    v.as_deref()
        .filter(|s| ["off", "on-miss", "always"].contains(s))
        .map(String::from)
}

fn validate_policy(p: &mut PolicyFields) {
    p.security = valid_security(&p.security);
    p.ask = valid_ask(&p.ask);
    p.ask_fallback = valid_security(&p.ask_fallback);
}

/// entry 归一：pattern 去空格、无 pattern 丢弃（返回 None）、补 id、剥 commandText。
fn normalize_entry(mut e: AllowlistEntry) -> Option<AllowlistEntry> {
    e.pattern = trim_opt(&e.pattern);
    if e.pattern.is_none() {
        return None; // 无 pattern 的 entry 丢弃（coerce 同款）
    }
    if e.id.as_deref().map(str::trim).unwrap_or("").is_empty() {
        e.id = Some(gen_id());
    } else {
        e.id = trim_opt(&e.id);
    }
    // openclaw normalize 剥 commandText（信息留在 lastUsedCommand）；入参带也兼容。
    e.command_text = None;
    e.arg_pattern = trim_opt(&e.arg_pattern);
    e.source = trim_opt(&e.source);
    e.last_used_command = trim_opt(&e.last_used_command);
    e.last_resolved_path = trim_opt(&e.last_resolved_path);
    Some(e)
}

fn normalize(mut file: ApprovalsFile) -> ApprovalsFile {
    file.version = 1;
    // socket：trim，双空 → 整个 socket 删（normalizeExecApprovals 同款）
    file.socket = file.socket.take().and_then(|s| {
        let path = trim_opt(&s.path);
        let token = trim_opt(&s.token);
        if path.is_none() && token.is_none() {
            None
        } else {
            Some(SocketInfo { path, token })
        }
    });
    if let Some(d) = file.defaults.as_mut() {
        validate_policy(d);
    }
    if let Some(mut agents) = file.agents.take() {
        // 旧版 agents.default 并入 agents.main（legacy merge）
        if let Some(legacy) = agents.remove(LEGACY_AGENT_KEY) {
            let entry = agents.entry(DEFAULT_AGENT_ID.into()).or_default();
            if entry.policy.security.is_none() {
                entry.policy.security = legacy.policy.security;
            }
            if entry.policy.ask.is_none() {
                entry.policy.ask = legacy.policy.ask;
            }
            if entry.policy.ask_fallback.is_none() {
                entry.policy.ask_fallback = legacy.policy.ask_fallback;
            }
            if entry.policy.auto_allow_skills.is_none() {
                entry.policy.auto_allow_skills = legacy.policy.auto_allow_skills;
            }
            if let Some(ll) = legacy.allowlist {
                entry.allowlist.get_or_insert_with(Vec::new).extend(ll);
            }
        }
        for e in agents.values_mut() {
            validate_policy(&mut e.policy);
            if let Some(list) = e.allowlist.take() {
                e.allowlist = Some(list.into_iter().filter_map(normalize_entry).collect());
            }
        }
        file.agents = Some(agents);
    }
    file
}

// ===== 读写（exec-approvals.ts readExecApprovalsSnapshot / saveExecApprovals / ensureExecApprovals）=====

fn sha256_hex(data: &[u8]) -> String {
    let d = Sha256::digest(data);
    d.iter().map(|x| format!("{x:02x}")).collect()
}

/// 读快照。缺失/不可解析 → exists:false + 空文件 + sha256("") 哈希（get 不建文件；ensure 才建）。
fn read_snapshot() -> Snapshot {
    let path = approvals_path();
    let raw = fs::read_to_string(&path).ok();
    match raw {
        None => {
            if has_unmigrated_legacy(&path) {
                return Snapshot {
                    path: path.to_string_lossy().to_string(),
                    exists: false,
                    hash: sha256_hex(b""),
                    file: legacy_fallback_file(),
                };
            }
            Snapshot {
                path: path.to_string_lossy().to_string(),
                exists: false,
                hash: sha256_hex(b""),
                file: empty_file(),
            }
        }
        Some(raw) => {
            // version!=1 或 JSON 坏 → 当空档（openclaw loadExecApprovals 同款容错）
            let file = serde_json::from_str::<Value>(&raw)
                .ok()
                .filter(|v| v.get("version").and_then(Value::as_i64) == Some(1))
                .and_then(|v| serde_json::from_value::<ApprovalsFile>(v).ok())
                .map(normalize)
                .unwrap_or_else(empty_file);
            Snapshot {
                path: path.to_string_lossy().to_string(),
                exists: true,
                hash: sha256_hex(raw.as_bytes()),
                file,
            }
        }
    }
}

/// 原子写：临时文件（create_new）+ rename（openclaw writeExecApprovalsRaw 同款；
/// Windows 无 unix 权限位，不做 0600 chmod——用户目录 ACL 默认保护，语义等价）。
fn save(file: &ApprovalsFile) -> Result<(), String> {
    let path = approvals_path();
    let dir = path.parent().ok_or("无父目录")?;
    fs::create_dir_all(dir).map_err(|e| format!("建目录失败: {e}"))?;
    let mut raw = serde_json::to_string_pretty(file).map_err(|e| e.to_string())?;
    raw.push('\n');
    let tmp = dir.join(format!(".exec-approvals.{}.{}.tmp", std::process::id(), gen_hex(6)));
    {
        use std::io::Write;
        let mut f = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&tmp)
            .map_err(|e| format!("写临时文件失败: {e}"))?;
        f.write_all(raw.as_bytes()).map_err(|e| format!("写临时文件失败: {e}"))?;
    }
    // Windows rename 目标存在时报错（openclaw CLI 可能同时持旧句柄）→ 先删旧再 rename
    if path.exists() {
        let _ = fs::remove_file(&path);
    }
    fs::rename(&tmp, &path).map_err(|e| {
        let _ = fs::remove_file(&tmp);
        format!("rename 失败: {e}")
    })?;
    Ok(())
}

/// ensureExecApprovals：文件不存在（或无 socket）→ 补 socket（path 默认 / token 生成）落盘。
/// 旧档未迁移时跳过（openclaw ensure 直接 return fallback 不落盘）。
fn ensure() {
    let path = approvals_path();
    if has_unmigrated_legacy(&path) {
        return;
    }
    let _lock = file_lock();
    let snap = read_snapshot();
    let mut next = snap.file;
    let socket = next.socket.take().unwrap_or_default();
    next.socket = Some(SocketInfo {
        path: Some(trim_opt(&socket.path).unwrap_or_else(default_socket_path)),
        token: Some(trim_opt(&socket.token).unwrap_or_else(gen_token)),
    });
    if let Err(e) = save(&next) {
        aiarsenal_client_core::util::log("warn", &format!("[desktop_node] exec-approvals ensure 写失败: {e}"));
    }
}

/// openclaw node get 返回的脱敏形状（invoke.ts redactExecApprovals）：socket 只留 path，空则整个删。
fn redact(mut file: ApprovalsFile) -> ApprovalsFile {
    file.socket = match file.socket.as_ref().and_then(|s| s.path.as_deref()).map(str::trim) {
        Some(p) if !p.is_empty() => Some(SocketInfo {
            path: Some(p.to_string()),
            token: None,
        }),
        _ => None,
    };
    file
}

fn snapshot_json(snap: Snapshot) -> Value {
    json!({
        "path": snap.path,
        "exists": snap.exists,
        "hash": snap.hash,
        "file": snap.file,
    })
}

// ===== node 命令：system.execApprovals.get / set（invoke.ts handler 对齐）=====

pub fn cmd_get() -> Value {
    ensure();
    let snap = read_snapshot();
    snapshot_json(Snapshot { file: redact(snap.file), ..snap })
}

/// set：baseHash CAS + normalize + socket 默认补齐（mergeExecApprovalsSocketDefaults）+ 原子写。
/// Err = (code, message)，对齐 gateway INVALID_REQUEST 语义（CLI 靠 message 文案判断重载重试）。
pub fn cmd_set(params: &Value) -> Result<Value, (&'static str, String)> {
    let file_v = params
        .get("file")
        .filter(|v| v.is_object())
        .ok_or(("INVALID_REQUEST", "exec approvals file required".to_string()))?;
    let incoming: ApprovalsFile = serde_json::from_value(file_v.clone())
        .map_err(|e| ("INVALID_REQUEST", format!("exec approvals file invalid: {e}")))?;
    ensure();
    let _lock = file_lock();
    let snap = read_snapshot();
    // CAS（requireExecApprovalsBaseHash）：exists 时 baseHash 必填且须等于当前哈希。
    if snap.exists {
        let base = params
            .get("baseHash")
            .and_then(Value::as_str)
            .map(str::trim)
            .unwrap_or("");
        if base.is_empty() {
            return Err((
                "INVALID_REQUEST",
                "exec approvals base hash required; reload and retry".to_string(),
            ));
        }
        if base != snap.hash {
            return Err((
                "INVALID_REQUEST",
                "exec approvals changed; reload and retry".to_string(),
            ));
        }
    }
    let normalized = normalize(incoming);
    // socket 补齐：normalized → 当前文件 → 默认（mergeExecApprovalsSocketDefaults 同款，token 兜底空串）
    let norm_socket = normalized.socket.clone().unwrap_or_default();
    let cur = snap.file.socket.unwrap_or_default();
    let mut next = normalized;
    next.socket = Some(SocketInfo {
        path: Some(
            trim_opt(&norm_socket.path)
                .or_else(|| trim_opt(&cur.path))
                .unwrap_or_else(default_socket_path),
        ),
        token: Some(
            trim_opt(&norm_socket.token)
                .or_else(|| trim_opt(&cur.token))
                .unwrap_or_default(),
        ),
    });
    save(&next).map_err(|e| ("INVALID_REQUEST", e))?;
    let next_snap = read_snapshot();
    Ok(snapshot_json(Snapshot {
        file: redact(next_snap.file),
        ..next_snap
    }))
}

// ===== 策略解析（effective policy + resolveExecModeFromPolicy）=====

pub const MODE_FULL: &str = "full";
pub const MODE_ALLOWLIST: &str = "allowlist";
pub const MODE_ASK: &str = "ask";
pub const MODE_DENY: &str = "deny";

/// (security, ask) → 模式（exec-approvals.ts resolveExecModeFromPolicy 逐分支对齐）。
pub fn mode_of(security: &str, ask: &str) -> &'static str {
    if security == "deny" {
        MODE_DENY
    } else if security == "allowlist" && ask == "off" {
        MODE_ALLOWLIST
    } else if security == "full" && ask != "always" {
        MODE_FULL
    } else {
        MODE_ASK
    }
}

/// 读侧策略：agents.main（逐字段 agents.* 兜底）→ defaults → (full, off)。不落盘。
/// v1 忽略 agentId 入参（gateway 单主 agent 场景，策略键固定 main）。
pub fn effective_policy() -> (String, String, Vec<AllowlistEntry>) {
    let snap = read_snapshot();
    let defaults = snap.file.defaults.clone().unwrap_or_default();
    let agents = snap.file.agents.clone().unwrap_or_default();
    let main = agents.get(DEFAULT_AGENT_ID);
    let wildcard = agents.get(WILDCARD_AGENT_KEY);
    // 逐字段：main → * → defaults（exec-approvals-effective.ts resolveAgentSecurityField/AskField）
    let pick = |get: fn(&PolicyFields) -> &Option<String>| -> Option<String> {
        main.and_then(|a| get(&a.policy).clone())
            .or_else(|| wildcard.and_then(|a| get(&a.policy).clone()))
    };
    let security = pick(|p| &p.security)
        .or(defaults.security)
        .unwrap_or_else(|| "full".into());
    let ask = pick(|p| &p.ask)
        .or(defaults.ask)
        .unwrap_or_else(|| "off".into());
    let allowlist = main
        .or(wildcard)
        .and_then(|a| a.allowlist.clone())
        .unwrap_or_default();
    (security, ask, allowlist)
}

// ===== 可执行解析（exec-command-resolution.ts buildExecutableResolution / resolveExecutableTrustPath）=====

#[derive(Clone, Debug)]
pub struct Resolution {
    pub raw_executable: String,
    pub resolved_path: Option<String>,
    pub resolved_real_path: Option<String>,
    pub executable_name: String,
}

fn has_path_selector(v: &str) -> bool {
    v.contains('/') || v.contains('\\') || v.contains('~')
}

fn realpath(p: &Path) -> Option<String> {
    fs::canonicalize(p).ok().map(|x| x.to_string_lossy().to_string())
}

/// Windows 可执行判定：文件存在即算（PATHEXT 候选在调用侧展开；无 unix 0o111 权限位）。
fn is_executable_file(p: &Path) -> bool {
    p.is_file()
}

/// 裸名在单个 PATH 段下的 PATHEXT 候选（`ping` → ping.exe/ping.cmd/ping.bat/ping 本名）。
/// openclaw Windows node-host 的 resolveExecutablePath 靠 Node 的 PATH+PATHEXT 语义，此处对齐。
fn executable_candidates(dir: &Path, raw: &str) -> Vec<PathBuf> {
    let base = dir.join(raw);
    let mut out = vec![base.clone()];
    if let Some(exts) = std::env::var("PATHEXT").ok() {
        for ext in exts.split(';') {
            let ext = ext.trim();
            if ext.is_empty() || !ext.starts_with('.') {
                continue;
            }
            out.push(dir.join(format!("{raw}{ext}")));
        }
    } else {
        for ext in [".exe", ".cmd", ".bat"] {
            out.push(dir.join(format!("{raw}{ext}")));
        }
    }
    out
}

/// 解析 argv[0]：路径选择器（含 / \ ~）→ 展开为绝对路径；裸名 → PATH 段搜索可执行文件。
/// `path_env`：目标进程的 PATH（system.run 传 env 时用 env 的 PATH，对齐 openclaw env?.PATH 优先）。
pub fn resolve_executable(raw: &str, cwd: Option<&str>, path_env: Option<&str>) -> Resolution {
    let raw = raw.trim();
    if has_path_selector(raw) {
        let expanded = expand_home(raw);
        let candidate = if Path::new(&expanded).is_absolute() {
            PathBuf::from(expanded)
        } else {
            match cwd {
                Some(c) => Path::new(c).join(&expanded),
                None => PathBuf::from(expanded),
            }
        };
        let resolved = candidate.to_string_lossy().to_string();
        let real = realpath(&candidate);
        let name = candidate
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_else(|| raw.to_string());
        return Resolution {
            raw_executable: raw.to_string(),
            resolved_path: Some(resolved),
            resolved_real_path: real,
            executable_name: name,
        };
    }
    // 裸名 → PATH 搜索（executable-path.ts resolveExecutablePath：env PATH 优先；
    // Windows PATH 分隔 ';'，PATHEXT 候选展开）
    let path_value = path_env
        .map(str::to_string)
        .or_else(|| std::env::var("PATH").ok())
        .unwrap_or_default();
    for dir in path_value.split(';') {
        if dir.is_empty() {
            continue;
        }
        for candidate in executable_candidates(Path::new(dir), raw) {
            if is_executable_file(&candidate) {
                let resolved = candidate.to_string_lossy().to_string();
                let real = realpath(&candidate);
                return Resolution {
                    raw_executable: raw.to_string(),
                    resolved_path: Some(resolved.clone()),
                    resolved_real_path: real,
                    executable_name: raw.to_string(),
                };
            }
        }
    }
    Resolution {
        raw_executable: raw.to_string(),
        resolved_path: None,
        resolved_real_path: None,
        executable_name: raw.to_string(),
    }
}

// ===== allowlist 匹配（exec-allowlist-pattern.ts + exec-command-resolution.ts matchAllowlist）=====

/// exec-allowlist-pattern.ts normalizeMatchTarget（Windows 分支）：反斜杠 → 正斜杠，
/// 使 glob pattern 与 trustPath 同口径（Windows 无 darwin 的 /private/var 前缀）。
fn normalize_match_target(value: &str) -> String {
    value.replace('\\', "/")
}

fn has_dot_segment(v: &str) -> bool {
    v.replace('\\', "/").split('/').any(|s| s == "." || s == "..")
}

/// posix normalize（仅 wildcard 目标带 ./.. 段时归一，防 `*/..` 结构被改写——只归一目标不归一
/// pattern）。Windows「绝对」判定：`/` 或盘符 `X:/` 开头（盘符路径否则会被当相对路径错并 `..`）。
fn posix_normalize(p: &str) -> String {
    let absolute = p.starts_with('/')
        || (p.len() >= 2 && p.as_bytes()[1] == b':' && p.as_bytes()[0].is_ascii_alphabetic());
    let mut out: Vec<&str> = Vec::new();
    for seg in p.split('/') {
        match seg {
            "" | "." => {}
            ".." => match out.last() {
                Some(&last) if last != ".." => {
                    out.pop();
                }
                _ if !absolute => out.push(".."),
                _ => {}
            },
            s => out.push(s),
        }
    }
    let joined = out.join("/");
    if absolute {
        format!("/{joined}")
    } else {
        joined
    }
}

/// glob 编译（compileGlobRegex）：`**`→`.*`、`*`→`[^/]*`、`?`→`[^/]`、其余转义，锚定 ^...$。
/// Windows：大小写不敏感匹配（文件系统大小写不敏感；pattern 写 "Ping.exe" 也能命中 ping.exe）。
fn compile_glob_regex(pattern: &str) -> Option<regex::Regex> {
    let mut out = String::from("^");
    let chars: Vec<char> = pattern.chars().collect();
    let mut i = 0;
    while i < chars.len() {
        let c = chars[i];
        if c == '*' {
            if chars.get(i + 1) == Some(&'*') {
                out.push_str(".*");
                i += 2;
                continue;
            }
            out.push_str("[^/]*");
            i += 1;
            continue;
        }
        if c == '?' {
            out.push_str("[^/]");
            i += 1;
            continue;
        }
        out.push_str(&regex::escape(&c.to_string()));
        i += 1;
    }
    out.push('$');
    regex::RegexBuilder::new(&out)
        .case_insensitive(true)
        .build()
        .ok()
}

/// matchesExecAllowlistPattern：pattern ~ 展开 + 双侧 normalizeMatchTarget + wildcard 目标 ./.. 归一。
fn matches_pattern(pattern: &str, target: &str) -> bool {
    let trimmed = pattern.trim();
    if trimmed.is_empty() {
        return false;
    }
    let expanded = expand_home(trimmed);
    let has_wildcard = expanded.contains('*') || expanded.contains('?');
    let normalized_pattern = normalize_match_target(&expanded);
    let mut normalized_target = normalize_match_target(target);
    if has_wildcard && has_dot_segment(&normalized_target) {
        normalized_target = normalize_match_target(&posix_normalize(&normalized_target));
    }
    compile_glob_regex(&normalized_pattern)
        .map(|re| re.is_match(&normalized_target))
        .unwrap_or(false)
}

/// matchArgPattern（exec-command-resolution.ts）：`\x00` 连接自动生成 pattern（带尾哨兵、
/// 零参 `\0\0`）；空格连接手写 pattern（无锚定假设 + 尾部 shell 重定向剥离重试）。
fn match_arg_pattern(arg_pattern: &str, argv: &[String]) -> bool {
    let sep_is_nul = arg_pattern.contains('\0');
    let args = if argv.is_empty() { &argv[0..0] } else { &argv[1..] };
    let args_string = if sep_is_nul {
        if args.is_empty() {
            "\0\0".to_string()
        } else {
            let mut s = args.join("\0");
            s.push('\0');
            s
        }
    } else {
        args.join(" ")
    };
    let Ok(re) = regex::Regex::new(arg_pattern) else {
        return false;
    };
    if re.is_match(&args_string) {
        return true;
    }
    if !sep_is_nul {
        // 手写 pattern：剥尾部 `2>&1` / `2>/dev/null` 重试（LLM 常在 cron 重跑时增删）
        if let Ok(strip_re) = regex::Regex::new(r"\s+(?:[12]>&[12]|[12]>/dev/null)\s*$") {
            let mut prev = args_string.clone();
            loop {
                let next = strip_re.replace(&prev, "").to_string();
                if next == prev {
                    break;
                }
                if next != args_string && re.is_match(&next) {
                    return true;
                }
                prev = next;
            }
        }
    }
    false
}

fn file_base_name(p: &str) -> String {
    Path::new(p)
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
        .unwrap_or_else(|| p.to_string())
}

/// matchAllowlist 全量对齐：
/// - 裸 `"*"`（无 argPattern）命中任何已解析命令（含 PATH 解析失败的）；
/// - resolvedPath 缺失 → 不命中；trustPath = realPath || resolvedPath；
/// - 含路径选择器的 pattern 对 trustPath 做 glob；裸名 pattern 只对 basename 候选匹配
///   （且 rawExecutable 本身不得带路径选择器，防工作区二进制继承全局信任）；
/// - 无 argPattern 的首个命中作 pathOnlyMatch 兜底；带 argPattern 的须 argv 再匹配。
pub fn match_allowlist(
    entries: &[AllowlistEntry],
    resolution: &Resolution,
    argv: Option<&[String]>,
) -> Option<AllowlistEntry> {
    if entries.is_empty() {
        return None;
    }
    let bare_wild = entries
        .iter()
        .find(|e| e.pattern.as_deref().map(str::trim) == Some("*") && e.arg_pattern.is_none());
    if let Some(w) = bare_wild {
        return Some(w.clone());
    }
    let resolved_path = resolution.resolved_path.as_deref()?;
    let trust_path = resolution
        .resolved_real_path
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .unwrap_or(resolved_path);
    let mut path_only_match: Option<&AllowlistEntry> = None;
    for entry in entries {
        let pattern = entry.pattern.as_deref().map(str::trim).unwrap_or("");
        if pattern.is_empty() {
            continue;
        }
        let pattern_matches = if has_path_selector(pattern) {
            matches_pattern(pattern, trust_path)
        } else if pattern == "*" {
            false // 裸 * 只走上面的 bare_wild 快路
        } else {
            matches_executable_basename(pattern, resolution)
        };
        if !pattern_matches {
            continue;
        }
        match entry.arg_pattern.as_deref() {
            None => {
                if path_only_match.is_none() {
                    path_only_match = Some(entry);
                }
            }
            Some(ap) => {
                if let Some(argv) = argv {
                    if match_arg_pattern(ap, argv) {
                        return Some(entry.clone());
                    }
                }
            }
        }
    }
    path_only_match.cloned()
}

fn matches_executable_basename(pattern: &str, resolution: &Resolution) -> bool {
    if has_path_selector(&resolution.raw_executable) {
        return false;
    }
    let mut candidates: Vec<String> = vec![resolution.executable_name.clone()];
    if let Some(rp) = &resolution.resolved_path {
        let b = file_base_name(rp);
        if !candidates.contains(&b) {
            candidates.push(b);
        }
    }
    candidates
        .iter()
        .any(|c| matches_pattern(pattern, c))
}

// ===== 「总是允许」落盘（addAllowAlwaysPattern 语义；Windows 带 argPattern）=====

/// buildArgPatternFromArgv 同款自动 pattern（Windows 生成）：`\0` 连接 + 尾哨兵，零参 `\0\0`。
/// （match_arg_pattern 的 sep_is_nul 分支按此格式匹配——两侧同源，自洽。）
fn auto_arg_pattern(argv: &[String]) -> Option<String> {
    if argv.is_empty() {
        return None;
    }
    let args = &argv[1..];
    Some(if args.is_empty() {
        "\0\0".to_string()
    } else {
        let mut s = args.join("\0");
        s.push('\0');
        s
    })
}

/// 追加 allow-always entry：pattern = trustPath 绝对路径；argPattern = argv 自动 pattern
///（仅 Windows 生成，见模块头）；同 (pattern, 同 argPattern 口径) 去重更新。
pub fn append_allow_always(
    pattern: &str,
    command_text: Option<&str>,
    resolved_path: Option<&str>,
    argv: &[String],
) -> Result<(), String> {
    let arg_pattern = auto_arg_pattern(argv);
    let _lock = file_lock();
    let path = approvals_path();
    if has_unmigrated_legacy(&path) {
        return Err("旧档未迁移（OPENCLAW_STATE_DIR 覆盖 + 默认位有 exec-approvals.json），请先跑 openclaw doctor".into());
    }
    let mut file = if path.exists() {
        let snap = read_snapshot();
        snap.file
    } else {
        empty_file()
    };
    let agents = file.agents.get_or_insert_with(BTreeMap::new);
    let agent = agents.entry(DEFAULT_AGENT_ID.into()).or_default();
    let list = agent.allowlist.get_or_insert_with(Vec::new);
    let now = now_ms();
    if let Some(existing) = list
        .iter_mut()
        .find(|e| e.pattern.as_deref() == Some(pattern) && e.arg_pattern == arg_pattern)
    {
        existing.last_used_at = Some(now);
        existing.last_used_command = command_text.map(str::to_string);
        existing.last_resolved_path = resolved_path.map(str::to_string);
    } else {
        list.push(AllowlistEntry {
            id: Some(gen_id()),
            pattern: Some(pattern.to_string()),
            source: Some("allow-always".into()),
            command_text: None,
            arg_pattern,
            last_used_at: Some(now),
            last_used_command: command_text.map(str::to_string),
            last_resolved_path: resolved_path.map(str::to_string),
        });
    }
    save(&file)
}

// ===== 杂项 =====

fn now_ms() -> f64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as f64)
        .unwrap_or(0.0)
}

fn gen_hex(n: usize) -> String {
    use rand::RngCore;
    let mut buf = vec![0u8; n];
    rand::thread_rng().fill_bytes(&mut buf);
    buf.iter().map(|x| format!("{x:02x}")).collect()
}

fn gen_id() -> String {
    gen_hex(16)
}

/// generateToken 同款：24 随机字节 base64url（32 字符）。
fn gen_token() -> String {
    use base64::Engine;
    use rand::RngCore;
    let mut buf = [0u8; 24];
    rand::thread_rng().fill_bytes(&mut buf);
    base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(buf)
}

// ===== 审批闸门（system.run 前置：模式分流 + ask 模式弹窗审批）=====

/// 审批等待上限。超时 = 拒绝（微信/网页入口的 agent 提示注入面，宁可误拒）。
const APPROVAL_TIMEOUT: Duration = Duration::from_secs(60);
/// 事件重发间隔：懒建窗口时前端 listen 可能未挂上，2s 一发直到有回应（前端幂等去重）。
const APPROVAL_REEMIT: Duration = Duration::from_secs(2);

struct PendingApproval {
    id: String,
    decision: Option<&'static str>, // once | always | deny
}

static APPROVAL: Mutex<Option<PendingApproval>> = Mutex::new(None);
static APPROVAL_COND: Condvar = Condvar::new();

fn lock_approval() -> std::sync::MutexGuard<'static, Option<PendingApproval>> {
    APPROVAL.lock().unwrap_or_else(|p| p.into_inner())
}

pub enum GateVerdict {
    Allow,
    Reject(String),
}

/// system.run 审批闸门。full 模式零开销放行（默认档 = 现状）；deny 全拒；
/// allowlist/ask 模式先匹配 allowlist，ask 未命中弹窗（60s 超时拒绝）。
///
/// 阻塞在 WS 读线程（dispatch）：60s 内其它 invoke 排队。单 agent 工具调用基本串行，
/// v1 接受（gateway 对 system.run 的 invoke 预算 ~30min）。
pub fn gate(
    app: &tauri::AppHandle,
    argv: &[String],
    cwd: Option<&str>,
    path_env: Option<&str>,
    log: &impl Fn(&str),
) -> GateVerdict {
    let (security, ask, allowlist) = effective_policy();
    let mode = mode_of(&security, &ask);
    match mode {
        MODE_FULL => return GateVerdict::Allow,
        MODE_DENY => return GateVerdict::Reject("exec approvals 策略为 deny（全部拒绝）".into()),
        _ => {}
    }
    let resolution = resolve_executable(&argv[0], cwd, path_env);
    if match_allowlist(&allowlist, &resolution, Some(argv)).is_some() {
        return GateVerdict::Allow;
    }
    if mode == MODE_ALLOWLIST {
        return GateVerdict::Reject("exec approvals 策略为 allowlist，命令不在白名单".into());
    }
    // ask 模式未命中 → 弹窗审批
    ask_user(app, argv, cwd, &resolution, &security, &ask, log)
}

fn ask_user(
    app: &tauri::AppHandle,
    argv: &[String],
    cwd: Option<&str>,
    resolution: &Resolution,
    security: &str,
    ask: &str,
    log: &impl Fn(&str),
) -> GateVerdict {
    use tauri::Emitter;
    let id = gen_id();
    let command_text = argv.join(" ");
    let expires_at = now_ms() as u64 + APPROVAL_TIMEOUT.as_millis() as u64;
    let payload = json!({
        "id": id,
        "argv": argv,
        "commandText": command_text,
        "cwd": cwd,
        "resolvedPath": resolution.resolved_path,
        "security": security,
        "ask": ask,
        "expiresAt": expires_at,
    });
    {
        let mut g = lock_approval();
        *g = Some(PendingApproval { id: id.clone(), decision: None });
    }
    // 拉起主窗口 + 系统通知（托盘态也能注意到）
    super::super::ensure_main_window(app);
    let _ = app.emit("exec-approval-request", &payload);
    let _ = super::system::notify(&json!({
        "title": "执行审批请求",
        "body": &command_text,
    }));
    log(&format!("等待审批（60s 超时拒绝）: {command_text}"));

    let deadline = Instant::now() + APPROVAL_TIMEOUT;
    let decision = loop {
        let mut g = lock_approval();
        if let Some(d) = g.as_ref().and_then(|p| p.decision) {
            *g = None;
            break d;
        }
        let now = Instant::now();
        if now >= deadline {
            *g = None;
            break "deny"; // 超时拒绝
        }
        // 重发（前端对同 id 幂等）：补窗口加载中丢掉的首发 + 刷新倒计时
        let _ = app.emit("exec-approval-request", &payload);
        let slice = APPROVAL_REEMIT.min(deadline - now);
        let (ng, _) = APPROVAL_COND.wait_timeout(g, slice).unwrap_or_else(|p| p.into_inner());
        drop(ng);
    };
    match decision {
        "once" => GateVerdict::Allow,
        "always" => {
            // pattern = trustPath（realPath 优先，与 matchAllowlist 同口径）；解析失败退回 argv[0]。
            // argPattern 按 Windows 语义随 argv 生成（buildArgPatternFromArgv，见模块头）。
            let trust_path = resolution
                .resolved_real_path
                .clone()
                .or_else(|| resolution.resolved_path.clone())
                .unwrap_or_else(|| argv[0].clone());
            if let Err(e) = append_allow_always(
                &trust_path,
                Some(&command_text),
                resolution.resolved_path.as_deref(),
                argv,
            ) {
                log(&format!("⚠️ allow-always 落盘失败（本次仍放行）: {e}"));
            }
            GateVerdict::Allow
        }
        _ => GateVerdict::Reject("用户拒绝了本次命令执行".into()),
    }
}

/// 前端审批回应（client_respond_exec_approval 命令）。decision: once|always|deny。
/// false = 当前无待审批请求或 id 不匹配（过期/已答）。
pub fn respond(id: &str, decision: &str) -> bool {
    let d: &'static str = match decision {
        "once" => "once",
        "always" => "always",
        "deny" => "deny",
        _ => return false,
    };
    let mut g = lock_approval();
    let matched = g
        .as_ref()
        .map(|p| p.id == id && p.decision.is_none())
        .unwrap_or(false);
    if matched {
        if let Some(p) = g.as_mut() {
            p.decision = Some(d);
        }
        APPROVAL_COND.notify_all();
    }
    matched
}

// ===== UI 档位读写（client_get/set_exec_policy 命令）=====

/// 当前档位摘要：{mode, security, ask, path, exists, allowlistCount, allowlist:[{pattern,source}]}。
pub fn policy_summary() -> Value {
    let (security, ask, allowlist) = effective_policy();
    let snap_exists = approvals_path().exists();
    json!({
        "mode": mode_of(&security, &ask),
        "security": security,
        "ask": ask,
        "path": approvals_path().to_string_lossy(),
        "exists": snap_exists,
        "allowlistCount": allowlist.len(),
        "allowlist": allowlist
            .iter()
            .map(|e| json!({"pattern": e.pattern, "source": e.source, "lastUsedCommand": e.last_used_command}))
            .collect::<Vec<_>>(),
    })
}

/// 首次切档时的种子白名单：只读常用命令（裸名 pattern，匹配 PATH+PATHEXT 解析的同名可执行；
/// Windows 诊断类命令集——cmd/PowerShell 内建（dir/echo/type）不是独立可执行文件，不列）。
const SEED_ALLOWLIST: &[&str] = &[
    "ipconfig", "ping", "systeminfo", "whoami", "tasklist", "where", "hostname", "nslookup",
    "netstat", "tracert", "route",
];

/// 切档（full→(full,off)、ask→(allowlist,on-miss)、allowlist→(allowlist,off)、deny→(deny,off)）。
/// 写 defaults；agents.main 已存在时同步覆写其 security/ask（agent 字段优先于 defaults，
/// 不覆写会出现 UI 显示新档但实际仍走旧档）。无文件先落种子白名单（ask/allowlist 档可用）。
pub fn set_policy_mode(mode: &str) -> Result<Value, String> {
    let (security, ask) = match mode {
        "full" => ("full", "off"),
        "ask" => ("allowlist", "on-miss"),
        "allowlist" => ("allowlist", "off"),
        "deny" => ("deny", "off"),
        other => return Err(format!("未知档位: {other}（full|ask|allowlist|deny）")),
    };
    let path = approvals_path();
    if has_unmigrated_legacy(&path) {
        return Err("旧档未迁移（OPENCLAW_STATE_DIR 覆盖 + 默认位有 exec-approvals.json），请先跑 openclaw doctor".into());
    }
    let _lock = file_lock();
    let mut file = if path.exists() {
        read_snapshot().file
    } else {
        let mut f = empty_file();
        // 种子白名单只对 ask/allowlist 档有意义（full 全放行用不上、deny 全拒放不放无所谓）
        if security == "allowlist" {
            let agents = f.agents.get_or_insert_with(BTreeMap::new);
            let agent = agents.entry(DEFAULT_AGENT_ID.into()).or_default();
            let now = now_ms();
            agent.allowlist = Some(
                SEED_ALLOWLIST
                    .iter()
                    .map(|cmd| AllowlistEntry {
                        id: Some(gen_id()),
                        pattern: Some((*cmd).to_string()),
                        source: Some("seed".into()),
                        command_text: None,
                        arg_pattern: None,
                        last_used_at: Some(now),
                        last_used_command: None,
                        last_resolved_path: None,
                    })
                    .collect(),
            );
        }
        f
    };
    let defaults = file.defaults.get_or_insert_with(PolicyFields::default);
    defaults.security = Some(security.into());
    defaults.ask = Some(ask.into());
    // agents.main 已有显式覆盖时同步改（agent 字段优先级高于 defaults，不改则档位不生效）
    if let Some(agents) = file.agents.as_mut() {
        if let Some(agent) = agents.get_mut(DEFAULT_AGENT_ID) {
            agent.policy.security = Some(security.into());
            agent.policy.ask = Some(ask.into());
        }
    }
    // socket 补齐（save 前的 normalize 不强制 socket；保持与 openclaw 落盘形状一致）
    if file.socket.is_none() {
        file.socket = Some(SocketInfo {
            path: Some(default_socket_path()),
            token: Some(String::new()),
        });
    }
    save(&file)?;
    Ok(policy_summary())
}
