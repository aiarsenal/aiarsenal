//! Windows 平台实现：openclaw / hermes / obsidian 的安装、启停、配置。
//!
//! - openclaw：npm 全局包（Node CLI），`openclaw gateway run` 子进程，配置 `%USERPROFILE%\.openclaw\openclaw.json`
//! - hermes：原生 Python（公网包安装，无 Docker），`hermes gateway run` 子进程
//! - obsidian：官方 .exe 静默安装（/S），`Obsidian.exe` 进程启停
//!
//! 各 agent 的实现拆到子模块（openclaw/hermes/obsidian），本文件只留共享 dispatch /
//! execute_command / 配置 / 工具。平台命令接收前端传入的 `spec`，不在本层做 HTTP。

#[cfg(feature = "agent-hermes")]
mod hermes;
#[cfg(feature = "agent-obsidian")]
mod obsidian;
#[cfg(feature = "agent-openclaw")]
mod openclaw;
// openclaw desktop node（client 自当 Windows node，替代 OpenClaw.app 的角色）：feature 禁用时
// 整个模块不编译（可卸载）。pub：lib.rs 的权限清单页 / exec 审批 / canvas 命令直接转发。
#[cfg(feature = "openclaw-desktop")]
pub mod desktop_node;

use std::collections::HashMap;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Condvar, Mutex};

use aiarsenal_client_core::ChannelLoginState;

/// 版本对齐确认门的共享态：install 线程发起（pending=true）→ 前端弹窗 →
/// client_confirm_openclaw_align 回填 response → condvar 唤醒等待方。
#[derive(Default)]
struct AlignConfirmState {
    pending: bool,
    response: Option<bool>,
}

/// 本次构建编译进二进制的 agent 列表（按 cargo feature 决定）。心跳上报 / collect_data /
/// 前端 agent 列表都用它，未打包的 agent 不上报、命令落 dispatch fallback。
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
/// openclaw：~/.openclaw/skills 平铺扫描（slug=一级目录）；hermes：$HERMES_HOME\skills
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

/// 确保 client desktop node 已启动（幂等：已启动则跳过）。
/// feature openclaw-desktop：client 自实现 desktop node 给 agent 提供截屏/控鼠键能力。
/// 心跳登录后调一次，让它跟着 client 自动起（不依赖后端下发 start 命令）；
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
        let out = match cmd_new(&bin)
            .args(["nodes", "pending", "--json"])
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
    let out = match cmd_new(&bin)
        .args(["config", "get", "gateway.nodes.allowCommands"])
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

/// 确保主窗口可见（后台线程要弹审批/确认框时用）。已有（含隐藏到托盘）→ show + focus。
/// Windows 的 tauri.conf.json 预建了 "main" 窗口，这里只是唤起；万一没有则按 mac 同规格补建
/// （1040×720 居中），保证审批弹窗始终有落点。
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

/// 长驻子进程注册表（openclaw 子进程句柄）。hermes/obsidian 由系统服务/进程名管理。
/// `Arc` 使其可 clone，便于在 `#[tauri::command] async` / 心跳任务间共享。
#[derive(Clone)]
pub struct PlatformState {
    procs: Arc<Mutex<HashMap<String, Child>>>,
    /// 渠道登录进行中状态（channel_login 命令写，心跳读回传后端）。None=无登录在进行。
    channel_login: Arc<Mutex<Option<ChannelLoginState>>>,
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

    /// 请求本机用户确认 openclaw 版本对齐：emit `openclaw-align-confirm` + 拉起主窗口，
    /// 然后阻塞等待用户回应。等待期间每 30s 走一次 on_log 刷新命令进度（防服务端 idle 超时）。
    /// 返回 Some(true)=确认 / Some(false)=拒绝 / None=超时（10 分钟）。
    pub fn request_align_confirm(
        &self,
        payload: &serde_json::Value,
        on_log: &(dyn Fn(&str) + Send + Sync),
    ) -> Option<bool> {
        use tauri::{Emitter, Manager};
        if let Some(app) = self.app.lock().unwrap().clone() {
            if let Some(w) = app.get_webview_window("main") {
                let _ = w.show();
                let _ = w.set_focus();
            }
            let _ = app.emit("openclaw-align-confirm", payload);
        }
        let (m, cv) = &*self.align;
        let mut g = m.lock().unwrap();
        g.pending = true;
        g.response = None;
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(600);
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
            let slice = std::time::Duration::from_secs(30).min(deadline - now);
            on_log("等待本机用户确认版本对齐（确认框已弹出，10 分钟超时）…");
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

// Tauri command 已把 spec 拆为参数，下面提供按 agent 分派的实现。
pub fn install_with_spec(
    agent: &str,
    spec: &serde_json::Value,
    state: &PlatformState,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> OpOut {
    match agent {
        #[cfg(feature = "agent-openclaw")]
        "openclaw" => openclaw::install_openclaw(state, spec, on_log),
        #[cfg(feature = "agent-hermes")]
        "hermes" => hermes::install_hermes(spec, on_log),
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => obsidian::install_obsidian(spec, on_log),
        _ => (false, format!("未知或未打包的客户端: {agent}"), String::new()),
    }
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
            // 先停进程再卸包：npm uninstall 只删包文件，不会动已经在跑的 openclaw
            // （前台 `gateway run` 子进程 / schtasks 服务都会残留）。官方 `gateway stop`
            // 停 schtasks 服务并排空退出；再兜底杀本 App 启的子进程 + taskkill。
            on_log("停止 openclaw 服务（gateway stop）");
            let _ = run_cmd(&openclaw_bin(), &["gateway", "stop"]); // 服务未安装时非 0，忽略
            on_log("停止 openclaw 进程");
            let _ = openclaw::stop_openclaw(state);
            on_log("$ npm uninstall -g openclaw");
            match run_cmd("npm", &["uninstall", "-g", "openclaw"]) {
                // 只卸 npm 包，~/.openclaw（配置/identity/渠道登录态）保留给用户，不销毁
                (true, _) => (true, "已卸载 openclaw（npm 包；~/.openclaw 用户数据保留）".into()),
                (false, e) => (false, format!("卸载 openclaw 失败: {e}")),
            }
        }
        #[cfg(feature = "agent-hermes")]
        "hermes" => {
            on_log("卸载 hermes（删除安装目录）");
            let _ = std::fs::remove_dir_all(hermes::hermes_home().join("hermes-agent"));
            (true, "已卸载 hermes".into())
        }
        #[cfg(feature = "agent-obsidian")]
        "obsidian" => (false, "obsidian 请通过系统设置卸载".into()),
        _ => (false, format!("未知或未打包的客户端: {agent}")),
    }
}

pub fn get_agent_runtime(agent: &str) -> (bool, bool, String, String) {
    // (installed, running, version, pid_or_container_id)
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
            // hermes：下发内容落 config.base.yaml，write_effective_config 按微信绑定态生成实际 config.yaml。
            on_log("远程：更新 hermes 配置");
            #[cfg(feature = "agent-hermes")]
            {
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
            // 技能安装：下载整包 zip（设备 token）→ PowerShell 解压 → 定位 SKILL.md 根 → 按 agent 落地：
            // - openclaw：openclaw skills install --as slug --global（装进全局托管 ~/.openclaw/skills）
            // - hermes：直接落盘 $HERMES_HOME\skills\<slug>\（hermes 枚举是全树扫盘，见 hermes.rs）
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
            let script = format!(
                "Expand-Archive -Path '{}' -DestinationPath '{}' -Force",
                zip.display(),
                extract.display()
            );
            let (ok, msg) = run_cmd("powershell", &["-NoProfile", "-Command", &script]);
            if !ok {
                let _ = std::fs::remove_dir_all(&tmp);
                return mk(false, format!("解压失败: {msg}"));
            }
            let Some(root) = aiarsenal_client_core::skills::locate_skill_root(&extract) else {
                let _ = std::fs::remove_dir_all(&tmp);
                return mk(false, "包内未找到 SKILL.md（非技能包）".into());
            };
            on_log(&format!("定位到技能根: {}", root.display()));
            // hermes：不走 openclaw CLI——直接落盘 $HERMES_HOME\skills\<slug>\（hermes 技能
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
            // hermes：删 $HERMES_HOME\skills 下按 slug 定位的技能目录（与扫盘同规则）。
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
        "appperm_reset" => {
            // App 控制清单重置：清引擎状态 + 重新下发（Windows 无 TCC，授权态引擎本地持有——
            // 重置后 granted/denied 清空，下一次心跳按清单原样上报回管理端）。
            on_log("远程：重置 App 控制清单状态");
            #[cfg(feature = "openclaw-desktop")]
            {
                match desktop_node::app_perm::reset_all() {
                    Ok(n) => mk(true, format!("已重置，清单 {n} 个 App 状态将重新上报")),
                    Err(e) => mk(false, format!("重置失败: {e}")),
                }
            }
            #[cfg(not(feature = "openclaw-desktop"))]
            {
                let _ = agent;
                mk(false, "本构建不支持 appperm_reset".into())
            }
        }
        other => ("failed".to_string(), "{}".to_string(), format!("未知 action: {other}")),
    }
}

pub fn save_agent_config(agent: &str, content: &str) -> (bool, String, String, String) {
    // (ok, content, path, msg)
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
    match cmd_new(program).args(args).output() {
        Ok(out) => String::from_utf8_lossy(&out.stdout).trim().to_string(),
        Err(_) => String::new(),
    }
}

/// Rust 原生 PATH 检索（零子进程）：原先用 `openclaw --version` 退出码判 installed，
/// npm 包损坏时 --version 报错 → exit 非 0 → installed 误判 false，状态在 running/
/// notinstalled 间抖。改直接查 PATH 里的可执行文件。
fn which(prog: &str) -> bool {
    let existing = std::env::var("PATH").unwrap_or_default();
    for dir in existing.split(';') {
        if dir.is_empty() {
            continue;
        }
        for ext in ["", ".exe", ".cmd", ".bat", ".ps1"] {
            if PathBuf::from(dir).join(format!("{prog}{ext}")).is_file() {
                return true;
            }
        }
    }
    // npm 全局：%APPDATA%\npm（GUI 应用 PATH 常不含它）
    if let Some(appdata) = std::env::var("APPDATA").ok() {
        let npm = PathBuf::from(appdata).join("npm");
        for ext in ["", ".cmd", ".ps1"] {
            if npm.join(format!("{prog}{ext}")).is_file() {
                return true;
            }
        }
    }
    false
}

/// openclaw CLI 绝对路径：生产模式下 Windows Hub 把 CLI 装到 ~/.openclaw/bin/openclaw.exe
///（待 openclaw-windows-node 仓库确认；若 gateway 在 WSL 内则 win 调用方式需调整为 wsl 包装）。
/// 不存在则 fallback "openclaw"（PATH，兼容 npm 存量/未装 Windows Hub 的过渡态——win 的
/// install/start/stop 切 Windows Hub 待 WSL 细节确认后单独做）。
fn openclaw_bin() -> String {
    if let Some(home) = dirs::home_dir() {
        let p = home.join(".openclaw").join("bin").join("openclaw.exe");
        if p.is_file() {
            return p.display().to_string();
        }
    }
    "openclaw".to_string()
}

// ==================== 通用工具 ====================

/// 统一子进程入口：App 是 windows GUI 子系统（无控制台），Windows 给每个控制台子进程
/// （hermes.exe / PowerShell / cmd…）新分配一个控制台 → 心跳每 tick 的版本探测/进程扫描
/// 会连弹黑窗（实测 2026-09-27：hermes.exe --version 是 Python shim，一闪 2~5s 极显眼）。
/// CREATE_NO_WINDOW 让子进程不带控制台跑（stdio 重定向不受影响，gateway/CLI 均无交互需求）。
fn cmd_new(program: &str) -> Command {
    // 非 Windows 目标（宿主 macOS 上 `cargo check` 的交叉类型校验）没有 std::os::windows，
    // cfg 隔离让宿主 check 也能编译通过（行为只在真实 Windows 目标生效）。
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        let mut c = Command::new(program);
        c.creation_flags(CREATE_NO_WINDOW);
        c
    }
    #[cfg(not(target_os = "windows"))]
    {
        Command::new(program)
    }
}

fn run_cmd(program: &str, args: &[&str]) -> (bool, String) {
    match cmd_new(program)
        .args(args)
        .output()
    {
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
/// stdout/stderr 在两个 scoped 线程并行读，避免管道写满死锁。供通用 shell action 用。
fn run_cmd_streaming(
    program: &str,
    args: &[&str],
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    use std::io::{BufRead, BufReader};
    let mut cmd = cmd_new(program);
    cmd.args(args);
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

/// run_cmd_streaming 的带 cwd 版：跑 uv pip install 等需指定工作目录的命令。
fn run_cmd_streaming_dir(
    program: &str,
    args: &[&str],
    cwd: &PathBuf,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    use std::io::{BufRead, BufReader};
    let mut cmd = cmd_new(program);
    cmd.args(args).current_dir(cwd);
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

fn download_file(url: &str, dest: &PathBuf, token: Option<&str>) -> (bool, String) {
    // 用 PowerShell 下载，避免引入额外依赖；带 Bearer 时加 -Headers
    let headers = match token {
        Some(t) if !t.is_empty() => format!(" -Headers @{{ Authorization = \"Bearer {t}\" }}"),
        _ => String::new(),
    };
    let script = format!(
        "Invoke-WebRequest -Uri '{}' -OutFile '{}' -UseBasicParsing{headers}",
        url,
        dest.display()
    );
    run_cmd("powershell", &["-NoProfile", "-Command", &script])
}

fn config_path(agent: &str) -> Option<PathBuf> {
    match agent {
        "openclaw" => dirs::home_dir().map(|h| h.join(".openclaw").join("openclaw.json")),
        #[cfg(feature = "agent-hermes")]
        "hermes" => Some(hermes::hermes_home().join("config.yaml")),
        "obsidian" => Some(app_data_dir().join("obsidian").join("config.json")),
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

fn app_data_dir() -> PathBuf {
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

// ==================== 防锁屏 / 防睡眠 ====================
//
// openclaw / hermes 要持续操作电脑，系统空闲锁屏/睡眠会中断操作。
// 通过 SetThreadExecutionState 告知电源管理器：本进程运行期间显示器与系统需保持唤醒。
// ES_DISPLAY_REQUIRED 在保持显示器唤醒的同时持续重置空闲计时器 → 一并阻止屏保/锁屏。
// assertion 绑定到线程/进程生命周期，App 退出时自动释放，无需手动清理。

static KEEP_AWAKE: AtomicBool = AtomicBool::new(false);

mod keep_awake_ffi {
    // SetThreadExecutionState 导出自 kernel32.dll（Windows 目标默认链接），WINAPI = extern "system"。
    const ES_CONTINUOUS: u32 = 0x80000000;
    const ES_SYSTEM_REQUIRED: u32 = 0x00000001;
    const ES_DISPLAY_REQUIRED: u32 = 0x00000002;
    extern "system" {
        fn SetThreadExecutionState(es_flags: u32) -> u32;
    }

    /// 开启防锁屏。返回 0 视为失败（调用线程无权限等罕见情况）。
    pub fn enable() -> bool {
        unsafe {
            SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED) != 0
        }
    }

    /// 关闭防锁屏（恢复系统默认空闲策略）。
    pub fn disable() {
        unsafe { SetThreadExecutionState(ES_CONTINUOUS); }
    }
}

/// 开启防锁屏。返回是否成功。
pub fn keep_awake_enable() -> bool {
    let ok = keep_awake_ffi::enable();
    KEEP_AWAKE.store(ok, Ordering::SeqCst);
    ok
}

/// 关闭防锁屏。
pub fn keep_awake_disable() {
    keep_awake_ffi::disable();
    KEEP_AWAKE.store(false, Ordering::SeqCst);
}

/// 防锁屏当前是否生效。
pub fn keep_awake_is_on() -> bool {
    KEEP_AWAKE.load(Ordering::SeqCst)
}
