//! openclaw agent（Node CLI）——安装 / 启停 / 运行检测 / 配置 / 渠道 CLI 降级登录。
//! 从 platform.rs 拆出，避免与 hermes/obsidian 互相影响。共享工具与类型经 `use super::*` 取用。

use super::*;

/// openclaw 安装版本钉死（编译期 option_env! AA_OPENCLAW_VERSION）。Some = 强制装该版本（忽略后端 spec.version）；
/// None = 沿用后端 install 命令下发的 spec.version（默认 "latest"）。client/build.sh --openclaw-version 注入。
const PINNED_OPENCLAW_VERSION: Option<&str> = option_env!("AA_OPENCLAW_VERSION");

/// openclaw 官方生产安装脚本（内置，编译进二进制）。跑它无人值守装 runtime（~/.openclaw/bin/openclaw + Node）
/// 到 ~/.openclaw，不依赖系统 Node。App 首起检测 installed=true 会跳过 Onboarding 的 install 步骤
/// （否则 App 要 Onboarding 窗口 visible 才触发 install，client 后台 open -a 起不来会卡 5min 超时）。
const INSTALL_CLI_SH: &str = include_str!("../../resources/install-cli.sh");

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
    let mut cmd = Command::new(&openclaw_bin());
    cmd.args(["channels", "login", "--channel", channel]).env("PATH", mac_path());
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => return (false, format!("启动 channels login 失败: {e}")),
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    let state_c = state.clone();
    let channel_c = channel.to_string();
    // stderr 尾部缓冲：失败时随消息回传服务器，避免只有本地日志、服务端只见「退出非 0」无法定位根因。
    let stderr_tail = std::sync::Arc::new(std::sync::Mutex::new(Vec::<String>::new()));
    // 后台线程：读 stdout 抓 QR + 等退出。命令已 done，输出走本地日志（不进命令日志）。
    std::thread::spawn(move || {
        use std::io::{BufRead, BufReader};
        // stderr 单线程读：逐行记本地日志 + 收集尾部（最多 30 行）供失败诊断
        let st_err = state_c.clone();
        let tail_err = stderr_tail.clone();
        std::thread::spawn(move || {
            for line in BufReader::new(stderr).lines().flatten() {
                aiarsenal_client_core::util::log("info", &format!("[channel_login] stderr: {line}"));
                if let Ok(mut v) = tail_err.lock() {
                    if v.len() >= 30 {
                        v.remove(0);
                    }
                    v.push(line);
                }
            }
            drop(st_err);
        });
        let mut next_is_url = false;
        for line in BufReader::new(stdout).lines().flatten() {
            aiarsenal_client_core::util::log("info", &format!("[channel_login] {line}"));
            let trimmed = line.trim();
            // 微信插件 displayQRCode 在"你可以访问以下链接以继续："后一行打印 QR 内容 URL
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
        // channels login 退出（扫码完成/超时）
        let ok = child.wait().map(|s| s.success()).unwrap_or(false);
        aiarsenal_client_core::util::log("info", &format!("[channel_login] CLI channels login 退出 ok={ok}"));
        if ok {
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c.clone(),
                login_status: "connected".into(),
                message: "登录成功".into(),
                ..Default::default()
            }));
            // 登录成功后【不】重启 gateway。channels login 经 gateway 完成登录，gateway 随即自动
            // 拉起账号 monitor（登录会写 channelConfigUpdatedAt → 渠道热重载，不换进程；实测登录
            // 确认 1s 后 monitor started、11s 内首条消息已回复）。此前这里补 `gateway restart`
            // 会把热路径上的 gateway 连同活跃长轮询一起杀掉——服务器侧死流未超时前（实测 ~12
            // 分钟）不向新进程派发消息，登录后用户发的消息全部静默丢失
            // （实踩 2026-08-26 22:11：重启致 12 分钟盲窗、丢 2 条消息）。
        } else {
            // 失败：把 stderr 尾部带进消息回传服务器（仅本地日志时服务端只见「退出非 0」无法定位根因）
            let tail = stderr_tail
                .lock()
                .ok()
                .map(|v| v.iter().rev().take(5).rev().cloned().collect::<Vec<_>>().join("; "))
                .unwrap_or_default();
            let msg = if tail.is_empty() {
                "channels login 退出非 0".to_string()
            } else {
                format!("channels login 退出非 0｜stderr: {tail}")
            };
            set_channel_login_state(&state_c, Some(ChannelLoginState {
                channel: channel_c,
                login_status: "failed".into(),
                message: msg,
                ..Default::default()
            }));
        }
    });
    // 立即返回 done：命令本身（下发登录）完成，扫码结果异步经心跳上报
    (true, "已下发扫码登录，请扫码（二维码稍后出现在渠道绑定区）".into())
}

pub(super) fn install_openclaw(
    state: &PlatformState,
    spec: &serde_json::Value,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> OpOut {
    // 两种模式（feature openclaw-desktop 开关）：
    // - feature 启用：client 自实现 desktop node（截屏/控鼠键/TCC/node 协议），**不装 OpenClaw.app**。
    //   install-cli.sh 装 runtime → 起 gateway → 起 desktop node（步骤6）。
    // - feature 禁用（default）：装 OpenClaw.app（desktop node + 签名 TCC + Onboarding 权限引导）。
    //   预装 runtime 让 App 首起跳过 Onboarding install（否则要 Onboarding visible 才装，卡）。
    // 版本优先级：编译期钉死（AA_OPENCLAW_VERSION）> 后端 spec.version > "latest"
    let version = PINNED_OPENCLAW_VERSION
        .or_else(|| spec.get("version").and_then(|v| v.as_str()))
        .unwrap_or("latest");

    // 步骤0：环境探测 + 版本对齐确认。本机已有 openclaw runtime 且版本 ≠ 目标（无论旧新，
    // client 的 desktop node / node 协议是按钉死版本对齐实现的）→ 必须经本机用户确认才升/降级；
    // 拒绝或超时就地终止，不做任何改动。探测结果写命令日志，管理员后台可见。
    let bin = openclaw_bin();
    let runtime_exists = PathBuf::from(&bin).is_file();
    let cur_ver = if runtime_exists { cli_version(&bin) } else { String::new() };
    if runtime_exists && version != "latest" && cur_ver != version {
        let action = if cur_ver.is_empty() {
            "reinstall"
        } else if version_cmp(&cur_ver, version) == std::cmp::Ordering::Less {
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
            "note": "对齐将在原位重装 openclaw runtime（保留 ~/.openclaw 的 identity 与渠道登录态；openclaw.json 若被下发配置覆盖，会先备份为 openclaw.json.bak.<ts>）",
        });
        on_log(&format!(
            "检测到本机 openclaw runtime {cur_ver} ≠ 目标 {version}，需{action_txt}——等待本机用户确认"
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
    } else if runtime_exists {
        on_log(&format!("openclaw runtime 已是目标版本 {cur_ver}，跳过重装"));
    }

    // 步骤1：预装 runtime（install-cli.sh，自带 Node 下载，不依赖系统 Node）。失败不阻断——继续装 App，
    // App 首起也会兜底装 runtime（但需用户手动完成 Onboarding，体验差）。
    let runtime_ok = ensure_openclaw_runtime(version, on_log);
    if !runtime_ok {
        on_log("⚠️ runtime 预装未完成，继续装 App（App 首起会兜底，但需手动完成 Onboarding）");
    }

    // 步骤2（feature 禁用）：装 OpenClaw.app（dmg）。
    // feature 启用时跳过——client 自实现 desktop node（步骤6），不需 App。
    #[cfg(not(feature = "openclaw-desktop"))]
    {
        let app_path = PathBuf::from("/Applications/OpenClaw.app");
        if !app_path.is_dir() {
            let dmg = std::env::temp_dir().join(format!("OpenClaw-{version}.dmg"));
            let pkg_url = spec.get("packageUrl").and_then(|v| v.as_str()).filter(|s| !s.is_empty());
            let github_url = |ver: &str| -> Option<String> {
                if ver == "latest" || ver.is_empty() {
                    None
                } else {
                    Some(format!("https://github.com/openclaw/openclaw/releases/download/v{ver}/OpenClaw-{ver}.dmg"))
                }
            };
            let (dok, dmsg) = if let Some(u) = pkg_url {
                on_log(&format!("从本服务下载 OpenClaw.app {version} ..."));
                let (ok, msg) = download_file(u, &dmg, None);
                if !ok {
                    if let Some(gu) = github_url(version) {
                        on_log(&format!("本服务下载失败，回退 GitHub releases: {msg}"));
                        download_file(&gu, &dmg, None)
                    } else {
                        (ok, msg)
                    }
                } else {
                    (ok, msg)
                }
            } else if let Some(gu) = github_url(version) {
                on_log(&format!("下载 OpenClaw.app {version}（{gu}）..."));
                download_file(&gu, &dmg, None)
            } else {
                return (false, "未提供 packageUrl 且 version=latest，无法拼 GitHub releases URL；请先在 aa_client_package 注册 OpenClaw.app 包".into(), String::new());
            };
            if !dok {
                return (false, format!("下载 OpenClaw.app 失败: {dmsg}"), String::new());
            }
            on_log("安装 OpenClaw.app 到 /Applications（如弹授权框请输入管理员密码）");
            let mount = std::env::temp_dir().join("aiarsenal-oc-mount");
            let script = format!(
                "mkdir -p '{mp}' && hdiutil attach '{dmg}' -nobrowse -quiet -mountpoint '{mp}' && cp -R '{mp}/OpenClaw.app' '/Applications/' && hdiutil detach '{mp}' -quiet && rmdir '{mp}'",
                mp = mount.display(),
                dmg = dmg.display()
            );
            let (ok, e) = run_osascript_admin(&script);
            let _ = std::fs::remove_file(&dmg);
            if !ok {
                return (false, format!("安装 OpenClaw.app 失败: {e}"), String::new());
            }
        } else {
            on_log("OpenClaw.app 已安装，跳过下载");
        }
    }
    #[cfg(feature = "openclaw-desktop")]
    {
        on_log("openclaw-desktop feature 启用：不装 OpenClaw.app，用 client 自实现 desktop node（步骤6）");
    }

    // 步骤3：写配置（LLM key 等）
    let _ = ensure_openclaw_config(spec);

    // 步骤3.1（feature openclaw-desktop）：放开 client 自实现命令的 allowlist。
    // computer.act（控鼠键）非 gateway 标准命令；screen.record/camera.snap/clip 是 DANGEROUS、
    // canvas.* 非平台默认集，gateway 默认 allowlist 都不含。install 时整体写入（全新装、无存量
    // 可合并；存量机器由 ensure_desktop_node 的合并式 ensure_canvas_allow_commands 补齐）。
    #[cfg(feature = "openclaw-desktop")]
    {
        let extras = serde_json::to_string(&super::DESKTOP_EXTRA_ALLOW_COMMANDS).unwrap_or_default();
        let _ = run_cmd(&openclaw_bin(), &["config", "set", "gateway.nodes.allowCommands", &extras]);
    }

    // 步骤3.5：装微信插件（@tencent-weixin/openclaw-weixin）。channel_login 微信渠道依赖此插件——
    // 不装则 channels login 弹交互「安装 Weixin 插件？」（client 后台 spawn 无人交互，卡住、QR 不出）。
    if runtime_ok {
        let bin = openclaw_bin();
        if PathBuf::from(&bin).is_file() {
            on_log("安装微信插件（@tencent-weixin/openclaw-weixin）...");
            let (pok, pmsg) = run_cmd(&bin, &["plugins", "install", "@tencent-weixin/openclaw-weixin"]);
            if pok {
                on_log("微信插件已安装");
            } else {
                on_log(&format!(
                    "⚠️ 微信插件安装失败（微信扫码将不可用；可手动：openclaw plugins install @tencent-weixin/openclaw-weixin）: {}",
                    pmsg.trim()
                ));
            }
        }
    }

    // 步骤4（feature 禁用）：首起 OpenClaw.app（App 走 Onboarding 引导授权 Screen Recording/Accessibility）。
    #[cfg(not(feature = "openclaw-desktop"))]
    {
        let _ = Command::new("open").arg("-a").arg("OpenClaw").spawn();
        on_log("已启动 OpenClaw.app。请在 App 内完成 Onboarding 授权（屏幕录制/辅助功能）");
    }

    // install 不自动启动：装好 runtime + 配置 + 微信插件即结束，gateway / desktop node 交给【启动】按钮
    // （start_openclaw）显式拉起。否则 install 完就 running，启动按钮失去意义。
    // （历史：曾在此处 `gateway start` + desktop_node::start，导致安装后自动运行——已移除。）

    if runtime_ok {
        #[cfg(feature = "openclaw-desktop")]
        {
            (true, "OpenClaw 安装成功（runtime + gateway + desktop node 就绪，client 自实现 desktop）".into(), String::new())
        }
        #[cfg(not(feature = "openclaw-desktop"))]
        {
            (true, "OpenClaw 安装成功（runtime + gateway 已就绪，请在 App 完成 Onboarding 授权屏幕录制/辅助功能）".into(), String::new())
        }
    } else {
        (false, "OpenClaw runtime 预装未完成".into(), String::new())
    }
}

/// 预装 openclaw runtime：跑内置 install-cli.sh（--no-onboard，自带 Node 下载，不依赖系统 Node），
/// 装出 ~/.openclaw/bin/openclaw + tools/node。App 首起检测 installed=true 会跳过 Onboarding install 步骤。
/// spawn + 逐行读 stdout 写命令日志（install-cli.sh 耗时数分钟，持续输出刷新命令循环进度，避免 idle 超时）。
/// 版本感知：bin 存在但版本 ≠ 目标（钉死版本构建）→ 原位重装对齐（npm 覆装指定版本，
/// 保留 ~/.openclaw 的 identity / openclaw.json / 插件）；目标 latest 无法比对，沿用「存在即跳过」。
fn ensure_openclaw_runtime(version: &str, on_log: &(dyn Fn(&str) + Send + Sync)) -> bool {
    use std::io::{BufRead, BufReader};
    let home = match dirs::home_dir() {
        Some(h) => h,
        None => return false,
    };
    let prefix = home.join(".openclaw");
    let bin = prefix.join("bin").join("openclaw");
    if bin.is_file() {
        let cur = cli_version(&bin.display().to_string());
        if version == "latest" || cur == version {
            on_log("openclaw runtime 已就绪且版本一致，跳过预装");
            return true;
        }
        on_log(&format!("版本不一致（当前 {cur}，目标 {version}），原位重装对齐（保留 identity/配置）"));
    }
    let script_path = std::env::temp_dir().join("aiarsenal-install-cli.sh");
    if let Err(e) = std::fs::write(&script_path, INSTALL_CLI_SH) {
        on_log(&format!("写入 install-cli.sh 失败: {e}"));
        return false;
    }
    on_log(&format!("预装 openclaw runtime（install-cli.sh, version={version}, 含 Node 下载，可能耗时数分钟）..."));
    let mut cmd = Command::new("bash");
    cmd.arg("-c").arg(format!(
        "bash '{script}' --no-onboard --prefix '{prefix}' --version '{version}'",
        script = script_path.display(),
        prefix = prefix.display(),
        version = version,
    ));
    // npm 走国内镜像加速（install-cli.sh 内部 npm install openclaw 读 npm_config_registry）
    cmd.env("npm_config_registry", "https://registry.npmmirror.com");
    // Node 版本：install-cli.sh 默认 22.22.2，但 openclaw 2026.7.1 要求 >=22.22.3（差 0.0.1 gateway 拒起）。
    // 覆盖为 22.22.3（install-cli.sh 读 OPENCLAW_NODE_VERSION env）。
    cmd.env("OPENCLAW_NODE_VERSION", "22.22.3");
    cmd.env("PATH", mac_path());
    cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            on_log(&format!("启动 install-cli.sh 失败: {e}"));
            let _ = std::fs::remove_file(&script_path);
            return false;
        }
    };
    let stdout = child.stdout.take().expect("piped stdout");
    let stderr = child.stderr.take().expect("piped stderr");
    // stderr 后台线程写本地日志（不刷命令进度，避免噪声）
    std::thread::spawn(move || {
        for line in BufReader::new(stderr).lines().flatten() {
            aiarsenal_client_core::util::log("info", &format!("[install-cli] {line}"));
        }
    });
    // stdout 逐行写命令日志（install-cli.sh --json 进度行），刷新命令循环 last_progress_at 防 idle 超时
    for line in BufReader::new(stdout).lines().flatten() {
        on_log(&line);
    }
    let ok = child.wait().map(|s| s.success()).unwrap_or(false);
    let _ = std::fs::remove_file(&script_path);
    if !ok && !bin.is_file() {
        on_log("install-cli.sh 执行未成功且 runtime 未就绪");
        return false;
    }
    bin.is_file()
}

pub(super) fn start_openclaw(state: &PlatformState) -> OpOut {
    // gateway 由 launchd 托管（ai.openclaw.gateway）。用 openclaw CLI（~/.openclaw/bin/openclaw）
    // 起 gateway（`gateway start` 自装 plist + bootstrap），不依赖 OpenClaw.app Onboarding。
    // 先清遗留 client 句柄。
    {
        let mut procs = state.procs.lock().unwrap();
        if let Some(mut child) = procs.remove("openclaw") {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
    // feature openclaw-desktop：起 client 自实现 desktop node（连 gateway，暴露 screen.snapshot/
    // computer.act 等）。放在 gateway 状态判断【之前】：不管 gateway 是"已在跑"（早返回）还是"本次启动"，
    // desktop_node 都要起（start 幂等，已起则跳过；连不上 protocol 内部会重试，不阻塞此处返回）。
    #[cfg(feature = "openclaw-desktop")]
    {
        // AppHandle 从 PlatformState 取（setup 已回填）；异常拿不到时跳过——心跳侧
        // ensure_desktop_node 还会兜底再起，不丢功能。
        if let Some(app) = state.app.lock().unwrap().clone() {
            let (_dn_ok, dn_msg) = super::desktop_node::start(app);
            aiarsenal_client_core::util::log("info", &format!("[openclaw] desktop node: {dn_msg}"));
        }
    }
    let bin = openclaw_bin();
    if !PathBuf::from(&bin).is_file() {
        // runtime 缺失但 18789 活着 = 用户自装的 openclaw（npm 全局等）gateway 在跑——不装 runtime
        // 也没法接管端口，如实报冲突而不是盲目 gateway start（绑不上端口只会假启动）。
        if gateway_port_alive() {
            return (false, "18789 已被外部 openclaw gateway（本机自装，非本客户端管理）占用；请先下发 install 安装 runtime 并完成版本对齐".into(), String::new());
        }
        return (false, "未检测到 openclaw runtime（~/.openclaw/bin/openclaw），请先 install openclaw".into(), String::new());
    }
    let (_, running, _, _) = openclaw_runtime();
    if running {
        // 复探兜底（防停止竞态假成功）：stop 在旧 gateway 退出途中返回时（listen socket 未关），
        // 首探 port_alive 会撞上垂死进程误判「已在运行」→ 秒回假 done，随后进程退完端口关闭、
        // launchd job 已 bootout，gateway 永久下线（实踩：解绑 start 0.8s done 但实际已死，
        // 后续扫码 login 1006）。间隔 500ms 二次探测，两次都活才算真在运行；首活复死 = 垂死
        // 竞态，落到下方 gateway start 真启动。真跑着的 gateway（含外部自装）500ms 后必然仍活，
        // 行为不变；stop 核验等端口释放是第一道防线，此处兜底防其它路径的同类竞态。
        std::thread::sleep(std::time::Duration::from_millis(500));
        let (_, running2, _, _) = openclaw_runtime();
        if running2 {
            // 端口活着但版本 ≠ 钉死目标 = 外部/旧版 gateway 占着 18789。不擅杀用户进程——
            // 如实报错引导走 install（版本对齐需本机用户确认）后再启动。
            if let Some(target) = PINNED_OPENCLAW_VERSION.filter(|t| !t.is_empty()) {
                let cur = cli_version(&bin);
                if !cur.is_empty() && cur != target {
                    return (false, format!("18789 被 openclaw gateway 占用且版本不符（当前 {cur}，目标 {target}）；请先下发 install 完成版本对齐（需本机用户确认）"), String::new());
                }
            }
            return (true, "openclaw gateway 已在运行".into(), String::new());
        }
    }
    let (rc, out) = run_cmd(&bin, &["gateway", "start"]);
    if !rc {
        return (false, format!("启动 openclaw gateway 失败: {}", out.trim()), String::new());
    }
    // 验证 gateway 真的起来了（端口 18789 监听）。`gateway start` 注册 launchd 后立即返回，但 gateway
    // bootstrap 需时——实测冷启动从进程拉起到 [gateway] ready 约 13~28s（node 模块加载 + channels 预加载）。
    // 轮询最多 ~30s（连接被拒即返回，每轮 ~500ms，共 60 轮）。窗口太短会误报「未监听 18789」假失败
    // （命令判 fail 但 gateway 随后正常起来）。gateway 已在跑时首次 connect 即通过（<1s）。
    let gw_addr: std::net::SocketAddr = "127.0.0.1:18789".parse().expect("valid addr");
    let mut up = false;
    for _ in 0..60 {
        if std::net::TcpStream::connect_timeout(&gw_addr, std::time::Duration::from_millis(500)).is_ok() {
            up = true;
            break;
        }
        std::thread::sleep(std::time::Duration::from_millis(500));
    }
    if !up {
        return (false, "openclaw gateway 启动后未监听 18789（bootstrap 失败/崩溃，查 ~/.openclaw/logs 或 client [desktop_node] 日志）".into(), String::new());
    }
    (true, "openclaw gateway 已启动".into(), String::new())
}

pub(super) fn stop_openclaw(state: &PlatformState) -> OpOut {
    // feature openclaw-desktop：先停 client desktop node（再停 gateway）
    #[cfg(feature = "openclaw-desktop")]
    {
        super::desktop_node::stop();
    }

    // openclaw gateway 由 launchd 托管（ai.openclaw.gateway，KeepAlive=true）：
    //   - 单纯 pkill 会被 launchd 在 ThrottleInterval(10s) 内重新拉起；
    //   - `openclaw gateway stop` 实测不会 bootout job（进程不退出，仍同 PID）。
    // 可靠停法：launchctl bootout/unload 卸载 job，再清 Child 句柄 / pkill 兜底，
    // 最后 pgrep 轮询如实核验是否真的停了。
    let mut pid_out = String::new();
    let bin = openclaw_bin();
    // 先 openclaw gateway stop（App CLI 官方停法），再 launchctl bootout 兜底（实测 gateway stop
    // 不一定 bootout job，KeepAlive 会重拉）。
    let _ = run_cmd(&bin, &["gateway", "stop"]);

    // 1) 卸载 launchd job（KeepAlive 的根因）。优先 bootout（按 label，现代 API），
    //    失败再回退 unload（按 plist 路径）。
    let plist = std::env::var("HOME")
        .ok()
        .map(|h| format!("{h}/Library/LaunchAgents/ai.openclaw.gateway.plist"));
    let uid = run_cmd("id", &["-u"]).1.trim().to_string();
    if !uid.is_empty() {
        let _ = run_cmd("launchctl", &["bootout", &format!("gui/{uid}/ai.openclaw.gateway")]);
    }
    if let Some(p) = &plist {
        let _ = run_cmd("launchctl", &["unload", p]);
    }

    // 2) 清前台 Child 句柄（若有）
    {
        let mut procs = state.procs.lock().unwrap();
        if let Some(mut child) = procs.remove("openclaw") {
            pid_out = child.id().to_string();
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    // 3) pkill 兜底清残留前台进程
    let _ = run_cmd("pkill", &["-f", "openclaw"]);

    // 4) 如实核验：job 已 bootout 则 launchd 不会再拉起，等进程退出即可。
    //    pgrep 同样收紧到 gateway 进程特征——裸 "openclaw" 会撞上残留的 CLI 进程
    //    （--version/gateway stop 调用）误报「停不掉」。
    //    进程特征消失 ≠ 停完：gateway 优雅退出（drain/落盘）途中 listen socket 仍开着，
    //    只查 pgrep 会在退出中途就返回「已停止」，紧随的 start 撞上垂死端口误判「已在运行」
    //    假成功，随后进程退完端口关闭、launchd job 已 bootout，gateway 永久下线
    //    （实踩：解绑 stop→rm→start 串行序列，start 0.8s 假 done，后续扫码 login 1006）。
    //    故必须同时等 18789 端口释放。
    let mut dead = false;
    let mut port_lingering = false;
    for _ in 0..10 {
        std::thread::sleep(std::time::Duration::from_millis(500));
        let (still, _) = run_cmd("pgrep", &["-f", "openclaw.*gateway --port"]);
        if still {
            continue;
        }
        if gateway_port_alive() {
            port_lingering = true; // 进程已退但端口未释放（退出途中/被外部占用），继续等
            continue;
        }
        dead = true;
        break;
    }
    if dead {
        (true, "openclaw 已停止".into(), pid_out)
    } else if port_lingering {
        (false, "openclaw 进程已退但 18789 未释放（退出途中或被外部占用），稍后重试 start".into(), pid_out)
    } else {
        (false, "openclaw 停止后仍被拉起（launchd KeepAlive 未卸载）".into(), pid_out)
    }
}

pub(super) fn openclaw_runtime() -> (bool, bool, String, String) {
    // installed 只认 ~/.openclaw/bin/openclaw（install-cli.sh 装的 runtime，feature 启用/禁用都一样）。
    // 不兜底 which（PATH 残留）——uninstall 删 ~/.openclaw 后，若 PATH 有旧 npm openclaw 残留会致
    // installed 误判 true、status 卡在 stopped，前端卸载按钮一直可点（实踩：设备曾 npm 装过 openclaw）。
    let bin = openclaw_bin();
    let installed = PathBuf::from(&bin).is_file();
    // version 只取 stdout 首行并截断 64：openclaw --version 异常时会把 node 报错打到 stderr，
    // run_cmd 合并 stdout+stderr 会把多行错误塞进 version，后端 varchar(64) 溢出（22001）。
    let version = if installed {
        let v = cmd_stdout(&bin, &["--version"]);
        let first = v.lines().next().unwrap_or("").trim();
        if first.chars().count() > 64 {
            first.chars().take(64).collect::<String>()
        } else {
            first.to_string()
        }
    } else {
        String::new()
    };
    // running 优先看 gateway 端口 18789（gateway alive 的可靠标志，不受进程命令行是否含 "openclaw"
    // 影响——gateway 经 node 跑、命令行可能不含 openclaw 字样致 pgrep 漏匹配，曾致启动后 status 卡
    // stopped、前端 start 按钮仍可点）。pgrep 兜底须收紧到真实 gateway 进程特征
    // （gateway --port）——裸 "openclaw" 会误匹配 client 自己 spawn 的瞬时 CLI 进程
    // （--version 版本探测 / gateway stop 停止调用，命令行是 entry.js 不带 --port），
    // 误判 running → start 命令走"已在运行"早返回假成功（1.6s 完成，真冷启动 13~28s），
    // 实踩：19:00 下发 start 秒回 done 但 gateway 没起，状态卡 stopped。
    let gw_addr: std::net::SocketAddr = "127.0.0.1:18789".parse().expect("valid addr");
    let port_alive = std::net::TcpStream::connect_timeout(&gw_addr, std::time::Duration::from_millis(300)).is_ok();
    let (proc_running, _) = run_cmd("pgrep", &["-f", "openclaw.*gateway --port"]);
    let running = port_alive || proc_running;
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

/// 探测结果 60s 缓存：要 spawn 子进程（--version / which / defaults），心跳 10s 一跳不必每次全跑。
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

/// 本机 openclaw 全量探测：多来源（client 管理的 runtime / PATH 里的 npm 全局 / OpenClaw.app）
/// 各自的版本 + 配置存在性 + gateway 端口占用 + 是否需要版本对齐。经心跳 modules.openclaw.env
/// 上报，服务端合进状态行 extra.env（管理后台设备页展示「待对齐」）。
fn probe_env() -> serde_json::Value {
    use serde_json::json;
    let target = PINNED_OPENCLAW_VERSION.unwrap_or("");
    let bin = openclaw_bin();
    let home = dirs::home_dir();
    let runtime_dir = home.as_ref().map(|h| h.join(".openclaw"));
    let runtime_exists = PathBuf::from(&bin).is_file();
    let runtime_version = if runtime_exists { cli_version(&bin) } else { String::new() };
    let mut found = Vec::new();
    if runtime_exists {
        found.push(json!({
            "origin": "runtime",
            "path": bin,
            "version": runtime_version,
        }));
    }
    // PATH 里的 openclaw（用户 npm -g 自装等），排除我们管理的 ~/.openclaw 下的路径
    let (_, w) = run_cmd("which", &["openclaw"]);
    let which_path = w.trim().to_string();
    let is_ours = runtime_dir
        .as_ref()
        .map(|d| std::path::Path::new(&which_path).starts_with(d))
        .unwrap_or(false);
    if !which_path.is_empty() && !is_ours {
        found.push(json!({
            "origin": "npm-global",
            "path": which_path,
            "version": cli_version(&which_path),
        }));
    }
    let app_dir = PathBuf::from("/Applications/OpenClaw.app");
    if app_dir.is_dir() {
        let v = cmd_stdout(
            "defaults",
            &["read", "/Applications/OpenClaw.app/Info.plist", "CFBundleShortVersionString"],
        );
        found.push(json!({
            "origin": "app",
            "path": "/Applications/OpenClaw.app",
            "version": v,
        }));
    }
    let config_exists = runtime_dir
        .as_ref()
        .map(|d| d.join("openclaw.json").is_file())
        .unwrap_or(false);
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
        "gatewayPortAlive": gateway_port_alive(),
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

/// 18789 gateway 端口是否被监听（不区分归属，归属甄别在 install/start 流程做）。
fn gateway_port_alive() -> bool {
    let addr: std::net::SocketAddr = "127.0.0.1:18789".parse().expect("valid addr");
    std::net::TcpStream::connect_timeout(&addr, std::time::Duration::from_millis(300)).is_ok()
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

