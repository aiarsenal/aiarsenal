//! openclaw desktop node 的 TCC 权限（Screen Recording + Accessibility）。
//!
//! client 自当 openclaw macOS node（替代 OpenClaw.app），权限绑 client App 本身
//! （须签名 notarized，否则 TCC 不稳——踩 peekaboo 那种子进程/未签名失效坑）。
//!
//! macOS 公共 API（任何 App 都能调，非 OpenClaw.app 专有）：
//! - Screen Recording：`CGRequestScreenCaptureAccess`（首次弹系统框）、`CGPreflightScreenCaptureAccess`（检查）
//! - Accessibility：`AXIsProcessTrusted`（检查；首次调 AX API 系统会提示去系统设置）
//!
//! 引导用户去系统设置用 `open x-apple.systempreferences:...` URL scheme。

use std::process::Command;

use core_foundation::base::{CFTypeRef, TCFType};
use core_foundation::boolean::CFBoolean;
use core_foundation::dictionary::CFDictionary;
use core_foundation::string::CFString;

use serde::Serialize;

use objc2_foundation::NSError;
use objc2_user_notifications::{
    UNAuthorizationOptions, UNAuthorizationStatus, UNNotificationSettings, UNUserNotificationCenter,
};

#[link(name = "CoreGraphics", kind = "framework")]
extern "C" {
    fn CGPreflightScreenCaptureAccess() -> bool;
    fn CGRequestScreenCaptureAccess() -> bool;
}

#[link(name = "ApplicationServices", kind = "framework")]
extern "C" {
    fn AXIsProcessTrusted() -> bool;
    /// AXIsProcessTrustedWithOptions：传 CFDictionary{kAXTrustedCheckOptionPrompt: true} 首次弹系统授权框。
    fn AXIsProcessTrustedWithOptions(options: CFTypeRef) -> bool;
}

const PRIVACY_SCREEN_CAPTURE: &str =
    "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture";
const PRIVACY_ACCESSIBILITY: &str =
    "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility";

/// permissions 模块内联日志（走全局 util::log，与 [desktop_node] 前缀一致）。
/// ensure_* 的 requestAccess 是 fire-and-forget，不加这行日志就无法确证「首次调用是否真的触发了
/// 系统授权框」（NotDetermined→弹 / Denied→不弹，两者都返 PERMISSION_MISSING，仅看结果分不清）。
fn plog(msg: &str) {
    aiarsenal_client_core::util::log("info", &format!("[desktop_node] [perm] {msg}"));
}

/// 用户是否允许本 App 主动弹框申请该权限。托盘「权限」子菜单关闭的项 = false（见 PERM_TOGGLES），
/// 各 ensure_* 的 requestAccess/系统弹框都过这道门——否则 TCC reset 回 NotDetermined 后，
/// agent 一用命令就再弹框，「关闭」形同虚设。
fn req_allowed(key: &str) -> bool {
    !perm_disabled(key)
}

/// Screen Recording：检查；interactive 时 `CGRequestScreenCaptureAccess` 触发系统弹框（macOS 10.15+ 首次）。
pub fn ensure_screen_recording(interactive: bool) -> bool {
    unsafe {
        if CGPreflightScreenCaptureAccess() {
            return true;
        }
        if interactive && req_allowed("screen") {
            // 首次调用弹系统「允许录屏」框；用户去系统设置开。已授权/已决定则不弹。
            let _ = CGRequestScreenCaptureAccess();
        }
        CGPreflightScreenCaptureAccess()
    }
}

/// Accessibility：检查；interactive 时 `AXIsProcessTrustedWithOptions` + prompt 触发系统弹框
/// （「client 想控制你的电脑」），用户去系统设置勾选 client。控鼠键（CGEvent）必须此权限。
pub fn ensure_accessibility(interactive: bool) -> bool {
    unsafe {
        if AXIsProcessTrusted() {
            return true;
        }
        if interactive && req_allowed("accessibility") {
            // CFDictionary{kAXTrustedCheckOptionPrompt: kCFBooleanTrue} → 弹系统授权框。
            // from_CFType_pairs 要 owned TCFType（CFString/CFBoolean），不能传引用；类型推断为
            // CFDictionary<CFString, CFBoolean>，as_CFTypeRef() 拿 CFTypeRef 给 C API。
            let dict = CFDictionary::from_CFType_pairs(&[(
                CFString::new("AXTrustedCheckOptionPrompt"),
                CFBoolean::true_value(),
            )]);
            let _ = AXIsProcessTrustedWithOptions(dict.as_CFTypeRef());
        }
        AXIsProcessTrusted()
    }
}

/// 打开系统设置 → 隐私与安全 → 屏幕录制（引导用户手动勾选 client）。
pub fn open_screen_recording_settings() {
    let _ = Command::new("open").arg(PRIVACY_SCREEN_CAPTURE).spawn();
}

/// 打开系统设置 → 隐私与安全 → 辅助功能。
pub fn open_accessibility_settings() {
    let _ = Command::new("open").arg(PRIVACY_ACCESSIBILITY).spawn();
}

/// 一次性申请 desktop node 所需 TCC 权限（Screen Recording + Accessibility）。
/// Screen Recording 触发系统弹框；两者未授权则打开系统设置面板引导。返回 (screen_ok, a11y_ok)。
pub fn request_desktop_permissions(on_log: &dyn Fn(&str)) -> (bool, bool) {
    on_log("申请 openclaw desktop node 权限（Screen Recording + Accessibility）...");
    let screen = ensure_screen_recording(true);
    if !screen {
        on_log("Screen Recording 未授权：已触发系统弹框，请在系统设置中勾选本 App");
        open_screen_recording_settings();
    }
    let a11y = ensure_accessibility(true);
    if !a11y {
        on_log("Accessibility 未授权：请到 系统设置 → 隐私与安全 → 辅助功能 勾选本 App");
        open_accessibility_settings();
    }
    (screen, a11y)
}

/// Notifications（system.notify 用）：查 UNUserNotificationCenter 授权态。
/// getNotificationSettings 是异步 block API（无同步版本）→ channel 同步等 5s 拿 status。
/// interactive 且状态为 NotDetermined 时，fire-and-forget 发 requestAuthorization（触发系统弹框；
/// 不等回调，避免阻塞 invoke）→ 首次仍返 false，用户授权后下次 status 变 Authorized 才返 true。
///
/// ⚠️ 只能在 .app bundle 内调（`in_app_bundle` 守卫，权限清单页已做）：dev 裸二进制无
/// CFBundleIdentifier，`+currentNotificationCenter` NSAssert 整进程崩（system.notify 改 osascript 的原因）。
pub fn ensure_notifications(interactive: bool) -> bool {
    use block2::RcBlock;
    use core::ptr::NonNull;
    use objc2::runtime::Bool;
    use std::sync::mpsc;
    use std::time::Duration;

    unsafe {
        let center = UNUserNotificationCenter::currentNotificationCenter();

        let (tx, rx) = mpsc::channel::<UNAuthorizationStatus>();
        let block = RcBlock::new(move |settings: NonNull<UNNotificationSettings>| {
            let s = unsafe { settings.as_ref() };
            let _ = tx.send(s.authorizationStatus());
        });
        center.getNotificationSettingsWithCompletionHandler(&block);
        let status = rx
            .recv_timeout(Duration::from_secs(5))
            .unwrap_or(UNAuthorizationStatus::NotDetermined);

        if status == UNAuthorizationStatus::Authorized {
            return true;
        }
        if interactive && status == UNAuthorizationStatus::NotDetermined && req_allowed("notifications") {
            let req_block = RcBlock::new(move |_granted: Bool, _err: *mut NSError| {});
            let _ = center.requestAuthorizationWithOptions_completionHandler(
                UNAuthorizationOptions::Alert | UNAuthorizationOptions::Sound,
                &req_block,
            );
        }
        false
    }
}

/// Location（location.get 用）：**纯状态查询**——是否已授权。不主动请求授权。
///
/// ⚠️ 不要在此 fire-and-forget `requestWhenInUseAuthorization`：CoreLocation 的授权框绑定**存活**的
/// CLLocationManager 实例，本函数返回即 drop manager → 框还没弹出就被 macOS 取消（这正是历史上
/// location.get 首次不弹框的根因；通讯录/日历/照片/摄像头用 completion-handler 式 requestAccess，由
/// TCC daemon 独立持有，不受影响）。真正的授权请求由 location::get 在专用线程内发起——manager 在
/// RunLoop 期间常驻，框才会出现，用户授权后 delegate 回调自动触发定位。
pub fn ensure_location(_interactive: bool) -> bool {
    use objc2_core_location::{CLAuthorizationStatus, CLLocationManager};
    let mgr = unsafe { CLLocationManager::new() };
    let status = unsafe { mgr.authorizationStatus() };
    // mgr 于此 drop 安全：只读了状态，无挂起的授权请求。
    match status {
        CLAuthorizationStatus::AuthorizedAlways | CLAuthorizationStatus::AuthorizedWhenInUse => true,
        CLAuthorizationStatus::NotDetermined => {
            plog("ensure_location: NotDetermined（location::get 将在专用线程内弹框）");
            false
        }
        _ => {
            plog("ensure_location: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→定位 手动开");
            false
        }
    }
}

/// Contacts（contacts.search 用）：查 CNContactStore 授权态（类方法）。
/// interactive 且 NotDetermined 时 requestAccessForEntityType（block fire-and-forget）。
pub fn ensure_contacts(interactive: bool) -> bool {
    use block2::RcBlock;
    use objc2::runtime::Bool;
    use objc2_contacts::{CNAuthorizationStatus, CNContactStore, CNEntityType};
    use objc2_foundation::NSError;
    let status = unsafe { CNContactStore::authorizationStatusForEntityType(CNEntityType::Contacts) };
    if status == CNAuthorizationStatus::Authorized {
        return true;
    }
    if status == CNAuthorizationStatus::NotDetermined {
        if interactive && req_allowed("contacts") {
            plog("ensure_contacts: NotDetermined → requestAccess（系统应弹通讯录授权框）");
            let store = unsafe { CNContactStore::new() };
            let block = RcBlock::new(move |_g: Bool, _e: *mut NSError| {});
            let _ = unsafe {
                store.requestAccessForEntityType_completionHandler(CNEntityType::Contacts, &block)
            };
        }
    } else {
        plog("ensure_contacts: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→通讯录 手动开");
    }
    false
}

/// Calendar（calendar.events 用）：查 EKEventStore 授权态（类方法，entity=Event）。
/// interactive 且 NotDetermined 时 requestAccessToEntityType（block fire-and-forget）。
/// 注：requestAccessToEntityType 被 Apple 标 deprecated（建议 macOS14+ requestFullAccessToEvents），
/// 但旧 API 全版本可用、签名稳定，这里沿用。completion type 是 *mut DynBlock（裸指针）→ RcBlock::as_ptr。
#[allow(deprecated)]
pub fn ensure_calendar(interactive: bool) -> bool {
    use block2::RcBlock;
    use objc2::runtime::Bool;
    use objc2_event_kit::{EKAuthorizationStatus, EKEntityType, EKEventStore};
    use objc2_foundation::NSError;
    let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Event) };
    if status == EKAuthorizationStatus::Authorized {
        return true;
    }
    if status == EKAuthorizationStatus::NotDetermined {
        if interactive && req_allowed("calendar") {
            plog("ensure_calendar: NotDetermined → requestAccess（系统应弹日历授权框）");
            let store = unsafe { EKEventStore::new() };
            let block = RcBlock::new(move |_g: Bool, _e: *mut NSError| {});
            let _ = unsafe {
                store.requestAccessToEntityType_completion(EKEntityType::Event, RcBlock::as_ptr(&block))
            };
        }
    } else {
        plog("ensure_calendar: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→日历 手动开");
    }
    false
}

/// Reminders（reminders.list 用）：查 EKEventStore 授权态（entity=Reminder）。
#[allow(deprecated)]
pub fn ensure_reminders(interactive: bool) -> bool {
    use block2::RcBlock;
    use objc2::runtime::Bool;
    use objc2_event_kit::{EKAuthorizationStatus, EKEntityType, EKEventStore};
    use objc2_foundation::NSError;
    let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Reminder) };
    if status == EKAuthorizationStatus::Authorized {
        return true;
    }
    if status == EKAuthorizationStatus::NotDetermined {
        if interactive && req_allowed("reminders") {
            plog("ensure_reminders: NotDetermined → requestAccess（系统应弹提醒事项授权框）");
            let store = unsafe { EKEventStore::new() };
            let block = RcBlock::new(move |_g: Bool, _e: *mut NSError| {});
            let _ = unsafe {
                store.requestAccessToEntityType_completion(EKEntityType::Reminder, RcBlock::as_ptr(&block))
            };
        }
    } else {
        plog("ensure_reminders: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→提醒事项 手动开");
    }
    false
}

/// Photos（photos.latest 用）：查 PHPhotoLibrary 授权态（全版本兼容用 deprecated authorizationStatus 无参版，
/// 新 authorizationStatusForAccessLevel 仅 macOS14+）。interactive 且 NotDetermined 时 requestAuthorization。
#[allow(deprecated)]
pub fn ensure_photos(interactive: bool) -> bool {
    use block2::RcBlock;
    use objc2_photos::{PHAuthorizationStatus, PHPhotoLibrary};
    let status = unsafe { PHPhotoLibrary::authorizationStatus() };
    if status == PHAuthorizationStatus::Authorized {
        return true;
    }
    if status == PHAuthorizationStatus::NotDetermined {
        if interactive && req_allowed("photos") {
            plog("ensure_photos: NotDetermined → requestAuthorization（系统应弹照片授权框）");
            let block = RcBlock::new(move |_s: PHAuthorizationStatus| {});
            let _ = unsafe { PHPhotoLibrary::requestAuthorization(&block) };
        }
    } else {
        plog("ensure_photos: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→照片 手动开");
    }
    false
}

/// Camera（camera.snap 用）：查 AVCaptureDevice 授权态（AVMediaTypeVideo）。
/// interactive 且 NotDetermined 时 requestAccessForMediaType（block fire-and-forget）。
pub fn ensure_camera(interactive: bool) -> bool {
    use block2::RcBlock;
    use objc2::runtime::Bool;
    use objc2_av_foundation::{AVAuthorizationStatus, AVCaptureDevice, AVMediaTypeVideo};
    let mt = match unsafe { AVMediaTypeVideo.as_ref() } {
        Some(m) => m,
        None => return false,
    };
    let status = unsafe { AVCaptureDevice::authorizationStatusForMediaType(mt) };
    if status == AVAuthorizationStatus::Authorized {
        return true;
    }
    if status == AVAuthorizationStatus::NotDetermined {
        if interactive && req_allowed("camera") {
            plog("ensure_camera: NotDetermined → requestAccess（系统应弹摄像头授权框）");
            let block = RcBlock::new(move |_g: Bool| {});
            let _ = unsafe {
                AVCaptureDevice::requestAccessForMediaType_completionHandler(mt, &block)
            };
        }
    } else {
        plog("ensure_camera: 已拒绝/受限，不弹框——去 系统设置→隐私与安全→摄像头 手动开");
    }
    false
}

// ==================== 权限清单页（一次性申请） ====================

/// 权限页清单行（`get_permission_statuses` 命令返回，前端 PermissionDialog 渲染）。
#[derive(Debug, Clone, Serialize)]
pub struct PermStatus {
    pub key: String,
    /// granted | notDetermined | denied | restricted | unknown。
    /// screen/accessibility 无 denied 探测（CG/AX 只给二元），未授权统一报 notDetermined；
    /// unknown = 该环境下查不了（如 dev 裸二进制的通知，见 `in_app_bundle`）。
    pub state: String,
    /// 本权限能否由 App 主动触发系统授权框（location 不能——RunLoop 存活陷阱）。
    #[serde(rename = "interactiveSupported")]
    pub interactive_supported: bool,
}

/// 是否跑在 .app bundle 里（`cargo tauri dev` 裸二进制 = false，`tauri build` 产物 = true）。
/// UNUserNotificationCenter 在无 CFBundleIdentifier 的裸二进制下 NSAssert 整进程崩
/// （memory：system.notify 因此改走 osascript）——通知相关查询/申请仅在 .app 内放开。
pub(crate) fn in_app_bundle() -> bool {
    std::env::current_exe()
        .map(|p| p.to_string_lossy().contains(".app/Contents/MacOS"))
        .unwrap_or(false)
}

fn state_screen() -> &'static str {
    if ensure_screen_recording(false) {
        "granted"
    } else {
        "notDetermined"
    }
}

fn state_accessibility() -> &'static str {
    if ensure_accessibility(false) {
        "granted"
    } else {
        "notDetermined"
    }
}

fn state_camera() -> &'static str {
    use objc2_av_foundation::{AVAuthorizationStatus, AVCaptureDevice, AVMediaTypeVideo};
    let Some(mt) = (unsafe { AVMediaTypeVideo.as_ref() }) else {
        return "unknown";
    };
    let s = unsafe { AVCaptureDevice::authorizationStatusForMediaType(mt) };
    if s == AVAuthorizationStatus::Authorized {
        "granted"
    } else if s == AVAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if s == AVAuthorizationStatus::Denied {
        "denied"
    } else if s == AVAuthorizationStatus::Restricted {
        "restricted"
    } else {
        "unknown"
    }
}

fn state_contacts() -> &'static str {
    use objc2_contacts::{CNAuthorizationStatus, CNContactStore, CNEntityType};
    let s = unsafe { CNContactStore::authorizationStatusForEntityType(CNEntityType::Contacts) };
    if s == CNAuthorizationStatus::Authorized || s == CNAuthorizationStatus::Limited {
        "granted"
    } else if s == CNAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if s == CNAuthorizationStatus::Denied {
        "denied"
    } else if s == CNAuthorizationStatus::Restricted {
        "restricted"
    } else {
        "unknown"
    }
}

/// Calendar/Reminders 共用 EKEventStore（entity 不同）。FullAccess=3 与旧 Authorized 同值
/// （deprecated 别名），只比 FullAccess 即可覆盖两代 SDK 语义。
fn state_ek(entity: objc2_event_kit::EKEntityType) -> &'static str {
    use objc2_event_kit::{EKAuthorizationStatus, EKEventStore};
    let s = unsafe { EKEventStore::authorizationStatusForEntityType(entity) };
    if s == EKAuthorizationStatus::FullAccess {
        "granted"
    } else if s == EKAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if s == EKAuthorizationStatus::Denied {
        "denied"
    } else if s == EKAuthorizationStatus::Restricted {
        "restricted"
    } else {
        "unknown"
    }
}

fn state_calendar() -> &'static str {
    state_ek(objc2_event_kit::EKEntityType::Event)
}

fn state_reminders() -> &'static str {
    state_ek(objc2_event_kit::EKEntityType::Reminder)
}

#[allow(deprecated)] // authorizationStatus 无参版全版本可用（同 ensure_photos）
fn state_photos() -> &'static str {
    use objc2_photos::{PHAuthorizationStatus, PHPhotoLibrary};
    let s = unsafe { PHPhotoLibrary::authorizationStatus() };
    if s == PHAuthorizationStatus::Authorized || s == PHAuthorizationStatus::Limited {
        "granted"
    } else if s == PHAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if s == PHAuthorizationStatus::Denied {
        "denied"
    } else if s == PHAuthorizationStatus::Restricted {
        "restricted"
    } else {
        "unknown"
    }
}

fn state_location() -> &'static str {
    use objc2_core_location::{CLAuthorizationStatus, CLLocationManager};
    let mgr = unsafe { CLLocationManager::new() };
    let s = unsafe { mgr.authorizationStatus() };
    // mgr drop 安全：只读状态、无挂起授权请求（同 ensure_location）
    if s == CLAuthorizationStatus::AuthorizedAlways || s == CLAuthorizationStatus::AuthorizedWhenInUse {
        "granted"
    } else if s == CLAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if s == CLAuthorizationStatus::Denied {
        "denied"
    } else if s == CLAuthorizationStatus::Restricted {
        "restricted"
    } else {
        "unknown"
    }
}

fn state_notifications() -> &'static str {
    use block2::RcBlock;
    use core::ptr::NonNull;
    use std::sync::mpsc;
    use std::time::Duration;
    if !in_app_bundle() {
        return "unknown";
    }
    let center = UNUserNotificationCenter::currentNotificationCenter();
    let (tx, rx) = mpsc::channel::<UNAuthorizationStatus>();
    let block = RcBlock::new(move |settings: NonNull<UNNotificationSettings>| {
        let s = unsafe { settings.as_ref() };
        let _ = tx.send(s.authorizationStatus());
    });
    center.getNotificationSettingsWithCompletionHandler(&block);
    let status = rx
        .recv_timeout(Duration::from_secs(5))
        .unwrap_or(UNAuthorizationStatus::NotDetermined);
    if status == UNAuthorizationStatus::Authorized
        || status == UNAuthorizationStatus::Provisional
        || status == UNAuthorizationStatus::Ephemeral
    {
        "granted"
    } else if status == UNAuthorizationStatus::NotDetermined {
        "notDetermined"
    } else if status == UNAuthorizationStatus::Denied {
        "denied"
    } else {
        "unknown"
    }
}

/// 全量权限状态（9 项，顺序即清单页展示序）。逐项查 TCC 真态，无缓存。
pub fn status_list() -> Vec<PermStatus> {
    let rows: [(&str, fn() -> &'static str, bool); 9] = [
        ("screen", state_screen, true),
        ("accessibility", state_accessibility, true),
        ("location", state_location, false),
        ("contacts", state_contacts, true),
        ("calendar", state_calendar, true),
        ("reminders", state_reminders, true),
        ("photos", state_photos, true),
        ("camera", state_camera, true),
        ("notifications", state_notifications, in_app_bundle()),
    ];
    rows.into_iter()
        .map(|(key, state, interactive)| PermStatus {
            key: key.into(),
            state: state().into(),
            interactive_supported: interactive,
        })
        .collect()
}

/// 申请单项权限（触发系统授权框）并返回申请后的当前态。
/// 各 ensure_*(true) 的 requestAccess 是 fire-and-forget：首次申请后系统弹框还挂着，
/// 返回值通常是 notDetermined——前端轮询 status_list，用户点允许后自动变绿。
pub fn request_one(key: &str) -> String {
    let after = match key {
        "screen" => {
            let _ = ensure_screen_recording(true);
            state_screen()
        }
        "accessibility" => {
            let _ = ensure_accessibility(true);
            state_accessibility()
        }
        "camera" => {
            let _ = ensure_camera(true);
            state_camera()
        }
        "contacts" => {
            let _ = ensure_contacts(true);
            state_contacts()
        }
        "calendar" => {
            let _ = ensure_calendar(true);
            state_calendar()
        }
        "reminders" => {
            let _ = ensure_reminders(true);
            state_reminders()
        }
        "photos" => {
            let _ = ensure_photos(true);
            state_photos()
        }
        // location：授权框绑定存活的 CLLocationManager（ensure_location 的 RunLoop 陷阱），
        // 此处不主动请求——location.get 节点命令执行时会在专用线程内弹框。只回当前态。
        "location" => state_location(),
        "notifications" => {
            if in_app_bundle() {
                let _ = ensure_notifications(true);
                state_notifications()
            } else {
                "unknown"
            }
        }
        _ => "unknown",
    };
    after.into()
}

/// 启动时自动串行申请全部「可弹框」权限（= 权限页「一键申请」的开机自动版）。
/// 已授权/已拒绝项静默跳过（ensure_* 先查状态再 requestAccess），实际只在首次启动弹一轮
/// 系统框，授权后永久生效、无人值守不再弹。
/// screen/accessibility 由 `request_desktop_permissions`（desktop_node::start 开头）负责；
/// location 例外：授权框绑定存活的 CLLocationManager（RunLoop 陷阱），只能由 location.get
/// 在专用线程内请求，此处不碰。
pub fn request_startup_permissions() {
    for key in [
        "camera", "contacts", "calendar", "reminders", "photos", "notifications",
    ] {
        if perm_disabled(key) {
            plog(&format!("启动申请权限 {key} 跳过（用户已在托盘关闭）"));
            continue;
        }
        plog(&format!("启动申请权限 {key}（NotDetermined 时系统弹框）"));
        let after = request_one(key);
        plog(&format!("启动申请权限 {key} → {after}"));
    }
}

/// 提前申请 System Events 自动化（AppleEvents）授权。TCC 的自动化权限按「本 App → 目标 App」
/// 逐对授予，没有「一次性预授所有目标」的 API；System Events 是 agent 进程/窗口枚举类
/// AppleScript 的最常用目标——启动时批一次，之后 system.run 跑 `tell application "System
/// Events" ...` 不再中途弹框挂起（曾致网关 node.invoke 30s 超时报错）。具体目标 App
/// （如 Cursor）仍只能首次用到时弹，系统机制躲不掉。
/// osascript 会阻塞到用户回应授权框 → 后台线程跑，不挡启动；仅 .app 内（dev 裸二进制下
/// TCC 归因到终端，会误授 Terminal）。
pub fn authorize_system_events_automation() {
    if !in_app_bundle() {
        plog("System Events 授权：非 .app 环境，跳过");
        return;
    }
    std::thread::spawn(|| {
        plog("System Events 自动化授权：osascript 探测（首次会弹「想要控制系统事件」框）...");
        let out = Command::new("osascript")
            .arg("-e")
            .arg(r#"tell application "System Events" to get name of first process"#)
            .output();
        match out {
            Ok(o) if o.status.success() => {
                plog("System Events 授权：已授权（后续 AppleScript 枚举不再弹框）")
            }
            Ok(o) => plog(&format!(
                "System Events 授权：未授权（{}）——可去 系统设置→隐私与安全→自动化 手动开",
                String::from_utf8_lossy(&o.stderr).trim()
            )),
            Err(e) => plog(&format!("System Events 授权：osascript 执行失败 {e}")),
        }
    });
}

/// 打开系统设置 → 隐私与安全 → 对应权限面板（引导用户手动勾选；denied 后唯一的路）。
pub fn open_settings(key: &str) {
    const PRIVACY_BASE: &str =
        "x-apple.systempreferences:com.apple.preference.security?Privacy_";
    let url = match key {
        "screen" => PRIVACY_SCREEN_CAPTURE.to_string(),
        "accessibility" => PRIVACY_ACCESSIBILITY.to_string(),
        "location" => format!("{PRIVACY_BASE}LocationServices"),
        "contacts" => format!("{PRIVACY_BASE}Contacts"),
        "calendar" => format!("{PRIVACY_BASE}Calendars"),
        "reminders" => format!("{PRIVACY_BASE}Reminders"),
        "photos" => format!("{PRIVACY_BASE}Photos"),
        "camera" => format!("{PRIVACY_BASE}Camera"),
        // 通知面板不在 Privacy 组下，是独立 pane
        "notifications" => "x-apple.systempreferences:com.apple.Notifications-Settings".into(),
        _ => return,
    };
    let _ = Command::new("open").arg(&url).spawn();
}

/// 各权限的手动操作路径文案（引导对话框正文用；与 open_settings 的 URL 一一对应）。
/// 只到面板位置不带动作词——开/关动作由各调用点的文案接续（路径里混进「允许通知」
/// 再接「手动关闭」会自相矛盾，实测误导用户把刚打开的开关又关掉）。
fn settings_path(key: &str) -> &'static str {
    match key {
        "screen" => "系统设置 → 隐私与安全性 → 屏幕录制",
        "accessibility" => "系统设置 → 隐私与安全性 → 辅助功能",
        "location" => "系统设置 → 隐私与安全性 → 定位服务",
        "contacts" => "系统设置 → 隐私与安全性 → 通讯录",
        "calendar" => "系统设置 → 隐私与安全性 → 日历",
        "reminders" => "系统设置 → 隐私与安全性 → 提醒事项",
        "photos" => "系统设置 → 隐私与安全性 → 照片",
        "camera" => "系统设置 → 隐私与安全性 → 摄像头",
        "notifications" => "系统设置 → 通知 → AI Arsenal",
        _ => "系统设置",
    }
}

/// 引导对话框（osascript display dialog，60s 自动消失）：权限动作 App 自己完成不了、
/// 需用户去系统设置手动操作时把「怎么做」说清楚。通知权限被拒后横幅通道是哑的，
/// 对话框不依赖任何权限，是托盘引导的可靠提示面。独立线程执行——对话框停留期间
/// 托盘菜单文字照常刷新（handle_tray_toggle 的返回不被阻塞）。
fn guide_dialog(message: &str) {
    let script = format!(
        "display dialog {} with title \"AI Arsenal\" buttons {{\"好的\"}} default button \"好的\" with icon note giving up after 60",
        super::system::apple_script_string(message)
    );
    std::thread::spawn(move || {
        let _ = Command::new("osascript").args(["-e", &script]).output();
    });
}

// ==================== 托盘权限子菜单（开/关切换） ====================
//
// macOS 没有「App 自己关闭已授权限」的 API，关闭 = `tccutil reset <Service> <bundle-id>`
// 把授权收回。收回后 TCC 状态回 NotDetermined——与「从未申请」无法区分，所以必须本地记住
// 用户的选择（client-settings.json permsDisabled），三处消费：托盘菜单文字、启动自动申请、
// 各 ensure_* 的弹框门控（req_allowed）。

const BUNDLE_ID: &str = "cc.aiarsenal.client";

/// 托盘权限子菜单项定义。tccutil 服务名对照 kTCCService（Notifications 不在 tccutil
/// 支持列表，只能引导去系统设置手动关）。
pub struct PermToggleSpec {
    pub key: &'static str,
    pub label: &'static str,
    pub tccutil: Option<&'static str>,
}

/// 顺序即子菜单展示序（与 status_list 权限页一致）。
pub const PERM_TOGGLES: [PermToggleSpec; 9] = [
    PermToggleSpec { key: "screen", label: "屏幕录制", tccutil: Some("ScreenCapture") },
    PermToggleSpec { key: "accessibility", label: "辅助功能", tccutil: Some("Accessibility") },
    PermToggleSpec { key: "location", label: "定位", tccutil: Some("Location") },
    PermToggleSpec { key: "contacts", label: "通讯录", tccutil: Some("Contacts") },
    PermToggleSpec { key: "calendar", label: "日历", tccutil: Some("Calendar") },
    PermToggleSpec { key: "reminders", label: "提醒事项", tccutil: Some("Reminders") },
    PermToggleSpec { key: "photos", label: "照片", tccutil: Some("Photos") },
    PermToggleSpec { key: "camera", label: "摄像头", tccutil: Some("Camera") },
    PermToggleSpec { key: "notifications", label: "通知", tccutil: None },
];

/// data_dir 全局化（OnceLock，lib.rs run() 里 init_log 旁 set 一次）：ensure_* / 启动申请 /
/// 托盘切换都要读 permsDisabled，不方便层层传参。
static DATA_DIR: std::sync::OnceLock<std::path::PathBuf> = std::sync::OnceLock::new();

pub fn init_data_dir(dir: std::path::PathBuf) {
    let _ = DATA_DIR.set(dir);
}

pub(crate) fn data_dir() -> Option<&'static std::path::Path> {
    DATA_DIR.get().map(|p| p.as_path())
}

/// 该权限是否被用户在托盘关闭（本地记忆；未初始化/未配置 = 未关闭）。
pub fn perm_disabled(key: &str) -> bool {
    let Some(dir) = data_dir() else {
        return false;
    };
    aiarsenal_client_core::util::load_settings(dir)
        .get("permsDisabled")
        .and_then(|v| v.as_array())
        .map(|a| a.iter().any(|x| x.as_str() == Some(key)))
        .unwrap_or(false)
}

/// 设置/清除本地关闭标志（原子写由 set_setting 保证——整体覆写 settings map）。
pub fn set_perm_disabled(key: &str, disabled: bool) {
    let Some(dir) = data_dir() else {
        return;
    };
    let mut list: Vec<String> =
        aiarsenal_client_core::util::load_settings(dir)
            .get("permsDisabled")
            .and_then(|v| v.as_array())
            .map(|a| {
                a.iter()
                    .filter_map(|x| x.as_str().map(std::string::ToString::to_string))
                    .collect()
            })
            .unwrap_or_default();
    list.retain(|k| k != key);
    if disabled {
        list.push(key.to_string());
    }
    aiarsenal_client_core::util::set_setting(
        dir,
        "permsDisabled",
        serde_json::Value::Array(
            list.into_iter().map(serde_json::Value::String).collect(),
        ),
    );
}

/// 单项 TCC 状态（state_* 系列按 key 分发；托盘文字/切换逻辑用）。
pub fn state_of(key: &str) -> &'static str {
    match key {
        "screen" => state_screen(),
        "accessibility" => state_accessibility(),
        "location" => state_location(),
        "contacts" => state_contacts(),
        "calendar" => state_calendar(),
        "reminders" => state_reminders(),
        "photos" => state_photos(),
        "camera" => state_camera(),
        "notifications" => state_notifications(),
        _ => "unknown",
    }
}

/// 托盘菜单文字：✓ 已授权（点击关闭）/ ⚠️ 未申请（点击授权）/ ✗ 已关闭（点击开启）。
pub fn menu_text(key: &str) -> String {
    let label = PERM_TOGGLES
        .iter()
        .find(|s| s.key == key)
        .map(|s| s.label)
        .unwrap_or(key);
    if perm_disabled(key) {
        return format!("✗ {label}（已关闭）");
    }
    match state_of(key) {
        "granted" => format!("✓ {label}"),
        "notDetermined" => format!("⚠️ {label}"),
        "denied" | "restricted" => format!("✗ {label}（已拒绝）"),
        _ => format!("？ {label}"),
    }
}

/// 系统级收回授权（tccutil reset）。阻塞 ~0.5s，调用方放后台线程。
pub fn reset_permission(key: &str) -> Result<(), String> {
    let spec = PERM_TOGGLES
        .iter()
        .find(|s| s.key == key)
        .ok_or_else(|| format!("未知权限 {key}"))?;
    let service = spec
        .tccutil
        .ok_or_else(|| "该权限不支持一键收回，请到 系统设置→隐私与安全 手动关闭".to_string())?;
    let out = Command::new("tccutil")
        .arg("reset")
        .arg(service)
        .arg(BUNDLE_ID)
        .output()
        .map_err(|e| format!("tccutil 执行失败: {e}"))?;
    if out.status.success() {
        Ok(())
    } else {
        Err(format!(
            "tccutil reset {service} 失败: {}",
            String::from_utf8_lossy(&out.stderr).trim()
        ))
    }
}

/// 收回本 App 的全部 AppleEvents（自动化）授权对。tccutil 只能按服务整体重置，
/// 无法按目标 App 粒度——重置后清单内所有 App 的授权框会重新弹出。
/// 阻塞 ~0.5s，调用方放后台线程。
pub fn reset_apple_events() -> Result<(), String> {
    let out = Command::new("tccutil")
        .arg("reset")
        .arg("AppleEvents")
        .arg(BUNDLE_ID)
        .output()
        .map_err(|e| format!("tccutil 执行失败: {e}"))?;
    if out.status.success() {
        Ok(())
    } else {
        Err(format!(
            "tccutil reset AppleEvents 失败: {}",
            String::from_utf8_lossy(&out.stderr).trim()
        ))
    }
}

/// 托盘权限项点击的完整切换流。阻塞（tccutil/状态查询），调用方放后台线程；
/// 返回刷新后的菜单文字。凡 App 自己完成不了的动作（denied 后申请不弹框、screen/a11y
/// 须手动勾选、通知不支持程序收回），都打开对应设置面板 + 对话框指路——不能静默。
pub fn handle_tray_toggle(key: &str) -> String {
    let label = PERM_TOGGLES
        .iter()
        .find(|s| s.key == key)
        .map(|s| s.label)
        .unwrap_or(key);
    if perm_disabled(key) {
        // 已关闭 → 开启：清本地标志（恢复自动申请/弹框），立刻走一次申请。
        set_perm_disabled(key, false);
        plog(&format!("托盘开启权限 {key}"));
        let after = request_one(key);
        if key == "screen" || key == "accessibility" {
            // 这两项申请弹框只做引导，重开必须去系统设置手动勾。
            open_settings(key);
            guide_dialog(&format!(
                "「{label}」需手动恢复。请到 {} 勾选 AI Arsenal。",
                settings_path(key)
            ));
        } else if after == "denied" || after == "restricted" {
            open_settings(key);
            guide_dialog(&format!(
                "「{label}」已被系统拒绝，macOS 不会再自动弹授权框。请到 {} 手动开启。",
                settings_path(key)
            ));
        }
    } else if state_of(key) == "granted" {
        // 已授权 → 关闭：系统收回 + 本地记住（否则启动申请/ensure_* 立刻再弹）。
        match reset_permission(key) {
            Ok(()) => {
                set_perm_disabled(key, true);
                plog(&format!("托盘关闭权限 {key}：tccutil 已收回 + 本地记住"));
            }
            Err(e) => {
                plog(&format!("托盘关闭权限 {key} 失败：{e}（改为打开系统设置引导）"));
                open_settings(key);
                guide_dialog(&format!(
                    "「{label}」不支持程序一键收回。请到 {} 手动关闭。",
                    settings_path(key)
                ));
            }
        }
    } else {
        // 未授权/未申请 → 申请（原子菜单项行为：弹框 + screen/a11y 打开设置面板）。
        // 已拒绝/受限 → request_* 一律不再弹框（macOS 拒绝后 requestAuthorization 静默
        // 返回失败，TCC requestAccess 同理）——打开对应设置面板 + 对话框指路（denied 后
        // 手动恢复是唯一路径；通知面板不在 Privacy 组下，独立 pane）。
        plog(&format!("托盘申请权限 {key}"));
        let st = state_of(key);
        if st == "denied" || st == "restricted" {
            open_settings(key);
            guide_dialog(&format!(
                "「{label}」已被系统拒绝，macOS 不会再自动弹授权框（已为你打开设置面板）。请到 {} 手动开启。",
                settings_path(key)
            ));
        } else {
            if key == "screen" || key == "accessibility" {
                open_settings(key);
                guide_dialog(&format!(
                    "「{label}」的授权须在设置面板手动勾选（已为你打开）。请到 {} 勾选 AI Arsenal。",
                    settings_path(key)
                ));
            }
            request_one(key);
        }
    }
    menu_text(key)
}
