//! openclaw desktop node 的控鼠键（computer.act 命令执行）。
//!
//! Windows 实现：SendInput 合成鼠标/键盘事件（user32，GUI 程序注入的标准路径；无 UAC
//! 隔离块则前台窗口可收）。对外 API（move_to/click/click_n/drag/mouse_down/mouse_up/
//! scroll/type_text/key_combo/hold_key）与错误文案与 mac control.rs 同构（computer.act
//! 的分支错误直接透传 agent）。
//!
//! 修饰键映射（Windows 无 Cmd 键，务实选择）：
//! - ctrl/control → VK_CONTROL；shift → VK_SHIFT；alt/option/opt → VK_MENU
//! - cmd/command/meta → 按 Ctrl 处理（mac 语义迁移：agent 常发的 "cmd+c" 在 Windows
//!   目标程序里等价 "ctrl+c"；不映射 VK_LWIN 是因为 Win 键注入会弹出开始菜单干扰前台）
//! - win/super → VK_LWIN
//!
//! 坐标：MOUSEEVENTF_ABSOLUTE 归一化到虚拟屏幕（多显示器），与 mac 全局坐标语义一致。
//! 滚轮：MOUSEEVENTF_WHEEL（正=上）/ MOUSEEVENTF_HWHEEL（正=右），amount=刻度×WHEEL_DELTA。
//!
//! 目标门：windows crate 只进 Windows 目标（其传递依赖 windows-future 在非 Windows
//! 宿主编不过），实现整体放 imp 模块按 target cfg——宿主 macOS 交叉 check 编占位桩。

/// Windows 目标实现（SendInput）。
#[cfg(target_os = "windows")]
mod imp {
    use windows::Win32::UI::Input::KeyboardAndMouse::{
        SendInput, INPUT, INPUT_0, INPUT_KEYBOARD, INPUT_MOUSE, KEYBDINPUT, KEYBD_EVENT_FLAGS,
        KEYEVENTF_KEYUP, KEYEVENTF_UNICODE, MOUSEEVENTF_ABSOLUTE, MOUSEEVENTF_HWHEEL,
        MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP, MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP,
        MOUSEEVENTF_MOVE, MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP, MOUSEEVENTF_WHEEL, MOUSEINPUT,
        VIRTUAL_KEY, VK_BACK, VK_CONTROL, VK_DELETE, VK_END, VK_ESCAPE, VK_HOME, VK_LWIN, VK_MENU,
        VK_NEXT, VK_PRIOR, VK_RETURN, VK_SHIFT, VK_SPACE, VK_TAB,
    };
    use windows::Win32::UI::WindowsAndMessaging::{
        GetSystemMetrics, SM_CXVIRTUALSCREEN, SM_CYVIRTUALSCREEN, SM_XVIRTUALSCREEN,
        SM_YVIRTUALSCREEN,
    };

    const WHEEL_DELTA: i32 = 120;
    // 组合键/点击事件间的小间隙：部分程序（尤其游戏/全屏）对零间隔事件流丢帧
    const INTER_EVENT_MS: u64 = 12;

    // ===== 鼠标 =====

    /// 虚拟屏幕参数（原点 + 尺寸）；枚举失败回退单主屏 0,0。
    fn virtual_screen() -> (i32, i32, i32, i32) {
        unsafe {
            let vx = GetSystemMetrics(SM_XVIRTUALSCREEN);
            let vy = GetSystemMetrics(SM_YVIRTUALSCREEN);
            let vw = GetSystemMetrics(SM_CXVIRTUALSCREEN);
            let vh = GetSystemMetrics(SM_CYVIRTUALSCREEN);
            if vw <= 0 || vh <= 0 {
                (0, 0, 1920, 1080)
            } else {
                (vx, vy, vw, vh)
            }
        }
    }

    /// 绝对坐标归一化（0..65535 虚拟屏幕）。
    fn normalize(x: f64, y: f64) -> (i32, i32) {
        let (vx, vy, vw, vh) = virtual_screen();
        let dx = ((x - vx as f64) * 65536.0 / vw as f64).round() as i64;
        let dy = ((y - vy as f64) * 65536.0 / vh as f64).round() as i64;
        (dx.clamp(0, 65535) as i32, dy.clamp(0, 65535) as i32)
    }

    /// 注入一组鼠标事件。返回失败描述（0 注入 = 被拦截）。
    fn post_mouse(mi: MOUSEINPUT) -> Result<(), String> {
        let input = INPUT {
            r#type: INPUT_MOUSE,
            Anonymous: INPUT_0 { mi },
        };
        let sent = unsafe { SendInput([input].as_ref(), std::mem::size_of::<INPUT>() as i32) };
        if sent == 0 {
            return Err("SendInput 鼠标注入失败（前台窗口可能以管理员权限运行）".into());
        }
        Ok(())
    }

    fn abs_move(x: f64, y: f64) -> Result<(), String> {
        let (dx, dy) = normalize(x, y);
        post_mouse(MOUSEINPUT {
            dx,
            dy,
            mouseData: 0,
            dwFlags: MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    pub(super) fn move_to(x: f64, y: f64) -> Result<(), String> {
        abs_move(x, y)
    }

    /// 点击（down + up）。button: "left" / "right" / "middle"。
    pub(super) fn click(button: &str, x: f64, y: f64) -> Result<(), String> {
        let (d, u) = match button {
            "right" => (MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP),
            "middle" => (MOUSEEVENTF_MIDDLEDOWN, MOUSEEVENTF_MIDDLEUP),
            "left" => (MOUSEEVENTF_LEFTDOWN, MOUSEEVENTF_LEFTUP),
            other => return Err(format!("无效按钮: {other}")),
        };
        abs_move(x, y)?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: d,
            time: 0,
            dwExtraInfo: 0,
        })?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: u,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    /// 连点 n 次（double_click=2 / triple_click=3）。
    pub(super) fn click_n(button: &str, x: f64, y: f64, n: u32) -> Result<(), String> {
        for _ in 0..n {
            click(button, x, y)?;
            std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        }
        Ok(())
    }

    /// 拖拽：down @ from → 分步 move → up @ to（分步保证目标程序识别为拖拽而非瞬移）。
    pub(super) fn drag(from_x: f64, from_y: f64, to_x: f64, to_y: f64) -> Result<(), String> {
        abs_move(from_x, from_y)?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: MOUSEEVENTF_LEFTDOWN,
            time: 0,
            dwExtraInfo: 0,
        })?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        let steps = 10;
        for i in 1..=steps {
            let t = i as f64 / steps as f64;
            abs_move(from_x + (to_x - from_x) * t, from_y + (to_y - from_y) * t)?;
            std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        }
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: MOUSEEVENTF_LEFTUP,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    /// 鼠标按下保持（computer.act 的 left_mouse_down，配合 release）。
    pub(super) fn mouse_down(button: &str, x: f64, y: f64) -> Result<(), String> {
        let d = match button {
            "right" => MOUSEEVENTF_RIGHTDOWN,
            "middle" => MOUSEEVENTF_MIDDLEDOWN,
            "left" => MOUSEEVENTF_LEFTDOWN,
            other => return Err(format!("无效按钮: {other}")),
        };
        abs_move(x, y)?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: d,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    /// 释放鼠标（left_mouse_up）。
    pub(super) fn mouse_up(button: &str, x: f64, y: f64) -> Result<(), String> {
        let u = match button {
            "right" => MOUSEEVENTF_RIGHTUP,
            "middle" => MOUSEEVENTF_MIDDLEUP,
            "left" => MOUSEEVENTF_LEFTUP,
            other => return Err(format!("无效按钮: {other}")),
        };
        abs_move(x, y)?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: 0,
            dwFlags: u,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    /// 滚轮。direction: "up"/"down"/"left"/"right"，amount: 刻度（正=上/右）。
    pub(super) fn scroll(direction: &str, amount: i32) -> Result<(), String> {
        // 垂直 WHEEL / 水平 HWHEEL；正=上/右，负=下/左（与 mac 语义一致）
        let (flags, delta) = match direction {
            "up" => (MOUSEEVENTF_WHEEL, amount * WHEEL_DELTA),
            "down" => (MOUSEEVENTF_WHEEL, -amount * WHEEL_DELTA),
            "right" => (MOUSEEVENTF_HWHEEL, amount * WHEEL_DELTA),
            "left" => (MOUSEEVENTF_HWHEEL, -amount * WHEEL_DELTA),
            _ => (MOUSEEVENTF_WHEEL, amount * WHEEL_DELTA),
        };
        post_mouse(MOUSEINPUT {
            dx: 0,
            dy: 0,
            mouseData: delta as u32,
            dwFlags: flags,
            time: 0,
            dwExtraInfo: 0,
        })
    }

    // ===== 键盘 =====

    fn post_key(vk: VIRTUAL_KEY, scan: u16, unicode: bool, down: bool) -> Result<(), String> {
        let dw_flags = match (unicode, down) {
            (true, true) => KEYEVENTF_UNICODE,
            (true, false) => KEYEVENTF_UNICODE | KEYEVENTF_KEYUP,
            (false, true) => KEYBD_EVENT_FLAGS(0),
            (false, false) => KEYEVENTF_KEYUP,
        };
        let input = INPUT {
            r#type: INPUT_KEYBOARD,
            Anonymous: INPUT_0 {
                ki: KEYBDINPUT {
                    wVk: if unicode { VIRTUAL_KEY(0) } else { vk },
                    wScan: scan,
                    dwFlags: dw_flags,
                    time: 0,
                    dwExtraInfo: 0,
                },
            },
        };
        let sent = unsafe { SendInput([input].as_ref(), std::mem::size_of::<INPUT>() as i32) };
        if sent == 0 {
            return Err("SendInput 键盘注入失败（前台窗口可能以管理员权限运行）".into());
        }
        Ok(())
    }

    fn key_down(vk: VIRTUAL_KEY) -> Result<(), String> {
        post_key(vk, 0, false, true)
    }

    fn key_up(vk: VIRTUAL_KEY) -> Result<(), String> {
        post_key(vk, 0, false, false)
    }

    /// 输入文本（逐 UTF-16 码元 KEYEVENTF_UNICODE，覆盖 computer.act 的 type；不需 keycode 表）。
    /// '\n' 走 VK_RETURN（unicode 注入 0x0A 多数程序不认）。
    pub(super) fn type_text(text: &str) -> Result<(), String> {
        for u in text.encode_utf16() {
            if u == 0x0A {
                key_down(VK_RETURN)?;
                std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
                key_up(VK_RETURN)?;
            } else {
                post_key(VIRTUAL_KEY(0), u, true, true)?;
                std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
                post_key(VIRTUAL_KEY(0), u, true, false)?;
            }
            std::thread::sleep(std::time::Duration::from_millis(2));
        }
        Ok(())
    }

    /// key combo（如 "ctrl+shift+t" / "Return"）：修饰键按下 → 末键 down+up → 修饰键松开。
    /// 支持修饰 ctrl/shift/alt(option/opt)/win(super)；cmd/command/meta 按 Ctrl 处理（模块头）。
    pub(super) fn key_combo(combo: &str) -> Result<(), String> {
        let (mods, keycode) = parse_combo(combo)?;
        for m in &mods {
            key_down(*m)?;
        }
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        key_down(keycode)?;
        std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        key_up(keycode)?;
        for m in mods.iter().rev() {
            key_up(*m)?;
            std::thread::sleep(std::time::Duration::from_millis(INTER_EVENT_MS));
        }
        Ok(())
    }

    /// 长按键（hold_key）：key down → sleep duration → up。duration 上限 10s。
    pub(super) fn hold_key(combo: &str, duration_ms: u64) -> Result<(), String> {
        let (mods, keycode) = parse_combo(combo)?;
        for m in &mods {
            key_down(*m)?;
        }
        key_down(keycode)?;
        std::thread::sleep(std::time::Duration::from_millis(duration_ms.min(10000)));
        key_up(keycode)?;
        for m in mods.iter().rev() {
            key_up(*m)?;
        }
        Ok(())
    }

    /// 解析 combo → (修饰键序列, 末键 vk)。
    fn parse_combo(combo: &str) -> Result<(Vec<VIRTUAL_KEY>, VIRTUAL_KEY), String> {
        let mut mods: Vec<VIRTUAL_KEY> = Vec::new();
        let mut keycode: Option<VIRTUAL_KEY> = None;
        for part in combo.split('+').map(|p| p.trim().to_lowercase()) {
            if part.is_empty() {
                continue;
            }
            match part.as_str() {
                "ctrl" | "control" | "cmd" | "command" | "meta" => mods.push(VK_CONTROL),
                "shift" => mods.push(VK_SHIFT),
                "alt" | "option" | "opt" => mods.push(VK_MENU),
                "win" | "super" => mods.push(VK_LWIN),
                other => {
                    if keycode.is_some() {
                        return Err(format!("combo 只能一个末键: {combo}"));
                    }
                    keycode = Some(keycode_of(other).ok_or_else(|| format!("未知键: {other}"))?);
                }
            }
        }
        let kc = keycode.ok_or_else(|| format!("combo 缺末键: {combo}"))?;
        Ok((mods, kc))
    }

    /// 键名/字符 → Windows 虚拟键码（字母数字 ASCII 直映 + 特殊键 + 符号 + F 键）。
    fn keycode_of(name: &str) -> Option<VIRTUAL_KEY> {
        use windows::Win32::UI::Input::KeyboardAndMouse::{
            VK_F1, VK_F10, VK_F11, VK_F12, VK_F13, VK_F14, VK_F15, VK_F16, VK_F17, VK_F18, VK_F19,
            VK_F2, VK_F20, VK_F21, VK_F22, VK_F23, VK_F24, VK_F3, VK_F4, VK_F5, VK_F6, VK_F7,
            VK_F8, VK_F9, VK_DOWN, VK_INSERT, VK_LEFT, VK_RIGHT, VK_UP,
        };
        let code = match name {
            // 特殊键
            "return" | "enter" => VK_RETURN,
            "tab" => VK_TAB,
            "space" => VK_SPACE,
            "delete" | "backspace" => VK_BACK,
            "del" | "forwarddelete" | "fn_delete" => VK_DELETE,
            "insert" => VK_INSERT,
            "escape" | "esc" => VK_ESCAPE,
            "home" => VK_HOME,
            "end" => VK_END,
            "pageup" => VK_PRIOR,
            "pagedown" => VK_NEXT,
            "left" => VK_LEFT,
            "right" => VK_RIGHT,
            "down" => VK_DOWN,
            "up" => VK_UP,
            // F 键
            "f1" => VK_F1, "f2" => VK_F2, "f3" => VK_F3, "f4" => VK_F4,
            "f5" => VK_F5, "f6" => VK_F6, "f7" => VK_F7, "f8" => VK_F8,
            "f9" => VK_F9, "f10" => VK_F10, "f11" => VK_F11, "f12" => VK_F12,
            "f13" => VK_F13, "f14" => VK_F14, "f15" => VK_F15, "f16" => VK_F16,
            "f17" => VK_F17, "f18" => VK_F18, "f19" => VK_F19, "f20" => VK_F20,
            "f21" => VK_F21, "f22" => VK_F22, "f23" => VK_F23, "f24" => VK_F24,
            // 符号（美式键盘扫描集）
            "-" | "minus" => VIRTUAL_KEY(0xBD),
            "=" | "equal" => VIRTUAL_KEY(0xBB),
            "[" => VIRTUAL_KEY(0xDB),
            "]" => VIRTUAL_KEY(0xDD),
            "\\" => VIRTUAL_KEY(0xDC),
            ";" => VIRTUAL_KEY(0xBA),
            "'" => VIRTUAL_KEY(0xDE),
            "`" => VIRTUAL_KEY(0xC0),
            "," => VIRTUAL_KEY(0xBC),
            "." => VIRTUAL_KEY(0xBE),
            "/" => VIRTUAL_KEY(0xBF),
            // 单字符兜底（字母/数字 ASCII 直映）
            other if other.len() == 1 => {
                let c = other.chars().next().unwrap().to_ascii_uppercase();
                let v = c as u16;
                if (0x30..=0x39).contains(&v) || (0x41..=0x5A).contains(&v) {
                    VIRTUAL_KEY(v)
                } else {
                    return None;
                }
            }
            _ => return None,
        };
        Some(code)
    }
}

/// 非 Windows 目标（宿主 macOS 交叉 check）占位：无 SendInput。
#[cfg(not(target_os = "windows"))]
mod imp {
    fn unavailable() -> String {
        "computer.act 输入注入仅 Windows 目标可用".into()
    }
    pub(super) fn move_to(_x: f64, _y: f64) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn click(_button: &str, _x: f64, _y: f64) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn click_n(_button: &str, _x: f64, _y: f64, _n: u32) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn drag(_fx: f64, _fy: f64, _tx: f64, _ty: f64) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn mouse_down(_button: &str, _x: f64, _y: f64) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn mouse_up(_button: &str, _x: f64, _y: f64) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn scroll(_direction: &str, _amount: i32) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn type_text(_text: &str) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn key_combo(_combo: &str) -> Result<(), String> {
        Err(unavailable())
    }
    pub(super) fn hold_key(_combo: &str, _duration_ms: u64) -> Result<(), String> {
        Err(unavailable())
    }
}

// ===== 对外 API（两个目标同签名，透传 imp） =====

/// 移动鼠标到 (x, y)。
pub fn move_to(x: f64, y: f64) -> Result<(), String> {
    imp::move_to(x, y)
}

/// 点击（down + up）。button: "left" / "right" / "middle"。
pub fn click(button: &str, x: f64, y: f64) -> Result<(), String> {
    imp::click(button, x, y)
}

/// 连点 n 次（double_click=2 / triple_click=3）。
pub fn click_n(button: &str, x: f64, y: f64, n: u32) -> Result<(), String> {
    imp::click_n(button, x, y, n)
}

/// 拖拽：down @ from → 分步 move → up @ to。
pub fn drag(from_x: f64, from_y: f64, to_x: f64, to_y: f64) -> Result<(), String> {
    imp::drag(from_x, from_y, to_x, to_y)
}

/// 鼠标按下保持（computer.act 的 left_mouse_down，配合 release）。
pub fn mouse_down(button: &str, x: f64, y: f64) -> Result<(), String> {
    imp::mouse_down(button, x, y)
}

/// 释放鼠标（left_mouse_up）。
pub fn mouse_up(button: &str, x: f64, y: f64) -> Result<(), String> {
    imp::mouse_up(button, x, y)
}

/// 滚轮。direction: "up"/"down"/"left"/"right"，amount: 刻度（正=上/右）。
pub fn scroll(direction: &str, amount: i32) -> Result<(), String> {
    imp::scroll(direction, amount)
}

/// 输入文本（逐 UTF-16 码元 KEYEVENTF_UNICODE）。
pub fn type_text(text: &str) -> Result<(), String> {
    imp::type_text(text)
}

/// key combo（如 "ctrl+shift+t" / "Return"）。
pub fn key_combo(combo: &str) -> Result<(), String> {
    imp::key_combo(combo)
}

/// 长按键（hold_key）：key down → sleep duration → up。
pub fn hold_key(combo: &str, duration_ms: u64) -> Result<(), String> {
    imp::hold_key(combo, duration_ms)
}
