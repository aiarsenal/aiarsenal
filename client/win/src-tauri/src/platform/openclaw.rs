//! openclaw agent（Node CLI）——安装 / 启停 / 运行检测 / 配置 / 渠道 CLI 降级登录。
//! 从 platform.rs 拆出，避免与 hermes/obsidian 互相影响。共享工具（run_cmd/which/...）
//! 与类型（PlatformState/OpOut）经 `use super::*` 取用。

use super::*;

/// openclaw 安装版本钉死（编译期 option_env! AA_OPENCLAW_VERSION）。Some = 强制装该版本（忽略后端 spec.version）；
/// None = 沿用后端 install 命令下发的 spec.version（默认 "latest"）。client/build.sh --openclaw-version 注入。
const PINNED_OPENCLAW_VERSION: Option<&str> = option_env!("AA_OPENCLAW_VERSION");

/// CLI 降级登录（非阻塞）：后台跑 `openclaw channels login --channel <id>`，**立即返回 done**，
/// 不阻塞命令队列。后台线程逐行读 stdout 抓 QR 内容 URL → 写 channel_login 共享态（心跳回传后端），
/// 等 channels login 退出（扫码完成/超时）→ 写 connected/failed。用于不实现 web.login RPC 的渠道（微信）。
/// on_log 仅用于记"已下发"那一行（命令随即 done），后台线程输出走 util::log 本地日志。
pub(super) fn channel_login_via_cli(
    channel: &str,
    state: &PlatformState,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> (bool, String) {
    on_log(&format!("降级 CLI 登录：$ openclaw channels login --channel {channel}（非阻塞，后台执行）"));
    aiarsenal_client_core::util::log("info", &format!("[channel_login] CLI 降级（非阻塞）channel={channel}"));
    let mut cmd = cmd_new(&openclaw_bin());
    cmd.args(["channels", "login", "--channel", channel]);
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => return (false, format!("启动 channels login 失败: {e}")),
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let state_c = state.clone();
    let channel_c = channel.to_string();
    // 后台线程：读 stdout 抓 QR + 等退出。命令已 done，输出走本地日志。
    std::thread::spawn(move || {
        use std::io::{BufRead, BufReader};
        let st_err = state_c.clone();
        std::thread::spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                aiarsenal_client_core::util::log("info", &format!("[channel_login] stderr: {line}"));
            }
            drop(st_err);
        });
        let mut next_is_url = false;
        for line in BufReader::new(stdout).lines().flatten() {
            aiarsenal_client_core::util::log("info", &format!("[channel_login] {line}"));
            let trimmed = line.trim();
            if next_is_url && !trimmed.is_empty() {
                set_channel_login_state(&state_c, Some(ChannelLoginState {
                    channel: channel_c.clone(),
                    login_status: "qr_pending".into(),
                    qr_data_url: trimmed.to_string(),
                    message: "请扫码".into(),
                    ..Default::default()
                }));
                aiarsenal_client_core::util::log("info", &format!("[channel_login] CLI 抓到 QR URL（{}字节）", trimmed.len()));
                next_is_url = false;
                continue;
            }
            if trimmed.contains("访问以下链接") || trimmed.contains("以下链接以继续") {
                next_is_url = true;
            }
        }
        let ok = child.wait().map(|s| s.success()).unwrap_or(false);
        aiarsenal_client_core::util::log("info", &format!("[channel_login] CLI channels login 退出 ok={ok}"));
        if ok {
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c.clone(),
                login_status: "connected".into(),
                message: "登录成功".into(),
                ..Default::default()
            }));
            // 登录成功后【不】重启 gateway：channels login 经 gateway 完成登录后它会自动拉起账号
            // monitor（channelConfigUpdatedAt 变化 → 渠道热重载，不换进程）。此处补 `gateway restart`
            // 反而杀掉活跃长轮询——服务器侧死流未超时前（实测 ~12 分钟）不派发消息，登录后的
            // 消息全部静默丢失（实踩 2026-08-26 22:11，mac 端同修）。
        } else {
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c,
                login_status: "failed".into(),
                message: "channels login 退出非 0".into(),
                ..Default::default()
            }));
        }
    });
    (true, "已下发扫码登录，请扫码（二维码稍后出现在渠道绑定区）".into())
}

pub(super) fn install_openclaw(
    state: &PlatformState,
    spec: &serde_json::Value,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> OpOut {
    // 版本优先级：编译期钉死（AA_OPENCLAW_VERSION）> 后端 spec.version > "latest"
    let version = PINNED_OPENCLAW_VERSION
        .or_else(|| spec.get("version").and_then(|v| v.as_str()))
        .unwrap_or("latest")
        .to_string();
    // 依赖 Node.js
    let (ok, _) = run_cmd("where", &["node"]);
    if !ok {
        return (false, "未检测到 Node.js，请先安装 Node.js (https://nodejs.org)".into(), String::new());
    }
    // 步骤0：版本对齐确认。本机已有 openclaw 且版本 ≠ 目标 → 经本机用户确认才升/降级
    // （npm install -g openclaw@<version> 会原位替换，保留 ~/.openclaw 配置/identity）。
    // 拒绝或超时就地终止，不做任何改动。探测结果写命令日志，管理员后台可见。
    let (installed, _, cur_ver, _) = openclaw_runtime();
    if installed && version != "latest" && cur_ver != version {
        let action = if cur_ver.is_empty() {
            "reinstall"
        } else if version_cmp(&cur_ver, &version) == std::cmp::Ordering::Less {
            "upgrade"
        } else {
            "downgrade"
        };
        let action_txt = if action == "upgrade" { "升级" } else if action == "downgrade" { "降级" } else { "重装" };
        let payload = serde_json::json!({
            "target": version,
            "current": cur_ver,
            "action": action,
            "found": env_report_cached()["found"],
            "note": "对齐将原位替换 npm 全局 openclaw 包（保留 ~/.openclaw 的 identity 与渠道登录态；openclaw.json 若被下发配置覆盖，会先备份为 openclaw.json.bak.<ts>）",
        });
        on_log(&format!(
            "检测到本机 openclaw {cur_ver} ≠ 目标 {version}，需{action_txt}——等待本机用户确认"
        ));
        match state.request_align_confirm(&payload, on_log) {
            Some(true) => on_log(&format!("本机用户已确认{action_txt}：{cur_ver} → {version}")),
            Some(false) => {
                return (false, format!("本机用户拒绝版本对齐（当前 {cur_ver}，目标 {version}），未做任何改动"), String::new());
            }
            None => {
                return (false, "等待本机用户确认版本对齐超时（10 分钟），未做任何改动".into(), String::new());
            }
        }
    } else if installed {
        on_log(&format!("openclaw 已是目标版本 {cur_ver}，跳过重装"));
    }
    let pkg = if version == "latest" || version.is_empty() {
        "openclaw".to_string()
    } else {
        format!("openclaw@{version}")
    };
    // 走国内镜像源，避免连不上 registry.npmjs.org（ECONNRESET）
    const MIRROR: &str = "https://registry.npmmirror.com";
    // 优先：若本服务已缓存 openclaw npm tarball（packageUrl+authToken），下载后本地安装
    if let (Some(url), Some(token)) = (
        spec.get("packageUrl").and_then(|v| v.as_str()).filter(|s| !s.is_empty()),
        spec.get("authToken").and_then(|v| v.as_str()).filter(|s| !s.is_empty()),
    ) {
        let tgz = std::env::temp_dir().join("aiarsenal-openclaw.tgz");
        on_log(&format!("从本服务下载 openclaw 安装包 {url} ..."));
        let (dok, dmsg) = download_file(url, &tgz, Some(token));
        if dok {
            on_log("$ npm install -g <本地 tarball>");
            let (ok, out) = run_cmd_streaming("npm", &["install", "-g", &tgz.display().to_string()], on_log);
            if ok {
                let _ = ensure_openclaw_config(spec);
                return (true, format!("openclaw 安装成功（本地包）\n{out}"), String::new());
            }
            on_log(&format!("本地包安装失败，回退镜像源: {out}"));
            let _ = dmsg;
        } else {
            on_log(&format!("下载本地包失败，回退镜像源: {dmsg}"));
        }
    }
    on_log(&format!("$ npm install -g {pkg} --registry={MIRROR}"));
    let (ok, out) = run_cmd_streaming("npm", &["install", "-g", &pkg, "--registry", MIRROR], on_log);
    if ok {
        let _ = ensure_openclaw_config(spec);
        (true, format!("openclaw 安装成功\n{out}"), String::new())
    } else {
        (false, format!("openclaw 安装失败: {out}"), String::new())
    }
}

pub(super) fn start_openclaw(state: &PlatformState) -> OpOut {
    // 已在跑则不重复拉起；但先核验句柄是否还活着——openclaw 自身崩溃会留 stale 句柄，
    // 不核验会误判「已在运行」导致 start/restart 不再拉起（对齐 manager：子进程退出即清句柄）。
    {
        let mut procs = state.procs.lock().unwrap();
        if let Some(child) = procs.get_mut("openclaw") {
            match child.try_wait() {
                Ok(Some(_)) => { /* 已退出，stale 句柄，落到下面清掉再 spawn */ }
                _ => return (true, "openclaw 已在运行".into(), child.id().to_string()),
            }
        }
        procs.remove("openclaw");
    }
    // 系统里已有 openclaw 进程在跑（非本 App 启的，如用户手动启的或新装 App 前就在跑）
    // 不重复拉起。但版本 ≠ 钉死目标时不能假成功——跑着的是外部/旧版 gateway，
    // 配置路由都不归本客户端管，如实报冲突引导走 install（版本对齐需本机用户确认）。
    let (_, running, cur_ver, _) = openclaw_runtime();
    if running {
        if let Some(target) = PINNED_OPENCLAW_VERSION.filter(|t| !t.is_empty()) {
            if !cur_ver.is_empty() && cur_ver != target {
                return (false, format!("openclaw gateway 已在运行但版本不符（当前 {cur_ver}，目标 {target}）；请先下发 install 完成版本对齐（需本机用户确认）"), String::new());
            }
        }
        return (true, "openclaw 已在运行（非本 App 启动）".into(), String::new());
    }
    match cmd_new("cmd")
        .args(["/C", "openclaw", "gateway", "run"])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
    {
        Ok(child) => {
            let pid = child.id().to_string();
            state.procs.lock().unwrap().insert("openclaw".into(), child);
            (true, "openclaw 已启动".into(), pid)
        }
        Err(e) => (false, format!("启动 openclaw 失败: {e}"), String::new()),
    }
}

pub(super) fn stop_openclaw(state: &PlatformState) -> OpOut {
    let mut procs = state.procs.lock().unwrap();
    if let Some(mut child) = procs.remove("openclaw") {
        let pid = child.id();
        let _ = child.kill();
        let _ = child.wait();
        return (true, "openclaw 已停止".into(), pid.to_string());
    }
    // 兜底：按进程名 kill，按返回值如实报（taskkill 找不到目标返回非 0）
    let (killed, _) = run_cmd("taskkill", &["/IM", "openclaw*", "/F"]);
    if killed {
        (true, "openclaw 已停止（按名）".into(), String::new())
    } else {
        (false, "openclaw 未在运行（taskkill 无匹配）".into(), String::new())
    }
}

pub(super) fn openclaw_runtime() -> (bool, bool, String, String) {
    let installed = which("openclaw");
    // version 只取 stdout 首行并截断 64：openclaw --version 异常时 stderr（node 报错全文）
    // 会被 run_cmd 合并进来，后端 varchar(64) 溢出（22001）。
    let version = if installed {
        let v = cmd_stdout(&openclaw_bin(), &["--version"]);
        let first = v.lines().next().unwrap_or("").trim();
        if first.chars().count() > 64 {
            first.chars().take(64).collect::<String>()
        } else {
            first.to_string()
        }
    } else {
        String::new()
    };
    // openclaw 跑成 node.exe，tasklist 按 IMAGENAME 匹配不到；按命令行含 openclaw 判断
    let (running, _) = run_cmd("powershell", &["-NoProfile", "-Command", "if ((Get-CimInstance Win32_Process -Filter \"Name='node.exe'\" | Where-Object { $_.CommandLine -like '*openclaw*' }).Count -gt 0) { exit 0 } else { exit 1 }"]);
    (installed, running && installed, version, String::new())
}

pub(super) fn ensure_openclaw_config(spec: &serde_json::Value) -> std::io::Result<()> {
    let path = config_path("openclaw").ok_or_else(|| std::io::Error::other("no path"))?;
    let tpl = spec.get("configTemplate");
    // 有 configTemplate（install 命令带了含 LLM key 的配置）→ 始终覆盖写入，确保新 key 生效；
    // 无模板 → 仅在文件不存在时写默认配置（不动用户已有配置）
    if tpl.is_none() && path.exists() {
        return Ok(());
    }
    // 覆盖前备份用户已有配置（可能是本机自装 openclaw 的自有 LLM/渠道配置）
    if tpl.is_some() && path.exists() {
        let ts = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);
        let bak = PathBuf::from(format!("{}.bak.{ts}", path.display()));
        let _ = std::fs::copy(&path, &bak);
        aiarsenal_client_core::util::log("info", &format!("[openclaw] 配置覆盖前已备份: {}", bak.display()));
    }
    let content = if let Some(t) = tpl {
        serde_json::to_string_pretty(t).unwrap_or_else(|_| "{}".into())
    } else {
        default_config("openclaw")
    };
    if let Some(p) = path.parent() {
        std::fs::create_dir_all(p)?;
    }
    std::fs::write(path, content)
}

// ==================== 环境探测（心跳 env 上报） ====================

/// 探测结果 60s 缓存：要 spawn 子进程（--version / where），心跳 10s 一跳不必每次全跑。
pub(super) fn env_report_cached() -> serde_json::Value {
    static CACHE: Mutex<Option<(std::time::Instant, serde_json::Value)>> = Mutex::new(None);
    {
        let g = CACHE.lock().unwrap();
        if let Some((t, v)) = g.as_ref() {
            if t.elapsed() < std::time::Duration::from_secs(60) {
                return v.clone();
            }
        }
    }
    let v = probe_env();
    *CACHE.lock().unwrap() = Some((std::time::Instant::now(), v.clone()));
    v
}

/// 本机 openclaw 全量探测：多来源（~/.openclaw/bin 运行时 / npm 全局）各自版本 +
/// 配置存在性 + gateway 端口占用 + 是否需对齐。经心跳 modules.openclaw.env 上报，
/// 服务端合进状态行 extra.env（管理后台设备页展示「待对齐」）。
fn probe_env() -> serde_json::Value {
    use serde_json::json;
    let target = PINNED_OPENCLAW_VERSION.unwrap_or("");
    let bin = openclaw_bin();
    let home = dirs::home_dir();
    let runtime_dir = home.as_ref().map(|h| h.join(".openclaw"));
    // openclaw_bin() 有 "openclaw" fallback：只有真指到 ~/.openclaw/bin 才算 runtime 安装
    let runtime_exists = runtime_dir
        .as_ref()
        .map(|d| d.join("bin").join("openclaw.exe").is_file())
        .unwrap_or(false);
    let runtime_version = if runtime_exists { cli_version(&bin) } else { String::new() };
    let mut found = Vec::new();
    if runtime_exists {
        found.push(json!({
            "origin": "runtime",
            "path": bin,
            "version": runtime_version,
        }));
    }
    // npm 全局（%APPDATA%\npm 或 PATH），which() 已覆盖 GUI 应用 PATH 缺 npm 目录的情况
    if which("openclaw") && !runtime_exists {
        let (_, w) = run_cmd("where", &["openclaw"]);
        let path = w.lines().next().unwrap_or("").trim().to_string();
        found.push(json!({
            "origin": "npm-global",
            "path": if path.is_empty() { "%APPDATA%\\npm".into() } else { path },
            "version": cli_version("openclaw"),
        }));
    }
    let config_exists = runtime_dir
        .as_ref()
        .map(|d| d.join("openclaw.json").is_file())
        .unwrap_or(false);
    let gw_addr: std::net::SocketAddr = "127.0.0.1:18789".parse().expect("valid addr");
    let port_alive =
        std::net::TcpStream::connect_timeout(&gw_addr, std::time::Duration::from_millis(300)).is_ok();
    let aligned = if target.is_empty() {
        serde_json::Value::Null // 未钉版本的构建无从比对，交给服务端按启用的版本判断
    } else {
        json!(runtime_exists && runtime_version == target)
    };
    json!({
        "targetVersion": target,
        "aligned": aligned,
        "found": found,
        "configExists": config_exists,
        "gatewayPortAlive": port_alive,
    })
}

/// 取 CLI 版本：stdout 首行（openclaw --version 异常时 stderr 混入，只认首行并截断）。
fn cli_version(bin: &str) -> String {
    let v = cmd_stdout(bin, &["--version"]);
    let first = v.lines().next().unwrap_or("").trim();
    if first.chars().count() > 64 {
        first.chars().take(64).collect()
    } else {
        first.to_string()
    }
}

/// 朴素的 x.y.z 版本比较（逐段数字；缺段按 0，非数字段按字典序），只用于判定升/降级文案。
fn version_cmp(a: &str, b: &str) -> std::cmp::Ordering {
    fn norm(s: &str) -> &str {
        s.trim().trim_start_matches(['v', 'V'])
    }
    let mut ia = norm(a).split('.');
    let mut ib = norm(b).split('.');
    loop {
        match (ia.next(), ib.next()) {
            (None, None) => return std::cmp::Ordering::Equal,
            (x, y) => {
                let sx = x.unwrap_or("0");
                let sy = y.unwrap_or("0");
                let ord = match (sx.parse::<u64>(), sy.parse::<u64>()) {
                    (Ok(nx), Ok(ny)) => nx.cmp(&ny),
                    _ => sx.cmp(sy),
                };
                if ord != std::cmp::Ordering::Equal {
                    return ord;
                }
            }
        }
    }
}
