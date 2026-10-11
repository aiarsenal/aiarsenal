//! location.get 命令（一次性 GPS 定位，对齐 openclaw node 协议）。
//!
//! params: timeoutMs?(10000), desiredAccuracy?("coarse"|"balanced"|"precise")
//! result: lat, lon, accuracyMeters?, altitudeMeters?, speedMps?, headingDeg?,
//!         timestamp(ISO8601), source:"coreLocation"
//!
//! CLLocation 的字段方法在 objc2-core-location 0.3.2 未生成 → 用 msg_send! 提取。
//! 定位是异步（CLLocationManager delegate 回调），需 RunLoop。invoke 在 ws 线程无 RunLoop
//! → spawn 专用线程，线程内创建 manager + setDelegate + requestLocation + 跑 CFRunLoop，
//! delegate 回调（同线程 RunLoop 派发）通过 channel 把结果传回 invoke 线程.

#![allow(non_snake_case)]

use std::cell::OnceCell;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Arc};
use std::thread;
use std::time::{Duration, Instant};

use objc2::rc::Retained;
use objc2::runtime::ProtocolObject;
use objc2::{define_class, msg_send, AnyThread, DefinedClass};
use objc2_core_location::{
    CLAuthorizationStatus, CLLocation, CLLocationCoordinate2D, CLLocationManager,
    CLLocationManagerDelegate,
};
use objc2_foundation::{NSArray, NSDate, NSError, NSObject, NSObjectProtocol};
use serde_json::{json, Value};

/// location 模块内联日志（走全局 util::log）。
fn llog(msg: &str) {
    aiarsenal_client_core::util::log("info", &format!("[desktop_node] [location] {msg}"));
}

// CFRunLoop 裸 FFI（专用线程跑 RunLoop 等 delegate 回调）。
#[link(name = "CoreFoundation", kind = "framework")]
extern "C" {
    static kCFRunLoopDefaultMode: *const std::ffi::c_void;
    fn CFRunLoopRunInMode(
        mode: *const std::ffi::c_void,
        seconds: f64,
        return_after_source_handled: bool,
    ) -> i32;
    fn CFRunLoopGetCurrent() -> *const std::ffi::c_void;
    fn CFRunLoopStop(rl: *const std::ffi::c_void);
}

/// 提取后的定位数据（回调线程 → invoke 线程）。
struct LocationData {
    lat: f64,
    lon: f64,
    accuracy: f64,  // -1 无效
    altitude: f64,
    speed: f64,     // -1 无效
    course: f64,    // -1 无效
    ts_secs: f64,
}

impl LocationData {
    fn to_json(&self) -> Value {
        let mut v = json!({
            "lat": self.lat,
            "lon": self.lon,
            "altitudeMeters": self.altitude,
            "timestamp": epoch_to_iso(self.ts_secs),
            "source": "coreLocation",
        });
        if self.accuracy >= 0.0 {
            v["accuracyMeters"] = json!(self.accuracy);
        }
        if self.speed >= 0.0 {
            v["speedMps"] = json!(self.speed);
        }
        if self.course >= 0.0 {
            v["headingDeg"] = json!(self.course);
        }
        v
    }
}

/// epoch 秒 → UTC ISO8601 "YYYY-MM-DDTHH:MM:SSZ"（Howard Hinnant civil_from_days 算法）。
fn epoch_to_iso(secs: f64) -> String {
    let total = secs as i64;
    let s = total.rem_euclid(60);
    let m = total.div_euclid(60).rem_euclid(60);
    let h = total.div_euclid(3600).rem_euclid(24);
    let days = total.div_euclid(86400);
    let (y, mo, d) = civil_from_days(days);
    format!("{y:04}-{mo:02}-{d:02}T{h:02}:{m:02}:{s:02}Z")
}

fn civil_from_days(z: i64) -> (i64, i64, i64) {
    let z = z + 719468;
    let era = (if z >= 0 { z } else { z - 146096 }) / 146097;
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    (if m <= 2 { y + 1 } else { y }, m, d)
}

// ===== LocationDelegate：define_class! 实现 CLLocationManagerDelegate =====

struct LocationDelegateIvars {
    tx: OnceCell<mpsc::Sender<Result<LocationData, String>>>,
    done: Arc<AtomicBool>,
}

define_class!(
    #[unsafe(super(NSObject))]
    #[ivars = LocationDelegateIvars]
    struct LocationDelegate;

    unsafe impl NSObjectProtocol for LocationDelegate {}

    unsafe impl CLLocationManagerDelegate for LocationDelegate {
        #[unsafe(method(locationManager:didUpdateLocations:))]
        unsafe fn locationManager_didUpdateLocations(
            &self,
            _manager: &CLLocationManager,
            locations: &NSArray<CLLocation>,
        ) {
            if let Some(loc) = locations.lastObject() {
                let data = extract_location(&loc);
                if let Some(s) = self.ivars().tx.get() {
                    let _ = s.send(Ok(data));
                }
                self.ivars().done.store(true, Ordering::Relaxed);
            }
        }

        #[unsafe(method(locationManager:didFailWithError:))]
        unsafe fn locationManager_didFailWithError(
            &self,
            _manager: &CLLocationManager,
            error: &NSError,
        ) {
            let msg = error.localizedDescription().to_string();
            if let Some(s) = self.ivars().tx.get() {
                let _ = s.send(Err(format!("定位失败: {msg}")));
            }
            self.ivars().done.store(true, Ordering::Relaxed);
        }

        /// 授权态变化（用户在系统弹框点了 允许/拒绝）。授权成功 → 自动 requestLocation；
        /// 拒绝/受限 → 回报错误。NotDetermined（用户还没操作）→ 继续等。
        /// 这是「首次调用即弹框 + 点允许后自动出坐标」的关键：requestWhenInUseAuthorization 的框
        /// 绑定**存活**的 manager，本线程 RunLoop 期间 manager 常驻，框才会真正出现并在此回调。
        #[unsafe(method(locationManagerDidChangeAuthorization:))]
        unsafe fn locationManagerDidChangeAuthorization(&self, manager: &CLLocationManager) {
            let status = unsafe { manager.authorizationStatus() };
            if status == CLAuthorizationStatus::AuthorizedAlways
                || status == CLAuthorizationStatus::AuthorizedWhenInUse
            {
                llog("授权变更→已授权，requestLocation");
                unsafe { manager.requestLocation() };
            } else if status == CLAuthorizationStatus::Denied
                || status == CLAuthorizationStatus::Restricted
            {
                llog("授权变更→拒绝/受限");
                if let Some(s) = self.ivars().tx.get() {
                    let _ = s.send(Err(
                        "PERMISSION_DENIED: location（用户在弹框拒绝/受限）".into(),
                    ));
                }
                self.ivars().done.store(true, Ordering::Relaxed);
            }
            // NotDetermined：用户尚未操作，继续等 RunLoop。
        }
    }
);

impl LocationDelegate {
    fn new(
        tx: mpsc::Sender<Result<LocationData, String>>,
        done: Arc<AtomicBool>,
    ) -> Retained<Self> {
        let this = Self::alloc().set_ivars(LocationDelegateIvars {
            tx: OnceCell::from(tx),
            done,
        });
        unsafe { msg_send![super(this), init] }
    }
}

/// CLLocation → LocationData（msg_send! 提取；objc2-core-location 0.3.2 未生成 CLLocation 方法）。
fn extract_location(loc: &CLLocation) -> LocationData {
    let coord: CLLocationCoordinate2D = unsafe { msg_send![loc, coordinate] };
    let altitude: f64 = unsafe { msg_send![loc, altitude] };
    let accuracy: f64 = unsafe { msg_send![loc, horizontalAccuracy] };
    let speed: f64 = unsafe { msg_send![loc, speed] };
    let course: f64 = unsafe { msg_send![loc, course] };
    let ts: Retained<NSDate> = unsafe { msg_send![loc, timestamp] };
    let ts_secs: f64 = unsafe { msg_send![&ts, timeIntervalSince1970] };
    LocationData {
        lat: coord.latitude,
        lon: coord.longitude,
        accuracy,
        altitude,
        speed,
        course,
        ts_secs,
    }
}

/// location.get：专用线程内查权限 → 必要时 requestWhenInUseAuthorization（manager 常驻 RunLoop，
/// 系统框才会真正出现）→ 用户授权后 locationManagerDidChangeAuthorization: 回调自动 requestLocation
/// → delegate 回 location 数据经 channel 返回。
///
/// 关键：CoreLocation 的授权框绑定**存活**的 CLLocationManager 实例。早期版本在 ensure_location 里
/// `new()` 完即 drop manager，导致 macOS 在框弹出前就取消了请求（这正是 location.get 首次不弹框、
/// 而通讯录/日历/照片/摄像头能弹的根因——后几者用 completion-handler 式 requestAccess，由 TCC daemon
/// 独立持有，不依赖对象存活）。现 manager 在本线程的 RunLoop 期间常驻，框才会真正出现。
pub fn get(params: &Value) -> (bool, Value) {
    let timeout_ms = params.get("timeoutMs").and_then(|v| v.as_u64()).unwrap_or(10000);
    let timeout = Duration::from_millis(timeout_ms);

    let (tx, rx) = mpsc::channel::<Result<LocationData, String>>();
    let done = Arc::new(AtomicBool::new(false));

    thread::spawn({
        let done = done.clone();
        // 已拒绝/受限时直接回报（不经 delegate 回调，因为授权态不会"变化"）。
        let pre_tx = tx.clone();
        move || unsafe {
            let manager = CLLocationManager::new();
            let delegate = LocationDelegate::new(tx, done.clone());
            let delegate_ref: &ProtocolObject<dyn CLLocationManagerDelegate> =
                ProtocolObject::from_ref(&*delegate);
            manager.setDelegate(Some(delegate_ref));

            let status = manager.authorizationStatus();
            match status {
                CLAuthorizationStatus::AuthorizedAlways
                | CLAuthorizationStatus::AuthorizedWhenInUse => {
                    llog("已授权 → requestLocation");
                    manager.requestLocation();
                }
                CLAuthorizationStatus::Denied | CLAuthorizationStatus::Restricted => {
                    llog("已拒绝/受限，不弹框（去 系统设置→隐私与安全→定位 手动开）");
                    let _ = pre_tx.send(Err(
                        "PERMISSION_DENIED: location（请到 系统设置→隐私与安全→定位 勾选本 App）".into(),
                    ));
                    done.store(true, Ordering::Relaxed);
                }
                _ => {
                    // NotDetermined：manager 在本线程 RunLoop 常驻，框会真正出现；
                    // 用户授权后 delegate 的 locationManagerDidChangeAuthorization: 自动 requestLocation。
                    if super::permissions::perm_disabled("location") {
                        llog("定位权限已被用户在托盘关闭，不弹框");
                        let _ = pre_tx.send(Err(
                            "PERMISSION_MISSING: location（已在客户端托盘关闭，可在托盘「权限」菜单重新开启）"
                                .into(),
                        ));
                        done.store(true, Ordering::Relaxed);
                    } else {
                        llog("NotDetermined → requestWhenInUseAuthorization（系统应弹定位框）");
                        manager.requestWhenInUseAuthorization();
                    }
                }
            }

            let rl = CFRunLoopGetCurrent();
            let deadline = Instant::now() + timeout;
            while Instant::now() < deadline && !done.load(Ordering::Relaxed) {
                CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.5, false);
            }
            CFRunLoopStop(rl);
            // manager/delegate 在块结束 drop（先 setDelegate(Some) 强引用 delegate 到 manager）
            let _ = manager;
            let _ = delegate;
        }
    });

    match rx.recv_timeout(timeout + Duration::from_secs(2)) {
        Ok(Ok(d)) => (true, d.to_json()),
        Ok(Err(e)) => (false, json!({ "error": { "code": "UNAVAILABLE", "message": e } })),
        Err(_) => (
            false,
            json!({ "error": { "code": "TIMEOUT", "message": "定位超时（首次调用需在系统弹框点「允许」，授权回调即自动触发定位；或重试）" } }),
        ),
    }
}
