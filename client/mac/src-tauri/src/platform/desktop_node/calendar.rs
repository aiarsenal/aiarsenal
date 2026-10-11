//! calendar.events 命令（读日历事件，对齐 openclaw node 协议）。
//!
//! params: startISO?, endISO?, limit?(50,1-500)
//! result: events:[{identifier, title, startISO, endISO, isAllDay, location?, calendarTitle?}]
//!
//! EKEventStore.eventsMatchingPredicate 同步返 NSArray<EKEvent>（fetch 用 predicate 限定时间窗）。
//! NSDate <-> ISO 走 super::timeutil。

#![allow(non_snake_case)]

use objc2_event_kit::{EKEvent, EKEventStore};
use objc2_foundation::{NSArray, NSDate, NSPredicate};
use serde_json::{json, Value};

use super::timeutil::{epoch_to_iso, iso_to_epoch};

/// 单个 EKEvent → JSON（getter 全 unsafe）。
unsafe fn extract_event(e: &EKEvent) -> Value {
    let identifier = e
        .eventIdentifier()
        .map(|s| s.to_string())
        .unwrap_or_default();
    let title = e.title().to_string();
    let start_secs: f64 = e.startDate().timeIntervalSince1970();
    let end_secs: f64 = e.endDate().timeIntervalSince1970();
    let is_all_day = e.isAllDay();
    let location = e.location().map(|s| s.to_string());
    let calendar_title = e.calendar().map(|c| c.title().to_string());
    let mut v = json!({
        "identifier": identifier,
        "title": title,
        "startISO": epoch_to_iso(start_secs),
        "endISO": epoch_to_iso(end_secs),
        "isAllDay": is_all_day,
    });
    if let Some(loc) = location {
        v["location"] = json!(loc);
    }
    if let Some(ct) = calendar_title {
        v["calendarTitle"] = json!(ct);
    }
    v
}

pub fn events(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_calendar(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: calendar（请到 系统设置→隐私与安全→日历 勾选本 App）"}}),
        );
    }
    let limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(50)
        .clamp(1, 500) as usize;

    // 时间窗：endISO 默认 now，startISO 默认 end-30d（openclaw 调用方常只给 limit 或不给）。
    let now_secs = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs_f64())
        .unwrap_or(0.0);
    let end_secs = params
        .get("endISO")
        .and_then(|v| v.as_str())
        .and_then(iso_to_epoch)
        .unwrap_or(now_secs);
    let start_secs = params
        .get("startISO")
        .and_then(|v| v.as_str())
        .and_then(iso_to_epoch)
        .unwrap_or(end_secs - 30.0 * 86400.0);
    if start_secs >= end_secs {
        return (
            false,
            json!({"error": {"message": "startISO 必须早于 endISO"}}),
        );
    }

    let store = unsafe { EKEventStore::new() };
    let start = NSDate::dateWithTimeIntervalSince1970(start_secs);
    let end = NSDate::dateWithTimeIntervalSince1970(end_secs);
    let pred: objc2::rc::Retained<NSPredicate> = unsafe {
        store.predicateForEventsWithStartDate_endDate_calendars(&start, &end, None)
    };
    let arr: objc2::rc::Retained<NSArray<EKEvent>> =
        unsafe { store.eventsMatchingPredicate(&pred) };
    let events: Vec<Value> = arr
        .iter()
        .take(limit)
        .map(|e| unsafe { extract_event(&e) })
        .collect();
    (true, json!({"events": events}))
}
