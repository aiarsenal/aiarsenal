//! system.run 命令（执行 shell 命令，对齐 openclaw node 协议）。
//!
//! params: command(argv 数组)/cwd/env/timeoutMs/needsScreenRecording
//! result: {exitCode, timedOut, success, stdout, stderr, error?}
//! needsScreenRecording=true 时先查 ScreenRecording 权限（不弹框），未授权返 PERMISSION_MISSING。
//! 超时：try_wait 轮询 + child.kill()（避免引入 libc 跨线程 kill）。

use std::collections::HashMap;
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use serde_json::{json, Value};

// 注：system.notify 不用 UNUserNotificationCenter——见 notify() 注释。

/// 执行 system.run。返回 (ok, payload)：ok=false 仅用于命令解析/权限失败；
/// 命令本身跑完（即使非 0 退出或超时）都返回 ok=true + payload 描述结果（对齐 openclaw）。
/// spawn 前过 exec 审批闸门（exec_approvals::gate）：默认 full 档零行为变化。
pub fn run(params: &Value, app: &tauri::AppHandle) -> (bool, Value) {
    // needsScreenRecording：只检查不弹框；未授权返 PERMISSION_MISSING（对齐 MacNodeRuntime validateScreenRecordingIfNeeded）
    let needs_sr = params
        .get("needsScreenRecording")
        .and_then(|v| v.as_bool())
        .unwrap_or(false);
    if needs_sr && !super::permissions::ensure_screen_recording(false) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: screenRecording"}}),
        );
    }

    // command：argv 数组（openclaw 协议）。空则报错。
    let argv: Vec<String> = params
        .get("command")
        .and_then(|c| c.as_array())
        .map(|a| a.iter().filter_map(|x| x.as_str().map(String::from)).collect())
        .unwrap_or_default();
    if argv.is_empty() {
        return (false, json!({"error": {"message": "command 为空"}}));
    }

    let cwd = params.get("cwd").and_then(|c| c.as_str());
    let env: HashMap<String, String> = params
        .get("env")
        .and_then(|e| e.as_object())
        .map(|o| {
            o.iter()
                .filter_map(|(k, v)| v.as_str().map(|s| (k.clone(), s.to_string())))
                .collect()
        })
        .unwrap_or_default();
    let timeout_ms = params.get("timeoutMs").and_then(|t| t.as_u64());
    let gate_log = |msg: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] {msg}"));

    // App 控制闸门（fail-closed，先于 exec 审批——避免注定被拒的命令还弹审批框）：
    // osascript 载荷目标不在「App 权限」清单即拒。拒绝走 ok=true + stderr（网关
    // ok=false 只读帧顶层 error、payload 被吞，agent 看不到原因——见 app_gate.rs）。
    match super::app_gate::gate(&argv) {
        super::app_gate::Verdict::Allow => {}
        super::app_gate::Verdict::Deny(reason) => {
            gate_log(&format!("[app-gate] 拒绝 system.run：{reason}"));
            return (true, super::app_gate::deny_payload(reason));
        }
    }

    // exec 审批闸门：full=放行（默认，零行为变化）/ deny=全拒 / allowlist|ask=白名单匹配，
    // ask 未命中弹窗（60s 超时拒绝）。agent 入口是微信+浏览器（提示注入面），这是 system.run 唯一闸门。
    match super::exec_approvals::gate(app, &argv, cwd, env.get("PATH").map(|s| s.as_str()), &gate_log) {
        super::exec_approvals::GateVerdict::Allow => {}
        super::exec_approvals::GateVerdict::Reject(reason) => {
            return (
                false,
                json!({"error": {"code": "UNAVAILABLE", "message": format!("EXEC_APPROVAL_DENIED: {reason}")}}),
            );
        }
    }

    let mut cmd = Command::new(&argv[0]);
    cmd.args(&argv[1..]);
    if let Some(d) = cwd {
        cmd.current_dir(d);
    }
    for (k, v) in &env {
        cmd.env(k, v);
    }
    cmd.stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            // 启动失败：exitCode=null，success=false，error 描述（对齐 ShellExecutor 行为）
            return (
                true,
                json!({
                    "exitCode": null,
                    "timedOut": false,
                    "success": false,
                    "stdout": "",
                    "stderr": "",
                    "error": format!("启动失败: {e}")
                }),
            );
        }
    };

    // 轮询等待 + 超时 kill（openclaw 超时返回 stdout/stderr 空 + timedOut=true）
    let deadline = timeout_ms.map(|ms| Instant::now() + Duration::from_millis(ms));
    let mut timed_out = false;
    loop {
        match child.try_wait() {
            Ok(Some(_status)) => break, // 退出
            Ok(None) => {
                if let Some(dl) = deadline {
                    if Instant::now() >= dl {
                        let _ = child.kill();
                        timed_out = true;
                        break;
                    }
                }
                thread::sleep(Duration::from_millis(40));
            }
            Err(_) => break,
        }
    }

    // wait_with_output 收 stdout/stderr（kill 后读 pipe：超时分支忽略 output，置空）
    if timed_out {
        let _ = child.wait();
        return (
            true,
            json!({
                "exitCode": null,
                "timedOut": true,
                "success": false,
                "stdout": "",
                "stderr": "",
            }),
        );
    }
    let out = match child.wait_with_output() {
        Ok(o) => o,
        Err(e) => {
            return (
                true,
                json!({
                    "exitCode": null,
                    "timedOut": false,
                    "success": false,
                    "stdout": "",
                    "stderr": "",
                    "error": format!("等待退出失败: {e}")
                }),
            );
        }
    };
    let exit_code = out.status.code();
    let success = exit_code == Some(0);
    (
        true,
        json!({
            "exitCode": exit_code,
            "timedOut": false,
            "success": success,
            "stdout": String::from_utf8_lossy(&out.stdout),
            "stderr": String::from_utf8_lossy(&out.stderr),
        }),
    )
}

/// system.notify：弹系统通知。params: title?/body?  result: {ok:true}（对齐 openclaw）。
///
/// 走 osascript（`display notification`）而非 UNUserNotificationCenter：后者在 tauri dev 裸
/// 二进制（无 CFBundleIdentifier / .app bundle）里 `+currentNotificationCenter` 触发 NSAssert
/// → uncaught ObjC exception → 整进程崩（EXC_BAD_ACCESS，见
/// ~/Library/Logs/DiagnosticReports/aiarsenal-client-mac-*.ips，多次复现于 system.notify）。
/// osascript 经 Script Editor 的 bundle 投递，dev 与打包 .app 都能弹，零 objc 风险。
pub fn notify(params: &Value) -> (bool, Value) {
    let title = params.get("title").and_then(|v| v.as_str()).unwrap_or("");
    let body = params.get("body").and_then(|v| v.as_str()).unwrap_or("");
    let script = format!(
        "display notification {body} with title {title}",
        title = apple_script_string(title),
        body = apple_script_string(body),
    );
    let _ = Command::new("osascript")
        .args(["-e", &script])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
    (true, json!({}))
}

/// 以本 App 名义发系统通知（UNUserNotificationCenter）。与 notify() 的关键差异：
/// osascript 的 `display notification` 归属 Script Editor——本 App 通知权限开不开都
/// 管不到它，横幅会被静默吞；本函数走本 App bundle 投递，受 系统设置→通知→AI Arsenal
/// 控制，权限已开时横幅可见。须 .app 内（裸二进制无 CFBundleIdentifier 会 NSAssert
/// 整进程崩）——dev 场景返回 false，调用方回落 notify()。
pub fn notify_app(title: &str, body: &str) -> bool {
    use std::sync::atomic::{AtomicU64, Ordering};

    use objc2::ClassType;
    use objc2_foundation::NSString;
    use objc2_user_notifications::{
        UNMutableNotificationContent, UNNotificationRequest, UNUserNotificationCenter,
    };

    if !super::permissions::in_app_bundle() {
        return false;
    }
    static SEQ: AtomicU64 = AtomicU64::new(1);
    let id = NSString::from_str(&format!(
        "aa-notify-{}",
        SEQ.fetch_add(1, Ordering::Relaxed)
    ));
    let content = UNMutableNotificationContent::new();
    content.setTitle(&NSString::from_str(title));
    content.setBody(&NSString::from_str(body));
    let request =
        UNNotificationRequest::requestWithIdentifier_content_trigger(&id, content.as_super(), None);
    UNUserNotificationCenter::currentNotificationCenter()
        .addNotificationRequest_withCompletionHandler(&request, None);
    true
}

/// 字符串 → AppleScript 字符串字面量（双引号包裹，转义 `"` 与 `\`）。
/// 喂给 `display notification` / `display dialog`（托盘权限引导对话框也用，permissions.rs）。
pub(super) fn apple_script_string(s: &str) -> String {
    let mut out = String::from("\"");
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            _ => out.push(c),
        }
    }
    out.push('"');
    out
}
