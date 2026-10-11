//! hermes agent（原生 Python，公网包安装，无 Docker）——安装 / 启停 / 运行检测。
//! 从 platform.rs 拆出。共享工具与类型经 `use super::*` 取用。

use super::*;

// HERMES_HOME：与官方 install.ps1 默认一致（%LOCALAPPDATA%\hermes）。
pub(super) fn hermes_home() -> PathBuf {
    std::env::var("LOCALAPPDATA")
        .ok()
        .map(|p| PathBuf::from(p).join("hermes"))
        .or_else(|| dirs::home_dir().map(|h| h.join(".hermes")))
        .unwrap_or_else(|| PathBuf::from("."))
}

// hermes CLI 入口，两种官方布局：
// ① v0.21.x（2026-09 起）：launcher shim 装在 $HERMES_HOME/bin/hermes.exe，由 store python 经
//   hermes_bootstrap 从 pm 世代化依赖库（installs/…/environments/…）lease 依赖启动，无 hermes-agent/venv；
// ② 旧版：scripts/install.ps1 把 venv 建在 $HERMES_HOME/hermes-agent/venv/Scripts/hermes.exe
//   （venv 目录名无点；更早写死 .venv 有点 → 探测不到、误报 notinstalled）。
// 优先认新 shim，回退旧 venv/.venv，都没有则回退 shim 路径让调用方据 exists() 报未装。
pub(super) fn hermes_binary() -> PathBuf {
    let home = hermes_home();
    let shim = home.join("bin").join("hermes.exe");
    if shim.exists() {
        return shim;
    }
    let base = home.join("hermes-agent");
    let canonical = base.join("venv").join("Scripts").join("hermes.exe");
    if canonical.exists() {
        return canonical;
    }
    let legacy = base.join(".venv").join("Scripts").join("hermes.exe");
    if legacy.exists() {
        return legacy;
    }
    shim
}

// hermes 运行时 python（shim 调 qr_login / messaging 补装用）。探测顺序：
// ① 旧布局：入口同目录的 venv/Scripts/python.exe；
// ② 新布局（v0.21.x）：入口是 bin/ shim（同目录无 python），依赖在 pm 世代 venv——
//    installs/<id>/facts.json 的 packages.venv.environment 指向选中世代（hermes_bootstrap
//    每次启动也读它 lease 依赖），qrcode 等补装必须进这个 venv 才对 hermes 运行时可见；
// ③ facts.json 缺失/解析失败时 glob 世代 venv（单 install 机器上即唯一）；
// ④ 兜底 tools/python-*/python.exe（store 裸解释器，无依赖——仅保路径非空）。
pub(super) fn venv_python() -> PathBuf {
    if let Some(p) = hermes_binary().parent().map(|d| d.join("python.exe")) {
        if p.exists() {
            return p;
        }
    }
    // ② facts.json：{"packages":{"venv":{"environment":"…\\venv",…}}}
    if let Ok(entries) = std::fs::read_dir(hermes_home().join("installs")) {
        for en in entries.flatten() {
            let facts = en.path().join("facts.json");
            if let Ok(data) = std::fs::read_to_string(&facts) {
                if let Ok(v) = serde_json::from_str::<serde_json::Value>(&data) {
                    if let Some(env) = v
                        .pointer("/packages/venv/environment")
                        .and_then(|x| x.as_str())
                    {
                        let py = PathBuf::from(env).join("Scripts").join("python.exe");
                        if py.exists() {
                            return py;
                        }
                    }
                }
            }
        }
    }
    // ③ glob 世代 venv
    if let Ok(entries) = std::fs::read_dir(hermes_home().join("installs")) {
        for en in entries.flatten() {
            let envs = en.path().join("environments");
            if let Ok(gens) = std::fs::read_dir(&envs) {
                for gen in gens.flatten() {
                    let py = gen.path().join("venv").join("Scripts").join("python.exe");
                    if py.exists() {
                        return py;
                    }
                }
            }
        }
    }
    // ④ store 裸解释器
    if let Ok(entries) = std::fs::read_dir(hermes_home().join("tools")) {
        let mut cands: Vec<PathBuf> = entries
            .flatten()
            .map(|e| e.path())
            .filter(|p| {
                p.file_name()
                    .and_then(|n| n.to_str())
                    .map(|n| n.starts_with("python-"))
                    .unwrap_or(false)
            })
            .map(|p| p.join("python.exe"))
            .filter(|p| p.exists())
            .collect();
        cands.sort(); // 目录名含版本号；同机多版本是异常态，取字典序最大即可
        if let Some(latest) = cands.pop() {
            return latest;
        }
    }
    hermes_binary()
        .parent()
        .map(|d| d.join("python.exe"))
        .unwrap_or_else(|| PathBuf::from("python.exe"))
}

// hermes 源码根（含 gateway/ 包，微信 shim 据此 import）。旧布局入口 venv/Scripts/hermes.exe 上溯
// 三级即 hermes-agent/；新布局（bin/ shim）上溯不出 → 回退 hermes_home/hermes-agent（install.ps1 固定检出版）。
pub(super) fn hermes_source_dir() -> PathBuf {
    if let Some(p) = hermes_binary()
        .parent()
        .and_then(|p| p.parent())
        .and_then(|p| p.parent())
    {
        if p.join("gateway").is_dir() {
            return p.to_path_buf();
        }
    }
    hermes_home().join("hermes-agent")
}

// hermes 托管的 uv：官方 install.ps1 v0.21.x 装在 $HERMES_HOME/tools/uv-*/uv.exe（旧版在 bin/uv.exe）；
// 都不在则回退 PATH 上的 uv。
pub(super) fn hermes_uv() -> String {
    let home = hermes_home();
    let mut cands = vec![home.join("bin").join("uv.exe")];
    if let Ok(entries) = std::fs::read_dir(home.join("tools")) {
        for e in entries.flatten() {
            if e.file_name().to_string_lossy().starts_with("uv-") {
                cands.push(e.path().join("uv.exe"));
            }
        }
    }
    for c in cands {
        if c.exists() {
            return c.display().to_string();
        }
    }
    "uv".to_string()
}

// $HERMES_HOME/.env：微信凭据（WEIXIN_*）+ GATEWAY_ALLOW_ALL_USERS 落这里，gateway 启动经 get_secret 解析。
pub(super) fn hermes_env_file() -> PathBuf {
    hermes_home().join(".env")
}

// 实际生效配置 $HERMES_HOME/config.yaml。
pub(super) fn hermes_config_path() -> PathBuf {
    hermes_home().join("config.yaml")
}

// 原始（model-only）base 配置 $HERMES_HOME/config.base.yaml：installer/用户配的 model 落这；
// 实际 config.yaml 由 write_effective_config 按微信绑定态在 base 上叠加（重启/重推不丢微信声明）。
pub(super) fn hermes_base_config_path() -> PathBuf {
    hermes_home().join("config.base.yaml")
}

pub(super) fn install_hermes(_spec: &serde_json::Value, on_log: &(dyn Fn(&str) + Send + Sync)) -> OpOut {
    // 1) 官方原生安装脚本：自动 bootstrap uv + 装 hermes + 配 HERMES_HOME。流式跑，进度逐行上报。
    //    -NonInteractive 跳过 setup 向导等需用户输入的 stage（install.ps1 顶部参数）——App 是无人
    //    值守安装（model 配置走 config.base.yaml 下发链路），且向导的 curses 交互（prompt_choice →
    //    curses_radiolist）在 Windows 上 import 即 ModuleNotFoundError: _curses，必崩。
    let url = "https://hermes-agent.nousresearch.com/install.ps1";
    on_log(&format!("$ & ([scriptblock]::Create((irm {url}))) -NonInteractive"));
    let script = format!("& ([scriptblock]::Create((irm '{url}'))) -NonInteractive");
    let (ok, out) = run_cmd_streaming("powershell", &["-NoProfile", "-Command", &script], on_log);
    // 官方脚本的 Fail 是 throw，经 scriptblock 跑时失败不一定产生非零退出码（实测 dependency
    // install failed 后 powershell 照样 exit 0）——退出码不可信，落盘核验才是唯一事实来源：
    // ① 退出码非 0 但 hermes.exe 已在（末段非致命 stage 崩，如向导前的构建尾差）→ 告警后继续；
    // ② 退出码 0 但 hermes.exe 未生成（内部失败被吞）→ 失败，错误信息带输出尾部供定位。
    if !ok && !hermes_binary().exists() {
        return (false, format!("hermes 安装失败: {out}"), String::new());
    }
    if !ok {
        on_log("⚠️ 官方安装器退出码非 0，但 hermes.exe 已在，继续后续步骤（输出尾部见安装日志）…");
    }
    if !hermes_binary().exists() {
        let tail: String = out.lines().rev().take(15).collect::<Vec<_>>().into_iter().rev().collect::<Vec<_>>().join("\n");
        return (
            false,
            format!("hermes 安装失败（安装器退出 0 但 {} 未生成，官方脚本内部失败）:\n{tail}", hermes_binary().display()),
            String::new(),
        );
    }
    // 2) 补装 aiohttp/qrcode（微信扫码依赖）：qr_login 的 aiohttp gate 在 v0.21.x 已随官方 [all]
    //    进 pm 世代 venv（已装则 uv no-op），旧 venv 布局则真装上；qrcode 渲染终端 ASCII 二维码。
    //    装进 venv_python() 探测到的世代 venv（hermes 运行时 lease 的同一环境，装别处不可见）。
    //    不再 `-e .[messaging]` 全量：telegram/discord/slack 与微信无关，editable 装在 pm 世代
    //    架构下也不对（依赖由 hermes_bootstrap 从世代库 lease，非源码 venv）。失败仅告警不阻断。
    on_log("补装微信扫码依赖（aiohttp/qrcode）…");
    let py = venv_python().display().to_string();
    let (ok2, out2) = run_cmd_streaming_dir(
        &hermes_uv(),
        &["pip", "install", "--python", &py, "aiohttp", "qrcode"],
        &hermes_source_dir(),
        on_log,
    );
    if !ok2 {
        on_log(&format!("⚠️ 微信扫码依赖补装失败（扫码会报 aiohttp missing，可重装）：{out2}"));
    }
    // 3) 建 config.base.yaml 基线（write_effective_config 按绑定态在其上叠加 weixin 块）。
    //    base 不存在时从 installer 生成的 config.yaml 快照（一次性，不丢 model 配置）。
    let base = hermes_base_config_path();
    if !base.exists() {
        if let Ok(content) = std::fs::read(hermes_config_path()) {
            let _ = std::fs::write(&base, content);
        }
    }
    (true, format!("hermes 安装成功\n{out}"), String::new())
}

pub(super) fn start_hermes(state: &PlatformState) -> OpOut {
    // 已在跑则不重复拉起（核验句柄存活，对齐 openclaw）
    {
        let mut procs = state.procs.lock().unwrap();
        if let Some(child) = procs.get_mut("hermes") {
            match child.try_wait() {
                Ok(Some(_)) => { /* stale，落到下面清掉再 spawn */ }
                _ => return (true, "hermes 已在运行".into(), child.id().to_string()),
            }
        }
        procs.remove("hermes");
    }
    let (_, running, _, _) = hermes_runtime();
    if running {
        return (true, "hermes 已在运行（非本 App 启动）".into(), String::new());
    }
    let bin = hermes_binary();
    if !bin.exists() {
        return (false, "未找到 hermes，请先安装".into(), String::new());
    }
    let home = hermes_home();
    let _ = std::fs::create_dir_all(&home);
    // app 当 external-supervisor：--no-supervise 让 gateway 以前台子进程跑（不重定向到 s6），
    // --external-supervisor 声明本 app 拥有它（in-chat restart/update 退回 app 而非自 fork 脱离）。
    match cmd_new(&bin.display().to_string())
        .args(["gateway", "run", "--no-supervise", "--external-supervisor"])
        .env("HERMES_HOME", &home)
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
    {
        Ok(child) => {
            let pid = child.id().to_string();
            state.procs.lock().unwrap().insert("hermes".into(), child);
            // 验证 gateway 真的起来了（control 管道 identify 应答）。此前 spawn 完即报
            // 成功——gateway 依赖缺失/配置坏时 boot 阶段就崩，命令照样 done（假成功）。
            // Python 冷启动慢（Electron 桌面端给 90s 窗口），轮询 60s（500ms×120 轮）；
            // 超时如实报未就绪，子进程不杀（可能仍在起，稍后心跳侧能看到）。
            for _ in 0..120 {
                if gateway_identify().is_some() {
                    return (true, "hermes 已启动（gateway control 管道已应答）".into(), pid);
                }
                std::thread::sleep(std::time::Duration::from_millis(500));
            }
            (false, "hermes 启动后 60s 内 control 管道未应答（bootstrap 失败/依赖缺失，查 $HERMES_HOME/logs）".into(), pid)
        }
        Err(e) => (false, format!("启动 hermes 失败: {e}"), String::new()),
    }
}

pub(super) fn stop_hermes(state: &PlatformState) -> OpOut {
    // 优先 hermes gateway stop（读 gateway.pid 发停止信号），回退杀本 App 持有的子进程 + taskkill。
    // 最后核验（对齐 openclaw stop 的如实核验，此前无条件报成功）：control 管道 identify
    // 失败 且 CIM 扫不到进程才算停。CIM 查询必须按进程名过滤（gateway 跑在 python.exe/
    // hermes.exe 下）——不带 Name 过滤时 powershell -Command 的命令行自带 hermes/gateway
    // 字样必自匹配（2026-09-29 实测）。刚跑过的 `hermes gateway stop` CLI（hermes.exe）自身
    // 仍会短暂命中——它数秒内退出，多轮等待天然跨过；持续存活才算没停掉。
    let bin = hermes_binary();
    if bin.exists() {
        let _ = run_cmd(&bin.display().to_string(), &["gateway", "stop"]);
    }
    let mut pid_out = String::new();
    {
        let mut procs = state.procs.lock().unwrap();
        if let Some(mut child) = procs.remove("hermes") {
            pid_out = child.id().to_string();
            let _ = child.kill();
            let _ = child.wait();
        }
    }
    let _ = run_cmd("taskkill", &["/IM", "hermes.exe", "/F"]);
    let ps = "if ((Get-CimInstance Win32_Process -Filter \"Name='python.exe' OR Name='hermes.exe'\" | Where-Object { $_.CommandLine -like '*gateway*' }).Count -gt 0) { exit 0 } else { exit 1 }";
    for _ in 0..10 {
        std::thread::sleep(std::time::Duration::from_millis(500));
        if gateway_identify().is_some() {
            continue; // 管道仍应答 = gateway 还活着
        }
        let (still, _) = run_cmd("powershell", &["-NoProfile", "-Command", ps]);
        if still {
            continue;
        }
        return (true, "hermes 已停止".into(), pid_out);
    }
    (false, "hermes 停止后 gateway 仍存活（control 管道或进程仍在，查 $HERMES_HOME/logs）".into(), pid_out)
}

// ===== gateway control socket（活性/身份探测，对齐 openclaw 的 18789 端口探测）=====
//
// hermes gateway 在 Windows 上以命名管道做 control 通道（gateway/control_socket.py）：
// \\.\pipe\hermes-gateway-<sha256(normcase(home))[:16]>，单连接单 JSON 行协议——请求
// {"verb":"identify","id":1,"protocol":1}\n，响应 {"ok":true,"result":{pid,code_version,
// supervisor,...}}。「能连上且 identify 应答合法」即活性（源码注释原话），比 PowerShell
// CIM 扫进程表快且可靠，顺带回填真 pid（此前心跳 pid 字段恒空）。旧版/自装 hermes 无
// 管道 → None，调用方降级 CIM 扫描（行为不劣于改造前）。

/// 管道名（镜像 control_socket.py windows_pipe_name / _home_hash）：
/// 对 home 规范化路径（resolve + normcase=小写）取 sha256 前 16 个十六进制字符。
fn windows_pipe_name(home: &PathBuf) -> String {
    use sha2::{Digest, Sha256};
    let canonical = home.canonicalize().unwrap_or_else(|_| home.clone());
    // normcase：Windows 大小写不敏感，统一小写（对齐 Python os.path.normcase）
    let normalized = canonical.display().to_string().to_lowercase();
    let mut h = Sha256::new();
    h.update(normalized.as_bytes());
    let digest = h.finalize();
    let hex: String = digest.iter().take(8).map(|b| format!("{b:02x}")).collect();
    format!(r"\\.\pipe\hermes-gateway-{hex}")
}

/// gateway_identify：连命名管道发 identify。管道不在/超时/应答非法 → None。
/// 管道读写无超时原语：I/O 丢后台线程，主线程 1s 收不到结果视同无应答（Python 客户端
/// 预算 2s 收紧）——路径性挂死不阻塞心跳；线程随进程生命周期回收。
pub(super) fn gateway_identify() -> Option<serde_json::Value> {
    let pipe = windows_pipe_name(&hermes_home());
    let (tx, rx) = std::sync::mpsc::channel();
    std::thread::spawn(move || {
        let _ = tx.send(identify_over_pipe(&pipe));
    });
    rx.recv_timeout(std::time::Duration::from_secs(1)).ok()?
}

fn identify_over_pipe(pipe: &str) -> Option<serde_json::Value> {
    use std::io::{Read, Write};
    // 打开：busy（其它客户端握手途中）短暂重试，对齐 Python 客户端 50ms 间隔；
    // 管道不存在 = 没有 gateway 在服务，立即放弃。
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(1);
    let mut file = loop {
        match std::fs::OpenOptions::new().read(true).write(true).open(pipe) {
            Ok(f) => break f,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return None,
            Err(_) => {
                if std::time::Instant::now() >= deadline {
                    return None;
                }
                std::thread::sleep(std::time::Duration::from_millis(50));
            }
        }
    };
    if file.write_all(br#"{"verb":"identify","id":1,"protocol":1}"#).is_err()
        || file.write_all(b"\n").is_err()
    {
        return None;
    }
    let mut buf: Vec<u8> = Vec::new();
    let mut chunk = [0u8; 8192];
    loop {
        let n = file.read(&mut chunk).ok()?;
        if n == 0 {
            break; // 服务端答完关管道
        }
        buf.extend_from_slice(&chunk[..n]);
        if buf.contains(&b'\n') {
            break; // 单行协议：首个换行即完整应答
        }
        if buf.len() > 512 * 1024 {
            return None; // 响应上限（对齐服务端 _MAX_RESPONSE_BYTES）
        }
    }
    let line = buf.split(|&b| b == b'\n').next()?;
    let v: serde_json::Value = serde_json::from_slice(line).ok()?;
    if v.get("ok").and_then(|x| x.as_bool()) != Some(true) {
        return None;
    }
    v.get("result").cloned().filter(|r| r.is_object())
}

pub(super) fn hermes_runtime() -> (bool, bool, String, String) {
    let bin = hermes_binary();
    let bin_exists = bin.exists();
    // version 只取首行并截 64：对齐 openclaw，防异常时多行错误塞爆后端 varchar(64)。
    let version = if bin_exists {
        let v = cmd_stdout(&bin.display().to_string(), &["--version"]);
        let first = v.lines().next().unwrap_or("").trim();
        if first.chars().count() > 64 {
            first.chars().take(64).collect::<String>()
        } else {
            first.to_string()
        }
    } else {
        String::new()
    };
    // installed 必须入口在 且 --version 读得出版本号：二进制在但 venv 损坏/依赖缺失跑不起来，
    // 等同没装好（光 exists() 会把"装坏了"也判成已装）。读不出版本 → notinstalled。
    let installed = bin_exists && !version.is_empty();
    // running 以 control 管道 identify 为准，顺带回填真 pid；identify 不通（未跑/旧版无
    // 管道）→ CIM 扫进程表兜底。Name 过滤（python.exe/hermes.exe）排除查询进程自身：
    // powershell -Command 命令行含 hermes/gateway 字样，不过滤必自匹配 → gateway 死透
    // 仍恒报 running、start_hermes 被短路「已在运行」（2026-09-29 实测）。
    let mut pid = String::new();
    let mut running = false;
    if installed {
        if let Some(id) = gateway_identify() {
            running = true;
            if let Some(p) = id.get("pid").and_then(|v| v.as_u64()) {
                pid = p.to_string();
            }
        } else {
            let ps = "if ((Get-CimInstance Win32_Process -Filter \"Name='python.exe' OR Name='hermes.exe'\" | Where-Object { $_.CommandLine -like '*gateway*' }).Count -gt 0) { exit 0 } else { exit 1 }";
            let (ps_ok, _) = run_cmd("powershell", &["-NoProfile", "-Command", ps]);
            running = ps_ok;
        }
    }
    (installed, running && installed, version, pid)
}

// ===== 技能装/卸（skill_install / skill_uninstall，平台层公共「下载 zip→解压→定位根」后调用）=====
//
// hermes 不走 `hermes skills install` CLI：其 identifier 只认 hub 名/HTTP URL（skills_hub.py
// 源适配器），平台包是设备 token 下载的本地 zip；而 hermes 的技能枚举（tools/skills_tool
// _find_all_skills → iter_skill_index_files）是全树扫盘，lockfile 只标注来源/trust——直接
// 落盘 $HERMES_HOME\skills\<slug>\ 即被识别为 local skill（无 trust 要求：trust 只管仓库内
// 项目级技能），且扫描缓存按目录 mtime 失效，落盘即生效无需重启 gateway。卸载=删目录。

// skill_install：解压出的技能根 root 落盘到 $HERMES_HOME\skills\<slug>\。
// 幂等覆盖（对齐 openclaw 流程的挪-装-回滚舞步）：先挪走旧版本 → move_dir 落盘 →
// 失败挪回。move_dir 跨卷回退递归复制（tmp 与 home 不同卷也能落）。
pub(super) fn skill_install(
    root: &PathBuf,
    slug: &str,
    name: &str,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    let skills_dir = hermes_home().join("skills");
    if let Err(e) = std::fs::create_dir_all(&skills_dir) {
        return (false, format!("创建技能目录失败: {e}"));
    }
    let target = skills_dir.join(slug);
    let bak = skills_dir.join(format!("{slug}.aa-installing"));
    let _ = std::fs::remove_dir_all(&bak); // 清上次异常残留
    let mut backed_up = false;
    if target.is_dir() {
        if let Err(e) = std::fs::rename(&target, &bak) {
            return (false, format!("挪走旧版本目录失败: {e}"));
        }
        backed_up = true;
    }
    match aiarsenal_client_core::skills::move_dir(root, &target) {
        Ok(()) => {
            let _ = std::fs::remove_dir_all(&bak); // 新版就位，清备份
            on_log(&format!("hermes 技能已落盘: {}", target.display()));
            (true, format!("技能 {name} 已安装（hermes 扫盘即生效，无需重启）"))
        }
        Err(e) => {
            if backed_up {
                let _ = std::fs::rename(&bak, &target); // 挪回旧版本
            }
            (false, format!("落盘 hermes 技能目录失败: {e}"))
        }
    }
}

// skill_uninstall：删技能目录（find_skill_dir_by_slug 与 hermes 扫盘同规则，支持 category
// 嵌套形态）。不在目录 = 幂等成功（对齐 openclaw skill_uninstall 语义）。
pub(super) fn skill_uninstall(slug: &str, on_log: &(dyn Fn(&str) + Send + Sync)) -> (bool, String) {
    let skills_dir = hermes_home().join("skills");
    match aiarsenal_client_core::skills::find_skill_dir_by_slug(&skills_dir, slug) {
        Some(dir) => match std::fs::remove_dir_all(&dir) {
            Ok(()) => {
                on_log(&format!("hermes 技能已删除: {}", dir.display()));
                (true, format!("技能 {slug} 已卸载（{}）", dir.display()))
            }
            Err(e) => (false, format!("删除技能目录失败: {e}")),
        },
        None => (true, format!("技能 {slug} 不在 hermes 技能目录，视为已卸载（幂等）")),
    }
}

// ===== 微信绑定（channel_login / channel_logout）=====
//
// hermes 无非交互微信登录 CLI（仅交互式 `gateway setup` 向导）。这里走 Python shim 调 hermes 内部
// gateway.platforms.weixin.qr_login：它把二维码 URL 打印到 stdout + 自持久化账号文件。
// 镜像 openclaw 的 channel_login_via_cli（非阻塞）：spawn shim 后立即返回 done，后台线程逐行抓 stdout
// 的 QR URL 写 channel_login 共享态（心跳回传后端渲染二维码），shim 退出（扫码完成）→ 写 connected/failed。
// 蓝图移植自 image/manager/hermes.go（容器侧已验证），路径改桌面布局（从 hermes_home/hermes_binary 推导）。
// 前置：messaging extra（aiohttp/qrcode）须已装（install_hermes 已追加），否则 qr_login import 即 raise。

// 绑定后追加到 base 的微信平台声明（open 策略，配合 .env 的 GATEWAY_ALLOW_ALL_USERS=true 放行所有发件人，
// 对齐 openclaw，免配对码）。与 image/manager/hermes.go 的 weixinOpenPlatformYAML 完全一致。
const WEIXIN_OPEN_PLATFORM_YAML: &str = "gateway:\n  platforms:\n    weixin:\n      enabled: true\n      extra:\n        dm_policy: \"open\"\n        group_policy: \"disabled\"\n";

// WEIXIN_LOGIN_SHIM_PY 调 hermes 内部 qr_login：qr_login 自身把二维码 URL（http liteapp）打印到 stdout +
// save_weixin_account 落账号 json；成功后 shim 把凭据（WEIXIN_ACCOUNT_ID/TOKEN/BASE_URL）合并写 .env，
// 打印 AA_WEIXIN_LOGIN_OK 哨兵；失败打印 AA_WEIXIN_LOGIN_FAILED。逐字移植自 image/manager/hermes.go，
// 桌面端补 Windows 编码：.env 是 UTF-8（官方 install.ps1 写的），Windows Python 的 open() 默认 locale
// 编码（GBK）读它必炸 UnicodeDecodeError——凭据已拿到却写不进 .env，扫码白扫（实测 2026-09-26）。
const WEIXIN_LOGIN_SHIM_PY: &str = "#!/usr/bin/env python3\n# aiarsenal 微信扫码登录 shim（由桌面 App 调用）。\nimport asyncio, os\n\n\ndef main():\n    try:\n        from gateway.platforms.weixin import qr_login\n    except Exception as exc:\n        print(\"AA_WEIXIN_LOGIN_FAILED import: %s\" % exc)\n        return\n    home = os.environ.get(\"HERMES_HOME\") or os.path.expanduser(\"~/.hermes\")\n    # qr_login 内部：打印二维码 URL（http liteapp）到 stdout + save_weixin_account 落账号 json。\n    creds = asyncio.run(qr_login(home))\n    if not creds:\n        print(\"AA_WEIXIN_LOGIN_FAILED\")\n        return\n    # 合并写 .env：去旧 WEIXIN_* 行，追加新凭据（gateway 启动经 get_secret 解析）。\n    env_path = os.path.join(home, \".env\")\n    kept = []\n    try:\n        with open(env_path, encoding=\"utf-8\", errors=\"surrogateescape\") as f:\n            for ln in f:\n                if not ln.startswith(\"WEIXIN_\"):\n                    kept.append(ln.rstrip(\"\\n\"))\n    except FileNotFoundError:\n        pass\n    rows = {\n        \"WEIXIN_ACCOUNT_ID\": creds.get(\"account_id\", \"\"),\n        \"WEIXIN_TOKEN\": creds.get(\"token\", \"\"),\n        \"WEIXIN_BASE_URL\": creds.get(\"base_url\", \"\"),\n    }\n    out = list(kept)\n    for k, v in rows.items():\n        if v:\n            out.append(\"%s=%s\" % (k, v))\n    with open(env_path, \"w\", encoding=\"utf-8\", errors=\"surrogateescape\") as f:\n        f.write(\"\\n\".join(out) + \"\\n\")\n    try:\n        os.chmod(env_path, 0o600)\n    except OSError:\n        pass\n    print(\"AA_WEIXIN_LOGIN_OK\")\n\n\nmain()\n";

// read_dot_env 解析 .env 为 map（KEY=VALUE，去引号/空白；忽略空行与 # 注释）。文件缺失返回空 map。
fn read_dot_env(path: &PathBuf) -> HashMap<String, String> {
    let mut out = HashMap::new();
    let data = match std::fs::read_to_string(path) {
        Ok(s) => s,
        Err(_) => return out,
    };
    for line in data.split('\n') {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if let Some(eq) = line.find('=') {
            let k = line[..eq].trim().to_string();
            let v = line[eq + 1..].trim().trim_matches(|c| c == '"' || c == '\'').to_string();
            out.insert(k, v);
        }
    }
    out
}

// ensure_env_key 确保 .env 含 key=val（已存在且值相同则不动；否则合并写入，保留其它行/注释）。
fn ensure_env_key(path: &PathBuf, key: &str, val: &str) {
    let data = std::fs::read_to_string(path).unwrap_or_default();
    let mut lines: Vec<String> = data.split('\n').map(|s| s.to_string()).collect();
    for i in 0..lines.len() {
        // 把对 lines[i] 的不可变借用局限在此 block，block 结束即可对 lines[i] 可变借用（NLL）。
        let already_present: Option<bool> = {
            let trimmed = lines[i].trim();
            match trimmed.find('=') {
                Some(eq) if eq > 0 && trimmed[..eq].trim() == key => {
                    Some(trimmed[eq + 1..].trim() == val) // true=已是目标值，false=需替换
                }
                _ => None,
            }
        };
        if let Some(already_correct) = already_present {
            if already_correct {
                return; // 已是目标值，不动
            }
            lines[i] = format!("{key}={val}");
            let _ = std::fs::write(path, lines.join("\n"));
            return;
        }
    }
    // 未找到 key：追加
    let mut out = data.trim_end_matches('\n').to_string();
    if !out.is_empty() {
        out.push('\n');
    }
    out.push_str(key);
    out.push('=');
    out.push_str(val);
    out.push('\n');
    let _ = std::fs::write(path, out);
}

// remove_env_keys 重写 .env，丢弃 key 命中 drop 的行（保留空行/注释/其它键）。文件缺失无操作。
fn remove_env_keys<F: Fn(&str) -> bool>(path: &PathBuf, drop_fn: F) {
    let data = match std::fs::read_to_string(path) {
        Ok(s) => s,
        Err(_) => return,
    };
    let mut kept: Vec<&str> = Vec::new();
    for line in data.split('\n') {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') || !trimmed.contains('=') {
            kept.push(line);
            continue;
        }
        if let Some(eq) = trimmed.find('=') {
            if drop_fn(trimmed[..eq].trim()) {
                continue;
            }
        }
        kept.push(line);
    }
    let _ = std::fs::write(path, kept.join("\n"));
}

// weixin_bound .env 是否已有 WEIXIN_TOKEN（=已绑定微信）。
fn weixin_bound() -> bool {
    read_dot_env(&hermes_env_file()).get("WEIXIN_TOKEN").map(|v| !v.is_empty()).unwrap_or(false)
}

// write_effective_config 按 weixin 绑定态生成实际 config.yaml：base +（已绑定时）weixin open 平台块。
// base 缺失时先从当前 config.yaml 快照建基线（一次性，兼容老安装）；未绑定 → 仅 base，已绑定 → 叠加 weixin。
fn write_effective_config() -> std::io::Result<()> {
    let base_path = hermes_base_config_path();
    if !base_path.exists() {
        if let Ok(content) = std::fs::read(hermes_config_path()) {
            let _ = std::fs::write(&base_path, content);
        }
    }
    let mut out = std::fs::read_to_string(&base_path).unwrap_or_default();
    if weixin_bound() && !out.contains("platforms:") {
        // base 已含顶层 gateway: 键（config_push 下发的配置可带，如本机实测的空 gateway: 段）→
        // 尾部再追加整块 gateway: 会 YAML 重复键。改为把 platforms 子块（WEIXIN_OPEN_PLATFORM_YAML
        // 去掉首行 gateway: 的余下部分，段内缩进已对齐）插入该键行后；无 gateway: 键才整体追加。
        if let Some(pos) = out.lines().position(|l| l.trim_end_matches('\r') == "gateway:") {
            let platforms_block = WEIXIN_OPEN_PLATFORM_YAML.trim_start_matches("gateway:\n");
            let mut lines: Vec<&str> = out.lines().collect();
            lines.insert(pos + 1, platforms_block);
            out = format!("{}\n", lines.join("\n"));
        } else {
            // base 与 weixin 块之间必须留换行：trim 掉 base 末尾换行后直接拼 gateway: 会粘到
            // 上一行（如 default: "..."gateway:）→ YAML 损坏 → hermes 判 corrupt 丢弃 model 段 →
            // 凭据回退「Provider authentication failed」。故显式补一个 \n 分隔。
            out = format!("{}\n{}", out.trim_end_matches('\n'), WEIXIN_OPEN_PLATFORM_YAML);
        }
    }
    std::fs::write(hermes_config_path(), out)
}

// save_hermes_base_config 写 API 下发的 model 配置到 config.base.yaml，再按微信绑定态生成实际 config.yaml
// （write_effective_config）。对应 openclaw 的 save_agent_config：下发内容落 base，effective 自动叠加 weixin 块。
pub(super) fn save_hermes_base_config(content: &str) -> std::io::Result<()> {
    if content.is_empty() {
        return Ok(());
    }
    let base = hermes_base_config_path();
    if let Some(parent) = base.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(&base, content)?;
    write_effective_config()
}

// is_qr_url 判断一行是否为二维码内容 URL（qr_login 打印的 http(s):// liteapp 或 weixin:// 链接）。
fn is_qr_url(s: &str) -> bool {
    s.starts_with("http://") || s.starts_with("https://") || s.starts_with("weixin://")
}

// ensure_weixin_login_shim 把 Python shim 写到 $HERMES_HOME/.aa-weixin-login.py（每次覆盖保持最新）。
fn ensure_weixin_login_shim() -> std::io::Result<PathBuf> {
    let path = hermes_home().join(".aa-weixin-login.py");
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(&path, WEIXIN_LOGIN_SHIM_PY)?;
    Ok(path)
}

// channel_login_via_shim 微信扫码登录（非阻塞，镜像 openclaw::channel_login_via_cli）：
// spawn venv python 跑 shim → 立即返回 done；后台线程抓 stdout 的 QR URL 写共享态、shim 退出判成功 →
// 放行所有发件人（.env 加 GATEWAY_ALLOW_ALL_USERS + write_effective_config 叠加 weixin open 块）+ restart + connected。
pub(super) fn channel_login_via_shim(
    channel: &str,
    state: &PlatformState,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    let shim = match ensure_weixin_login_shim() {
        Ok(p) => p,
        Err(e) => return (false, format!("写微信登录 shim 失败: {e}")),
    };
    let py = venv_python();
    if !py.exists() {
        return (false, format!("未找到 hermes python（{}），请先安装 hermes", py.display()));
    }
    let src = hermes_source_dir();
    on_log(&format!("hermes 微信扫码登录：{} {}", py.display(), shim.display()));
    aiarsenal_client_core::util::log("info", &format!("[channel_login] hermes shim 启动 channel={channel}"));
    let mut cmd = cmd_new(&py.display().to_string());
    cmd.arg(&shim)
        .current_dir(&src)
        .env("HERMES_HOME", hermes_home())
        .env("PYTHONPATH", &src)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => return (false, format!("启动微信登录 shim 失败: {e}")),
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let state_c = state.clone();
    let channel_c = channel.to_string();
    // 后台线程：读 stdout 抓 QR + 哨兵；stderr 单线程排空。命令已 done，输出走本地日志。
    std::thread::spawn(move || {
        use std::io::{BufRead, BufReader};
        let st_err = state_c.clone();
        std::thread::spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                aiarsenal_client_core::util::log("info", &format!("[channel_login] hermes stderr: {line}"));
            }
            drop(st_err);
        });
        let mut ok = false;
        for line in BufReader::new(stdout).lines().flatten() {
            aiarsenal_client_core::util::log("info", &format!("[channel_login] hermes {line}"));
            let trimmed = line.trim();
            if trimmed == "AA_WEIXIN_LOGIN_OK" {
                ok = true;
            } else if trimmed.starts_with("AA_WEIXIN_LOGIN_FAILED") {
                aiarsenal_client_core::util::log("error", &format!("[channel_login] hermes shim 失败: {trimmed}"));
            } else if is_qr_url(trimmed) {
                set_channel_login_state(&state_c, Some(ChannelLoginState {
                    channel: channel_c.clone(),
                    login_status: "qr_pending".into(),
                    qr_data_url: trimmed.to_string(),
                    message: "请扫码".into(),
                    ..Default::default()
                }));
            }
        }
        let exited_ok = child.wait().map(|s| s.success()).unwrap_or(false);
        if ok && exited_ok {
            // 放行所有发件人（对齐 openclaw，免配对码）：.env 加 GATEWAY_ALLOW_ALL_USERS + 叠加 weixin open 块 + restart。
            ensure_env_key(&hermes_env_file(), "GATEWAY_ALLOW_ALL_USERS", "true");
            if let Err(e) = write_effective_config() {
                aiarsenal_client_core::util::log("error", &format!("[channel_login] hermes 写有效配置失败: {e}"));
            }
            stop_hermes(&state_c);
            start_hermes(&state_c);
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c.clone(),
                login_status: "connected".into(),
                message: "登录成功".into(),
                ..Default::default()
            }));
        } else {
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c.clone(),
                login_status: "failed".into(),
                message: if ok { "shim 异常退出".into() } else { "扫码未完成".into() },
                ..Default::default()
            }));
        }
    });
    (true, "已下发扫码登录，请扫码（二维码稍后出现在渠道绑定区）".into())
}

// channel_logout_hermes 解绑微信：删 .env 的 WEIXIN_* + GATEWAY_ALLOW_ALL_USERS、删账号 json、
// 重生成 config.yaml（无 WEIXIN_TOKEN → 不含 weixin 块）+ restart 丢弃 weixin。
pub(super) fn channel_logout_hermes(state: &PlatformState) -> (bool, String) {
    remove_env_keys(&hermes_env_file(), |k| k.starts_with("WEIXIN_") || k == "GATEWAY_ALLOW_ALL_USERS");
    let accounts_dir = hermes_home().join("weixin").join("accounts");
    if let Ok(entries) = std::fs::read_dir(&accounts_dir) {
        for en in entries.flatten() {
            let p = en.path();
            if p.extension().map(|e| e == "json").unwrap_or(false) {
                let _ = std::fs::remove_file(&p);
            }
        }
    }
    let _ = write_effective_config();
    stop_hermes(state);
    start_hermes(state);
    (true, "已解绑微信（清凭据 + 重启生效）".into())
}

