//! openclaw desktop node：client 自当 openclaw 的 macOS node（替代 OpenClaw.app）。
//!
//! client 自己实现 desktop 控制（截屏/控鼠键）+ TCC 权限，连本地 openclaw gateway
//! （`ws://127.0.0.1:18789`）暴露 `computer.act` / `screen.snapshot` 命令。
//! gateway 仍是 openclaw CLI（install-cli.sh 装，client 不重写 gateway）。
//!
//! feature `openclaw-desktop` 编译时开关（platform.rs: `#[cfg(feature="openclaw-desktop")] mod desktop_node;`）。
//! 禁用时整个模块不编译，openclaw 退回无 desktop（只渠道+浏览器），轻松卸载（将来只支持 hermes）。
//!
//! 阶段进展（见 plan）：
//! - 阶段 1（本文件 + permissions.rs）：骨架 + TCC 权限申请
//! - 阶段 2：screen.rs（screencapturekit 截屏 + screen.snapshot + displayFrameId）
//! - 阶段 3：control.rs（CGEvent 控鼠键 + computer.act + 坐标映射 + 按键 hold）
//! - 阶段 4：protocol.rs（WebSocket + Ed25519 auth + node.invoke + 本地 auto-pair）
//! - 阶段 5：集成 install/start/stop feature-gated 起 desktop_node + 删装 OpenClaw.app

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

/// 启动 desktop node：申请 TCC 权限 + spawn 线程跑 node 协议。幂等（已启动跳过）。
/// 不接 on_log（线程超出 install 命令生命周期），进度走 util::log（[desktop_node] 前缀）。
/// `app`：AppHandle 透传给 protocol 线程（exec 审批弹窗 / canvas 窗口要拉主窗口、emit 事件）。
pub fn start(app: tauri::AppHandle) -> (bool, String) {
    if NODE_STARTED.swap(true, Ordering::SeqCst) {
        return (true, "desktop node 已启动（跳过重复）".into());
    }
    let log = |msg: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] {msg}"));
    log("启动 openclaw desktop node（client 自实现，替代 OpenClaw.app）...");
    let (screen, a11y) = permissions::request_desktop_permissions(&log);
    if !screen || !a11y {
        // 权限不足不阻断 node 协议（连 gateway 不依赖 TCC；截屏/控鼠键等用户在系统设置勾选本 App 后再生效）。
        log(&format!(
            "⚠️ 权限未齐 Screen={screen} Accessibility={a11y}：请在系统设置勾选本 App；node 协议仍启动"
        ));
    }
    // 启动即申请全部可弹框权限（权限页「一键申请」的开机版）：已授权项静默，仅首次弹一轮。
    // 后台线程跑——notifications 状态查询带 5s channel 等待，不能挡 node 协议连接；
    // System Events 自动化授权独立线程（osascript 阻塞到用户点授权框）。
    std::thread::spawn(permissions::request_startup_permissions);
    permissions::authorize_system_events_automation();
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
        format!("desktop node 已启动（Screen={screen}, A11y={a11y}；node 协议连本地 gateway，日志见 [desktop_node]）"),
    )
}

/// 停止 desktop node。当前 protocol 线程 daemon（无优雅 stop）；后续可加 channel 通知退出。
pub fn stop() {
    // TODO: 加 channel 通知 protocol 线程断 ws 退出 + 释放按键 hold
}
