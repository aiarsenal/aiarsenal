//! openclaw desktop node 的 node 协议（连本地 gateway，暴露 screen.snapshot/computer.act）。
//!
//! WebSocket（tungstenite 同步）+ Ed25519 设备身份 + challenge 鉴权 + token 鉴权 + node.invoke 收发。
//! 本地 127.0.0.1 连接也强制 token 鉴权（gateway auth.mode=token）：client 读 ~/.openclaw/openclaw.json
//! 的 gateway.auth.token，填入 connect 帧 auth.token + 签名 payload 的 token 位。
//!
//! 协议规范来自 openclaw gateway-client/protocol 源码（核对 openclaw-main + 实测 gateway 2026.7.1 dist），
//! 与 mac 客户端同构（mac 版见 client/mac/.../desktop_node/protocol.rs，仅平台标识与权限文案不同）：
//! - 握手：connect.challenge(nonce) → connect(Ed25519 签名 v3 payload + auth.token) → {type:res,ok:true,payload:{type:hello-ok}}
//! - 协议版本：minProtocol=3(MIN_NODE_PROTOCOL_VERSION) ~ maxProtocol=4(PROTOCOL_VERSION)（version.ts）
//! - 签名 payload v3（device-auth.ts buildDeviceAuthPayloadV3）：`v3|{deviceId}|node-host|node|node||{signedAtMs}|{token}|{nonce}|{platform}|{deviceFamily}`
//! - Windows 平台标识：platform "windows" / deviceFamily "Windows"（gateway 侧 normalize 小写后比对，
//!   须与 connect 帧 client.platform / client.deviceFamily 逐字段一致）。
//!   ⚠️ gateway node-command-policy 的 PLATFORM_DEFAULTS 目前只内置 "macos" 的命令 allowlist，
//!   "windows" 命中默认集失败时命令会被 "not in the allowlist" 拦——兜底路径是把命令写进
//!   gateway.nodes.allowCommands（platform::ensure_canvas_allow_commands 已对自研命令做合并写入；
//!   若后端补齐 windows 平台默认集则无需任何额外配置）。
//! - connect 帧：{type:req, method:connect, params:{minProtocol,maxProtocol,auth:{token},client,caps,commands,role,scopes,device}}
//! - node.invoke.request(event) → 执行 → node.invoke.result(req method，params 含 nodeId=本 node deviceId)
//! - device identity：Ed25519，deviceId=sha256(publicKey) hex，独立文件 ~/.openclaw/identity/client-node.json
//!   （不与 gateway 的 device.json 共享：gateway 存 PEM v1、client 存 raw b64，共享会互相读不懂→各自重建→deviceId 漂移→握手断续失败）。
//! - 验签自洽：gateway 用 connect 帧字段重算 payload 比对签名，故 connect 帧字段须与 payload 一致。

use std::fs;
use std::time::{SystemTime, UNIX_EPOCH};

use base64::Engine;
use ed25519_dalek::{Signer, SigningKey};
use rand::RngCore;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use tungstenite::{connect, Message};
use url::Url;

use super::{browser, calendar, camera, canvas, contacts, control, device, exec_approvals, location, photos, reminders, screen, system};

const GATEWAY_URL: &str = "ws://127.0.0.1:18789";

/// node 主循环：连 gateway + 握手 + 收 node.invoke 执行。阻塞，应在独立线程跑。
/// 进度走 aiarsenal_client_core::util::log（线程安全；不接 on_log，因线程超出 install 命令生命周期）。
///
/// `app`：AppHandle 透传给 dispatch（exec 审批弹窗 / canvas 窗口要拉主窗口、emit 事件）。
///
/// 重连：device 身份 load 一次跨重连复用；session（connect→握手→invoke）任何一步失败或
/// 运行中断开 → 等 3s 重试，直到连上。根治 gateway/client 启动顺序竞态（gateway 没起好时
/// 连不上，旧逻辑连一次失败线程就死，永不复活；现在会一直重试到 gateway 起来）。
pub fn run(app: tauri::AppHandle) {
    let log = |msg: &str| aiarsenal_client_core::util::log("info", &format!("[desktop_node] {msg}"));
    let device = match load_or_create_device() {
        Ok(d) => d,
        Err(e) => {
            // device identity 都建不出来（极少见：无 HOME / 磁盘满）→ 没法跑，不重试。
            log(&format!("device identity 加载失败，desktop node 无法运行: {e}"));
            return;
        }
    };
    log(&format!("device id: {}", device.device_id));
    loop {
        match run_session(&device, &app, &log) {
            Ok(()) => log("session 正常结束（invoke loop 不应主动退出），3s 后重连..."),
            Err(e) => log(&format!("断开: {e}，3s 后重连 gateway...")),
        }
        std::thread::sleep(std::time::Duration::from_secs(3));
    }
}

/// 单次 session：连 gateway → challenge → connect(签名) → hello-ok → invoke loop。
/// connect/握手/invoke 任一步失败或 socket 断开 → 返回 Err，由上层 run() 重连。
fn run_session(device: &Device, app: &tauri::AppHandle, log: &impl Fn(&str)) -> Result<(), String> {
    let url = Url::parse(GATEWAY_URL).map_err(|e| format!("url parse: {e}"))?;
    let (mut socket, _resp) =
        connect(url.as_str()).map_err(|e| format!("ws connect 失败（gateway 没跑？{e}）"))?;
    log("已连 gateway");

    // 1) 收 connect.challenge → 取 nonce
    let nonce = loop {
        let msg = socket.read().map_err(|e| format!("read challenge: {e}"))?;
        if let Message::Text(t) = msg {
            let v: Value = serde_json::from_str(&t).map_err(|e| format!("parse: {e}"))?;
            if v.get("event").and_then(|e| e.as_str()) == Some("connect.challenge") {
                if let Some(n) = v.pointer("/payload/nonce").and_then(|n| n.as_str()) {
                    break n.to_string();
                }
            }
        }
    };
    log("收到 challenge，签名回应");

    // 2) 发 connect（Ed25519 签名 v3 payload）。payload 格式须与 gateway buildDeviceAuthPayloadV3
    //    逐字段一致：v3|deviceId|clientId|clientMode|role|scopes|signedAtMs|token|nonce|platform|deviceFamily。
    //    token 位 = connect 帧 auth.token（gateway 用 connectParams.auth.token 重算 payload 验签，
    //    两边必须同值；留空 → DEVICE_AUTH_SIGNATURE_INVALID）。
    let gateway_token = load_gateway_token();
    if gateway_token.is_none() {
        log("⚠️ 未读到 gateway token（~/.openclaw/openclaw.json gateway.auth.token），token 鉴权下握手会失败");
    }
    let token_for_payload = gateway_token.clone().unwrap_or_default();
    let signed_at_ms = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|e| format!("time: {e}"))?
        .as_millis() as u64;
    // 签名 payload v3（device-auth.ts:45-65）：v3|deviceId|clientId|clientMode|role|scopes|signedAtMs|token|nonce|platform|deviceFamily。
    // gateway 侧 normalize 小写后比对，故 payload platform/deviceFamily 须与 connect 帧一致。
    // Windows 平台标识用 "windows|Windows"（见模块头：gateway PLATFORM_DEFAULTS 暂无 windows 条目，
    // 命令默认集靠 gateway.nodes.allowCommands 兜底；后端补齐 windows 默认集后此处无需改动）。
    let payload = format!(
        "v3|{}|node-host|node|node||{}|{}|{}|windows|Windows",
        device.device_id, signed_at_ms, token_for_payload, nonce
    );
    let sig = device.signing_key.sign(payload.as_bytes());
    let sig_b64 = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(sig.to_bytes());
    let pub_b64 = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(device.public_key_raw);

    let connect_frame = json!({
        "type": "req",
        "id": rand_id(),
        "method": "connect",
        "params": {
            "minProtocol": 3,
            "maxProtocol": 4,
            "auth": {
                "token": gateway_token.clone().unwrap_or_default()
            },
            "client": {
                "id": "node-host",
                "displayName": "AIArsenal Client",
                "version": "0.1.0",
                "platform": "windows",
                "deviceFamily": "Windows",
                "mode": "node"
            },
            "caps": ["system", "screen", "computer", "browser", "location", "contacts", "calendar", "photos", "camera", "canvas"],
            // system.execApprovals.* / browser.proxy 在 gateway 的 DESKTOP_HOST_COMMANDS（connect 帧声明即放行）；
            // canvas.* 等非平台默认集，须 gateway.nodes.allowCommands 枚举（ensure_canvas_allow_commands 写入）。
            // 新命令声明 → gateway 建 pending pairing → 握手后自动批准（见 hello-ok 分支）→
            // updateSurface 原地刷新命令集（无需 nodes remove 重配）。
            "commands": ["screen.snapshot", "screen.record", "computer.act", "system.run", "system.notify", "browser.proxy", "location.get", "contacts.search", "calendar.events", "reminders.list", "photos.latest", "device.info", "device.status", "camera.list", "camera.snap", "camera.clip", "system.execApprovals.get", "system.execApprovals.set", "canvas.present", "canvas.hide", "canvas.navigate", "canvas.eval", "canvas.snapshot", "canvas.a2ui.push", "canvas.a2ui.pushJSONL", "canvas.a2ui.reset"],
            "role": "node",
            "scopes": [],
            "device": {
                "id": device.device_id,
                "publicKey": pub_b64,
                "signature": sig_b64,
                "signedAt": signed_at_ms,
                "nonce": nonce
            }
        }
    });
    socket
        .send(Message::Text(connect_frame.to_string()))
        .map_err(|e| format!("send connect: {e}"))?;

    // 3) 收 hello-ok（或 error）
    loop {
        let msg = socket.read().map_err(|e| format!("read hello: {e}"))?;
        if let Message::Text(t) = msg {
            let v: Value = serde_json::from_str(&t).unwrap_or_default();
            let ty = v.get("type").and_then(|t| t.as_str()).unwrap_or("");
            let ok_flag = v.get("ok").and_then(|o| o.as_bool());
            // gateway 成功响应：{type:"res", ok:true, payload:{type:"hello-ok",...}}
            // （只认顶层 type:"hello-ok" 会漏掉 res 包装 → 永远阻塞在 read）
            if ty == "hello-ok" || (ty == "res" && ok_flag == Some(true)) {
                log("握手成功（hello-ok），node 已注册");
                // canvas a2ui 宿主页地址：gateway 在 hello-ok payload.pluginSurfaceUrls 下发
                // （scoped url，message-handler.ts）——canvas.a2ui.* 靠它定位宿主页。
                if let Some(url) = v
                    .get("payload")
                    .and_then(|p| p.get("pluginSurfaceUrls"))
                    .and_then(|s| s.get("canvas"))
                    .and_then(|u| u.as_str())
                {
                    canvas::set_surface_url(url);
                    log(&format!("canvas surface url 已记录: {url}"));
                }
                // 每次握手后都查 pending pairing 并自动批准自己的 node：连上声明新命令
                // （如 canvas.*）→ gateway 建新 pending → 此处批掉命令才路由得进来。
                // （旧逻辑只在启动时跑一次，升级 connect 帧后存量连接拿不到新命令。）
                std::thread::spawn(crate::platform::approve_own_node_if_pending);
                break;
            }
            if ty == "error" || v.get("error").is_some() || (ty == "res" && ok_flag == Some(false)) {
                return Err(format!("握手失败: {v}"));
            }
        }
    }

    // 4) invoke loop：收 node.invoke.request → 分发 screen/control → 回 result
    loop {
        let msg = socket.read().map_err(|e| format!("ws read: {e}"))?;
        let Message::Text(t) = msg else {
            continue;
        };
        let v: Value = match serde_json::from_str(&t) {
            Ok(v) => v,
            Err(_) => continue,
        };
        if v.get("event").and_then(|e| e.as_str()) != Some("node.invoke.request") {
            continue;
        }
        let Some(p) = v.get("payload") else {
            continue;
        };
        let req_id = p.get("id").and_then(|i| i.as_str()).unwrap_or("").to_string();
        let command = p.get("command").and_then(|c| c.as_str()).unwrap_or("").to_string();
        let params_json = p.get("paramsJSON").and_then(|x| x.as_str()).unwrap_or("{}");
        let params: Value = serde_json::from_str(params_json).unwrap_or(json!({}));

        log(&format!("收命令: {command}"));
        let (ok, result) = dispatch(&command, &params, app, &log);

        // node.invoke.result 是 RPC method（非 event）：nodes.handlers.invoke-result.ts:31-78。
        // gateway 校验 params.nodeId === connect device.id（callerNodeId），缺/不匹配 → INVALID_REQUEST。
        let mut frame_params = json!({
            "id": req_id,
            "nodeId": device.device_id,
            "ok": ok,
            "payload": result
        });
        if !ok {
            // 网关只读帧**顶层** error 拼 "<code>: <message>"（nodes.helpers.ts:54-60），
            // payload 里的 error 会被塌成笼统 "node invoke failed" 吞掉原因——提升到
            // 顶层，agent 才看得到 PERMISSION_MISSING / EXEC_APPROVAL_DENIED 等具体原因。
            if let Some(e) = frame_params["payload"].get("error").cloned() {
                frame_params["error"] = e;
            }
        }
        let result_frame = json!({
            "type": "req",
            "id": rand_id(),
            "method": "node.invoke.result",
            "params": frame_params
        });
        let _ = socket.send(Message::Text(result_frame.to_string()));
    }
}

/// 分发命令到 screen/control/system 等。
/// `app`：AppHandle（system.run 审批弹窗 / canvas 窗口用）。
/// pub(super)：mcp_server（本地环回 MCP，hermes aa-desktop）复用同一分发链路——
/// 闸门（app_gate / exec_approvals / act_gate）只此一份，两个 agent 同一套权限边界。
pub(super) fn dispatch(command: &str, params: &Value, app: &tauri::AppHandle, log: &impl Fn(&str)) -> (bool, Value) {
    match command {
        "screen.record" => {
            let r = screen::record(params);
            if !r.0 {
                log(&format!("screen.record 失败: {}", r.1));
            }
            r
        }
        "screen.snapshot" => {
            // 权限硬校验：Windows 没有系统级 TCC，但客户端可配置权限引擎允许用户在托盘/设置里
            // 关掉截屏能力（fail-closed 默认允许）。关掉时明确报错，不给无声假图。
            if !super::permissions::ensure_screen_recording(true) {
                log("snapshot 拒绝：截屏能力已在客户端关闭");
                (
                    false,
                    json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: screen（截屏能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
                )
            } else {
                match exec_snapshot(params) {
                    Ok(s) => (
                        true,
                        json!({
                            "format": s.format,
                            "base64": s.base64,
                            "displayFrameId": s.display_frame_id,
                            "screenIndex": params.get("screenIndex").and_then(|i| i.as_u64()).unwrap_or(0),
                            "width": s.width,
                            "height": s.height
                        }),
                    ),
                    Err(e) => {
                        log(&format!("snapshot 失败: {e}"));
                        (false, json!({"error": {"message": e}}))
                    }
                }
            }
        }
        "computer.act" => match exec_act(params) {
            Ok(()) => (true, json!({})),
            Err(e) => {
                log(&format!("act 失败: {e}"));
                (false, json!({"error": {"message": e}}))
            }
        },
        "system.run" => {
            let r = system::run(params, app);
            if !r.0 {
                log(&format!("system.run 失败: {}", r.1));
            }
            r
        }
        "system.notify" => {
            let r = system::notify(params);
            if !r.0 {
                log(&format!("system.notify 失败: {}", r.1));
            }
            r
        }
        // 薄转发：浏览器控制本体在本地 gateway 的 browser 插件（端口 = gateway+2）。
        // 阻塞式 HTTP（超时由调用方 timeoutMs 控制，browser 工具默认 20s）——期间 node 循环
        // 不收其它命令；agent 工具调用实际是串行的，可接受，并发场景再改线程池。
        "browser.proxy" => {
            let r = browser::proxy(params);
            if !r.0 {
                log(&format!("browser.proxy 失败: {}", r.1));
            }
            r
        }
        "location.get" => {
            let r = location::get(params);
            if !r.0 {
                log(&format!("location.get 失败: {}", r.1));
            }
            r
        }
        "contacts.search" => {
            let r = contacts::search(params);
            if !r.0 {
                log(&format!("contacts.search 失败: {}", r.1));
            }
            r
        }
        "calendar.events" => {
            let r = calendar::events(params);
            if !r.0 {
                log(&format!("calendar.events 失败: {}", r.1));
            }
            r
        }
        "reminders.list" => {
            let r = reminders::list(params);
            if !r.0 {
                log(&format!("reminders.list 失败: {}", r.1));
            }
            r
        }
        "photos.latest" => {
            let r = photos::latest(params);
            if !r.0 {
                log(&format!("photos.latest 失败: {}", r.1));
            }
            r
        }
        "device.info" => device::info(),
        "device.status" => device::status(),
        "camera.list" => camera::list(),
        "camera.snap" => {
            let r = camera::snap(params);
            if !r.0 {
                log(&format!("camera.snap 失败: {}", r.1));
            }
            r
        }
        "camera.clip" => {
            let r = camera::clip(params);
            if !r.0 {
                log(&format!("camera.clip 失败: {}", r.1));
            }
            r
        }
        // exec 审批策略文件读写（openclaw CLI `exec approvals node get/set` 走这里，互操作格式见模块头）
        "system.execApprovals.get" => (true, exec_approvals::cmd_get()),
        "system.execApprovals.set" => match exec_approvals::cmd_set(params) {
            Ok(v) => (true, v),
            Err((code, message)) => {
                log(&format!("system.execApprovals.set 拒绝: {code}: {message}"));
                (false, json!({"error": {"code": code, "message": message}}))
            }
        },
        // canvas 面板（窗口/scheme 托管/文件监听见 canvas.rs 模块头）
        "canvas.present" => {
            let r = canvas::cmd_present(app, params);
            if !r.0 {
                log(&format!("canvas.present 失败: {}", r.1));
            }
            r
        }
        "canvas.hide" => canvas::cmd_hide(app, params),
        "canvas.navigate" => {
            let r = canvas::cmd_navigate(app, params);
            if !r.0 {
                log(&format!("canvas.navigate 失败: {}", r.1));
            }
            r
        }
        "canvas.eval" => {
            let r = canvas::cmd_eval(app, params);
            if !r.0 {
                log(&format!("canvas.eval 失败: {}", r.1));
            }
            r
        }
        "canvas.snapshot" => {
            let r = canvas::cmd_snapshot(app, params);
            if !r.0 {
                log(&format!("canvas.snapshot 失败: {}", r.1));
            }
            r
        }
        // canvas a2ui（宿主页 = hello-ok 下发的 surface url，就绪轮询 + eval 注入消息）
        "canvas.a2ui.push" => {
            let r = canvas::cmd_a2ui_push(app, params);
            if !r.0 {
                log(&format!("canvas.a2ui.push 失败: {}", r.1));
            }
            r
        }
        "canvas.a2ui.pushJSONL" => {
            let r = canvas::cmd_a2ui_push_jsonl(app, params);
            if !r.0 {
                log(&format!("canvas.a2ui.pushJSONL 失败: {}", r.1));
            }
            r
        }
        "canvas.a2ui.reset" => {
            let r = canvas::cmd_a2ui_reset(app, params);
            if !r.0 {
                log(&format!("canvas.a2ui.reset 失败: {}", r.1));
            }
            r
        }
        other => (false, json!({"error": {"message": format!("未知命令: {other}")}})),
    }
}

fn exec_snapshot(params: &Value) -> Result<screen::Snapshot, String> {
    let idx = params.get("screenIndex").and_then(|i| i.as_u64()).unwrap_or(0) as usize;
    screen::capture(idx)
}

fn exec_act(params: &Value) -> Result<(), String> {
    let action = params.get("action").and_then(|a| a.as_str()).unwrap_or("");
    let x = params.get("x").and_then(|v| v.as_f64()).unwrap_or(0.0);
    let y = params.get("y").and_then(|v| v.as_f64()).unwrap_or(0.0);
    // App 控制闸门（输入注入型）：注入打进的是前台 App，须前台在清单内才放行
    //（fail-closed：前台未知/是本 App 自身都拒——防自我点掉 exec 审批弹窗）。
    // mouse_move/wait 不注入，不在其列。
    const INJECTIVE_ACTIONS: &[&str] = &[
        "left_click", "right_click", "middle_click", "double_click", "triple_click",
        "left_click_drag", "left_mouse_down", "left_mouse_up", "scroll", "type", "key", "hold_key",
    ];
    if INJECTIVE_ACTIONS.contains(&action) {
        super::app_gate::act_gate()?;
        // 权限硬校验：Windows 的 SendInput 无系统级开关，但客户端权限引擎允许用户关闭
        // 键鼠注入能力（fail-closed 默认允许）——关掉时明确报错，不给无声失败。
        if !super::permissions::ensure_accessibility(true) {
            return Err("PERMISSION_MISSING: accessibility（键鼠控制能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）".into());
        }
    }
    match action {
        "left_click" => control::click("left", x, y),
        "right_click" => control::click("right", x, y),
        "middle_click" => control::click("middle", x, y),
        "double_click" => control::click_n("left", x, y, 2),
        "triple_click" => control::click_n("left", x, y, 3),
        "mouse_move" => control::move_to(x, y),
        "left_click_drag" => {
            let fx = params.get("fromX").and_then(|v| v.as_f64()).unwrap_or(0.0);
            let fy = params.get("fromY").and_then(|v| v.as_f64()).unwrap_or(0.0);
            control::drag(fx, fy, x, y)
        }
        "left_mouse_down" => control::mouse_down("left", x, y),
        "left_mouse_up" => control::mouse_up("left", x, y),
        "scroll" => {
            let dir = params.get("scrollDirection").and_then(|d| d.as_str()).unwrap_or("up");
            let amt = params.get("scrollAmount").and_then(|a| a.as_i64()).unwrap_or(3) as i32;
            control::scroll(dir, amt)
        }
        "type" => control::type_text(params.get("text").and_then(|t| t.as_str()).unwrap_or("")),
        "key" => control::key_combo(params.get("keys").and_then(|k| k.as_str()).unwrap_or("")),
        "hold_key" => control::hold_key(
            params.get("keys").and_then(|k| k.as_str()).unwrap_or(""),
            params.get("durationMs").and_then(|d| d.as_u64()).unwrap_or(0),
        ),
        "wait" => {
            let ms = params.get("durationMs").and_then(|d| d.as_u64()).unwrap_or(0);
            std::thread::sleep(std::time::Duration::from_millis(ms));
            Ok(())
        }
        "" => Err("无 action".into()),
        other => Err(format!("未知 action: {other}")),
    }
}

// ===== device identity（Ed25519 + sha256 deviceId，~/.openclaw/identity/client-node.json）=====

struct Device {
    device_id: String,
    signing_key: SigningKey,
    public_key_raw: [u8; 32],
}

/// 读 gateway 鉴权 token（~/.openclaw/openclaw.json → gateway.auth.token）。
/// gateway 默认 token 鉴权（loopback 也校验）；connect 帧须带 auth.token，否则 AUTH_TOKEN_MISSING。
fn load_gateway_token() -> Option<String> {
    let path = dirs::home_dir()?.join(".openclaw").join("openclaw.json");
    let s = std::fs::read_to_string(&path).ok()?;
    let v: Value = serde_json::from_str(&s).ok()?;
    v.get("gateway")?.get("auth")?.get("token")?.as_str().map(String::from)
}

fn load_or_create_device() -> Result<Device, String> {
    // 独立路径（不与 gateway 的 ~/.openclaw/identity/device.json 共享）：gateway 存 PEM v1 格式，client 存
    // raw b64；共享会互相读不懂→各自重建→deviceId 漂移→握手断续失败。client 自管即可（node 连 gateway
    // 靠 connect 帧 publicKey 验签，loopback auto-pair 不依赖共享 identity）。
    let path = dirs::home_dir()
        .ok_or("no home")?
        .join(".openclaw")
        .join("identity")
        .join("client-node.json");
    if let Ok(s) = fs::read_to_string(&path) {
        if let Ok(v) = serde_json::from_str::<Value>(&s) {
            let device_id = v.get("deviceId").and_then(|d| d.as_str()).unwrap_or("").to_string();
            let pub_b64 = v.get("publicKey").and_then(|p| p.as_str()).unwrap_or("");
            let priv_b64 = v.get("privateKey").and_then(|p| p.as_str()).unwrap_or("");
            let pub_bytes = base64::engine::general_purpose::STANDARD
                .decode(pub_b64)
                .unwrap_or_default();
            let priv_bytes = base64::engine::general_purpose::STANDARD
                .decode(priv_b64)
                .unwrap_or_default();
            if device_id.len() == 64 && pub_bytes.len() == 32 && priv_bytes.len() == 32 {
                let mut pub_raw = [0u8; 32];
                let mut priv_raw = [0u8; 32];
                pub_raw.copy_from_slice(&pub_bytes);
                priv_raw.copy_from_slice(&priv_bytes);
                let signing_key = SigningKey::from_bytes(&priv_raw);
                return Ok(Device {
                    device_id,
                    signing_key,
                    public_key_raw: pub_raw,
                });
            }
        }
    }
    // 生成新 device
    let mut rng = rand::thread_rng();
    let signing_key = SigningKey::generate(&mut rng);
    let public_key_raw = signing_key.verifying_key().to_bytes();
    let device_id = hex_encode(&Sha256::digest(&public_key_raw));
    let _ = fs::create_dir_all(path.parent().unwrap_or(&path));
    let device_json = json!({
        "deviceId": device_id,
        "publicKey": base64::engine::general_purpose::STANDARD.encode(public_key_raw),
        "privateKey": base64::engine::general_purpose::STANDARD.encode(signing_key.to_bytes())
    });
    let _ = fs::write(&path, device_json.to_string());
    Ok(Device {
        device_id,
        signing_key,
        public_key_raw,
    })
}

fn hex_encode(b: &[u8]) -> String {
    b.iter().map(|x| format!("{:02x}", x)).collect()
}

fn rand_id() -> String {
    let mut buf = [0u8; 16];
    rand::thread_rng().fill_bytes(&mut buf);
    hex_encode(&buf)
}
