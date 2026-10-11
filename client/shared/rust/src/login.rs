//! device-code 登录中转工具。
//!
//! App 生成一次性 deviceCode → 浏览器登录后绑定 → App 用 deviceCode 换 JWT。
//! deviceCode 走 `aiarsenal://login?dc=<code>` 回调 URL 传回，JWT 不暴露在 URL。

use rand::Rng;

/// 生成 32 位 hex deviceCode（无时间序、不可枚举）。
pub fn generate_device_code() -> String {
    let mut rng = rand::thread_rng();
    let bytes: [u8; 16] = rng.gen();
    hex_encode(&bytes)
}

fn hex_encode(bytes: &[u8]) -> String {
    let mut s = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        s.push_str(&format!("{:02x}", b));
    }
    s
}

/// 从 `aiarsenal://login?dc=xxx` 形式的回调 URL 解析出 deviceCode。
/// 兼容 `aiarsenal://login#dc=xxx`。
pub fn parse_dc_from_deep_link(url: &str) -> Option<String> {
    let q = url.split('?').nth(1).or_else(|| url.split('#').nth(1))?;
    for pair in q.split('&') {
        let mut it = pair.splitn(2, '=');
        if it.next()? == "dc" {
            return Some(it.next()?.to_string());
        }
    }
    None
}

/// 从 API base URL 推导 UI 站点 URL：去掉 host 部分的 "api." 前缀。
/// http://api.aiarsenal.cc → http://aiarsenal.cc（test，HTTP + /etc/hosts）
/// https://api.aiarsenal.cc → https://aiarsenal.cc（prod）
/// 约定对齐 deploy.sh（DOMAIN 由 VITE_API_URL 去掉 api. 得到）。
/// 桌面登录页挂在 UI 站点（aiarsenal.cc），不在 API host（api.aiarsenal.cc 是纯 JSON 代理）。
/// vben 用 hash 路由，App 须打开 {ui_url}/#/client-login?dc=...（见各端 lib.rs open_login_page）。
pub fn derive_ui_url(api_base_url: &str) -> String {
    api_base_url.replace("://api.", "://")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_dc_from_query() {
        assert_eq!(
            parse_dc_from_deep_link("aiarsenal://login?dc=abc123"),
            Some("abc123".to_string())
        );
    }

    #[test]
    fn parses_dc_from_fragment() {
        assert_eq!(
            parse_dc_from_deep_link("aiarsenal://login#dc=xyz"),
            Some("xyz".to_string())
        );
    }

    #[test]
    fn ignores_other_params() {
        assert_eq!(
            parse_dc_from_deep_link("aiarsenal://login?foo=bar&dc=zzz&a=1"),
            Some("zzz".to_string())
        );
    }

    #[test]
    fn device_code_length() {
        let dc = generate_device_code();
        assert_eq!(dc.len(), 32);
        assert!(dc.chars().all(|c| c.is_ascii_hexdigit()));
    }
}
