//! macOS 平台实现：openclaw / hermes / obsidian 的安装、启停、配置。
//!
//! - openclaw：npm 全局包（Node CLI），`openclaw gateway run` 子进程，配置 `~/.openclaw/openclaw.json`
//!   全局安装可能需要管理员权限：失败时回退用 osascript 提权（弹系统授权框 = 用户授权）
//! - hermes：原生 Python（公网包安装，无 Docker），`hermes gateway run` 子进程
//! - obsidian：官方 .dmg，`hdiutil attach` + 拷到 /Applications（提权）+ `open -a`/`osascript quit`
//!
//! 各 agent 的实现拆到子模块（openclaw/hermes/obsidian），本文件只留共享 dispatch /
//! execute_command / 配置 / 工具。GUI 应用 PATH 缺失 /opt/homebrew/bin 等，统一注入。

#[cfg(feature = "agent-hermes")]
mod hermes;
#[cfg(feature = "agent-obsidian")]
mod obsidian;
#[cfg(feature = "agent-openclaw")]
mod openclaw;
mod permissions;
// openclaw desktop node（client 自实现，替代 OpenClaw.app）：feature 禁用时整个模块不编译（可卸载）。
// pub：lib.rs 的权限清单页命令直接转发到 desktop_node::permissions（exec 审批/canvas 同理）。
#[cfg(feature = "openclaw-desktop")]
pub mod desktop_node;

// 权限引导（托盘菜单用）：转发到 desktop_node::permissions。feature 禁用时无 desktop node、不需要权限，
// 这几个 fn 不编译；lib.rs 的权限菜单项也在同 feature cfg 下，配套缺省。
#[cfg(feature = "openclaw-desktop")]
pub fn ensure_screen_recording(interactive: bool) -> bool {
    desktop_node::permissions::ensure_screen_recording(interactive)
}
#[cfg(feature = "openclaw-desktop")]
pub fn ensure_accessibility(interactive: bool) -> bool {
    desktop_node::permissions::ensure_accessibility(interactive)
}
#[cfg(feature = "openclaw-desktop")]
pub fn open_screen_recording_settings() {
    desktop_node::permissions::open_screen_recording_settings()
}
#[cfg(feature = "openclaw-desktop")]
pub fn open_accessibility_settings() {
    desktop_node::permissions::open_accessibility_settings()
}

use std::collections::HashMap;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Condvar, Mutex};

use aiarsenal_client_core::ChannelLoginState;

/// 版本对齐确认门的共享态：install 线程发起（pending=true）→ 前端弹窗 →
/// client_confirm_openclaw_align 回填 response → condvar 唤醒等待方。
#[derive(Default)]
struct AlignConfirmState {
    pending: bool,
    response: Option<bool>,
}

/// 长驻子进程注册表（openclaw 子进程句柄）。`Arc` 使其可 clone，
/// 便于在 `#[tauri::command] async` 中把副本 move 进 `spawn_blocking`。
#[derive(Clone)]
pub struct PlatformState {
    procs: Arc<Mutex<HashMap<String, Child>>>,
    /// 渠道登录进行中状态（channel_login 命令写，心跳读回传后端）。None=无登录在进行。
    channel_login: Arc<Mutex<Option<ChannelLoginState>>>,
    /// caffeinate 子进程（防锁屏/防睡眠）。None=未启用或已退出。
    keep_awake: Arc<Mutex<Option<Child>>>,
    /// AppHandle（setup 时回填）：确认弹窗要 emit 事件 + 把主窗口从托盘拉出来。
    app: Arc<Mutex<Option<tauri::AppHandle>>>,
    /// openclaw 版本对齐（升/降级）用户确认门。
    align: Arc<(Mutex<AlignConfirmState>, Condvar)>,
    /// 设备 API token（心跳循环在认领 skill_install 前回填）：下载技能整包要 Bearer。
    /// 空串=未登录。不进命令 payload（会落库），只活在内存。
    api_token: Arc<Mutex<String>>,
}

impl Default for PlatformState {
    fn default() -> Self {
        Self {
            procs: Arc::new(Mutex::new(HashMap::new())),
            channel_login: Arc::new(Mutex::new(None)),
            keep_awake: Arc::new(Mutex::new(None)),
            app: Arc::new(Mutex::new(None)),
            align: Arc::new((
                Mutex::new(AlignConfirmState::default()),
                Condvar::new(),
            )),
            api_token: Arc::new(Mutex::new(String::new())),
        }
    }
}

impl PlatformState {
    /// setup 时回填 AppHandle（manage 发生在 setup 之前，构造时拿不到）。
    pub fn set_app_handle(&self, h: tauri::AppHandle) {
        *self.app.lock().unwrap() = Some(h);
    }

    /// 心跳循环回填设备 token（认领 skill_install 前调用），供下载端点 Bearer 鉴权。
    pub fn set_api_token(&self, token: &str) {
        *self.api_token.lock().unwrap() = token.to_string();
    }

    /// 请求本机用户确认 openclaw 版本对齐：emit `openclaw-align-confirm` + 懒拉起主窗口，
    /// 然后阻塞等待用户回应。等待期间每 30s 走一次 on_log 刷新命令进度（防服务端 idle 超时）。
    /// 返回 Some(true)=确认 / Some(false)=拒绝 / None=超时（10 分钟）。
    ///
    /// 事件每 2s 重发一次直到有回应：懒建窗口时前端 JS 可能还没加载完（listen 未挂上），
    /// 只发一次会丢事件 → 永远等不到回应只能超时。前端收重复事件幂等（同一弹窗刷新数据）。
    pub fn request_align_confirm(
        &self,
        payload: &serde_json::Value,
        on_log: &(dyn Fn(&str) + Send + Sync),
    ) -> Option<bool> {
        use tauri::Emitter;
        let app = self.app.lock().unwrap().clone();
        if let Some(app) = &app {
            ensure_main_window(app);
        }
        let (m, cv) = &*self.align;
        let mut g = m.lock().unwrap();
        g.pending = true;
        g.response = None;
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(600);
        let mut last_progress = std::time::Instant::now() - std::time::Duration::from_secs(30);
        loop {
            if let Some(r) = g.response.take() {
                g.pending = false;
                return Some(r);
            }
            let now = std::time::Instant::now();
            if now >= deadline {
                g.pending = false;
                return None;
            }
            // 事件重发（2s 一发）；进度日志 30s 一发（防服务端 idle 超时）
            if let Some(app) = &app {
                let _ = app.emit("openclaw-align-confirm", payload);
            }
            if last_progress.elapsed() >= std::time::Duration::from_secs(30) {
                last_progress = std::time::Instant::now();
                on_log("等待本机用户确认版本对齐（确认框已弹出，10 分钟超时）…");
            }
            let slice = std::time::Duration::from_secs(2).min(deadline - now);
            let (ng, _) = cv.wait_timeout(g, slice).unwrap();
            g = ng;
        }
    }

    /// 前端确认/拒绝回调（client_confirm_openclaw_align 命令）。false=当前无待确认请求。
    pub fn respond_align_confirm(&self, accept: bool) -> bool {
        let (m, cv) = &*self.align;
        let mut g = m.lock().unwrap();
        if !g.pending {
            return false;
        }
        g.response = Some(accept);
        cv.notify_all();
        true
    }
}

type OpOut = (bool, String, String); // (success, message, pid_or_container_id)

/// 确保 client desktop node 已启动（幂等：已启动则跳过）。
/// feature openclaw-desktop：client 自实现 desktop node 替代 OpenClaw.app，给 agent 提供
/// 截屏/控鼠键能力。心跳登录后调一次，让它跟着 client 自动起（不依赖后端下发 start 命令）；
/// desktop_node 内部自带重连，gateway 没起好会重试到连上。
/// pending pairing 的自动批准在 protocol.rs 握手成功（hello-ok）后触发——每次重连/声明新命令
/// 都会有新 pending，跟着握手走才不漏。
#[cfg(feature = "openclaw-desktop")]
pub fn ensure_desktop_node(app: &tauri::AppHandle) {
    let (_ok, msg) = desktop_node::start(app.clone());
    aiarsenal_client_core::util::log("info", &format!("[desktop_node] auto: {msg}"));
    // canvas 5 命令非 gateway 平台默认集，必须进 gateway.nodes.allowCommands 才能被路由。
    // 合并式写（读→并集→缺才写），存量机器不重装也能拿到。
    ensure_canvas_allow_commands();
}
#[cfg(not(feature = "openclaw-desktop"))]
pub fn ensure_desktop_node(_app: &tauri::AppHandle) {}

/// 读本 node 的 deviceId（~/.openclaw/identity/client-node.json → deviceId）。
#[cfg(feature = "openclaw-desktop")]
fn own_node_id() -> Option<String> {
    let p = dirs::home_dir()?.join(".openclaw").join("identity").join("client-node.json");
    let s = std::fs::read_to_string(&p).ok()?;
    serde_json::from_str::<serde_json::Value>(&s).ok()?
        .get("deviceId")?
        .as_str()
        .map(String::from)
}

/// 轮询 gateway pending pairing，找到本 node 的就 approve（自动批准）。
/// gateway 在 node 连上后建 pending；client 用本地 `openclaw nodes approve`（gateway 自身权限）批准。
#[cfg(feature = "openclaw-desktop")]
fn approve_own_node_if_pending() {
    let node_id = match own_node_id() {
        Some(id) => id,
        None => return,
    };
    let bin = openclaw_bin();
    if !PathBuf::from(&bin).is_file() {
        return;
    }
    // desktop_node 线程刚起，给点时间连上 + gateway 建 pending；轮询几次。
    for attempt in 0..8u32 {
        if attempt > 0 {
            std::thread::sleep(std::time::Duration::from_secs(2));
        }
        // nodes pending --json 只取 stdout（stderr 的 config 告警会破坏 JSON 解析）
        let out = match std::process::Command::new(&bin)
            .args(["nodes", "pending", "--json"])
            .env("PATH", mac_path())
            .output()
        {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(_) => continue,
        };
        let req_id = serde_json::from_str::<serde_json::Value>(&out)
            .ok()
            .and_then(|v| v.as_array().map(|a| a.clone()))
            .unwrap_or_default()
            .into_iter()
            .find(|r| r.get("nodeId").and_then(|n| n.as_str()) == Some(&node_id))
            .and_then(|r| r.get("requestId").and_then(|i| i.as_str()).map(String::from));
        if let Some(rid) = req_id {
            let (aok, _) = run_cmd(&bin, &["nodes", "approve", &rid]);
            aiarsenal_client_core::util::log("info", &format!("[desktop_node] 自动批准 node pairing ok={aok}"));
            return;
        }
    }
}

/// client 自实现、gateway 平台默认集之外的命令（须进 gateway.nodes.allowCommands 才能路由）。
/// install 时整体覆盖写入同一份（openclaw.rs 步骤3.1），两处须保持一致。
#[cfg(feature = "openclaw-desktop")]
const DESKTOP_EXTRA_ALLOW_COMMANDS: &[&str] = &[
    "computer.act",
    "screen.record",
    "camera.snap",
    "camera.clip",
    "canvas.present",
    "canvas.hide",
    "canvas.navigate",
    "canvas.eval",
    "canvas.snapshot",
    "canvas.a2ui.push",
    "canvas.a2ui.pushJSONL",
    "canvas.a2ui.reset",
];

/// 把 DESKTOP_EXTRA_ALLOW_COMMANDS 合并进 gateway.nodes.allowCommands（读→并集→缺才写）。
/// 不能整体覆盖：用户/上游可能已往里加过自定义命令。幂等（不缺则不写），
/// ensure_desktop_node 每次登录心跳调一次——升级 client 后存量机器不用重装。
#[cfg(feature = "openclaw-desktop")]
fn ensure_canvas_allow_commands() {
    let bin = openclaw_bin();
    if !PathBuf::from(&bin).is_file() {
        return;
    }
    // config get 只取 stdout：stderr 的 config 告警会破坏 JSON 解析（同 nodes pending 的坑）。
    let out = match std::process::Command::new(&bin)
        .args(["config", "get", "gateway.nodes.allowCommands"])
        .env("PATH", mac_path())
        .output()
    {
        Ok(o) => String::from_utf8_lossy(&o.stdout).trim().to_string(),
        Err(_) => return,
    };
    let current: Vec<String> = serde_json::from_str::<serde_json::Value>(&out)
        .ok()
        .and_then(|v| v.as_array().cloned())
        .map(|a| {
            a.into_iter()
                .filter_map(|x| x.as_str().map(String::from))
                .collect()
        })
        .unwrap_or_default();
    let missing: Vec<&str> = DESKTOP_EXTRA_ALLOW_COMMANDS
        .iter()
        .filter(|c| !current.iter().any(|x| x == *c))
        .copied()
        .collect();
    if missing.is_empty() {
        return;
    }
    let mut merged = current;
    merged.extend(missing.iter().map(|s| s.to_string()));
    let val = serde_json::to_string(&merged).unwrap_or_default();
    let (ok, _) = run_cmd(&bin, &["config", "set", "gateway.nodes.allowCommands", &val]);
    aiarsenal_client_core::util::log(
        "info",
        &format!("[desktop_node] allowCommands 合并写入 +{missing:?} ok={ok}"),
    );
}

/// 懒创建/唤起主窗口（mac 纯托盘形态：tauri.conf.json 无预建窗口，按需拉出）。
/// 已有（含隐藏到托盘）→ show + focus；没有 → 建 1040×720 居中。
/// capabilities/default.json 已含 windows:["main"]，CloseRequested→hide 拦截在 lib.rs，
/// 窗口建后常驻（关=隐藏）。后台线程可安全调（wry 经事件循环 proxy 创建窗口）。
pub fn ensure_main_window(app: &tauri::AppHandle) -> Option<tauri::WebviewWindow> {
    use tauri::Manager;
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.show();
        let _ = w.set_focus();
        return Some(w);
    }
    match tauri::WebviewWindowBuilder::new(
        app,
        "main",
        tauri::WebviewUrl::App("index.html".into()),
    )
    .title("AI Arsenal")
    .inner_size(1040.0, 720.0)
    .center()
    .build()
    {
        Ok(w) => {
            let _ = w.set_focus();
            Some(w)
        }
        Err(e) => {
            aiarsenal_client_core::util::log("error", &format!("主窗口创建失败: {e}"));
            None
        }
    }
}

/// 本构建打包进的 agent 列表（按 cargo feature `agent-xxx`）。供心跳上报 / collect_data 等
/// 遍历 agent 的地方用，替代写死的 `["openclaw","hermes","obsidian"]`（避免带上未打包的 agent）。
/// client/build.sh `--agents openclaw,hermes` → `--features agent-openclaw,agent-hermes` 决定本表。
pub fn compiled_agents() -> Vec<&'static str> {
    let mut v = Vec::new();
    #[cfg(feature = "agent-openclaw")]
    v.push("openclaw");
    #[cfg(feature = "agent-hermes")]
    v.push("hermes");
    #[cfg(feature = "agent-obsidian")]
    v.push("obsidian");
    v
}

/// 汇总各已打包 agent 的已装技能（心跳 skills 快照 + 生命周期上报共用，纯本地 FS 读）。
/// openclaw：~/.openclaw/skills 平铺扫描（slug=一级目录）；hermes：$HERMES_HOME/skills
/// 全树扫描（每个直接含 SKILL.md 的目录是技能，支持 category 嵌套）。
/// 同 slug 去重（openclaw 优先——存量设备的既有语义），按 name 排序（后端 diff 稳定）。
pub fn scan_all_skills() -> Vec<aiarsenal_client_core::types::SkillStateReport> {
    let mut out = Vec::new();
    let mut seen: Vec<String> = Vec::new();
    for agent in compiled_agents() {
        let list = match agent {
            "openclaw" => dirs::home_dir()
                .map(|h| {
                    aiarsenal_client_core::skills::scan_skills_flat(&h.join(".openclaw").join("skills"))
                })
                .unwrap_or_default(),
            #[cfg(feature = "agent-hermes")]
            "hermes" => {
                aiarsenal_client_core::skills::scan_skills_grouped(&hermes::hermes_home().join("skills"))
            }
            _ => continue,
        };
        for s in list {
            if !seen.contains(&s.slug) {
                seen.push(s.slug.clone());
                out.push(s);
            }
        }
    }
    out.sort_by(|a, b| a.name.cmp(&b.name));
    out
}

/// 安装时把子进程逐行输出回调出去（前端展示实时进度）。
/// `Send + Sync` 因为流式执行时 stdout/stderr 在两个 scope 线程里同时调用。
pub fn install_with_spec(
    agent: &str,
    spec: &serde_json::Value,
    state: &PlatformState,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> OpOut {
    let out = match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => openclaw::install_openclaw(state, spec, on_log),
        #[cfg(feature = "agent-hermes")]
        "hermes" => hermes::install_hermes(spec, on_log),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => obsidian::install_obsidian(spec, on_log),
        _ => return (false, format!("未知或未打包的客户端: {agent}"), String::new()),
    };
    let (ok, msg, pid) = out;
    // mac：hermes 装好后预申请 TCC 权限（cua-driver）。openclaw 的权限由 feature 分支处理
    // （启用：desktop_node::start 申请 TCC；禁用：OpenClaw.app Onboarding 引导），无需此处额外处理。
    #[cfg(feature = "agent-hermes")]
    if ok && agent == "hermes" {
        permissions::request_hermes_permissions(on_log);
    }
    (ok, msg, pid)
}

pub fn start_with_spec(agent: &str, spec: &serde_json::Value, state: &PlatformState) -> OpOut {
    let _ = spec; // spec 供各 agent start 按需读取；未打包 agent 不用
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => openclaw::start_openclaw(state),
        #[cfg(feature = "agent-hermes")]
        "hermes" => hermes::start_hermes(state),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => obsidian::start_obsidian(),
        _ => (false, format!("未知或未打包的客户端: {agent}"), String::new()),
    }
}

pub fn stop_agent(agent: &str, state: &PlatformState) -> OpOut {
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => openclaw::stop_openclaw(state),
        #[cfg(feature = "agent-hermes")]
        "hermes" => hermes::stop_hermes(state),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => obsidian::stop_obsidian(),
        _ => (false, format!("未知或未打包的客户端: {agent}"), String::new()),
    }
}

pub fn uninstall_agent(agent: &str, state: &PlatformState, on_log: &(dyn Fn(&str) + Send + Sync)) -> (bool, String) {
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => {
            // 停 gateway（stop_openclaw 内含 feature desktop_node::stop + launchd bootout）
            on_log("停止 openclaw gateway");
            let _ = openclaw::stop_openclaw(state);
            // 卸载不销毁用户数据：~/.openclaw（含用户自用的配置 / identity / 渠道登录态 /
            // 插件）整体挪到 ~/.openclaw.bak.<ts>，确认无需后由用户手动删。
            let home = std::env::var("HOME").unwrap_or_default();
            let ts = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_secs())
                .unwrap_or(0);
            let bak = format!("{home}/.openclaw.bak.{ts}");
            #[cfg(feature = "openclaw-desktop")]
            let script = format!("mv '{home}/.openclaw' '{bak}'");
            #[cfg(not(feature = "openclaw-desktop"))]
            let script = format!("rm -rf /Applications/OpenClaw.app && mv '{home}/.openclaw' '{bak}'");
            let (ok, e) = run_osascript_admin(&script);
            let msg = if ok {
                format!("已卸载 openclaw；原 ~/.openclaw 数据备份至 {bak}（确认无需后可手动删除）")
            } else {
                format!("卸载 openclaw 失败: {e}")
            };
            (ok, msg)
        }
        #[cfg(feature = "agent-hermes")]
        "hermes" => {
            on_log("卸载 hermes（删除安装目录 + 本地 origin + 源码缓存）");
            let home = hermes::hermes_home();
            let _ = std::fs::remove_dir_all(home.join("hermes-agent"));
            // zip 预置法产物：本地 bare origin（重装时会重建）与源码 zip 缓存（~75MB）
            let _ = std::fs::remove_dir_all(home.join(".hermes-origin.git"));
            let _ = std::fs::remove_dir_all(home.join(".aa-cache"));
            (true, "已卸载 hermes".into())
        }
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => (false, "obsidian 请拖到废纸篓卸载".into()),
        _ => (false, format!("未知或未打包的客户端: {agent}")),
    }
}

pub fn get_agent_runtime(agent: &str) -> (bool, bool, String, String) {
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => openclaw::openclaw_runtime(),
        #[cfg(feature = "agent-hermes")]
        "hermes" => hermes::hermes_runtime(),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => obsidian::obsidian_runtime(),
        _ => (false, false, String::new(), String::new()),
    }
}

/// openclaw 环境探测（本机多来源版本 + 是否需对齐），心跳 modules.openclaw.env 用。
#[cfg(feature = "agent-openclaw")]
pub fn openclaw_env_report() -> serde_json::Value {
    openclaw::env_report_cached()
}

/// 取当前渠道登录进行中状态（心跳读取回传后端）。无登录在进行返回 None。
pub fn channel_login_state(state: &PlatformState) -> Option<ChannelLoginState> {
    state.channel_login.lock().unwrap().clone()
}

/// 设置/清空渠道登录状态（channel_login 命令执行期写）。
pub fn set_channel_login_state(state: &PlatformState, v: Option<ChannelLoginState>) {
    *state.channel_login.lock().unwrap() = v;
}

// ==================== 防锁屏 / 防睡眠 ====================
//
// openclaw / hermes 要持续操作电脑，系统空闲锁屏/睡眠会中断操作。
// 起 `caffeinate -dis -w <本进程pid>`：
//   -d 防显示器睡眠（即防锁屏链路）、-i 防空闲睡眠、-s 防系统睡眠（AC 电源）；
//   -w <pid> 让 caffeinate 在 App 退出/崩溃时自动结束，不留孤儿 assertion。
// 句柄存 PlatformState.keep_awake，关掉时 kill。

/// 开启防锁屏。已开启（子进程在跑）则幂等返回 true。
pub fn keep_awake_enable(state: &PlatformState) -> bool {
    let mut g = state.keep_awake.lock().unwrap();
    // 已有子进程且未退出 → 幂等成功
    if let Some(c) = g.as_mut() {
        if c.try_wait().ok().flatten().is_none() {
            return true;
        }
    }
    let our_pid = std::process::id().to_string();
    match Command::new("caffeinate")
        .args(["-dis", "-w", &our_pid])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
    {
        Ok(child) => {
            *g = Some(child);
            true
        }
        Err(e) => {
            aiarsenal_client_core::util::log("error", &format!("[keep-awake] 启动 caffeinate 失败: {e}"));
            false
        }
    }
}

/// 关闭防锁屏（kill caffeinate 子进程，恢复系统默认空闲策略）。
pub fn keep_awake_disable(state: &PlatformState) {
    let mut g = state.keep_awake.lock().unwrap();
    if let Some(mut c) = g.take() {
        let _ = c.kill();
        let _ = c.wait();
    }
}

/// 防锁屏当前是否生效（子进程存在且未退出）。
pub fn keep_awake_is_on(state: &PlatformState) -> bool {
    let mut g = state.keep_awake.lock().unwrap();
    g.as_mut()
        .map(|c| c.try_wait().ok().flatten().is_none())
        .unwrap_or(false)
}

/// 远程指令执行入口：按 action 分发到既有 install/start/stop/uninstall/config 函数。
/// 返回 (status, result_json, error)。status="done"/"failed"。
pub fn execute_command(
    cmd: &aiarsenal_client_core::ClientCommand,
    state: &PlatformState,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (String, String, String) {
    let action = cmd.action.as_str();
    let agent = cmd.agent.as_str();
    // 后端 Payload 是 json.RawMessage，序列化成裸 JSON 对象，cmd.payload 直接是 Object。
    // install/start 用 payload.spec；若无，用 payload 自身作 spec
    let spec = cmd.payload.get("spec").unwrap_or(&cmd.payload);
    let mk = |ok: bool, msg: String| -> (String, String, String) {
        if ok {
            ("done".to_string(), format!("{{\"message\":{}}}", serde_json::Value::String(msg)), String::new())
        } else {
            ("failed".to_string(), "{}".to_string(), msg)
        }
    };
    // 若 payload 带 configContent（install/restart/start openclaw 由后端塞入新签 key 的配置），
    // 先写 openclaw.json 再执行原逻辑——单条命令完成「更新配置 + 装启停」，避免命令链回传竞态。
    if let Some(content) = cmd.payload.get("configContent").and_then(|v| v.as_str()).filter(|s| !s.is_empty()) {
        if agent == "openclaw" {
            on_log(&format!("远程：更新配置 {}", agent));
            let (ok, _, _, msg) = save_agent_config(agent, content);
            if !ok {
                return mk(false, format!("更新配置失败: {msg}"));
            }
            on_log("配置已更新");
        } else if agent == "hermes" {
            #[cfg(feature = "agent-hermes")]
            {
                // hermes：下发内容落 config.base.yaml，write_effective_config 按微信绑定态生成实际 config.yaml。
                on_log("远程：更新 hermes 配置");
                match hermes::save_hermes_base_config(content) {
                    Ok(()) => on_log("hermes 配置已更新"),
                    Err(e) => return mk(false, format!("更新 hermes 配置失败: {e}")),
                }
            }
            #[cfg(not(feature = "agent-hermes"))]
            {
                return mk(false, "hermes 未打包进本构建".to_string());
            }
        }
    }
    match action {
        "install" => {
            on_log(&format!("远程：安装 {} {}", agent, cmd.version));
            let (ok, msg, _) = install_with_spec(agent, spec, state, on_log);
            mk(ok, msg)
        }
        "uninstall" => {
            on_log(&format!("远程：卸载 {}", agent));
            let (ok, msg) = uninstall_agent(agent, state, on_log);
            mk(ok, msg)
        }
        "start" => {
            on_log(&format!("远程：启动 {}", agent));
            let (ok, msg, _) = start_with_spec(agent, spec, state);
            mk(ok, msg)
        }
        "stop" => {
            on_log(&format!("远程：停止 {}", agent));
            let (ok, msg, _) = stop_agent(agent, state);
            mk(ok, msg)
        }
        "restart" => {
            on_log(&format!("远程：重启 {}", agent));
            if agent == "hermes" {
                // hermes：stop + start 重启（对齐 channel_login 的重启方式，最稳）。
                // 不能沿用下方 openclaw 的 `gateway restart`——那是 openclaw 专属命令，hermes 走自己的二进制。
                let _ = stop_agent(agent, state);
                let (ok, msg, _) = start_with_spec(agent, spec, state);
                on_log(&format!("启动: {msg}"));
                mk(ok, msg)
            } else {
                // openclaw：官方 `gateway restart` 发 SIGUSR1 触发 in-process 重启，
                // 优雅（排空活跃任务、状态不丢）、配置随重启重新加载。gateway 没跑时它返回非 0 → fallback 到启动。
                let (rc, out) = run_cmd(&openclaw_bin(), &["gateway", "restart"]);
                if rc {
                    on_log("已发重启信号（gateway restart）");
                    mk(true, "openclaw 已重启（gateway restart）".into())
                } else {
                    on_log(&format!("gateway restart 未生效（{}），改用启动", out.trim()));
                    let (ok, msg, _) = start_with_spec(agent, spec, state);
                    on_log(&format!("启动: {msg}"));
                    mk(ok, msg)
                }
            }
        }
        "config_push" => {
            on_log(&format!("远程：下发配置 {}", agent));
            let content = cmd.payload.get("configContent").and_then(|v| v.as_str()).unwrap_or("");
            let (ok, _, _, msg) = save_agent_config(agent, content);
            mk(ok, msg)
        }
        "skill_install" => {
            // 技能安装：下载整包 zip（设备 token）→ 解压 → 定位 SKILL.md 根 → 按 agent 落地：
            // - openclaw：openclaw skills install --as slug --global（装进全局托管 ~/.openclaw/skills）
            // - hermes：直接落盘 $HERMES_HOME/skills/<slug>/（hermes 枚举是全树扫盘，见 hermes.rs）
            let slug = cmd.payload.get("slug").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let name = cmd.payload.get("name").and_then(|v| v.as_str()).unwrap_or(&slug).to_string();
            let url = cmd.payload.get("packageUrl").and_then(|v| v.as_str()).unwrap_or("").to_string();
            on_log(&format!("远程：安装技能 {name}"));
            if slug.is_empty() || url.is_empty() {
                return mk(false, "skill_install 缺少 slug/packageUrl".into());
            }
            let tmp = std::env::temp_dir().join(format!("aa-skill-{}", cmd.command_id));
            let _ = std::fs::remove_dir_all(&tmp);
            let extract = tmp.join("x");
            if std::fs::create_dir_all(&extract).is_err() {
                return mk(false, format!("创建临时目录失败: {}", tmp.display()));
            }
            let zip = tmp.join("skill.zip");
            let token = state.api_token.lock().unwrap().clone();
            let (ok, msg) = download_file(&url, &zip, if token.is_empty() { None } else { Some(&token) });
            if !ok {
                let _ = std::fs::remove_dir_all(&tmp);
                return mk(false, format!("下载技能包失败: {msg}"));
            }
            on_log("下载完成，解压中");
            let (ok, msg) = run_cmd("/usr/bin/unzip", &["-q", &zip.display().to_string(), "-d", &extract.display().to_string()]);
            if !ok {
                let _ = std::fs::remove_dir_all(&tmp);
                return mk(false, format!("解压失败: {msg}"));
            }
            let Some(root) = aiarsenal_client_core::skills::locate_skill_root(&extract) else {
                let _ = std::fs::remove_dir_all(&tmp);
                return mk(false, "包内未找到 SKILL.md（非技能包）".into());
            };
            on_log(&format!("定位到技能根: {}", root.display()));
            // hermes：不走 openclaw CLI——直接落盘 $HERMES_HOME/skills/<slug>/（hermes 技能
            // 枚举是全树扫盘，落盘即生效；见 hermes.rs 技能装/卸注释）。
            if agent == "hermes" {
                #[cfg(feature = "agent-hermes")]
                {
                    let (ok, msg) = hermes::skill_install(&root, &slug, &name, on_log);
                    let _ = std::fs::remove_dir_all(&tmp);
                    return mk(ok, msg);
                }
                #[cfg(not(feature = "agent-hermes"))]
                {
                    let _ = std::fs::remove_dir_all(&tmp);
                    return mk(false, "hermes 未打包进本构建".into());
                }
            }
            // 下载解压已成功 → install 前先挪走旧版本目录：CLI 对已存在目录的覆盖语义
            // 未定义（文档只说 Git/local 源须重装刷新），挪走重装才幂等；install 失败则
            // 挪回备份，旧版本原样保留。托管目录解析与 skill_uninstall 同源。
            let managed_dir = {
                let (lok, lout) = run_cmd(&openclaw_bin(), &["skills", "list", "--json"]);
                if lok {
                    aiarsenal_client_core::skills::parse_managed_skills_dir(&lout)
                        .unwrap_or_else(|| dirs::home_dir().map(|h| h.join(".openclaw").join("skills")).unwrap_or_default())
                } else {
                    PathBuf::new() // list 都跑不动，install 也多半没戏；不挪旧目录，让 install 自己撞
                }
            };
            let backup = if !managed_dir.as_os_str().is_empty() {
                let target = managed_dir.join(&slug);
                let bak = managed_dir.join(format!("{slug}.aa-installing"));
                let _ = std::fs::remove_dir_all(&bak); // 清上次异常残留
                if target.is_dir() {
                    match std::fs::rename(&target, &bak) {
                        Ok(()) => Some(bak),
                        Err(e) => {
                            let _ = std::fs::remove_dir_all(&tmp);
                            return mk(false, format!("挪走旧版本目录失败: {e}"));
                        }
                    }
                } else {
                    None
                }
            } else {
                None
            };
            let root_str = root.display().to_string();
            let (ok, out) = run_cmd_streaming(
                &openclaw_bin(),
                &["skills", "install", &root_str, "--as", &slug, "--global"],
                on_log,
            );
            let _ = std::fs::remove_dir_all(&tmp);
            if ok {
                if let Some(bak) = &backup {
                    let _ = std::fs::remove_dir_all(bak); // 新版就位，清备份
                }
                mk(true, format!("技能 {name} 已安装（--as {slug} --global）"))
            } else {
                if let Some(bak) = &backup {
                    let _ = std::fs::rename(bak, managed_dir.join(&slug)); // 挪回旧版本
                }
                mk(false, format!("openclaw skills install 失败: {}", out.trim()))
            }
        }
        "skill_uninstall" => {
            // 技能卸载：openclaw 无 uninstall 子命令 → skills list --json 取 managedSkillsDir，
            // 删 <managedSkillsDir>/<slug> 目录（path-watch 自动生效，无需重启）。幂等：不在列表=成功。
            let slug = cmd.payload.get("slug").and_then(|v| v.as_str()).unwrap_or("").to_string();
            on_log(&format!("远程：卸载技能 {slug}"));
            if slug.is_empty() {
                return mk(false, "skill_uninstall 缺少 slug".into());
            }
            // hermes：删 $HERMES_HOME/skills 下按 slug 定位的技能目录（与扫盘同规则）。
            if agent == "hermes" {
                #[cfg(feature = "agent-hermes")]
                {
                    let (ok, msg) = hermes::skill_uninstall(&slug, on_log);
                    return mk(ok, msg);
                }
                #[cfg(not(feature = "agent-hermes"))]
                {
                    return mk(false, "hermes 未打包进本构建".into());
                }
            }
            let (ok, out) = run_cmd(&openclaw_bin(), &["skills", "list", "--json"]);
            if !ok {
                return mk(false, format!("openclaw skills list 失败: {}", out.trim()));
            }
            let managed_dir = match aiarsenal_client_core::skills::parse_managed_skills_dir(&out) {
                Some(d) => d,
                None => {
                    // 输出异常解析不出 → 兜底默认托管目录（list --json 仍给空 JSON 时也能卸）。
                    let Some(home) = dirs::home_dir() else {
                        return mk(false, "无法定位 home 目录".into());
                    };
                    home.join(".openclaw").join("skills")
                }
            };
            let target = managed_dir.join(&slug);
            if !target.is_dir() {
                return mk(true, format!("技能 {slug} 不在托管目录，视为已卸载（幂等）"));
            }
            match std::fs::remove_dir_all(&target) {
                Ok(()) => mk(true, format!("技能 {slug} 已卸载（{}）", target.display())),
                Err(e) => mk(false, format!("删除技能目录失败: {e}")),
            }
        }
        "appperm_reset" => {
            // App 自动化授权重置：tccutil 清掉本 App 全部 AppleEvents 授权对（无按目标
            // 粒度）+ 清引擎状态 + 立即唤醒引擎——清单内所有运行中 App 的授权框重弹。
            on_log("远程：重置 App 自动化授权（tccutil reset AppleEvents，清单内所有授权框将重新弹出）");
            #[cfg(feature = "openclaw-desktop")]
            {
                match desktop_node::app_perm::reset_all() {
                    Ok(n) => mk(true, format!("已重置，清单 {n} 个 App 将重新弹框")),
                    Err(e) => mk(false, format!("重置失败: {e}")),
                }
            }
            #[cfg(not(feature = "openclaw-desktop"))]
            {
                let _ = agent;
                mk(false, "本构建不支持 appperm_reset".into())
            }
        }
        "collect_data" => {
            on_log("远程：采集运行数据");
            let mut arr = serde_json::Map::new();
            for c in compiled_agents() {
                let (inst, run, ver, pid) = get_agent_runtime(c);
                arr.insert(
                    c.to_string(),
                    serde_json::json!({ "installed": inst, "running": run, "version": ver, "pid": pid }),
                );
            }
            ("done".to_string(), serde_json::json!({ "agents": arr }).to_string(), String::new())
        }
        "shell" => {
            // 通用命令执行：payload = {program, args[]}。流式跑，stdout 逐行当日志，
            // 合并 stdout 当 result 回传。后端用它下发各种简单命令（装插件/登出/查状态…），
            // 加新命令无需改 App。
            let program = cmd.payload.get("program").and_then(|v| v.as_str()).unwrap_or("");
            if program.is_empty() {
                return mk(false, "program 不能为空".into());
            }
            let args: Vec<String> = cmd.payload.get("args").and_then(|v| v.as_array())
                .map(|arr| arr.iter().filter_map(|a| a.as_str().map(String::from)).collect())
                .unwrap_or_default();
            let args_ref: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
            on_log(&format!("$ {} {}", program, args.join(" ")));
            aiarsenal_client_core::util::log("info", &format!("[shell] {} {}", program, args.join(" ")));
            let (ok, out) = run_cmd_streaming(program, &args_ref, on_log);
            mk(ok, out)
        }
        "channel_login" => {
            // 渠道扫码登录：openclaw gateway call web.login.start 拿 qrDataUrl，轮询 web.login.wait 至 connected。
            // 进行中状态写 PlatformState.channel_login，心跳读取回传后端（管理后台/设备端双扫码）。
            let channel = cmd.payload.get("channel").and_then(|v| v.as_str()).unwrap_or("").to_string();
            if channel.is_empty() {
                return mk(false, "channel 不能为空".into());
            }
            // hermes 走 Python qr_login shim（非阻塞，抓 stdout 二维码）；openclaw 走 web.login RPC + CLI 降级。
            if agent == "hermes" {
                #[cfg(feature = "agent-hermes")]
                {
                    let (ok, msg) = hermes::channel_login_via_shim(&channel, state, on_log);
                    return mk(ok, msg);
                }
                #[cfg(not(feature = "agent-hermes"))]
                {
                    return mk(false, "hermes 未打包进本构建".to_string());
                }
            }
            on_log(&format!("远程：渠道登录 {channel}"));
            aiarsenal_client_core::util::log("info", &format!("[channel_login] 开始 channel={channel}"));
            set_channel_login_state(state, Some(ChannelLoginState {
                channel: channel.clone(),
                login_status: "qr_pending".into(),
                qr_data_url: String::new(),
                account_alias: String::new(),
                message: "获取二维码中".into(),
            }));
            // 自包含「确保渠道配置已加载」：微信等外部插件渠道的 channels.* 配置只在 gateway 启动时读取
            // （openclaw 文档 channels/wechat.md「Restart the Gateway after install」+ AGENTS.md
            // 「Gateway/plugin metadata is process-stable, changes require restart」）。仅写 openclaw.json 不重启
            // → channels list 仍「not configured」→ channels login 抓不到 QR。故扫码前若渠道未配置：
            // `openclaw config set` 写 channels.<ch> + plugins.entries.<ch>.enabled，再 gateway restart 让插件加载。
            // 已配置则跳过（免每次扫码都重启 gateway）。
            if agent == "openclaw" {
                let (list_rc, list_out) = run_cmd(&openclaw_bin(), &["channels", "list"]);
                let configured = list_rc && list_out.contains(channel.as_str());
                if !configured {
                    let dm = cmd.payload.get("dmPolicy").and_then(|v| v.as_str()).filter(|s| !s.is_empty()).unwrap_or("pairing");
                    let gp = cmd.payload.get("groupPolicy").and_then(|v| v.as_str()).filter(|s| !s.is_empty()).unwrap_or("disabled");
                    on_log(&format!("远程：渠道未配置，写 channels.{channel}（dmPolicy={dm}, groupPolicy={gp}）并重启 gateway 加载"));
                    let _ = run_cmd(&openclaw_bin(), &["config", "set", &format!("channels.{channel}.dmPolicy"), dm]);
                    let _ = run_cmd(&openclaw_bin(), &["config", "set", &format!("channels.{channel}.groupPolicy"), gp]);
                    let _ = run_cmd(&openclaw_bin(), &["config", "set", &format!("plugins.entries.{channel}.enabled"), "true"]);
                    let (rc, out) = run_cmd(&openclaw_bin(), &["gateway", "restart"]);
                    aiarsenal_client_core::util::log("info", &format!("[channel_login] ensure-config gateway restart rc={rc} out={}", out.trim()));
                    if rc {
                        on_log("gateway 已重启，渠道配置已加载");
                    } else {
                        on_log(&format!("gateway restart 未生效（{}），继续尝试登录", out.trim()));
                    }
                } else {
                    aiarsenal_client_core::util::log("info", &format!("[channel_login] 渠道 {channel} 已配置，跳过 ensure-config"));
                }
            }
            // web.login.start（仅 whatsapp/zalo 等实现 web.login RPC 的渠道可用）
            let (rc, out) = run_cmd(&openclaw_bin(), &["gateway", "call", "web.login.start", "--params", "{}", "--json"]);
            aiarsenal_client_core::util::log("info", &format!("[channel_login] web.login.start rc={rc} out={}", out.trim()));
            if !rc {
                // web login provider is not available → 微信等不实现 web.login RPC 的渠道，
                // 降级跑 CLI `channels login`，从 stdout 抓 QR 内容 URL 回传后端。
                if out.contains("provider is not available") || out.contains("web login provider") {
                    #[cfg(feature = "agent-openclaw")]
                    {
                        let (ok, msg) = openclaw::channel_login_via_cli(&channel, state, on_log);
                        return mk(ok, msg);
                    }
                    #[cfg(not(feature = "agent-openclaw"))]
                    {
                        return mk(false, "openclaw 未打包进本构建".to_string());
                    }
                }
                aiarsenal_client_core::util::log("error", &format!("[channel_login] web.login.start 失败 channel={channel}"));
                set_channel_login_state(state, Some(ChannelLoginState {
                    channel: channel.clone(),
                    login_status: "failed".into(),
                    message: format!("web.login.start 失败: {}", out.trim()),
                    ..Default::default()
                }));
                return mk(false, format!("web.login.start 失败: {}", out.trim()));
            }
            let start: serde_json::Value = match serde_json::from_str(&out) {
                Ok(v) => v,
                Err(e) => {
                    set_channel_login_state(state, Some(ChannelLoginState {
                        channel: channel.clone(),
                        login_status: "failed".into(),
                        message: format!("解析 web.login.start 输出失败: {e}"),
                        ..Default::default()
                    }));
                    return mk(false, format!("解析 web.login.start 输出失败: {e}"));
                }
            };
            let connected = start.get("connected").and_then(|v| v.as_bool()).unwrap_or(false);
            if connected {
                set_channel_login_state(state, Some(ChannelLoginState {
                    channel: channel.clone(),
                    login_status: "connected".into(),
                    message: "已连接".into(),
                    ..Default::default()
                }));
                on_log("渠道已连接（无需扫码）");
                let _ = run_cmd(&openclaw_bin(), &["gateway", "restart"]);
                return mk(true, "渠道已连接".into());
            }
            let mut qr = start.get("qrDataUrl").and_then(|v| v.as_str()).unwrap_or("").to_string();
            if qr.is_empty() {
                aiarsenal_client_core::util::log("error", &format!("[channel_login] web.login.start 未返回 qrDataUrl channel={channel}"));
                set_channel_login_state(state, Some(ChannelLoginState {
                    channel: channel.clone(),
                    login_status: "failed".into(),
                    message: "未返回二维码".into(),
                    ..Default::default()
                }));
                return mk(false, "web.login.start 未返回二维码".into());
            }
            on_log("已获取二维码，请用手机扫码（设备端或管理后台均可）");
            aiarsenal_client_core::util::log("info", &format!("[channel_login] 已获取 QR（{}字节），开始轮询 web.login.wait channel={channel}", qr.len()));
            set_channel_login_state(state, Some(ChannelLoginState {
                channel: channel.clone(),
                login_status: "qr_pending".into(),
                qr_data_url: qr.clone(),
                account_alias: String::new(),
                message: "请扫码".into(),
            }));
            // 轮询 web.login.wait（每 3s，最长 ~120s）
            let deadline = std::time::Instant::now() + std::time::Duration::from_secs(120);
            let mut poll_count = 0u32;
            loop {
                if std::time::Instant::now() >= deadline {
                    aiarsenal_client_core::util::log("error", &format!("[channel_login] 扫码超时 channel={channel}"));
                    set_channel_login_state(state, Some(ChannelLoginState {
                        channel: channel.clone(),
                        login_status: "failed".into(),
                        message: "扫码超时".into(),
                        ..Default::default()
                    }));
                    return mk(false, "扫码超时".into());
                }
                std::thread::sleep(std::time::Duration::from_secs(3));
                poll_count += 1;
                let params = format!("{{\"currentQrDataUrl\":\"{}\"}}", qr.replace('\\', "\\\\").replace('"', "\\\""));
                let (wrc, wout) = run_cmd(&openclaw_bin(), &["gateway", "call", "web.login.wait", "--params", &params, "--json"]);
                if !wrc {
                    aiarsenal_client_core::util::log("warn", &format!("[channel_login] web.login.wait 第{poll_count}次失败 rc=false out={}", wout.trim()));
                    // 网关暂时不可达：继续轮询
                    continue;
                }
                let wv: serde_json::Value = match serde_json::from_str(&wout) {
                    Ok(v) => v,
                    Err(_) => continue,
                };
                let w_connected = wv.get("connected").and_then(|v| v.as_bool()).unwrap_or(false);
                aiarsenal_client_core::util::log("info", &format!("[channel_login] web.login.wait 第{poll_count}次 connected={w_connected}"));
                if let Some(new_qr) = wv.get("qrDataUrl").and_then(|v| v.as_str()) {
                    if new_qr != qr {
                        qr = new_qr.to_string();
                        set_channel_login_state(state, Some(ChannelLoginState {
                            channel: channel.clone(),
                            login_status: "qr_pending".into(),
                            qr_data_url: qr.clone(),
                            message: "二维码已刷新，请扫码".into(),
                            ..Default::default()
                        }));
                        on_log("二维码已刷新");
                        aiarsenal_client_core::util::log("info", &format!("[channel_login] QR 已刷新（{}字节）channel={channel}", qr.len()));
                    }
                }
                if w_connected {
                    set_channel_login_state(state, Some(ChannelLoginState {
                        channel: channel.clone(),
                        login_status: "connected".into(),
                        message: "登录成功".into(),
                        ..Default::default()
                    }));
                    on_log("扫码登录成功");
                    aiarsenal_client_core::util::log("info", &format!("[channel_login] 成功 channel={channel}（轮询{poll_count}次）"));
                    // 登录成功后重启网关使渠道 listener 生效
                    let (rrc, rout) = run_cmd(&openclaw_bin(), &["gateway", "restart"]);
                    aiarsenal_client_core::util::log("info", &format!("[channel_login] gateway restart rc={rrc} out={}", rout.trim()));
                    return mk(true, "渠道扫码登录成功".into());
                }
            }
        }
        "channel_logout" => {
            // 渠道解绑：hermes 清 .env WEIXIN_* + 重生成 config（去 weixin 块）+ restart；
            // openclaw 由后端走 shell 跑 channels logout，不经此 action。
            on_log(&format!("远程：渠道解绑 {agent}"));
            if agent == "hermes" {
                #[cfg(feature = "agent-hermes")]
                {
                    let (ok, msg) = hermes::channel_logout_hermes(state);
                    return mk(ok, msg);
                }
                #[cfg(not(feature = "agent-hermes"))]
                {
                    return mk(false, "hermes 未打包进本构建".to_string());
                }
            }
            mk(false, format!("channel_logout 仅 hermes 支持，收到 agent={agent}"))
        }
        other => ("failed".to_string(), "{}".to_string(), format!("未知 action: {other}")),
    }
}

pub fn save_agent_config(agent: &str, content: &str) -> (bool, String, String, String) {
    let path = config_path(agent);
    if let Some(p) = &path {
        if let Some(parent) = p.parent() {
            let _ = std::fs::create_dir_all(parent);
        }
        match std::fs::write(p, content) {
            Ok(_) => return (true, content.to_string(), p.display().to_string(), "已保存".into()),
            Err(e) => return (false, String::new(), p.display().to_string(), format!("写入失败: {e}")),
        }
    }
    (false, String::new(), String::new(), "无法确定配置路径".into())
}

pub fn read_agent_config(agent: &str) -> (bool, String, String, String) {
    let path = config_path(agent);
    if let Some(p) = &path {
        if !p.exists() {
            return (true, default_config(agent), p.display().to_string(), "使用默认模板".into());
        }
        match std::fs::read_to_string(p) {
            Ok(c) => return (true, c, p.display().to_string(), "已读取".into()),
            Err(e) => return (false, String::new(), p.display().to_string(), format!("读取失败: {e}")),
        }
    }
    (false, String::new(), String::new(), "无法确定配置路径".into())
}

/// 只捕获 stdout（不含 stderr），用于取版本号等「正常输出」场景。
fn cmd_stdout(program: &str, args: &[&str]) -> String {
    let mut cmd = Command::new(program);
    cmd.args(args).env("PATH", mac_path());
    match cmd.output() {
        Ok(out) => String::from_utf8_lossy(&out.stdout).trim().to_string(),
        Err(_) => String::new(),
    }
}

// ==================== 通用工具 ====================

fn mac_path() -> String {
    // GUI 应用 PATH 缺 homebrew；显式补上常见路径
    let existing = std::env::var("PATH").unwrap_or_default();
    format!("/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:{}", existing)
}

/// openclaw CLI 绝对路径：install-cli.sh 把 CLI 装到 `~/.openclaw/bin/openclaw`（稳定路径，
/// feature 启用=client 跑 install-cli.sh 装；禁用=OpenClaw.app 内部也跑 install-cli.sh 装同路径）。
/// 不存在则 fallback "openclaw"（PATH，兼容未装的过渡态）。
fn openclaw_bin() -> String {
    if let Some(home) = dirs::home_dir() {
        let p = home.join(".openclaw").join("bin").join("openclaw");
        if p.is_file() {
            return p.display().to_string();
        }
    }
    "openclaw".to_string()
}

fn which(prog: &str) -> bool {
    // 原先 shell out `which` 子进程在 GUI + 高频心跳负载下偶发 fork/exec 失败
    // → installed 误判 false → 状态在 running/notinstalled 间抖动。
    // 改 Rust 原生 PATH 检索，确定性强、零子进程。
    let existing = std::env::var("PATH").unwrap_or_default();
    let full = format!("/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:{}", existing);
    for dir in full.split(':') {
        if dir.is_empty() {
            continue;
        }
        if PathBuf::from(dir).join(prog).is_file() {
            return true;
        }
    }
    false
}

fn run_cmd(program: &str, args: &[&str]) -> (bool, String) {
    let mut cmd = Command::new(program);
    cmd.args(args).env("PATH", mac_path());
    match cmd.output() {
        Ok(out) => {
            let mut s = String::new();
            if !out.stdout.is_empty() {
                s.push_str(&String::from_utf8_lossy(&out.stdout));
            }
            if !out.stderr.is_empty() {
                s.push_str(&String::from_utf8_lossy(&out.stderr));
            }
            (out.status.success(), s.trim().to_string())
        }
        Err(e) => (false, format!("{e}")),
    }
}

/// 流式执行：逐行读 stdout/stderr，每行 `on_log` 回调出去，最后返回合并文本+退出状态。
/// stdout/stderr 在两个 scoped 线程并行读，避免管道写满死锁。
fn run_cmd_streaming(
    program: &str,
    args: &[&str],
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    use std::io::{BufRead, BufReader};
    let mut cmd = Command::new(program);
    cmd.args(args).env("PATH", mac_path());
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            let m = format!("{e}");
            on_log(&m);
            return (false, m);
        }
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let mut out = String::new();
    std::thread::scope(|s| {
        // stderr 单独线程读，避免管道满；行通过 on_log 回调
        s.spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                on_log(&line);
            }
        });
        for line in BufReader::new(stdout).lines().flatten() {
            on_log(&line);
            out.push_str(&line);
            out.push('\n');
        }
    });
    let ok = child.wait().map(|s| s.success()).unwrap_or(false);
    (ok, out.trim().to_string())
}

/// run_cmd_streaming 的带额外环境变量版：hermes 安装注入 spec.envVars（如 UV_DEFAULT_INDEX
/// 走国内 PyPI 镜像）+ HERMES_HOME。envs 在 mac_path 之后设置，可覆盖继承值。
fn run_cmd_streaming_env(
    program: &str,
    args: &[&str],
    envs: &[(&str, &str)],
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    use std::io::{BufRead, BufReader};
    let mut cmd = Command::new(program);
    cmd.args(args).env("PATH", mac_path());
    for (k, v) in envs {
        cmd.env(k, v);
    }
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            let m = format!("{e}");
            on_log(&m);
            return (false, m);
        }
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let mut out = String::new();
    std::thread::scope(|s| {
        s.spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                on_log(&line);
            }
        });
        for line in BufReader::new(stdout).lines().flatten() {
            on_log(&line);
            out.push_str(&line);
            out.push('\n');
        }
    });
    let ok = child.wait().map(|s| s.success()).unwrap_or(false);
    (ok, out.trim().to_string())
}

/// run_cmd_streaming 的带 cwd 版（+ 注入 mac_path）：跑 uv pip install 等需指定工作目录的命令。
fn run_cmd_streaming_dir(
    program: &str,
    args: &[&str],
    cwd: &PathBuf,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    use std::io::{BufRead, BufReader};
    let mut cmd = Command::new(program);
    cmd.args(args).current_dir(cwd).env("PATH", mac_path());
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            let m = format!("{e}");
            on_log(&m);
            return (false, m);
        }
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let mut out = String::new();
    std::thread::scope(|s| {
        s.spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                on_log(&line);
            }
        });
        for line in BufReader::new(stdout).lines().flatten() {
            on_log(&line);
            out.push_str(&line);
            out.push('\n');
        }
    });
    let ok = child.wait().map(|s| s.success()).unwrap_or(false);
    (ok, out.trim().to_string())
}

/// 用 osascript 以管理员权限执行 shell 命令（弹系统授权框 = 用户授权）。
fn run_osascript_admin(shell_cmd: &str) -> (bool, String) {
    // 转义双引号
    let escaped = shell_cmd.replace('\\', "\\\\").replace('"', "\\\"");
    let script = format!("do shell script \"{}\" with administrator privileges", escaped);
    match Command::new("osascript").args(["-e", &script]).output() {
        Ok(out) => {
            let s = String::from_utf8_lossy(&out.stderr).to_string();
            (out.status.success(), s)
        }
        Err(e) => (false, format!("{e}")),
    }
}

fn download_file(url: &str, dest: &PathBuf, token: Option<&str>) -> (bool, String) {
    // curl -fL（跟随重定向、失败退出码非 0）；带 Bearer 时加 -H。
    // 整条 shell 命令嵌在 AppleScript 字符串字面量里：-H 的值必须用 shell 单引号包
    //（内层裸双引号会让 AppleScript 在 -H " 处提前收串 → syntax error -2740，
    // 实踩：skill_install 带 token 首次走此路径），再对整条命令做 \ 与 " 的 AppleScript 转义兜底。
    let header = match token {
        Some(t) if !t.is_empty() => format!(" -H 'Authorization: Bearer {t}'"),
        _ => String::new(),
    };
    let shell_cmd = format!("curl -fL{header} -o '{}' '{}'", dest.display(), url);
    let escaped = shell_cmd.replace('\\', "\\\\").replace('"', "\\\"");
    let script = format!("do shell script \"{escaped}\"");
    match Command::new("osascript").args(["-e", &script]).output() {
        Ok(out) => {
            let s = String::from_utf8_lossy(&out.stderr).to_string();
            (out.status.success(), s)
        }
        Err(e) => (false, format!("{e}")),
    }
}

fn config_path(agent: &str) -> Option<PathBuf> {
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => dirs::home_dir().map(|h| h.join(".openclaw").join("openclaw.json")),
        #[cfg(feature = "agent-hermes")]
        "hermes" => Some(hermes::hermes_home().join("config.yaml")),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => Some(app_support_dir().join("obsidian").join("config.json")),
        // client 自有配置 aa.json：心跳捎带全文，服务端 hash 变化落「配置变更」留痕
        //（App 权限清单下发 + 引擎写 granted/denied 状态都会体现在这里）。
        "client" => Some(app_support_dir().join("aa.json")),
        _ => None,
    }
}

/// 取客户端配置文件信息：(hash, size, mtime_empty, content)。心跳上报用，hash 变化触发后端落库。
pub fn config_info(agent: &str) -> Option<(String, i64, String, String)> {
    let path = config_path(agent)?;
    if !path.exists() {
        return None;
    }
    let content = std::fs::read_to_string(&path).ok()?;
    let size = content.len() as i64;
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    let mut h = DefaultHasher::new();
    content.hash(&mut h);
    Some((format!("{:x}", h.finish()), size, String::new(), content))
}

fn app_support_dir() -> PathBuf {
    dirs::data_dir()
        .map(|d| d.join("aiarsenal-client"))
        .unwrap_or_else(|| PathBuf::from("."))
}

fn default_config(agent: &str) -> String {
    match agent {
        "openclaw" => r#"{
  "apiKey": "",
  "baseUrl": "",
  "channels": []
}
"#.to_string(),
        "hermes" => r#"{ "enabled": true, "skills": [] }
"#.to_string(),
        "obsidian" => r#"{ "vaultPath": "" }
"#.to_string(),
        _ => "{}\n".to_string(),
    }
}
