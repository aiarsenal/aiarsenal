//! macOS 自动化（AppleEvents）授权引擎——服务端可配置版。
//!
//! 背景：TCC 自动化权限按「本 App → 目标 App」逐对授予，agent 首跑 `tell application
//! "iTERM2"` 中途弹框挂起（15s/30s 超时事故）。管理后台「App 权限」按设备配置 App 清单，
//! 心跳下发（HeartbeatRes.appPerms），本引擎对清单里的 App 逐个提前触发授权框。
//!
//! 探测语义（关键约束）：
//! - `tell application id "<bundle>" to ...` 会**拉起**未运行的目标 App → 绝不能盲发。
//!   先经 System Events（启动时已授权）枚举在跑进程的 bundle id，只探「正在运行」的。
//! - 授权判定（三态）：探测成功或任何**非授权类**错误（如 "Can't get windows"）= Granted
//!   ——事件已送达目标 App 即证明该对授权已给；`not authorized`（-1743）= Denied——TCC
//!   已缓存拒绝（被拒后系统不再弹框，-1743 快速返回；框待点实际是阻塞至 -1712 超时）；
//!   目标退出（-600）或 AppleEvent 超时（-1712）= Retry 下轮再试。
//! - 授权状态持久化 aa.json 的 granted_perms/denied_perms 键（util::set_perm_state），
//!   重启不重探；granted/denied 是 TCC 真相，**不**随清单移除剪除（TCC 对不会因
//!   移出清单消失），上报时按当前清单过滤（pending_upload）——勾回的 App 随缓存
//!   真相即刻上报为已授权、不重探不弹框。denied 留在 60s 轮转里——用户在
//!   系统设置>隐私与安全性>自动化 把开关打开后，下轮探测自动转 granted。
//! - 主动上报：状态与上次已发快照（LAST_SENT）不同时，心跳带 perm_state 上去
//!   （pending_upload/mark_uploaded），发送成功才收口，失败下轮重发。
//! - 清单收缩（后台取消勾选）时发一条系统通知：移出清单**不撤销**已授予的 TCC 授权
//!   （macOS 无按目标撤销 API），但 app_gate 在 system.run spawn 前按清单拦截
//!   osascript 目标——移出 = agent 控制即时被拒；重新勾选即恢复（授权仍在，不重弹
//!   框），通知正文带恢复路径。新增不发（授权框 macOS 自己会弹，天然可见）。
//! - 后台「重新授权」（appperm_reset 命令）→ reset_all：tccutil 清掉本 App 全部
//!   AppleEvents 授权对（无按目标粒度）+ 清状态 + 立即唤醒引擎——清单内所有
//!   运行中 App 的授权框重新弹一遍。
//! - 每个探测独立短命线程：osascript 在授权框未点时会阻塞至 AppleEvent 超时（~2min），
//!   不能占住引擎循环，也不能互相挡。
//! - 触发：desktop_node::start（用缓存清单播种，登录前就开始）+ 心跳清单变更（update_apps）
//!   + 60s 周期（覆盖「App 后来才启动」的场景）。
//! - 仅 .app 内跑（dev 裸二进制 TCC 归因到终端，会误授 Terminal）。

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

/// 引擎内部状态：当前清单 + 授权/拒绝/在探集合。
struct State {
    apps: Vec<AppPerm>,
    granted: HashSet<String>,
    denied: HashSet<String>,
    probing: HashSet<String>,
    /// 状态代数：reset_all 清空时 +1，在途探测线程据此丢弃过期结果（防 reset 后又写回）。
    epoch: u64,
}

static STATE: Mutex<Option<State>> = Mutex::new(None);

/// 最近一次成功上报的授权状态快照（发送成功后由 mark_uploaded 记入）。
static LAST_SENT: Mutex<Option<PermStateReport>> = Mutex::new(None);

/// 启动授权引擎（幂等，desktop_node::start 调用）。启动即用缓存清单播种——
/// App 冷启动后、登录心跳前就能开始授权上次的清单。
pub fn start_engine() {
    let (tx, rx) = mpsc::channel::<Vec<AppPerm>>();
    if ENGINE_TX.set(tx).is_err() {
        return; // 已启动
    }
    // 播种：读缓存清单 + 已授权/已拒绝集
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
            probing: HashSet::new(),
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
/// 只放行 System Events 查询。清单语义自此从「预授权工单」升级为「控制边界」：
/// 不在清单的 App，system.run 的 osascript 载荷会被直接拒绝（见 app_gate.rs）。
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
/// 不报新增：新增的授权框 macOS 自己会弹；移除完全静默，且移出清单不撤销已授予的
/// TCC 授权（无按目标撤销 API）——想恢复只需管理端重新勾选（授权仍在，不重弹框），
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
    // 已授予的 TCC 对仍在（无按目标撤销 API），故重新勾选即刻放行、不重弹框。
    let body = format!(
        "管理端已将 {shown} 移出 App 控制清单，agent 对其的自动化控制已即时拦截。如需恢复，请在管理端重新勾选（不重弹系统框，勾选后即刻放行）"
    );
    std::thread::Builder::new()
        .name("app-perm-notify".into())
        .spawn(move || {
            // 优先本 App 名义（UNUserNotificationCenter）——osascript 的 display notification
            // 归属 Script Editor，本 App 通知权限管不到，横幅会被吞；dev 裸二进制无 bundle
            // 时 notify_app 返回 false，回落 osascript（开发环境横幅可能不显示，仅日志可查）。
            if !super::system::notify_app(&title, &body) {
                let _ = super::system::notify(&json!({ "title": title, "body": body }));
            }
        })
        .ok();
}

/// 跑一轮：清单 ∩ 运行中 − 已授权 − 在探 → 逐个探测。
fn run_pass() {
    if !permissions::in_app_bundle() {
        return; // dev 裸二进制：TCC 归因到终端，静默跳过（不刷日志）
    }
    let targets = {
        let st = STATE.lock().unwrap();
        let Some(s) = st.as_ref() else { return };
        s.apps.clone()
    };
    if targets.is_empty() {
        return; // 未配置：空转不打扰（System Events 授权是内置常开的另一路）
    }
    let running = match running_bundle_ids() {
        Some(set) => set,
        None => return, // System Events 未授权/失败——下轮再试
    };
    let mut to_probe: Vec<AppPerm> = Vec::new();
    {
        let mut st = STATE.lock().unwrap();
        let Some(s) = st.as_mut() else { return };
        // 注意：granted/denied 是 TCC 真相，**不**随清单移除剪除——移出再勾回的 App
        // 授权仍在（无按目标撤销 API），无需重探、不重弹框，上报按清单过滤即可
        //（见 pending_upload）。剪除会卡住「未运行的已勾回 App」：探测只对运行中的
        // 做，剪掉后永远探不回来，UI 一直显示未授权。
        for a in &targets {
            if s.granted.contains(&a.bundle_id)
                || s.probing.contains(&a.bundle_id)
                || !running.contains(&a.bundle_id)
            {
                continue; // denied 不跳过：留在轮转里，系统设置开关打开后下轮自动转 granted
            }
            s.probing.insert(a.bundle_id.clone());
            to_probe.push(a.clone());
        }
    }
    if to_probe.is_empty() {
        return;
    }
    plog(&format!(
        "本轮探测 {} 个（清单 {} / 运行中命中）",
        to_probe.len(),
        targets.len()
    ));
    for a in to_probe {
        let bundle = a.bundle_id.clone();
        let name = if a.app_name.is_empty() { bundle.clone() } else { a.app_name.clone() };
        let epoch = {
            let st = STATE.lock().unwrap();
            st.as_ref().map(|s| s.epoch).unwrap_or(0)
        };
        std::thread::Builder::new()
            .name(format!("app-perm-{name}"))
            .spawn(move || {
                let outcome = probe(&bundle, &name);
                let mut st = STATE.lock().unwrap();
                let Some(s) = st.as_mut() else { return };
                s.probing.remove(&bundle);
                if s.epoch != epoch {
                    return; // 探测期间发生过 reset——结果过期，丢弃
                }
                let mut changed = false;
                match outcome {
                    ProbeOutcome::Granted => {
                        changed = s.granted.insert(bundle.clone());
                        if s.denied.remove(&bundle) {
                            changed = true;
                        }
                    }
                    ProbeOutcome::Denied => {
                        changed = s.denied.insert(bundle.clone());
                        if s.granted.remove(&bundle) {
                            changed = true;
                        }
                    }
                    ProbeOutcome::Retry => {}
                }
                if changed {
                    persist_state(&s.granted, &s.denied);
                }
            })
            .ok();
    }
}

/// System Events 枚举在跑进程的 bundle id。失败（未授权/osascript 异常）返回 None。
fn running_bundle_ids() -> Option<HashSet<String>> {
    let out = std::process::Command::new("osascript")
        .arg("-e")
        .arg(
            r#"tell application "System Events" to get bundle identifier of every process"#,
        )
        .output()
        .ok()?;
    if !out.status.success() {
        plog(&format!(
            "枚举运行进程失败（System Events 未授权？）：{}",
            String::from_utf8_lossy(&out.stderr).trim()
        ));
        return None;
    }
    let text = String::from_utf8_lossy(&out.stdout);
    Some(
        text.split(',')
            .map(|s| s.trim().trim_matches('"').to_string())
            .filter(|s| !s.is_empty() && s != "missing value")
            .collect(),
    )
}

/// 单次探测的三态结果。
enum ProbeOutcome {
    /// 事件已送达（成功或非授权类错误）——该对授权已给。
    Granted,
    /// TCC 缓存拒绝（-1743）——用户点过「不允许」，系统不再弹框；
    /// 留在轮转里等系统设置>自动化 开关恢复。
    Denied,
    /// 未送达/超时/异常——下轮再试。
    Retry,
}

/// 探测一个 App 的自动化授权。
fn probe(bundle: &str, name: &str) -> ProbeOutcome {
    // count windows 是无副作用的最小 Apple event；多数 App 响应，不支持窗口的也会因
    // 「事件已送达」而落在非授权错误里 → 判定授权成功。
    let script = format!(r#"tell application id "{bundle}" to count windows"#);
    plog(&format!("探测 {name}（{bundle}）：未授权时将弹一次系统框"));
    let out = std::process::Command::new("osascript")
        .arg("-e")
        .arg(&script)
        .output();
    match out {
        Ok(o) if o.status.success() => {
            plog(&format!("{name} 已授权（探测成功）"));
            ProbeOutcome::Granted
        }
        Ok(o) => {
            let err = String::from_utf8_lossy(&o.stderr).to_lowercase();
            if err.contains("not authorized") || err.contains("-1743") {
                plog(&format!("{name} 已拒绝（TCC 缓存 -1743，需系统设置开回或重置）"));
                ProbeOutcome::Denied
            } else if err.contains("isn't running") || err.contains("-600") {
                // 枚举运行进程后、探测前目标恰好退出——事件未送达，不算授权，下轮再探
                plog(&format!("{name} 探测时已退出（未送达），下轮再试"));
                ProbeOutcome::Retry
            } else if err.contains("timed out") || err.contains("-1712") {
                // 授权框一直没点，AppleEvent 超时——事件未送达，不算授权（也非拒绝）
                plog(&format!("{name} 探测超时（框未点或阻塞），下轮再试"));
                ProbeOutcome::Retry
            } else {
                // 事件已送达（如 Can't get windows）——该对授权实际已给
                plog(&format!(
                    "{name} 已授权（事件已送达：{}）",
                    String::from_utf8_lossy(&o.stderr).trim()
                ));
                ProbeOutcome::Granted
            }
        }
        Err(e) => {
            plog(&format!("{name} 探测失败（osascript 异常 {e}），下轮再试"));
            ProbeOutcome::Retry
        }
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
/// 勾回的即刻随缓存真相上报为已授权——兑现「重新勾选即恢复显示、不重弹框」。
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

/// 后台「重新授权」（appperm_reset 命令）：tccutil 清掉本 App 全部 AppleEvents
/// 授权对（无按目标粒度）→ 清引擎状态并落盘 → 立即唤醒引擎全量重弹。
/// 返回清单内 App 数（即将重弹框的规模上限）。
pub fn reset_all() -> Result<usize, String> {
    super::permissions::reset_apple_events()?;
    let (count, granted, denied) = {
        let mut st = STATE.lock().unwrap();
        let Some(s) = st.as_mut() else { return Err("引擎未启动".into()) };
        s.granted.clear();
        s.denied.clear();
        s.epoch += 1; // 在途探测结果全部过期
        (s.apps.len(), s.granted.clone(), s.denied.clone())
    };
    persist_state(&granted, &denied);
    *LAST_SENT.lock().unwrap() = None; // 清空态需要立刻上报
    // 唤醒引擎立即跑一轮（engine_loop 收到消息即 run_pass）
    let seeded = permissions::data_dir().map(util::load_app_perms).unwrap_or_default();
    update_apps(&seeded);
    plog(&format!("已重置自动化授权（tccutil），清单 {} 个将重新弹框", count));
    Ok(count)
}
