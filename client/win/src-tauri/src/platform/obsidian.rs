//! obsidian agent（原生桌面应用）——安装 / 启停 / 运行检测 / per-OS 下载地址。
//! 从 platform.rs 拆出。共享工具与类型经 `use super::*` 取用。

use super::*;

/// 按 OS 取下载地址：优先 extra.downloads.<platform>（per-OS），回退 downloadUrl。
pub(super) fn download_url_for(spec: &serde_json::Value, platform: &str) -> String {
    if let Some(url) = spec
        .get("extra")
        .and_then(|e| e.get("downloads"))
        .and_then(|d| d.get(platform))
        .and_then(|v| v.as_str())
        .filter(|s| !s.is_empty())
    {
        return url.to_string();
    }
    spec.get("downloadUrl")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string()
}

pub(super) fn install_obsidian(spec: &serde_json::Value, on_log: &(dyn Fn(&str) + Send + Sync)) -> OpOut {
    // 优先从本服务下载已缓存的安装包（packageUrl + authToken），否则按 OS 取下载地址
    let platform = if cfg!(target_os = "windows") {
        "win"
    } else {
        "mac"
    };
    let (url, token) = match spec.get("packageUrl").and_then(|v| v.as_str()) {
        Some(u) if !u.is_empty() => (u.to_string(), spec.get("authToken").and_then(|v| v.as_str()).unwrap_or("").to_string()),
        _ => (download_url_for(spec, platform), String::new()),
    };
    if url.is_empty() {
        return (false, "未配置 obsidian 下载地址（downloadUrl）".into(), String::new());
    }
    // 下载到临时目录
    let tmp = std::env::temp_dir().join("aiarsenal-obsidian-setup.exe");
    on_log(&format!("下载 {url} ..."));
    let (ok, msg) = download_file(&url, &tmp, if token.is_empty() { None } else { Some(&token) });
    if !ok {
        return (false, format!("下载 obsidian 安装包失败: {msg}"), String::new());
    }
    // 静默安装（Obsidian NSIS 支持 /S）
    on_log("静默安装（/S）...");
    match cmd_new(&tmp.display().to_string()).args(["/S"]).status() {
        Ok(s) if s.success() => (true, "obsidian 安装成功".into(), String::new()),
        Ok(s) => (false, format!("obsidian 安装异常，退出码 {}", s.code().unwrap_or(-1)), String::new()),
        Err(e) => (false, format!("运行 obsidian 安装程序失败: {e}"), String::new()),
    }
}

pub(super) fn start_obsidian() -> OpOut {
    let exe = resolve_obsidian_exe();
    match exe {
        Some(p) => match cmd_new(&p.display().to_string()).spawn() {
            Ok(child) => (true, "obsidian 已启动".into(), child.id().to_string()),
            Err(e) => (false, format!("启动 obsidian 失败: {e}"), String::new()),
        },
        None => (false, "未找到 Obsidian.exe，请先安装".into(), String::new()),
    }
}

pub(super) fn stop_obsidian() -> OpOut {
    let (ok, _) = run_cmd("taskkill", &["/IM", "Obsidian.exe", "/F"]);
    (ok, if ok { "obsidian 已停止".into() } else { "停止 obsidian 失败或未运行".into() }, String::new())
}

pub(super) fn obsidian_runtime() -> (bool, bool, String, String) {
    let exe = resolve_obsidian_exe();
    let installed = exe.is_some();
    let (running, _) = run_cmd("tasklist", &["/FI", "IMAGENAME eq Obsidian.exe", "/NH"]);
    let version = if let Some(p) = &exe {
        // 从 exe 文件版本信息取 ProductVersion
        let ps = format!("(Get-Item '{}').VersionInfo.ProductVersion", p.display());
        let v = cmd_stdout("powershell", &["-NoProfile", "-Command", &ps]);
        v.lines().next().unwrap_or("").trim().to_string()
    } else {
        String::new()
    };
    (installed, running && installed, version, String::new())
}

fn resolve_obsidian_exe() -> Option<PathBuf> {
    // 常见安装位置：%LOCALAPPDATA%\Obsidian\Obsidian.exe，%PROGRAMFILES%\Obsidian\Obsidian.exe
    let candidates: Vec<PathBuf> = [
        dirs::data_dir().map(|d| d.join("Obsidian").join("Obsidian.exe")),
        dirs::data_dir().map(|d| d.join("..").join("Local").join("Obsidian").join("Obsidian.exe")),
        std::env::var("PROGRAMFILES").ok().map(|p| PathBuf::from(p).join("Obsidian").join("Obsidian.exe")),
        std::env::var("LOCALAPPDATA").ok().map(|p| PathBuf::from(p).join("Obsidian").join("Obsidian.exe")),
    ]
    .into_iter()
    .flatten()
    .collect();
    candidates.into_iter().find(|p| p.exists())
}
