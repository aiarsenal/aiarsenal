//! desktop node 权限引擎（Windows 版）。
//!
//! Windows 没有 macOS TCC 那样的系统级隐私授权（SendInput/BitBlt/文件读取默认可用），
//! 「权限」在 Windows 侧收敛为**客户端自己的能力开关**：用户在客户端设置/托盘里可以
//! 逐项关掉某能力（permsDisabled 数组存 client-settings.json），关掉的项 ensure_* 返回
//! false，各命令 fail-closed 报 PERMISSION_MISSING——与 mac 端「未授权」错误同形同码，
//! agent 侧降级逻辑完全一致。默认（未配置）= 全部可用。
//!
//! 与 mac permissions.rs 的对外 API 同名同形（ensure_* / PermStatus / status_list /
//! request_one / open_settings / PERM_TOGGLES / menu_text / handle_tray_toggle /
//! init_data_dir / data_dir / perm_disabled / set_perm_disabled / state_of），
//! 语义差异都文档化在各函数头。审批 UI 挂钩（request_one/open_settings）保留原位，
//! 将来接入真权限面板时无需动调用点。

use serde::Serialize;

/// permissions 模块内联日志（走全局 util::log，与 [desktop_node] 前缀一致）。
fn plog(msg: &str) {
    aiarsenal_client_core::util::log("info", &format!("[desktop_node] [perm] {msg}"));
}

/// 用户是否允许本 App 启用该能力（托盘/设置关闭的项 = false）。
/// Windows 无系统弹框，这道门只挡「能力开关」本身（mac 上挡的是系统授权框）。
fn req_allowed(key: &str) -> bool {
    !perm_disabled(key)
}

/// Screen Recording（screen.capture / screen.record / system.run needsScreenRecording）。
/// Windows 无录屏授权：GDI 截屏默认可用，唯一开关是客户端能力开关。
pub fn ensure_screen_recording(interactive: bool) -> bool {
    ensure_cap("screen", "屏幕录制", interactive)
}

/// Accessibility（computer.act 键鼠注入）。Windows 无辅助功能授权：SendInput 默认可用，
/// 唯一开关是客户端能力开关（app_gate/act 的 fail-closed 闸门独立于此，见 app_gate.rs）。
pub fn ensure_accessibility(interactive: bool) -> bool {
    ensure_cap("accessibility", "辅助功能（键鼠控制）", interactive)
}

/// Location（location.get）。Windows 客户端暂无定位实现（WinRT Geolocator 未接），
/// 开关语义保留：关掉时 location.get 走 PERMISSION_MISSING；开着时命令报 UNAVAILABLE。
pub fn ensure_location(interactive: bool) -> bool {
    ensure_cap("location", "定位", interactive)
}

/// Contacts（contacts.search）。
pub fn ensure_contacts(interactive: bool) -> bool {
    ensure_cap("contacts", "通讯录", interactive)
}

/// Calendar（calendar.events）。
pub fn ensure_calendar(interactive: bool) -> bool {
    ensure_cap("calendar", "日历", interactive)
}

/// Reminders（reminders.list）。
pub fn ensure_reminders(interactive: bool) -> bool {
    ensure_cap("reminders", "提醒事项", interactive)
}

/// Photos（photos.latest）。
pub fn ensure_photos(interactive: bool) -> bool {
    ensure_cap("photos", "照片", interactive)
}

/// Camera（camera.list/snap/clip）。
pub fn ensure_camera(interactive: bool) -> bool {
    ensure_cap("camera", "摄像头", interactive)
}

/// Notifications（system.notify）。Windows 气泡通知无独立权限开关，恒可用；
/// 保留开关语义供 UI 统一（关掉 = 客户端静默，不再发气泡）。
pub fn ensure_notifications(interactive: bool) -> bool {
    ensure_cap("notifications", "通知", interactive)
}

/// 能力开关统一检查：未关闭 = 可用（Windows 无 TCC，默认放行）。
/// interactive 在 Windows 无弹框对象，仅记日志（审批 UI 挂钩点，保留参数与 mac 同形）。
fn ensure_cap(key: &str, label: &str, interactive: bool) -> bool {
    let on = req_allowed(key);
    if !on {
        plog(&format!("能力 {label}（{key}）已在客户端设置中关闭，拒绝"));
    } else if interactive {
        plog(&format!("能力 {label}（{key}）检查通过（Windows 无 TCC，无系统弹框）"));
    }
    on
}

// ==================== 权限清单页 ====================

/// 权限页清单行（`get_permission_statuses` 命令返回，前端 PermissionDialog 渲染）。
#[derive(Debug, Clone, Serialize)]
pub struct PermStatus {
    pub key: String,
    /// granted | notDetermined | denied | restricted | unknown。
    /// Windows 只有两态：denied（用户在客户端关闭）/ granted（默认或已重新开启）——
    /// 无系统授权态，notDetermined/restricted 永不出现。
    pub state: String,
    /// 本权限能否由 App 主动触发授权动作。Windows 的「授权」= 客户端自己翻开关，
    /// 每项都能做（mac 的 location=false RunLoop 陷阱在 Windows 无对应物）。
    #[serde(rename = "interactiveSupported")]
    pub interactive_supported: bool,
}

/// 是否跑在安装后的正式包里。Windows 无 .app bundle 概念（dev `cargo tauri dev` 与
/// 打包产物行为一致——气泡通知/文件枚举都不依赖 bundle），恒 true。保留给
/// status_list 的 notifications 行与 app_perm 的 in_app_bundle 语义对齐用。
pub(crate) fn in_app_bundle() -> bool {
    true
}

/// 单项状态（state_of 按 key 分发；托盘文字/切换逻辑/清单页用）。
pub fn state_of(key: &str) -> &'static str {
    if perm_disabled(key) {
        // Windows 无 TCC，「关闭」就是用户本地关的开关，语义对齐 mac 的 denied
        "denied"
    } else {
        "granted"
    }
}

/// 全量权限状态（9 项，顺序即清单页展示序，与 mac status_list 一致）。
pub fn status_list() -> Vec<PermStatus> {
    let rows: [(&str, bool); 9] = [
        ("screen", true),
        ("accessibility", true),
        ("location", true),
        ("contacts", true),
        ("calendar", true),
        ("reminders", true),
        ("photos", true),
        ("camera", true),
        ("notifications", in_app_bundle()),
    ];
    rows.into_iter()
        .map(|(key, interactive)| PermStatus {
            key: key.into(),
            state: state_of(key).into(),
            interactive_supported: interactive,
        })
        .collect()
}

/// 申请单项权限。Windows 无系统授权框：已关闭的项保持 denied（用户需先在客户端开启），
/// 未关闭的项本就 granted。返回申请后的当前态（前端轮询兼容）。
pub fn request_one(key: &str) -> String {
    plog(&format!("申请权限 {key}（Windows 无 TCC，仅本地开关语义）"));
    state_of(key).into()
}

/// 启动时自动申请全部权限。Windows 无 TCC 弹框、默认全可用——无需启动申请，
/// 保留函数（no-op）与 mac mod.rs 的启动序列对齐，将来接真授权面板时补实现。
pub fn request_startup_permissions() {
    plog("启动权限申请：Windows 无 TCC，无需申请（能力开关默认全开）");
}

/// 提前授权「自动化到 System Events」。mac 专属（AppleEvents 逐对授权）；Windows 无对应物。
/// 保留 no-op 与 mac 启动序列对齐。
pub fn authorize_system_events_automation() {
    plog("System Events 自动化授权：Windows 无 AppleEvents，跳过");
}

/// 打开系统设置 → 对应权限面板（explorer.exe 拉 ms-settings: URI）。
/// Windows 无对应面板的项（screen/accessibility/reminders/photos）落在「隐私常规」页
/// ——能力开关本就在客户端内，这里只是给好奇的用户一个系统侧入口。
pub fn open_settings(key: &str) {
    let uri = match key {
        "location" => "ms-settings:privacy-location",
        "contacts" => "ms-settings:privacy-contacts",
        "calendar" => "ms-settings:privacy-calendar",
        "camera" => "ms-settings:privacy-webcam",
        // 通知面板不在 Privacy 组下，是独立页
        "notifications" => "ms-settings:notifications",
        // 无对应系统面板：退到隐私常规页（Windows 无录屏/辅助功能/提醒/照片的系统授权）
        "screen" | "accessibility" | "reminders" | "photos" => "ms-settings:privacy-general",
        _ => return,
    };
    let _ = std::process::Command::new("explorer").arg(uri).spawn();
}

// ==================== 客户端能力开关（client-settings.json permsDisabled） ====================

/// 托盘权限子菜单项定义。tccutil 字段在 Windows 无对应物（无系统授权可收回），恒 None
/// ——保留字段与 mac PermToggleSpec 同形，前端/托盘菜单构建代码两侧共用形状。
pub struct PermToggleSpec {
    pub key: &'static str,
    pub label: &'static str,
    pub tccutil: Option<&'static str>,
}

/// 顺序即子菜单展示序（与 status_list 权限页一致）。
pub const PERM_TOGGLES: [PermToggleSpec; 9] = [
    PermToggleSpec { key: "screen", label: "屏幕录制", tccutil: None },
    PermToggleSpec { key: "accessibility", label: "辅助功能", tccutil: None },
    PermToggleSpec { key: "location", label: "定位", tccutil: None },
    PermToggleSpec { key: "contacts", label: "通讯录", tccutil: None },
    PermToggleSpec { key: "calendar", label: "日历", tccutil: None },
    PermToggleSpec { key: "reminders", label: "提醒事项", tccutil: None },
    PermToggleSpec { key: "photos", label: "照片", tccutil: None },
    PermToggleSpec { key: "camera", label: "摄像头", tccutil: None },
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

/// 该权限是否被用户在客户端关闭（本地记忆；未初始化/未配置 = 未关闭）。
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

/// 托盘菜单文字：✓ 可用（点击关闭）/ ✗ 已关闭（点击开启）。与 mac 的三态形状对齐
///（Windows 无 notDetermined/系统拒绝态，⚠️/✗（已拒绝）不出现）。
pub fn menu_text(key: &str) -> String {
    let label = PERM_TOGGLES
        .iter()
        .find(|s| s.key == key)
        .map(|s| s.label)
        .unwrap_or(key);
    if perm_disabled(key) {
        format!("✗ {label}（已关闭）")
    } else {
        format!("✓ {label}")
    }
}

/// 重置单项权限（Windows：清本地关闭标志，恢复默认可用）。mac 的 tccutil 收回在
/// Windows 无对应物（系统本就无授权）。
pub fn reset_permission(key: &str) -> Result<(), String> {
    if PERM_TOGGLES.iter().all(|s| s.key != key) {
        return Err(format!("未知权限 {key}"));
    }
    set_perm_disabled(key, false);
    plog(&format!("重置权限 {key}（清本地关闭标志）"));
    Ok(())
}

/// 收回本 App 的全部「自动化」授权对。mac 专属（AppleEvents/tccutil）；Windows 无对应物，
/// no-op 成功返回（app_perm::reset_all 的重置语义在 Windows 只清本地引擎状态）。
pub fn reset_apple_events() -> Result<(), String> {
    plog("reset_apple_events：Windows 无 AppleEvents，no-op");
    Ok(())
}

/// 托盘权限项点击的完整切换流（阻塞毫秒级本地写，调用方可直接同步调）；
/// 返回刷新后的菜单文字。Windows 的切换就是翻本地开关（mac 的 tccutil/系统弹框
/// 在 Windows 无对应物），无引导对话框路径。
pub fn handle_tray_toggle(key: &str) -> String {
    if perm_disabled(key) {
        // 已关闭 → 开启：清本地标志，能力即刻恢复（ensure_* 下次调用即放行）
        set_perm_disabled(key, false);
        plog(&format!("托盘开启权限 {key}"));
    } else {
        // 可用 → 关闭：本地记住（各 ensure_* 立即开始拒绝，fail-closed）
        set_perm_disabled(key, true);
        plog(&format!("托盘关闭权限 {key}"));
    }
    menu_text(key)
}
