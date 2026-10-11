//! 设备信息采集（Win/Mac 共用）：主机名、MAC 地址、平台、OS、架构、App 版本。
//!
//! 在 exchange 时随 deviceCode 一起上报后端，落 aa_client_device 供管理后台审计。

use crate::types::DeviceInfo;

/// App 版本（取共享核心 crate 的 Cargo 包版本，作为 App 版本近似值）。
const APP_VERSION: &str = env!("CARGO_PKG_VERSION");

/// 采集本机设备信息。device_id 由调用方持久化传入（App 首次启动生成并落盘）。
pub fn collect_device_info(device_id: String) -> DeviceInfo {
    let (mac_address, mac_extra) = collect_macs();
    let name = hostname::get()
        .ok()
        .and_then(|h| h.into_string().ok())
        .unwrap_or_default();

    DeviceInfo {
        device_id,
        name,
        platform: platform_str().to_string(),
        os: std::env::consts::OS.to_string(),
        arch: std::env::consts::ARCH.to_string(),
        app_version: APP_VERSION.to_string(),
        mac_address,
        mac_extra,
        ip: String::new(), // IP 由后端从请求取（c.AgentIP），前端不报
    }
}

/// 平台短名：win / mac / 其它。
fn platform_str() -> &'static str {
    #[cfg(target_os = "windows")]
    {
        "win"
    }
    #[cfg(target_os = "macos")]
    {
        "mac"
    }
    #[cfg(not(any(target_os = "windows", target_os = "macos")))]
    {
        std::env::consts::OS
    }
}

/// 取主 MAC + 全部网卡 MAC 列表。取不到时主 MAC 为空、列表为空。
fn collect_macs() -> (String, Vec<String>) {
    match mac_address::get_mac_address() {
        Ok(Some(addr)) => {
            let primary = format!("{}", addr);
            // mac_address crate 只取主网卡；mac_extra 暂仅含主 MAC，避免引入更重依赖。
            // 如需全部网卡，可后续换 sysinfo。
            let extra = vec![primary.clone()];
            (primary, extra)
        }
        _ => (String::new(), Vec::new()),
    }
}
