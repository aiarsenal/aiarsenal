//! openclaw desktop node 的控鼠键（computer.act 命令执行）。
//!
//! FFI 直接合成 CGEvent 鼠标/键盘事件（不依赖 crate，API 确定性强）。
//! 要 Accessibility 权限（控鼠键绑 client，permissions.rs 申请）。
//!
//! 阶段 3 范围：mouse（move/click/drag/scroll/down/up）+ type_text（unicode 输入）完整。
//! key combo（cmd+shift+t 这类修饰键组合）的 keycode 表后续补（CGEventCreateKeyboardEvent + CGEventSetFlags）。

use std::ffi::c_void;

#[repr(C)]
#[derive(Clone, Copy)]
struct CGPoint {
    x: f64,
    y: f64,
}

#[link(name = "CoreGraphics", kind = "framework")]
extern "C" {
    fn CGEventCreateMouseEvent(
        source: *mut c_void,
        ty: u32,
        coord: CGPoint,
        button: u32,
    ) -> *mut c_void;
    fn CGEventCreateScrollWheelEvent2(
        source: *mut c_void,
        units: u32,
        wheel_count: u32,
        w1: i32,
        w2: i32,
        w3: i32,
    ) -> *mut c_void;
    fn CGEventCreateKeyboardEvent(source: *mut c_void, key_code: u16, key_down: bool) -> *mut c_void;
    fn CGEventKeyboardSetUnicodeString(event: *mut c_void, max_length: usize, string: *const u16);
    fn CGEventSetFlags(event: *mut c_void, flags: u64);
    fn CGEventPost(tap: u32, event: *mut c_void);
    fn CGEventSourceCreate(state_id: u32) -> *mut c_void;
}

// 常量（CoreGraphics）
const HID_TAP: u32 = 0; // kCGHIDEventTap
const SRC_HID: u32 = 1; // kCGEventSourceStateHIDSystemState
const SCROLL_PIXEL: u32 = 0; // kCGScrollEventUnitPixel

// CGEventType（鼠标）
const T_L_DOWN: u32 = 1;
const T_L_UP: u32 = 2;
const T_R_DOWN: u32 = 3;
const T_R_UP: u32 = 4;
const T_MOUSE_MOVED: u32 = 5;
const T_O_DOWN: u32 = 25;
const T_O_UP: u32 = 26;

// CGMouseButton
const B_LEFT: u32 = 0;
const B_RIGHT: u32 = 1;

// CGEventFlags（修饰键掩码）
const FLAG_CMD: u64 = 0x100000; // kCGEventFlagMaskCommand
const FLAG_SHIFT: u64 = 0x020000; // kCGEventFlagMaskShift
const FLAG_CTRL: u64 = 0x040000; // kCGEventFlagMaskControl
const FLAG_ALT: u64 = 0x080000; // kCGEventFlagMaskAlternate (option)

fn src() -> *mut c_void {
    unsafe { CGEventSourceCreate(SRC_HID) }
}

fn post_mouse(ty: u32, x: f64, y: f64, button: u32) -> Result<(), String> {
    let s = src();
    if s.is_null() {
        return Err("CGEventSource null（可能 Accessibility 未授权）".into());
    }
    let e = unsafe { CGEventCreateMouseEvent(s, ty, CGPoint { x, y }, button) };
    if e.is_null() {
        return Err("CGEventCreateMouseEvent null".into());
    }
    unsafe { CGEventPost(HID_TAP, e) };
    Ok(())
}

/// 移动鼠标到 (x, y)。
pub fn move_to(x: f64, y: f64) -> Result<(), String> {
    post_mouse(T_MOUSE_MOVED, x, y, B_LEFT)
}

/// 点击（down + up）。button: "left" / "right" / "middle"。
pub fn click(button: &str, x: f64, y: f64) -> Result<(), String> {
    let (d, u, b) = match button {
        "right" => (T_R_DOWN, T_R_UP, B_RIGHT),
        "middle" => (T_O_DOWN, T_O_UP, 2),
        _ => (T_L_DOWN, T_L_UP, B_LEFT),
    };
    post_mouse(d, x, y, b)?;
    post_mouse(u, x, y, b)?;
    Ok(())
}

/// 连点 n 次（double_click=2 / triple_click=3）。
pub fn click_n(button: &str, x: f64, y: f64, n: u32) -> Result<(), String> {
    for _ in 0..n {
        click(button, x, y)?;
    }
    Ok(())
}

/// 拖拽：down @ from → move → up @ to。
pub fn drag(from_x: f64, from_y: f64, to_x: f64, to_y: f64) -> Result<(), String> {
    post_mouse(T_L_DOWN, from_x, from_y, B_LEFT)?;
    post_mouse(T_MOUSE_MOVED, to_x, to_y, B_LEFT)?;
    post_mouse(T_L_UP, to_x, to_y, B_LEFT)?;
    Ok(())
}

/// 鼠标按下保持（computer.act 的 left_mouse_down，配合 release）。
pub fn mouse_down(button: &str, x: f64, y: f64) -> Result<(), String> {
    let (d, b) = match button {
        "right" => (T_R_DOWN, B_RIGHT),
        "middle" => (T_O_DOWN, 2),
        _ => (T_L_DOWN, B_LEFT),
    };
    post_mouse(d, x, y, b)
}

/// 释放鼠标（left_mouse_up）。
pub fn mouse_up(button: &str, x: f64, y: f64) -> Result<(), String> {
    let (u, b) = match button {
        "right" => (T_R_UP, B_RIGHT),
        "middle" => (T_O_UP, 2),
        _ => (T_L_UP, B_LEFT),
    };
    post_mouse(u, x, y, b)
}

/// 滚轮。direction: "up"/"down"/"left"/"right"，amount: 滚轮刻度。
pub fn scroll(direction: &str, amount: i32) -> Result<(), String> {
    let s = src();
    if s.is_null() {
        return Err("CGEventSource null".into());
    }
    // 垂直 w1，水平 w2；正=上/右，负=下/左
    let (w1, w2) = match direction {
        "up" => (amount, 0),
        "down" => (-amount, 0),
        "right" => (0, amount),
        "left" => (0, -amount),
        _ => (amount, 0),
    };
    let e = unsafe { CGEventCreateScrollWheelEvent2(s, SCROLL_PIXEL, 2, w1, w2, 0) };
    if e.is_null() {
        return Err("CGEventCreateScrollWheelEvent2 null".into());
    }
    unsafe { CGEventPost(HID_TAP, e) };
    Ok(())
}

/// 输入文本（逐字符 unicode，覆盖 computer.act 的 type）。
/// 用 CGEventKeyboardSetUnicodeString，不需 keycode 表。
pub fn type_text(text: &str) -> Result<(), String> {
    let s = src();
    if s.is_null() {
        return Err("CGEventSource null".into());
    }
    for u in text.encode_utf16() {
        // key_down event（keycode=0 占位）→ 注入 unicode string → post → key_up
        let down = unsafe { CGEventCreateKeyboardEvent(s, 0, true) };
        if down.is_null() {
            return Err("CGEventCreateKeyboardEvent null".into());
        }
        let buf = [u];
        unsafe { CGEventKeyboardSetUnicodeString(down, 1, buf.as_ptr()) };
        unsafe { CGEventPost(HID_TAP, down) };
        let up = unsafe { CGEventCreateKeyboardEvent(s, 0, false) };
        if !up.is_null() {
            unsafe { CGEventPost(HID_TAP, up) };
        }
    }
    Ok(())
}

/// key combo（如 "cmd+shift+t" / "Return"）：解析修饰键 + 末键 → CGEvent key down + up。
/// 支持修饰 cmd/ctrl/shift/alt(option) + 末键（字母数字 / 常见键名 Return/Tab/Space/方向/F键）。
pub fn key_combo(combo: &str) -> Result<(), String> {
    let (source, keycode, flags) = parse_combo(combo)?;
    post_key(source, keycode, flags, true);
    post_key(source, keycode, flags, false);
    Ok(())
}

/// 长按键（hold_key）：key down → sleep duration → up。duration 上限 10s。
pub fn hold_key(combo: &str, duration_ms: u64) -> Result<(), String> {
    let (source, keycode, flags) = parse_combo(combo)?;
    post_key(source, keycode, flags, true);
    std::thread::sleep(std::time::Duration::from_millis(duration_ms.min(10000)));
    post_key(source, keycode, flags, false);
    Ok(())
}

/// 解析 combo → (source, keycode, flags)。
fn parse_combo(combo: &str) -> Result<(*mut c_void, u16, u64), String> {
    let s = src();
    if s.is_null() {
        return Err("CGEventSource null（可能 Accessibility 未授权）".into());
    }
    let mut flags: u64 = 0;
    let mut keycode: Option<u16> = None;
    for part in combo.split('+').map(|p| p.trim().to_lowercase()) {
        match part.as_str() {
            "cmd" | "command" | "meta" => flags |= FLAG_CMD,
            "ctrl" | "control" => flags |= FLAG_CTRL,
            "shift" => flags |= FLAG_SHIFT,
            "alt" | "option" | "opt" => flags |= FLAG_ALT,
            other => {
                if keycode.is_some() {
                    return Err(format!("combo 只能一个末键: {combo}"));
                }
                keycode = Some(keycode_of(other).ok_or_else(|| format!("未知键: {other}"))?);
            }
        }
    }
    let kc = keycode.ok_or_else(|| format!("combo 缺末键: {combo}"))?;
    Ok((s, kc, flags))
}

/// 单键事件（down/up）+ 修饰键 flags。
fn post_key(source: *mut c_void, keycode: u16, flags: u64, down: bool) {
    let e = unsafe { CGEventCreateKeyboardEvent(source, keycode, down) };
    if e.is_null() {
        return;
    }
    if flags != 0 {
        unsafe { CGEventSetFlags(e, flags) };
    }
    unsafe { CGEventPost(HID_TAP, e) };
}

/// 键名/字符 → mac virtual keycode（kVK_ANSI_*，常用字母数字 + 特殊键）。
fn keycode_of(name: &str) -> Option<u16> {
    let code = match name {
        // 字母（mac keycode 不规则）
        "a" => 0, "s" => 1, "d" => 2, "f" => 3, "h" => 4, "g" => 5, "z" => 6, "x" => 7,
        "c" => 8, "v" => 9, "b" => 11, "q" => 12, "w" => 13, "e" => 14, "r" => 15,
        "y" => 16, "t" => 17, "u" => 32, "i" => 34, "o" => 31, "p" => 35,
        "l" => 37, "j" => 38, "k" => 40, "n" => 45, "m" => 46,
        // 数字
        "0" => 29, "1" => 18, "2" => 19, "3" => 20, "4" => 21, "5" => 23, "6" => 22,
        "7" => 26, "8" => 28, "9" => 25,
        // 特殊键
        "return" | "enter" => 36, "tab" => 48, "space" => 49, "delete" | "backspace" => 51,
        "escape" | "esc" => 53, "forwarddelete" | "fn_delete" => 117,
        "home" => 115, "end" => 119, "pageup" => 116, "pagedown" => 121,
        "left" => 123, "right" => 124, "down" => 125, "up" => 126,
        // F 键
        "f1" => 122, "f2" => 120, "f3" => 99, "f4" => 118, "f5" => 96, "f6" => 97,
        "f7" => 98, "f8" => 100, "f9" => 101, "f10" => 109, "f11" => 103, "f12" => 111,
        // 符号（部分）
        "-" | "minus" => 27, "=" | "equal" => 24, "[" => 33, "]" => 30, "\\" => 42,
        ";" => 41, "'" => 39, "`" => 50, "," => 43, "." => 47, "/" => 44,
        _ => return None,
    };
    Some(code)
}
