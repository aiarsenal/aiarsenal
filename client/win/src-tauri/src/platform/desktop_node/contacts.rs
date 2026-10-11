//! contacts.search 命令（查通讯录，对齐 openclaw node 协议）。
//!
//! params: query?, limit?(25,1-200)
//! result: contacts:[{identifier, displayName, givenName, familyName,
//!                    organizationName, phoneNumbers:[], emails:[]}]
//!
//! Windows 实现：枚举用户「联系人」目录（%USERPROFILE%\Contacts）下的 *.contact 文件
//!（Windows 联系人文件夹的标准格式，映射/邮件客户端「导出联系人」的落点）逐个解析。
//! .contact 是 XML（schemas.microsoft.com/contacts/2009/contact，元素带 c: 前缀，前缀
//! 不固定）——解析用命名空间宽容的正则取标签体，不引完整 XML 依赖。
//!
//! 与 mac CNContactStore 的语义差异（如实降级）：
//! - 没有系统级通讯录服务，只有文件目录；目录为空 = 空结果（不是错误）。
//! - displayName/givenName 等取自文件字段；identifier 用 contactId 元素，缺省退文件名。
//! - 查询过滤与 mac 同口径：query 大小写不敏感子串匹配 displayName/givenName/
//!   familyName/organizationName 任一；空 query = 全量（截 limit）。

use serde_json::{json, Value};

/// 命名空间宽容取标签体：`<c:TAG attrs>body</c:TAG>` / `<TAG>body</TAG>`（首个匹配）。
fn first_tag(doc: &str, tag: &str) -> Option<String> {
    let re = regex::Regex::new(&format!(
        r"(?is)<(?:[A-Za-z0-9_]+:)?{tag}(?:\s[^>]*)?>(.*?)</(?:[A-Za-z0-9_]+:)?{tag}\s*>"
    ))
    .ok()?;
    re.captures(doc)
        .and_then(|c| c.get(1))
        .map(|m| xml_unescape(m.as_str()))
}

/// 命名空间宽容取标签体：全部匹配。
fn all_tags(doc: &str, tag: &str) -> Vec<String> {
    let Ok(re) = regex::Regex::new(&format!(
        r"(?is)<(?:[A-Za-z0-9_]+:)?{tag}(?:\s[^>]*)?>(.*?)</(?:[A-Za-z0-9_]+:)?{tag}\s*>"
    )) else {
        return Vec::new();
    };
    re.captures_iter(doc)
        .filter_map(|c| c.get(1).map(|m| xml_unescape(m.as_str())))
        .collect()
}

/// XML 实体反转义（数字实体 + 常用命名实体；& 最后解，防二次解）。
fn xml_unescape(s: &str) -> String {
    let mut out = s.to_string();
    // 数字实体：&#x1F600; / &#128512;
    if let Ok(re) = regex::Regex::new(r"&#x([0-9A-Fa-f]+);") {
        out = re
            .replace_all(&out, |c: &regex::Captures| {
                u32::from_str_radix(&c[1], 16)
                    .ok()
                    .and_then(char::from_u32)
                    .map(|c| c.to_string())
                    .unwrap_or_default()
            })
            .into_owned();
    }
    if let Ok(re) = regex::Regex::new(r"&#(\d+);") {
        out = re
            .replace_all(&out, |c: &regex::Captures| {
                c[1].parse::<u32>().ok().and_then(char::from_u32).map(|c| c.to_string()).unwrap_or_default()
            })
            .into_owned();
    }
    out = out
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", "\"")
        .replace("&apos;", "'");
    out.replace("&amp;", "&")
}

pub fn search(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_contacts(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: contacts（通讯录能力已在 AI Arsenal 客户端设置中关闭，请重新开启后再试）"}}),
        );
    }
    let limit = params
        .get("limit")
        .and_then(|v| v.as_u64())
        .unwrap_or(25)
        .clamp(1, 200) as usize;
    let query = params
        .get("query")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .trim()
        .to_lowercase();

    let Some(home) = dirs::home_dir() else {
        return (true, json!({"contacts": []}));
    };
    let dir = home.join("Contacts");
    let mut contacts: Vec<Value> = Vec::new();
    let Ok(entries) = std::fs::read_dir(&dir) else {
        // 目录不存在 = 用户没建过联系人：空结果（与 mac 空库同形）
        return (true, json!({"contacts": []}));
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path
            .extension()
            .and_then(|e| e.to_str())
            .map(|e| !e.eq_ignore_ascii_case("contact"))
            .unwrap_or(true)
        {
            continue;
        }
        let Ok(raw) = std::fs::read_to_string(&path) else {
            continue;
        };
        // givenName/familyName 在 <Names> 块内；找不到块退全文（少数导出格式不带 Names 包装）
        let names_block = first_tag(&raw, "Names").unwrap_or_else(|| raw.clone());
        let given = first_tag(&names_block, "givenName").unwrap_or_default();
        let family = first_tag(&names_block, "familyName").unwrap_or_default();
        let org = first_tag(&raw, "CompanyName").unwrap_or_default();
        let display = if !given.is_empty() || !family.is_empty() {
            format!("{given} {family}").trim().to_string()
        } else {
            org.clone()
        };
        let display = if display.is_empty() {
            first_tag(&raw, "displayName").unwrap_or_default()
        } else {
            display
        };
        // 集合块内逐 <Value> 提取（块缺失 = 空列表）
        let phones = first_tag(&raw, "PhoneNumberCollection")
            .map(|b| all_tags(&b, "Value"))
            .unwrap_or_default();
        let emails = first_tag(&raw, "EmailAddressCollection")
            .map(|b| all_tags(&b, "Value"))
            .unwrap_or_default();
        let identifier = first_tag(&raw, "contactId")
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| {
                path.file_stem()
                    .map(|s| s.to_string_lossy().to_string())
                    .unwrap_or_default()
            });
        let val = json!({
            "identifier": identifier,
            "displayName": display,
            "givenName": given,
            "familyName": family,
            "organizationName": org,
            "phoneNumbers": phones,
            "emails": emails,
        });
        if !query.is_empty() {
            let hit = ["displayName", "givenName", "familyName", "organizationName"]
                .iter()
                .any(|k| {
                    val.get(k)
                        .and_then(|v| v.as_str())
                        .map(|s| s.to_lowercase().contains(&query))
                        .unwrap_or(false)
                });
            if !hit {
                continue;
            }
        }
        contacts.push(val);
        if contacts.len() >= limit {
            break;
        }
    }
    (true, json!({"contacts": contacts}))
}
