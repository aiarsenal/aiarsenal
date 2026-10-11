//! App 控制清单引擎（Windows 版）——服务端可配置版。
//!
//! mac 版是 AppleEvents 自动化授权引擎（TCC 逐对授权，60s 轮探 + 授权框）；Windows 没有
//! TCC、没有系统授权框——**清单即授权**：管理端勾选的 App 就在「App 控制清单」内，
//! app_gate（system.run 载荷分析 + computer.act 前台校验）按清单放行/拦截，不需要也不存在
//! 「提前弹框授权」。引擎在这里的角色收敛为：
//! - 接收心跳下发的清单（start_engine 播种 + update_apps 增量），维护 allowed_snapshot
//!   给 app_gate 闸门查（引擎未启动 = 空清单 = 全部 App 禁控，fail-closed）；
//! - 把「清单内 = granted」的授权状态持久化（aa.json granted_perms/denied_perms，
//!   util::set_perm_state）并随心跳主动上报（pending_upload/mark_uploaded）——管理端
//!   权限页渲染的数据源与 mac 同形（Windows 的 denied 恒空：无系统拒绝态）；
//! - 清单收缩（后台取消勾选）时发一条系统通知：移出清单 = agent 对其的自动化控制
//!   即时被 app_gate 拦截（app_gate 每次闸门判定都取 allowed_snapshot 实时快照），
//!   重新勾选即刻放行——通知正文与 mac 同文案（mac 的「不重弹系统框」半句在
//!   Windows 本就无框，保留整体形状便于两端文案一致）。
//! - 后台「重新授权」（appperm_reset 命令）→ reset_all：清本地 granted/denied + 立即
//!   唤醒引擎按当前清单重推一轮（Windows 无 tccutil，重置只影响本引擎的持久化状态）。
//!
//! 触发：desktop_node::start（用缓存清单播种，登录前就绪）+ 心跳清单变更（update_apps）
//! + 60s 周期兜底（引擎消息驱动的形状与 mac 一致，Windows 当前轮空转成本为零）。

use std::collections::HashSet;
use std::sync::mpsc::{self, Receiver};
use std::sync::{Mutex, OnceLock};
use std::time::Duration;

use aiarsenal_client_core::types::{AppPerm, PermStateReport};
use aiarsenal_client_core::util;
use serde_json::json;

use super::permissions;

const TICK: Duration = Duration::from_secs(60);

fn plog(msg: &str) {
    util::log("info", &format!("[desktop_node] [app-perm] {msg}"));
}

/// 引擎线程的唤醒通道（OnceLock 幂等启动；update_apps 借它通知清单更新）。
static ENGINE_TX: OnceLock<mpsc::Sender<Vec<AppPerm>>> = OnceLock::new();

/// 引擎内部状态：当前清单 + 授权/拒绝集合。
/// Windows 的 granted/denied 由清单推导（清单即授权）：granted = 清单内 bundle_id 集，
/// denied 恒空。epoch 保留（reset_all 清空时 +1 的语义与 mac 同形，当前无在途探测）。
struct State {
    apps: Vec<AppPerm>,
    granted: HashSet<String>,
    denied: HashSet<String>,
    /// 状态代数：reset_all 清空时 +1（与 mac 同形，防过期结果写回）。
    epoch: u64,
}

static STATE: Mutex<Option<State>> = Mutex::new(None);

/// 最近一次成功上报的授权状态快照（发送成功后由 mark_uploaded 记入）。
static LAST_SENT: Mutex<Option<PermStateReport>> = Mutex::new(None);

/// 启动引擎（幂等，desktop_node::start 调用）。启动即用缓存清单播种——
/// App 冷启动后、登录心跳前 app_gate 闸门就有清单可查。
pub fn start_engine() {
    let (tx, rx) = mpsc::channel::<Vec<AppPerm>>();
    if ENGINE_TX.set(tx).is_err() {
        return; // 已启动
    }
    // 播种：读缓存清单 + 持久化的授权/拒绝集
    let seeded = permissions::data_dir().map(util::load_app_perms).unwrap_or_default();
    let granted = load_granted();
    let denied = load_denied();
    plog(&format!(
        "引擎启动：缓存清单 {} 个，已授权 {} 个，已拒绝 {} 个",
        seeded.len(),
        granted.len(),
        denied.len()
    ));
    {
        let mut st = STATE.lock().unwrap();
        *st = Some(State {
            apps: seeded.clone(),
            granted,
            denied,
            epoch: 0,
        });
    }
    // 启动首跳必上报一次当前状态（LAST_SENT 置空即可）
    *LAST_SENT.lock().unwrap() = None;
    std::thread::Builder::new()
        .name("app-perm-engine".into())
        .spawn(move || engine_loop(&rx))
        .ok();
    // 启动先跑一轮（不等 60s）：把播种清单原样发一遍——收到的清单永远视为权威
    //（包括空清单=清空配置；不能用空 Vec 当占位，否则后台清空配置时引擎收不到）。
    if let Some(tx) = ENGINE_TX.get() {
        let _ = tx.send(seeded);
    }
}

/// 心跳侧调用：清单有变化时唤醒引擎。不阻塞心跳线程。
pub fn update_apps(list: &[AppPerm]) {
    let Some(tx) = ENGINE_TX.get() else {
        return; // 引擎未启动（非 desktop_node 场景）——忽略
    };
    let _ = tx.send(list.to_vec());
}

/// 当前清单快照（app_gate 闸门校验用）。引擎未启动 = 空清单 → 闸门 fail-closed
///（全部 App 禁控）。清单语义是「控制边界」：不在清单的 App，system.run 的拉起/终止
/// 载荷与 computer.act 的前台注入一律拒绝（见 app_gate.rs）。
pub fn allowed_snapshot() -> Vec<AppPerm> {
    let st = STATE.lock().unwrap();
    st.as_ref().map(|s| s.apps.clone()).unwrap_or_default()
}

fn engine_loop(rx: &Receiver<Vec<AppPerm>>) {
    loop {
        match rx.recv_timeout(TICK) {
            Ok(new_apps) => {
                // 服务端新清单（含空清单=配置被清空），一律覆盖为权威清单
                let mut st = STATE.lock().unwrap();
                if let Some(s) = st.as_mut() {
                    plog(&format!("清单更新：{} → {}", s.apps.len(), new_apps.len()));
                    notify_removed(&s.apps, &new_apps);
                    s.apps = new_apps;
                }
                drop(st);
                run_pass();
            }
            Err(mpsc::RecvTimeoutError::Timeout) => run_pass(),
            Err(mpsc::RecvTimeoutError::Disconnected) => return,
        }
    }
}

/// 清单收缩时给坐在电脑前的人发一条系统通知（独立线程——不占引擎循环）。只报移除
/// 不报新增：新增的 App 下一轮闸门即放行（无系统框可弹，天然可见）；移除完全静默且
/// 移出清单 = agent 对其的自动化控制被 app_gate 即时拦截——想恢复只需管理端重新勾选，
/// 通知正文带这条恢复路径。启动播种（新旧同清单）与 reset_all（清单未变）都算不出
/// 移除，天然不发。
fn notify_removed(old: &[AppPerm], new: &[AppPerm]) {
    let new_ids: HashSet<&str> = new.iter().map(|a| a.bundle_id.as_str()).collect();
    let names: Vec<String> = old
        .iter()
        .filter(|a| !new_ids.contains(a.bundle_id.as_str()))
        .map(|a| {
            if a.app_name.is_empty() {
                a.bundle_id.clone()
            } else {
                a.app_name.clone()
            }
        })
        .collect();
    if names.is_empty() {
        return;
    }
    // 超 5 个截断——通知横幅两行就到头，全量名字反而看不全
    let shown = if names.len() > 5 {
        format!("{} 等 {} 个 App", names[..5].join("、"), names.len())
    } else {
        names.join("、")
    };
    plog(&format!("清单移除 {} 个，发系统通知", names.len()));
    let title = "AI Arsenal".to_string();
    // 拦截语义（app_gate）：移出清单 = agent 对该 App 的自动化控制即时被拒；
    // 重新勾选即刻放行。文案与 mac 保持一致（「不重弹系统框」在 Windows 本就无框）。
    let body = format!(
        "管理端已将 {shown} 移出 App 控制清单，agent 对其的自动化控制已即时拦截。如需恢复，请在管理端重新勾选（不重弹系统框，勾选后即刻放行）"
    );
    std::thread::Builder::new()
        .name("app-perm-notify".into())
        .spawn(move || {
            // Windows 的气泡由本进程投递（PowerShell NotifyIcon），无独立通知权限开关，
            // notify_app 恒成功；保留 fallback 形状与 mac 对齐。
            if !super::system::notify_app(&title, &body) {
                let _ = super::system::notify(&json!({ "title": title, "body": body }));
            }
        })
        .ok();
}

/// 跑一轮：Windows 版 = 清单即授权，按当前清单重推授权态（无 TCC 探测、无弹框）。
/// granted = 清单内 bundle_id 集；denied 恒空（无系统拒绝态）。仅在集合变化时落盘。
fn run_pass() {
    let (targets, epoch) = {
        let st = STATE.lock().unwrap();
        let Some(s) = st.as_ref() else { return };
        (s.apps.clone(), s.epoch)
    };
    let next_granted: HashSet<String> = targets
        .iter()
        .map(|a| a.bundle_id.trim().to_string())
        .filter(|b| !b.is_empty())
        .collect();
    let mut st = STATE.lock().unwrap();
    let Some(s) = st.as_mut() else { return };
    if s.epoch != epoch {
        return; // 期间发生过 reset_all，等待其唤醒的一轮（防旧清单写回）
    }
    let mut changed = false;
    if s.granted != next_granted {
        plog(&format!(
            "授权态按清单重推：granted {} → {}（清单即授权，无 TCC 探测）",
            s.granted.len(),
            next_granted.len()
        ));
        s.granted = next_granted;
        changed = true;
    }
    // denied 恒空（Windows 无系统拒绝态）；若旧档有残留（如从 mac 语义迁来的数据）清掉
    if !s.denied.is_empty() {
        s.denied.clear();
        changed = true;
    }
    if changed {
        persist_state(&s.granted, &s.denied);
    }
}

fn load_granted() -> HashSet<String> {
    permissions::data_dir()
        .map(util::load_granted_perms)
        .unwrap_or_default()
}

fn load_denied() -> HashSet<String> {
    permissions::data_dir()
        .map(util::load_denied_perms)
        .unwrap_or_default()
}

fn persist_state(granted: &HashSet<String>, denied: &HashSet<String>) {
    if let Some(d) = permissions::data_dir() {
        util::set_perm_state(d, granted, denied);
    }
}

// ===== 授权状态主动上报（变更时随心跳带 perm_state） =====

/// 当前状态与上次已发快照不同则返回 Some（启动首跳 LAST_SENT 为空必发）。
/// 引擎未启动（非 desktop_node 场景）返回 None。
/// granted/denied 按**当前清单**过滤后上报：移出清单的不报（管理端不显示），
/// 勾回的即刻按清单即授权语义上报为已授权。
pub fn pending_upload() -> Option<PermStateReport> {
    let st = STATE.lock().unwrap();
    let s = st.as_ref()?;
    let configured: HashSet<String> = s.apps.iter().map(|a| a.bundle_id.clone()).collect();
    let cur = PermStateReport {
        granted_perms: sorted(s.granted.intersection(&configured).cloned().collect()),
        denied_perms: sorted(s.denied.intersection(&configured).cloned().collect()),
    };
    let last = LAST_SENT.lock().unwrap();
    if last.as_ref() == Some(&cur) {
        None
    } else {
        Some(cur)
    }
}

/// 心跳发送成功后记入已发快照（传实际发出的那份，避免发送途中状态再变被误标）。
pub fn mark_uploaded(sent: &PermStateReport) {
    *LAST_SENT.lock().unwrap() = Some(sent.clone());
}

fn sorted(set: HashSet<String>) -> Vec<String> {
    let mut v: Vec<String> = set.into_iter().collect();
    v.sort();
    v
}

/// 后台「重新授权」（appperm_reset 命令，platform.rs 转发）：Windows 无 tccutil——
/// 清引擎的 granted/denied 持久化态 → 立即唤醒引擎按当前清单重推一轮（清单即授权，
/// 下一跳心跳按清单原样上报回管理端）。返回清单内 App 数。
pub fn reset_all() -> Result<usize, String> {
    // mac 的 reset_apple_events（tccutil）在 Windows 是文档化 no-op
    super::permissions::reset_apple_events()?;
    let (count, granted, denied) = {
        let mut st = STATE.lock().unwrap();
        let Some(s) = st.as_mut() else { return Err("引擎未启动".into()) };
        s.granted.clear();
        s.denied.clear();
        s.epoch += 1; // 在途写回全部过期
        (s.apps.len(), s.granted.clone(), s.denied.clone())
    };
    persist_state(&granted, &denied);
    *LAST_SENT.lock().unwrap() = None; // 清空态需要立刻上报
    // 唤醒引擎立即跑一轮（engine_loop 收到消息即 run_pass，按清单重推 granted）
    let seeded = permissions::data_dir().map(util::load_app_perms).unwrap_or_default();
    update_apps(&seeded);
    plog(&format!("已重置 App 控制状态，清单 {count} 个将重新上报"));
    Ok(count)
}
