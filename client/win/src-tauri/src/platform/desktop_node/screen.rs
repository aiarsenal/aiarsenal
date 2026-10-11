//! 屏幕截屏（GDI BitBlt，对齐 mac screen.rs 的对外形态：capture(idx)/record(params)/Snapshot）。
//!
//! Windows 实现：EnumDisplayMonitors 枚举显示器（index = 按 left,top 排序，左→右、上→下；
//! mac 是 NSScreen 顺序=主屏优先，语义略有差异——agent 拿 displayFrameId/screenIndex 前先
//! 全屏截一张即可确认）。逐显示器：GetDC(None) 屏幕DC → CreateCompatibleDC →
//! CreateDIBSection(32bpp top-down) → BitBlt(SRCCOPY|CAPTUREBLT) → BGRA→RGBA → PNG。
//!
//! screen.record 在 Windows 无 AVFoundation 对应物（原生录屏要 Media Foundation Sink Writer
//! 或 Windows.Graphics.Capture，复杂度不成比例）→ 结构化 Unsupported 错误（与 mac 端
//!「无 TCC 授权」错误同形，agent 可读可降级）。
//!
//! DPI：BitBlt 用物理像素。若进程未设 DPI awareness（无窗口早于 desktop_node 启动时），
//! 坐标可能被虚拟化——tauri 建主窗口时已设 per-monitor awareness，desktop_node 心跳期
//! 启动晚于 setup，实际路径下均为物理像素。

use serde_json::{json, Value};

/// 截屏结果（protocol.rs 包装进 node 协议 payload；字段与 mac Snapshot 同名）。
pub struct Snapshot {
    pub format: String,
    pub base64: String,
    pub display_frame_id: String,
    pub width: i64,
    pub height: i64,
}

/// 截指定显示器（0 起）。成功返回 PNG base64 + 宽高。
/// Win32 实现在 capture_gdi（windows crate 只进 Windows 目标——windows-future 传递依赖
/// 在非 Windows 宿主编不过，宿主 check 时整个 GDI 段 cfg 掉，行为不受影响）。
pub fn capture(screen_index: usize) -> Result<Snapshot, String> {
    #[cfg(target_os = "windows")]
    return capture_gdi(screen_index);
    #[cfg(not(target_os = "windows"))]
    {
        let _ = screen_index;
        Err("screen.capture 仅 Windows 目标可用".into())
    }
}

/// GDI 截屏实现（仅 Windows 目标编译）。
#[cfg(target_os = "windows")]
fn capture_gdi(screen_index: usize) -> Result<Snapshot, String> {
    use base64::Engine as _;
    let rects = enum_monitor_rects()?;
    if rects.is_empty() {
        return Err("未枚举到显示器".into());
    }
    if screen_index >= rects.len() {
        return Err(format!(
            "screenIndex {screen_index} 超出范围（共 {} 个显示器）",
            rects.len()
        ));
    }
    let rect = rects[screen_index];
    let w = (rect.right - rect.left) as usize;
    let h = (rect.bottom - rect.top) as usize;
    if w == 0 || h == 0 {
        return Err(format!("显示器 {screen_index} 尺寸异常: {rect:?}"));
    }
    let bgra = bitblt_monitor(&rect, w, h)?;
    // BGRA → RGBA（CreateDIBSection 32bpp 内存序 B,G,R,A）
    let mut rgba = bgra;
    for px in rgba.chunks_exact_mut(4) {
        px.swap(0, 2);
    }
    let img = image::RgbaImage::from_raw(w as u32, h as u32, rgba)
        .ok_or_else(|| "像素缓冲尺寸不匹配".to_string())?;
    let mut png = Vec::new();
    img.write_to(&mut std::io::Cursor::new(&mut png), image::ImageFormat::Png)
        .map_err(|e| format!("PNG 编码失败: {e}"))?;
    Ok(Snapshot {
        format: "png".into(),
        base64: base64::engine::general_purpose::STANDARD.encode(&png),
        display_frame_id: format!("display-{screen_index}"),
        width: w as i64,
        height: h as i64,
    })
}

/// screen.record：Windows 暂不支持视频录制（结构化 Unsupported，shape 对齐 mac）。
pub fn record(_params: &Value) -> (bool, Value) {
    (
        false,
        json!({
            "error": {
                "code": "UNAVAILABLE",
                "message": "screen.record 在 Windows 客户端暂不支持（无屏幕录制实现）"
            }
        }),
    )
}

// ===== Win32：显示器枚举 + BitBlt（仅 Windows 目标编译） =====

#[cfg(target_os = "windows")]
fn enum_monitor_rects() -> Result<Vec<windows::Win32::Foundation::RECT>, String> {
    use windows::Win32::Foundation::{LPARAM, RECT};
    use windows::Win32::Graphics::Gdi::EnumDisplayMonitors;
    // 回调把 RECT 推进 leaked Vec（LPARAM 携指针），调用后回收。
    let collected: Box<Vec<RECT>> = Box::default();
    let lparam = LPARAM(Box::into_raw(collected) as isize);
    let _ = unsafe {
        EnumDisplayMonitors(
            None,  // NULL hdc：按虚拟屏幕枚举全部显示器
            None,  // 无裁剪矩形
            Some(monitor_enum_proc),
            lparam,
        )
    };
    let mut rects = unsafe { Box::from_raw(lparam.0 as *mut Vec<RECT>) };
    // 稳定排序：左→右、上→下（index 语义）
    rects.sort_by(|a, b| (a.left, a.top).cmp(&(b.left, b.top)));
    Ok(*rects)
}

#[cfg(target_os = "windows")]
unsafe extern "system" fn monitor_enum_proc(
    _hmonitor: windows::Win32::Graphics::Gdi::HMONITOR,
    _hdc: windows::Win32::Graphics::Gdi::HDC,
    rect: *mut windows::Win32::Foundation::RECT,
    lparam: windows::Win32::Foundation::LPARAM,
) -> windows::core::BOOL {
    use windows::core::BOOL;
    if rect.is_null() {
        return BOOL(1); // 继续枚举
    }
    let list = &mut *(lparam.0 as *mut Vec<windows::Win32::Foundation::RECT>);
    list.push(*rect);
    BOOL(1) // TRUE：继续
}

/// BitBlt 抓指定矩形 → BGRA 原始字节（len = w*h*4）。
#[cfg(target_os = "windows")]
fn bitblt_monitor(rect: &windows::Win32::Foundation::RECT, w: usize, h: usize) -> Result<Vec<u8>, String> {
    use windows::Win32::Graphics::Gdi::{
        BitBlt, CreateCompatibleDC, CreateDIBSection, DeleteDC, DeleteObject, GetDC, ReleaseDC,
        SelectObject, BITMAPINFO, BITMAPINFOHEADER, BI_RGB, CAPTUREBLT, DIB_RGB_COLORS, SRCCOPY,
    };
    unsafe {
        let hdc_screen = GetDC(None);
        if hdc_screen.is_invalid() {
            return Err("GetDC 失败".into());
        }
        let hdc_mem = CreateCompatibleDC(Some(hdc_screen));
        if hdc_mem.is_invalid() {
            ReleaseDC(None, hdc_screen);
            return Err("CreateCompatibleDC 失败".into());
        }
        let mut bmi = BITMAPINFO::default();
        bmi.bmiHeader = BITMAPINFOHEADER {
            biSize: std::mem::size_of::<BITMAPINFOHEADER>() as u32,
            biWidth: w as i32,
            // 负高 = top-down（内存首行是屏幕顶行，不用翻转）
            biHeight: -(h as i32),
            biPlanes: 1,
            biBitCount: 32,
            biCompression: BI_RGB.0,
            ..Default::default()
        };
        let mut bits: *mut core::ffi::c_void = std::ptr::null_mut();
        let hbmp = match CreateDIBSection(Some(hdc_mem), &bmi, DIB_RGB_COLORS, &mut bits, None, 0) {
            Ok(b) if !bits.is_null() => b,
            Ok(_) => {
                let _ = DeleteDC(hdc_mem);
                ReleaseDC(None, hdc_screen);
                return Err("CreateDIBSection 失败（位缓冲为空）".into());
            }
            Err(e) => {
                let _ = DeleteDC(hdc_mem);
                ReleaseDC(None, hdc_screen);
                return Err(format!("CreateDIBSection 失败: {e}"));
            }
        };
        let old = SelectObject(hdc_mem, hbmp.into());
        let blt = BitBlt(
            hdc_mem,
            0,
            0,
            w as i32,
            h as i32,
            Some(hdc_screen),
            rect.left,
            rect.top,
            SRCCOPY | CAPTUREBLT, // CAPTUREBLT：含分层窗口（同 mac 的整屏语义）
        );
        let ok = blt.is_ok();
        // 先回收 GDI 资源再决定返回
        SelectObject(hdc_mem, old);
        let _ = DeleteObject(hbmp.into());
        let _ = DeleteDC(hdc_mem);
        ReleaseDC(None, hdc_screen);
        if !ok {
            return Err(format!(
                "BitBlt 失败（显示器区域 {}x{} @ {},{}）",
                w,
                h,
                rect.left,
                rect.top
            ));
        }
        let len = w * h * 4;
        let mut out = vec![0u8; len];
        std::ptr::copy_nonoverlapping(bits as *const u8, out.as_mut_ptr(), len);
        Ok(out)
    }
}
