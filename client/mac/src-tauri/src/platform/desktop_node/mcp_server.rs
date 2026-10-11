//! 本地环回 MCP server：把 desktop_node 能力（system.run / computer.act / screen.snapshot /
//! system.notify）以 MCP 工具暴露给本机 agent —— hermes 经配置里的 mcp_servers.aa-desktop
//! 直连 http://127.0.0.1:43250/mcp（后端 issueHermesConfig 注入该条目）。
//!
//! 为什么：openclaw 的桌面控制走 node 协议（protocol.rs 连 openclaw gateway），闸门
//! （app_gate 的「App 权限」清单 + exec_approvals 审批）在 dispatch 链路里；hermes 不接
//! node 协议，但原生支持 MCP。本 server 复用同一个 dispatch —— 闸门零新写，openclaw /
//! hermes 两个 agent 共享同一套权限边界（后台「App 权限管理」清单即权威）。
//!
//! 协议：MCP streamable HTTP（POST /mcp，JSON-RPC 2.0，application/json 应答）。
//! 无会话（initialize 应答不回 Mcp-Session-Id 头）：mcp python 客户端（hermes 内置 2.0.0）
//! 无 session id 即不建 GET SSE 流、会话终止也不发 DELETE（streamable_http.py 的
//! handle_get_stream / terminate_session 首行判空返回）——POST-only 交互最简且兼容。
//! GET/DELETE 一律 405（spec 允许不提供 SSE 流 / 无会话可终止）。
//! 鉴权：仅绑定 127.0.0.1（同机进程可达，与本机常见本地 MCP server 同信任模型；
//! 后续如需收紧可改客户端生成 token + 写 hermes 配置头）。
//! 端口固定 43250：与 api 侧 issueHermesConfig 注入的 aa-desktop URL 是编译期约定，
//! 改端口须两边同步（api/application/service/client_service.go deviceDesktopMcpURL）。

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};

use serde_json::{json, Value};

use super::protocol;

pub const DESKTOP_MCP_PORT: u16 = 43250;

static STARTED: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

/// 启动本地 MCP server（幂等）。线程常驻；绑定失败（端口被占）只记日志不阻断 App。
pub fn start(app: tauri::AppHandle) {
    if STARTED.swap(true, std::sync::atomic::Ordering::SeqCst) {
        return;
    }
    std::thread::spawn(move || serve(app));
}

fn serve(app: tauri::AppHandle) {
    let log = |m: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] [mcp] {m}"));
    let listener = match TcpListener::bind(("127.0.0.1", DESKTOP_MCP_PORT)) {
        Ok(l) => l,
        Err(e) => {
            log(&format!(
                "⚠️ 绑定 127.0.0.1:{DESKTOP_MCP_PORT} 失败（端口被占？hermes 桌面工具将不可用）: {e}"
            ));
            return;
        }
    };
    log(&format!("本地 MCP server 监听 http://127.0.0.1:{DESKTOP_MCP_PORT}/mcp（aa-desktop）"));
    for stream in listener.incoming() {
        match stream {
            Ok(s) => {
                let app = app.clone();
                std::thread::spawn(move || handle_conn(s, app));
            }
            Err(_) => continue,
        }
    }
}

/// 单连接单请求（应答带 Connection: close；httpx 对 close 会重开新连接，无 keep-alive 需求）。
fn handle_conn(mut stream: TcpStream, app: tauri::AppHandle) {
    // 读超时给宽（osascript 等 TCC 授权框可阻塞 ~2min；命令自身的超时由 system.run 的
    // timeoutMs 管，这里只防连接僵死）。写超时同理。
    let _ = stream.set_read_timeout(Some(std::time::Duration::from_secs(600)));
    let _ = stream.set_write_timeout(Some(std::time::Duration::from_secs(600)));
    let Some(req) = read_request(&mut stream) else {
        return;
    };
    let (status, ctype, body) = if req.method == "POST" && (req.path == "/mcp" || req.path == "/mcp/") {
        match handle_rpc(&req.body, &app) {
            Some((status, body)) => (
                status,
                if body.is_empty() { "text/plain" } else { "application/json" },
                body,
            ),
            None => (400, "application/json", r#"{"error":"bad json-rpc request"}"#.into()),
        }
    } else if req.method == "GET" {
        // 不提供 SSE 流（无会话，客户端本就不会来；来了也明确告知只收 POST）
        (405, "application/json", r#"{"error":"SSE not supported; POST only"}"#.into())
    } else if req.method == "DELETE" {
        (405, "application/json", r#"{"error":"stateless server; no session to terminate"}"#.into())
    } else {
        (404, "application/json", r#"{"error":"not found"}"#.into())
    };
    write_response(&mut stream, status, ctype, &body);
}

// ===== 极简 HTTP/1.1 解析（std 手写：客户端零 HTTP server 依赖，loopback 单端点够用）=====

struct Req {
    method: String,
    path: String,
    body: Vec<u8>,
}

/// 读一个完整请求（头到 \r\n\r\n + Content-Length body）。EOF/超时/超限 → None。
fn read_request(stream: &mut TcpStream) -> Option<Req> {
    let mut buf: Vec<u8> = Vec::new();
    let mut tmp = [0u8; 8192];
    let head_end = loop {
        if let Some(p) = find_subsequence(&buf, b"\r\n\r\n") {
            break p;
        }
        if buf.len() > 64 * 1024 {
            return None; // 头部超限（异常客户端）
        }
        let n = stream.read(&mut tmp).ok()?;
        if n == 0 {
            return None; // EOF（连接关闭/超时）
        }
        buf.extend_from_slice(&tmp[..n]);
    };
    let head = String::from_utf8_lossy(&buf[..head_end]).to_string();
    let mut lines = head.split("\r\n");
    let req_line = lines.next()?;
    let mut parts = req_line.split_whitespace();
    let method = parts.next()?.to_string();
    let path = parts.next()?.to_string();
    let mut content_length = 0usize;
    for line in lines {
        if let Some((k, v)) = line.split_once(':') {
            if k.trim().eq_ignore_ascii_case("content-length") {
                content_length = v.trim().parse().unwrap_or(0);
            }
        }
    }
    if content_length > 8 * 1024 * 1024 {
        return None; // 工具参数不会这么大；防异常占用
    }
    let mut body = buf[head_end + 4..].to_vec();
    while body.len() < content_length {
        let n = stream.read(&mut tmp).ok()?;
        if n == 0 {
            break;
        }
        body.extend_from_slice(&tmp[..n]);
    }
    body.truncate(content_length);
    Some(Req { method, path, body })
}

fn write_response(stream: &mut TcpStream, status: u16, content_type: &str, body: &str) {
    let reason = match status {
        200 => "OK",
        202 => "Accepted",
        400 => "Bad Request",
        404 => "Not Found",
        405 => "Method Not Allowed",
        _ => "OK",
    };
    let resp = format!(
        "HTTP/1.1 {status} {reason}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
    let _ = stream.write_all(resp.as_bytes());
    let _ = stream.flush();
}

fn find_subsequence(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|w| w == needle)
}

// ===== JSON-RPC 2.0 / MCP 方法分发 =====

/// 处理一条 JSON-RPC 消息。返回 (HTTP 状态, 应答体)。notification（无 id）→ (202, "")。
fn handle_rpc(body: &[u8], app: &tauri::AppHandle) -> Option<(u16, String)> {
    let v: Value = serde_json::from_slice(body).ok()?;
    let method = v.get("method").and_then(|m| m.as_str()).unwrap_or("").to_string();
    let id = v.get("id").cloned();
    if id.is_none() {
        // spec：server MUST NOT 应答 notification（notifications/initialized 等）→ 202 空体
        return Some((202, String::new()));
    }
    let outcome = match method.as_str() {
        "initialize" => Ok(json!({
            // 回显客户端请求的协议版本（未知/缺失用 2025-06-18）；客户端校验的是自己发的版本
            "protocolVersion": v
                .pointer("/params/protocolVersion")
                .and_then(|p| p.as_str())
                .unwrap_or("2025-06-18"),
            "capabilities": { "tools": { "listChanged": false } },
            "serverInfo": { "name": "aiarsenal-desktop", "version": "0.1.0" },
        })),
        "tools/list" => Ok(json!({ "tools": tools_schema() })),
        "tools/call" => Ok(call_tool(&v, app)),
        "ping" => Ok(json!({})),
        other => Err(json!({ "code": -32601, "message": format!("method not found: {other}") })),
    };
    let body = match outcome {
        Ok(result) => json!({ "jsonrpc": "2.0", "id": id.unwrap(), "result": result }).to_string(),
        Err(error) => json!({ "jsonrpc": "2.0", "id": id.unwrap(), "error": error }).to_string(),
    };
    Some((200, body))
}

/// tools/call → 复用 protocol::dispatch（app_gate / exec_approvals / act_gate 全在同一链路）。
fn call_tool(v: &Value, app: &tauri::AppHandle) -> Value {
    let name = v.pointer("/params/name").and_then(|n| n.as_str()).unwrap_or("");
    let args = v.pointer("/params/arguments").cloned().unwrap_or(json!({}));
    let log = |m: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] [mcp] {m}"));
    log(&format!("tools/call {name}: {args}"));
    let (ok, payload) = match name {
        "desktop_run" => protocol::dispatch("system.run", &args, app, &log),
        "desktop_act" => protocol::dispatch("computer.act", &args, app, &log),
        "desktop_snapshot" => protocol::dispatch("screen.snapshot", &args, app, &log),
        "desktop_notify" => protocol::dispatch("system.notify", &args, app, &log),
        other => (false, json!({ "error": { "message": format!("未知工具: {other}") } })),
    };
    // isError：ok=false（闸门/解析拒绝）或 payload 自述失败（success=false / error 段）。
    // 拒绝原因在 payload 文本里原样给 agent（app_gate 的 deny_payload 语义），模型可读因调整。
    let is_error = !ok
        || payload.get("error").is_some()
        || payload.get("success") == Some(&Value::Bool(false));
    json!({
        "content": [{
            "type": "text",
            "text": serde_json::to_string_pretty(&payload).unwrap_or_default(),
        }],
        "isError": is_error,
    })
}

/// 工具目录（描述给 LLM 看，权限语义写明——模型才知道被拒时怎么应对）。
fn tools_schema() -> Value {
    json!([
        {
            "name": "desktop_run",
            "description": "在本机执行 shell 命令（argv 数组）。操作 macOS 应用：用 osascript -e 'tell application \"<应用名>\" …'（AppleScript）或 open -a \"<应用名>\" 拉起。目标应用必须在管理端配置的「App 权限」清单内，否则被拒（换清单内应用，或把被拒原因转告用户）。普通 shell 命令（ls/curl 等）不受清单限制。需要看屏幕结果的场景配合 desktop_snapshot。",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "argv 数组，如 [\"osascript\", \"-e\", \"tell application \\\"备忘录\\\" to activate\"]"
                    },
                    "cwd": { "type": "string", "description": "工作目录（可省）" },
                    "env": { "type": "object", "description": "环境变量（可省）" },
                    "timeoutMs": { "type": "integer", "description": "超时毫秒；AppleScript 等待用户操作时给足（如 30000）" },
                    "needsScreenRecording": { "type": "boolean", "description": "命令需要屏幕录制权限时置 true（先校验再执行）" }
                },
                "required": ["command"]
            }
        },
        {
            "name": "desktop_act",
            "description": "鼠标 / 键盘控制（点击、移动、输入、按键、滚动、拖拽、等待）。注入类动作（click/type/key/scroll 等）要求当前前台应用在「App 权限」清单内；mouse_move / wait 不受限。坐标先经 desktop_snapshot 确认。",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "action": {
                        "type": "string",
                        "enum": ["left_click", "right_click", "middle_click", "double_click", "triple_click",
                                 "mouse_move", "left_click_drag", "left_mouse_down", "left_mouse_up",
                                 "scroll", "type", "key", "hold_key", "wait"],
                        "description": "动作类型"
                    },
                    "x": { "type": "number", "description": "目标 X（屏幕坐标）" },
                    "y": { "type": "number", "description": "目标 Y（屏幕坐标）" },
                    "text": { "type": "string", "description": "type：要输入的文本" },
                    "keys": { "type": "string", "description": "key/hold_key：组合键，如 \"cmd+c\"（macOS 修饰键 cmd/ctrl/alt/shift）" },
                    "fromX": { "type": "number", "description": "left_click_drag：起点 X" },
                    "fromY": { "type": "number", "description": "left_click_drag：起点 Y" },
                    "scrollDirection": { "type": "string", "enum": ["up", "down"], "description": "scroll：方向" },
                    "scrollAmount": { "type": "integer", "description": "scroll：格数（默认 3）" },
                    "durationMs": { "type": "integer", "description": "hold_key/wait：持续毫秒" }
                },
                "required": ["action"]
            }
        },
        {
            "name": "desktop_snapshot",
            "description": "截取屏幕（返回 base64 PNG + 宽高），用于看屏幕现状、确认操作结果。操作前先截屏定位坐标。",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "screenIndex": { "type": "integer", "description": "屏幕序号（主屏 0，可省）" }
                }
            }
        },
        {
            "name": "desktop_notify",
            "description": "弹一条 macOS 系统通知（提醒用户任务完成 / 需要注意）。",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "title": { "type": "string", "description": "标题" },
                    "body": { "type": "string", "description": "正文" }
                }
            }
        }
    ])
}
