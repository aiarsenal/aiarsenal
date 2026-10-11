//! system.run 命令（执行 shell 命令，对齐 openclaw node 协议）。
//!
//! params: command(argv 数组)/cwd/env/timeoutMs/needsScreenRecording
//! result: {exitCode, timedOut, success, stdout, stderr, error?}
//! needsScreenRecording=true 时先查客户端截屏能力开关（不弹框），关着返 PERMISSION_MISSING。
//! 超时：try_wait 轮询 + child.kill()。
//!
//! Windows 差异（对 mac system.rs）：
//! - 子进程一律 CREATE_NO_WINDOW：App 是 GUI 子系统，给控制台子进程新开黑窗会随命令
//!   弹出一闪而过的控制台窗口（同 platform.rs cmd_new 的既有结论）。
//! - system.notify 走 PowerShell NotifyIcon 气泡（System.Windows.Forms），不走
//!   UNUserNotificationCenter/objc2（mac 专属）；通知中心归属与 mac 无对应物，dev/打包一致。
//! - notify_app 在 Windows 无「本 App bundle 专属通知通道」的区分（气泡由本进程投递，
//!   PowerShell 通知无独立权限开关），直接返回 true——调用方语义（可发）保持不变。

use std::collections::HashMap;
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use serde_json::{json, Value};

/// Windows 子进程构造（CREATE_NO_WINDOW）。非 Windows 目标（宿主 macOS 交叉 check）
/// 无 std::os::windows，cfg 隔离让类型校验也能编译（行为只在真实 Windows 目标生效）。
fn win_command(program: &str) -> Command {
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        let mut c = Command::new(program);
        c.creation_flags(CREATE_NO_WINDOW);
        c
    }
    #[cfg(not(target_os = "windows"))]
    {
        Command::new(program)
    }
}

/// 执行 system.run。返回 (ok, payload)：ok=false 仅用于命令解析/权限失败；
/// 命令本身跑完（即使非 0 退出或超时）都返回 ok=true + payload 描述结果（对齐 openclaw）。
/// spawn 前过 exec 审批闸门（exec_approvals::gate）：默认 full 档零行为变化。
pub fn run(params: &Value, app: &tauri::AppHandle) -> (bool, Value) {
    // needsScreenRecording：只检查不弹框；关着返 PERMISSION_MISSING（对齐 MacNodeRuntime validateScreenRecordingIfNeeded）
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
    // cmd start / powershell Start-Process 等载荷目标不在「App 权限」清单即拒。拒绝走
    // ok=true + deny payload（网关 ok=false 只读帧顶层 error、payload 被吞，agent 看不到
    // 原因——见 app_gate.rs）。
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

    let mut cmd = win_command(&argv[0]);
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
    // 已知局限：控制台子进程输出按系统代码页（如 GBK）编码时，非 ASCII 段会以 U+FFFD 呈现
    //（未引入编码转换依赖；agent 主要消费 ASCII 工具输出，需要精确中文输出时命令内自转 UTF-8，
    // 如 powershell -Command "[Console]::OutputEncoding=[Text.Encoding]::UTF8; ..."）。
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
/// Windows 实现：PowerShell NotifyIcon 气泡（System.Windows.Forms）。不用 tauri 通知插件
///（desktop_node 不引额外插件依赖）也不用 Win32 Shell_NotifyIcon 原生绑定（要消息泵/窗口
/// 类注册，复杂度不成比例）——PowerShell 一次性进程投递，dev 与打包一致，崩溃也不影响本进程。
pub fn notify(params: &Value) -> (bool, Value) {
    let title = params.get("title").and_then(|v| v.as_str()).unwrap_or("");
    let body = params.get("body").and_then(|v| v.as_str()).unwrap_or("");
    let script = format!(
        "Add-Type -AssemblyName System.Windows.Forms; \
         Add-Type -AssemblyName System.Drawing; \
         $n = New-Object System.Windows.Forms.NotifyIcon; \
         $n.Icon = [System.Drawing.SystemIcons]::Information; \
         $n.Visible = $true; \
         $n.ShowBalloonTip(5000, {}, {}, [System.Drawing.ToolTipIcon]::None); \
         Start-Sleep -Seconds 6; \
         $n.Dispose()",
        ps_single_quote(title),
        ps_single_quote(body),
    );
    let _ = win_command("powershell")
        .args(["-NoProfile", "-NonInteractive", "-Command", &script])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
    (true, json!({}))
}

/// 以本 App 名义发系统通知（app_perm::notify_removed 用）。Windows 上气泡本就由本进程
/// 发起、无独立通知权限开关，与 notify() 无归属差异——直接成功返回 true（mac 的
/// in_app_bundle()/UNUserNotificationCenter 语义在 Windows 无对应物，文档化偏离）。
pub fn notify_app(title: &str, body: &str) -> bool {
    let (ok, _) = notify(&json!({ "title": title, "body": body }));
    ok
}

/// 字符串 → PowerShell 单引号字面量（单引号包裹，内部 `'` 翻倍转义）。
/// 喂给 `ShowBalloonTip` / Add-Type 脚本参数（与 mac 的 apple_script_string 同位helper）。
pub(super) fn ps_single_quote(s: &str) -> String {
    format!("'{}'", s.replace('\'', "''"))
}
