//! contacts.search 命令（查通讯录，对齐 openclaw node 协议）。
//!
//! params: query?, limit?(25,1-200)
//! result: contacts:[{identifier, displayName, givenName, familyName,
//!                    organizationName, phoneNumbers:[], emails:[]}]
//!
//! CNContactStore.enumerateContactsWithFetchRequest（block 每 contact 调一次）→
//! Arc<Mutex<Vec>> accumulate，达 limit 设 *stop=YES 停止枚举。

#![allow(non_snake_case)]

use std::ptr::NonNull;
use std::sync::{Arc, Mutex};

use block2::RcBlock;
use objc2::rc::Retained;
use objc2::runtime::{Bool, ProtocolObject};
use objc2::AnyThread;
use objc2_contacts::{
    CNContact, CNContactEmailAddressesKey, CNContactFamilyNameKey,
    CNContactFetchRequest, CNContactGivenNameKey, CNContactIdentifierKey, CNContactOrganizationNameKey,
    CNContactPhoneNumbersKey, CNContactStore, CNKeyDescriptor,
};
use objc2_foundation::{NSArray, NSError};
use serde_json::{json, Value};

/// 单个 CNContact → JSON（字段全 unsafe fn，extract 标 unsafe fn）。
unsafe fn extract_contact(c: &CNContact) -> Value {
    let identifier = c.identifier().to_string();
    let given = c.givenName().to_string();
    let family = c.familyName().to_string();
    let org = c.organizationName().to_string();
    let display = if !given.is_empty() || !family.is_empty() {
        format!("{given} {family}").trim().to_string()
    } else {
        org.clone()
    };
    let phones: Vec<String> = c
        .phoneNumbers()
        .iter()
        .map(|lv| lv.value().stringValue().to_string())
        .collect();
    let emails: Vec<String> = c
        .emailAddresses()
        .iter()
        .map(|lv| lv.value().to_string())
        .collect();
    json!({
        "identifier": identifier,
        "displayName": display,
        "givenName": given,
        "familyName": family,
        "organizationName": org,
        "phoneNumbers": phones,
        "emails": emails,
    })
}

pub fn search(params: &Value) -> (bool, Value) {
    if !super::permissions::ensure_contacts(true) {
        return (
            false,
            json!({"error": {"code": "UNAVAILABLE", "message": "PERMISSION_MISSING: contacts（请到 系统设置→隐私与安全→通讯录 勾选本 App）"}}),
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
        .to_lowercase();

    let results: Arc<Mutex<Vec<Value>>> = Arc::new(Mutex::new(Vec::new()));
    let store = unsafe { CNContactStore::new() };

    // keys: NSArray<ProtocolObject<dyn CNKeyDescriptor>>（NSString key 常量经 ProtocolObject 转）
    let key_refs: Vec<&ProtocolObject<dyn CNKeyDescriptor>> = unsafe {
        vec![
            ProtocolObject::from_ref(&*CNContactIdentifierKey),
            ProtocolObject::from_ref(&*CNContactGivenNameKey),
            ProtocolObject::from_ref(&*CNContactFamilyNameKey),
            ProtocolObject::from_ref(&*CNContactOrganizationNameKey),
            ProtocolObject::from_ref(&*CNContactPhoneNumbersKey),
            ProtocolObject::from_ref(&*CNContactEmailAddressesKey),
        ]
    };
    let keys = NSArray::from_slice(&key_refs);
    let req = unsafe { CNContactFetchRequest::initWithKeysToFetch(CNContactFetchRequest::alloc(), &keys) };

    let results_blk = Arc::clone(&results);
    let query_blk = query.clone();
    let block = RcBlock::new(move |contact: NonNull<CNContact>, stop: NonNull<Bool>| {
        let c = unsafe { contact.as_ref() };
        let val = unsafe { extract_contact(c) };
        if !query_blk.is_empty() {
            let hit = ["displayName", "givenName", "familyName", "organizationName"]
                .iter()
                .any(|k| {
                    val.get(k)
                        .and_then(|v| v.as_str())
                        .map(|s| s.to_lowercase().contains(&query_blk))
                        .unwrap_or(false)
                });
            if !hit {
                return;
            }
        }
        let mut guard = results_blk.lock().unwrap();
        if guard.len() >= limit {
            unsafe { *stop.as_ptr() = Bool::YES }; // 达 limit 停止枚举
            return;
        }
        guard.push(val);
    });

    let mut error: Option<Retained<NSError>> = None;
    let ok = unsafe {
        store.enumerateContactsWithFetchRequest_error_usingBlock(&req, Some(&mut error), &block)
    };
    if !ok {
        let msg = error
            .as_ref()
            .map(|e| e.localizedDescription().to_string())
            .unwrap_or_else(|| "通讯录查询失败".into());
        return (false, json!({"error": {"message": msg}}));
    }
    let contacts = results.lock().unwrap().clone();
    (true, json!({"contacts": contacts}))
}
