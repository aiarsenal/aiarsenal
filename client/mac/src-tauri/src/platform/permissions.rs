//! macOS 权限预申请（mac crate 独占，win 无此模块）。
//!
//! hermes 的 TCC 权限（Screen Recording / Accessibility）绑独立签名的 cua-driver
//!（com.trycua.driver，随 `hermes computer-use install` 安装），**不绑本 client App**。
//! 走 hermes 自带 CLI（install + permissions grant）即归因正确，无需本 App 调 CGRequestScreenCaptureAccess、
//! 也无需签名/引 objc 依赖。install_with_spec 尾部 fire-and-forget 触发。
//!
//! openclaw 侧的权限/desktop node：feature openclaw-desktop 启用时由 desktop_node/（client 自实现，
//! 权限绑 client App）；禁用时由 OpenClaw.app（Onboarding 引导）。不在本模块（本模块只处理 hermes）。

use super::*;

/// hermes 权限预申请：fire-and-forget 后台跑 `hermes computer-use install`（装 cua-driver，幂等）
/// + `permissions grant`（弹 Screen Recording / Accessibility 框，阻塞等用户）。
/// spawn 后不等 → install 命令立即 done，不阻塞命令链（借鉴 hermes web server 把 grant 当后台 action 的模式）。
/// 权限绑 cua-driver，hermes 自带 CLI 走 LaunchServices 启动 cua-driver，TCC 归因正确。
#[cfg(feature = "agent-hermes")]
pub(super) fn request_hermes_permissions(on_log: &(dyn Fn(&str) + Send + Sync)) {
    let bin = hermes::hermes_binary();
    on_log("正在后台请求 hermes 屏幕录制/辅助功能权限，请按系统弹框授权 Screen Recording 与 Accessibility");
    let b = bin.display().to_string();
    let _ = Command::new("sh")
        .arg("-c")
        .arg(format!("{b} computer-use install && {b} computer-use permissions grant"))
        .env("PATH", mac_path())
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn(); // 不 wait：detached 子进程独立装 cua-driver 并弹框；install 命令立即返回 done
}
