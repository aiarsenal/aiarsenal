//! openclaw desktop node：client 自当 openclaw 的 Windows node（对齐 mac desktop_node）。
//!
//! client 自己实现 desktop 控制（截屏/控鼠键，Win32 GDI + SendInput）+ 权限开关引擎
//!（Windows 无 TCC——能力开关在客户端设置 JSON 里，`permsDisabled` 数组；App 清单
//! 授权「清单即授权」），连本地 openclaw gateway（`ws://127.0.0.1:18789`）暴露
//! `computer.act` / `screen.snapshot` 等命令。gateway 仍是 openclaw CLI（client 不重写）。
//!
//! feature `openclaw-desktop` 编译时开关（platform.rs: `#[cfg(feature="openclaw-desktop")] mod desktop_node;`）。
//! 禁用时整个模块不编译，openclaw 退回无 desktop（只渠道+浏览器）。
//!
//! 与 mac 的差异：
//! - 权限层：无 TCC/系统弹框——9 个能力开关读 settings `permsDisabled`，默认全开；
//!   开关关着时命令报结构化 PERMISSION_MISSING（与 mac 未授权错误同形，agent 降级一致）。
//! - camera/calendar/reminders/location：设备枚举（camera.list）可用，取流/存储访问
//!   v1 返回结构化 UNAVAILABLE。
//! - canvas：WebView2（PrintWindow 快照 + 自定义 scheme 托管）。

pub mod permissions;
pub mod app_perm;
pub mod app_gate;
pub mod browser;
pub mod screen;
pub mod control;
pub mod system;
pub mod device;
pub mod camera;
pub mod location;
pub mod contacts;
pub mod timeutil;
pub mod calendar;
pub mod reminders;
pub mod photos;
pub mod exec_approvals;
pub mod canvas;
pub mod protocol;
pub mod mcp_server;

use std::sync::atomic::{AtomicBool, Ordering};

static NODE_STARTED: AtomicBool = AtomicBool::new(false);

/// 启动 desktop node：启动权限引擎 + MCP server + spawn 线程跑 node 协议。幂等（已启动跳过）。
/// 不接 on_log（线程超出 install 命令生命周期），进度走 util::log（[desktop_node] 前缀）。
/// `app`：AppHandle 透传给 protocol 线程（exec 审批弹窗 / canvas 窗口要拉主窗口、emit 事件）。
pub fn start(app: tauri::AppHandle) -> (bool, String) {
    if NODE_STARTED.swap(true, Ordering::SeqCst) {
        return (true, "desktop node 已启动（跳过重复）".into());
    }
    let log = |msg: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] {msg}"));
    log("启动 openclaw desktop node（client 自实现，替代 OpenClaw.app）...");
    // Windows 无 TCC：不需要系统权限弹框；9 个能力开关在设置页（permsDisabled），
    // 命令执行时按开关闸门报 PERMISSION_MISSING（见 permissions.rs）。
    // 授权引擎（服务端按设备配置的 App 清单）：启动用缓存清单播种，心跳清单变更时更新
    app_perm::start_engine();
    // 本地环回 MCP server（hermes 经 mcp_servers.aa-desktop 直连，复用 dispatch 闸门）
    mcp_server::start(app.clone());
    // spawn 线程跑 node 协议（连本地 gateway + invoke loop，阻塞；线程超出 install 命令生命周期，用 util::log）
    std::thread::spawn(move || {
        protocol::run(app);
    });
    (
        true,
        "desktop node 已启动（node 协议连本地 gateway，日志见 [desktop_node]）".into(),
    )
}

/// 停止 desktop node。当前 protocol 线程 daemon（无优雅 stop）；后续可加 channel 通知退出。
pub fn stop() {
    // TODO: 加 channel 通知 protocol 线程断 ws 退出 + 释放按键 hold
}
