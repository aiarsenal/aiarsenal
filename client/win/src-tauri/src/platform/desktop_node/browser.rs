//! browser.proxy 命令——把 gateway 浏览器工具的 HTTP 调用转发给本地 gateway 的浏览器控制服务。
//!
//! 对齐 OpenClaw.app 的 MacNodeBrowserProxy（薄转发，不自研 CDP）：浏览器控制本体（托管
//! Chrome/profiles/CDP）跑在本地 gateway 的 browser 插件里，监听 gateway 端口 + 2 的
//! loopback 控制口（openclaw sdk-config.ts deriveDefaultBrowserControlPort）。本模块只做
//! node.invoke → HTTP 的桥：gateway 的 browser 工具 auto 模式检测到节点声明 browser.proxy
//! 后，调用会经节点通道转发（browser-tool.ts resolveBrowserNodeTarget），路径统一。
//!
//! 鉴权与 WS 握手同源（~/.openclaw/openclaw.json 的 gateway.auth）：控制服务接受
//! Authorization: Bearer <token> 或 x-openclaw-password（http-auth.ts）；凭据缺失时 gateway
//! 会自动生成并回写 gateway.auth 供 loopback 客户端读取（control-auth.ts），故这里总能读到。
//!
//! 响应里控制服务落盘的文件（截图 path/imagePath、下载 download.path）base64 捎回（≤10MB，
//! 对齐 OpenClaw.app 的限制），gateway 侧 persistProxyFiles 再映射回自己的路径。
//!
//! 注意（与 OpenClaw.app 的差异）：app 侧有设置开关（写 browser.enabled）控制是否声明该
//! 命令；client 无此设置页，无条件声明——browser 插件关闭时 gateway 根本不会发 browser
//! 调用，转发失败（UNAVAILABLE）只发生在 gateway 在跑但控制口没起的窗口期。

use std::time::Duration;

use base64::Engine;
use serde_json::{json, Value};

/// gateway 默认端口（与 protocol.rs GATEWAY_URL 一致；实际以 openclaw.json gateway.port 为准）。
const DEFAULT_GATEWAY_PORT: u64 = 18789;
/// 浏览器控制口 = gateway 端口 + 2（openclaw 端口派生规则）。
const CONTROL_PORT_OFFSET: u64 = 2;
const DEFAULT_TIMEOUT_SECS: u64 = 5;
const MAX_FILE_BYTES: u64 = 10 * 1024 * 1024;

/// 转发 browser.proxy 调用。params 形状对齐 MacNodeBrowserProxy.RequestParams：
/// { method, path, query, body, timeoutMs, profile }。
pub fn proxy(params: &Value) -> (bool, Value) {
    match exec_proxy(params) {
        Ok(payload) => (true, payload),
        Err(e) => (false, json!({ "error": { "message": e } })),
    }
}

fn exec_proxy(params: &Value) -> Result<Value, String> {
    let method = params
        .get("method")
        .and_then(|m| m.as_str())
        .unwrap_or("GET")
        .trim()
        .to_uppercase();
    let raw_path = params.get("path").and_then(|p| p.as_str()).unwrap_or("").trim().to_string();
    if raw_path.is_empty() {
        return Err("INVALID_REQUEST: path required".into());
    }
    let path = if raw_path.starts_with('/') { raw_path } else { format!("/{raw_path}") };

    let (port, token, password) = control_endpoint();
    let base = format!("http://127.0.0.1:{port}");

    // query：键排序 + 标量字符串化（跳过 null）；profile 参数缺省时补上（对齐 MacNodeBrowserProxy）
    let mut query: Vec<(String, String)> = Vec::new();
    if let Some(q) = params.get("query").and_then(|q| q.as_object()) {
        for (k, v) in q {
            if v.is_null() {
                continue;
            }
            query.push((k.clone(), scalar_to_string(v)));
        }
    }
    let profile = params.get("profile").and_then(|p| p.as_str()).unwrap_or("").trim().to_string();
    if !profile.is_empty() && !query.iter().any(|(k, _)| k == "profile") {
        query.push(("profile".into(), profile));
    }
    query.sort();
    let url = if query.is_empty() {
        format!("{base}{path}")
    } else {
        let mut ser = url::form_urlencoded::Serializer::new(format!("{base}{path}?"));
        for (k, v) in &query {
            ser.append_pair(k, v);
        }
        ser.finish()
    };

    // 调用方（gateway browser 工具）带 timeoutMs（默认 20s）；缺省 5s 对齐 MacNodeBrowserProxy
    let timeout_ms = params.get("timeoutMs").and_then(|t| t.as_u64()).unwrap_or(0);
    let timeout = if timeout_ms == 0 {
        Duration::from_secs(DEFAULT_TIMEOUT_SECS)
    } else {
        Duration::from_millis(timeout_ms.max(1))
    };

    let client = reqwest::blocking::Client::builder()
        .timeout(timeout)
        .build()
        .map_err(|e| format!("HTTP client 构建失败: {e}"))?;
    let method = reqwest::Method::from_bytes(method.as_bytes())
        .map_err(|_| format!("INVALID_REQUEST: method 不合法: {method}"))?;
    let mut req = client.request(method, &url);
    if let Some(t) = &token {
        req = req.bearer_auth(t);
    } else if let Some(p) = &password {
        req = req.header("x-openclaw-password", p);
    }
    if let Some(body) = params.get("body") {
        if !body.is_null() {
            req = req.json(body);
        }
    }
    let resp = req
        .send()
        .map_err(|e| format!("UNAVAILABLE: 连不上本地浏览器控制服务 {base}（gateway 没跑或 browser 插件未启用）：{e}"))?;
    let status = resp.status();
    let text = resp.text().map_err(|e| format!("读取控制服务响应失败: {e}"))?;
    if !status.is_success() {
        return Err(http_error_message(status.as_u16(), &text));
    }
    let result: Value = serde_json::from_str(&text)
        .map_err(|_| format!("控制服务返回非 JSON：{}", truncate(&text, 200)))?;
    let files = collect_files(&result)?;
    let mut payload = json!({ "result": result });
    if !files.is_empty() {
        payload["files"] = json!(files);
    }
    Ok(payload)
}

/// 读控制端点与鉴权（gateway.port / gateway.auth.token / gateway.auth.password）。
/// 端口缺省回落 18789+2，与 protocol.rs 固定连 18789 的口径一致。
fn control_endpoint() -> (u64, Option<String>, Option<String>) {
    let fallback = (DEFAULT_GATEWAY_PORT + CONTROL_PORT_OFFSET, None, None);
    let Some(home) = dirs::home_dir() else { return fallback };
    let Ok(s) = std::fs::read_to_string(home.join(".openclaw").join("openclaw.json")) else {
        return fallback;
    };
    let Ok(v) = serde_json::from_str::<Value>(&s) else { return fallback };
    let gw = v.get("gateway").cloned().unwrap_or(Value::Null);
    let port = gw
        .get("port")
        .and_then(|p| p.as_u64())
        .filter(|p| *p > 0 && *p + CONTROL_PORT_OFFSET <= 65535)
        .unwrap_or(DEFAULT_GATEWAY_PORT)
        + CONTROL_PORT_OFFSET;
    let cred = |key: &str| {
        gw.get("auth")
            .and_then(|a| a.get(key))
            .and_then(|t| t.as_str())
            .map(|t| t.trim().to_string())
            .filter(|t| !t.is_empty())
    };
    (port, cred("token"), cred("password"))
}

/// 结果里控制服务落盘的文件（截图/下载）→ base64 载荷数组。超 10MB 直接报错（对齐 app）。
fn collect_files(result: &Value) -> Result<Vec<Value>, String> {
    let mut paths: Vec<String> = Vec::new();
    for key in ["path", "imagePath"] {
        if let Some(p) = result.get(key).and_then(|p| p.as_str()).map(str::trim) {
            if !p.is_empty() && !paths.iter().any(|x| x == p) {
                paths.push(p.to_string());
            }
        }
    }
    if let Some(p) = result
        .get("download")
        .and_then(|d| d.get("path"))
        .and_then(|p| p.as_str())
        .map(str::trim)
    {
        if !p.is_empty() && !paths.iter().any(|x| x == p) {
            paths.push(p.to_string());
        }
    }
    paths.iter().map(|p| load_file(p)).collect()
}

fn load_file(path: &str) -> Result<Value, String> {
    let meta = std::fs::metadata(path).map_err(|_| format!("browser proxy file not found: {path}"))?;
    if !meta.is_file() {
        return Err(format!("browser proxy file not found: {path}"));
    }
    if meta.len() > MAX_FILE_BYTES {
        return Err(format!("browser proxy file exceeds 10MB: {path}"));
    }
    let data = std::fs::read(path).map_err(|e| format!("读取 {path} 失败: {e}"))?;
    let mut f = json!({
        "path": path,
        "base64": base64::engine::general_purpose::STANDARD.encode(&data),
    });
    if let Some(m) = mime_for(path) {
        f["mimeType"] = json!(m);
    }
    Ok(f)
}

fn mime_for(path: &str) -> Option<&'static str> {
    let ext = path.rsplit('.').next()?.to_lowercase();
    Some(match ext.as_str() {
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "webp" => "image/webp",
        "gif" => "image/gif",
        "svg" => "image/svg+xml",
        "pdf" => "application/pdf",
        _ => return None,
    })
}

/// 控制服务错误报文：JSON error 字段 > 原文 > HTTP 状态码（对齐 MacNodeBrowserProxy.httpErrorMessage）。
fn http_error_message(status: u16, text: &str) -> String {
    if let Ok(v) = serde_json::from_str::<Value>(text) {
        if let Some(e) = v.get("error").and_then(|e| e.as_str()).map(str::trim) {
            if !e.is_empty() {
                return e.to_string();
            }
        }
    }
    let t = text.trim();
    if !t.is_empty() {
        return truncate(t, 300).to_string();
    }
    format!("HTTP {status}")
}

fn scalar_to_string(v: &Value) -> String {
    match v {
        Value::String(s) => s.clone(),
        Value::Bool(b) => b.to_string(),
        Value::Number(n) => n.to_string(),
        _ => String::new(),
    }
}

fn truncate(s: &str, max: usize) -> String {
    if s.len() <= max {
        s.to_string()
    } else {
        format!("{}…", &s[..max])
    }
}
