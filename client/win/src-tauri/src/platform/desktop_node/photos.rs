//! photos.latest 命令（取最近 N 张照片，base64 返回，对齐 openclaw node 协议）。
//!
//! params: limit?(1,max20), maxWidth?, quality?
//! result: photos:[{format("jpg"|"png"), base64, width, height, createdAt?}]
//!
//! Windows 实现：递归扫描「图片」用户目录（dirs::picture_dir()，深度 6）里的
//! jpg/jpeg/png，按修改时间降序取前 N 张，读原文件 base64 返回（mac 的
//! requestImageDataAndOrientation 返回的也是原始图数据，口径一致）。宽高从文件头
//! 解析：JPEG 扫 SOF 段 / PNG 读 IHDR——不引图像解码依赖（image crate 的 decoder
//! 走整图解码，只为读宽高太重）。
//!
//! 与 mac 的语义差异（如实降级）：
//! - mac 的照片库（PHAsset）含 iCloud 云照片与截图/导出的全部媒体；Windows 没有
//!   系统级「相册」概念，「图片」目录就是用户照片的约定位置（含 Pictures 下的
//!   Screenshots/Camera Roll 子目录）。
//! - createdAt 用文件修改时间（Windows 文件系统没有 EXIF 拍摄时间的系统级索引）。
//! - maxWidth/quality 与 mac 同样忽略（返回原始数据）。

use base64::Engine as _;
use serde_json::{json, Value};

use super::timeutil::epoch_to_iso;

/// 扫描扩展名 → format。
fn ext_to_format(ext: &str) -> Option<&'static str> {
    match ext.to_ascii_lowercase().as_str() {
        "jpg" | "jpeg" => Some("jpg"),
        "png" => Some("png"),
        _ => None,
    }
}

pub fn latest(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_photos(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: photos（照片能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(1)
        .clamp(1, 20) as usize;
    // maxWidth/quality：返回原始图数据（不缩放），与 mac 同样忽略。

    let Some(root) = dirs::picture_dir() else {
        return (true, json!({"photos": []}));
    };
    let mut files = scan_images(&root, 0);
    // 修改时间降序（最近在前；mac 是 creationDate desc 的同位口径）
    files.sort_by(|a, b| b.1.cmp(&a.1));

    let mut photos: Vec<Value> = Vec::new();
    for (path, mtime) in files.into_iter().take(limit) {
        let Ok(data) = std::fs::read(&path) else {
            continue; // 读失败（OneDrive 占位/权限）跳过
        };
        if data.is_empty() {
            continue;
        }
        let format = ext_to_format(
            &path
                .extension()
                .and_then(|e| e.to_str())
                .unwrap_or_default(),
        )
        .unwrap_or("jpg");
        let (width, height) = match format {
            "png" => png_dims(&data),
            _ => jpeg_dims(&data),
        };
        let mut p = json!({
            "format": format,
            "base64": base64::engine::general_purpose::STANDARD.encode(&data),
            "width": width,
            "height": height,
        });
        if let Some(iso) = mtime_to_iso(mtime) {
            p["createdAt"] = json!(iso);
        }
        photos.push(p);
    }
    (true, json!({"photos": photos}))
}

// ===== 目录递归扫描 =====

type Found = (std::path::PathBuf, Option<std::time::SystemTime>);

/// 深度优先收集 jpg/jpeg/png（深度上限 6；单目录项按名字序，排序交给调用方）。
fn scan_images(dir: &std::path::Path, depth: usize) -> Vec<Found> {
    let mut out = Vec::new();
    if depth > 6 {
        return out;
    }
    let Ok(entries) = std::fs::read_dir(dir) else {
        return out;
    };
    let mut subdirs: Vec<std::path::PathBuf> = Vec::new();
    for entry in entries.flatten() {
        let path = entry.path();
        let Ok(ft) = entry.file_type() else { continue };
        if ft.is_dir() {
            subdirs.push(path);
            continue;
        }
        let ext = path
            .extension()
            .and_then(|e| e.to_str())
            .unwrap_or_default()
            .to_ascii_lowercase();
        if !matches!(ext.as_str(), "jpg" | "jpeg" | "png") {
            continue;
        }
        let mtime = entry.metadata().ok().and_then(|m| m.modified().ok());
        out.push((path, mtime));
    }
    for d in subdirs {
        out.extend(scan_images(&d, depth + 1));
    }
    out
}

fn mtime_to_iso(mtime: Option<std::time::SystemTime>) -> Option<String> {
    mtime.map(|t| {
        let secs = t
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_secs() as f64)
            .unwrap_or(0.0);
        epoch_to_iso(secs)
    })
}

// ===== 文件头宽高解析 =====

/// PNG：IHDR 在固定偏移（8 签名 + 4 长度 + 4 类型），宽 u32be @16、高 @20。
fn png_dims(data: &[u8]) -> (u64, u64) {
    if data.len() >= 24 && data[12..16] == *b"IHDR" {
        let w = u32::from_be_bytes([data[16], data[17], data[18], data[19]]);
        let h = u32::from_be_bytes([data[20], data[21], data[22], data[23]]);
        return (w as u64, h as u64);
    }
    (0, 0)
}

/// JPEG：扫 0xFF 标记段找 SOF（0xC0-0xCF，排除 C4 DHT / C8 JPG / CC DAC），
/// 高 be16 @len 后+3、宽 @+5。只读文件头区域，不整图解码。
fn jpeg_dims(data: &[u8]) -> (u64, u64) {
    if data.len() < 4 || data[0] != 0xFF || data[1] != 0xD8 {
        return (0, 0);
    }
    let mut i = 2usize;
    let head_end = data.len().min(256 * 1024); // SOF 几乎总在头部
    while i + 4 <= head_end {
        if data[i] != 0xFF {
            i += 1;
            continue;
        }
        let marker = data[i + 1];
        if marker == 0xFF {
            i += 1; // 填充字节
            continue;
        }
        if (0xD0..=0xD9).contains(&marker) || marker == 0x01 {
            i += 2; // 无长度段（RSTn/SOI/EOI/TEM）
            continue;
        }
        if i + 4 > data.len() {
            break;
        }
        let seg_len = usize::from(u16::from_be_bytes([data[i + 2], data[i + 3]]));
        let is_sof = (0xC0..=0xCF).contains(&marker) && !matches!(marker, 0xC4 | 0xC8 | 0xCC);
        if is_sof {
            if i + 9 <= data.len() {
                let h = u16::from_be_bytes([data[i + 5], data[i + 6]]);
                let w = u16::from_be_bytes([data[i + 7], data[i + 8]]);
                return (w as u64, h as u64);
            }
            break;
        }
        i += 2 + seg_len;
    }
    (0, 0)
}
