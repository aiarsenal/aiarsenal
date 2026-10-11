//! device.info / device.status（对齐 openclaw node 协议；Windows 原生实现，无权限要求）。
//!
//! 与 mac device.rs 的对外形状逐字段同形，取数路径换 Windows 原生：
//! - deviceName：环境变量 COMPUTERNAME（fallback 注册表
//!   `SYSTEM\CurrentControlSet\Control\ComputerName\ComputerName`；后者是 NetBIOS 名，
//!   与 openclaw 的 hostname 语义一致；mac 是 scutil --get ComputerName 的友好名，
//!   Windows 无同物，如实降级）。
//! - modelIdentifier：`HARDWARE\DESCRIPTION\System\BIOS\SystemProductName`（SMBIOS
//!   System Product，如 "XPS 15 9520"——mac 的 hw.model 同位）。
//! - systemVersion：sysinfo（OS 产品名 + 版本号拼接，如 "Windows 11 Pro 10.0.22631"）。
//! - locale：GetUserDefaultLocaleName（用户 locale，如 "zh-CN"；mac 读 AppleLocale）。
//! - battery：Windows 无跨版本统一电池 API（Win32 GetSystemPowerStatus 只给整型百分比
//!   且对台式机无意义；WinRT Windows.Devices.Power 需要 RoInitialize 生命周期管理，
//!   复杂度不成比例）→ level null / state "unknown"（协议可空，agent 侧按未知处理）。
//! - storage：sysinfo::Disks 汇总所有本地卷（mac 是 df / 根卷；Windows 多卷是常态，
//!   汇总才是「本机存储」的合理口径）。uptime：System::uptime()。thermal：无对应物，nominal。
//!
//! device.permissions / device.health 在 openclaw 是 Android 专用，client 不实现。

use serde_json::{json, Value};

/// device.info：设备基本信息（deviceName/modelIdentifier/systemName/Version/appVersion/build/locale）。
pub fn info() -> (bool, Value) {
    (
        true,
        json!({
            "deviceName": device_name(),
            "modelIdentifier": model_identifier(),
            "systemName": "Windows",
            "systemVersion": system_version(),
            "appVersion": env!("CARGO_PKG_VERSION"),
            "appBuild": env!("CARGO_PKG_VERSION"),
            "locale": user_locale(),
        }),
    )
}

/// device.status：电池 / 存储 / uptime / thermal。
pub fn status() -> (bool, Value) {
    (
        true,
        json!({
            "battery": battery(),
            "storage": storage(),
            "uptimeSeconds": uptime_secs(),
            "thermal": {"state": "nominal"},
        }),
    )
}

// ===== 取数实现 =====

/// 计算机名：COMPUTERNAME 环境变量 → 注册表 NetBIOS 名 → 空串。
/// （注册表段按 target cfg：windows crate 只进 Windows 目标，宿主 check 走环境变量/空串。）
fn device_name() -> String {
    if let Ok(n) = std::env::var("COMPUTERNAME") {
        if !n.trim().is_empty() {
            return n.trim().to_string();
        }
    }
    #[cfg(target_os = "windows")]
    {
        use windows::Win32::System::Registry::HKEY_LOCAL_MACHINE;
        return registry_string(
            HKEY_LOCAL_MACHINE,
            r"SYSTEM\CurrentControlSet\Control\ComputerName\ComputerName",
            "ComputerName",
        )
        .unwrap_or_default();
    }
    #[cfg(not(target_os = "windows"))]
    String::new()
}

/// SMBIOS System Product 名（HARDWARE\DESCRIPTION\System\BIOS；读不到返回空串）。
fn model_identifier() -> String {
    #[cfg(target_os = "windows")]
    {
        use windows::Win32::System::Registry::HKEY_LOCAL_MACHINE;
        return registry_string(
            HKEY_LOCAL_MACHINE,
            r"HARDWARE\DESCRIPTION\System\BIOS",
            "SystemProductName",
        )
        .unwrap_or_default();
    }
    #[cfg(not(target_os = "windows"))]
    String::new()
}

/// OS 名 + 版本（sysinfo 静态查询，如 "Windows 11 Pro 10.0.22631"）。
fn system_version() -> String {
    let name = sysinfo::System::name().unwrap_or_else(|| "Windows".into());
    let ver = sysinfo::System::os_version().unwrap_or_default();
    if ver.is_empty() {
        name
    } else {
        format!("{name} {ver}")
    }
}

/// 用户 locale（GetUserDefaultLocaleName，如 "zh-CN"；失败回退空串）。
fn user_locale() -> String {
    #[cfg(target_os = "windows")]
    {
        use windows::Win32::Globalization::GetUserDefaultLocaleName;
        let mut buf = [0u16; 85]; // LOCALE_NAME_MAX_LENGTH
        let n = unsafe { GetUserDefaultLocaleName(&mut buf) };
        if n <= 0 {
            return String::new();
        }
        return String::from_utf16_lossy(&buf[..(n as usize).min(buf.len())])
            .trim_end_matches('\0')
            .to_string();
    }
    #[cfg(not(target_os = "windows"))]
    String::new()
}

/// 电池：Windows 客户端不实现（level null / state unknown，协议可空）。
fn battery() -> Value {
    json!({
        "level": Value::Null,
        "state": "unknown",
        "lowPowerModeEnabled": false,
    })
}

/// 存储汇总：所有本地卷 total/available 求和（used = total - available）。
fn storage() -> Value {
    let disks = sysinfo::Disks::new_with_refreshed_list();
    let mut total: u64 = 0;
    let mut free: u64 = 0;
    for d in disks.list() {
        total += d.total_space();
        free += d.available_space();
    }
    json!({
        "totalBytes": total,
        "usedBytes": total.saturating_sub(free),
        "freeBytes": free,
    })
}

/// uptime 秒（System::uptime 静态）。
fn uptime_secs() -> f64 {
    sysinfo::System::uptime() as f64
}

// ===== 注册表读取（deviceName fallback / modelIdentifier；仅 Windows 目标编译）=====

/// 读注册表 REG_SZ 字符串值（UTF-16，最多 1KB）。失败（键/值不存在）返回 None。
#[cfg(target_os = "windows")]
fn registry_string(
    root: windows::Win32::System::Registry::HKEY,
    subkey: &str,
    value: &str,
) -> Option<String> {
    use windows::Win32::Foundation::ERROR_SUCCESS;
    use windows::Win32::System::Registry::{
        RegCloseKey, RegOpenKeyExW, RegQueryValueExW, HKEY, KEY_READ, REG_VALUE_TYPE,
    };
    let subkey_w: Vec<u16> = subkey.encode_utf16().chain(std::iter::once(0)).collect();
    let value_w: Vec<u16> = value.encode_utf16().chain(std::iter::once(0)).collect();
    unsafe {
        let mut hkey = HKEY::default();
        let opened = RegOpenKeyExW(
            root,
            windows::core::PCWSTR(subkey_w.as_ptr()),
            None,
            KEY_READ,
            &mut hkey,
        );
        if opened != ERROR_SUCCESS {
            return None;
        }
        let mut buf = [0u8; 1024];
        let mut len: u32 = buf.len() as u32;
        let mut vtype = REG_VALUE_TYPE::default();
        let qr = RegQueryValueExW(
            hkey,
            windows::core::PCWSTR(value_w.as_ptr()),
            None,
            Some(&mut vtype),
            Some(buf.as_mut_ptr()),
            Some(&mut len),
        );
        let out = if qr != ERROR_SUCCESS || len as usize > buf.len() {
            None
        } else {
            // REG_SZ：UTF-16LE，末尾 NUL（len 含 NUL 字节数）
            let pairs: Vec<u16> = buf[..len as usize]
                .chunks_exact(2)
                .map(|c| u16::from_le_bytes([c[0], c[1]]))
                .take_while(|&u| u != 0)
                .collect();
            let s = String::from_utf16_lossy(&pairs).trim().to_string();
            if s.is_empty() {
                None
            } else {
                Some(s)
            }
        };
        let _ = RegCloseKey(hkey);
        out
    }
}
