//! 通用工具。

use std::collections::HashSet;
use std::fs::{File, OpenOptions};
use std::io::Write;
use std::path::Path;
use std::sync::Mutex;

use chrono::{Local, Timelike};

/// App 自身作为心跳 agent 的 key。启动报 running、退出报 stopped，记录 App 在线/离线。
pub const CLIENT_AGENT_KEY: &str = "client";

/// App 版本（编译期取 Cargo.toml version）。
pub const APP_VERSION: &str = env!("CARGO_PKG_VERSION");

/// 心跳上报间隔（秒）：当地时区 8:00–20:00（含 8 不含 20）每 2 秒一次，其它时段每 10 秒一次。
/// 在心跳 loop 每轮调用，跨时段自动切换。
pub fn heartbeat_interval_secs() -> u64 {
    let h = Local::now().hour();
    if (8..20).contains(&h) {
        2
    } else {
        10
    }
}

// ===== 文件日志（GUI 应用 stdout/stderr 不可见，落盘到 data_dir/aiarsenal-client.log） =====
static LOG_FILE: Mutex<Option<File>> = Mutex::new(None);

/// 初始化日志文件。在 setup 阶段调一次。失败则日志静默丢弃（不影响业务）。
pub fn init_log(data_dir: &Path) {
    let _ = std::fs::create_dir_all(data_dir);
    let path = data_dir.join("aiarsenal-client.log");
    // 控制单文件体积：超过 5MB 先滚动掉旧文件再新建
    if let Ok(meta) = std::fs::metadata(&path) {
        if meta.len() > 5 * 1024 * 1024 {
            let _ = std::fs::remove_file(&path);
        }
    }
    let f = OpenOptions::new()
        .create(true)
        .append(true)
        .open(&path)
        .ok();
    if let Some(file) = f {
        *LOG_FILE.lock().unwrap() = Some(file);
        log("info", &format!("=== 日志初始化，文件：{} ===", path.display()));
    }
}

/// 写一行日志：`[2026-07-08 00:12:34.567] INFO 消息`。
pub fn log(level: &str, msg: &str) {
    let line = format!(
        "[{}] {} {}\n",
        Local::now().format("%Y-%m-%d %H:%M:%S%.3f"),
        level.to_uppercase(),
        msg
    );
    if let Some(f) = LOG_FILE.lock().unwrap().as_mut() {
        let _ = f.write_all(line.as_bytes());
        let _ = f.flush();
    }
}

// ===== 已激活 agent 集合（持久化） =====
// 只有「装过/下过命令」的 agent 才上报状态。App 认领到某 agent 的命令即把它加入
// 集合并持久化到 data_dir/activated-agents.json；心跳只上报集合内的 agent。
// 这样本机自装的 Obsidian、从未触碰的 hermes 不会被上报污染状态列表。
static ACTIVATED_FILE: &str = "activated-agents.json";

/// 读取已激活 agent 集合。文件不存在或损坏返回空集。
pub fn load_activated_agents(data_dir: &Path) -> HashSet<String> {
    let path = data_dir.join(ACTIVATED_FILE);
    match std::fs::read_to_string(&path) {
        Ok(s) => {
            let v: serde_json::Value = serde_json::from_str(&s).unwrap_or(serde_json::json!([]));
            v.as_array()
                .map(|arr| {
                    arr.iter()
                        .filter_map(|x| x.as_str().map(|s| s.to_string()))
                        .collect()
                })
                .unwrap_or_default()
        }
        Err(_) => HashSet::new(),
    }
}

/// 把一个 agent 加入已激活集合并持久化。已存在则幂等。
pub fn mark_agent_activated(data_dir: &Path, agent: &str) {
    let _ = std::fs::create_dir_all(data_dir);
    let mut set = load_activated_agents(data_dir);
    if !set.insert(agent.to_string()) {
        return; // 已在集合，无需写盘
    }
    let arr: Vec<&String> = set.iter().collect();
    let json = serde_json::to_string(&arr).unwrap_or_else(|_| "[]".into());
    let path = data_dir.join(ACTIVATED_FILE);
    let _ = std::fs::write(&path, json);
    log("info", &format!("agent {} 纳入已激活集合并持久化", agent));
}

/// 用服务端返回的选中 agent 集合**覆盖**本地激活集合（替代旧的「补全」语义）。
/// 用户在设备控制弹窗多选框选哪些 agent，App 就上报哪些——完全跟随服务端选择。
pub fn set_activated_agents(data_dir: &Path, agents: &[String]) {
    let _ = std::fs::create_dir_all(data_dir);
    let set: HashSet<String> = agents.iter().cloned().collect();
    let arr: Vec<&String> = set.iter().collect();
    let json = serde_json::to_string(&arr).unwrap_or_else(|_| "[]".into());
    let path = data_dir.join(ACTIVATED_FILE);
    let _ = std::fs::write(&path, json);
    log("info", &format!("已激活集合按服务端选择覆盖：{:?}", arr));
}

// ===== 授权 App 清单（持久化；服务端「App 权限」按设备配置，心跳下发） =====
// 读写模式与 activated-agents 一致；App 启动时先读缓存清单授权（登录前就开始），
// 心跳拿到新清单再覆盖并唤醒授权引擎。
// ===== 客户端自有配置 aa.json（KV 结构，可扩展） =====
// 形如 {"app_perms":[…], "granted_perms":[…], "denied_perms":[…]}；后续新增配置
// 加兄弟 key 即可，serde 默认值容忍旧文件缺 key。app_perms 是服务端下发值，
// granted_perms/denied_perms 是本机授权引擎的状态（已授权/已拒绝集，客户端自产，
// 变更时随心跳上报）——同文件不同键，读改写互相保留。alias 兼容读取历史 camelCase
// 文件（appPerms/grantedPerms），首次写出即迁移为 snake_case。
// 历史版本曾把已授权集以裸数组存独立文件 perm-granted.json——读取时自动并入。
static AA_CONFIG_FILE: &str = "aa.json";
static LEGACY_GRANTED_FILE: &str = "perm-granted.json";

/// aa.json 的结构（client 自有配置；扩展时加字段）。
#[derive(Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct AaConfig {
    /// 服务端下发的自动化授权 App 清单（心跳覆盖式更新）。
    #[serde(
        default,
        rename = "app_perms",
        alias = "appPerms",
        skip_serializing_if = "Vec::is_empty"
    )]
    pub app_perms: Vec<crate::types::AppPerm>,
    /// 已确认授权的 bundle id 集（授权引擎写入；清单移除的会被修剪）。
    #[serde(
        default,
        rename = "granted_perms",
        alias = "grantedPerms",
        skip_serializing_if = "Vec::is_empty"
    )]
    pub granted_perms: Vec<String>,
    /// 已拒绝的 bundle id 集（TCC 缓存拒绝 -1743；用户在系统设置开回后转 granted）。
    #[serde(default, rename = "denied_perms", skip_serializing_if = "Vec::is_empty")]
    pub denied_perms: Vec<String>,
}

fn read_aa_config(data_dir: &Path) -> AaConfig {
    let mut cfg = if let Ok(s) = std::fs::read_to_string(data_dir.join(AA_CONFIG_FILE)) {
        serde_json::from_str::<AaConfig>(&s).unwrap_or_default()
    } else {
        AaConfig::default()
    };
    // 旧版独立文件 perm-granted.json（裸数组）一次性并入 aa.json 并删除
    if cfg.granted_perms.is_empty() {
        if let Ok(s) = std::fs::read_to_string(data_dir.join(LEGACY_GRANTED_FILE)) {
            if let Ok(list) = serde_json::from_str::<Vec<String>>(&s) {
                cfg.granted_perms = list;
                write_aa_config(data_dir, &cfg);
                let _ = std::fs::remove_file(data_dir.join(LEGACY_GRANTED_FILE));
            }
        }
    }
    cfg
}

fn write_aa_config(data_dir: &Path, cfg: &AaConfig) {
    let _ = std::fs::create_dir_all(data_dir);
    if let Ok(json) = serde_json::to_string_pretty(cfg) {
        let _ = std::fs::write(data_dir.join(AA_CONFIG_FILE), json);
    }
}

/// 读取缓存的授权 App 清单（aa.json 的 appPerms）。文件不存在或损坏返回空。
pub fn load_app_perms(data_dir: &Path) -> Vec<crate::types::AppPerm> {
    read_aa_config(data_dir).app_perms
}

/// 覆盖缓存清单（只动 aa.json 的 appPerms 键，其余键保留）。
pub fn set_app_perms(data_dir: &Path, apps: &[crate::types::AppPerm]) {
    let mut cfg = read_aa_config(data_dir);
    cfg.app_perms = apps.to_vec();
    write_aa_config(data_dir, &cfg);
}

/// 服务端清单与缓存清单是否不同（bundle_id 集合比对；名称变化不算）。
pub fn app_perms_changed(
    data_dir: &Path,
    apps: &[crate::types::AppPerm],
) -> bool {
    let old: HashSet<String> =
        load_app_perms(data_dir).into_iter().map(|a| a.bundle_id).collect();
    let new: HashSet<String> = apps.iter().map(|a| a.bundle_id.clone()).collect();
    old != new
}

/// 读取已授权 bundle id 集（aa.json 的 granted_perms）。文件不存在或损坏返回空集。
pub fn load_granted_perms(data_dir: &Path) -> HashSet<String> {
    read_aa_config(data_dir).granted_perms.into_iter().collect()
}

/// 读取已拒绝 bundle id 集（aa.json 的 denied_perms）。文件不存在或损坏返回空集。
pub fn load_denied_perms(data_dir: &Path) -> HashSet<String> {
    read_aa_config(data_dir).denied_perms.into_iter().collect()
}

/// 一次写掉授权状态两键（granted_perms + denied_perms，各自排序；其余键保留）。
/// 授权/拒绝互斥迁移由调用方保证；同写避免两次读改写互相覆盖丢状态。
pub fn set_perm_state(data_dir: &Path, granted: &HashSet<String>, denied: &HashSet<String>) {
    let mut cfg = read_aa_config(data_dir);
    let mut g: Vec<String> = granted.iter().cloned().collect();
    g.sort();
    let mut d: Vec<String> = denied.iter().cloned().collect();
    d.sort();
    cfg.granted_perms = g;
    cfg.denied_perms = d;
    write_aa_config(data_dir, &cfg);
}

// ===== 通用设置（持久化到 data_dir/client-settings.json） =====
// JSON object 形式存散落的小开关（如防锁屏）。读写模式与 activated-agents 一致。
static SETTINGS_FILE: &str = "client-settings.json";

/// 读取全部设置。文件不存在或损坏返回空 map。
pub fn load_settings(data_dir: &Path) -> serde_json::Map<String, serde_json::Value> {
    let path = data_dir.join(SETTINGS_FILE);
    match std::fs::read_to_string(&path) {
        Ok(s) => serde_json::from_str::<serde_json::Value>(&s)
            .ok()
            .and_then(|v| v.as_object().cloned())
            .unwrap_or_default(),
        Err(_) => serde_json::Map::new(),
    }
}

/// 写单个设置项（读改写整个文件，幂等）。
pub fn set_setting(data_dir: &Path, key: &str, value: serde_json::Value) {
    let _ = std::fs::create_dir_all(data_dir);
    let mut map = load_settings(data_dir);
    map.insert(key.to_string(), value);
    let json = serde_json::to_string(&map).unwrap_or_else(|_| "{}".into());
    let path = data_dir.join(SETTINGS_FILE);
    let _ = std::fs::write(&path, json);
}

/// 防锁屏开关是否启用：默认 true（启动即常开），仅当显式存了 false 才返回 false。
pub fn keep_awake_enabled(data_dir: &Path) -> bool {
    !matches!(
        load_settings(data_dir).get("keepAwake").and_then(|v| v.as_bool()),
        Some(false)
    )
}

