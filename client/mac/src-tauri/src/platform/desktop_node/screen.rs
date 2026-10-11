//! openclaw desktop node 的截屏（screen.snapshot 命令执行）。
//!
//! 进程内 ScreenCaptureKit 单次截图（`SCScreenshotManager.captureImageInRect`）：权限弹框同意后
//! 同进程即时生效、不重启（与 OpenClaw.app 同属进程内 SC；screencapture 子进程的权限快照滞后，
//! 首次授权后要重启 client 才稳）。macOS<14（SCScreenshotManager 需 14+）或 SC 失败 → fallback
//! `screencapture` 命令；权限未授权不 fallback（fallback 会掩盖问题且子进程同样失败）。
//!
//! captureImageInRect 是 display-agnostic 的屏幕空间 rect 截图；多屏用 CoreGraphics FFI
//! （CGMainDisplayID / CGGetActiveDisplayList / CGDisplayBounds）取目标 display 的 rect。

#![allow(non_snake_case)]

use std::ffi::c_void;
use std::process::Command;
use std::sync::{mpsc, OnceLock};
use std::time::Duration;

use base64::Engine;

use objc2_core_foundation::CGRect;
use objc2_core_graphics::CGImage;
use objc2_foundation::NSError;
use objc2_screen_capture_kit::SCScreenshotManager;

use block2::RcBlock;

// ===== CoreGraphics FFI：display 枚举 + bounds + CGImage 尺寸 =====
#[link(name = "CoreGraphics", kind = "framework")]
extern "C" {
    fn CGMainDisplayID() -> u32;
    fn CGGetActiveDisplayList(max: u32, arr: *mut u32, cnt: *mut u32) -> bool;
    fn CGDisplayBounds(disp: u32) -> CGRect;
    pub fn CGImageGetWidth(image: *const c_void) -> usize;
    pub fn CGImageGetHeight(image: *const c_void) -> usize;
}

// ===== ImageIO + CoreFoundation FFI：CGImage → PNG bytes =====
// （pub：canvas.rs 的 snapshot 编码复用同一批符号，避免跨模块重复声明触发 redeclare 告警）
#[link(name = "ImageIO", kind = "framework")]
extern "C" {
    pub fn CGImageDestinationCreateWithData(
        data: *const c_void,
        uti: *const c_void,
        count: isize,
        options: *const c_void,
    ) -> *mut c_void;
    pub fn CGImageDestinationAddImage(
        dest: *mut c_void,
        image: *const c_void,
        props: *const c_void,
    ) -> bool;
    pub fn CGImageDestinationFinalize(dest: *mut c_void) -> bool;
}
#[link(name = "CoreFoundation", kind = "framework")]
extern "C" {
    pub fn CFDataCreateMutable(alloc: *const c_void, capacity: isize) -> *mut c_void;
    pub fn CFDataGetBytePtr(data: *const c_void) -> *const u8;
    pub fn CFDataGetLength(data: *const c_void) -> isize;
    pub fn CFStringCreateWithBytes(
        alloc: *const c_void,
        bytes: *const u8,
        num_bytes: isize,
        encoding: u32,
        is_external: bool,
    ) -> *mut c_void;
    pub fn CFRelease(cf: *const c_void);
}
// kCFStringEncodingUTF8 = 0x08000100
const CF_UTF8: u32 = 0x0800_0100;

/// 截屏结果（对应 openclaw screen.snapshot 返回 payload）。
pub struct Snapshot {
    /// 图像格式："png"
    pub format: &'static str,
    /// base64 编码的图像数据
    pub base64: String,
    pub width: u32,
    pub height: u32,
    /// displayFrameId：client node 自定格式（client screen.snapshot 返回 + client computer.act 验证，
    /// 自洽即可，不对齐 OpenClaw.app）。computer.act 的 displayFrameId 必须 echo 最近一次 snapshot 的。
    pub display_frame_id: String,
}

/// 截指定屏（screenIndex，0=主屏）。返回 Snapshot。
pub fn capture(screen_index: usize) -> Result<Snapshot, String> {
    // 权限：触发授权请求（首次弹框），但不硬性拦截——CGPreflightScreenCaptureAccess
    // 在重新签名/系统设置手动添加后可能误报 false（2026-08-14 实测授权后仍 false），
    // 实际截图成败才是权威判定（未授权时 SC 会返权限错误）。
    let _ = super::permissions::ensure_screen_recording(true);
    // macOS 14+ 走 SC（SCScreenshotManager 需 14+）；否则直接 fallback。
    if macos_major() >= 14 {
        match capture_via_sc(screen_index) {
            Ok(s) => return Ok(s),
            Err(e) => {
                // SC 失败（超时/异常等）→ fallback screencapture。权限已确保，子进程这次能成。
                eprintln!("[desktop_node] SC 截屏失败，回退 screencapture: {e}");
            }
        }
    }
    capture_via_screencapture(screen_index)
}

/// fallback：系统 `screencapture` 命令截屏（macOS<14 或 SC 失败时）。
fn capture_via_screencapture(screen_index: usize) -> Result<Snapshot, String> {
    let path = std::env::temp_dir().join(format!("oc_snap_{screen_index}.png"));
    let mut cmd = Command::new("screencapture");
    cmd.arg("-x").arg("-t png");
    // screencapture -D <display> 是 1-based；screenIndex 0(主) → 1
    cmd.arg("-D").arg((screen_index + 1).to_string());
    cmd.arg(&path);
    let out = cmd
        .output()
        .map_err(|e| format!("screencapture 启动失败: {e}"))?;
    if !out.status.success() {
        return Err(format!(
            "screencapture 失败: {}",
            String::from_utf8_lossy(&out.stderr)
        ));
    }
    let png = std::fs::read(&path).map_err(|e| format!("读截图失败: {e}"))?;
    let _ = std::fs::remove_file(&path);
    let (width, height) = parse_png_dimensions(&png).unwrap_or((0, 0));
    let b64 = base64::engine::general_purpose::STANDARD.encode(&png);
    Ok(Snapshot {
        format: "png",
        base64: b64,
        width,
        height,
        display_frame_id: format!("display-{screen_index}"),
    })
}

/// ScreenCaptureKit 单次截图（captureImageInRect）。同步阻塞等 completion（≤5s）。
fn capture_via_sc(screen_index: usize) -> Result<Snapshot, String> {
    let rect = unsafe { display_bounds(screen_index)? };
    // completion block 把结果（CGImage 指针 or 错误标志）通过 channel 回传。
    let (tx, rx) = mpsc::channel::<Result<*mut CGImage, String>>();
    let block = RcBlock::new(move |img: *mut CGImage, err: *mut NSError| {
        let res = if !err.is_null() {
            Err("SC capture 返回错误".to_string())
        } else if img.is_null() {
            Err("SC capture 返回空图像".to_string())
        } else {
            Ok(img)
        };
        let _ = tx.send(res);
    });
    unsafe {
        SCScreenshotManager::captureImageInRect_completionHandler(rect, Some(&block));
    }
    // completion 异步回调；阻塞等（protocol.rs 的 dispatch 本就是同步线程）。
    let img_ptr = rx
        .recv_timeout(Duration::from_secs(5))
        .map_err(|e| format!("SC completion 超时: {e}"))??;
    let png = unsafe { encode_png(img_ptr) }?;
    let (width, height) = unsafe {
        (
            CGImageGetWidth(img_ptr as *const c_void),
            CGImageGetHeight(img_ptr as *const c_void),
        )
    };
    Ok(Snapshot {
        format: "png",
        base64: base64::engine::general_purpose::STANDARD.encode(&png),
        width: width as u32,
        height: height as u32,
        display_frame_id: format!("display-{screen_index}"),
    })
}

/// 目标 display 的屏幕空间 rect（screen_index 0=主屏=CGMainDisplayID；其余按 active list 顺序）。
unsafe fn display_bounds(screen_index: usize) -> Result<CGRect, String> {
    let disp = if screen_index == 0 {
        CGMainDisplayID()
    } else {
        let mut arr = [0u32; 16];
        let mut cnt: u32 = 0;
        if !CGGetActiveDisplayList(16, arr.as_mut_ptr(), &mut cnt)
            || (cnt as usize) <= screen_index
        {
            return Err(format!("display {screen_index} 不存在（active={cnt}）"));
        }
        arr[screen_index]
    };
    Ok(CGDisplayBounds(disp))
}

/// CGImage → PNG bytes（ImageIO: CGImageDestination → CFMutableData）。
unsafe fn encode_png(img: *mut CGImage) -> Result<Vec<u8>, String> {
    let data = CFDataCreateMutable(std::ptr::null(), 0);
    if data.is_null() {
        return Err("CFDataCreateMutable null".into());
    }
    let uti_bytes = b"public.png";
    let uti = CFStringCreateWithBytes(
        std::ptr::null(),
        uti_bytes.as_ptr(),
        uti_bytes.len() as isize,
        CF_UTF8,
        false,
    );
    if uti.is_null() {
        CFRelease(data);
        return Err("CFStringCreateWithBytes null".into());
    }
    // 真实 C 签名 4 参（data, type, count, options）——漏声明 options 会让函数读到
    // 寄存器残留垃圾指针，内部 CFGetTypeID(options) 直接 SIGSEGV（2026-08-14 崩溃实锤）。
    let dest = CGImageDestinationCreateWithData(data, uti, 1, std::ptr::null());
    if dest.is_null() {
        CFRelease(uti);
        CFRelease(data);
        return Err("CGImageDestinationCreateWithData null".into());
    }
    CGImageDestinationAddImage(dest, img as *const c_void, std::ptr::null());
    let ok = CGImageDestinationFinalize(dest);
    CFRelease(dest);
    CFRelease(uti);
    if !ok {
        CFRelease(data);
        return Err("CGImageDestinationFinalize 失败".into());
    }
    let ptr = CFDataGetBytePtr(data);
    let len = CFDataGetLength(data) as usize;
    let bytes = std::slice::from_raw_parts(ptr, len).to_vec();
    CFRelease(data);
    Ok(bytes)
}

/// macOS 主版本号（缓存；sw_vers 只跑一次）。决定能否用 SCScreenshotManager（14+）。
fn macos_major() -> u32 {
    static V: OnceLock<u32> = OnceLock::new();
    *V.get_or_init(|| {
        Command::new("sw_vers")
            .arg("-productVersion")
            .output()
            .ok()
            .and_then(|o| String::from_utf8(o.stdout).ok())
            .and_then(|s| s.split('.').next().and_then(|m| m.parse().ok()))
            .unwrap_or(0)
    })
}

/// 解析 PNG IHDR 宽高（PNG 签名 8 + IHDR 长度/类型 8 + 宽 4 + 高 4，偏移 16/20）。
fn parse_png_dimensions(data: &[u8]) -> Option<(u32, u32)> {
    if data.len() < 24 {
        return None;
    }
    if &data[0..8] != b"\x89PNG\r\n\x1a\n" {
        return None;
    }
    let width = u32::from_be_bytes([data[16], data[17], data[18], data[19]]);
    let height = u32::from_be_bytes([data[20], data[21], data[22], data[23]]);
    Some((width, height))
}

// ===== screen.record（SCStream + SCRecordingOutput 录 mp4，macOS 14+）=====
//
// SCShareableContent 取 display[screenIndex] → SCContentFilter → SCStream 配置
// (width/height/minimumFrameInterval=1/fps/showsCursor/capturesAudio) + SCRecordingOutput
// （outputURL 临时 mp4，默认 H264/MPEG-4）→ addRecordingOutput → startCapture → 录 durationMs →
// stopCapture → 等 didFinishRecording（文件 finalize）→ std::fs::read base64。
// 无需 AVAssetWriter（SCRecordingOutput 直接录文件）。

use std::cell::OnceCell;

use objc2::{define_class, msg_send, AnyThread, DefinedClass};
use objc2::rc::Retained;
use objc2_core_media::CMTime;
use objc2_foundation::{NSArray, NSObject, NSObjectProtocol, NSString, NSURL};
use objc2_screen_capture_kit::{
    SCContentFilter, SCRecordingOutput, SCRecordingOutputConfiguration, SCRecordingOutputDelegate,
    SCShareableContent, SCStream, SCStreamConfiguration,
};

#[link(name = "CoreMedia", kind = "framework")]
extern "C" {
    fn CMTimeMake(value: i64, timescale: i32) -> CMTime;
}

struct RecDelegateIvars {
    finish_tx: OnceCell<mpsc::Sender<Result<(), String>>>,
}

define_class!(
    #[unsafe(super(NSObject))]
    #[ivars = RecDelegateIvars]
    struct RecDelegate;

    unsafe impl NSObjectProtocol for RecDelegate {}

    unsafe impl SCRecordingOutputDelegate for RecDelegate {
        #[unsafe(method(recordingOutput:didFailWithError:))]
        unsafe fn recordingOutput_didFailWithError(
            &self,
            _o: &SCRecordingOutput,
            error: &NSError,
        ) {
            if let Some(s) = self.ivars().finish_tx.get() {
                let _ = s.send(Err(format!("录制失败: {}", error.localizedDescription().to_string())));
            }
        }

        #[unsafe(method(recordingOutputDidFinishRecording:))]
        unsafe fn recordingOutputDidFinishRecording(&self, _o: &SCRecordingOutput) {
            // 停止 + 文件 finalize 完成；通知上层可读文件。
            if let Some(s) = self.ivars().finish_tx.get() {
                let _ = s.send(Ok(()));
            }
        }
    }
);

impl RecDelegate {
    fn new(finish_tx: mpsc::Sender<Result<(), String>>) -> Retained<Self> {
        let this = Self::alloc().set_ivars(RecDelegateIvars {
            finish_tx: OnceCell::from(finish_tx),
        });
        unsafe { msg_send![super(this), init] }
    }
}

fn rand_local_id() -> String {
    // 进程内单调 id（不用 Date/time 避免不必要的依赖）：静态计数器。
    use std::sync::atomic::{AtomicU64, Ordering as AOrdering};
    static C: AtomicU64 = AtomicU64::new(1);
    let n = C.fetch_add(1, AOrdering::Relaxed);
    // 拼进程 pid 保证多实例不撞。
    format!("{}-{}", std::process::id(), n)
}

/// screen.record 录屏。返回 (ok, Value)。
pub fn record(params: &serde_json::Value) -> (bool, serde_json::Value) {
    use std::thread;
    use std::time::Instant;

    // 同 capture：触发授权请求但不硬拦（preflight 可能误报），实际录制成败为准。
    let _ = super::permissions::ensure_screen_recording(true);
    let duration_ms = params
        .get("durationMs")
        .and_then(|v| v.as_u64())
        .unwrap_or(10000)
        .clamp(1, 60000);
    let fps = params
        .get("fps")
        .and_then(|v| v.as_u64())
        .unwrap_or(10)
        .clamp(1, 60) as i32;
    let screen_index = params
        .get("screenIndex")
        .and_then(|v| v.as_u64())
        .unwrap_or(0) as usize;
    let include_audio = params
        .get("includeAudio")
        .and_then(|v| v.as_bool())
        .unwrap_or(false);

    let tmp = std::env::temp_dir().join(format!("oc-screen-record-{}.mp4", rand_local_id()));
    let tmp_path = tmp.to_string_lossy().to_string();
    let tmp_for_thread = tmp_path.clone();

    // (base64, width, height)；线程跑完整 SCStream 流程后 send。
    let (tx, rx) = mpsc::channel::<Result<(String, usize, usize), String>>();
    let (finish_tx, finish_rx) = mpsc::channel::<Result<(), String>>();

    thread::spawn(move || unsafe {
        // 1. getShareableContent（异步 block → channel 等 5s）
        let content = match wait_shareable_content() {
            Ok(c) => c,
            Err(e) => {
                let _ = tx.send(Err(e));
                return;
            }
        };
        let displays = content.displays();
        let count = displays.count() as usize;
        if screen_index >= count {
            let _ = tx.send(Err(format!("screenIndex {screen_index} 超出（共 {count} 屏）")));
            return;
        }
        let display = displays.objectAtIndex(screen_index as _);
        let dwidth = display.width() as usize;
        let dheight = display.height() as usize;

        // 2. filter（排除窗口列表为空 = 录整屏）
        let empty: Retained<NSArray<objc2_screen_capture_kit::SCWindow>> = NSArray::new();
        let filter = SCContentFilter::initWithDisplay_excludingWindows(
            SCContentFilter::alloc(),
            &display,
            &empty,
        );

        // 3. config
        let config = SCStreamConfiguration::new();
        config.setWidth(dwidth);
        config.setHeight(dheight);
        let interval = CMTimeMake(1, fps);
        config.setMinimumFrameInterval(interval);
        config.setShowsCursor(true);
        if include_audio {
            config.setCapturesAudio(true);
        }

        // 4. recording output（outputURL；codec/fileType 默认 H264/MPEG-4）
        let url = NSURL::fileURLWithPath(&NSString::from_str(&tmp_for_thread));
        let out_cfg = SCRecordingOutputConfiguration::new();
        out_cfg.setOutputURL(&url);

        // 5. delegate + recording output
        let delegate = RecDelegate::new(finish_tx);
        let delegate_ref: &objc2::runtime::ProtocolObject<dyn SCRecordingOutputDelegate> =
            objc2::runtime::ProtocolObject::from_ref(&*delegate);
        let recording = SCRecordingOutput::initWithConfiguration_delegate(
            SCRecordingOutput::alloc(),
            &out_cfg,
            delegate_ref,
        );

        // 6. stream + addRecordingOutput
        let stream = SCStream::initWithFilter_configuration_delegate(
            SCStream::alloc(),
            &filter,
            &config,
            None,
        );
        if let Err(e) = stream.addRecordingOutput_error(&recording) {
            let _ = tx.send(Err(format!("addRecordingOutput: {}", e.localizedDescription().to_string())));
            return;
        }

        // 7. startCapture（异步 block 等）
        if let Err(e) = start_capture(&stream) {
            let _ = tx.send(Err(format!("startCapture: {e}")));
            return;
        }

        // 8. 录 durationMs（SCStream 回调在 dispatch queue，不需本线程 RunLoop）
        let deadline = Instant::now() + std::time::Duration::from_millis(duration_ms);
        while Instant::now() < deadline {
            std::thread::sleep(std::time::Duration::from_millis(100));
        }

        // 9. stopCapture
        let stop_res = stop_capture(&stream);

        // 10. 等 didFinishRecording（文件 finalize）5s；超时也尝试读文件（stop 已成）
        let _ = finish_rx.recv_timeout(std::time::Duration::from_secs(5));
        if let Err(e) = stop_res {
            let _ = tx.send(Err(format!("stopCapture: {e}")));
            return;
        }

        // 11. 读文件 base64
        match std::fs::read(&tmp_for_thread) {
            Ok(bytes) => {
                let b64 = base64::engine::general_purpose::STANDARD.encode(&bytes);
                let _ = tx.send(Ok((b64, dwidth, dheight)));
            }
            Err(e) => {
                let _ = tx.send(Err(format!("读录屏文件失败: {e}")));
            }
        }
    });

    match rx.recv_timeout(std::time::Duration::from_millis(duration_ms + 15000)) {
        Ok(Ok((b64, w, h))) => {
            let _ = std::fs::remove_file(&tmp_path);
            (
                true,
                serde_json::json!({
                    "format": "mp4",
                    "base64": b64,
                    "durationMs": duration_ms,
                    "fps": fps,
                    "screenIndex": screen_index,
                    "hasAudio": include_audio,
                    "width": w,
                    "height": h
                }),
            )
        }
        Ok(Err(e)) => {
            let _ = std::fs::remove_file(&tmp_path);
            (false, serde_json::json!({"error": {"message": e}}))
        }
        Err(_) => {
            let _ = std::fs::remove_file(&tmp_path);
            (
                false,
                serde_json::json!({"error": {"code": "TIMEOUT", "message": "录屏超时"}}),
            )
        }
    }
}

/// getShareableContent 异步 → 同步等 SCShareableContent。
fn wait_shareable_content() -> Result<Retained<SCShareableContent>, String> {
    let (tx, rx) = mpsc::channel::<Result<Retained<SCShareableContent>, String>>();
    let block = block2::RcBlock::new(
        move |c: *mut SCShareableContent, e: *mut objc2_foundation::NSError| {
            if !e.is_null() {
                let _ = tx.send(Err(format!(
                    "getShareableContent: {}",
                    unsafe { &*e }.localizedDescription().to_string()
                )));
            } else if c.is_null() {
                let _ = tx.send(Err("getShareableContent 返回空".into()));
            } else {
                let owned = unsafe { Retained::retain(c).expect("retain SCShareableContent") };
                let _ = tx.send(Ok(owned));
            }
        },
    );
    unsafe {
        SCShareableContent::getShareableContentWithCompletionHandler(&block);
    }
    rx.recv_timeout(std::time::Duration::from_secs(5))
        .map_err(|e| format!("getShareableContent 超时: {e}"))?
}

/// startCaptureWithCompletionHandler 异步 → 同步等（错误 nil 即成功）。
fn start_capture(stream: &SCStream) -> Result<(), String> {
    let (tx, rx) = mpsc::channel::<*mut objc2_foundation::NSError>();
    let block = block2::RcBlock::new(move |e: *mut objc2_foundation::NSError| {
        let _ = tx.send(e);
    });
    unsafe {
        stream.startCaptureWithCompletionHandler(Some(&block));
    }
    let e = rx
        .recv_timeout(std::time::Duration::from_secs(5))
        .map_err(|e| format!("startCapture 超时: {e}"))?;
    if e.is_null() {
        Ok(())
    } else {
        Err(unsafe { &*e }.localizedDescription().to_string())
    }
}

/// stopCaptureWithCompletionHandler 异步 → 同步等。
fn stop_capture(stream: &SCStream) -> Result<(), String> {
    let (tx, rx) = mpsc::channel::<*mut objc2_foundation::NSError>();
    let block = block2::RcBlock::new(move |e: *mut objc2_foundation::NSError| {
        let _ = tx.send(e);
    });
    unsafe {
        stream.stopCaptureWithCompletionHandler(Some(&block));
    }
    let e = rx
        .recv_timeout(std::time::Duration::from_secs(5))
        .map_err(|e| format!("stopCapture 超时: {e}"))?;
    if e.is_null() {
        Ok(())
    } else {
        Err(unsafe { &*e }.localizedDescription().to_string())
    }
}
