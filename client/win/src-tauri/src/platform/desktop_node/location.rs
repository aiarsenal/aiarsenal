//! location.get 命令（读定位，对齐 openclaw node 协议）。
//!
//! params: timeoutMs?(10000) — mac 还有权限等待/精确度等参数，Windows 全部忽略。
//! result: {lat, lon, altitudeMeters, timestamp(ISO), source, accuracyMeters?, speedMps?, headingDeg?}
//!
//! Windows 客户端暂不支持：系统定位要走 WinRT Windows.Devices.Geolocation.Geolocator
//!（RoInitialize + IAsyncOperation 异步泵；tauri 的 WS 线程没有 STA 泵，同步等要起
//! STA 线程 + 消息循环），复杂度不成比例，v1 不做。结构对齐 mac：能力开关闸门 →
//! 结构化 UNAVAILABLE。开关关着时报 PERMISSION_MISSING（与 mac 未授权错误同形），
//! agent 降级逻辑两侧一致。

use serde_json::{json, Value};

pub fn get(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_location(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: location（定位能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let _timeout_ms = params
        .get("timeoutMs")
        .and_then(|v| v.as_u64())
        .unwrap_or(10000);
    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "location.get 在 Windows 客户端暂不支持（WinRT Geolocator 未接入）"}}),
    )
}
