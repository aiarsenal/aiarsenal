//! camera.list（列摄像头）+ camera.snap（拍照 base64 jpg，对齐 openclaw node 协议）。
//!
//! camera.snap params: facing?("front"|"back"), delayMs?(2000), maxWidth?(1600),
//!                     quality?(0.9), format?("jpg"), deviceId?
//! result: {format:"jpg", base64, width, height}
//!
//! AVCaptureSession（专用线程 + CFRunLoop）：device input + photo output → startRunning →
//! 等 delayMs（自动曝光稳定）→ capturePhotoWithSettings:delegate:（AVCapturePhotoSettings
//! 默认即 JPEG）→ AVCapturePhotoCaptureDelegate 回调拿 AVCapturePhoto.fileDataRepresentation
//! (NSData) → base64。尺寸走 msg_send![photo, pixelSize] (CGSize)。

#![allow(non_snake_case)]

use std::cell::OnceCell;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Arc};
use std::thread;
use std::time::{Duration, Instant};

use base64::Engine;
use objc2::rc::Retained;
use objc2::runtime::ProtocolObject;
use objc2::{define_class, msg_send, AnyThread, DefinedClass};
use objc2_av_foundation::{
    AVCaptureConnection, AVCaptureDevice, AVCaptureDeviceInput, AVCaptureDevicePosition,
    AVCaptureFileOutput, AVCaptureFileOutputRecordingDelegate, AVCaptureMovieFileOutput,
    AVCapturePhoto, AVCapturePhotoCaptureDelegate, AVCapturePhotoOutput, AVCapturePhotoSettings,
    AVCaptureSession, AVMediaTypeAudio, AVMediaTypeVideo,
};
use objc2_core_foundation::CGSize;
use objc2_foundation::{NSArray, NSError, NSObject, NSObjectProtocol, NSString, NSURL};
use serde_json::{json, Value};

// ===== camera.list =====

/// camera.list：列视频采集设备 [{id, name, position}]。
pub fn list() -> (bool, Value) {
    let mut devices = Vec::new();
    unsafe {
        if let Some(mt) = AVMediaTypeVideo.as_ref() {
            // devicesWithMediaType 官方 deprecated（建议 DiscoverySession），列设备够用最简。
            #[allow(deprecated)]
            let arr = AVCaptureDevice::devicesWithMediaType(mt);
            for d in arr.iter() {
                devices.push(json!({
                    "id": d.uniqueID().to_string(),
                    "name": d.localizedName().to_string(),
                    "position": position_str(d.position()),
                }));
            }
        }
    }
    (true, json!({ "devices": devices }))
}

fn position_str(p: AVCaptureDevicePosition) -> &'static str {
    if p == AVCaptureDevicePosition::Back {
        "back"
    } else if p == AVCaptureDevicePosition::Front {
        "front"
    } else {
        "unspecified"
    }
}

// ===== CFRunLoop 裸 FFI（专用线程跑 session + 等 delegate 回调，同 location.rs）=====
#[link(name = "CoreFoundation", kind = "framework")]
extern "C" {
    static kCFRunLoopDefaultMode: *const std::ffi::c_void;
    fn CFRunLoopRunInMode(mode: *const std::ffi::c_void, seconds: f64, ret: bool) -> i32;
    fn CFRunLoopGetCurrent() -> *const std::ffi::c_void;
    fn CFRunLoopStop(rl: *const std::ffi::c_void);
}

// ===== camera.snap =====

struct PhotoData {
    base64: String,
    width: u32,
    height: u32,
}

struct PhotoDelegateIvars {
    tx: OnceCell<mpsc::Sender<Result<PhotoData, String>>>,
    done: Arc<AtomicBool>,
}

define_class!(
    #[unsafe(super(NSObject))]
    #[ivars = PhotoDelegateIvars]
    struct PhotoDelegate;

    unsafe impl NSObjectProtocol for PhotoDelegate {}

    unsafe impl AVCapturePhotoCaptureDelegate for PhotoDelegate {
        // 主回调：拍照完成（photo 非 nil）或失败（error 非 nil）。全 optional 协议，实现这一个够。
        #[unsafe(method(captureOutput:didFinishProcessingPhoto:error:))]
        unsafe fn captureOutput_didFinishProcessingPhoto_error(
            &self,
            _output: &AVCapturePhotoOutput,
            photo: &AVCapturePhoto,
            error: Option<&NSError>,
        ) {
            let ivars = self.ivars();
            let result: Result<PhotoData, String> = if let Some(e) = error {
                Err(format!(
                    "拍照失败: {}",
                    e.localizedDescription().to_string()
                ))
            } else {
                let data = unsafe { photo.fileDataRepresentation() };
                match data {
                    Some(d) => {
                        let len: usize = unsafe { msg_send![&d, length] };
                        let ptr: *const std::ffi::c_void = unsafe { msg_send![&d, bytes] };
                        let slice = unsafe { std::slice::from_raw_parts(ptr as *const u8, len) };
                        let b64 = base64::engine::general_purpose::STANDARD.encode(slice);
                        let size: CGSize = unsafe { msg_send![photo, pixelSize] };
                        Ok(PhotoData {
                            base64: b64,
                            width: size.width as u32,
                            height: size.height as u32,
                        })
                    }
                    None => Err("无图数据（fileDataRepresentation 为空）".into()),
                }
            };
            if let Some(s) = ivars.tx.get() {
                let _ = s.send(result);
            }
            ivars.done.store(true, Ordering::Relaxed);
        }
    }
);

impl PhotoDelegate {
    fn new(tx: mpsc::Sender<Result<PhotoData, String>>, done: Arc<AtomicBool>) -> Retained<Self> {
        let this = Self::alloc().set_ivars(PhotoDelegateIvars {
            tx: OnceCell::from(tx),
            done,
        });
        unsafe { msg_send![super(this), init] }
    }
}

/// 按 facing 选设备（精确匹配 front/back，找不到退第一个可用设备）。
fn select_device(facing: &str) -> Option<Retained<AVCaptureDevice>> {
    let mt = unsafe { AVMediaTypeVideo.as_ref() }?;
    let want_front = facing == "front";
    #[allow(deprecated)]
    let arr = unsafe { AVCaptureDevice::devicesWithMediaType(mt) };
    let mut fallback = None;
    for d in arr.iter() {
        let p = unsafe { d.position() };
        if (want_front && p == AVCaptureDevicePosition::Front)
            || (!want_front && p == AVCaptureDevicePosition::Back)
        {
            return Some(d);
        }
        if fallback.is_none() {
            fallback = Some(d);
        }
    }
    fallback
}

pub fn snap(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_camera(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: camera（请到 系统设置→隐私与安全→摄像头 勾选本 App）"}}),
        );
    }
    let facing = params
        .get("facing")
        .and_then(|v| v.as_str())
        .unwrap_or("front");
    let delay_ms = params.get("delayMs").and_then(|v| v.as_u64()).unwrap_or(2000);
    // maxWidth/quality：当前返原始 JPEG（photoSettings 默认），桌面端暂不缩放重编码。

    let (tx, rx) = mpsc::channel::<Result<PhotoData, String>>();
    let done = Arc::new(AtomicBool::new(false));
    let facing_s = facing.to_string();

    thread::spawn(move || unsafe {
        let device = match select_device(&facing_s) {
            Some(d) => d,
            None => {
                let _ = tx.send(Err(format!("找不到 {facing_s} 摄像头")));
                return;
            }
        };
        let input = match AVCaptureDeviceInput::deviceInputWithDevice_error(&device) {
            Ok(i) => i,
            Err(e) => {
                let _ = tx.send(Err(format!(
                    "创建摄像头输入失败: {}",
                    e.localizedDescription().to_string()
                )));
                return;
            }
        };
        let session = AVCaptureSession::new();
        session.beginConfiguration();
        session.addInput(&input);
        let output = AVCapturePhotoOutput::new();
        session.addOutput(&output);
        session.commitConfiguration();
        session.startRunning(); // 阻塞至启动完成

        // 等自动曝光/白平衡稳定（openclaw macOS 默认 delayMs=2000）
        if delay_ms > 0 {
            let deadline = Instant::now() + Duration::from_millis(delay_ms);
            while Instant::now() < deadline {
                CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.2, false);
            }
        }

        let delegate = PhotoDelegate::new(tx, done.clone());
        let delegate_ref: &ProtocolObject<dyn AVCapturePhotoCaptureDelegate> =
            ProtocolObject::from_ref(&*delegate);
        let settings = AVCapturePhotoSettings::photoSettings(); // 默认 JPEG
        output.capturePhotoWithSettings_delegate(&settings, delegate_ref);

        let rl = CFRunLoopGetCurrent();
        let deadline = Instant::now() + Duration::from_secs(15);
        while Instant::now() < deadline && !done.load(Ordering::Relaxed) {
            CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.5, false);
        }
        CFRunLoopStop(rl);
        session.stopRunning();
        let _ = session;
        let _ = output;
        let _ = input;
        let _ = delegate;
    });

    match rx.recv_timeout(Duration::from_secs(20)) {
        Ok(Ok(d)) => (
            true,
            json!({"format":"jpg","base64":d.base64,"width":d.width,"height":d.height}),
        ),
        Ok(Err(e)) => (false, json!({"error": {"message": e}})),
        Err(_) => (
            false,
            json!({"error": {"code": "TIMEOUT", "message": "拍照超时"}}),
        ),
    }
}

// ===== camera.clip（录 mp4 base64，对齐 openclaw node 协议）=====
//
// camera.clip params: facing?, durationMs?(3000), includeAudio?(true), format("mp4"), deviceId?
// result: {format:"mp4", base64, durationMs, hasAudio}
//
// AVCaptureSession + AVCaptureDeviceInput(video，可选 audio) + AVCaptureMovieFileOutput
// → startRecordingToOutputFileURL:recordingDelegate: → 录 durationMs → stopRecording
// → AVCaptureFileOutputRecordingDelegate 回调（文件 finalize）→ 读文件 base64。
// 无需 CFRunLoop：回调在 session 内部 queue 派发，专用线程 sleep 计时即可。

struct ClipDelegateIvars {
    finish_tx: OnceCell<mpsc::Sender<Result<(), String>>>,
}

define_class!(
    #[unsafe(super(NSObject))]
    #[ivars = ClipDelegateIvars]
    struct ClipDelegate;

    unsafe impl NSObjectProtocol for ClipDelegate {}

    unsafe impl AVCaptureFileOutputRecordingDelegate for ClipDelegate {
        // 录制结束（stopRecording 后触发）：error=nil 即成功。
        #[unsafe(method(captureOutput:didFinishRecordingToOutputFileAtURL:fromConnections:error:))]
        unsafe fn captureOutput_didFinishRecordingToOutputFileAtURL_fromConnections_error(
            &self,
            _output: &AVCaptureFileOutput,
            _url: &NSURL,
            _connections: &NSArray<AVCaptureConnection>,
            error: Option<&NSError>,
        ) {
            let res = match error {
                Some(e) => Err(format!("录像失败: {}", e.localizedDescription().to_string())),
                None => Ok(()),
            };
            if let Some(s) = self.ivars().finish_tx.get() {
                let _ = s.send(res);
            }
        }
    }
);

impl ClipDelegate {
    fn new(finish_tx: mpsc::Sender<Result<(), String>>) -> Retained<Self> {
        let this = Self::alloc().set_ivars(ClipDelegateIvars {
            finish_tx: OnceCell::from(finish_tx),
        });
        unsafe { msg_send![super(this), init] }
    }
}

fn clip_local_id() -> String {
    use std::sync::atomic::{AtomicU64, Ordering as AOrdering};
    static C: AtomicU64 = AtomicU64::new(1);
    let n = C.fetch_add(1, AOrdering::Relaxed);
    format!("{}-{}", std::process::id(), n)
}

pub fn clip(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_camera(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: camera（请到 系统设置→隐私与安全→摄像头 勾选本 App）"}}),
        );
    }
    let facing = params.get("facing").and_then(|v| v.as_str()).unwrap_or("front");
    let duration_ms = params
        .get("durationMs")
        .and_then(|v| v.as_u64())
        .unwrap_or(3000)
        .clamp(500, 60000);
    let include_audio = params.get("includeAudio").and_then(|v| v.as_bool()).unwrap_or(true);

    let tmp = std::env::temp_dir().join(format!("oc-camera-clip-{}.mp4", clip_local_id()));
    let tmp_path = tmp.to_string_lossy().to_string();
    let tmp_for_thread = tmp_path.clone();

    // 线程跑完录制流程后 send Ok(())；主线程读文件。
    let (tx, rx) = mpsc::channel::<Result<(), String>>();
    let (finish_tx, finish_rx) = mpsc::channel::<Result<(), String>>();
    let facing_s = facing.to_string();

    thread::spawn(move || unsafe {
        let device = match select_device(&facing_s) {
            Some(d) => d,
            None => {
                let _ = tx.send(Err(format!("找不到 {facing_s} 摄像头")));
                return;
            }
        };
        let input = match AVCaptureDeviceInput::deviceInputWithDevice_error(&device) {
            Ok(i) => i,
            Err(e) => {
                let _ = tx.send(Err(format!(
                    "创建摄像头输入失败: {}",
                    e.localizedDescription().to_string()
                )));
                return;
            }
        };
        let session = AVCaptureSession::new();
        session.beginConfiguration();
        session.addInput(&input);

        // 可选麦克风：AVMediaTypeAudio defaultDevice + input。
        if include_audio {
            if let Some(mt) = AVMediaTypeAudio.as_ref() {
                if let Some(audio_dev) = AVCaptureDevice::defaultDeviceWithMediaType(mt) {
                    if let Ok(audio_input) = AVCaptureDeviceInput::deviceInputWithDevice_error(&audio_dev)
                    {
                        session.addInput(&audio_input);
                    }
                }
            }
        }

        let movie_output = AVCaptureMovieFileOutput::new();
        session.addOutput(&movie_output);
        session.commitConfiguration();
        session.startRunning();

        let delegate = ClipDelegate::new(finish_tx);
        let delegate_ref: &ProtocolObject<dyn AVCaptureFileOutputRecordingDelegate> =
            ProtocolObject::from_ref(&*delegate);
        let url = NSURL::fileURLWithPath(&NSString::from_str(&tmp_for_thread));
        movie_output.startRecordingToOutputFileURL_recordingDelegate(&url, delegate_ref);

        // 录 durationMs
        std::thread::sleep(Duration::from_millis(duration_ms));

        movie_output.stopRecording();

        // 等 delegate 回调（文件 finalize）
        let res = match finish_rx.recv_timeout(Duration::from_secs(5)) {
            Ok(Ok(())) => Ok(()),
            Ok(Err(e)) => Err(e),
            Err(e) => Err(format!("录像回调超时: {e}")),
        };
        session.stopRunning();
        let _ = tx.send(res);
        // 保活：让 session/output/delegate 存活到本线程结束（回调依赖）。
        let _ = session;
        let _ = movie_output;
        let _ = input;
        let _ = delegate;
    });

    match rx.recv_timeout(Duration::from_millis(duration_ms + 10000)) {
        Ok(Ok(())) => match std::fs::read(&tmp_path) {
            Ok(bytes) => {
                let b64 = base64::engine::general_purpose::STANDARD.encode(&bytes);
                let _ = std::fs::remove_file(&tmp_path);
                (
                    true,
                    json!({"format":"mp4","base64":b64,"durationMs":duration_ms,"hasAudio":include_audio}),
                )
            }
            Err(e) => {
                let _ = std::fs::remove_file(&tmp_path);
                (
                    false,
                    json!({"error": {"message": format!("读录像文件失败: {e}")}}),
                )
            }
        },
        Ok(Err(e)) => {
            let _ = std::fs::remove_file(&tmp_path);
            (false, json!({"error": {"message": e}}))
        }
        Err(_) => {
            let _ = std::fs::remove_file(&tmp_path);
            (
                false,
                json!({"error": {"code": "TIMEOUT", "message": "录像超时"}}),
            )
        }
    }
}
