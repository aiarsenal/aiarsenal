//! reminders.list 命令（读提醒，对齐 openclaw node 协议）。
//!
//! params: status?("incomplete"|"completed"|"all"), limit?(50,1-500)
//! result: reminders:[{identifier, title, dueISO?, completed, listName?}]
//!
//! Windows 客户端暂不支持：本机提醒读取要走 WinRT Windows.ApplicationModel.UserTasks
//! / Microsoft.ToDo 的私有接口（无公开系统级提醒存储；Outlook 数据在 MAPI 层）——
//! 无可用的公开实现路径，v1 不做。结构对齐 mac：能力开关闸门 → 参数归一 → 结构化
//! UNAVAILABLE。开关关着时报 PERMISSION_MISSING（与 mac 未授权错误同形）。

use serde_json::{json, Value};

pub fn list(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_reminders(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: reminders（提醒事项能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let _limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(50)
        .clamp(1, 500) as usize;
    let _status = params
        .get("status")
        .and_then(|v| v.as_str())
        .unwrap_or("all");

    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "reminders.list 在 Windows 客户端暂不支持（无系统级提醒存储访问实现）"}}),
    )
}
