//! device.info / device.status（对齐 openclaw node 协议；shell 获取，无 TCC 权限）。
//!
//! device.permissions / device.health 在 openclaw 是 Android 专用（macOS 无对应），client 不实现。

use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

fn shell(bin: &str, args: &[&str]) -> String {
    Command::new(bin)
        .args(args)
        .output()
        .ok()
        .and_then(|o| String::from_utf8(o.stdout).ok())
        .map(|s| s.trim().to_string())
        .unwrap_or_default()
}

/// device.info：设备基本信息（deviceName/modelIdentifier/systemName/Version/appVersion/build/locale）。
pub fn info() -> (bool, Value) {
    (
        true,
        json!({
            "deviceName": shell("scutil", &["--get", "ComputerName"]),
            "modelIdentifier": shell("sysctl", &["-n", "hw.model"]),
            "systemName": "macOS",
            "systemVersion": shell("sw_vers", &["-productVersion"]),
            "appVersion": env!("CARGO_PKG_VERSION"),
            "appBuild": env!("CARGO_PKG_VERSION"),
            "locale": shell("defaults", &["read", "-g", "AppleLocale"]),
        }),
    )
}

/// device.status：电池 / 存储 / uptime（shell）；thermal/network 简化（IOKit thermal 略，默认 nominal）。
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

/// pmset -g batt 解析：level（0-1）/ state（charging|full|unplugged|unknown）。
fn battery() -> Value {
    let out = shell("pmset", &["-g", "batt"]);
    // 形如 "... -InternalBattery-0 (id1) 45%; discharging; ..."
    let level = out
        .split('%')
        .next()
        .and_then(|s| s.rsplit(|c: char| !c.is_ascii_digit()).next())
        .and_then(|n| n.parse::<f64>().ok())
        .map(|v| v / 100.0);
    let state = if out.contains("charging") {
        "charging"
    } else if out.contains("AC Power") || out.contains("AC attached") {
        "full"
    } else if level.is_some() {
        "unplugged"
    } else {
        "unknown"
    };
    json!({
        "level": level,
        "state": state,
        "lowPowerModeEnabled": false,
    })
}

/// df -k / 解析：total/used/free bytes。
fn storage() -> Value {
    let out = shell("df", &["-k", "/"]);
    let nums: Vec<u64> = out
        .lines()
        .nth(1)
        .map(|l| l.split_whitespace().filter_map(|n| n.parse::<u64>().ok()).collect())
        .unwrap_or_default();
    // df -k 列：Filesystem 1K-blocks Used Available Use% MountedOn
    if nums.len() >= 4 {
        json!({
            "totalBytes": nums[1] * 1024,
            "usedBytes": nums[2] * 1024,
            "freeBytes": nums[3] * 1024,
        })
    } else {
        json!({"totalBytes": 0, "usedBytes": 0, "freeBytes": 0})
    }
}

/// sysctl -n kern.boottime 解析开机时间 → now - boot = uptime 秒。
fn uptime_secs() -> f64 {
    let out = shell("sysctl", &["-n", "kern.boottime"]);
    // 形如 "{ sec = 1690000000, usec = 0 } ..."
    let boot = out
        .split("sec =")
        .nth(1)
        .and_then(|s| s.split(',').next())
        .and_then(|s| s.trim().parse::<u64>().ok());
    match (boot, SystemTime::now().duration_since(UNIX_EPOCH)) {
        (Some(b), Ok(n)) => n.as_secs() as f64 - b as f64,
        _ => 0.0,
    }
}
