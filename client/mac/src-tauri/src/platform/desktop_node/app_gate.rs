//! system.run 的 App 控制闸门（fail-closed）。
//!
//! 背景：「App 权限」清单原本只做预授权（app_perm.rs——提前弹 TCC 框防任务中途
//! 挂起），但移出清单不撤销已授予的 TCC 对（macOS 无按目标撤销 API），管理端
//! 取消勾选后 agent 仍能控制该 App。本闸门补上执行边界：system.run spawn 前解析
//! osascript 载荷的 AppleScript 目标，对清单校验，不在清单即拒——「取消勾选=
//! 立即禁控」。清单快照取自 app_perm 引擎当前态（服务端心跳下发的权威清单；
//! 空清单=只放行 System Events 查询，其余全拒）。
//!
//! 判定规则（未知形态一律拒）：
//! - openclaw exec 工具默认把命令包成 `["/bin/sh","-lc","<command>"]`（见 openclaw
//!   src/infra/node-shell.ts buildNodeShellCommand）——闸门识别 shell 包装形态，
//!   对内层命令串做引号感知的分词/元字符分析：内层是单条 osascript（可带 env 前缀）
//!   就剥壳继续走目标分析；含管道/链式/替换/重定向等元字符或嵌套包装 → 拒（无法
//!   归因目标）；osascript 出现在非首位（grep/echo 提及该词的误伤按 fail-closed
//!   接受，agent 换写法即可）；完全无关命令（无 osascript）直放
//! - 其余非 shell 包装的 argv 中任一参数含独立词 `osascript` → 拒（env osascript
//!   等混淆调用）
//! - osascript 载荷 = 全部 `-e` 片段拼接 + 位置参数脚本文件（>512KB 或非 UTF-8
//!   拒——编译版 .scpt 无法静态解析）；`-l` 声明非 AppleScript 语言（JXA）拒
//! - 脚本含 `do shell script` → 拒（脚本内再起 shell，内层目标不可见）
//! - `tell application|app (id)? "X"`：X 匹配清单（bundle_id 全等 / app_name /
//!   bundle 末段，忽略大小写）才放行；System Events（com.apple.systemevents）
//!   作为目标常放（窗口枚举等无害查询），但其 UI scripting 受下面两条约束
//! - `tell (application) process "P"`（System Events UI scripting）：P 必须匹配清单
//! - `keystroke`/`key code`/`click at` 且无 process 目标 → 拒（输入打进前台 App，
//!   等于绕过清单的 computer.act）
//! - 有 `tell` 字样但提取不出目标 / 无载荷（stdin）→ 拒
//! - `open` 命令：`-a/-b` 目标与裸 `.app` 路径对照清单（拉起+喂文件都是触碰——
//!   编辑器吃文件＝内容投递，实测文本编辑两度中招）；`-t/-f`（强制文本编辑器）按
//!   TextEdit 校验；裸位置参数：URL（含 scheme）放行（浏览器域自管）、目录（只能开
//!   Finder）放行、`-R` reveal（Finder）放行；**其余本地文件拒**——目标 App 由
//!   LaunchServices 默认关联决定，静态不可归因，请显式 `open -a "清单内App"`
//! - `killall`/`pkill`：目标进程名/模式必须命中清单内 App 名才放行（杀进程也是
//!   触碰；pkill 模式不可枚举，只认精确等于 App 名）
//! - `osacompile`/`shortcuts`/`automator`：任何位置出现即拒（自动化工单生成器，
//!   产物绕过 osascript 闸门）
//! - 执行 `.app/Contents/MacOS/` 内的二进制：按该 .app 的真实 bundle id（读其
//!   Info.plist）对照清单——自造 applet / 直跑 App 内部二进制都走这条
//! - computer.act 输入注入（click/key/type/scroll…，见 act_gate）：注入前查前台
//!   App 的 bundle id 对照清单；前台是本 App 自身也拒（防自我点掉 exec 审批弹窗）；
//!   查询失败拒（fail-closed）
//!
//! 拒绝的返回形态：ok=true + success=false + stderr 带原因——网关对 ok=false 只读
//! 帧顶层 error，塞 payload 里会被塌成笼统 "node invoke failed" 吞掉原因（见
//! protocol.rs 注释），ok=true 才把 stderr 原样透传给 agent。
//!
//! 已知盲区（记录在案）：非 osascript 的 Apple event 发送者（python+pyobjc / swift /
//! 自编译工具）无法逐一枚举，闸门看不见其载荷，唯一兜底是该设备 exec 审批复到
//! allowlist/ask 档（命令层白名单）；`defaults write` 等文件系统层触碰不属 App
//! 控制语义，同属 exec 审批领域；act 的前台校验存在毫秒级竞态（查完到注入的瞬间
//! 前台可能变化）。

use serde_json::Value;

use super::app_perm;

/// 闸门判定。
pub enum Verdict {
    Allow,
    /// 拒绝（进 stderr 给 agent 看的原因，一段完整文案）。
    Deny(String),
}

/// system.run spawn 前的 App 控制闸门入口。
pub fn gate(argv: &[String]) -> Verdict {
    if argv.is_empty() {
        return Verdict::Allow; // 空命令由 system.run 上层报错，不归闸门管
    }
    if basename(&argv[0]) == "osascript" {
        let osa: Vec<String> = ["osascript".to_string()]
            .into_iter()
            .chain(argv[1..].iter().cloned())
            .collect();
        return analyze_oscript(&osa, &app_perm::allowed_snapshot());
    }
    if let Some(cmd) = shell_wrapper_command(argv) {
        // openclaw exec 的默认包装形态：剥壳分析内层命令串（引号感知）。
        let tokens = shell_tokenize(cmd);
        return judge_tokens(&tokens, has_shell_metachar(cmd), &app_perm::allowed_snapshot());
    }
    // 非 shell 包装：argv 本身当 token 流（无 shell 语义、无元字符）走同一套扫描
    judge_tokens(argv, false, &app_perm::allowed_snapshot())
}

/// osascript argv → 脚本 → 清单校验（纯逻辑，judge_tokens 与 gate 直连分支共用）。
fn analyze_oscript(osa: &[String], list: &[aiarsenal_client_core::types::AppPerm]) -> Verdict {
    match collect_script(osa) {
        Some(s) => check_script(&s, list),
        None => Verdict::Deny(deny(
            "UNPARSEABLE_SCRIPT",
            "osascript 载荷不可解析（无 -e/脚本文件、文件>512KB、非 UTF-8 或编译版 .scpt），拒绝执行。请用 -e 传 AppleScript 源码",
        )),
    }
}

/// token 流的统一判定（shell 包装剥壳后的分词 / 非 shell 包装的 argv，测试直调）：
/// 自动化工单生成器 → .app 内二进制 → 首位 osascript 剥壳分析 → open → killall/
/// pkill → 独立词 osascript 兜底。与以上皆无关 → 放。
fn judge_tokens(tokens: &[String], metachar: bool, list: &[aiarsenal_client_core::types::AppPerm]) -> Verdict {
    // osacompile/shortcuts/automator：自动化工单生成器，任何位置出现即拒
    if let Some(t) = tokens
        .iter()
        .find(|t| matches!(basename(t), "osacompile" | "shortcuts" | "automator"))
    {
        let tool = basename(t).to_string();
        return Verdict::Deny(deny(
            "AUTOMATION_TOOL",
            &format!("命令调用 {tool}（自动化工单生成器，产物可绕过 osascript 闸门），拒绝执行"),
        ));
    }
    // 首个有效 token 落在 .app/Contents/MacOS 内 = 直跑 App 内部二进制（含自造
    // applet）——按该 .app 真实 bundle id（读 Info.plist）对照清单
    if let Some((_, t)) = first_significant(tokens) {
        if let Some(app) = app_bundle_name_of_path(t) {
            return if is_system_events(&app) || match_app_target(&app, list) {
                Verdict::Allow // 清单内 App 的二进制：照常执行（参数不扫）
            } else {
                Verdict::Deny(not_allowed(&app, list))
            };
        }
    }
    // 首个有效 token 是 osascript（可带 env/VAR= 前缀）→ 剥壳走 AppleScript 目标分析
    if let Some((i, t)) = first_significant(tokens) {
        if basename(t) == "osascript" {
            if metachar {
                return Verdict::Deny(deny(
                    "NESTED_OSASCRIPT",
                    "osascript 与管道/链式/重定向/命令替换混排（闸门无法归因目标），拒绝执行。请把 osascript 单独一条执行",
                ));
            }
            let osa: Vec<String> = ["osascript".to_string()]
                .into_iter()
                .chain(tokens[i + 1..].iter().cloned())
                .collect();
            return analyze_oscript(&osa, list);
        }
    }
    // open：扫 token 流里每个 open 的旗标与位置参数（find -exec open / 串行多命令）
    if let Some(v) = check_open(tokens, list) {
        return v;
    }
    // killall / pkill：目标必须命中清单内 App 名
    if let Some(v) = check_kill(tokens, list) {
        return v;
    }
    // 独立词 osascript 兜底（嵌套包装、find -exec、参数内提及等）——fail-closed 拒
    if tokens.iter().any(|t| has_osascript_word(t)) {
        return Verdict::Deny(deny(
            "NESTED_OSASCRIPT",
            "命令在嵌套/混淆位置调用 osascript（闸门无法归因目标），拒绝执行。请把 osascript 作为命令行的第一个词直接执行",
        ));
    }
    Verdict::Allow
}

/// open 命令校验：扫 token 流里的每个 open，解析其旗标与位置参数（见模块注释）。
/// Some(判定)=须拦/放行中断；None=无 open 或全部通过（继续后续扫描）。
fn check_open(tokens: &[String], list: &[aiarsenal_client_core::types::AppPerm]) -> Option<Verdict> {
    let mut i = 0;
    while i < tokens.len() {
        if basename(&tokens[i]) != "open" {
            i += 1;
            continue;
        }
        let mut j = i + 1;
        let mut a_target: Option<String> = None; // -a/-b 的目标
        let mut textedit = false; // -t/-f：强制文本编辑器
        let mut reveal = false; // -R/--reveal：Finder 里显示
        let mut positional: Vec<String> = Vec::new();
        while j < tokens.len() {
            let t = tokens[j].as_str();
            if t == "--args" {
                break; // 其后是目标 App 自己的参数
            }
            if let Some(long) = t.strip_prefix("--") {
                if long == "reveal" {
                    reveal = true;
                }
                j += 1;
                continue;
            }
            if t.starts_with('-') && t.len() > 1 {
                let flags = t.trim_start_matches('-');
                if flags.contains('a') || flags.contains('b') {
                    a_target = tokens.get(j + 1).cloned();
                    j += 2;
                    continue;
                }
                if flags.contains('t') || flags.contains('f') {
                    textedit = true;
                }
                if flags.contains('R') {
                    reveal = true;
                }
                j += 1;
                continue;
            }
            positional.push(t.to_string());
            j += 1;
        }
        if let Some(target) = &a_target {
            let norm = normalize_open_target(target);
            if !is_system_events(&norm) && !match_app_target(&norm, list) {
                return Some(Verdict::Deny(not_allowed(&norm, list)));
            }
        } else if textedit {
            if !match_list("com.apple.TextEdit", list) && !match_app_target("TextEdit", list) {
                return Some(Verdict::Deny(not_allowed("com.apple.TextEdit", list)));
            }
        } else {
            // 裸位置参数：URL（含 scheme）/目录（Finder）/reveal 放行；.app 路径按
            // 该 App 归因校验；其余本地文件拒（默认关联不可静态归因）
            for p in &positional {
                if reveal || looks_like_url(p) || is_dir(p) {
                    continue;
                }
                if p.ends_with(".app") {
                    let norm = normalize_open_target(p);
                    if is_system_events(&norm) || match_app_target(&norm, list) {
                        continue;
                    }
                    return Some(Verdict::Deny(not_allowed(&norm, list)));
                }
                return Some(Verdict::Deny(deny(
                    "OPEN_UNATTRIBUTED",
                    &format!("open 本地文件 {p} 的目标 App 由系统默认关联决定，闸门无法归因（fail-closed）。请用 open -a \"App名\" 显式指定清单内的 App"),
                )));
            }
        }
        i = j.max(i + 1);
    }
    None
}

/// killall/pkill 校验：目标必须命中清单内 App 名（杀进程也是触碰）。
fn check_kill(tokens: &[String], list: &[aiarsenal_client_core::types::AppPerm]) -> Option<Verdict> {
    for (i, t) in tokens.iter().enumerate() {
        match basename(t) {
            "killall" => {
                let name = tokens[i + 1..].iter().find(|x| !x.starts_with('-'));
                match name {
                    Some(n) if is_system_events(n) || match_app_target(n, list) => {}
                    Some(n) => return Some(Verdict::Deny(not_allowed(n, list))),
                    None => {
                        return Some(Verdict::Deny(deny(
                            "KILL_UNATTRIBUTED",
                            "killall 缺进程名，拒绝执行",
                        )))
                    }
                }
            }
            "pkill" => {
                // 模式匹配范围不可枚举，只认精确等于清单内 App 名/进程名的模式
                let pat = tokens[i + 1..].iter().rev().find(|x| !x.starts_with('-'));
                match pat {
                    Some(p) if match_app_target(p, list) => {}
                    Some(p) => {
                        return Some(Verdict::Deny(deny(
                            "PKILL_PATTERN",
                            &format!("pkill 仅放行精确等于清单内 App 名的模式（当前: {p}）——模式通配范围不可归因，拒绝执行"),
                        )))
                    }
                    None => {
                        return Some(Verdict::Deny(deny(
                            "PKILL_PATTERN",
                            "pkill 缺模式，拒绝执行",
                        )))
                    }
                }
            }
            _ => {}
        }
    }
    None
}

/// open -a 的目标归一：.app 路径取 bundle 名，其余原样。
fn normalize_open_target(v: &str) -> String {
    let t = v.trim();
    if t.ends_with(".app") {
        basename(t).trim_end_matches(".app").to_string()
    } else {
        t.to_string()
    }
}

/// 路径落在 .app/Contents/MacOS 内 → 返回该 .app 的 bundle 名（末路径段去 .app）。
fn app_bundle_name_of_path(p: &str) -> Option<String> {
    let idx = p.find(".app/Contents/MacOS")?;
    Some(basename(&p[..idx + 4]).trim_end_matches(".app").to_string())
}

/// 目标（App 名/.app 路径）对照清单：先名字直配（bundle 全等/名/末段），再读磁盘
/// 把名字解析成真实 bundle id 精配——如 "Google Chrome"→com.google.Chrome，避免
/// App 显示名与清单 app_name 不同字时误拒。
fn match_app_target(target: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> bool {
    if match_list(target, list) {
        return true;
    }
    match resolve_bundle_id(target) {
        Some(bid) => list.iter().any(|a| a.bundle_id.trim().eq_ignore_ascii_case(&bid)),
        None => false,
    }
}

/// 名字/.app 路径 → 真实 bundle id：读其 Info.plist 的 CFBundleIdentifier（plutil）。
/// 名字形态在常见安装位置（/Applications、/System/Applications、CoreServices）找。
fn resolve_bundle_id(name_or_path: &str) -> Option<String> {
    let p = name_or_path.trim();
    let plist = if p.ends_with(".app") {
        format!("{p}/Contents/Info.plist")
    } else {
        ["/Applications", "/System/Applications", "/System/Library/CoreServices"]
            .iter()
            .map(|d| format!("{d}/{p}.app/Contents/Info.plist"))
            .find(|c| std::path::Path::new(c).exists())?
    };
    if !std::path::Path::new(&plist).exists() {
        return None;
    }
    let out = std::process::Command::new("plutil")
        .args(["-extract", "CFBundleIdentifier", "raw", "-o", "--", &plist])
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let bid = String::from_utf8_lossy(&out.stdout).trim().to_string();
    if bid.is_empty() {
        None
    } else {
        Some(bid)
    }
}

/// URL 判定：带 scheme（http://、mailto:、sms: …）——含 "://" 或「冒号前无 /」。
fn looks_like_url(s: &str) -> bool {
    if s.contains("://") {
        return true;
    }
    match s.find(':') {
        Some(c) => !s[..c].contains('/'),
        None => false,
    }
}

/// 路径是否目录（open 目录只能开 Finder——Finder 揭示视为无害放行）。
fn is_dir(p: &str) -> bool {
    std::fs::metadata(p).map(|m| m.is_dir()).unwrap_or(false)
}

/// computer.act 输入注入闸门：注入前查前台 App 的 bundle id 对照清单。
/// fail-closed：查询失败/前台未知 → 拒；前台是本 App 自身 → 拒（防自我点掉
/// exec 审批弹窗）。调用点见 protocol.rs exec_act 的 INJECTIVE_ACTIONS。
pub fn act_gate() -> Result<(), String> {
    let out = std::process::Command::new("osascript")
        .arg("-e")
        .arg(
            r#"tell application "System Events" to get bundle identifier of first application process whose frontmost is true"#,
        )
        .output();
    let bid = match out {
        Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout)
            .trim()
            .trim_matches('"')
            .to_string(),
        _ => {
            return Err(deny(
                "ACT_FRONTMOST_UNKNOWN",
                "无法确定前台 App（System Events 查询失败），fail-closed 拒绝输入注入",
            ))
        }
    };
    if bid.is_empty() {
        return Err(deny(
            "ACT_FRONTMOST_UNKNOWN",
            "无法确定前台 App（查询结果为空），fail-closed 拒绝输入注入",
        ));
    }
    // 前台是本 App 自身 → 拒：防 agent 用 act 点掉/操纵自己的 exec 审批弹窗
    if bid.eq_ignore_ascii_case(&self_bundle_id()) {
        return Err(deny(
            "ACT_SELF",
            "前台是本 App 自身（exec 审批弹窗所在处），拒绝注入输入——防自我操纵审批",
        ));
    }
    let list = app_perm::allowed_snapshot();
    if list.iter().any(|a| a.bundle_id.trim().eq_ignore_ascii_case(&bid)) {
        return Ok(());
    }
    Err(not_allowed(&bid, &list))
}

/// 本 App 自身的 bundle id（act 防自击用；读不到返回空串=只做清单校验）。
fn self_bundle_id() -> String {
    std::env::current_exe()
        .ok()
        .and_then(|p| {
            let s = p.to_string_lossy().to_string();
            let idx = s.find(".app/")?;
            let app = &s[..idx + 4];
            resolve_bundle_id(app)
        })
        .unwrap_or_default()
}

/// 识别 shell 包装 argv：`sh|bash|zsh|dash|ksh` + 旗标（c/l/i 组合，恰一个 c）+
/// 单个命令串。命令串后还有参数（sh 的 $0/$1…）不认——回落外层词扫描（fail-closed）。
fn shell_wrapper_command(argv: &[String]) -> Option<&str> {
    let sh = basename(&argv[0]);
    if !matches!(sh, "sh" | "bash" | "zsh" | "dash" | "ksh") {
        return None;
    }
    let mut i = 1;
    let mut saw_c = false;
    while i < argv.len() {
        let a = argv[i].as_str();
        if a.starts_with("--") || !a.starts_with('-') {
            break; // 长选项/非旗标——非标准包装形态
        }
        let body = &a[1..];
        if body.is_empty()
            || !body.chars().all(|c| c == 'c' || c == 'l' || c == 'i')
            || body.matches('c').count() > 1
        {
            return None; // 带 -e/-x 等其他旗标：可能有副作用（-e 直接执行串），不认
        }
        i += 1;
        if body.contains('c') {
            saw_c = true;
            break;
        }
    }
    if !saw_c {
        return None;
    }
    match argv.get(i) {
        Some(cmd) if argv.len() == i + 1 => Some(cmd.as_str()),
        _ => None,
    }
}

/// shell 引号感知分词（近似）：单引号内全字面；双引号内 `\" \\ \$ \` 转义；裸 `\x`
/// 转义下一字符。产出 token 已剥引号/解转义——`-e` 载荷拿到的是 AppleScript 原文。
fn shell_tokenize(s: &str) -> Vec<String> {
    let mut tokens = Vec::new();
    let mut cur = String::new();
    let mut have = false;
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
                '\\' => {
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
                }
                '"' => {
                    in_double = true;
                    have = true;
                }
                '\\' => {
                    if let Some(n) = chars.next() {
                        cur.push(n);
                    }
                    have = true;
                }
                c if c.is_whitespace() => {
                    if have {
                        tokens.push(std::mem::take(&mut cur));
                        have = false;
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
        tokens.push(cur);
    }
    tokens
}

/// 命令串是否含引号外的 shell 元字符（; | & 换行 <> ( ) ` $( )——双引号内仅
/// ` $( 视为活元字符，其余字面）。osascript 与这些混排时目标不可归因 → 拒。
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

/// 首个有效 token 的下标（跳过前导 VAR= 赋值与 env 前缀）。
fn first_significant(tokens: &[String]) -> Option<(usize, &String)> {
    let mut i = 0;
    while i < tokens.len() {
        let t = tokens[i].as_str();
        if basename(t) == "env" || is_env_assignment(t) {
            i += 1;
            continue;
        }
        return Some((i, &tokens[i]));
    }
    None
}

/// `NAME=value` 形态（环境变量前置赋值）。
fn is_env_assignment(t: &str) -> bool {
    let Some(eq) = t.find('=') else { return false };
    let name = &t[..eq];
    !name.is_empty()
        && name.chars().next().map(|c| c.is_ascii_alphabetic() || c == '_').unwrap_or(false)
        && name.chars().all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// 纯校验逻辑（测试直调）：脚本 + 清单 → Allow / Deny。
fn check_script(script: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> Verdict {
    let low = script.to_lowercase();
    if low.contains("do shell script") {
        return Verdict::Deny(deny(
            "NESTED_SHELL",
            "脚本含 do shell script（脚本内再起 shell，内层目标闸门不可见），拒绝执行",
        ));
    }
    let app_targets = extract_app_targets(script);
    let proc_targets = extract_process_targets(script);
    // keystroke 无 process 目标 = 输入打进前台 App（绕过清单），拒；有 process
    // 目标则由下方清单校验兜住。click at 同理（坐标点击命中前台）。
    let untargeted_input = low.contains("keystroke")
        || low.contains("key code")
        || low.contains("click at");
    if untargeted_input && proc_targets.is_empty() {
        return Verdict::Deny(deny(
            "UNTARGETED_INPUT",
            "keystroke/key code/click at 未通过 tell process 指定目标 App（输入会打进前台 App，绕过清单）。请用 tell process \"App名\" 包住，或对该 App 用 tell application 直接驱动",
        ));
    }
    if app_targets.is_empty() && proc_targets.is_empty() {
        if low.contains("tell") {
            return Verdict::Deny(deny(
                "UNPARSEABLE_TARGET",
                "脚本含 tell 但解析不出目标 App（变量间接引用等，fail-closed）。请用 tell application \"App名\" / tell application id \"bundle.id\" 直书目标",
            ));
        }
        return Verdict::Allow; // 无控制目标：display dialog / 纯计算等，放行
    }
    for t in app_targets.iter().chain(proc_targets.iter()) {
        if is_system_events(t) {
            continue; // System Events 作为目标常放；其危险动词已被上面三条约束
        }
        if !match_list(t, list) {
            return Verdict::Deny(not_allowed(t, list));
        }
    }
    Verdict::Allow
}

// ===== osascript argv → 脚本文本 =====

/// 从 osascript argv 收集脚本文本：全部 `-e` 片段拼接 + 位置参数脚本文件内容。
/// `-l` 声明非 AppleScript 语言（JXA 语法解析不了）返回 None 拒绝。不可解析 None。
fn collect_script(argv: &[String]) -> Option<String> {
    let mut parts: Vec<String> = Vec::new();
    let mut files: Vec<String> = Vec::new();
    let mut i = 1;
    while i < argv.len() {
        let a = argv[i].as_str();
        if a == "-e" || a == "--expression" {
            parts.push(argv.get(i + 1)?.clone());
            i += 2;
        } else if a == "-l" || a == "--language" {
            let lang = argv.get(i + 1)?;
            if lang.to_lowercase() != "applescript" {
                return None; // JXA 等语法静态解析不了——fail-closed
            }
            i += 2;
        } else if a == "--" {
            files.extend(argv.iter().skip(i + 1).cloned());
            break;
        } else if a.starts_with('-') {
            i += 1; // 其余开关（-s 等）按无值处理；漏吞值只会漏载荷→拒，安全向
        } else {
            files.push(a.to_string());
            i += 1;
        }
    }
    let mut script = parts.join("\n");
    for f in files {
        script.push('\n');
        script.push_str(&read_script_file(&f)?);
    }
    if script.trim().is_empty() {
        return None; // 无载荷（stdin 不可用）——拒
    }
    Some(script)
}

/// 读脚本文件：>512KB / 非 UTF-8（编译版 .scpt）拒。
fn read_script_file(path: &str) -> Option<String> {
    let bytes = std::fs::read(path).ok()?;
    if bytes.len() > 512 * 1024 {
        return None;
    }
    String::from_utf8(bytes).ok()
}

// ===== AppleScript 目标提取 =====

/// `tell application|app (id)? "X"` → X 列表。
fn extract_app_targets(script: &str) -> Vec<String> {
    let re = regex::Regex::new(
        r#"(?i)tell\s+(?:application|app)\s+(?:id\s+)?["']([^"'\n]+)["']"#,
    )
    .unwrap();
    re.captures_iter(script)
        .map(|c| c[1].trim().to_string())
        .filter(|s| !s.is_empty())
        .collect()
}

/// `tell (application) process "P"`（System Events UI scripting）→ P 列表。
fn extract_process_targets(script: &str) -> Vec<String> {
    let re = regex::Regex::new(r#"(?i)tell\s+(?:application\s+)?process\s+["']([^"'\n]+)["']"#)
        .unwrap();
    re.captures_iter(script)
        .map(|c| c[1].trim().to_string())
        .filter(|s| !s.is_empty())
        .collect()
}

/// System Events 的两种称呼（名 / bundle id）。
fn is_system_events(target: &str) -> bool {
    let t = target.trim().to_lowercase();
    t == "system events" || t == "com.apple.systemevents"
}

/// 目标匹配清单：bundle_id 全等 / app_name / bundle 末段（tell application
/// "TextEdit" vs com.apple.TextEdit / 进程名=本地化 app_name），忽略大小写。
fn match_list(target: &str, list: &[aiarsenal_client_core::types::AppPerm]) -> bool {
    let t = target.trim().to_lowercase();
    list.iter().any(|a| {
        let bundle = a.bundle_id.trim().to_lowercase();
        if bundle.is_empty() {
            return false;
        }
        let last = bundle.rsplit('.').next().unwrap_or("").to_string();
        t == bundle || t == a.app_name.trim().to_lowercase() || t == last
    })
}

// ===== 文案 =====

/// 拒绝文案（结构类：嵌套/不可解析/无目标输入）。统一 APP_GATE_DENIED 前缀，
/// agent 可区分于 APP_NOT_ALLOWED（清单未勾选）。
fn deny(code: &str, detail: &str) -> String {
    format!("APP_GATE_DENIED({code}): {detail}")
}

/// 清单未命中文案：带目标名 + 当前清单提示 + 恢复路径（勾选即生效，不重弹框）。
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
        "APP_NOT_ALLOWED: {target} 不在 App 控制清单{list_hint}。如需允许，请在管理端「App 权限」勾选该 App 后重试（已授权过的不重弹系统框，勾选后放行即刻生效）"
    )
}

// ===== 小工具 =====

fn basename(p: &str) -> &str {
    p.rsplit('/').next().unwrap_or(p)
}

/// 独立词 osascript（大小写不敏感；/usr/bin/osascript 的路径边界也算）。
fn has_osascript_word(s: &str) -> bool {
    regex::Regex::new(r"(?i)\bosascript\b")
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

    fn list(bundles: &[(&str, &str)]) -> Vec<AppPerm> {
        bundles
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

    #[test]
    fn allow_harmless_script_without_tell() {
        let v = check_script(r#"display dialog "hi""#, &list(&[("com.apple.TextEdit", "文本编辑")]));
        assert!(matches!(v, Verdict::Allow));
    }

    #[test]
    fn allow_listed_app_by_bundle_id_name_and_last_segment() {
        let l = list(&[("com.apple.TextEdit", "文本编辑"), ("com.google.Chrome", "Chrome")]);
        for s in [
            r#"tell application id "com.apple.TextEdit" to count windows"#,
            r#"tell application "文本编辑" to count windows"#,
            r#"tell app "TextEdit" to count windows"#,
        ] {
            assert!(matches!(check_script(s, &l), Verdict::Allow), "应放行: {s}");
        }
    }

    #[test]
    fn allow_system_events_queries_but_not_unlisted_process() {
        let l = list(&[("com.apple.TextEdit", "文本编辑")]);
        assert!(matches!(
            check_script(r#"tell application "System Events" to count windows"#, &l),
            Verdict::Allow
        ));
        let r = denied(check_script(
            r#"tell application "System Events" to tell process "Safari" to keystroke "x""#,
            &l,
        ));
        assert!(r.starts_with("APP_NOT_ALLOWED: Safari"), "{r}");
    }

    #[test]
    fn allow_targeted_keystroke_to_listed_process() {
        let l = list(&[("com.apple.Safari", "Safari")]);
        assert!(matches!(
            check_script(
                r#"tell application "System Events" to tell process "Safari" to keystroke "x""#,
                &l
            ),
            Verdict::Allow
        ));
    }

    #[test]
    fn deny_unlisted_app() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        let r = denied(check_script(r#"tell application id "com.apple.TextEdit" to count windows"#, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: com.apple.TextEdit"), "{r}");
    }

    #[test]
    fn deny_empty_list_blocks_all_apps() {
        let r = denied(check_script(r#"tell application "Finder" to count windows"#, &[]));
        assert!(r.contains("清单为空"), "{r}");
    }

    #[test]
    fn deny_do_shell_script_and_untargeted_keystroke_and_unparseable_tell() {
        let l = list(&[("com.apple.TextEdit", "文本编辑")]);
        assert!(denied(check_script(
            r#"tell application "TextEdit" to do shell script "ls""#, &l
        ))
        .contains("NESTED_SHELL"));
        assert!(denied(check_script(
            r#"tell application "System Events" to keystroke "x""#, &l
        ))
        .contains("UNTARGETED_INPUT"));
        assert!(denied(check_script(r#"tell foo to bar"#, &l)).contains("UNPARSEABLE_TARGET"));
    }

    #[test]
    fn collect_script_joins_e_fragments_and_reads_files() {
        let argv: Vec<String> = ["osascript", "-e", r#"tell application "Finder" to count windows"#, "-e", "return 1"]
            .iter().map(|s| s.to_string()).collect();
        let s = collect_script(&argv).unwrap();
        assert!(s.contains("Finder") && s.contains("return 1"));
        // -l JXA → None（fail-closed）
        let jxa: Vec<String> = ["osascript", "-l", "JavaScript", "-e", "Application('TextEdit')"]
            .iter().map(|s| s.to_string()).collect();
        assert!(collect_script(&jxa).is_none());
        // 无载荷 → None
        let bare: Vec<String> = vec!["osascript".to_string()];
        assert!(collect_script(&bare).is_none());
    }

    #[test]
    fn gate_paths_nested_and_direct() {
        // shell 包装 + osascript：剥壳分析（引擎未启动=空清单 → APP_NOT_ALLOWED 而非 NESTED）
        let sh: Vec<String> = ["/bin/sh", "-lc", "osascript -e 'tell application \"Finder\" to count windows'"]
            .iter().map(|s| s.to_string()).collect();
        match gate(&sh) {
            Verdict::Allow => {} // 引擎恰好播种了 Finder 的场景
            Verdict::Deny(r) => assert!(r.starts_with("APP_NOT_ALLOWED"), "{r}"),
        }
        // 非 osascript 常规命令直放
        let ls: Vec<String> = ["/bin/ls", "-la"].iter().map(|s| s.to_string()).collect();
        assert!(matches!(gate(&ls), Verdict::Allow));
        // 非 shell 包装的 argv：env 前缀 osascript 现在可解析（首个有效 token 剥壳），
        // 无 tell 目标 → 放行
        let env: Vec<String> = ["/usr/bin/env", "osascript", "-e", "return 1"]
            .iter().map(|s| s.to_string()).collect();
        assert!(matches!(gate(&env), Verdict::Allow));
        // osascript 直跑、清单命中（借 app_perm::allowed_snapshot——引擎未启动=空清单，
        // 故走 Deny 分支验证文案而非崩溃）
        let osa: Vec<String> = ["/usr/bin/osascript", "-e", r#"tell application id "com.apple.TextEdit" to count windows"#]
            .iter().map(|s| s.to_string()).collect();
        match gate(&osa) {
            Verdict::Allow => {} // 引擎恰好播种了 TextEdit 的场景
            Verdict::Deny(r) => assert!(r.starts_with("APP_NOT_ALLOWED"), "{r}"),
        }
    }

    #[test]
    fn wrapper_unwraps_single_osascript() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        // 单引号载荷、清单内 → 放行
        let v = judge_tokens(
            &shell_tokenize(r#"osascript -e 'tell application id "com.google.Chrome" to count windows'"#),
            false,
            &l,
        );
        assert!(matches!(v, Verdict::Allow));
        // 双引号 + 转义引号载荷（agent 常见形态）→ 放行
        let v = judge_tokens(
            &shell_tokenize(r#"osascript -e "tell application id \"com.google.Chrome\" to count windows""#),
            false,
            &l,
        );
        assert!(matches!(v, Verdict::Allow));
        // 清单外 → APP_NOT_ALLOWED（剥壳后目标可归因）
        let r = denied(judge_tokens(
            &shell_tokenize(r#"osascript -e 'tell application id "com.apple.TextEdit" to count windows'"#),
            false,
            &list(&[("com.google.Chrome", "Chrome")]),
        ));
        assert!(r.starts_with("APP_NOT_ALLOWED: com.apple.TextEdit"), "{r}");
        // env 前缀 / VAR= 赋值 / 全路径 → 仍识别为 osascript
        for cmd in [
            "env /usr/bin/osascript -e 'return 1'",
            "FOO=bar osascript -e 'return 1'",
            "/usr/bin/osascript -e 'return 1'",
        ] {
            let v = judge_tokens(&shell_tokenize(cmd), false, &l);
            assert!(matches!(v, Verdict::Allow), "应放行: {cmd}");
        }
    }

    #[test]
    fn wrapper_denies_mixed_and_nested_osascript() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        // 管道/重定向混排 → 拒（无法归因）
        for cmd in [
            r#"osascript -e 'tell application "Chrome" to count windows' | pbcopy"#,
            r#"osascript -e 'return 1' 2>/dev/null"#,
            r#"echo x && osascript -e 'return 1'"#,
        ] {
            let r = denied(judge_tokens(&shell_tokenize(cmd), has_shell_metachar(cmd), &l));
            assert!(r.contains("NESTED_OSASCRIPT"), "{cmd}: {r}");
        }
        // 嵌套包装 / 非首位提及 → 拒
        for cmd in [
            "/bin/sh -c 'osascript -e \"return 1\"'",
            "grep osascript /tmp/log",
            "find . -exec osascript -e 'return 1' ;",
        ] {
            let r = denied(judge_tokens(&shell_tokenize(cmd), has_shell_metachar(cmd), &l));
            assert!(r.contains("NESTED_OSASCRIPT"), "{cmd}: {r}");
        }
        // 与 osascript 无关的混排命令 → 放行
        assert!(matches!(
            judge_tokens(
                &shell_tokenize("ls -la | grep foo > /tmp/out"),
                true,
                &l
            ),
            Verdict::Allow
        ));
    }

    #[test]
    fn open_a_b_and_app_path_gate_by_list() {
        let l = list(&[("com.apple.TextEdit", "文本编辑"), ("com.google.Chrome", "Chrome")]);
        let allow = |cmd: &str| {
            let v = judge_tokens(&shell_tokenize(cmd), false, &l);
            assert!(matches!(v, Verdict::Allow), "应放行: {cmd}");
        };
        allow(r#"open -a TextEdit /tmp/x.txt"#); // -a 清单内（末段匹配）
        allow(r#"open -a "文本编辑" /tmp/x.txt"#); // -a 清单内（app_name）
        allow(r#"open -b com.apple.TextEdit /tmp/x.txt"#); // -b bundle id
        allow(r#"open -a Chrome https://example.com"#);
        allow(r#"open -ga TextEdit"#); // 组合旗标 -ga
        allow("/Applications/TextEdit.app"); // 裸 .app 路径按该 App 归因（系统 App 可解析）
        // 清单外 → APP_NOT_ALLOWED（拉起即触碰）
        let r = denied(judge_tokens(&shell_tokenize("open -a Stickies"), false, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: Stickies"), "{r}");
        // 清单外（名字在磁盘可解析 bundle id 也拒）
        let r = denied(judge_tokens(&shell_tokenize(r#"open -a "Calculator""#), false, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: Calculator"), "{r}");
        // -t/-f 强制文本编辑器：按 TextEdit 校验
        assert!(matches!(
            judge_tokens(&shell_tokenize("open -t /tmp/x.txt"), false, &l),
            Verdict::Allow
        ));
        let chrome_only = list(&[("com.google.Chrome", "Chrome")]);
        let r = denied(judge_tokens(&shell_tokenize("open -f /tmp/x.txt"), false, &chrome_only));
        assert!(r.starts_with("APP_NOT_ALLOWED: com.apple.TextEdit"), "{r}");
    }

    #[test]
    fn open_bare_positional_url_dir_reveal_ok_file_denied() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        for cmd in [
            "open https://example.com",
            "open mailto:someone@example.com", // 含 scheme 无 //
            "open /tmp",                       // 目录 → Finder
            "open -R /etc/hostname",           // reveal → Finder
        ] {
            let v = judge_tokens(&shell_tokenize(cmd), false, &l);
            assert!(matches!(v, Verdict::Allow), "应放行: {cmd}");
        }
        let r = denied(judge_tokens(&shell_tokenize("open /etc/hostname"), false, &l));
        assert!(r.contains("OPEN_UNATTRIBUTED"), "{r}");
    }

    #[test]
    fn killall_pkill_gate_by_list() {
        let l = list(&[("com.google.Chrome", "Chrome"), ("com.apple.TextEdit", "文本编辑")]);
        assert!(matches!(
            judge_tokens(&shell_tokenize("killall Chrome"), false, &l),
            Verdict::Allow
        ));
        let r = denied(judge_tokens(&shell_tokenize("killall Finder"), false, &l));
        assert!(r.starts_with("APP_NOT_ALLOWED: Finder"), "{r}");
        assert!(matches!(
            judge_tokens(&shell_tokenize("pkill -x Chrome"), false, &l),
            Verdict::Allow
        ));
        // 模式通配不可归因 → 拒（即使字面上含清单内名字）
        for cmd in ["pkill -f 'Chrome.*Helper'", "pkill Chrome."] {
            let r = denied(judge_tokens(&shell_tokenize(cmd), false, &l));
            assert!(r.contains("PKILL_PATTERN"), "{cmd}: {r}");
        }
    }

    #[test]
    fn automation_tool_generators_denied_anywhere() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        for cmd in [
            "osacompile -o /tmp/a.scpt -e 'return 1'",
            "shortcuts run MyFlow",
            "automator -i file.txt /tmp/my.workflow",
            "echo hi && osacompile -o /tmp/b.scpt -e 'return 1'",
        ] {
            let mc = has_shell_metachar(cmd);
            let r = denied(judge_tokens(&shell_tokenize(cmd), mc, &l));
            assert!(r.contains("AUTOMATION_TOOL"), "{cmd}: {r}");
        }
    }

    #[test]
    fn app_bundle_binary_exec_gate_by_list() {
        let l = list(&[("com.google.Chrome", "Chrome")]);
        // 真实系统 App：bundle id 可解析 → 清单外拒
        let r = denied(judge_tokens(
            &shell_tokenize("/System/Applications/Calculator.app/Contents/MacOS/Calculator"),
            false,
            &l,
        ));
        assert!(r.starts_with("APP_NOT_ALLOWED: Calculator"), "{r}");
        // 清单内（Chrome 常见安装位）→ 放行
        let v = judge_tokens(
            &shell_tokenize("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
            false,
            &list(&[("com.google.Chrome", "Chrome")]),
        );
        assert!(matches!(v, Verdict::Allow));
        // 自造 applet（磁盘上不存在、清单也没有）→ 拒
        let r = denied(judge_tokens(
            &shell_tokenize("/tmp/MyAgent.app/Contents/MacOS/MyAgent"),
            false,
            &l,
        ));
        assert!(r.starts_with("APP_NOT_ALLOWED: MyAgent"), "{r}");
    }

    #[test]
    fn shell_wrapper_command_detects_standard_forms() {
        let v = |a: &[&str]| -> Vec<String> { a.iter().map(|s| s.to_string()).collect() };
        assert_eq!(
            shell_wrapper_command(&v(&["/bin/sh", "-lc", "echo hi"])),
            Some("echo hi")
        );
        assert_eq!(shell_wrapper_command(&v(&["bash", "-c", "echo hi"])), Some("echo hi"));
        assert_eq!(
            shell_wrapper_command(&v(&["/bin/zsh", "-l", "-c", "echo hi"])),
            Some("echo hi")
        );
        // 命令串后多余参数 / 带 -e 等其他旗标 / 非识别 shell → None（回落词扫描）
        assert_eq!(shell_wrapper_command(&v(&["/bin/sh", "-c", "echo", "hi"])), None);
        assert_eq!(shell_wrapper_command(&v(&["/bin/sh", "-e", "echo hi"])), None);
        assert_eq!(shell_wrapper_command(&v(&["/usr/bin/python3", "-c", "x"])), None);
    }
}
