//! Canvas：openclaw canvas.* node 命令的 macOS 实现（替代 OpenClaw.app 的 CanvasManager）。
//!
//! 对齐 openclaw-2026.7.1（extensions/canvas + apps/macos）：
//! - canvas 根 = gateway 写入侧 `$OPENCLAW_STATE_DIR/canvas`（默认 `~/.openclaw/canvas`，
//!   extensions/canvas/src/documents.ts resolveCanvasRootDir）。openclaw.app 的
//!   Application Support/OpenClaw/canvas 是它自家约定，不采用。
//! - 自定义 scheme `openclaw-canvas` 托管 canvas 根下的静态文件（macOS origin 形如
//!   `openclaw-canvas://localhost/<path>`）：目录补 index.html、缺文件给占位页、
//!   `..` 段拒绝 + canonicalize 前缀双保险（防穿越/防 symlink 逃逸）。
//! - 窗口 label "canvas" 常驻：hide 只藏不销毁（保 WKWebView 会话状态，eval/snapshot 不丢上下文）。
//! - 文件监听（notify 递归 watch canvas 根）去抖 300ms 自动 reload——agent 写完即所见。
//! - a2ui 三命令：宿主页 = hello-ok 下发的 pluginSurfaceUrls.canvas + `/__openclaw__/a2ui/`，
//!   push/pushJSONL 经 eval `openclawA2UI.applyMessages(<json>)`，reset 同理。
//!
//! 已知差距：wry 的 eval 会吞 JS 异常（callback 不回调），eval 失败/超时按 openclaw
//! 容忍口径返回 `{result:""}`。

use std::ffi::c_void;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc;
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

// ImageIO/CoreFoundation/CGImage 尺寸符号复用 screen.rs 的声明（那边已 pub）
use super::screen::{
    CFDataCreateMutable, CFDataGetBytePtr, CFDataGetLength, CFRelease, CFStringCreateWithBytes,
    CGImageDestinationAddImage, CGImageDestinationCreateWithData, CGImageDestinationFinalize,
    CGImageGetHeight, CGImageGetWidth,
};

const CANVAS_WINDOW_LABEL: &str = "canvas";
const CANVAS_SCHEME: &str = "openclaw-canvas";

/// gateway hello-ok 下发的 canvas surface url（pluginSurfaceUrls.canvas，scoped）。
/// a2ui 宿主页 = 该 url + `/__openclaw__/a2ui/?platform=macos`。握手时由 protocol.rs 写入。
pub static CANVAS_SURFACE_URL: OnceLock<String> = OnceLock::new();
pub fn set_surface_url(url: &str) {
    let _ = CANVAS_SURFACE_URL.set(url.to_string());
}

// ==================== 根目录 ====================

/// canvas 根（gateway 文件工具实际写入侧）。
pub fn canvas_root() -> PathBuf {
    if let Ok(dir) = std::env::var("OPENCLAW_STATE_DIR") {
        let dir = dir.trim();
        if !dir.is_empty() {
            return expand_home(dir).join("canvas");
        }
    }
    dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("/tmp"))
        .join(".openclaw")
        .join("canvas")
}

fn expand_home(p: &str) -> PathBuf {
    if p == "~" {
        dirs::home_dir().unwrap_or_else(|| PathBuf::from(p))
    } else if let Some(rest) = p.strip_prefix("~/") {
        dirs::home_dir()
            .unwrap_or_else(|| PathBuf::from(p))
            .join(rest)
    } else {
        PathBuf::from(p)
    }
}

// ==================== scheme handler（静态文件托管） ====================

fn percent_decode(s: &str) -> String {
    let b = s.as_bytes();
    let mut out: Vec<u8> = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        if b[i] == b'%' && i + 2 < b.len() {
            let h = (b[i + 1] as char).to_digit(16);
            let l = (b[i + 2] as char).to_digit(16);
            if let (Some(h), Some(l)) = (h, l) {
                out.push((h * 16 + l) as u8);
                i += 3;
                continue;
            }
        }
        out.push(b[i]);
        i += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

/// MIME 表（对齐 openclaw.app CanvasScheme.mimeType）。
fn mime_for(path: &std::path::Path) -> &'static str {
    match path
        .extension()
        .and_then(|e| e.to_str())
        .map(|e| e.to_ascii_lowercase())
        .as_deref()
    {
        Some("html") | Some("htm") => "text/html",
        Some("js") | Some("mjs") => "application/javascript",
        Some("css") => "text/css",
        Some("json") | Some("map") => "application/json",
        Some("svg") => "image/svg+xml",
        Some("png") => "image/png",
        Some("jpg") | Some("jpeg") => "image/jpeg",
        Some("gif") => "image/gif",
        Some("ico") => "image/x-icon",
        Some("woff2") => "font/woff2",
        Some("woff") => "font/woff",
        Some("ttf") => "font/ttf",
        Some("wasm") => "application/wasm",
        _ => "application/octet-stream",
    }
}

fn resp_text(status: u16, content_type: &str, body: String) -> tauri::http::Response<Vec<u8>> {
    tauri::http::Response::builder()
        .status(status)
        .header("Content-Type", content_type)
        .body(body.into_bytes())
        .unwrap_or_else(|_| tauri::http::Response::new(Vec::new()))
}

/// canvas 根为空时的占位页（根路径无 index.html）。
fn scaffold_page(root: &std::path::Path) -> String {
    format!(
        r#"<!doctype html>
<html><head><meta charset="utf-8"><title>Canvas</title>
<style>
  body {{ margin:0; height:100vh; display:flex; align-items:center; justify-content:center;
         background:#161a22; color:#8a94a6; font-family:-apple-system,"PingFang SC",sans-serif; }}
  .card {{ text-align:center; max-width:420px; padding:32px; }}
  h1 {{ font-size:20px; color:#e6edf3; margin:0 0 12px; }}
  p {{ font-size:13px; line-height:1.8; margin:0; }}
  code {{ color:#42b983; word-break:break-all; }}
</style></head>
<body><div class="card">
  <h1>Canvas</h1>
  <p>目录还是空的：<br><code>{}</code></p>
  <p style="margin-top:14px">agent 写入 index.html 后窗口自动刷新（文件变更 300ms 去抖 reload）。</p>
</div></body></html>"#,
        root.display()
    )
}

/// `openclaw-canvas://localhost/<path>` 静态文件托管（lib.rs builder 注册）。
/// 安全：`..` 段直接拒（403）+ canonicalize 后必须仍在 canvas 根内（防 symlink 逃逸）。
pub fn handle_scheme(request: tauri::http::Request<Vec<u8>>) -> tauri::http::Response<Vec<u8>> {
    // 解 URL 取 path（authority 固定 localhost；不能按 "localhost" 字符串切，
    // 文件名段本身可能含 "localhost"）
    let path_only = match url::Url::parse(&request.uri().to_string()) {
        Ok(u) => u.path().to_string(),
        Err(_) => "/".to_string(),
    };
    let decoded = percent_decode(&path_only);

    let root = canvas_root();
    let root_canon = std::fs::canonicalize(&root).unwrap_or_else(|_| root.clone());

    let mut full = root_canon.clone();
    let mut has_seg = false;
    for seg in decoded.split('/') {
        match seg {
            "" | "." => continue,
            ".." => {
                return resp_text(403, "text/plain", "forbidden".into());
            }
            s => {
                full.push(s);
                has_seg = true;
            }
        }
    }
    // 目录（含根）补 index.html
    if full.is_dir() {
        full = full.join("index.html");
    }
    if !full.is_file() {
        if !has_seg {
            return resp_text(200, "text/html", scaffold_page(&root));
        }
        return resp_text(404, "text/plain", format!("not found: {decoded}"));
    }
    // 双保险：canonicalize 后必须仍在根内
    match std::fs::canonicalize(&full) {
        Ok(c) if c.starts_with(&root_canon) => {}
        _ => return resp_text(403, "text/plain", "forbidden".into()),
    }
    match std::fs::read(&full) {
        Ok(bytes) => tauri::http::Response::builder()
            .status(200)
            .header("Content-Type", mime_for(&full))
            .body(bytes)
            .unwrap_or_else(|_| tauri::http::Response::new(Vec::new())),
        Err(e) => resp_text(500, "text/plain", format!("read error: {e}")),
    }
}

// ==================== URL 映射（对齐 openclaw.app load(target:)） ====================

fn home_url() -> String {
    format!("{CANVAS_SCHEME}://localhost/")
}

/// target 归一：
/// 1) http(s):// file:// （及已是 openclaw-canvas://）→ 原样直返
/// 2) 绝对路径且是已存在文件 → file:// 直载
/// 3) 其余（裸相对名 / 根路由路径 / ~/）→ canvas 根下 scheme 托管
fn map_url(target: &str) -> Result<String, String> {
    let t = target.trim();
    if t.is_empty() {
        return Ok(home_url());
    }
    if let Ok(u) = url::Url::parse(t) {
        match u.scheme() {
            "http" | "https" | "file" | CANVAS_SCHEME => return Ok(u.to_string()),
            _ => {}
        }
    }
    // Url::parse 无 scheme 会失败 → 这里是相对/绝对路径
    let candidate = expand_home(t);
    if candidate.is_file() {
        let canon = candidate.canonicalize().unwrap_or(candidate);
        return url::Url::from_file_path(canon)
            .map(|u| u.to_string())
            .map_err(|_| format!("file URL 转换失败: {t}"));
    }
    if t.starts_with('/') {
        return Ok(format!("{CANVAS_SCHEME}://localhost{t}"));
    }
    // 裸相对名 / ~/ 前缀：都按 canvas 根相对路径托管
    let rel = t.strip_prefix("~/").unwrap_or(t);
    Ok(format!("{CANVAS_SCHEME}://localhost/{}", rel.trim_start_matches('/')))
}

// ==================== 窗口与基础命令 ====================

fn ensure_canvas_window(app: &tauri::AppHandle) -> Result<tauri::WebviewWindow, String> {
    use tauri::Manager;
    if let Some(w) = app.get_webview_window(CANVAS_WINDOW_LABEL) {
        return Ok(w);
    }
    let url = url::Url::parse(&home_url()).map_err(|e| format!("canvas URL 解析失败: {e}"))?;
    tauri::WebviewWindowBuilder::new(
        app,
        CANVAS_WINDOW_LABEL,
        tauri::WebviewUrl::External(url),
    )
    .title("Canvas")
    .inner_size(900.0, 640.0)
    .center()
    .build()
    .map_err(|e| format!("创建 canvas 窗口失败: {e}"))
}

fn navigate_url(w: &tauri::WebviewWindow, url: &str) -> Result<(), String> {
    let u = url::Url::parse(url).map_err(|e| format!("URL 解析失败: {url}（{e}）"))?;
    w.navigate(u).map_err(|e| format!("navigate 失败: {e}"))
}

/// eval 并等结果（5s 上限）。wry 吞 JS 异常时不回调 → 走超时。
fn eval_js(w: &tauri::WebviewWindow, js: &str) -> Result<String, String> {
    let (tx, rx) = mpsc::channel::<String>();
    w.eval_with_callback(js, move |s| {
        let _ = tx.send(s);
    })
    .map_err(|e| format!("eval 提交失败: {e}"))?;
    rx.recv_timeout(Duration::from_secs(5))
        .map_err(|_| "eval 超时（5s）".to_string())
}

fn err_invalid(msg: impl Into<String>) -> (bool, Value) {
    (
        false,
        json!({"error": {"code": "INVALID_REQUEST", "message": msg.into()}}),
    )
}

fn err_unavailable(msg: impl Into<String>) -> (bool, Value) {
    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": msg.into()}}),
    )
}

/// canvas.present {url?, placement?{x,y,width,height}}：建/取窗口 → 可选 navigate → 可选摆位 → show+focus。
pub fn cmd_present(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    start_watcher_once(app);
    let w = match ensure_canvas_window(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    if let Some(u) = params
        .get("url")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|s| !s.is_empty())
    {
        match map_url(u).and_then(|target| navigate_url(&w, &target)) {
            Ok(()) => {}
            Err(e) => return err_invalid(e),
        }
    }
    if let Some(p) = params.get("placement").and_then(Value::as_object) {
        let (wd, ht) = (
            p.get("width").and_then(Value::as_f64),
            p.get("height").and_then(Value::as_f64),
        );
        if let (Some(wd), Some(ht)) = (wd, ht) {
            if wd > 0.0 && ht > 0.0 {
                let _ = w.set_size(tauri::LogicalSize::new(wd, ht));
            }
        }
        let (x, y) = (
            p.get("x").and_then(Value::as_f64),
            p.get("y").and_then(Value::as_f64),
        );
        if let (Some(x), Some(y)) = (x, y) {
            let _ = w.set_position(tauri::LogicalPosition::new(x, y));
        }
    }
    let _ = w.show();
    let _ = w.set_focus();
    (true, json!({}))
}

/// canvas.hide {}：只藏不销毁（保 WKWebView 状态）。
pub fn cmd_hide(app: &tauri::AppHandle, _params: &Value) -> (bool, Value) {
    use tauri::Manager;
    if let Some(w) = app.get_webview_window(CANVAS_WINDOW_LABEL) {
        let _ = w.hide();
    }
    (true, json!({}))
}

/// canvas.navigate {url}。
pub fn cmd_navigate(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    let Some(u) = params
        .get("url")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|s| !s.is_empty())
    else {
        return err_invalid("canvas.navigate 需要 url");
    };
    let w = match ensure_canvas_window(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    match map_url(u).and_then(|target| navigate_url(&w, &target)) {
        Ok(()) => (true, json!({})),
        Err(e) => err_invalid(e),
    }
}

/// canvas.eval {javaScript} → {result: string}。
/// 异常/超时返回 `{result:""}`（wry 吞 JS 异常，openclaw 端容忍空串）。
pub fn cmd_eval(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    let Some(js) = params
        .get("javaScript")
        .and_then(Value::as_str)
        .filter(|s| !s.trim().is_empty())
    else {
        return err_invalid("canvas.eval 需要 javaScript");
    };
    let w = match ensure_canvas_window(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    match eval_js(&w, js) {
        Ok(r) => (true, json!({"result": r})),
        Err(e) => {
            aiarsenal_client_core::util::log("warn", &format!("[canvas] eval 失败: {e}"));
            (true, json!({"result": ""}))
        }
    }
}

// ==================== a2ui ====================

fn a2ui_host_url() -> Option<String> {
    let base = CANVAS_SURFACE_URL.get()?.trim().trim_end_matches('/');
    if base.is_empty() {
        None
    } else {
        Some(format!("{base}/__openclaw__/a2ui/?platform=macos"))
    }
}

const A2UI_PROBE: &str = "(() => { try { \
return globalThis.openclawA2UI && typeof globalThis.openclawA2UI.applyMessages === 'function' ? 'ok' : ''; \
} catch (e) { return ''; } })()";

fn host_ready(w: &tauri::WebviewWindow) -> bool {
    matches!(eval_js(w, A2UI_PROBE), Ok(s) if s.trim().trim_matches('"') == "ok")
}

/// a2ui 前置：当前页已 ready 直接用；否则跳宿主页并轮询就绪（≤6s）。
/// surface url 未下发 → A2UI_HOST_NOT_CONFIGURED；页不可达/超时 → A2UI_HOST_UNAVAILABLE。
fn ensure_a2ui_host(app: &tauri::AppHandle) -> Result<tauri::WebviewWindow, String> {
    let w = ensure_canvas_window(app)?;
    if host_ready(&w) {
        return Ok(w);
    }
    let url = a2ui_host_url().ok_or_else(|| {
        "A2UI_HOST_NOT_CONFIGURED: gateway 未在 hello-ok 下发 canvas surface url（重连后重试）"
            .to_string()
    })?;
    navigate_url(&w, &url).map_err(|e| format!("A2UI_HOST_UNAVAILABLE: {e}"))?;
    for _ in 0..20 {
        std::thread::sleep(Duration::from_millis(300));
        if host_ready(&w) {
            return Ok(w);
        }
    }
    Err("A2UI_HOST_UNAVAILABLE: a2ui 宿主页就绪超时（6s）".to_string())
}

/// canvas.a2ui.push {messages: [...]}。
pub fn cmd_a2ui_push(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    let Some(messages) = params.get("messages").and_then(Value::as_array) else {
        return err_invalid("canvas.a2ui.push 需要 messages 数组");
    };
    push_messages(app, messages.clone())
}

/// canvas.a2ui.pushJSONL {jsonl}：逐行解析成 JSON 数组再 push。
pub fn cmd_a2ui_push_jsonl(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    let Some(jsonl) = params
        .get("jsonl")
        .and_then(Value::as_str)
        .filter(|s| !s.trim().is_empty())
    else {
        return err_invalid("canvas.a2ui.pushJSONL 需要 jsonl");
    };
    let mut messages = Vec::new();
    for (i, line) in jsonl.lines().enumerate() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        match serde_json::from_str::<Value>(line) {
            Ok(v) => messages.push(v),
            Err(e) => {
                return err_invalid(format!("canvas.a2ui.pushJSONL 第 {} 行解析失败: {e}", i + 1))
            }
        }
    }
    push_messages(app, messages)
}

fn push_messages(app: &tauri::AppHandle, messages: Vec<Value>) -> (bool, Value) {
    let w = match ensure_a2ui_host(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    let Ok(mj) = serde_json::to_string(&messages) else {
        return err_unavailable("messages 序列化失败");
    };
    let js = format!(r#"globalThis.openclawA2UI.applyMessages({mj})"#);
    match eval_js(&w, &js) {
        // applyMessages 返回 JSON 对象（宿主页契约），原样转成 payload
        Ok(r) => (true, serde_json::from_str(&r).unwrap_or(json!({}))),
        Err(e) => err_unavailable(format!("canvas.a2ui.push 失败: {e}")),
    }
}

/// canvas.a2ui.reset {}。
pub fn cmd_a2ui_reset(app: &tauri::AppHandle, _params: &Value) -> (bool, Value) {
    let w = match ensure_a2ui_host(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    match eval_js(&w, "globalThis.openclawA2UI.reset()") {
        Ok(r) => (true, serde_json::from_str(&r).unwrap_or(json!({}))),
        Err(e) => err_unavailable(format!("canvas.a2ui.reset 失败: {e}")),
    }
}

// ==================== 文件监听（自动 reload） ====================

static WATCHER_STARTED: AtomicBool = AtomicBool::new(false);
static LAST_RELOAD: Mutex<Option<Instant>> = Mutex::new(None);

/// 递归 watch canvas 根，变更（可托管资源）去抖 300ms → canvas 窗口 reload。首个 present 启动。
fn start_watcher_once(app: &tauri::AppHandle) {
    if WATCHER_STARTED.swap(true, Ordering::SeqCst) {
        return;
    }
    let app = app.clone();
    std::thread::spawn(move || {
        use notify::Watcher;
        let root = canvas_root();
        if let Err(e) = std::fs::create_dir_all(&root) {
            aiarsenal_client_core::util::log(
                "warn",
                &format!("[canvas] 建根目录失败 {}: {e}", root.display()),
            );
            return;
        }
        let app2 = app.clone();
        let watcher = notify::recommended_watcher(move |ev: Result<notify::Event, notify::Error>| {
            let Ok(ev) = ev else { return };
            let relevant = ev.paths.iter().any(|p| {
                matches!(
                    p.extension().and_then(|e| e.to_str()).map(|e| e.to_ascii_lowercase()).as_deref(),
                    Some("html" | "htm" | "js" | "mjs" | "css" | "json" | "map" | "svg" | "png"
                        | "jpg" | "jpeg" | "gif" | "ico" | "woff" | "woff2" | "ttf" | "wasm")
                ) || p.extension().is_none()
            });
            if !relevant {
                return;
            }
            // 去抖：burst 期间只放行第一个事件
            if let Ok(mut last) = LAST_RELOAD.lock() {
                if let Some(t) = *last {
                    if t.elapsed() < Duration::from_millis(300) {
                        return;
                    }
                }
                *last = Some(Instant::now());
            }
            use tauri::Manager;
            if let Some(w) = app2.get_webview_window(CANVAS_WINDOW_LABEL) {
                let _ = w.reload();
            }
        });
        let mut watcher = match watcher {
            Ok(w) => w,
            Err(e) => {
                aiarsenal_client_core::util::log("warn", &format!("[canvas] watcher 启动失败: {e}"));
                return;
            }
        };
        if let Err(e) = watcher.watch(&root, notify::RecursiveMode::Recursive) {
            aiarsenal_client_core::util::log(
                "warn",
                &format!("[canvas] watch 失败（{}）: {e}", root.display()),
            );
            return;
        }
        aiarsenal_client_core::util::log(
            "info",
            &format!("[canvas] watcher 已启动: {}", root.display()),
        );
        std::thread::park(); // watcher 须保持存活，本线程常驻
    });
}

// ==================== canvas.snapshot（WKWebView 渲染图） ====================

#[repr(C)]
#[allow(non_snake_case)]
struct CGRect {
    x: f64,
    y: f64,
    width: f64,
    height: f64,
}

#[link(name = "CoreGraphics", kind = "framework")]
extern "C" {
    fn CGColorSpaceCreateDeviceRGB() -> *mut c_void;
    fn CGBitmapContextCreate(
        data: *mut c_void,
        width: usize,
        height: usize,
        bitsPerComponent: usize,
        bytesPerRow: usize,
        space: *mut c_void,
        bitmapInfo: u32,
    ) -> *mut c_void;
    fn CGContextDrawImage(c: *mut c_void, rect: CGRect, image: *const c_void);
    fn CGBitmapContextCreateImage(c: *mut c_void) -> *mut c_void;
    fn CGContextRelease(c: *mut c_void);
}

#[link(name = "ImageIO", kind = "framework")]
extern "C" {
    static kCGImageDestinationLossyCompressionQuality: *const c_void; // CFStringRef
}

#[link(name = "CoreFoundation", kind = "framework")]
extern "C" {
    fn CFDictionaryCreate(
        alloc: *mut c_void,
        keys: *const *const c_void,
        values: *const *const c_void,
        numValues: isize,
        keyCallBacks: *const c_void,
        valueCallBacks: *const c_void,
    ) -> *mut c_void;
    fn CFNumberCreate(alloc: *mut c_void, theType: isize, valuePtr: *const c_void) -> *mut c_void;
    static kCFTypeDictionaryKeyCallBacks: [u8; 48]; // 实为 CFDictionaryKeyCallBacks 结构体，只取地址
    static kCFTypeDictionaryValueCallBacks: [u8; 48];
}

const CF_UTF8: u32 = 0x0800_0100;
const CF_NUMBER_FLOAT64: isize = 6;
const K_CGIMAGE_ALPHA_PREMULTIPLIED_LAST: u32 = 1;

unsafe fn cf_string(s: &str) -> *mut c_void {
    CFStringCreateWithBytes(
        std::ptr::null_mut(),
        s.as_ptr(),
        s.len() as isize,
        CF_UTF8,
        false,
    )
}

/// CGImage → PNG/JPEG bytes。quality 仅 JPEG（ImageIO lossy 压缩质量）。
unsafe fn encode_image(cg: *mut c_void, uti: &str, quality: Option<f64>) -> Result<Vec<u8>, String> {
    let uti_ref = cf_string(uti);
    if uti_ref.is_null() {
        return Err("UTI 字符串创建失败".into());
    }
    let mut props: *mut c_void = std::ptr::null_mut();
    if let Some(q) = quality {
        // key = kCGImageDestinationLossyCompressionQuality（extern static 本身就是 CFStringRef 指针值）
        let mut qv = q;
        let num = CFNumberCreate(
            std::ptr::null_mut(),
            CF_NUMBER_FLOAT64,
            &mut qv as *mut f64 as *const c_void,
        );
        if !num.is_null() {
            let keys = [kCGImageDestinationLossyCompressionQuality];
            let values = [num as *const c_void];
            props = CFDictionaryCreate(
                std::ptr::null_mut(),
                keys.as_ptr(),
                values.as_ptr(),
                1,
                kCFTypeDictionaryKeyCallBacks.as_ptr() as *const c_void,
                kCFTypeDictionaryValueCallBacks.as_ptr() as *const c_void,
            );
            CFRelease(num);
        }
    }
    let data = CFDataCreateMutable(std::ptr::null(), 0);
    if data.is_null() {
        if !props.is_null() {
            CFRelease(props);
        }
        CFRelease(uti_ref);
        return Err("CFData 创建失败".into());
    }
    let dest = CGImageDestinationCreateWithData(data, uti_ref, 1, std::ptr::null());
    if dest.is_null() {
        CFRelease(data);
        if !props.is_null() {
            CFRelease(props);
        }
        CFRelease(uti_ref);
        return Err("CGImageDestination 创建失败".into());
    }
    CGImageDestinationAddImage(dest, cg, props);
    CGImageDestinationFinalize(dest);
    CFRelease(dest);
    if !props.is_null() {
        CFRelease(props);
    }
    CFRelease(uti_ref);
    let len = CFDataGetLength(data);
    let ptr = CFDataGetBytePtr(data);
    let bytes = if !ptr.is_null() && len > 0 {
        std::slice::from_raw_parts(ptr, len as usize).to_vec()
    } else {
        Vec::new()
    };
    CFRelease(data);
    if bytes.is_empty() {
        return Err("图像编码结果为空".into());
    }
    Ok(bytes)
}

/// 超宽缩放（等比例，kCGImageAlphaPremultipliedLast RGBA）。
unsafe fn scale_cgimage(cg: *mut c_void, dw: usize, dh: usize) -> Result<*mut c_void, String> {
    let space = CGColorSpaceCreateDeviceRGB();
    if space.is_null() {
        return Err("CGColorSpace 创建失败".into());
    }
    let ctx = CGBitmapContextCreate(
        std::ptr::null_mut(),
        dw,
        dh,
        8,
        dw * 4,
        space,
        K_CGIMAGE_ALPHA_PREMULTIPLIED_LAST,
    );
    CFRelease(space);
    if ctx.is_null() {
        return Err("CGBitmapContext 创建失败".into());
    }
    CGContextDrawImage(
        ctx,
        CGRect { x: 0.0, y: 0.0, width: dw as f64, height: dh as f64 },
        cg,
    );
    let out = CGBitmapContextCreateImage(ctx);
    CGContextRelease(ctx);
    if out.is_null() {
        return Err("缩放输出创建失败".into());
    }
    Ok(out)
}

/// NSImage → 编码 bytes（含 maxWidth 缩放）。
unsafe fn snapshot_nsimage(
    img: *mut objc2_app_kit::NSImage,
    is_png: bool,
    max_w: u64,
    quality: f64,
) -> Result<Vec<u8>, String> {
    let cg: *mut c_void = objc2::msg_send![
        &*img,
        CGImageForProposedRect: std::ptr::null_mut::<CGRect>() as *mut c_void,
        context: std::ptr::null_mut::<c_void>(),
        hints: std::ptr::null_mut::<c_void>()
    ];
    if cg.is_null() {
        return Err("NSImage→CGImage 转换失败".into());
    }
    let w = CGImageGetWidth(cg);
    let h = CGImageGetHeight(cg);
    let final_cg = if w > max_w as usize && h > 0 && max_w > 0 {
        let dw = max_w as usize;
        let dh = ((h * dw) / w).max(1);
        scale_cgimage(cg, dw, dh)?
    } else {
        cg
    };
    let uti = if is_png { "public.png" } else { "public.jpeg" };
    encode_image(final_cg, uti, if is_png { None } else { Some(quality) })
}

/// canvas.snapshot {format? png|jpeg, maxWidth?, quality?} → {format, base64}。
/// maxWidth 缺省 png:900 / jpeg:1600；quality 缺省 0.9（仅 jpeg）。
pub fn cmd_snapshot(app: &tauri::AppHandle, params: &Value) -> (bool, Value) {
    let format = params
        .get("format")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .unwrap_or("jpeg");
    let format = match format.to_ascii_lowercase().as_str() {
        "png" => "png",
        "jpeg" | "jpg" => "jpeg",
        other => return err_invalid(format!("canvas.snapshot format 非法: {other}（png/jpeg）")),
    };
    let is_png = format == "png";
    let max_w = params
        .get("maxWidth")
        .and_then(Value::as_u64)
        .filter(|v| *v > 0)
        .unwrap_or(if is_png { 900 } else { 1600 });
    let quality = params
        .get("quality")
        .and_then(Value::as_f64)
        .unwrap_or(0.9)
        .clamp(0.0, 1.0);

    let w = match ensure_canvas_window(app) {
        Ok(w) => w,
        Err(e) => return err_unavailable(e),
    };
    // 隐藏窗口 takeSnapshot 拿到空图，先拉起
    let _ = w.show();
    let (tx, rx) = mpsc::channel::<Result<Vec<u8>, String>>();
    if let Err(e) = w.with_webview(move |pw| unsafe {
        let wk = &*(pw.inner() as *mut objc2_web_kit::WKWebView);
        let block = block2::RcBlock::new(
            move |img: *mut objc2_app_kit::NSImage, err: *mut objc2_foundation::NSError| {
                let res = if !err.is_null() {
                    Err("takeSnapshot 返回错误".to_string())
                } else if img.is_null() {
                    Err("takeSnapshot 空图".to_string())
                } else {
                    snapshot_nsimage(img, is_png, max_w, quality)
                };
                let _ = tx.send(res);
            },
        );
        wk.takeSnapshotWithConfiguration_completionHandler(None, &block);
    }) {
        return err_unavailable(format!("canvas.snapshot 提交失败: {e}"));
    }
    match rx.recv_timeout(Duration::from_secs(6)) {
        Ok(Ok(bytes)) => {
            use base64::Engine;
            (
                true,
                json!({
                    "format": format,
                    "base64": base64::engine::general_purpose::STANDARD.encode(bytes),
                }),
            )
        }
        Ok(Err(e)) => err_unavailable(format!("canvas.snapshot 失败: {e}")),
        Err(_) => err_unavailable("canvas.snapshot 超时（6s）"),
    }
}
