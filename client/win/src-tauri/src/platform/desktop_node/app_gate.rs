//! system.run 的 App 控制闸门（fail-closed）。
//!
//! 与 mac app_gate.rs 同构（Verdict / gate / judge / deny_payload / not_allowed 文案同形），
//! Windows 侧判定对象换成 Windows 的 App 拉起/终止载荷（mac 是 osascript 目标分析）：
//!
//! 判定规则（未知形态一律拒）：
//! - shell 包装识别后剥壳分析内层命令串（引号感知分词 + 元字符分析）：
//!   `sh|bash|zsh|dash|ksh -lc <串>`（openclaw exec 默认包装形态，Git-Bash 同款）+
//!   `cmd|cmd.exe /c <串>` + `powershell|pwsh -Command/-EncodedCommand <串>`。
//!   含管道/链式/替换/重定向等元字符时仅当内层无 App 触碰动词才放；嵌套包装 → 拒。
//! - `start`（cmd）拉起目标：旗标跳过 + 首个**带引号**位置参数按窗口标题跳过（cmd
//!   start 语义：第一个带引号参数是 title）→ 目标分类：URL/目录放行；`.lnk` 按
//!   快捷方式名匹配；`.exe`/裸名按 PATH+PATHEXT 解析后以 exe 名匹配清单；其余本地
//!   文件拒（默认关联不可静态归因，OPEN_UNATTRIBUTED）
//! - `Start-Process`/`start`（PowerShell）：-FilePath 值或首个位置参数 = 目标，同上分类
//! - `taskkill /IM <名>`：进程名须命中清单（杀进程也是触碰）；`/PID` 或缺 /IM → 拒
//!   （PID 不可静态归因，KILL_UNATTRIBUTED）
//! - `Stop-Process`（PowerShell）：-Name 值命中清单放行；-Id / 位置参数形态拒
//! - `SendKeys`（[Windows.Forms.SendKeys] / WScript.Shell.SendKeys）：输入注入打进
//!   前台 App（绕过 computer.act 的清单校验），任何位置出现即拒（UNTARGETED_INPUT）
//! - `wscript|cscript|mshta`：脚本宿主可驱动任意 App（产物绕过本闸门），任何位置
//!   出现即拒（AUTOMATION_TOOL；mac 侧同位规则是 osacompile/shortcuts/automator）
//! - computer.act 输入注入（见 act_gate）：注入前查前台窗口进程 exe 对照清单；
//!   前台是本 App 自身也拒（防自我点掉 exec 审批弹窗）；查询失败拒（fail-closed）
//!
//! 拒绝的返回形态：ok=true + success=false + stderr 带原因——网关对 ok=false 只读
//! 帧顶层 error，塞 payload 里会被塌成笼统 "node invoke failed" 吞掉原因（见
//! protocol.rs 注释），ok=true 才把 stderr 原样透传给 agent。
//!
//! 已知盲区（记录在案）：COM 自动化（powershell New-Object -ComObject WScript.Shell
//! 之外的对象）、python/pywin32、自编译工具无法逐一枚举，闸门看不见其载荷，唯一兜底
//! 是该设备 exec 审批复到 allowlist/ask 档（命令层白名单）；act 的前台校验存在毫秒级
//! 竞态（查完到注入的瞬间前台可能变化）。

use serde_json::Value;

use super::app_perm;

/// 闸门判定。
#[derive(Debug)]
pub enum Verdict {
    Allow,
    /// 拒绝（进 stderr 给 agent 看的原因，一段完整文案）。
    Deny(String),
}

/// shell 包装种类（决定内层串的解释细节：cmd 的 start 有 title 语义，PowerShell 有参数值旗标）。
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum ShellKind {
    None,
    Sh,
    Cmd,
    PowerShell,
}

/// system.run spawn 前的 App 控制闸门入口。
pub fn gate(argv: &[String]) -> Verdict {
    if argv.is_empty() {
        return Verdict::Allow; // 空命令由 system.run 上层报错，不归闸门管
    }
    let list = app_perm::allowed_snapshot();
    if let Some((cmd, kind)) = shell_wrapper_command(argv) {
        // openclaw exec / cmd / powershell 的包装形态：剥壳分析内层命令串（引号感知；
        // 反斜杠仅 POSIX shell 按转义，cmd/PowerShell 按字面量——见 shell_tokenize_kind）。
        let toks = shell_tokenize_kind(&cmd, kind == ShellKind::Sh);
        return judge(&toks, has_shell_metachar(&cmd), kind, &list);
    }
    // 非 shell 包装：argv 本身当 token 流（无 shell 语义、无引号、无元字符）走同一套扫描
    let toks: Vec<(String, bool)> = argv.iter().map(|t| (t.clone(), false)).collect();
    judge(&toks, false, ShellKind::None, &list)
}

/// token 流的统一判定（剥壳后的分词 / 非 shell 包装的 argv，测试直调）：
/// 脚本宿主 → start/Start-Process → taskkill/Stop-Process → SendKeys 兜底。皆无关 → 放。
fn judge(
    toks: &[(String, bool)],
    metachar: bool,
    kind: ShellKind,
    list: &[aiarsenal_client_core::types::AppPerm],
) -> Verdict {
    // wscript/cscript/mshta：脚本宿主，可驱动任意 App（产物绕过本闸门），任何位置出现即拒
    if let Some((t, _)) = toks.iter().find(|(t, _)| {
        matches!(basename(t).to_lowercase().as_str(), "wscript" | "cscript" | "mshta")
    }) {
        let tool = basename(t);
        return Verdict::Deny(deny(
            "AUTOMATION_TOOL",
            &format!("命令调用 {tool}（Windows 脚本宿主，可驱动任意 App 绕过闸门），拒绝执行"),
        ));
    }
    // start / Start-Process：拉起 App 是触碰，逐个校验目标
    if let Some(v) = check_start(toks, kind, list) {
        return v;
    }
    // taskkill / Stop-Process：目标必须命中清单内 App 名
    if let Some(v) = check_kill(toks, list) {
        return v;
    }
    // SendKeys 兜底（WScript.Shell / Windows.Forms.SendKeys）：输入打进前台 App，拒
    if toks.iter().any(|(t, _)| has_sendkeys_word(t)) {
        return Verdict::Deny(deny(
            "UNTARGETED_INPUT",
            "命令含 SendKeys（输入注入打进前台 App，绕过清单）。请改用 computer.act（前台 App 需在清单内），或在管理端「App 权限」勾选目标 App 后用显式 App 自动化",
        ));
    }
    let _ = metachar; // 与 App 触碰动词无关的混排命令（管道等）放行，见 check_start 内部分支
    Verdict::Allow
}

/// start / Start-Process 校验：扫 token 流里的每个拉起动词，解析旗标与位置参数。
/// Some(判定)=须拦/放行中断；None=无拉起动词或全部通过。
fn check_start(
    toks: &[(String, bool)],
    kind: ShellKind,
    list: &[aiarsenal_client_core::types::AppPerm],
) -> Option<Verdict> {
    let mut i = 0;
    while i < toks.len() {
        let name = basename(&toks[i].0).to_lowercase();
        let is_cmd_start = name == "start" && matches!(kind, ShellKind::Cmd | ShellKind::None);
        let is_ps_start = name == "start-process" || (name == "start" && kind == ShellKind::PowerShell);
        if !is_cmd_start && !is_ps_start {
            i += 1;
            continue;
        }
        let mut j = i + 1;
        let mut target: Option<String> = None;
        while j < toks.len() {
            let (t, quoted) = (&toks[j].0, toks[j].1);
            if is_ps_start && t.starts_with('-') && t.len() > 1 {
                // PowerShell 参数：-FilePath/-ArgumentList/-Verb 等带值旗标取下一个当目标候选
                let pname = t[1..].split(':').next().unwrap_or("").to_lowercase();
                if matches!(
                    pname.as_str(),
                    "filepath" | "argumentlist" | "verb" | "windowstyle" | "workingdirectory" | "credential"
                ) {
                    if pname == "filepath" && target.is_none() {
                        target = toks.get(j + 1).map(|(v, _)| v.clone());
                    }
                    j += 2;
                    continue;
                }
                j += 1; // 开关型旗标（-NoNewWindow 等）
                continue;
            }
            if is_cmd_start && t.starts_with('/') && t.len() > 1 {
                // cmd start 旗标：/d /node /affinity 带值，其余单 token（/min /wait …）
                let f = t[1..].split(':').next().unwrap_or("").to_lowercase();
                if matches!(f.as_str(), "d" | "node" | "affinity") {
                    j += 2;
                } else {
                    j += 1;
                }
                continue;
            }
            if is_cmd_start && target.is_none() && quoted {
                // cmd start 语义：首个带引号的位置参数是窗口标题，跳过
                j += 1;
                continue;
            }
            if target.is_none() {
                target = Some(t.clone());
                j += 1;
                continue;
            }
            break; // 目标后的参数（App 自己的实参）不影响归因
        }
        if let Some(t) = target {
            let verdict = judge_start_target(&t, list);
            if let Some(v) = verdict {
                return Some(v);
            }
        }
        i = j.max(i + 1);
    }
    None
}

/// 拉起目标分类：URL/目录放行；.lnk 按快捷方式名；.exe/裸名按 PATH 解析 exe 名；
/// 其余本地文件拒（默认关联不可归因）。None=放行，Some=终局判定。
fn judge_start_target(
    raw: &str,
    list: &[aiarsenal_client_core::types::AppPerm],
) -> Option<Verdict> {
    let t = raw.trim();
    if t.is_empty() {
        return None;
    }
    if looks_like_url(t) || is_dir(t) {
        return None;
    }
    let low = t.to_lowercase();
    if low.ends_with(".lnk") {
        // .lnk 落点才是真目标，但快捷方式名通常=App 名；不可静态解引用（要 COM IShellLink），
        // 按名匹配——名字不在清单 → 拒（fail-closed，agent 可改用直接 .exe）
        let stem = basename(t).trim_end_matches(&low[low.len() - 4..]).to_string();
        return if match_app_target(&stem, list) {
            None
        } else {
            Some(Verdict::Deny(not_allowed(&stem, list)))
        };
    }
    if low.ends_with(".exe") {
        let exe = basename(t);
        return if match_app_target(exe, list) {
            None
        } else {
            Some(Verdict::Deny(not_allowed(exe, list)))
        };
    }
    if !low.contains('.') && !has_path_selector(t) {
        // 裸名：PATH+PATHEXT 解析成可执行文件再按 exe 名匹配（复用 exec_approvals 解析）
        let res = super::exec_approvals::resolve_executable(t, None, None);
        if let Some(p) = &res.resolved_path {
            let exe = basename(p);
            return if match_app_target(exe, list) {
                None
            } else {
                Some(Verdict::Deny(not_allowed(exe, list)))
            };
        }
        return Some(Verdict::Deny(not_allowed(t, list)));
    }
    if has_path_selector(t) && !low.contains('.') {
        // 无扩展名的路径：按最后一段当 exe 名试配（POSIX 风格二进制路径）
        let exe = basename(t);
        return if match_app_target(exe, list) {
            None
        } else {
            Some(Verdict::Deny(not_allowed(exe, list)))
        };
    }
    Some(Verdict::Deny(deny(
        "OPEN_UNATTRIBUTED",
        &format!("start 打开本地文件 {t} 的目标 App 由系统默认关联决定，闸门无法归因（fail-closed）。请用 Start-Process 清单内的 App 可执行文件显式拉起"),
    )))
}

fn has_path_selector(v: &str) -> bool {
    v.contains('/') || v.contains('\\') || v.contains('~')
}

/// taskkill / Stop-Process 校验：目标必须命中清单内 App 名（杀进程也是触碰）。
fn check_kill(
    toks: &[(String, bool)],
    list: &[aiarsenal_client_core::types::AppPerm],
) -> Option<Verdict> {
    for (i, (t, _)) in toks.iter().enumerate() {
        match basename(t).to_lowercase().as_str() {
            "taskkill" => {
                // /IM <名> 精确匹配放行；/PID 或缺 /IM 拒（PID 不可静态归因）
                let mut im: Option<String> = None;
                let mut has_pid = false;
                let mut j = i + 1;
                while j < toks.len() {
                    let a = toks[j].0.to_lowercase();
                    if a == "/im" || a == "-im" {
                        im = toks.get(j + 1).map(|(v, _)| v.clone());
                        j += 2;
                        continue;
                    }
                    if a == "/pid" || a == "-pid" {
                        has_pid = true;
                    }
                    j += 1;
                }
                if has_pid {
                    return Some(Verdict::Deny(deny(
                        "KILL_UNATTRIBUTED",
                        "taskkill /PID 按进程号终止（闸门无法归因到 App），拒绝执行。请用 taskkill /IM <进程名> 并确保该 App 在清单内",
                    )));
                }
                match im {
                    Some(n) if match_app_target(&n, list) => {}
                    Some(n) => return Some(Verdict::Deny(not_allowed(&n, list))),
                    None => {
                        return Some(Verdict::Deny(deny(
                            "KILL_UNATTRIBUTED",
                            "taskkill 缺 /IM 进程名，拒绝执行",
                        )))
                    }
                }
            }
            "stop-process" => {
                // -Name 值命中清单放行；-Id / 位置参数形态拒（同 taskkill /PID 的归因缺口）
                let mut name: Option<String> = None;
                let mut unattributed = false;
                let mut j = i + 1;
                while j < toks.len() {
                    let (a, _) = &toks[j];
                    let low = a.to_lowercase();
                    if low == "-name" || low.starts_with("-name:") {
                        if low.contains(':') {
                            name = Some(a.splitn(2, ':').nth(1).unwrap_or("").to_string());
                            j += 1;
                        } else {
                            name = toks.get(j + 1).map(|(v, _)| v.clone());
                            j += 2;
                        }
                        continue;
                    }
                    if low == "-id" || low.starts_with("-id:") {
                        unattributed = true;
                        break;
                    }
                    if !low.starts_with('-') && name.is_none() {
                        unattributed = true; // 位置参数形态（按管道/位置传 Id 或 Name，不可归因）
                        break;
                    }
                    j += 1;
                }
                if unattributed {
                    return Some(Verdict::Deny(deny(
                        "KILL_UNATTRIBUTED",
                        "Stop-Process 用 -Id/位置参数终止进程（闸门无法归因到 App），拒绝执行。请用 Stop-Process -Name <进程名> 并确保该 App 在清单内",
                    )));
                }
                match name {
                    Some(n) if match_app_target(&n, list) => {}
                    Some(n) => return Some(Verdict::Deny(not_allowed(&n, list))),
                    None => {
                        return Some(Verdict::Deny(deny(
                            "KILL_UNATTRIBUTED",
                            "Stop-Process 缺 -Name，拒绝执行",
                        )))
                    }
                }
            }
            _ => {}
        }
    }
    None
}

/// 目标（exe 名/App 名/bundle id）对照清单：名字直配（含 .exe 归一）。
fn match_app_target(target: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> bool {
    match_list(target, list)
}

/// 目标匹配清单：bundle_id 全等 / app_name / bundle 末段，忽略大小写；
/// Windows 追加 `.exe` 归一（exe 名 "notepad" ↔ bundle_id "notepad.exe" 互配）。
fn match_list(target: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> bool {
    let t = target.trim().to_lowercase();
    let t_exe = t.trim_end_matches(".exe").to_string();
    list.iter().any(|a| {
        let bundle = a.bundle_id.trim().to_lowercase();
        if bundle.is_empty() {
            return false;
        }
        let bundle_exe = bundle.trim_end_matches(".exe").to_string();
        let last = bundle.rsplit(['.', '/']).next().unwrap_or("").to_string();
        let last_exe = last.trim_end_matches(".exe").to_string();
        let app_name = a.app_name.trim().to_lowercase();
        let app_name_exe = app_name.trim_end_matches(".exe").to_string();
        t == bundle
            || t == app_name
            || t == last
            || t_exe == bundle_exe
            || t_exe == app_name_exe
            || t_exe == last_exe
    })
}

/// computer.act 输入注入闸门：注入前查前台窗口进程 exe 对照清单。
/// fail-closed：查询失败/前台未知 → 拒；前台是本 App 自身 → 拒（防自我点掉
/// exec 审批弹窗）。调用点见 protocol.rs exec_act 的 INJECTIVE_ACTIONS。
/// （Win32 段按 target cfg：windows crate 只进 Windows 目标——其传递依赖
/// windows-future 在非 Windows 宿主编不过；非 Windows 目标恒 fail-closed 拒绝。）
pub fn act_gate() -> Result<(), String> {
    #[cfg(target_os = "windows")]
    return act_gate_win();
    #[cfg(not(target_os = "windows"))]
    Err(deny(
        "ACT_FRONTMOST_UNKNOWN",
        "computer.act 输入注入仅 Windows 目标可用，fail-closed 拒绝输入注入",
    ))
}

/// Win32 前台进程查询实现（仅 Windows 目标编译）。
#[cfg(target_os = "windows")]
fn act_gate_win() -> Result<(), String> {
    use windows::Win32::Foundation::{CloseHandle, HANDLE};
    use windows::Win32::System::Threading::{
        OpenProcess, PROCESS_QUERY_LIMITED_INFORMATION,
    };
    use windows::Win32::UI::WindowsAndMessaging::{GetForegroundWindow, GetWindowThreadProcessId};

    unsafe {
        let hwnd = GetForegroundWindow();
        if hwnd.0.is_null() {
            return Err(deny(
                "ACT_FRONTMOST_UNKNOWN",
                "无法确定前台 App（无前台窗口），fail-closed 拒绝输入注入",
            ));
        }
        let mut pid: u32 = 0;
        GetWindowThreadProcessId(hwnd, Some(&mut pid));
        if pid == 0 {
            return Err(deny(
                "ACT_FRONTMOST_UNKNOWN",
                "无法确定前台 App（取进程号失败），fail-closed 拒绝输入注入",
            ));
        }
        let handle: HANDLE = match OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid) {
            Ok(h) => h,
            Err(_) => {
                return Err(deny(
                    "ACT_FRONTMOST_UNKNOWN",
                    "无法确定前台 App（OpenProcess 失败，前台可能以管理员运行），fail-closed 拒绝输入注入",
                ))
            }
        };
        let exe = query_process_image(handle);
        let _ = CloseHandle(handle);
        let Some(exe) = exe else {
            return Err(deny(
                "ACT_FRONTMOST_UNKNOWN",
                "无法确定前台 App（QueryFullProcessImageName 失败），fail-closed 拒绝输入注入",
            ));
        };
        if exe.is_empty() {
            return Err(deny(
                "ACT_FRONTMOST_UNKNOWN",
                "无法确定前台 App（查询结果为空），fail-closed 拒绝输入注入",
            ));
        }
        // 前台是本 App 自身 → 拒：防 agent 用 act 点掉/操纵自己的 exec 审批弹窗
        if is_self_exe(&exe) {
            return Err(deny(
                "ACT_SELF",
                "前台是本 App 自身（exec 审批弹窗所在处），拒绝注入输入——防自我操纵审批",
            ));
        }
        let list = app_perm::allowed_snapshot();
        let exe_name = basename(&exe);
        if list.iter().any(|a| {
            a.bundle_id.trim().eq_ignore_ascii_case(exe_name)
                || a.bundle_id
                    .trim()
                    .trim_end_matches(".exe")
                    .eq_ignore_ascii_case(exe_name.trim_end_matches(".exe"))
                || a.app_name.trim().eq_ignore_ascii_case(exe_name)
        }) {
            return Ok(());
        }
        Err(not_allowed(exe_name, &list))
    }
}

/// QueryFullProcessImageNameW → exe 全路径（仅 Windows 目标编译）。
#[cfg(target_os = "windows")]
fn query_process_image(handle: windows::Win32::Foundation::HANDLE) -> Option<String> {
    use windows::Win32::System::Threading::{QueryFullProcessImageNameW, PROCESS_NAME_WIN32};
    use windows::core::PWSTR;
    unsafe {
        let mut buf = [0u16; 1024];
        let mut len = buf.len() as u32;
        let ok = QueryFullProcessImageNameW(
            handle,
            PROCESS_NAME_WIN32,
            PWSTR::from_raw(buf.as_mut_ptr()),
            &mut len,
        );
        if ok.is_err() || len == 0 {
            return None;
        }
        Some(String::from_utf16_lossy(&buf[..len as usize]))
    }
}

/// 本 App 自身的 exe 名（act 防自击用；读不到返回空串=只做清单校验）。
/// 仅 Windows 目标编译（唯一调用点在 act_gate_win；宿主 check 免 dead_code 告警）。
#[cfg(target_os = "windows")]
fn is_self_exe(foreground_exe: &str) -> bool {
    let fg = basename(foreground_exe).to_lowercase();
    std::env::current_exe()
        .ok()
        .map(|p| basename(&p.to_string_lossy()).to_lowercase())
        .map(|self_name| !self_name.is_empty() && self_name == fg)
        .unwrap_or(false)
}

/// 识别 shell 包装 argv：(内层命令串, 种类)。命令串后还有参数不认——回落外层词扫描
///（fail-closed）。
fn shell_wrapper_command(argv: &[String]) -> Option<(String, ShellKind)> {
    let sh = basename(&argv[0]).to_lowercase();
    // 1) POSIX shell（openclaw exec 默认包装；Git-Bash / WSL 同款）
    if matches!(sh.as_str(), "sh" | "bash" | "zsh" | "dash" | "ksh" | "sh.exe" | "bash.exe") {
        let mut i = 1;
        let mut saw_c = false;
        while i < argv.len() {
            let a = argv[i].as_str();
            if a.starts_with("--") || !a.starts_with('-') {
                break;
            }
            let body = &a[1..];
            if body.is_empty()
                || !body.chars().all(|c| c == 'c' || c == 'l' || c == 'i')
                || body.matches('c').count() > 1
            {
                return None;
            }
            i += 1;
            if body.contains('c') {
                saw_c = true;
                break;
            }
        }
        if saw_c {
            if let Some(cmd) = argv.get(i) {
                if argv.len() == i + 1 {
                    return Some((cmd.clone(), ShellKind::Sh));
                }
            }
        }
        return None;
    }
    // 2) cmd（/c 或 /k 后是命令串，允许穿插 /d /s /q 等前置旗标）
    if sh == "cmd" || sh == "cmd.exe" {
        let mut i = 1;
        while i < argv.len() {
            let a = argv[i].as_str();
            let low = a.to_lowercase();
            if low == "/c" || low == "/k" || low == "-c" || low == "-k" {
                let rest: Vec<String> = argv[i + 1..].to_vec();
                if rest.is_empty() {
                    return None;
                }
                return Some((rest.join(" "), ShellKind::Cmd));
            }
            // 前置旗标（/d /s /q /a /u /x /y /e:on|off …）跳过；遇到位置参数就放弃
            if a.starts_with('/') && a.len() > 1 && !a.contains(' ') {
                i += 1;
                continue;
            }
            return None;
        }
        return None;
    }
    // 3) PowerShell（-Command/-c <串...> 或 -EncodedCommand/-ec <b64>）
    if sh == "powershell" || sh == "powershell.exe" || sh == "pwsh" || sh == "pwsh.exe" {
        let mut i = 1;
        let mut encoded: Option<String> = None;
        let mut cmd_parts: Vec<String> = Vec::new();
        let mut saw_command = false;
        while i < argv.len() {
            let a = argv[i].as_str();
            let low = a.to_lowercase();
            // 仅认无歧义写法（-e 是 -ExecutionPolicy 的缩写，不收）
            if low == "-command" || low == "-c" {
                saw_command = true;
                cmd_parts.extend(argv[i + 1..].iter().cloned());
                break;
            }
            if low == "-encodedcommand" || low == "-ec" {
                encoded = argv.get(i + 1).cloned();
                break;
            }
            // 无值开关（-NoProfile -NonInteractive -NoLogo -ExecutionPolicy Bypass 组合里
            // ExecutionPolicy 带值，一并处理）跳过
            if low == "-executionpolicy" {
                i += 2;
                continue;
            }
            if low.starts_with('-') && low.len() > 1 {
                i += 1;
                continue;
            }
            return None; // 位置参数先于 -Command：非标准包装形态
        }
        if saw_command && !cmd_parts.is_empty() {
            return Some((cmd_parts.join(" "), ShellKind::PowerShell));
        }
        if let Some(b64) = encoded {
            if let Some(script) = decode_ps_encoded_command(&b64) {
                return Some((script, ShellKind::PowerShell));
            }
            return None; // b64 解不出来 → 回落外层词扫描（fail-closed）
        }
        return None;
    }
    None
}

/// -EncodedCommand 的 base64(UTF-16LE) 解码。
fn decode_ps_encoded_command(b64: &str) -> Option<String> {
    use base64::Engine as _;
    let raw = base64::engine::general_purpose::STANDARD.decode(b64.trim()).ok()?;
    let u16s: Vec<u16> = raw.chunks_exact(2).map(|c| u16::from_le_bytes([c[0], c[1]])).collect();
    Some(String::from_utf16_lossy(&u16s))
}

/// shell 引号感知分词（近似）：单引号内全字面；双引号内转义仅 POSIX（\" \\ \$ \`）；
/// 裸 `\x` 仅 POSIX 转义下一字符——cmd/PowerShell 的反斜杠是字面量（路径分隔符；
/// PowerShell 的转义符是反引号），若按转义处理 `C:\tmp\x.txt` 会被吃成 `C:tmpx.txt`，
/// 再被 looks_like_url 当 scheme 放行（闸门绕过）。
/// 产出 (token, 是否带引号)——cmd `start` 的 title 判别依赖引号位。
fn shell_tokenize_kind(s: &str, posix_escape: bool) -> Vec<(String, bool)> {
    let mut tokens: Vec<(String, bool)> = Vec::new();
    let mut cur = String::new();
    let mut have = false;
    let mut quoted = false;
    let mut in_single = false;
    let mut in_double = false;
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if in_single {
            if c == '\'' {
                in_single = false;
            } else {
                cur.push(c);
            }
        } else if in_double {
            match c {
                '"' => in_double = false,
                '\\' if posix_escape => {
                    if let Some(&n) = chars.peek() {
                        if matches!(n, '"' | '\\' | '$' | '`') {
                            cur.push(n);
                            chars.next();
                        } else {
                            cur.push('\\');
                        }
                    } else {
                        cur.push('\\');
                    }
                }
                _ => cur.push(c),
            }
        } else {
            match c {
                '\'' => {
                    in_single = true;
                    have = true;
                    quoted = true;
                }
                '"' => {
                    in_double = true;
                    have = true;
                    quoted = true;
                }
                '\\' if posix_escape => {
                    if let Some(n) = chars.next() {
                        cur.push(n);
                    }
                    have = true;
                }
                c if c.is_whitespace() => {
                    if have {
                        tokens.push((std::mem::take(&mut cur), quoted));
                        have = false;
                        quoted = false;
                    }
                }
                c => {
                    cur.push(c);
                    have = true;
                }
            }
        }
    }
    if have {
        tokens.push((cur, quoted));
    }
    tokens
}

/// 命令串是否含引号外的 shell 元字符（; | & 换行 <> ( ) ` $( )。保留该分析供未来
/// 分支细分（当前 judge 对无关混排命令放行），与 mac 版语义对齐。
fn has_shell_metachar(s: &str) -> bool {
    let mut in_single = false;
    let mut in_double = false;
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if in_single {
            if c == '\'' {
                in_single = false;
            }
            continue;
        }
        if in_double {
            match c {
                '"' => in_double = false,
                '\\' => {
                    chars.next();
                }
                '`' => return true,
                '$' => {
                    if chars.peek() == Some(&'(') {
                        return true;
                    }
                }
                _ => {}
            }
            continue;
        }
        match c {
            '\'' => in_single = true,
            '"' => in_double = true,
            '\\' => {
                chars.next();
            }
            ';' | '|' | '&' | '`' | '(' | ')' | '<' | '>' | '\n' => return true,
            '$' => {
                if chars.peek() == Some(&'(') {
                    return true;
                }
            }
            _ => {}
        }
    }
    false
}

/// URL 判定：带 scheme（http://、mailto:、ms-edge: …）——含 "://" 或「冒号前无 /」。
fn looks_like_url(s: &str) -> bool {
    if s.contains("://") {
        return true;
    }
    match s.find(':') {
        Some(c) => {
            // Windows 盘符路径（C:\… / C:/…）：冒号前单字母 + 冒号后路径分隔 → 不是 URL
            //（否则 start C:\tmp\x.txt 会被当 scheme "c" 的 URL 无条件放行——闸门绕过）
            if c == 1
                && s.len() > 2
                && (s.as_bytes()[2] == b'\\' || s.as_bytes()[2] == b'/')
            {
                return false;
            }
            !s[..c].contains('/') && !s[..c].contains('\\')
        }
        None => false,
    }
}

/// 路径是否目录（start 目录 → 资源管理器，无害放行）。
fn is_dir(p: &str) -> bool {
    std::fs::metadata(p).map(|m| m.is_dir()).unwrap_or(false)
}

// ===== 文案 =====

/// 拒绝文案（结构类：脚本宿主/不可归因/无目标输入）。统一 APP_GATE_DENIED 前缀，
/// agent 可区分于 APP_NOT_ALLOWED（清单未勾选）。
fn deny(code: &str, detail: &str) -> String {
    format!("APP_GATE_DENIED({code}): {detail}")
}

/// 清单未命中文案：带目标名 + 当前清单提示 + 恢复路径（勾选即生效）。
fn not_allowed(target: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> String {
    let names: Vec<&str> = list
        .iter()
        .map(|a| {
            if a.app_name.is_empty() {
                a.bundle_id.as_str()
            } else {
                a.app_name.as_str()
            }
        })
        .take(5)
        .collect();
    let list_hint = if list.is_empty() {
        "（当前清单为空=全部 App 禁控）".to_string()
    } else {
        format!("（当前清单：{}{}）", names.join("、"), if list.len() > 5 { " 等" } else { "" })
    };
    format!(
        "APP_NOT_ALLOWED: {target} 不在 App 控制清单{list_hint}。如需允许，请在管理端「App 权限」勾选该 App 后重试（勾选后放行即刻生效）"
    )
}

// ===== 小工具 =====

fn basename(p: &str) -> &str {
    p.rsplit(['/', '\\']).next().unwrap_or(p)
}

/// 独立词 SendKeys（大小写不敏感，[Windows.Forms]::SendKeys / WScript.Shell.SendKeys 都命中）。
fn has_sendkeys_word(s: &str) -> bool {
    regex::Regex::new(r"(?i)\bsendkeys\b")
        .unwrap()
        .is_match(s)
}

/// system.run 接线用的便捷判定：Deny → ok=true + success=false + stderr（网关
/// ok=false 会吞原因；见模块注释）。
pub fn deny_payload(reason: String) -> Value {
    serde_json::json!({
        "exitCode": null, // 未执行（spawn 前拦截），与「启动失败」同形
        "timedOut": false,
        "success": false,
        "stdout": "",
        "stderr": reason,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use aiarsenal_client_core::types::AppPerm;

    fn list(entries: &[(&str, &str)]) -> Vec<AppPerm> {
        entries
            .iter()
            .map(|(b, n)| AppPerm { bundle_id: b.to_string(), app_name: n.to_string() })
            .collect()
    }

    fn denied(v: Verdict) -> String {
        match v {
            Verdict::Deny(r) => r,
            Verdict::Allow => panic!("期望 Deny，得到 Allow"),
        }
    }

    /// POSIX 口径分词（sh -c 内层串；真实管线 gate() 按 ShellKind 选）。
    fn toks(s: &str) -> Vec<(String, bool)> {
        shell_tokenize_kind(s, true)
    }

    /// cmd / PowerShell 内层串口径（反斜杠=字面量路径分隔；真实管线 gate() 按 ShellKind 选）。
    fn toks_win(s: &str) -> Vec<(String, bool)> {
        shell_tokenize_kind(s, false)
    }

    #[test]
    fn cmd_start_gates_by_exe_name() {
        let l = list(&[("notepad.exe", "Notepad")]);
        // 清单内裸名：PATH+PATHEXT 解析是 Windows 语义——真 Windows 上解析到 notepad.exe
        // 放行；宿主 macOS check 解析不到 → fail-closed 拒（两个环境都确定性断言）。
        if cfg!(target_os = "windows") {
            assert!(matches!(
                judge(&toks_win("start notepad"), false, ShellKind::Cmd, &l),
                Verdict::Allow
            ));
            // 清单内带引号 title 形态（"" 是 title，notepad 是目标）
            assert!(matches!(
                judge(&toks_win(r#"start "" notepad"#), false, ShellKind::Cmd, &l),
                Verdict::Allow
            ));
        } else {
            let r = denied(judge(&toks_win("start notepad"), false, ShellKind::Cmd, &l));
            assert!(r.starts_with("APP_NOT_ALLOWED: notepad"), "{r}");
            let r = denied(judge(&toks_win(r#"start "" notepad"#), false, ShellKind::Cmd, &l));
            assert!(r.starts_with("APP_NOT_ALLOWED: notepad"), "{r}");
        }
        // 清单外 .exe 直书 → APP_NOT_ALLOWED（反斜杠路径按字面量分词，两平台一致）
        let r = denied(judge(&toks_win(r#"start "" C:\Windows\System32\calc.exe"#), false, ShellKind::Cmd, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: calc.exe"), "{r}");
        // URL 放行
        assert!(matches!(
            judge(&toks_win("start https://example.com"), false, ShellKind::Cmd, &l),
            Verdict::Allow
        ));
        // 目录放行：目标目录须真实存在（is_dir）——按宿主可用目录断言
        //（真 Windows: C:\Users；宿主 check: /tmp）
        if cfg!(target_os = "windows") {
            assert!(matches!(
                judge(&toks_win(r#"start "" C:\Users"#), false, ShellKind::Cmd, &l),
                Verdict::Allow
            ));
        } else {
            assert!(matches!(
                judge(&toks_win("start /tmp"), false, ShellKind::Cmd, &l),
                Verdict::Allow
            ));
        }
        // 本地文件（默认关联不可归因）→ OPEN_UNATTRIBUTED（盘符路径不得被当 URL 放行）
        let r = denied(judge(&toks_win(r#"start "" C:\tmp\x.txt"#), false, ShellKind::Cmd, &l));
        assert!(r.contains("OPEN_UNATTRIBUTED"), "{r}");
    }

    #[test]
    fn powershell_start_process_gated() {
        let l = list(&[("notepad.exe", "Notepad")]);
        // 裸名目标：真 Windows 放行 / 宿主 macOS 解析不到 fail-closed 拒（同 cmd_start_gates）。
        if cfg!(target_os = "windows") {
            assert!(matches!(
                judge(&toks_win("Start-Process notepad"), false, ShellKind::PowerShell, &l),
                Verdict::Allow
            ));
            assert!(matches!(
                judge(&toks_win("Start-Process -FilePath notepad -WindowStyle Hidden"), false, ShellKind::PowerShell, &l),
                Verdict::Allow
            ));
        } else {
            let r = denied(judge(&toks_win("Start-Process notepad"), false, ShellKind::PowerShell, &l));
            assert!(r.starts_with("APP_NOT_ALLOWED: notepad"), "{r}");
        }
        let r = denied(judge(
            &toks_win("Start-Process -FilePath 'C:\\Windows\\System32\\calc.exe'"),
            false,
            ShellKind::PowerShell,
            &l,
        ));
        assert!(r.starts_with("APP_NOT_ALLOWED: calc.exe"), "{r}");
    }

    #[test]
    fn taskkill_and_stop_process_gated() {
        let l = list(&[("notepad.exe", "Notepad")]);
        assert!(matches!(
            judge(&toks_win("taskkill /IM notepad.exe /F"), false, ShellKind::Cmd, &l),
            Verdict::Allow
        ));
        let r = denied(judge(&toks_win("taskkill /IM chrome.exe"), false, ShellKind::Cmd, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: chrome.exe"), "{r}");
        let r = denied(judge(&toks_win("taskkill /PID 1234 /F"), false, ShellKind::Cmd, &l));
        assert!(r.contains("KILL_UNATTRIBUTED"), "{r}");
        assert!(matches!(
            judge(&toks_win("Stop-Process -Name notepad"), false, ShellKind::PowerShell, &l),
            Verdict::Allow
        ));
        let r = denied(judge(&toks_win("Stop-Process -Id 42"), false, ShellKind::PowerShell, &l));
        assert!(r.contains("KILL_UNATTRIBUTED"), "{r}");
    }

    #[test]
    fn sendkeys_and_script_hosts_denied() {
        let l = list(&[("notepad.exe", "Notepad")]);
        let r = denied(judge(
            &toks(r#"powershell -Command "[System.Windows.Forms.SendKeys]::SendWait('x')""#),
            true,
            ShellKind::Sh,
            &l,
        ));
        assert!(r.contains("UNTARGETED_INPUT"), "{r}");
        for cmd in ["wscript x.vbs", "cscript //nologo a.vbs", "mshta evil.hta"] {
            let r = denied(judge(&toks(cmd), false, ShellKind::Cmd, &l));
            assert!(r.contains("AUTOMATION_TOOL"), "{cmd}: {r}");
        }
    }

    #[test]
    fn wrapper_kinds_detected() {
        let v = |a: &[&str]| -> Vec<String> { a.iter().map(|s| s.to_string()).collect() };
        assert_eq!(
            shell_wrapper_command(&v(&["/bin/sh", "-lc", "echo hi"])),
            Some(("echo hi".into(), ShellKind::Sh))
        );
        assert_eq!(
            shell_wrapper_command(&v(&["cmd", "/c", "start", "notepad"])),
            Some(("start notepad".into(), ShellKind::Cmd))
        );
        assert_eq!(
            shell_wrapper_command(&v(&["cmd.exe", "/d", "/s", "/c", "dir"])),
            Some(("dir".into(), ShellKind::Cmd))
        );
        assert_eq!(
            shell_wrapper_command(&v(&["powershell", "-NoProfile", "-Command", "Get-Process"])),
            Some(("Get-Process".into(), ShellKind::PowerShell))
        );
        assert_eq!(
            shell_wrapper_command(&v(&["pwsh", "-ec", "eAA="])),
            Some(("x".into(), ShellKind::PowerShell)) // base64(UTF-16LE "x"=78 00)
        );
        assert_eq!(shell_wrapper_command(&v(&["cmd", "/c", "a", "b"])), Some(("a b".into(), ShellKind::Cmd)));
        assert_eq!(shell_wrapper_command(&v(&["python3", "-c", "x"])), None);
    }

    #[test]
    fn gate_passes_through_plain_commands() {
        // 非 shell 包装的普通命令直放
        let argv: Vec<String> = ["C:\\Windows\\System32\\ipconfig.exe", "/all"]
            .iter().map(|s| s.to_string()).collect();
        assert!(matches!(gate(&argv), Verdict::Allow));
    }
}
