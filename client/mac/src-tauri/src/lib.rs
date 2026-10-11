//! AI Arsenal 桌面客户端（macOS）应用入口。
//!
//! 共享命令（登录中转 / 客户端规格 / 上报 / 环境切换）是对 `aiarsenal-client-core::AppState`
//! 的薄包装；平台命令（安装/启停 openclaw / hermes / obsidian）在 `platform.rs`。

pub mod platform;

use std::collections::{HashMap, VecDeque};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use aiarsenal_client_core::{AppState, EnvInfo, ExchangeRes, HeartbeatReq, LogEntry, ModuleReport, RunningCommandReport};
use serde::Serialize;
use tauri::{
    menu::{CheckMenuItem, Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    menu::Submenu,
    AppHandle, Emitter, Manager,
};

const DEFAULT_ENV: &str = "test";

#[derive(Debug, Serialize)]
pub struct AgentOpResult {
    pub success: bool,
    pub message: String,
    #[serde(rename = "pidOrContainerId")]
    pub pid_or_container_id: String,
}

#[derive(Debug, Serialize)]
pub struct AgentRuntime {
    pub agent: String,
    pub installed: bool,
    pub running: bool,
    pub version: String,
    #[serde(rename = "pidOrContainerId")]
    pub pid_or_container_id: String,
}

#[derive(Debug, Serialize, Clone)]
pub struct ConfigResult {
    pub success: bool,
    pub content: String,
    pub path: String,
    pub message: String,
}

/// 安装进度日志事件载荷（`install-log` 事件）。
#[derive(Debug, Serialize, Clone)]
pub struct InstallLog {
    pub agent: String,
    pub line: String,
}

/// 当前执行中命令的共享状态（心跳 loop 与执行器之间传递进度/结果）。
#[derive(Clone, Default)]
struct CurrentCmd {
    command_id: String,
    agent: String,
    action: String, // install/uninstall/start/stop/restart/config_push
    status: String, // running/done/failed
    progress: String,
    percent: i32,
    last_line: String,
    result: String,
    error: String,
    /// 终态（done/failed）是否已随心跳上报一次。对齐 manager 的 cmdState.reported：
    /// 保证结果至少上报一次后，下一轮循环顶部 clearReported 才会清掉 current，避免快命令
    /// done 后被直接清掉、服务端收不到结果只能超时标 failed。build 挂心跳时置 true。
    reported: bool,
}

/// 托盘菜单项手柄：靠 set_text / set_checked 原地更新，不重建菜单。
/// `auth` 文字在「登入」↔「登出」间变形（互斥，同一格只显示其一）。
/// `perm_items`（仅 openclaw-desktop feature）：「权限」子菜单 9 项句柄（顺序 = PERM_TOGGLES），
/// 点击切换开/关（granted→tccutil 收回+本地记住 / 已关闭→清标志+申请 / 未申请→弹框），
/// 后台线程执行后回主线程刷新文字。
struct TrayRefs {
    auth: MenuItem<tauri::Wry>,
    ka: CheckMenuItem<tauri::Wry>, // 防止锁屏：勾选态 = keep_awake_is_on
    #[cfg(feature = "openclaw-desktop")]
    perm_items: Vec<MenuItem<tauri::Wry>>,
}

/// 登录轮询防重入标记：同一时刻只跑一个 begin_login 后台轮询。
static LOGIN_IN_PROGRESS: AtomicBool = AtomicBool::new(false);

// ==================== 共享命令（薄包装） ====================

#[tauri::command]
async fn client_start_login(state: tauri::State<'_, AppState>) -> Result<String, String> {
    let dc = state.start_login();
    aiarsenal_client_core::util::log(
        "info",
        &format!("start_login: 生成 deviceCode={}", dc),
    );
    Ok(dc)
}

#[tauri::command]
async fn client_exchange(
    state: tauri::State<'_, AppState>,
    app: tauri::AppHandle,
    device_code: String,
) -> Result<ExchangeRes, String> {
    aiarsenal_client_core::util::log(
        "info",
        &format!("exchange: 轮询 deviceCode={}", device_code),
    );
    let res = state
        .exchange(&device_code)
        .await
        .map_err(|e| {
            aiarsenal_client_core::util::log("error", &format!("exchange 失败: {e}"));
            e.to_string()
        })?;
    aiarsenal_client_core::util::log(
        "info",
        &format!("exchange: status={}, has_token={}", res.status, !res.token.is_empty()),
    );
    // 授权成功 → 隐藏到托盘（后台继续心跳）
    if res.status == "ok" && !res.token.is_empty() {
        aiarsenal_client_core::util::log("info", "授权成功，隐藏主窗口到托盘");
        if let Some(w) = app.get_webview_window("main") {
            let _ = w.hide();
        }
    }
    Ok(res)
}

/// 手动隐藏主窗口到托盘（Dashboard「隐藏到托盘」按钮用）。
#[tauri::command]
fn client_hide_window(app: tauri::AppHandle) -> Result<(), String> {
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.hide();
    }
    Ok(())
}

#[tauri::command]
async fn client_get_agents(state: tauri::State<'_, AppState>) -> Result<serde_json::Value, String> {
    state
        .get_agents()
        .await
        .map(|c| serde_json::to_value(c).unwrap_or(serde_json::Value::Null))
        .map_err(|e| e.to_string())
}

// ===== MCP 分组自助管理（MCP 管理弹窗；权限/配置重建都在服务端，端上只转发） =====

#[tauri::command]
async fn client_mcp_list(state: tauri::State<'_, AppState>) -> Result<serde_json::Value, String> {
    state
        .mcp_list()
        .await
        .map(|v| serde_json::to_value(v).unwrap_or(serde_json::Value::Null))
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn client_mcp_bind(
    state: tauri::State<'_, AppState>,
    group_id: String,
) -> Result<(), String> {
    state.mcp_bind(&group_id).await.map_err(|e| e.to_string())
}

#[tauri::command]
async fn client_mcp_unbind(
    state: tauri::State<'_, AppState>,
    group_id: String,
) -> Result<(), String> {
    state.mcp_unbind(&group_id).await.map_err(|e| e.to_string())
}

#[tauri::command]
fn client_get_env(state: tauri::State<'_, AppState>) -> Result<EnvInfo, String> {
    Ok(state.env_info())
}

#[tauri::command]
fn client_logout(state: tauri::State<'_, AppState>) -> Result<(), String> {
    state.logout();
    Ok(())
}

/// 打开系统浏览器到桌面登录页，App 端随后轮询 exchange。
#[tauri::command]
async fn open_login_page(state: tauri::State<'_, AppState>, device_code: String) -> Result<(), String> {
    let env_info = state.env_info();
    // 桌面登录页挂在 UI 站点（aiarsenal.cc），不是 API host（api.aiarsenal.cc 是纯 JSON 代理）
    let ui_url = aiarsenal_client_core::derive_ui_url(&env_info.base_url);
    // vben 用 hash 路由（VITE_ROUTER_HISTORY=hash），必须用 /#/client-login 才能命中路由；
    // 写成 /client-login 会被 hash 路由当成空 hash → 重定向到 #/landing（落地页），看不到登录表单。
    let url = format!("{}/#/client-login?dc={}", ui_url, device_code);
    aiarsenal_client_core::util::log("info", &format!("open_login_page: 打开浏览器 {}", url));
    open::that(&url).map_err(|e| {
        aiarsenal_client_core::util::log("error", &format!("打开浏览器失败: {e}"));
        format!("打开浏览器失败: {}", e)
    })
}

// ==================== 平台命令（macOS） ====================
//
// 安装/启停涉及 docker pull / hdiutil / osascript 等可能数十秒的阻塞调用，
// 同步 `#[tauri::command] fn` 会跑在主线程冻住窗口（按钮点了没反应、消息弹不出）。
// 这里全部用 `async fn` + `spawn_blocking` 把阻塞调用丢到后台线程池，
// 主线程保持响应，前端能立刻看到 loading/消息。

#[tauri::command]
async fn install_agent(
    agent: String,
    mut spec: serde_json::Value,
    app: tauri::AppHandle,
    state: tauri::State<'_, AppState>,
    plat: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    // 注入本地安装包下载信息：若该版本有当前平台的缓存包，App 从本服务下载（不直连官网）
    let token = state.token();
    let base = state.base_url();
    if let Some(obj) = spec.as_object_mut() {
        let platform = if cfg!(target_os = "macos") { "mac" } else { "win" };
        let has_pkg = obj
            .get("packages")
            .and_then(|p| p.as_array())
            .map(|arr| arr.iter().any(|v| v.get("platform").and_then(|p| p.as_str()) == Some(platform)))
            .unwrap_or(false);
        if has_pkg && !base.is_empty() && !token.is_empty() {
            if let (Some(ver), Some(cli)) = (obj.get("version").and_then(|v| v.as_str()), obj.get("key").and_then(|v| v.as_str())) {
                let url = format!("{}/api/v1/system/client/packages/{cli}/{ver}/{platform}", base.trim_end_matches('/'));
                let _ = obj.insert("packageUrl".into(), serde_json::Value::String(url));
                let _ = obj.insert("authToken".into(), serde_json::Value::String(token.clone()));
            }
        }
    }
    let agent_for_log = agent.clone();
    let plat = plat.inner().clone();
    let (ok, msg, id) = tauri::async_runtime::spawn_blocking(move || {
        platform::install_with_spec(&agent, &spec, &plat, &move |line: &str| {
            let _ = app.emit(
                "install-log",
                InstallLog {
                    agent: agent_for_log.clone(),
                    line: line.to_string(),
                },
            );
        })
    })
    .await
    .map_err(|e| format!("安装任务异常: {e}"))?;
    Ok(AgentOpResult {
        success: ok,
        message: msg,
        pid_or_container_id: id,
    })
}

#[tauri::command]
async fn start_agent(
    agent: String,
    spec: serde_json::Value,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let plat = state.inner().clone();
    let (ok, msg, id) = tauri::async_runtime::spawn_blocking(move || {
        platform::start_with_spec(&agent, &spec, &plat)
    })
    .await
    .map_err(|e| format!("启动任务异常: {e}"))?;
    Ok(AgentOpResult {
        success: ok,
        message: msg,
        pid_or_container_id: id,
    })
}

#[tauri::command]
async fn stop_agent(
    agent: String,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let plat = state.inner().clone();
    let (ok, msg, id) = tauri::async_runtime::spawn_blocking(move || {
        platform::stop_agent(&agent, &plat)
    })
    .await
    .map_err(|e| format!("停止任务异常: {e}"))?;
    Ok(AgentOpResult {
        success: ok,
        message: msg,
        pid_or_container_id: id,
    })
}

#[tauri::command]
async fn uninstall_agent(
    agent: String,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let plat = state.inner().clone();
    let (ok, msg) = tauri::async_runtime::spawn_blocking(move || {
        platform::uninstall_agent(&agent, &plat, &|_line: &str| {})
    })
    .await
    .map_err(|e| format!("卸载任务异常: {e}"))?;
    Ok(AgentOpResult {
        success: ok,
        message: msg,
        pid_or_container_id: String::new(),
    })
}

#[tauri::command]
async fn get_agent_runtime(agent: String) -> Result<AgentRuntime, String> {
    let key = agent.clone();
    let (installed, running, version, id) =
        tauri::async_runtime::spawn_blocking(move || platform::get_agent_runtime(&key))
            .await
            .map_err(|e| format!("状态查询异常: {e}"))?;
    Ok(AgentRuntime {
        agent,
        installed,
        running,
        version,
        pid_or_container_id: id,
    })
}

#[tauri::command]
async fn save_agent_config(agent: String, content: String) -> Result<ConfigResult, String> {
    let (ok, written, path, msg) =
        tauri::async_runtime::spawn_blocking(move || platform::save_agent_config(&agent, &content))
            .await
            .map_err(|e| format!("保存配置异常: {e}"))?;
    Ok(ConfigResult {
        success: ok,
        content: written,
        path,
        message: msg,
    })
}

#[tauri::command]
async fn read_agent_config(agent: String) -> Result<ConfigResult, String> {
    let (ok, content, path, msg) =
        tauri::async_runtime::spawn_blocking(move || platform::read_agent_config(&agent))
            .await
            .map_err(|e| format!("读取配置异常: {e}"))?;
    Ok(ConfigResult {
        success: ok,
        content,
        path,
        message: msg,
    })
}

/// openclaw 版本对齐确认回填（App.vue 确认弹窗的确认/拒绝按钮）。
/// false = 当前无待确认请求（弹窗过期/已处理）。
#[tauri::command]
fn client_confirm_openclaw_align(
    plat: tauri::State<'_, platform::PlatformState>,
    accept: bool,
) -> Result<bool, String> {
    Ok(plat.respond_align_confirm(accept))
}

#[tauri::command]
fn client_get_keep_awake(plat: tauri::State<'_, platform::PlatformState>) -> Result<bool, String> {
    Ok(platform::keep_awake_is_on(plat.inner()))
}

/// 防锁屏开关：持久化到设置文件 + 立即生效。返回生效后的实际状态。
#[tauri::command]
fn client_set_keep_awake(
    enabled: bool,
    app_state: tauri::State<'_, AppState>,
    plat: tauri::State<'_, platform::PlatformState>,
) -> Result<bool, String> {
    aiarsenal_client_core::util::set_setting(app_state.data_dir(), "keepAwake", enabled.into());
    if enabled {
        if !platform::keep_awake_enable(plat.inner()) {
            return Err("启用防锁屏失败（无法启动 caffeinate）".into());
        }
    } else {
        platform::keep_awake_disable(plat.inner());
    }
    aiarsenal_client_core::util::log("info", &format!("[keep-awake] 设为 enabled={enabled}"));
    Ok(platform::keep_awake_is_on(plat.inner()))
}

// ==================== 权限清单页（feature openclaw-desktop） ====================
//
// 状态查询/申请涉及 TCC API 与 5s channel 等待，走 spawn_blocking 不冻主线程；
// 未启用 feature 的构建不注册（前端按平台隐藏入口，误调报错可恢复）。

/// 全量权限状态（9 项：screen/accessibility/location/contacts/calendar/reminders/photos/
/// camera/notifications），前端清单页渲染 + 2s 轮询（用户在系统弹框/设置里授权后自动变绿）。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn get_permission_statuses() -> Result<Vec<platform::desktop_node::permissions::PermStatus>, String> {
    tauri::async_runtime::spawn_blocking(platform::desktop_node::permissions::status_list)
        .await
        .map_err(|e| format!("状态查询异常: {e}"))
}

/// 申请单项权限（触发系统授权框；fire-and-forget，弹框挂着时返回仍 notDetermined，
/// 靠前端轮询看到变绿）。返回申请后的当前态。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn request_permission(key: String) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        platform::desktop_node::permissions::request_one(&key)
    })
    .await
    .map_err(|e| format!("权限申请异常: {e}"))
}

/// 打开系统设置 → 隐私与安全 → 对应权限面板（denied 后唯一的路）。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn open_permission_settings(key: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        platform::desktop_node::permissions::open_settings(&key)
    })
    .await
    .map_err(|e| format!("打开设置异常: {e}"))?;
    Ok(())
}

/// 是否 dev 构建（debug_assertions）。权限页展示「开发模式 TCC 归因可能不稳」横幅。
/// mac/win 都注册（不 feature 门）。
#[tauri::command]
fn client_is_dev_build() -> bool {
    cfg!(debug_assertions)
}

/// 前端审批弹窗回应（exec-approval-request 事件）。decision: once|always|deny。
/// false = 当前无待审批或 id 已过期/已答（60s 超时后端已自动拒绝）。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
fn client_respond_exec_approval(id: String, decision: String) -> Result<bool, String> {
    Ok(platform::desktop_node::exec_approvals::respond(&id, &decision))
}

/// 读 exec 审批档位摘要（mode/security/ask/path/allowlist），Dashboard 档位卡渲染。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn client_get_exec_policy() -> Result<serde_json::Value, String> {
    tauri::async_runtime::spawn_blocking(platform::desktop_node::exec_approvals::policy_summary)
        .await
        .map_err(|e| format!("读策略异常: {e}"))
}

/// 切 exec 审批档位（full|ask|allowlist|deny），写 ~/.openclaw/exec-approvals.json。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn client_set_exec_policy(mode: String) -> Result<serde_json::Value, String> {
    tauri::async_runtime::spawn_blocking(move || {
        platform::desktop_node::exec_approvals::set_policy_mode(&mode)
    })
    .await
    .map_err(|e| format!("切档异常: {e}"))?
}

/// 发起设备码登录：生成 dc → 打开浏览器授权页 → 后台轮询 exchange（成功自动存 token）。
/// 无主窗口形态下替代前端轮询：心跳 loop 靠 `token().is_empty()` 自动接管，无需额外接线。
/// `LOGIN_IN_PROGRESS` 防重入，避免重复弹窗/轮询。
fn begin_login(handle: AppHandle) {
    if LOGIN_IN_PROGRESS.swap(true, Ordering::SeqCst) {
        return; // 已有登录轮询进行中
    }
    let state = handle.state::<AppState>();
    let dc = state.start_login();
    let url = format!(
        "{}/#/client-login?dc={}",
        aiarsenal_client_core::derive_ui_url(&state.base_url()),
        dc
    );
    aiarsenal_client_core::util::log("info", &format!("begin_login: 打开浏览器 {}", url));
    let _ = open::that(&url);
    let h = handle.clone();
    tauri::async_runtime::spawn(async move {
        let state = h.state::<AppState>();
        let started = Instant::now();
        loop {
            tokio::time::sleep(Duration::from_secs(2)).await;
            match state.exchange(&dc).await {
                Ok(r) if r.status == "ok" && !r.token.is_empty() => {
                    LOGIN_IN_PROGRESS.store(false, Ordering::SeqCst);
                    aiarsenal_client_core::util::log("info", "登录成功（exchange ok），托盘 登入→登出");
                    if let Some(t) = h.try_state::<TrayRefs>() {
                        let _ = t.auth.set_text("登出");
                    }
                    return;
                }
                _ if started.elapsed() > Duration::from_secs(300) => {
                    LOGIN_IN_PROGRESS.store(false, Ordering::SeqCst);
                    aiarsenal_client_core::util::log(
                        "warn",
                        "登录轮询超时（5min）放弃；可点托盘「登入」重试",
                    );
                    return;
                }
                _ => {}
            }
        }
    });
}

// ==================== 启动 ====================

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // 数据目录：日志文件也落这里（aiarsenal-client.log）
    let data_dir = dirs::data_dir()
        .map(|d| d.join("aiarsenal-client"))
        .unwrap_or_else(std::path::PathBuf::new);
    aiarsenal_client_core::util::init_log(&data_dir);
    // 权限开关本地记忆（permsDisabled）读 data_dir：ensure_* / 启动申请 / 托盘切换深处都要用，全局化一次。
    #[cfg(feature = "openclaw-desktop")]
    platform::desktop_node::permissions::init_data_dir(data_dir.clone());
    aiarsenal_client_core::util::log("info", "App 启动（mac）");
    let data_dir_task = data_dir.clone();
    tauri::Builder::default()
        .plugin(tauri_plugin_deep_link::init())
        .manage(
            AppState::new(data_dir, DEFAULT_ENV).expect("初始化应用状态失败"),
        )
        .manage(platform::PlatformState::default())
        // canvas 静态文件托管：openclaw-canvas://localhost/<path> → ~/.openclaw/canvas 下文件。
        // 须在建窗口前注册（canvas 窗口用 External(this scheme) 创建）。
        // （scheme 名常驻注册：feature 关闭时 desktop_node 不编译，闭包返回空响应即可。）
        .register_uri_scheme_protocol("openclaw-canvas", |_ctx, request| {
            #[cfg(feature = "openclaw-desktop")]
            return platform::desktop_node::canvas::handle_scheme(request);
            #[allow(unreachable_code)]
            #[cfg(not(feature = "openclaw-desktop"))]
            tauri::http::Response::new(Vec::new())
        })
        .on_window_event(|window, event| {
            // 关闭按钮=隐藏到托盘（保活心跳），不退出；退出走托盘菜单。
            // canvas 窗口同策略：hide 不销毁（保 WKWebView 会话状态，eval/snapshot 不丢上下文）。
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" || window.label() == "canvas" {
                    let _ = window.hide();
                    api.prevent_close();
                }
            }
        })
        .setup(move |app| {
            let handle = app.handle().clone();
            // 回填 AppHandle：版本对齐确认门要 emit 事件 + 把主窗口从托盘拉出来
            handle
                .state::<platform::PlatformState>()
                .set_app_handle(app.handle().clone());
            // 纯托盘形态：仅菜单栏、无 Dock 图标（后台守护）
            #[cfg(target_os = "macos")]
            {
                let _ = app.set_activation_policy(tauri::ActivationPolicy::Accessory);
            }
            // 防锁屏：默认启动即常开。在建托盘前启用，使菜单勾选态反映启用后真态。
            if aiarsenal_client_core::util::keep_awake_enabled(&data_dir_task) {
                let ka_state = handle.state::<platform::PlatformState>().inner().clone();
                let ok = platform::keep_awake_enable(&ka_state);
                aiarsenal_client_core::util::log(
                    if ok { "info" } else { "warn" },
                    &format!("[keep-awake] 启动启用防锁屏 ok={ok}"),
                );
            }
            // 系统托盘：登入/登出（互斥，同格变形）、防止锁屏（勾选）、退出
            {
                let state = handle.state::<AppState>();
                let logged_in = !state.token().is_empty();
                let ka_on = platform::keep_awake_is_on(
                    &handle.state::<platform::PlatformState>().inner(),
                );
                let auth_text = if logged_in { "登出" } else { "登入" };
                let auth_item = MenuItem::with_id(app, "auth", auth_text, true, None::<&str>)?;
                let ka_item =
                    CheckMenuItem::with_id(app, "ka", "防止锁屏", true, ka_on, None::<&str>)?;
                let quit_item = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
                // openclaw-desktop：「权限」子菜单（放最前，替代原 screen/a11y 两个单项）。
                // 9 项点击即切换：已授权→tccutil 收回+本地记住；已关闭→清标志+申请；未申请→弹框申请。
                // 初始文字占位「…」，后台线程算好状态回主线程刷新（notifications 查询带 5s 等待）。
                #[cfg(feature = "openclaw-desktop")]
                let perm_items: Vec<MenuItem<tauri::Wry>> =
                    platform::desktop_node::permissions::PERM_TOGGLES
                        .iter()
                        .map(|spec| {
                            MenuItem::with_id(
                                app,
                                format!("perm:{}", spec.key),
                                "…",
                                true,
                                None::<&str>,
                            )
                        })
                        .collect::<Result<Vec<_>, _>>()?;
                #[cfg(feature = "openclaw-desktop")]
                let perm_submenu = {
                    let item_refs: Vec<&dyn tauri::menu::IsMenuItem<tauri::Wry>> = perm_items
                        .iter()
                        .map(|i| i as &dyn tauri::menu::IsMenuItem<tauri::Wry>)
                        .collect();
                    Submenu::with_items(app, "权限", true, &item_refs)?
                };
                #[cfg(feature = "openclaw-desktop")]
                let menu =
                    Menu::with_items(app, &[&perm_submenu, &auth_item, &ka_item, &quit_item])?;
                #[cfg(not(feature = "openclaw-desktop"))]
                let menu = Menu::with_items(app, &[&auth_item, &ka_item, &quit_item])?;
                app.manage(TrayRefs {
                    auth: auth_item,
                    ka: ka_item,
                    #[cfg(feature = "openclaw-desktop")]
                    perm_items: perm_items.clone(),
                });
                #[cfg(feature = "openclaw-desktop")]
                {
                    let h = handle.clone();
                    tauri::async_runtime::spawn_blocking(move || {
                        let texts: Vec<String> =
                            platform::desktop_node::permissions::PERM_TOGGLES
                                .iter()
                                .map(|s| platform::desktop_node::permissions::menu_text(s.key))
                                .collect();
                        let h2 = h.clone();
                        let _ = h.run_on_main_thread(move || {
                            if let Some(t) = h2.try_state::<TrayRefs>() {
                                for (item, text) in t.perm_items.iter().zip(texts) {
                                    let _ = item.set_text(text);
                                }
                            }
                        });
                    });
                }
                let icon = tauri::image::Image::from_bytes(include_bytes!(
                    "../../../shared/rust/assets/tray-icon.png"
                ))?;
                TrayIconBuilder::with_id("main-tray")
                    .icon(icon)
                    .tooltip("AI Arsenal")
                    .menu(&menu)
                    .on_menu_event(|app, event| match event.id.as_ref() {
                        #[cfg(feature = "openclaw-desktop")]
                        k if k.starts_with("perm:") => {
                            // 权限子菜单切换：tccutil/状态查询阻塞 → 后台线程执行，回主线程刷文字。
                            let key = k.strip_prefix("perm:").unwrap_or("").to_string();
                            let h = app.clone();
                            std::thread::spawn(move || {
                                let text = platform::desktop_node::permissions::handle_tray_toggle(&key);
                                let h2 = h.clone();
                                let _ = h.run_on_main_thread(move || {
                                    if let Some(t) = h2.try_state::<TrayRefs>() {
                                        if let Some(item) =
                                            platform::desktop_node::permissions::PERM_TOGGLES
                                                .iter()
                                                .position(|s| s.key == key)
                                                .and_then(|i| t.perm_items.get(i))
                                        {
                                            let _ = item.set_text(text);
                                        }
                                    }
                                });
                            });
                        }
                        "auth" => {
                            let state = app.state::<AppState>();
                            if state.token().is_empty() {
                                // 未登录 → 登入（弹浏览器 + 后台轮询 exchange）
                                begin_login(app.clone());
                            } else {
                                // 已登录 → 登出
                                state.logout();
                                if let Some(t) = app.try_state::<TrayRefs>() {
                                    let _ = t.auth.set_text("登入");
                                }
                                aiarsenal_client_core::util::log(
                                    "info",
                                    "已登出（托盘菜单），token 清空",
                                );
                            }
                        }
                        "ka" => {
                            let plat = app.state::<platform::PlatformState>().inner().clone();
                            let want = !platform::keep_awake_is_on(&plat);
                            let applied = if want {
                                platform::keep_awake_enable(&plat)
                            } else {
                                platform::keep_awake_disable(&plat);
                                true
                            };
                            if applied {
                                let data_dir = app.state::<AppState>().data_dir().to_path_buf();
                                aiarsenal_client_core::util::set_setting(
                                    &data_dir,
                                    "keepAwake",
                                    want.into(),
                                );
                                if let Some(t) = app.try_state::<TrayRefs>() {
                                    let _ = t.ka.set_checked(want);
                                }
                            }
                            aiarsenal_client_core::util::log(
                                "info",
                                &format!("[keep-awake] 托盘切换 want={want} applied={applied}"),
                            );
                        }
                        "quit" => {
                            // 退出前上报 client=stopped（App 离线），尽力而为不阻塞过久
                            let st = app.state::<AppState>();
                            let pid = std::process::id();
                            let _ = tauri::async_runtime::block_on(
                                st.report_app_lifecycle("stopped", pid, platform::scan_all_skills()),
                            );
                            app.exit(0);
                        }
                        _ => {}
                    })
                    .on_tray_icon_event(|tray, event| {
                        // 左键单击托盘图标：未登录 → 发起登入；已登录 → 无操作（纯托盘无窗口可显）
                        if let TrayIconEvent::Click {
                            button: MouseButton::Left,
                            button_state: MouseButtonState::Up,
                            ..
                        } = event
                        {
                            let app = tray.app_handle();
                            let state = app.state::<AppState>();
                            if state.token().is_empty() {
                                begin_login(app.clone());
                            }
                        }
                    })
                    .build(app)?;
                // 30s 定时刷新权限子菜单文字：用户在系统设置/托盘操作后，菜单自动变形（不重建菜单）。
                #[cfg(feature = "openclaw-desktop")]
                {
                    let h = handle.clone();
                    tauri::async_runtime::spawn(async move {
                        loop {
                            tokio::time::sleep(Duration::from_secs(30)).await;
                            let Some(t) = h.try_state::<TrayRefs>() else {
                                continue;
                            };
                            // menu_text 含阻塞查询（notifications 5s channel 等待）→ 阻塞池算，回 async 刷
                            //（set_text 内部走 run_main_thread，任意线程可调）。
                            let texts = tauri::async_runtime::spawn_blocking(|| {
                                platform::desktop_node::permissions::PERM_TOGGLES
                                    .iter()
                                    .map(|s| {
                                        platform::desktop_node::permissions::menu_text(s.key)
                                    })
                                    .collect::<Vec<_>>()
                            })
                            .await
                            .unwrap_or_default();
                            for (item, text) in t.perm_items.iter().zip(texts) {
                                let _ = item.set_text(text);
                            }
                        }
                    });
                }
            }
            // 启动登录态：已登录 → 上报 running；未登录 → 自动跳浏览器登入
            {
                let state = handle.state::<AppState>();
                if state.token().is_empty() {
                    aiarsenal_client_core::util::log("info", "启动时未登录，自动跳浏览器登入");
                    begin_login(handle.clone());
                } else {
                    aiarsenal_client_core::util::log("info", "启动时已登录，纯托盘后台运行");
                    let boot_handle = handle.clone();
                    let pid = std::process::id();
                    tauri::async_runtime::spawn(async move {
                        let st = boot_handle.state::<AppState>();
                        st.report_app_lifecycle("running", pid, platform::scan_all_skills()).await;
                    });
                }
            }
            {
                use tauri_plugin_deep_link::DeepLinkExt;
                let _ = app.deep_link().register_all();
                // on_open_url 的 move 闭包会占用 handle，先 clone 一份给它，
                // 原始 handle 仍可被后面的远程指令轮询器 clone 使用。
                let dl_handle = handle.clone();
                app.deep_link().on_open_url(move |event| {
                    if let Some(url) = event.urls().first() {
                        if let Some(dc) =
                            aiarsenal_client_core::parse_dc_from_deep_link(url.as_str())
                        {
                            aiarsenal_client_core::util::log(
                                "info",
                                &format!("deep-link 收到 dc={}", dc),
                            );
                            let _ = dl_handle.emit("client://deep-link", dc);
                        }
                    }
                });
            }
            // 心跳 loop：上行多模块状态(日志/配置变更/命令进度)，下行认领命令
            // 命令 start/done 时 notify 触发立即上报；执行中每 1s；空闲按自适应
            let log_buffers: Arc<Mutex<HashMap<String, VecDeque<LogEntry>>>> = Arc::new(Mutex::new(HashMap::new()));
            let current: Arc<Mutex<Option<CurrentCmd>>> = Arc::new(Mutex::new(None));
            let notify: Arc<tokio::sync::Notify> = Arc::new(tokio::sync::Notify::new());
            let lb1 = log_buffers.clone();
            let cc1 = current.clone();
            let poll_handle = handle.clone();
            let data_dir_hb = data_dir_task.clone();
            tauri::async_runtime::spawn(async move {
                // 强制登出去重标记：被 revoke 后只 emit 一次 force-logout；
                // 重登恢复正常心跳后（下方 Ok 分支）重置，下次再被 revoke 能再次触发。
                let mut force_logged_out = false;
                // desktop_node 已随登录自动起过一次的标记（start 本身幂等，这里避免每跳心跳重复尝试）。
                let mut desktop_node_ensured = false;
                loop {
                    // 循环顶部 clearReported（对齐 manager 的 cmdState）：终态且已上报过的命令清掉，
                    // 释放队列接下一条。不在底部按 status 清——那样会吞掉「done 后还没挂进任何心跳」
                    // 的终态（服务端收不到结果只能超时标 failed）。reported 由下方 build 时标记。
                    {
                        let mut c = cc1.lock().unwrap();
                        if let Some(cc) = c.as_ref() {
                            if cc.reported && (cc.status == "done" || cc.status == "failed") {
                                *c = None;
                            }
                        }
                    }
                    // 间隔：命令执行中（current 处于 running）每 1s 上报输出/错误/进度；
                    // 否则按自适应（8–20 点 2s，其它 10s）。命令 start/done 时 notify 立即触发。
                    // 只认 running 不认 done/failed：报 done 的同一轮就能认领下一条，不必等下一跳（夜里 10s）。
                    let interval = if cc1
                        .lock()
                        .unwrap()
                        .as_ref()
                        .map_or(false, |c| c.status == "running")
                    {
                        1
                    } else {
                        aiarsenal_client_core::util::heartbeat_interval_secs()
                    };
                    tokio::select! {
                        _ = tokio::time::sleep(std::time::Duration::from_secs(interval)) => {}
                        _ = notify.notified() => {}
                    }
                    let state = poll_handle.state::<AppState>();
                    if state.token().is_empty() {
                        continue;
                    }
                    // 登录后自动起 desktop node（一次；它内部重连，跟 client 生命周期常驻，不依赖后端下发 start）
                    if !desktop_node_ensured {
                        platform::ensure_desktop_node(&poll_handle);
                        desktop_node_ensured = true;
                    }
                    let device_id = state.device_id();
                    let plat = poll_handle.state::<platform::PlatformState>().inner().clone();
                    // 构造心跳请求（阻塞 I/O：get_agent_runtime + 读配置文件）
                    let lb_b = lb1.clone();
                    let cc_b = cc1.clone();
                    // 只上报「已激活」（装过/下过命令）的 agent，避免本机自装 Obsidian、
                    // 从未触碰的 hermes 污染状态列表
                    let activated = aiarsenal_client_core::util::load_activated_agents(&data_dir_hb);
                    let app_pid = std::process::id();
                    let plat_hb = plat.clone();
                    let req = match tauri::async_runtime::spawn_blocking(move || {
                        let mut modules = HashMap::new();
                        // App 自身作为 client agent 上报：在线（running）+ 版本 + pid
                        let mut client_report = ModuleReport::default();
                        client_report.status = "running".into();
                        client_report.version = aiarsenal_client_core::util::APP_VERSION.into();
                        client_report.pid_or_container_id = app_pid.to_string();
                        // aa.json 快照捎带（hash+全文）：服务端 hash 变化落「配置变更」留痕
                        //（清单下发/授权状态写入都体现在这份文件里）
                        if let Some((hash, size, _mtime, content)) = platform::config_info("client") {
                            client_report.config_hash = hash;
                            client_report.config_size = size;
                            client_report.config_content = content;
                        }
                        modules.insert(aiarsenal_client_core::util::CLIENT_AGENT_KEY.into(), client_report);
                        // 当前执行中命令的 agent 必须始终上报——即便不在服务端 selectedAgents（activated）
                        // 里。否则命令的进度/终态会被下面的 activated 过滤吞掉，服务端收不到 done，
                        // 只能 20min 后超时标 failed（实际安装已成功）。详见 clearReported。
                        let cur_agent: Option<String> =
                            cc_b.lock().unwrap().as_ref().map(|c| c.agent.clone());
                        for agent in platform::compiled_agents() {
                            if !activated.contains(agent) && cur_agent.as_deref() != Some(agent) {
                                continue;
                            }
                            let (installed, running, version, pid) = platform::get_agent_runtime(agent);
                            let status = if running { "running" } else if installed { "stopped" } else { "notinstalled" };
                            let mut report = ModuleReport::default();
                            report.status = status.into();
                            report.version = version;
                            report.pid_or_container_id = pid;
                            // openclaw：环境探测（本机多来源版本 + 是否需对齐）随心跳上报，
                            // 管理后台据此看到「本机已有 openclaw / 版本待对齐」，install 对齐需本机用户确认
                            #[cfg(feature = "agent-openclaw")]
                            if agent == "openclaw" {
                                report.env = Some(platform::openclaw_env_report());
                            }
                            let logs: Vec<LogEntry> = {
                                let mut bufs = lb_b.lock().unwrap();
                                bufs.entry(agent.into()).or_default().drain(..).collect()
                            };
                            report.logs = logs;
                            if let Some((hash, size, _mtime, content)) = platform::config_info(agent) {
                                report.config_hash = hash;
                                report.config_size = size;
                                report.config_content = content;
                            }
                            modules.insert(agent.into(), report);
                        }
                        // 挂当前执行中命令到对应模块。reported 只在真正挂进【要发出去的】module 后置真——
                        // 否则 clearReported 会清掉「标了 reported 却没发」的终态，done/failed 永远丢。
                        let cur = {
                            let guard = cc_b.lock().unwrap();
                            guard.as_ref().map(|c| c.clone())
                        };
                        if let Some(c) = cur {
                            if let Some(m) = modules.get_mut(&c.agent) {
                                // 终态已挂进将发出的 module → 标 reported，下一轮 clearReported 才会清。
                                // command_id 校验防竞态：读 cur 到再锁之间命令可能已被替换。
                                if c.status == "done" || c.status == "failed" {
                                    if let Some(x) = cc_b.lock().unwrap().as_mut() {
                                        if x.command_id == c.command_id {
                                            x.reported = true;
                                        }
                                    }
                                }
                                // 先取 running 标志，下面 c.status 会被 move 进 RunningCommandReport
                                let is_running = c.status == "running";
                                m.running_command = Some(RunningCommandReport {
                                    command_id: c.command_id,
                                    action: c.action.clone(),
                                    status: c.status,
                                    progress: c.progress,
                                    percent: c.percent,
                                    last_line: c.last_line,
                                    result: c.result,
                                    error: c.error,
                                });
                                // 命令执行中：按命令类型报 interim 状态，确保状态变化在时间线可见
                                // （检测可能漏掉短暂状态，如 1.5s 重启里的 stopped 瞬间）
                                if is_running {
                                    match c.action.as_str() {
                                        "install" => m.status = "installing".into(),
                                        "stop" | "restart" => m.status = "stopped".into(),
                                        "uninstall" => m.status = "notinstalled".into(),
                                        _ => {} // start / config_push：报真实检测状态
                                    }
                                }
                            }
                        }
                        // 渠道登录进行中状态回传后端（扫码期 QR / 已连接 / 失败）
                        let mut channels: Vec<aiarsenal_client_core::ChannelStateReport> = Vec::new();
                        if let Some(st) = platform::channel_login_state(&plat_hb) {
                            channels.push(aiarsenal_client_core::ChannelStateReport {
                                channel: st.channel,
                                account_id: "default".into(),
                                login_status: st.login_status.clone(),
                                qr_data_url: st.qr_data_url,
                                account_alias: st.account_alias,
                                message: st.message,
                            });
                            // 终态（connected/failed/disconnected）发一次即清空，避免残留态
                            // 覆盖后续后台操作（如禁用渠道设 idle）。
                            if st.login_status == "connected" || st.login_status == "failed" || st.login_status == "disconnected" {
                                platform::set_channel_login_state(&plat_hb, None);
                            }
                        }
                        // App 授权状态快照：与上次已发不同才带（启动首跳/变更后），变更主动上报
                        #[cfg(feature = "openclaw-desktop")]
                        let perm_state = platform::desktop_node::app_perm::pending_upload();
                        #[cfg(not(feature = "openclaw-desktop"))]
                        let perm_state: Option<aiarsenal_client_core::types::PermStateReport> = None;
                        // 已装技能快照：扫 ~/.openclaw/skills（纯本地 FS 读），后端整体覆盖落库
                        let skills = platform::scan_all_skills();
                        HeartbeatReq { device_id, modules, channels, perm_state, skills }
                    }).await {
                        Ok(r) => r,
                        Err(_) => continue,
                    };
                    // 发心跳
                    let res = match state.heartbeat(&req).await {
                        Ok(r) => {
                            // 恢复正常心跳 → 清除强制登出标记
                            force_logged_out = false;
                            r
                        }
                        Err(e) => {
                            if e.is_unauthorized() {
                                // 设备登录态被服务端吊销（管理员 revoke）→ 清 token + 通知前端
                                // 弹窗重新授权。只 emit 一次，避免循环每轮重复弹；清 token 后下一轮
                                // is_empty 自然跳过心跳，用户重登填回 token 后心跳自动恢复
                                // （不 break，否则重登后心跳再也起不来）。
                                if !force_logged_out {
                                    force_logged_out = true;
                                    aiarsenal_client_core::util::log(
                                        "warn",
                                        "设备登录态已被服务端撤销（revoke），强制登出，等待用户重新授权",
                                    );
                                    state.logout();
                                    // 同步托盘 登出→登入，并自动跳浏览器重新授权（对齐
                                    // 「未登录自动跳浏览器登入」的设计；前端不在，靠 begin_login 轮询）
                                    if let Some(t) = poll_handle.try_state::<TrayRefs>() {
                                        let _ = t.auth.set_text("登入");
                                    }
                                    begin_login(poll_handle.clone());
                                    let _ = poll_handle.emit("force-logout", ());
                                }
                            } else {
                                aiarsenal_client_core::util::log("error", &format!("心跳失败: {e}"));
                            }
                            continue;
                        }
                    };
                    // 授权状态快照已随本轮心跳送达 → 收口已发标记（未带/心跳失败不动，下轮比对重发）
                    #[cfg(feature = "openclaw-desktop")]
                    if let Some(p) = &req.perm_state {
                        platform::desktop_node::app_perm::mark_uploaded(p);
                    }
                    // 服务端返回该设备选中的 agent → 覆盖本地已激活集合
                    // （用户在设备控制弹窗多选框选哪些 agent，App 就上报哪些；完全跟随服务端选择）
                    aiarsenal_client_core::util::set_activated_agents(&data_dir_hb, &res.activated_agents);
                    // 授权 App 清单（管理后台「App 权限」按设备配置）：变化才落盘并唤醒授权引擎
                    #[cfg(feature = "openclaw-desktop")]
                    if aiarsenal_client_core::util::app_perms_changed(&data_dir_hb, &res.app_perms) {
                        aiarsenal_client_core::util::log(
                            "info",
                            &format!("[desktop_node] [app-perm] 服务端清单变更：{} 个 App", res.app_perms.len()),
                        );
                        aiarsenal_client_core::util::set_app_perms(&data_dir_hb, &res.app_perms);
                        platform::desktop_node::app_perm::update_apps(&res.app_perms);
                    }
                    // 认领到命令且当前无执行中 → 起执行器
                    for mut cmd in res.commands {
                        aiarsenal_client_core::util::log(
                            "info",
                            &format!("认领命令: id={} action={} agent={}", cmd.command_id, cmd.action, cmd.agent),
                        );
                        // 只认 running 才算忙（对齐 manager 的 isBusy）：done/failed 不挡下一条，
                        // 这样「报 done 的同一轮心跳」即可认领响应里同时返回的下一条命令，流水线不卡。
                        let already = cc1
                            .lock()
                            .unwrap()
                            .as_ref()
                            .map_or(false, |c| c.status == "running");
                        if already {
                            break;
                        }
                        let cmd_agent = cmd.agent.clone();
                        let cmd_id = cmd.command_id.clone();
                        let cmd_action = cmd.action.clone();
                        // 认领即标记该 agent 已激活（后续心跳开始上报它）
                        aiarsenal_client_core::util::mark_agent_activated(&data_dir_hb, &cmd_agent);
                        *cc1.lock().unwrap() = Some(CurrentCmd {
                            command_id: cmd_id.clone(),
                            agent: cmd_agent.clone(),
                            action: cmd_action.clone(),
                            status: "running".into(),
                            ..Default::default()
                        });
                        let lb2 = lb1.clone();
                        let cc2 = cc1.clone();
                        let plat2 = plat.clone();
                        let emith = poll_handle.clone();
                        let n2 = notify.clone();
                        // 内层日志闭包会 move 自己捕获的变量，单独 clone 一份 agent/cc 给它，
                        // 外层 cmd_agent、cc2 留给后面的 CurrentCmd 用，避免 use-after-move。
                        let log_agent = cmd_agent.clone();
                        let log_cc = cc2.clone();
                        // install 命令：注入 packageUrl（App 从本服务下载安装包，不直连官网/镜像）。
                        // 下载端点公开（无鉴权），不需 authToken。覆盖后端下发 install（管理后台点安装）。
                        if cmd.action == "install" {
                            let bas = state.base_url();
                            let plat = if cfg!(target_os = "macos") { "mac" } else { "win" };
                            if !bas.is_empty() && !cmd.version.is_empty() {
                                if let Some(obj) = cmd.payload.as_object_mut() {
                                    let url = format!(
                                        "{}/api/v1/system/client/packages/{}/{}/{}",
                                        bas.trim_end_matches('/'),
                                        cmd.agent,
                                        cmd.version,
                                        plat
                                    );
                                    obj.insert("packageUrl".into(), serde_json::Value::String(url));
                                }
                            }
                        }
                        // skill_install 命令：payload.packagePath（相对路径）拼成绝对 packageUrl +
                        // 回填设备 token（下载端点 clientAuth 要 Bearer；token 只进内存不落命令 payload）。
                        if cmd.action == "skill_install" {
                            let bas = state.base_url();
                            if let Some(obj) = cmd.payload.as_object_mut() {
                                if let Some(p) = obj.get("packagePath").and_then(|v| v.as_str()) {
                                    if !bas.is_empty() && !p.is_empty() {
                                        let url = format!(
                                            "{}/{}",
                                            bas.trim_end_matches('/'),
                                            p.trim_start_matches('/')
                                        );
                                        obj.insert("packageUrl".into(), serde_json::Value::String(url));
                                    }
                                }
                            }
                            plat2.set_api_token(&state.token());
                        }
                        tauri::async_runtime::spawn_blocking(move || {
                            let action = cmd.action.clone();
                            let (status, result, err) = platform::execute_command(&cmd, &plat2, &move |line: &str| {
                                {
                                    let mut bufs = lb2.lock().unwrap();
                                    bufs.entry(log_agent.clone()).or_default().push_back(LogEntry {
                                        line: line.into(),
                                        level: "info".into(),
                                        ts: String::new(),
                                    });
                                }
                                if let Some(c) = log_cc.lock().unwrap().as_mut() {
                                    c.last_line = line.into();
                                }
                                let _ = emith.emit(
                                    "install-log",
                                    InstallLog { agent: format!("远程:{}", action), line: line.to_string() },
                                );
                            });
                            aiarsenal_client_core::util::log(
                                "info",
                                &format!("命令执行结束: status={}", status),
                            );
                            let mut c = cc2.lock().unwrap();
                            *c = Some(CurrentCmd {
                                command_id: cmd_id,
                                agent: cmd_agent,
                                action: cmd_action,
                                status,
                                result,
                                error: err,
                                ..Default::default()
                            });
                            // 执行完立即触发上报（done/failed + 结果/错误）
                            n2.notify_one();
                        });
                        // 命令刚开始：立即触发上报（installing 状态）
                        notify.notify_one();
                        break; // 串行：一次只执行一条
                    }
                    // 终态清理已移到循环顶部 clearReported（保证至少上报一次再清），此处不再清。
                }
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            client_start_login,
            client_exchange,
            client_get_agents,
            client_mcp_list,
            client_mcp_bind,
            client_mcp_unbind,
            client_get_env,
            client_logout,
            client_hide_window,
            open_login_page,
            install_agent,
            start_agent,
            stop_agent,
            uninstall_agent,
            get_agent_runtime,
            save_agent_config,
            read_agent_config,
            client_get_keep_awake,
            client_set_keep_awake,
            client_confirm_openclaw_align,
            #[cfg(feature = "openclaw-desktop")]
            get_permission_statuses,
            #[cfg(feature = "openclaw-desktop")]
            request_permission,
            #[cfg(feature = "openclaw-desktop")]
            open_permission_settings,
            #[cfg(feature = "openclaw-desktop")]
            client_respond_exec_approval,
            #[cfg(feature = "openclaw-desktop")]
            client_get_exec_policy,
            #[cfg(feature = "openclaw-desktop")]
            client_set_exec_policy,
            client_is_dev_build
        ])
        .run(tauri::generate_context!())
        .expect("error while running AI Arsenal client");
}
