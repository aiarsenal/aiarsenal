//! hermes agent（原生 Python，公网包安装，无 Docker）——安装 / 启停 / 运行检测。
//! 从 platform.rs 拆出。共享工具与类型经 `use super::*` 取用。

use super::*;

// HERMES_HOME：与官方 install.sh 默认一致（~/.hermes）。
pub(super) fn hermes_home() -> PathBuf {
    std::env::var("HERMES_HOME")
        .ok()
        .map(PathBuf::from)
        .or_else(|| dirs::home_dir().map(|h| h.join(".hermes")))
        .unwrap_or_else(|| PathBuf::from("."))
}

// hermes CLI 入口：官方 install.sh 把 venv 建在 $HERMES_HOME/hermes-agent/venv/bin/hermes
// （scripts/install.sh:1628 `HERMES_BIN="$INSTALL_DIR/venv/bin/hermes"`，目录名是 venv 无点）。
// 先前写死 .venv（有点）→ hermes_runtime() 查不到入口、误报 notinstalled。
// 这里优先认官方现行 venv，回退 .venv（历史/其它装法），都没有则回退 venv 路径让调用方据 exists() 报未装。
pub(super) fn hermes_binary() -> PathBuf {
    let base = hermes_home().join("hermes-agent");
    let canonical = base.join("venv").join("bin").join("hermes");
    if canonical.exists() {
        return canonical;
    }
    let legacy = base.join(".venv").join("bin").join("hermes");
    if legacy.exists() {
        return legacy;
    }
    canonical
}

// hermes venv 的 python：与 hermes 入口同目录（venv/bin/python）。shim 调 qr_login 用。
pub(super) fn venv_python() -> PathBuf {
    hermes_binary()
        .parent()
        .map(|d| d.join("python"))
        .unwrap_or_else(|| PathBuf::from("python"))
}

// hermes 源码根（含 gateway/ 包，shim 据此 import）。入口 venv/bin/hermes → 上溯三级得 hermes-agent/；
// 不含 gateway/ 时回退 hermes_home/hermes-agent。
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

// hermes 托管的 uv：官方 install.sh 装在 $HERMES_HOME/bin/uv（install.sh:558 managed uv）；
// 不在则回退 PATH 上的 uv（mac_path 已补 homebrew 等）。
pub(super) fn hermes_uv() -> String {
    let uv = hermes_home().join("bin").join("uv");
    if uv.exists() {
        uv.display().to_string()
    } else {
        "uv".to_string()
    }
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

// ===== 源码 zip 预置（国内网络优化，2026-09 验证）=====
//
// 官方 install.sh 的 clone_repo 只认两种状态：目录不存在 → git clone github.com（国内常不可达）；
// 已是 git 仓库 → update 路径（git fetch origin <branch>）。zip 源码包预置法：把发布 tag 的
// 源码 zip 解压后 init 成 git 仓库 + 建本地 bare 仓库当 origin，install.sh 即走 update 路径
// fetch 本地 origin —— 全程不碰 github.com（clone_repo 对「目录在但非 git 仓库」直接 exit 1，
// 所以必须先预置成 git 仓库再跑脚本）。zip 下载走镜像列表加速 + 直连兜底，缓存复用（重装免下载）。

// 预置的 hermes 版本（GitHub release tag）。升级 = 改这里 + 清 .aa-cache（或换 tag 文件名自然失效）。
const HERMES_SOURCE_TAG: &str = "v2026.9.14";
// 源码 zip 镜像前缀（GitHub 加速站，前缀 + 官方 archive 路径）。速度实测 gh.ddlc.top 远快于直连。
const HERMES_SOURCE_ZIP_MIRRORS: &[&str] = &[
    "https://gh.ddlc.top/https://github.com",
    "https://gh-proxy.com/https://github.com",
];
const HERMES_SOURCE_ZIP_DIRECT: &str = "https://github.com";

// hermes_source_zip_urls：候选下载地址。后端 spec 显式指定（downloadUrl / extra.sourceZipUrl）
// 时只按它来（管理员可控发布渠道）；否则镜像 + 直连兜底拼官方 archive 地址。
fn hermes_source_zip_urls(spec: &serde_json::Value) -> Vec<String> {
    let explicit = spec
        .get("downloadUrl")
        .and_then(|v| v.as_str())
        .filter(|s| !s.is_empty())
        .or_else(|| {
            spec.get("extra")
                .and_then(|e| e.get("sourceZipUrl"))
                .and_then(|v| v.as_str())
                .filter(|s| !s.is_empty())
        });
    if let Some(u) = explicit {
        return vec![u.to_string()];
    }
    let path = format!("/NousResearch/hermes-agent/archive/refs/tags/{HERMES_SOURCE_TAG}.zip");
    let mut urls: Vec<String> = HERMES_SOURCE_ZIP_MIRRORS.iter().map(|m| format!("{m}{path}")).collect();
    urls.push(format!("{HERMES_SOURCE_ZIP_DIRECT}{path}"));
    urls
}

// download_hermes_zip：依次尝试候选地址下载 zip（>1MB 视为有效，官方包 ~75MB；空文件/HTML 错误页剔除）。
fn download_hermes_zip(zip: &PathBuf, urls: &[String], on_log: &(dyn Fn(&str) + Send + Sync)) -> Result<(), String> {
    for url in urls {
        on_log(&format!("$ curl -fLsS --connect-timeout 20 --retry 2 -o hermes-src.zip {url}"));
        let (ok, out) = run_cmd_streaming(
            "curl",
            &[
                "-fLsS",
                "--connect-timeout",
                "20",
                "--retry",
                "2",
                "-o",
                &zip.display().to_string(),
                url,
            ],
            on_log,
        );
        let size_ok = std::fs::metadata(zip).map(|m| m.len() > 1_000_000).unwrap_or(false);
        if ok && size_ok {
            on_log("源码包下载完成");
            return Ok(());
        }
        let _ = std::fs::remove_file(zip); // 失败/不完整 → 清掉试下一个源
        on_log(&format!("下载失败（{out}），尝试下一个源…"));
    }
    Err(format!("hermes 源码包下载失败（已试 {} 个源）", urls.len()))
}

// unpack_and_seed_git：解压 zip → git init + 全量提交 + 本地 bare origin → 挪到 hermes-agent/。
// install_dir 已是 git 仓库时由调用方短路；存在但非 git 仓库（上次异常残留）挪走备份不删。
fn unpack_and_seed_git(
    zip: &PathBuf,
    install_dir: &PathBuf,
    origin: &PathBuf,
    cache: &PathBuf,
    on_log: &(dyn Fn(&str) + Send + Sync),
) -> Result<(), String> {
    let tmp = cache.join("src-unpack");
    let _ = std::fs::remove_dir_all(&tmp);
    std::fs::create_dir_all(&tmp).map_err(|e| format!("创建解压目录失败: {e}"))?;
    let (ok, out) = run_cmd_streaming(
        "unzip",
        &["-q", &zip.display().to_string(), "-d", &tmp.display().to_string()],
        on_log,
    );
    if !ok {
        return Err(format!("解压失败: {out}"));
    }
    // GitHub archive 顶层固定是 hermes-agent-<ver>/；防御性取第一个目录项
    let root = std::fs::read_dir(&tmp)
        .ok()
        .and_then(|mut it| it.next())
        .and_then(|r| r.ok())
        .map(|e| e.path())
        .ok_or_else(|| "解压结果为空".to_string())?;
    if !root.is_dir() {
        return Err("解压结果缺少源码根目录".into());
    }
    // git 仓库化：main 分支 + 单提交。author 用 -c 注入（不动全局 git 配置）。
    let gca = ["-c", "user.name=aiarsenal-client", "-c", "user.email=client@aiarsenal.cc"];
    for args in [
        vec!["init", "-q", "-b", "main"],
        vec![gca[0], gca[1], gca[2], gca[3], "add", "-A"],
        vec![gca[0], gca[1], gca[2], gca[3], "commit", "-qm", &format!("seed {HERMES_SOURCE_TAG}")],
    ] {
        let (ok, out) = run_cmd_streaming_dir("git", &args, &root, on_log);
        if !ok {
            return Err(format!("git 预置失败（{}）: {out}", args.join(" ")));
        }
    }
    // 本地 bare origin（install.sh update 路径 fetch 它；放 HERMES_HOME 下随卸载清理）
    let _ = std::fs::remove_dir_all(origin);
    let (ok, out) = run_cmd_streaming(
        "git",
        &["clone", "-q", "--bare", &root.display().to_string(), &origin.display().to_string()],
        on_log,
    );
    if !ok {
        return Err(format!("建本地 origin 失败: {out}"));
    }
    let (ok, out) = run_cmd_streaming_dir(
        "git",
        &["remote", "add", "origin", &origin.display().to_string()],
        &root,
        on_log,
    );
    if !ok {
        return Err(format!("挂 origin 失败: {out}"));
    }
    // 就位：非 git 的旧目录（异常残留）挪走备份；同卷 rename，跨卷回退 move_dir
    if install_dir.exists() && !install_dir.join(".git").is_dir() {
        let bak = cache.join(format!("hermes-agent.broken-{}", chrono_like_ts()));
        std::fs::rename(install_dir, &bak).map_err(|e| format!("挪走异常残留目录失败: {e}"))?;
        on_log(&format!("旧的非 git 目录已备份到 {}", bak.display()));
    }
    if let Err(e) = std::fs::rename(&root, install_dir) {
        aiarsenal_client_core::skills::move_dir(&root, install_dir).map_err(|e2| format!("挪到安装目录失败: {e} / {e2}"))?;
    }
    Ok(())
}

// chrono_like_ts：本地时间戳（备份目录名用），不引 chrono 依赖。
fn chrono_like_ts() -> String {
    let secs = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    format!("{secs}")
}

// seed_hermes_source：确保 hermes-agent/ 是「zip 内容的 git 仓库 + 本地 origin」。
// 已是 git 仓库（旧官方安装/上次预置）→ 跳过（重装/升级由 install.sh update 路径处理）。
fn seed_hermes_source(spec: &serde_json::Value, on_log: &(dyn Fn(&str) + Send + Sync)) -> Result<(), String> {
    let home = hermes_home();
    let install_dir = home.join("hermes-agent");
    if install_dir.join(".git").is_dir() {
        on_log("hermes 源码已就位（git 仓库），跳过 zip 预置");
        return Ok(());
    }
    let cache = home.join(".aa-cache");
    std::fs::create_dir_all(&cache).map_err(|e| format!("创建缓存目录失败: {e}"))?;
    let zip = cache.join(format!("hermes-src-{HERMES_SOURCE_TAG}.zip"));
    let origin = home.join(".hermes-origin.git");
    let urls = hermes_source_zip_urls(spec);
    if !zip.exists() {
        download_hermes_zip(&zip, &urls, on_log)?;
    }
    match unpack_and_seed_git(&zip, &install_dir, &origin, &cache, on_log) {
        Ok(()) => Ok(()),
        Err(e) => {
            // 缓存的 zip 可能损坏（上次下载中断）→ 删掉重下再试一轮
            let _ = std::fs::remove_file(&zip);
            on_log(&format!("预置失败（{e}），删除缓存重新下载…"));
            download_hermes_zip(&zip, &urls, on_log)?;
            unpack_and_seed_git(&zip, &install_dir, &origin, &cache, on_log)
        }
    }
}

pub(super) fn install_hermes(spec: &serde_json::Value, on_log: &(dyn Fn(&str) + Send + Sync)) -> OpOut {
    // 0) 源码 zip 预置：官方 install.sh 首装要 git clone github.com（部分网络不可达），
    //    预置成「git 仓库 + 本地 origin」后脚本走 update 路径，全程不碰 github。
    if let Err(e) = seed_hermes_source(spec, on_log) {
        return (false, format!("hermes 源码预置失败: {e}"), String::new());
    }
    // 1) 官方原生安装脚本：自动 bootstrap uv + 装 hermes + 配 HERMES_HOME。
    //    --skip-setup：向导读 /dev/tty，App 无法应答（无终端时脚本也自跳，显式传防终端环境误入向导）。
    //    spec.envVars 注入子进程：UV_DEFAULT_INDEX 走国内 PyPI 镜像等由后端版本规格下发。
    let url = "https://hermes-agent.nousresearch.com/install.sh";
    let script = hermes_home().join(".aa-hermes-install.sh");
    on_log(&format!("$ curl -fsSL {url}"));
    let (ok, out) = run_cmd_streaming("curl", &["-fsSL", "-o", &script.display().to_string(), url], on_log);
    if !ok {
        return (false, format!("下载 install.sh 失败: {out}"), String::new());
    }
    let home_str = hermes_home().display().to_string();
    let script_str = script.display().to_string();
    let mut envs: Vec<(String, String)> = vec![
        ("HERMES_HOME".into(), home_str.clone()),
        ("AA_HERMES_SOURCE_TAG".into(), HERMES_SOURCE_TAG.into()),
    ];
    if let Some(map) = spec.get("envVars").and_then(|v| v.as_object()) {
        for (k, v) in map {
            if let Some(v) = v.as_str() {
                envs.push((k.clone(), v.to_string()));
            }
        }
    }
    let env_refs: Vec<(&str, &str)> = envs.iter().map(|(k, v)| (k.as_str(), v.as_str())).collect();
    let (ok, out) = run_cmd_streaming_env("bash", &[&script_str, "--skip-setup"], &env_refs, on_log);
    if !ok {
        return (false, format!("hermes 安装失败: {out}"), String::new());
    }
    // 2) 追加 messaging extra（aiohttp/qrcode）：官方 installer 装 .[all] 不含 messaging
    //    （[all] 自 2026-05-12 剔除 messaging 改 lazy-install，而 lazy-install 无 platform.weixin 条目），
    //    不装则微信扫码直接 raise "aiohttp is required"。失败仅告警不阻断（可重试）。
    on_log("安装 messaging 扩展（aiohttp/qrcode，微信扫码依赖）…");
    let py = venv_python().display().to_string();
    let (ok2, out2) = run_cmd_streaming_dir(
        &hermes_uv(),
        &["pip", "install", "--python", &py, "-e", ".[messaging]"],
        &hermes_source_dir(),
        on_log,
    );
    if !ok2 {
        on_log(&format!("⚠️ messaging 扩展安装失败（微信扫码会报 aiohttp missing，可重装）：{out2}"));
    }
    // 3) 建 config.base.yaml 基线（write_effective_config 按绑定态在其上叠加 weixin 块）。
    //    base 不存在时从 installer 生成的 config.yaml 快照（一次性，不丢 model 配置）。
    let base = hermes_base_config_path();
    if !base.exists() {
        if let Ok(content) = std::fs::read(hermes_config_path()) {
            let _ = std::fs::write(&base, content);
        }
    }
    // 4) 清官方技能库：install.sh 把源码 skills/ 整库（12 分类 58 个英文手册：apple/research/…）
    //    拷进 $HERMES_HOME/skills，设备「技能管理」会报一大堆外文技能。按源码树顶层条目名对删
    //    ——平台下发（aiarsenal-*）/用户自装的目录不在源码树里，天然保留。
    remove_official_skill_library(on_log);
    (true, format!("hermes 安装成功（源码 {HERMES_SOURCE_TAG}）\n{out}"), String::new())
}

// remove_official_skill_library：删 $HERMES_HOME/skills 下与源码树 skills/ 顶层同名的条目
// （官方库按分类目录整体拷入；重装/升级后 installer 会重新拷回，故每次 install 后都清）。
// 源码 skills/ 不存在（异常装法）时无操作。
fn remove_official_skill_library(on_log: &(dyn Fn(&str) + Send + Sync)) {
    let src = hermes_source_dir().join("skills");
    let home_skills = hermes_home().join("skills");
    let Ok(entries) = std::fs::read_dir(&src) else {
        return;
    };
    let mut removed = 0usize;
    for en in entries.flatten() {
        let name = en.file_name();
        let target = home_skills.join(&name);
        if !target.exists() {
            continue; // 源树有、home 没有（上次已清/装法差异）
        }
        let ok = if en.path().is_dir() {
            std::fs::remove_dir_all(&target).is_ok()
        } else {
            std::fs::remove_file(&target).is_ok()
        };
        if ok {
            removed += 1;
        } else {
            on_log(&format!("⚠️ 清理官方技能 {} 失败（不影响安装）", name.to_string_lossy()));
        }
    }
    if removed > 0 {
        on_log(&format!("已清理 hermes 官方技能库 {removed} 个分类目录（平台下发技能保留）"));
    }
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
    let mut cmd = Command::new(&bin);
    cmd.args(["gateway", "run", "--no-supervise", "--external-supervisor"])
        .env("HERMES_HOME", &home)
        .env("PATH", mac_path())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    match cmd.spawn() {
        Ok(child) => {
            let pid = child.id().to_string();
            state.procs.lock().unwrap().insert("hermes".into(), child);
            // 验证 gateway 真的起来了（control socket identify 应答）。此前 spawn 完即报
            // 成功——gateway 依赖缺失/配置坏时 boot 阶段就崩，命令照样 done（假成功）。
            // Python 冷启动慢（Electron 桌面端给 90s 窗口），轮询 60s（500ms×120 轮）；
            // 超时如实报未就绪，子进程不杀（可能仍在起，稍后心跳侧能看到）。
            for _ in 0..120 {
                if gateway_identify().is_some() {
                    return (true, "hermes 已启动（gateway control socket 已应答）".into(), pid);
                }
                std::thread::sleep(std::time::Duration::from_millis(500));
            }
            (false, "hermes 启动后 60s 内 control socket 未应答（bootstrap 失败/依赖缺失，查 ~/.hermes/logs）".into(), pid)
        }
        Err(e) => (false, format!("启动 hermes 失败: {e}"), String::new()),
    }
}

pub(super) fn stop_hermes(state: &PlatformState) -> OpOut {
    // 优先 hermes gateway stop（读 gateway.pid 发 SIGTERM），回退杀本 App 持有的子进程 + pkill。
    // 最后核验（对齐 openclaw stop 的如实核验，此前无条件报成功）：control socket identify
    // 失败 且 ps 扫描无 gateway run 进程才算停。gateway 优雅退出会自删 socket；被杀则 socket
    // 文件残留但连接被拒——identify 失败即视为不在服务。
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
    let _ = run_cmd("pkill", &["-f", "hermes gateway"]);
    // 核验轮询：10 轮 × 500ms。进程判定用 hermes_gateway_process（特征 "gateway run"）：
    // 不匹配刚 spawn 的 `hermes gateway stop` CLI 瞬态进程，也不受并发 pgrep 互相误报影响。
    for _ in 0..10 {
        std::thread::sleep(std::time::Duration::from_millis(500));
        if gateway_identify().is_some() {
            continue; // socket 仍应答 = gateway 还活着
        }
        if hermes_gateway_process().is_some() {
            continue;
        }
        return (true, "hermes 已停止".into(), pid_out);
    }
    // 升级 SIGKILL：SIGTERM 后 gateway 优雅退出可能极慢（实测 v0.21.3 关闭 forensics 在
    // 孤儿进程场景 RecursionError 风暴，7 分钟+ 才退；App 直挂子进程已在上面 SIGKILL，
    // 这里兜底非 App 拉起的场景）。-9 后再核 6 轮，仍活着才报失败。
    let _ = run_cmd("pkill", &["-9", "-f", "hermes gateway"]);
    for _ in 0..6 {
        std::thread::sleep(std::time::Duration::from_millis(500));
        if gateway_identify().is_none() && hermes_gateway_process().is_none() {
            return (true, "hermes 已停止（SIGKILL 兜底）".into(), pid_out);
        }
    }
    (false, "hermes 停止后 gateway 仍存活（control socket 或进程仍在，查 ~/.hermes/logs）".into(), pid_out)
}

// ===== gateway control socket（活性/身份探测，对齐 openclaw 的 18789 端口探测）=====
//
// hermes gateway 启动时在 $HERMES_HOME/gateway.sock 建 Unix socket（gateway/control_socket.py），
// 单连接单 JSON 行协议：请求 {"verb":"identify","id":1,"protocol":1}\n，响应
// {"ok":true,"result":{pid,code_version,supervisor,...}}。home 路径超 sun_path(104B) 时
// socket 挪系统 tmp、$HERMES_HOME/gateway.sock.path 指针文件记真实位置——客户端跟指针。
// 「能连上且 identify 应答合法」即活性（源码注释原话），比 pgrep 进程名匹配可靠，
// 且顺带回填真 pid（此前心跳 pid 字段恒空）。旧版/用户自装 hermes 无 socket → None，
// 调用方降级 pgrep（行为不劣于改造前）。

/// gateway_identify：连 control socket 发 identify。socket 不在/连接被拒/超时/应答非法 → None。
pub(super) fn gateway_identify() -> Option<serde_json::Value> {
    use std::io::{Read, Write};
    use std::os::unix::net::UnixStream;
    let home = hermes_home();
    let mut sock_path = home.join("gateway.sock");
    if !sock_path.exists() {
        // 指针文件兜底：内容是真实 socket 绝对路径（首行）
        let target = std::fs::read_to_string(home.join("gateway.sock.path"))
            .ok()?
            .trim()
            .to_string();
        if target.is_empty() || target.contains('\n') {
            return None;
        }
        sock_path = PathBuf::from(target);
    }
    let mut stream = match UnixStream::connect(&sock_path) {
        Ok(s) => s,
        Err(_) => return None,
    };
    // 本地 IPC，正常应答毫秒级；1s 足够宽松（Python 客户端默认 2s）
    let _ = stream.set_read_timeout(Some(std::time::Duration::from_secs(1)));
    let _ = stream.set_write_timeout(Some(std::time::Duration::from_secs(1)));
    if stream.write_all(br#"{"verb":"identify","id":1,"protocol":1}"#).is_err()
        || stream.write_all(b"\n").is_err()
    {
        return None;
    }
    let mut buf: Vec<u8> = Vec::new();
    let mut chunk = [0u8; 8192];
    loop {
        let n = match stream.read(&mut chunk) {
            Ok(n) => n,
            Err(_) => return None,
        };
        if n == 0 {
            break; // 服务端答完关连接
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

// hermes_gateway_process：全量 ps 扫描找 gateway run 进程（socket identify 失败时的兜底）。
// 不能用 pgrep -f "hermes gateway"：pgrep 只排除自身 pid，而心跳线程与命令执行线程会并发
// 调 hermes_runtime（--version 是秒级 Python 冷启动，两线程在其中停留数秒），各自 spawn 的
// pgrep 命令行都含特征 → 互相命中 → running 误报 true（start 误判「已在运行」，实测复现
// 100% 命中）。改 ps ax：ps 自身命令行不含特征、过滤在 Rust 侧做，无任何自匹配可能。
// 特征用 "hermes gateway run"（gateway run 命令行稳定含它）：排除 "hermes gateway stop"
// CLI 瞬态进程与 "hermes --version" 版本探测进程。顺带回填 pid（旧 pgrep 兜底路径 pid 恒空）。
fn hermes_gateway_process() -> Option<u32> {
    let out = Command::new("ps")
        .args(["ax", "-o", "pid=,args="])
        .env("PATH", mac_path())
        .output()
        .ok()?;
    let text = String::from_utf8_lossy(&out.stdout);
    for line in text.lines() {
        let line = line.trim_start();
        let Some((pid_s, args)) = line.split_once(char::is_whitespace) else {
            continue;
        };
        if !args.contains("hermes gateway run") {
            continue;
        }
        if let Ok(p) = pid_s.trim().parse::<u32>() {
            if p != std::process::id() {
                return Some(p);
            }
        }
    }
    None
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
    // running 以 control socket identify 为准（对齐 openclaw 端口探测优先），顺带回填真 pid；
    // identify 不通（未跑/旧版无 socket）→ ps 扫描兜底（hermes_gateway_process，勿换 pgrep，
    // 见其注释的并发自匹配竞态）。
    let mut pid = String::new();
    let mut running = false;
    if installed {
        if let Some(id) = gateway_identify() {
            running = true;
            if let Some(p) = id.get("pid").and_then(|v| v.as_u64()) {
                pid = p.to_string();
            }
        } else if let Some(p) = hermes_gateway_process() {
            running = true;
            pid = p.to_string();
        }
    }
    (installed, running && installed, version, pid)
}

// ===== 技能装/卸（skill_install / skill_uninstall，平台层公共「下载 zip→解压→定位根」后调用）=====
//
// hermes 不走 `hermes skills install` CLI：其 identifier 只认 hub 名/HTTP URL（skills_hub.py
// 源适配器），平台包是设备 token 下载的本地 zip；而 hermes 的技能枚举（tools/skills_tool
// _find_all_skills → iter_skill_index_files）是全树扫盘，lockfile 只标注来源/trust——直接
// 落盘 $HERMES_HOME/skills/<slug>/ 即被识别为 local skill（无 trust 要求：trust 只管仓库内
// 项目级技能），且扫描缓存按目录 mtime 失效，落盘即生效无需重启 gateway。卸载=删目录。

// skill_install：解压出的技能根 root 落盘到 $HERMES_HOME/skills/<slug>/。
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
// 打印 AA_WEIXIN_LOGIN_OK 哨兵；失败打印 AA_WEIXIN_LOGIN_FAILED。逐字移植自 image/manager/hermes.go。
const WEIXIN_LOGIN_SHIM_PY: &str = "#!/usr/bin/env python3\n# aiarsenal 微信扫码登录 shim（由桌面 App 调用）。\nimport asyncio, os\n\n\ndef main():\n    try:\n        from gateway.platforms.weixin import qr_login\n    except Exception as exc:\n        print(\"AA_WEIXIN_LOGIN_FAILED import: %s\" % exc)\n        return\n    home = os.environ.get(\"HERMES_HOME\") or os.path.expanduser(\"~/.hermes\")\n    # qr_login 内部：打印二维码 URL（http liteapp）到 stdout + save_weixin_account 落账号 json。\n    creds = asyncio.run(qr_login(home))\n    if not creds:\n        print(\"AA_WEIXIN_LOGIN_FAILED\")\n        return\n    # 合并写 .env：去旧 WEIXIN_* 行，追加新凭据（gateway 启动经 get_secret 解析）。\n    env_path = os.path.join(home, \".env\")\n    kept = []\n    try:\n        with open(env_path) as f:\n            for ln in f:\n                if not ln.startswith(\"WEIXIN_\"):\n                    kept.append(ln.rstrip(\"\\n\"))\n    except FileNotFoundError:\n        pass\n    rows = {\n        \"WEIXIN_ACCOUNT_ID\": creds.get(\"account_id\", \"\"),\n        \"WEIXIN_TOKEN\": creds.get(\"token\", \"\"),\n        \"WEIXIN_BASE_URL\": creds.get(\"base_url\", \"\"),\n    }\n    out = list(kept)\n    for k, v in rows.items():\n        if v:\n            out.append(\"%s=%s\" % (k, v))\n    with open(env_path, \"w\") as f:\n        f.write(\"\\n\".join(out) + \"\\n\")\n    try:\n        os.chmod(env_path, 0o600)\n    except OSError:\n        pass\n    print(\"AA_WEIXIN_LOGIN_OK\")\n\n\nmain()\n";

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
        // base 与 weixin 块之间必须留换行：trim 掉 base 末尾换行后直接拼 gateway: 会粘到
        // 上一行（如 default: "..."gateway:）→ YAML 损坏 → hermes 判 corrupt 丢弃 model 段 →
        // 凭据回退「Provider authentication failed」。故显式补一个 \n 分隔。
        out = format!("{}\n{}", out.trim_end_matches('\n'), WEIXIN_OPEN_PLATFORM_YAML);
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
    let mut cmd = Command::new(&py);
    cmd.arg(&shim)
        .current_dir(&src)
        .env("HERMES_HOME", hermes_home())
        .env("PYTHONPATH", &src)
        .env("PATH", mac_path())
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

