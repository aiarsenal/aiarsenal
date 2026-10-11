//! reminders.list 命令（读提醒，对齐 openclaw node 协议）。
//!
//! params: status?("incomplete"|"completed"|"all"), limit?(50,1-500)
//! result: reminders:[{identifier, title, dueISO?, completed, listName?}]
//!
//! EKEventStore.fetchRemindersMatchingPredicate_completion 异步 block 回调 → mpsc channel 同步等。
//! 三种 predicate：incomplete / completed / all（predicateForRemindersInCalendars）。
//! reminder 的 dueDateComponents 是 NSDateComponents（无时区）→ 拼 "YYYY-MM-DD[THH:MM:SS]"。

#![allow(non_snake_case)]

use std::sync::mpsc;
use std::time::Duration;

use block2::RcBlock;
use objc2_event_kit::{EKEventStore, EKReminder};
use objc2_foundation::{NSArray, NSDateComponents, NSPredicate};
use serde_json::{json, Value};

/// NSDateComponents 未设字段返回 NSDateComponentUndefined（NSInteger 的极大值），
/// 用合理区间判断是否有效（year 1..=9999、month 1..=12、hour 0..=23、minute/second 0..=59）。
unsafe fn components_to_iso(c: &NSDateComponents) -> Option<String> {
    let y = c.year();
    let m = c.month();
    let d = c.day();
    if !(1..=9999).contains(&y) || !(1..=12).contains(&m) || !(1..=31).contains(&d) {
        return None;
    }
    let mut out = format!("{y:04}-{m:02}-{d:02}");
    let h = c.hour();
    let mi = c.minute();
    if (0..=23).contains(&h) && (0..=59).contains(&mi) {
        let s = c.second();
        let ss = if (0..=59).contains(&s) { s } else { 0 };
        out.push_str(&format!("T{h:02}:{mi:02}:{ss:02}"));
    }
    Some(out)
}

unsafe fn extract_reminder(r: &EKReminder) -> Value {
    let identifier = r.calendarItemIdentifier().to_string();
    let title = r.title().to_string();
    let completed = r.isCompleted();
    let due = r
        .dueDateComponents()
        .and_then(|c| unsafe { components_to_iso(&c) });
    let list_name = r.calendar().map(|c| c.title().to_string());
    let mut v = json!({
        "identifier": identifier,
        "title": title,
        "completed": completed,
    });
    if let Some(d) = due {
        v["dueISO"] = json!(d);
    }
    if let Some(ln) = list_name {
        v["listName"] = json!(ln);
    }
    v
}

pub fn list(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_reminders(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: reminders（请到 系统设置→隐私与安全→提醒事项 勾选本 App）"}}),
        );
    }
    let limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(50)
        .clamp(1, 500) as usize;
    let status = params
        .get("status")
        .and_then(|v| v.as_str())
        .unwrap_or("all");

    let store = unsafe { EKEventStore::new() };
    let pred: objc2::rc::Retained<NSPredicate> = unsafe {
        match status {
            "incomplete" => store
                .predicateForIncompleteRemindersWithDueDateStarting_ending_calendars(None, None, None),
            "completed" => store
                .predicateForCompletedRemindersWithCompletionDateStarting_ending_calendars(
                    None, None, None,
                ),
            _ => store.predicateForRemindersInCalendars(None),
        }
    };

    // fetchRemindersMatchingPredicate_completion 异步 → block 内直接 extract 成 Vec<Value>，channel 传回。
    let (tx, rx) = mpsc::channel::<Vec<Value>>();
    let block = RcBlock::new(
        move |rms: *mut NSArray<EKReminder>| {
            let arr = unsafe { &*rms };
            let v: Vec<Value> = arr
                .iter()
                .take(limit)
                .map(|r| unsafe { extract_reminder(&*r) })
                .collect();
            let _ = tx.send(v);
        },
    );
    let _ = unsafe { store.fetchRemindersMatchingPredicate_completion(&pred, &block) };

    let reminders = match rx.recv_timeout(Duration::from_secs(10)) {
        Ok(v) => v,
        Err(_) => {
            return (
                false,
                json!({"error": {"code": "TIMEOUT", "message": "提醒查询超时"}}),
            )
        }
    };
    (true, json!({"reminders": reminders}))
}
