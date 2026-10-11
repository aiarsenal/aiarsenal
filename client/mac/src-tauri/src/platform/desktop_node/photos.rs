//! photos.latest 命令（取相册最近 N 张照片，base64 返回，对齐 openclaw node 协议）。
//!
//! params: limit?(1,max20), maxWidth?, quality?
//! result: photos:[{format("jpg"|"png"), base64, width, height, createdAt?}]
//!
//! PHAsset.fetchAssetsWithMediaType(Image) + sortDescriptors(creationDate desc) + fetchLimit →
//! PHImageManager.requestImageDataAndOrientationForAsset block（*mut NSData 等）→ block 内同步
//! base64 编码（避免 retain NSData 裸指针）→ mpsc channel 传回。

#![allow(non_snake_case)]

use std::sync::mpsc;
use std::time::Duration;

use block2::RcBlock;
use objc2::rc::Retained;
use objc2_foundation::{
    NSArray, NSData, NSDataBase64EncodingOptions, NSDictionary, NSString, NSSortDescriptor,
};
use objc2_image_io::CGImagePropertyOrientation;
use objc2_photos::{
    PHAsset, PHAssetMediaType, PHFetchOptions, PHImageManager,
};
use serde_json::{json, Value};

use super::timeutil::epoch_to_iso;

/// dataUTI（"public.jpeg"/"public.png"）→ "jpg"/"png"；未知默认 jpg（多数相册照片是 jpeg）。
fn uti_to_format(uti: &str) -> &'static str {
    let u = uti.to_lowercase();
    if u.contains("jpeg") || u.contains("jpg") {
        "jpg"
    } else if u.contains("png") {
        "png"
    } else {
        "jpg"
    }
}

/// 请求单张 asset 原始图数据（同步等 block 回调，10s 超时）。返回 (base64 字符串, dataUTI)。
/// block 内直接 base64 编码发 String —— 避免对 *mut NSData 做 Retained::retain 的裸指针摩擦。
fn request_image_data(manager: &PHImageManager, asset: &PHAsset) -> (String, String) {
    let (tx, rx) = mpsc::channel::<(String, String)>();
    let block = RcBlock::new(
        move |data: *mut NSData,
              uti: *mut NSString,
              _ori: CGImagePropertyOrientation,
              _info: *mut NSDictionary| {
            if data.is_null() {
                let _ = tx.send((String::new(), String::new()));
                return;
            }
            let d = unsafe { &*data };
            let b64: Retained<NSString> =
                d.base64EncodedStringWithOptions(NSDataBase64EncodingOptions::empty());
            let uti_s = if uti.is_null() {
                String::new()
            } else {
                unsafe { &*uti }.to_string()
            };
            let _ = tx.send((b64.to_string(), uti_s));
        },
    );
    let _ = unsafe {
        manager.requestImageDataAndOrientationForAsset_options_resultHandler(asset, None, &block)
    };
    rx.recv_timeout(Duration::from_secs(10))
        .unwrap_or((String::new(), String::new()))
}

pub fn latest(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_photos(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: photos（请到 系统设置→隐私与安全→照片 勾选本 App）"}}),
        );
    }
    let limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(1)
        .clamp(1, 20) as usize;
    // maxWidth/quality：PHImageManager 返原始图数据（不缩放），桌面端暂忽略（与 plan「可选」一致）。

    // fetch：creationDate 降序（最近在前）+ fetchLimit
    let sort = NSSortDescriptor::sortDescriptorWithKey_ascending(
        Some(&NSString::from_str("creationDate")),
        false,
    );
    let descriptors = NSArray::from_slice(&[&*sort]);
    let options = unsafe { PHFetchOptions::new() };
    unsafe {
        options.setSortDescriptors(Some(&descriptors));
        options.setFetchLimit(limit);
    }
    let fetch: Retained<objc2_photos::PHFetchResult<PHAsset>> = unsafe {
        PHAsset::fetchAssetsWithMediaType_options(PHAssetMediaType::Image, Some(&options))
    };
    let count = unsafe { fetch.count() } as usize;
    let n = count.min(limit);

    let manager = unsafe { PHImageManager::defaultManager() };
    let mut photos: Vec<Value> = Vec::new();
    for i in 0..n {
        let asset = unsafe { fetch.objectAtIndex(i as _) };
        let (b64, uti) = request_image_data(&manager, &asset);
        if b64.is_empty() {
            continue;
        }
        let format = uti_to_format(&uti);
        let width = unsafe { asset.pixelWidth() };
        let height = unsafe { asset.pixelHeight() };
        let created = unsafe { asset.creationDate() }.map(|d| {
            let secs: f64 = unsafe { objc2::msg_send![&d, timeIntervalSince1970] };
            epoch_to_iso(secs)
        });
        let mut p = json!({
            "format": format,
            "base64": b64,
            "width": width,
            "height": height,
        });
        if let Some(c) = created {
            p["createdAt"] = json!(c);
        }
        photos.push(p);
    }
    (true, json!({"photos": photos}))
}
