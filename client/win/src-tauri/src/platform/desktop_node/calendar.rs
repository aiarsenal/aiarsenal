//! calendar.events 命令（读日历事件，对齐 openclaw node 协议）。
//!
//! params: startISO?, endISO?, limit?(50,1-500)
//! result: events:[{identifier, title, startISO, endISO, isAllDay, location?, calendarTitle?}]
//!
//! Windows 客户端暂不支持：本机日历读取要走 WinRT Windows.ApplicationModel.Appointments
//!（AppointmentStore，需 RoInitialize/异步等待）或解析 .ics 文件——复杂度不成比例，v1
//! 不做。结构对齐 mac：能力开关闸门 → 参数校验（与 mac 逐条同序同错文）→ 结构化
//! UNAVAILABLE（agent 可读可降级）。开关关着时报 PERMISSION_MISSING（与 mac 未授权
//! 错误同形，agent 降级逻辑一致）。

use serde_json::{json, Value};

pub fn events(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_calendar(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: calendar（日历能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    // 参数校验与 mac 同序同错文（无效参数两个平台报同样的错）
    let _limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(50)
        .clamp(1, 500) as usize;

    let now_secs = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs_f64())
        .unwrap_or(0.0);
    let end_secs = params
        .get("endISO")
        .and_then(|v| v.as_str())
        .and_then(super::timeutil::iso_to_epoch)
        .unwrap_or(now_secs);
    let start_secs = params
        .get("startISO")
        .and_then(|v| v.as_str())
        .and_then(super::timeutil::iso_to_epoch)
        .unwrap_or(end_secs - 30.0 * 86400.0);
    if start_secs >= end_secs {
        return (
            false,
            json!({"error": {"message": "startISO 必须早于 endISO"}}),
        );
    }

    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "calendar.events 在 Windows 客户端暂不支持（无日历存储访问实现）"}}),
    )
}
