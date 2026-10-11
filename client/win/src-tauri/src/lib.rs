//! AI Arsenal 桌面客户端（Windows）应用入口。
//!
//! 共享命令（登录中转 / 客户端规格 / 上报 / 环境切换）是对 `aiarsenal-client-core::AppState`
//! 的薄包装；平台命令（安装/启停 openclaw / hermes / obsidian）在 `platform.rs`。

pub mod platform;

use std::collections::{HashMap, VecDeque};
use std::sync::{Arc, Mutex};

use aiarsenal_client_core::{AppState, EnvInfo, ExchangeRes, HeartbeatReq, LogEntry, ModuleReport, RunningCommandReport};
use serde::Serialize;
use tauri::{menu::{Menu, MenuItem}, tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent}, Emitter, Manager};

/// 默认环境：test（api.aiarsenal.cc，HTTP + /etc/hosts 本机解析）。
const DEFAULT_ENV: &str = "test";

/// 当前执行中命令的共享状态（心跳 loop 与执行器之间传递进度/结果）。
#[derive(Clone, Default)]
struct CurrentCmd {
    command_id: String,
    agent: String,
    action: String,
    status: String,
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

#[derive(Debug, Serialize)]
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
    state.get_agents().await
        .map(|c| serde_json::to_value(c).unwrap_or(serde_json::Value::Null))
        .map_err(|e| e.to_string())
}

// ===== MCP 分组自助管理（MCP 管理弹窗；权限/配置重建都在服务端，端上只转发） =====

#[tauri::command]
async fn client_mcp_list(state: tauri::State<'_, AppState>) -> Result<serde_json::Value, String> {
    state.mcp_list().await
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

// ==================== 平台命令（Windows） ====================

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
        let platform = if cfg!(target_os = "windows") { "win" } else { "mac" };
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

// platform 层返回裸元组（与 mac 一致），用 let 解构而非 .map（元组无 map）
#[tauri::command]
fn start_agent(
    agent: String,
    spec: serde_json::Value,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let (ok, msg, id) = platform::start_with_spec(&agent, &spec, &state);
    Ok(AgentOpResult { success: ok, message: msg, pid_or_container_id: id })
}

#[tauri::command]
fn stop_agent(
    agent: String,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let (ok, msg, id) = platform::stop_agent(&agent, &state);
    Ok(AgentOpResult { success: ok, message: msg, pid_or_container_id: id })
}

#[tauri::command]
fn uninstall_agent(
    agent: String,
    state: tauri::State<'_, platform::PlatformState>,
) -> Result<AgentOpResult, String> {
    let (ok, msg) = platform::uninstall_agent(&agent, &state, &|_line: &str| {});
    Ok(AgentOpResult { success: ok, message: msg, pid_or_container_id: String::new() })
}

#[tauri::command]
fn get_agent_runtime(agent: String) -> Result<AgentRuntime, String> {
    let (installed, running, version, id) = platform::get_agent_runtime(&agent);
    Ok(AgentRuntime {
        agent,
        installed,
        running,
        version,
        pid_or_container_id: id,
    })
}

#[tauri::command]
fn save_agent_config(agent: String, content: String) -> Result<ConfigResult, String> {
    let (ok, content, path, msg) = platform::save_agent_config(&agent, &content);
    Ok(ConfigResult { success: ok, content, path, message: msg })
}

#[tauri::command]
fn read_agent_config(agent: String) -> Result<ConfigResult, String> {
    let (ok, content, path, msg) = platform::read_agent_config(&agent);
    Ok(ConfigResult { success: ok, content, path, message: msg })
}

#[tauri::command]
fn client_get_keep_awake() -> Result<bool, String> {
    Ok(platform::keep_awake_is_on())
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

/// 防锁屏开关：持久化到设置文件 + 立即生效。返回生效后的实际状态。
#[tauri::command]
fn client_set_keep_awake(
    enabled: bool,
    app_state: tauri::State<'_, AppState>,
) -> Result<bool, String> {
    aiarsenal_client_core::util::set_setting(app_state.data_dir(), "keepAwake", enabled.into());
    if enabled {
        if !platform::keep_awake_enable() {
            return Err("启用防锁屏失败".into());
        }
    } else {
        platform::keep_awake_disable();
    }
    aiarsenal_client_core::util::log("info", &format!("[keep-awake] 设为 enabled={enabled}"));
    Ok(platform::keep_awake_is_on())
}

// ==================== 权限/审批页命令（feature openclaw-desktop） ====================
//
// 状态查询/申请走 spawn_blocking 不冻主线程；未启用 feature 的构建不注册
//（前端按平台隐藏入口，误调报错可恢复）。

/// 全量权限状态（9 项：screen/accessibility/location/contacts/calendar/reminders/photos/
/// camera/notifications），前端清单页渲染 + 轮询。Windows 无 TCC：状态即设置页
/// 开关（permsDisabled），开关变化后轮询立即变绿。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn get_permission_statuses() -> Result<Vec<platform::desktop_node::permissions::PermStatus>, String> {
    tauri::async_runtime::spawn_blocking(platform::desktop_node::permissions::status_list)
        .await
        .map_err(|e| format!("状态查询异常: {e}"))
}

/// 申请单项权限（Windows 无系统弹框：跳转对应 ms-settings 页 / 回提示）。
/// 返回申请后的当前态。
#[cfg(feature = "openclaw-desktop")]
#[tauri::command]
async fn request_permission(key: String) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        platform::desktop_node::permissions::request_one(&key)
    })
    .await
    .map_err(|e| format!("权限申请异常: {e}"))
}

/// 打开对应能力的相关设置页（ms-settings: URI / 控制面板）。
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

/// 是否 dev 构建（debug_assertions）。权限页展示提示横幅用。mac/win 都注册（不 feature 门）。
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

// ==================== 启动 ====================

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // 数据目录：日志文件也落这里（aiarsenal-client.log）
    let data_dir = dirs::data_dir()
        .map(|d| d.join("aiarsenal-client"))
        .unwrap_or_else(std::path::PathBuf::new);
    aiarsenal_client_core::util::init_log(&data_dir);
    // 权限开关本地记忆（permsDisabled）读 data_dir：ensure_* 闸门 / 设置页开关深处都要用，全局化一次。
    #[cfg(feature = "openclaw-desktop")]
    platform::desktop_node::permissions::init_data_dir(data_dir.clone());
    aiarsenal_client_core::util::log("info", "App 启动（win）");
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
            // canvas 窗口同策略：hide 不销毁（保 WebView2 会话状态，eval/snapshot 不丢上下文）。
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" || window.label() == "canvas" {
                    let _ = window.hide();
                    api.prevent_close();
                }
            }
        })
        .setup(move |app| {
            // 注册自定义 scheme（生产由安装器写注册表；dev 需 register_all）
            let handle = app.handle().clone();
            // 回填 AppHandle：版本对齐确认门要 emit 事件 + 把主窗口从托盘拉出来
            handle
                .state::<platform::PlatformState>()
                .set_app_handle(app.handle().clone());
            // 启动时若已登录（有持久 token），直接隐藏到托盘，不打扰用户；
            // 未登录则显示主窗口走登录流程。托盘图标随时可唤出。
            {
                let state = handle.state::<AppState>();
                if !state.token().is_empty() {
                    aiarsenal_client_core::util::log("info", "启动时已登录，直接隐藏到托盘");
                    if let Some(w) = handle.get_webview_window("main") {
                        let _ = w.hide();
                    }
                    // 启动即上报 client=running（App 在线）
                    let boot_handle = handle.clone();
                    let pid = std::process::id();
                    tauri::async_runtime::spawn(async move {
                        let st = boot_handle.state::<AppState>();
                        st.report_app_lifecycle("running", pid, platform::scan_all_skills()).await;
                    });
                }
            }
            // 系统托盘：显示主窗口 / 退出
            {
                let show_item = MenuItem::with_id(app, "show", "显示主窗口", true, None::<&str>)?;
                let quit_item = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
                let menu = Menu::with_items(app, &[&show_item, &quit_item])?;
                let icon = tauri::image::Image::from_bytes(include_bytes!(
                    "../../../shared/rust/assets/tray-icon.png"
                ))?;
                TrayIconBuilder::with_id("main-tray")
                    .icon(icon)
                    .tooltip("AI Arsenal")
                    .menu(&menu)
                    .on_menu_event(|app, event| match event.id.as_ref() {
                        "show" => {
                            if let Some(w) = app.get_webview_window("main") {
                                let _ = w.show();
                                let _ = w.set_focus();
                            }
                        }
                        "quit" => {
                            // 退出前上报 client=stopped（App 离线），尽力而为
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
                        // 左键单击托盘图标 → 显示主窗口
                        if let TrayIconEvent::Click {
                            button: MouseButton::Left,
                            button_state: MouseButtonState::Up,
                            ..
                        } = event
                        {
                            let app = tray.app_handle();
                            if let Some(w) = app.get_webview_window("main") {
                                let _ = w.show();
                                let _ = w.set_focus();
                            }
                        }
                    })
                    .build(app)?;
            }
            {
                use tauri_plugin_deep_link::DeepLinkExt;
                let _ = app.deep_link().register_all();
                // on_open_url 的 move 闭包会占用 handle，先 clone 一份给它，
                // 原始 handle 仍可被后面的心跳 loop clone 使用。
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
            // 防锁屏：默认启动即常开（可在 Dashboard 关掉）。防 openclaw/hermes 操作中系统睡眠/锁屏。
            if aiarsenal_client_core::util::keep_awake_enabled(&data_dir_task) {
                let ok = platform::keep_awake_enable();
                aiarsenal_client_core::util::log(
                    if ok { "info" } else { "warn" },
                    &format!("[keep-awake] 启动启用防锁屏 ok={ok}"),
                );
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
                        modules.insert(aiarsenal_client_core::util::CLIENT_AGENT_KEY.into(), client_report);
                        for agent in platform::compiled_agents() {
                            if !activated.contains(agent) {
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
                        // 挂当前执行中命令到对应模块；终态在此标记 reported（对齐 manager 的 snapshot），
                        // 保证 done/failed 至少随一次心跳上报后，下一轮循环顶部 clearReported 才会清掉。
                        let cur = {
                            let mut guard = cc_b.lock().unwrap();
                            if let Some(c) = guard.as_mut() {
                                if c.status == "done" || c.status == "failed" {
                                    c.reported = true;
                                }
                                Some(c.clone())
                            } else {
                                None
                            }
                        };
                        if let Some(c) = cur {
                            if let Some(m) = modules.get_mut(&c.agent) {
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
                            if st.login_status == "connected" || st.login_status == "failed" || st.login_status == "disconnected" {
                                platform::set_channel_login_state(&plat_hb, None);
                            }
                        }
                        // App 授权状态快照：与上次已发不同才带（启动首跳/变更后），变更主动上报
                        #[cfg(feature = "openclaw-desktop")]
                        let perm_state = platform::desktop_node::app_perm::pending_upload();
                        #[cfg(not(feature = "openclaw-desktop"))]
                        let perm_state: Option<aiarsenal_client_core::types::PermStateReport> = None;
                        HeartbeatReq { device_id, modules, channels, perm_state, skills: platform::scan_all_skills() }
                    }).await {
                        Ok(r) => r,
                        Err(_) => continue,
                    };
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
                                    let _ = poll_handle.emit("force-logout", ());
                                }
                            } else {
                                aiarsenal_client_core::util::log("error", &format!("心跳失败: {e}"));
                            }
                            continue;
                        }
                    };
                    // 服务端返回该设备选中的 agent → 覆盖本地已激活集合
                    // （用户在设备控制弹窗多选框选哪些 agent，App 就上报哪些；完全跟随服务端选择）
                    aiarsenal_client_core::util::set_activated_agents(&data_dir_hb, &res.activated_agents);
                    // 授权状态快照已随本轮心跳送达 → 收口已发标记（未带/心跳失败不动，下轮比对重发）
                    #[cfg(feature = "openclaw-desktop")]
                    if let Some(p) = &req.perm_state {
                        platform::desktop_node::app_perm::mark_uploaded(p);
                    }
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
                            // status 下面要 move 进 CurrentCmd，先记日志
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
                        break;
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
            client_is_dev_build,
            get_permission_statuses,
            request_permission,
            open_permission_settings,
            client_respond_exec_approval,
            client_get_exec_policy,
            client_set_exec_policy
        ])
        .run(tauri::generate_context!())
        .expect("error while running AI Arsenal client");
}
