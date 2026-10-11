//! camera.list / camera.snap / camera.clip（对齐 openclaw node 协议）。
//!
//! list：Media Foundation 枚举视频捕获设备（MFEnumDeviceSources，VIDCAP source type），
//! 设备 id 用符号链接（稳定跨重启，mac 用 AVCaptureDevice uniqueID 的同位口径），
//! name 用 FriendlyName，position 恒 "unspecified"（Windows 枚举不给前后置语义）。
//!
//! snap / clip：Windows 客户端暂不支持——取流要 IMFMediaSource + Sink Writer（拍照）/
//! Windows.Graphics.Capture 或 MF Capture Engine（录像），复杂度不成比例，v1 不做。
//! 返回结构化 UNAVAILABLE（与 mac 的错误形状同形，agent 可读可降级）；设备枚举不受
//! 影响（camera.list 恒可用，agent 可先用它确认摄像头在不在）。
//!
//! COM/MF 生命周期：每次枚举独立 CoInitializeEx + MFStartup/MFShutdown（WS 读线程
//! 可能已是 STA，RPC_E_CHANGED_MODE 时沿用现有 apartment 继续）；激活数组先逐项
//! drop 接口引用再 CoTaskMemFree，MFShutdown 无论成败都执行。枚举失败不致命，
//! camera.list 返回错误描述（agent 侧可读）。

use serde_json::{json, Value};

/// camera.list：枚举摄像头设备。
/// （MF 段按 target cfg：windows crate 只进 Windows 目标，宿主 check 走占位错误。）
pub fn list() -> (bool, Value) {
    #[cfg(target_os = "windows")]
    return match mf_video_devices() {
        Ok(devices) => (true, json!({ "devices": devices })),
        Err(e) => (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": format!("摄像头枚举失败: {e}")}}),
        ),
    };
    #[cfg(not(target_os = "windows"))]
    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "camera.list 仅 Windows 目标可用"}}),
    )
}

/// camera.snap：拍照。Windows 暂不支持（取流拍照未实现；设备枚举可用 camera.list）。
pub fn snap(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_camera(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: camera（摄像头能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let _ = params; // facing/delayMs/maxWidth/quality/format/deviceId 留待真实现
    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "camera.snap 在 Windows 客户端暂不支持（无取流拍照实现，设备枚举可用 camera.list）"}}),
    )
}

/// camera.clip：录像。同上。
pub fn clip(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_camera(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: camera（摄像头能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let _ = params; // durationMs/includeAudio/deviceId 留待真实现
    (
        false,
        json!({"error": {"code": "UNAVAILABLE", "message": "camera.clip 在 Windows 客户端暂不支持（无取流录像实现，设备枚举可用 camera.list）"}}),
    )
}

// ===== Media Foundation 设备枚举 =====

/// 枚举全部视频捕获设备 → [{id, name, position}]（仅 Windows 目标编译）。
#[cfg(target_os = "windows")]
fn mf_video_devices() -> Result<Vec<Value>, String> {
    use windows::Win32::Media::MediaFoundation::{
        MFCreateAttributes, MFEnumDeviceSources, MFShutdown, MFStartup, IMFActivate,
        MFSTARTUP_NOSOCKET, MF_DEVSOURCE_ATTRIBUTE_FRIENDLY_NAME,
        MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE, MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP_GUID,
        MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP_SYMBOLIC_LINK, MF_VERSION,
    };
    use windows::Win32::System::Com::{CoInitializeEx, COINIT, COINIT_MULTITHREADED};

    // COM：线程可能已有 apartment（RPC_E_CHANGED_MODE）——失败继续，MF 不强制 MTA
    unsafe {
        let _ = CoInitializeEx(None, COINIT(COINIT_MULTITHREADED.0));
        MFStartup(MF_VERSION, MFSTARTUP_NOSOCKET).map_err(|e| format!("MFStartup: {e}"))?;
    }

    let result = (|| -> Result<Vec<Value>, String> {
        unsafe {
            let mut attrs = None;
            MFCreateAttributes(&mut attrs, 1).map_err(|e| format!("MFCreateAttributes: {e}"))?;
            let attrs = attrs.ok_or("MFCreateAttributes 未返回属性对象")?;
            attrs
                .SetGUID(
                    &MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE,
                    &MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP_GUID,
                )
                .map_err(|e| format!("SetGUID: {e}"))?;

            let mut ptr: *mut Option<IMFActivate> = std::ptr::null_mut();
            let mut count: u32 = 0;
            MFEnumDeviceSources(&attrs, &mut ptr, &mut count)
                .map_err(|e| format!("MFEnumDeviceSources: {e}"))?;
            if ptr.is_null() || count == 0 {
                return Ok(Vec::new());
            }

            let mut devices = Vec::new();
            for i in 0..count as usize {
                let slot = ptr.add(i);
                let Some(activate) = (*slot).as_ref() else { continue };
                let name = activate_get_string(activate, &MF_DEVSOURCE_ATTRIBUTE_FRIENDLY_NAME);
                let symlink = activate_get_string(activate, &MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE_VIDCAP_SYMBOLIC_LINK);
                devices.push(json!({
                    "id": if symlink.is_empty() { name.clone() } else { symlink },
                    "name": name,
                    "position": "unspecified",
                }));
            }
            // 数组是 CoTaskMemAlloc 的：先逐项 drop 接口引用（Release），再释放数组本体
            for i in 0..count as usize {
                std::ptr::drop_in_place(ptr.add(i));
            }
            windows::Win32::System::Com::CoTaskMemFree(Some(ptr.cast::<core::ffi::c_void>()));
            Ok(devices)
        }
    })();

    unsafe {
        let _ = MFShutdown();
    }
    result
}

/// IMFActivate.GetAllocatedString（CoTaskMemAlloc 的 PWSTR，用完 CoTaskMemFree）。
#[cfg(target_os = "windows")]
fn activate_get_string(
    activate: &windows::Win32::Media::MediaFoundation::IMFActivate,
    guid: &windows::core::GUID,
) -> String {
    use windows::Win32::System::Com::CoTaskMemFree;
    use windows::core::PWSTR;
    unsafe {
        let mut pw: PWSTR = PWSTR::null();
        let mut len: u32 = 0;
        if activate.GetAllocatedString(guid, &mut pw, &mut len).is_err() {
            return String::new();
        }
        let out = if pw.0.is_null() {
            String::new()
        } else {
            let mut end = 0usize;
            while end < len as usize && *pw.0.add(end) != 0 {
                end += 1;
            }
            String::from_utf16_lossy(std::slice::from_raw_parts(pw.0, end))
        };
        CoTaskMemFree(Some(pw.0.cast::<core::ffi::c_void>()));
        out
    }
}
